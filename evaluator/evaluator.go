package evaluator

import (
	"beedance/ast"
	"beedance/object"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	// NULL is a singleton object representing the null value.
	NULL = &object.Null{}
	// TRUE is a singleton object representing the boolean true value.
	TRUE = &object.Boolean{Value: true}
	// FALSE is a singleton object representing the boolean false value.
	FALSE = &object.Boolean{Value: false}
	// EXIT is a singleton object used to signal the termination of a loop.
	EXIT = &object.Exit{}

	// currentResultVar is the internal name used in the environment to store the
	// Instruction List (IL) accumulator, also known as the Current Result (CR).
	currentResultVar = "__CURRENT_RESULT__"

	// nowFunc is a variable that holds the function to get the current time.
	// It can be overridden in tests to provide a mock time source for deterministic testing of timers.
	nowFunc = time.Now
)

// ioMap simulates a hardware I/O map for located variables (AT %).
var ioMap = make(map[string]object.Object)

// integerTypeRanges defines the minimum and maximum values for standard IEC integer types.
// This is used for overflow/underflow checking during type conversions and arithmetic.
var integerTypeRanges = map[string]struct {
	minSigned   int64
	maxSigned   int64
	maxUnsigned uint64
}{
	"SINT":  {math.MinInt8, math.MaxInt8, 0},
	"INT":   {math.MinInt16, math.MaxInt16, 0},
	"DINT":  {math.MinInt32, math.MaxInt32, 0},
	"LINT":  {math.MinInt64, math.MaxInt64, 0},
	"USINT": {0, 0, math.MaxUint8},
	"UINT":  {0, 0, math.MaxUint16},
	"UDINT": {0, 0, math.MaxUint32},
	"ULINT": {0, 0, math.MaxUint64},
}

// standardFBs maps the names of standard IEC 61131-3 function blocks to their
// corresponding evaluation functions. This allows the evaluator to dynamically
// call the correct logic for built-in FBs like TON, CTU, etc.
var standardFBs = map[string]*object.BuiltinFunctionBlock{
	"TON":    {Fn: evalTON},
	"TOF":    {Fn: evalTOF},
	"CTU":    {Fn: evalCTU},
	"CTD":    {Fn: evalCTD},
	"R_TRIG": {Fn: evalR_TRIG},
	"F_TRIG": {Fn: evalF_TRIG},
	"CTUD":   {Fn: evalCTUD},
	"TP":     {Fn: evalTP},
	"SR":     {Fn: evalSR},
	"RS":     {Fn: evalRS},
}

// Eval is the main entry point for the evaluator. It recursively traverses an
// Abstract Syntax Tree (AST) node, evaluating it within the context of a given
// environment and returning the resulting runtime object.
func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {

	// Statements
	case *ast.Program:
		// A Program is the root of the AST, and its evaluation is the sequential evaluation of its statements.
		return evalProgram(node, env)

	case *ast.SFCProgram:
		return evalSFCProgram(node, env)

	case *ast.ConfigurationDeclaration:
		return evalConfigurationDeclaration(node, env)

	case *ast.TaskDeclaration:
		return evalTaskDeclaration(node, env)

	case *ast.BlockStatement:
		// A BlockStatement can be either a standard block of ST statements or,
		// through a heuristic, the body of an Instruction List (IL) program.
		// Check if this is an IL program body
		if len(node.Statements) > 0 {
			if _, ok := node.Statements[0].(*ast.IlInstructionStatement); ok {
				return evalIlProgram(node.Statements, env)
			}
		}
		return evalBlockStatement(node, env)

	case *ast.GlobalVarDeclaration:
		// A GlobalVarDeclaration is a block of global variables.
		return evalGenericVarBlock(node.Vars, env)
	case *ast.ExternalVarDeclaration:
		// An ExternalVarDeclaration is a block of external variables.
		return evalGenericVarBlock(node.Vars, env)
	case *ast.AccessVarDeclaration:
		return evalAccessVarDeclaration(node, env)
	case *ast.ConfigVarDeclaration:
		return evalVarConfigDeclaration(node, env)
	case *ast.TempVarDeclaration:
		// A TempVarDeclaration defines temporary variables for a POU.
		// For a single evaluation pass, VAR_TEMP is the same as VAR.
		// The cyclical re-initialization would be handled by the scheduler.
		return evalGenericVarBlock(node.Vars, env)

	case *ast.VarBlockDeclaration:
		// A VarBlockDeclaration is a standard block of local variables.
		return evalVarBlockStatement(node, env)

	// SFC elements are handled within the context of a program/function block body, not as standalone statements.

	case *ast.TypeBlockDeclaration:
		// A TypeBlockDeclaration defines one or more user-defined types.
		// We store the AST node for the type declaration itself in the environment,
		// prefixed with `_type_` to avoid name collisions. The evaluator can then
		// inspect this node when a variable of this type is declared.
		for _, decl := range node.Declarations {
			// Before storing, perform validation for subrange types.
			if subrange, ok := decl.Subrange.(*ast.InfixExpression); ok && subrange.Operator == ".." {
				// 1. Validate that the base type is an integer type.
				baseTypeStr := decl.DataType.String()
				if !object.IsIntegerType(baseTypeStr) {
					return newError(decl, "subrange base type must be an integer type, got %s", baseTypeStr)
				}

				// 2. Evaluate the bounds.
				lower := Eval(subrange.Left, env)
				if isError(lower) {
					return lower
				}
				upper := Eval(subrange.Right, env)
				if isError(upper) {
					return upper
				}

				// 3. Validate that the bounds themselves are integers.
				_, _, okL := object.GetIntegerObjectValue(lower)
				_, _, okU := object.GetIntegerObjectValue(upper)
				if !okL || !okU {
					return newError(decl, "subrange bounds must be integers, got %s and %s", lower.Type(), upper.Type())
				}
			}

			env.Set("_type_"+decl.Name.Value, &object.Quote{Node: decl})
		}
		return NULL

	case *ast.ExpressionStatement:
		// An ExpressionStatement is a statement that consists of a single expression (e.g., a function call).
		return Eval(node.Expression, env)

	case *ast.FunctionDeclaration:
		// A FunctionDeclaration creates a new Function object and stores it in the environment.
		fn := &object.Function{
			Name:       node.Name,
			VarInputs:  node.VarInputs,
			VarOutputs: node.VarOutputs,
			VarInOuts:  node.VarInOuts,
			Vars:       node.Vars,
			Body:       node.Body,
			Env:        env,
		}
		// Store with a `_function_` prefix to avoid being shadowed by variables.
		env.Set("_function_"+node.Name.Value, fn)
		return fn

	case *ast.FunctionBlockDeclaration:
		// A FunctionBlockDeclaration creates a "template" or "class" for a function block,
		// which can then be instantiated as variables.
		fb := &object.FunctionBlock{
			Name:       node.Name,
			VarInputs:  node.VarInputs,
			VarOutputs: node.VarOutputs,
			VarInOuts:  node.VarInOuts,
			Vars:       node.Vars,
			VarTemp:    node.VarTemp,
			Body:       node.Body,
			Env:        env, // The environment where the FB is declared
		}
		env.Set(node.Name.Value, fb)
		return fb

	case *ast.ProgramDeclaration:
		// When a PROGRAM is declared, we create an object representing it.
		// This object needs its own persistent environment for its static VARs.
		instanceEnv := object.NewEnclosedEnvironment(env)
		prog := &object.Program{
			Name:       node.Name,
			VarInputs:  node.VarInputs,
			VarOutputs: node.VarOutputs,
			VarInOuts:  node.VarInOuts,
			Vars:       node.Vars,
			VarTemp:    node.VarTemp,
			Body:       node.Body,
			Env:        instanceEnv, // The program's own persistent environment
		}
		// Set the program definition in the outer environment.
		env.Set(node.Name.Value, prog)

		// When a program is declared, its static variables (VAR, VAR_INPUT, etc.)
		// must be initialized within its persistent environment.
		allVarBlocks := [][]*ast.VarDeclStatement{
			node.VarInputs,
			node.VarOutputs,
			node.VarInOuts,
			node.Vars,
		}
		for _, varBlock := range allVarBlocks {
			for _, varDecl := range varBlock {
				if err := evalVarDeclStatement(varDecl, instanceEnv); isError(err) {
					return err
				}
			}
		}

		// Initialize the accumulator for IL programs.
		instanceEnv.Set(currentResultVar, NULL)

		// If the program has an SFC body, evaluate it to create the SFC object instance
		// and store it in the program's persistent environment.
		if sfcAST, isSFC := node.Body.(*ast.SFCProgram); isSFC {
			sfcObj := evalSFCProgram(sfcAST, instanceEnv)
			instanceEnv.Set("__sfc_instance__", sfcObj)
			// For testing convenience, return the created SFC object.
			return sfcObj
		}

		// For non-SFC programs, the declaration itself doesn't return a value.
		return prog

	case *ast.ReturnStatement:
		// A ReturnStatement evaluates its return value and wraps it in a ReturnValue object to signal a return.
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.TypedLiteral:
		// A TypedLiteral (e.g., INT#10, T#5s) is parsed and converted into the
		// corresponding runtime object, with type and range checking.
		targetTypeName := strings.ToUpper(node.TypeName)
		if !isKnownType(targetTypeName, env) {
			return newError(node, "unknown type: %s", targetTypeName)
		}

		// The parser now gives us an identifier with the full value string.
		valueIdent, ok := node.Value.(*ast.Identifier)
		if !ok {
			return newError(node, "internal error: value for typed literal is not an identifier, got %T", node.Value)
		}
		valueStr := valueIdent.Value

		// 1. Handle time/date types
		if object.IsTimeDateKeyword(targetTypeName) {
			return applyTimeDateConversion(valueStr, targetTypeName)
		}

		// Handle BOOL type
		if object.IsBooleanType(targetTypeName) {
			upperVal := strings.ToUpper(valueStr)
			if upperVal == "1" || upperVal == "TRUE" {
				return TRUE
			}
			if upperVal == "0" || upperVal == "FALSE" {
				return FALSE
			}
			return newError(node, "invalid value for BOOL literal: %s", valueStr)
		}

		// 2. Handle bit string types (BYTE, WORD, etc.)
		if object.IsBitStringType(targetTypeName) {
			// These can have a base, e.g., BYTE#16#FF. The valueStr will be "16#FF".
			return applyBitStringConversion(valueStr, targetTypeName)
		}

		// 3. Handle numeric types (INT, REAL, etc.)
		if object.IsIntegerType(targetTypeName) || object.IsRealType(targetTypeName) {
			return applyNumericConversion(valueStr, targetTypeName)
		}

		// 4. Handle enumerated types (e.g., COLOR#RED)
		if typeQuote, ok := env.Get("_type_" + targetTypeName); ok {
			if quote, isQuote := typeQuote.(*object.Quote); isQuote {
				if typeDecl, isTypeDecl := quote.Node.(*ast.TypeDeclaration); isTypeDecl {
					if enumDef, isEnumDef := typeDecl.DataType.(*ast.EnumDefinition); isEnumDef {
						// It's an enum type. Check if the value exists.
						valueFound := false
						for _, enumVal := range enumDef.Values {
							if enumVal.Value == valueStr {
								valueFound = true
								break
							}
						}
						if valueFound {
							return &object.EnumeratedValue{TypeName: targetTypeName, Value: valueStr}
						}
						return newError(node, "enumerated value '%s' not found in type '%s'", valueStr, targetTypeName)
					}
				}
			}
		}

		return newError(node, "unsupported typed literal: %s#%s", targetTypeName, valueStr)

	// Expressions
	case *ast.IntegerLiteral:
		// An untyped integer literal is promoted to the largest integer type (LINT) to prevent overflow during intermediate calculations.
		// Untyped integer literals are treated as the largest possible integer type (LINT)
		// to allow for implicit type promotion in expressions without overflow.
		return &object.LInt{Value: node.Value}

	case *ast.UnsignedIntegerLiteral:
		// Untyped unsigned integer literals are treated as ULINT.
		return &object.ULInt{Value: node.Value}

	case *ast.RealLiteral:
		// A real literal is evaluated as either a REAL (float32, represented as float64) or LREAL (float64).
		// Distinguish between REAL and LREAL based on the precision set by the parser.
		if node.Precision == 64 {
			return &object.LReal{Value: node.Value}
		}
		// Default to REAL for 32-bit or unspecified precision.
		// Note: We use float64 internally for both for simplicity in Go.
		return &object.Real{Value: node.Value}

	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.WStringLiteral:
		return &object.WString{Value: node.Value}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.BitStringLiteral:
		return &object.BitString{Value: node.Value, Width: node.Width}

	case *ast.PrefixExpression:
		// A PrefixExpression (e.g., -5, NOT TRUE) is evaluated by first evaluating its operand, then applying the operator.
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node, right)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return object.EvalInfix(left, node.Operator, right)

	case *ast.MemberAccessExpression:
		// A MemberAccessExpression (e.g., MyTimer.Q) accesses a field of a struct or function block instance.
		return evalMemberAccessExpression(node, env)

	case *ast.IfStatement:
		return evalIfStatement(node, env)
	case *ast.ForLoopStatement:
		// Debug: print environment names before executing the loop
		// println("DEBUG: before FOR loop, env names:")
		// for _, n := range env.Names() {
		// 	println("DEBUG:  -", n)
		// }

		res := evalForLoopStatement(node, env)

		// Debug: print environment names after executing the loop
		// println("DEBUG: after FOR loop, env names:")
		// for _, n := range env.Names() {
		// 	println("DEBUG:  -", n)
		// }
		return res
	case *ast.WhileStatement:
		return evalWhileStatement(node, env)
	case *ast.RepeatStatement:
		return evalRepeatStatement(node, env)
	case *ast.ExitStatement:
		// An ExitStatement returns a special EXIT object to signal loop termination.
		// EXIT statements simply return a special EXIT object
		// that loop evaluators will catch.
		return EXIT
	case *ast.CaseStatement:
		return evalCaseStatement(node, env)

	case *ast.VarDeclStatement:
		// A VarDeclStatement evaluates the initial value (if any) and sets the variable in the environment.
		return evalVarDeclStatement(node, env)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.FunctionLiteral:
		// A FunctionLiteral is evaluated into a runtime Function object.
		// The parser now provides VarInputs directly on the FunctionLiteral node.
		return &object.Function{
			VarInputs: node.VarInputs, Env: env, Body: node.Body,
		}

	case *ast.CallExpression:
		// A CallExpression can be a regular function call, a function block call, or a macro invocation.
		// Special handling for 'quote' macro
		if node.Function.TokenLiteral() == "EXPR" {
			if len(node.Arguments) != 1 {
				return newError(node, "wrong number of arguments for EXPR. got=%d, want=1", len(node.Arguments))
			}
			return quote(node.Arguments[0], env)
		}

		var function object.Object
		// When evaluating a call, we first look for a mangled function name to avoid
		// ambiguity with variables of the same name.
		if ident, ok := node.Function.(*ast.Identifier); ok {
			if fnObj, ok := env.Get("_function_" + ident.Value); ok {
				function = fnObj
			}
		}

		if function == nil {
			// Fallback to normal evaluation for function block instances, built-ins, etc.
			function = Eval(node.Function, env)
		}

		if isError(function) {
			return function
		}

		return applyFunction(function, node.Arguments, env, node)

	case *ast.ArrayLiteral:
		// An ArrayLiteral is evaluated by evaluating each of its elements and creating an Array object.
		var elements []object.Object
		for _, el := range node.Elements {
			if rep, ok := el.(*ast.ArrayRepetition); ok {
				// It's a repetition, e.g., 3(0)
				// Evaluate the factor
				factorObj := Eval(rep.Factor, env)
				if isError(factorObj) {
					return factorObj
				}
				factor, _, ok := object.GetIntegerObjectValue(factorObj)
				if !ok {
					return newError(rep.Factor, "array repetition factor must be an integer, got %s", factorObj.Type())
				}
				if factor < 0 {
					return newError(rep.Factor, "array repetition factor cannot be negative, got %d", factor)
				}

				// Evaluate the elements to be repeated
				repeatedElements := evalExpressions(rep.Elements, env)
				if len(repeatedElements) == 1 && isError(repeatedElements[0]) {
					return repeatedElements[0]
				}

				// Append the elements `factor` times
				for i := 0; i < int(factor); i++ {
					elements = append(elements, repeatedElements...)
				}
			} else {
				// It's a regular element
				evaluated := Eval(el, env)
				if isError(evaluated) {
					return evaluated
				}
				elements = append(elements, evaluated)
			}
		}
		return &object.Array{Elements: elements}

	case *ast.IndexExpression:
		// An IndexExpression (e.g., MyArray[i]) is evaluated by evaluating the array/hash and the index, then performing the lookup.
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(node, left, index)

	case *ast.HashLiteral:
		return evalHashLiteral(node, env)

	// An IlInstructionStatement is evaluated by the dedicated IL instruction evaluator.
	case *ast.IlInstructionStatement:
		return evalIlInstructionStatement(node, env)

	case *ast.AssignmentStatement:
		return evalAssignmentStatement(node, env)
	}
	return nil
}

// evalSFCProgram builds the runtime SFC object from the AST. It populates the
// steps, transitions, and actions, and sets the initial step. This object can
// then be "cycled" by the `evalSFCCycle` function to simulate PLC execution.
func evalSFCProgram(program *ast.SFCProgram, env *object.Environment) object.Object {
	sfc := &object.SFC{
		Steps:       make(map[string]*object.Step),
		Transitions: []*object.Transition{},
		Actions:     make(map[string]*object.Action),
		ActiveSteps: make(map[string]bool),
	}

	// 1. First Pass: Pre-populate all defined ACTIONs with their bodies.
	// This ensures that when we encounter an action call in a step, the
	// action object (including its ST body) already exists.
	// We implement a local 'walk' function to traverse the AST since ast.Inspect is not available.
	var walk func(ast.Node)
	walk = func(node ast.Node) {
		if node == nil {
			return
		}

		// Check if the current node is an ActionStatement.
		if actionStmt, ok := node.(*ast.ActionStatement); ok {
			actionName := actionStmt.Name.Value
			var body *ast.BlockStatement
			if actionStmt.Body != nil {
				body, _ = actionStmt.Body.(*ast.BlockStatement)
			}
			sfc.Actions[actionName] = &object.Action{Name: actionStmt.Name, Body: body, AssociatedSteps: []*object.Step{}}
			// We don't need to walk inside the action's body for this pass.
			return
		}

		// Recursively walk through the children of the node.
		// This is a simplified traversal covering the most likely places for actions.
		switch n := node.(type) {
		case *ast.SFCProgram:
			for _, el := range n.Elements {
				walk(el)
			}
		case *ast.ProgramDeclaration:
			// Actions can be inside VAR blocks in some test cases.
			for _, varBlock := range n.VarGlobal {
				walk(varBlock)
			}
			walk(n.Body)
		case *ast.VarBlockDeclaration:
			for _, decl := range n.Declarations {
				// In some legacy structures, an action might be part of a declaration.
				// This is not standard but we handle it for robustness.
				walk(decl)
			}
		}
	}
	walk(program)

	// 2. Second Pass: Build the Step and Transition structure and associate steps with the pre-populated actions.
	for _, element := range program.Elements {
		switch elem := element.(type) {
		case *ast.StepStatement: // Initial steps are also regular steps
			if elem.IsInitial {
				sfc.InitialStepName = elem.Name.Value
			}
			// The parser puts action associations into elem.Actions. The evaluator's
			// cycle logic expects them in the Body of the runtime object.Step.
			// We create a synthetic BlockStatement to hold them.
			body := &ast.BlockStatement{Statements: []ast.Statement{}}
			for _, action := range elem.Actions {
				body.Statements = append(body.Statements, action)
			}

			step := &object.Step{Name: elem.Name, Body: body, IsActive: false}
			sfc.Steps[elem.Name.Value] = step

			// Now, associate the step with its actions.
			for _, actionAssoc := range elem.Actions {
				actionName := actionAssoc.ActionName.Value

				// Ensure the action object exists
				if _, exists := sfc.Actions[actionName]; !exists {
					sfc.Actions[actionName] = &object.Action{Name: &ast.Identifier{Value: actionName}}
				}
				// Associate this step with the action
				sfc.Actions[actionName].AssociatedSteps = append(sfc.Actions[actionName].AssociatedSteps, step)
			}

		case *ast.TransitionStatement:
			sfc.Transitions = append(sfc.Transitions, &object.Transition{
				FromSteps: elem.From,
				ToSteps:   elem.To,
				Condition: elem.Condition,
			})

		}
	}

	// 3. Initialize the SFC state
	if sfc.InitialStepName == "" {
		return newError(program, "SFC program has no initial step") //
	}
	sfc.ActiveSteps[sfc.InitialStepName] = true
	sfc.Steps[sfc.InitialStepName].IsActive = true
	// Set the activation time for the initial step.
	sfc.Steps[sfc.InitialStepName].ActivationTime = nowFunc()

	// Return the initialized SFC object. The caller (e.g., a test or a scheduler) is responsible for cycling it.
	return sfc
}

// evalSFCCycle simulates one scan cycle of an SFC. It follows the standard
// five-phase execution model:
// 1. Evaluate action control logic. 2. Update action outputs and execute bodies.
// 3. Evaluate transitions. 4. Update step states. 5. Re-evaluate actions for new steps.
func evalSFCCycle(sfc *object.SFC, env *object.Environment) object.Object {
	// Phase 1: Evaluate Action Control Logic
	for _, action := range sfc.Actions {
		evaluateAction(action, env)
	}

	// Phase 2: Update Action Outputs & Execute Action Bodies
	// This must happen BEFORE evaluating transitions so that transition conditions
	// see the current state of the action outputs.
	for _, action := range sfc.Actions {
		// Update the boolean variable in the environment to reflect the action's active state.
		if _, ok := env.Get(action.Name.Value); ok {
			env.Assign(action.Name.Value, nativeBoolToBooleanObject(action.IsActive))
		}

		// If the action is active and has a body (ST code), evaluate it.
		if action.IsActive && action.Body != nil {
			// We must evaluate the action body in the *same* environment as the SFC cycle
			// to ensure that assignments within the action (e.g., `x := x + 1`) modify
			// the actual program variables, not variables in a temporary, enclosed scope.
			res := Eval(action.Body, env)
			if isError(res) {
				// Propagate the error immediately to halt the current SFC cycle.
				return res
			}
		}
	}

	// Phase 3: Evaluate Transitions
	transitionsToClear := []*object.Transition{}
	for _, transition := range sfc.Transitions {
		// Check if the transition is enabled (all preceding steps are active)
		isEnabled := true
		for _, fromStepIdent := range transition.FromSteps {
			if !sfc.ActiveSteps[fromStepIdent.Value] {
				isEnabled = false
				break
			}
		}

		if isEnabled {
			conditionResult := Eval(transition.Condition, env)
			if isTruthy(conditionResult) {
				transitionsToClear = append(transitionsToClear, transition)
			}
		}
	}

	// Phase 4: Update Step States for the next cycle
	for _, transition := range transitionsToClear {
		for _, fromStep := range transition.FromSteps {
			delete(sfc.ActiveSteps, fromStep.Value)
			step := sfc.Steps[fromStep.Value]
			step.IsActive = false
			step.ActivationTime = time.Time{} // Reset timer when step deactivates
		}
		for _, toStep := range transition.ToSteps {
			sfc.ActiveSteps[toStep.Value] = true
			step := sfc.Steps[toStep.Value]
			step.IsActive = true
			step.ActivationTime = nowFunc() // Set activation time
		}
	}

	// Phase 5: Re-evaluate actions for any newly activated steps.
	// This ensures that the actions of a new step are executed in the same
	// cycle in which the transition occurs, making the SFC's behavior more immediate.
	if len(transitionsToClear) > 0 {
		// Create a set of actions that need re-evaluation to avoid redundant processing.
		actionsToReEvaluate := make(map[string]*object.Action) // cspell:disable-line
		for _, transition := range transitionsToClear {
			// Add actions from newly DEACTIVATED steps to ensure they are turned off if non-stored.
			for _, fromStepIdent := range transition.FromSteps {
				for _, action := range sfc.Actions {
					for _, associatedStep := range action.AssociatedSteps {
						if associatedStep.Name.Value == fromStepIdent.Value {
							actionsToReEvaluate[action.Name.Value] = action
						}
					}
				}
			}

			// Add actions from newly ACTIVATED steps.
			for _, toStepIdent := range transition.ToSteps {
				// Find all actions associated with this newly activated step.
				for _, action := range sfc.Actions { // cspell:disable-line
					for _, associatedStep := range action.AssociatedSteps {
						if associatedStep.Name.Value == toStepIdent.Value {
							actionsToReEvaluate[action.Name.Value] = action
						}
					}
				}
			}
		}

		// Now, re-run the evaluation and update logic for these specific actions.
		for _, action := range actionsToReEvaluate {
			evaluateAction(action, env)
			if _, ok := env.Get(action.Name.Value); ok {
				env.Assign(action.Name.Value, nativeBoolToBooleanObject(action.IsActive))
			}
			if action.IsActive && action.Body != nil {
				res := Eval(action.Body, env)
				if isError(res) {
					return res // Propagate error if action body fails.
				}
			}
		}
	}

	return NULL // A single cycle completes successfully
}

// evaluateAction determines the active state (`.IsActive`) of a single action
// based on its highest-priority qualifier among all currently active associated
// steps. It handles the logic for all standard qualifiers (N, S, R, P, D, L, etc.).
func evaluateAction(action *object.Action, env *object.Environment) {
	qualifier, isStepActive := getHighestPriorityActiveQualifier(action, env)

	// --- Case 1: The controlling step is ACTIVE ---
	if isStepActive {
		// If the qualifier has changed from a previous active step, reset timers/counters.
		if qualifier != action.Qualifier {
			action.TimerStart = time.Time{}
			action.ActivationCount = 0
		}
		// The current qualifier is now the action's controlling qualifier.
		action.Qualifier = qualifier

		switch qualifier {
		case "N", "S":
			action.IsActive = true
		case "R":
			action.IsActive = false
			action.TimerStart = time.Time{} // Reset timer on R
		case "P":
			action.IsActive = (action.ActivationCount == 0)
			action.ActivationCount++
		case "D": // Non-stored delayed
			// Timer starts when step becomes active. Action becomes active when timer >= duration.
			if action.TimerStart.IsZero() {
				action.TimerStart = nowFunc()
			}
			action.IsActive = nowFunc().Sub(action.TimerStart) >= action.Duration
		case "L": // Non-stored limited
			if action.TimerStart.IsZero() {
				action.TimerStart = nowFunc()
			}
			action.IsActive = nowFunc().Sub(action.TimerStart) < action.Duration
		case "SD", "DS": // Stored delayed
			if action.TimerStart.IsZero() {
				action.TimerStart = nowFunc()
				action.IsActive = false // Stays false until timer elapses
			} else if !action.IsActive { // Only check to turn it on, not off.
				// Only turn it on if timer is met, don't turn it off.
				if nowFunc().Sub(action.TimerStart) >= action.Duration {
					action.IsActive = true
				}
			}
		case "SL": // Stored limited
			if action.TimerStart.IsZero() {
				action.TimerStart = nowFunc()
				action.IsActive = true // Active immediately
			} else if nowFunc().Sub(action.TimerStart) >= action.Duration {
				// Once the time limit is reached, it turns off and stays off
				// because it's a "stored" action. It needs an 'R' to reset.
				action.IsActive = false
			}
		}
		return // Done with active step logic.
	}

	// --- Case 2: The controlling step is NOT ACTIVE ---
	// The action's behavior now depends on its stored qualifier from when it was last active.
	switch action.Qualifier {
	case "S":
		// Stored, remains active.
		return
	case "SD", "DS":
		// Stored delayed. If it became active, it stays active.
		// If it was not yet active, the timer continues.
		if !action.IsActive && !action.TimerStart.IsZero() {
			if nowFunc().Sub(action.TimerStart) >= action.Duration {
				action.IsActive = true
			}
		}
		return
	case "SL":
		// Stored limited. If it was active, the timer continues to run it down.
		if action.IsActive && !action.TimerStart.IsZero() {
			if nowFunc().Sub(action.TimerStart) >= action.Duration {
				action.IsActive = false
			}
		}
		return
	default:
		// All other qualifiers are non-stored (N, P, D, L, R). They become inactive.
		action.IsActive = false
	}

	// Reset timers for non-stored timed actions when their step deactivates.
	if action.Qualifier == "D" || action.Qualifier == "L" {
		action.TimerStart = time.Time{}
	}
}

// getHighestPriorityActiveQualifier finds the highest-priority qualifier for a
// given action among all of its associated steps that are currently active.
// It respects the standard IEC 61131-3 precedence: R > S > (all others).
// IEC 61131-3 specifies the precedence: R > S > (all others).
func getHighestPriorityActiveQualifier(action *object.Action, env *object.Environment) (qualifier string, isStepActive bool) {
	qualifierPrecedence := map[string]int{"R": 3, "S": 2} // R and S have highest precedence
	highestQualifier := "N"                               // Default qualifier
	highestPrecedence := 0
	anyStepActive := false

	for _, step := range action.AssociatedSteps {
		if step.IsActive {
			anyStepActive = true
			if step.Body == nil {
				continue
			}
			for _, stmt := range step.Body.Statements { // cspell:disable-line
				actionAssoc, ok := stmt.(*ast.ActionBlockStatement)
				if !ok {
					continue
				}

				if actionAssoc.ActionName.Value == action.Name.Value {
					q := "N" // Default qualifier is Non-stored
					if actionAssoc.Qualifier != nil {
						q = actionAssoc.Qualifier.Value
					}
					// If it's a timed qualifier, parse the duration.
					switch q {
					case "D", "L", "SD", "DS", "SL":
						if actionAssoc.Duration != nil {
							// The duration is the second argument. We need to evaluate it.
							durationObj := Eval(actionAssoc.Duration, env)
							if timeObj, ok := durationObj.(*object.Time); ok {
								action.Duration = timeObj.Value
							}
						}
					}

					if precedence, ok := qualifierPrecedence[q]; ok {
						if precedence > highestPrecedence {
							highestPrecedence = precedence
							highestQualifier = q
						}
					} else if highestPrecedence == 0 { // If no R or S found yet, take the current one (N, P, etc.)
						highestQualifier = q
					}
				}
			}
		}
	}
	return highestQualifier, anyStepActive
}

// evalProgram evaluates a program by sequentially evaluating its statements.
// It returns the value of the last evaluated statement, or a ReturnValue/Error if one is encountered.
func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		// Special handling for configuration blocks at the top level.
		// They must be evaluated in the provided environment to correctly populate it.
		config, ok := statement.(*ast.ConfigurationDeclaration)
		if ok {
			return evalConfigurationDeclaration(config, env)
		}

		result = Eval(statement, env)

		// At the program level, a RETURN statement should halt execution and return its value.
		if returnValue, ok := result.(*object.ReturnValue); ok {
			return returnValue.Value
		}

		if isError(result) {
			return result
		}
	}
	return result
}

// evalIlProgram evaluates a block of Instruction List (IL) statements. It first
// builds a map of labels to their positions, then executes the instructions
// sequentially, handling jumps and returns.
func evalIlProgram(stmts []ast.Statement, env *object.Environment) object.Object {
	// 1. Build a map of labels to program counter indices.
	labelMap := make(map[string]int)
	for i, stmt := range stmts {
		if ilStmt, ok := stmt.(*ast.IlInstructionStatement); ok {
			if ilStmt.Label != nil {
				// Check for duplicate labels
				if _, exists := labelMap[ilStmt.Label.Value]; exists {
					return newError(ilStmt, "duplicate label defined: %s", ilStmt.Label.Value)
				}
				labelMap[ilStmt.Label.Value] = i
			}
		}
	}

	// 2. Execute statements using a program counter.
	var result object.Object
	pc := 0
	for pc < len(stmts) {
		result = Eval(stmts[pc], env)

		// Propagate errors immediately.
		if isError(result) {
			return result
		}

		// Handle jumps
		if jump, ok := result.(*object.Jump); ok {
			targetIdx, exists := labelMap[jump.TargetLabel]
			if !exists {
				return newError(stmts[pc], "jump target label not found: %s", jump.TargetLabel)
			}
			pc = targetIdx // Set PC to the target index for the next iteration
			continue
		}
		// Handle returns
		if _, ok := result.(*object.Return); ok {
			// A RET instruction was hit. Stop executing this IL program.
			// The final value of the accumulator will be returned after the loop.
			break
		}

		// If the result is not a flow-control object, it represents the new value of the accumulator.
		// We update the CR in the environment for the next instruction to use.
		if result != nil {
			env.Set(currentResultVar, result)
		}

		pc++ // Increment PC for next instruction
	}

	// The result of an IL program is the final value of the accumulator (Current Result).
	// We fetch it from the environment after all instructions have run.
	finalResult, ok := env.Get(currentResultVar)
	if !ok {
		// If the accumulator was never loaded (e.g., an empty program or a program
		// with only ST instructions), the result is undefined. Return NULL.
		return NULL
	}
	return finalResult
}

// evalIlInstructionStatement evaluates a single IL instruction. It handles the
// operator, operand, and any modifiers (like 'N' for negation or 'C' for
// conditional execution), updating the Current Result (CR) in the environment.
func evalIlInstructionStatement(node *ast.IlInstructionStatement, env *object.Environment) object.Object {
	op := strings.ToUpper(node.Operator)
	modifier := strings.ToUpper(node.Modifier)

	// Deconstruct combined mnemonics like JMPC, JMPCN, etc.
	if strings.HasSuffix(op, "CN") {
		op = strings.TrimSuffix(op, "CN")
		modifier += "CN"
	} else if strings.HasSuffix(op, "C") {
		op = strings.TrimSuffix(op, "C")
		modifier += "C"
	} else if strings.HasSuffix(op, "N") {
		op = strings.TrimSuffix(op, "N")
		modifier += "N"
	}

	// 1. Handle conditional execution for CAL, RET. JMP is handled separately.
	isConditional := (op == "RET") && strings.Contains(modifier, "C")
	if isConditional {
		crObj, ok := env.Get(currentResultVar)
		crIsTruthy := ok && isTruthy(crObj)
		isNegated := strings.Contains(modifier, "N")

		shouldExecute := false
		if isNegated { // For JMPCN, CALCN, RETCN
			shouldExecute = !crIsTruthy
		} else { // For JMPC, CALC, RETC
			shouldExecute = crIsTruthy
		}

		if !shouldExecute {
			cr, _ := env.Get(currentResultVar)
			return cr
		}
	}

	// Execute the operator logic
	switch op {
	case "LD":
		operand := evalOperand(node.Operand, env)
		if isError(operand) {
			return operand
		}
		operand = applyNegationToOperand(operand, modifier, isConditional, node)
		if isError(operand) {
			return operand
		}
		// The loop now sets the CR from the return value.
		return operand

	case "ST":
		crObj, ok := env.Get(currentResultVar)
		if !ok {
			return newError(node, "ST instruction executed but Current Result is not set")
		}

		isNegatedOperand := strings.Contains(modifier, "N") && !isConditional
		if isNegatedOperand {
			// Create a synthetic node for error reporting, using the token from the original instruction.
			dummyNode := &ast.PrefixExpression{Token: node.Token, Operator: "NOT", Right: &ast.Identifier{Value: "CR"}}
			crObj = evalNotOperatorExpression(dummyNode, crObj)
			if isError(crObj) {
				// This error would typically be "unknown operator: NOT<TYPE>" if CR is not a boolean/bitstring.
				return crObj
			}
		}

		if targetIdent, ok := node.Operand.(*ast.Identifier); ok {
			env.Assign(targetIdent.Value, crObj)
			return crObj
		}
		return newError(node, "operand for ST must be a variable identifier")

	case "S", "R": // Set, Reset
		if targetIdent, ok := node.Operand.(*ast.Identifier); ok {
			// The S and R instructions only execute if the current result is TRUE.
			if cr, ok := env.Get(currentResultVar); ok && object.IsTruthy(cr) {
				var valToSet *object.Boolean
				if op == "S" {
					valToSet = TRUE
				} else { // op == "R"
					valToSet = FALSE
				}
				env.Assign(targetIdent.Value, valToSet)
			}
			// S and R instructions do not modify the current result. The result of
			// the instruction is the (unmodified) current result.
			cr, _ := env.Get(currentResultVar)
			return cr
		}
		return newError(node, "operand for S/R must be a boolean variable")

	case "ADD", "SUB", "MUL", "DIV", "AND", "OR", "XOR", "GT", "LT", "EQ", "NE", "GE", "LE":
		crObj, ok := env.Get(currentResultVar)
		if !ok {
			return newError(node, "%s instruction executed but Current Result is not set", op)
		}
		operand := evalOperand(node.Operand, env)
		if isError(operand) {
			return operand
		}
		operand = applyNegationToOperand(operand, modifier, isConditional, node)
		if isError(operand) {
			return operand
		}

		goOp := op
		switch strings.ToUpper(op) {
		case "ADD":
			goOp = "+"
		case "SUB":
			goOp = "-"
		case "MUL":
			goOp = "*"
		case "DIV":
			goOp = "/"
		case "GT":
			goOp = ">"
		case "LT":
			goOp = "<"
		case "EQ":
			goOp = "="
		case "NE":
			goOp = "<>"
		case "GE":
			goOp = ">="
		case "LE":
			goOp = "<="
		}
		result := object.EvalInfix(crObj, goOp, operand)

		if isError(result) {
			return result
		}

		// The loop now sets the CR from the return value.
		return result

	case "CAL":
		isConditionalCall := strings.Contains(modifier, "C")
		if isConditionalCall {
			crObj, ok := env.Get(currentResultVar)
			crIsTruthy := ok && isTruthy(crObj)
			isNegated := strings.Contains(modifier, "N")

			shouldExecute := false
			if isNegated { // CALCN
				shouldExecute = !crIsTruthy
			} else { // CALC
				shouldExecute = crIsTruthy
			}

			if !shouldExecute {
				// If we skip, the instruction does nothing, and the CR remains unchanged.
				cr, _ := env.Get(currentResultVar)
				return cr
			}
		}

		// If we are here, it's an unconditional CAL or a conditional one that should execute.
		// The result of the function call becomes the new current result.
		result := evalOperand(node.Operand, env)
		if isError(result) {
			return result
		}
		// The loop now sets the CR from the return value.
		// The instruction's result is the new value of the accumulator, which the loop will handle.
		return result

	case "RET":
		return &object.Return{}

	case "JMP":
		isConditional := strings.Contains(modifier, "C")
		if !isConditional {
			// Unconditional JMP
			if operandIdent, ok := node.Operand.(*ast.Identifier); ok {
				return &object.Jump{TargetLabel: operandIdent.Value}
			} else {
				return newError(node, "operand for JMP must be a label identifier")
			}
		} else {

			// Conditional JMP (JMPC, JMPCN)
			crObj, ok := env.Get(currentResultVar)
			crIsTruthy := ok && isTruthy(crObj)
			isNegated := strings.Contains(modifier, "N")

			var shouldJump bool
			if isNegated {
				// This is JMPCN, jump if CR is false.
				shouldJump = !crIsTruthy
			} else {
				// This is JMPC, jump if CR is true.
				shouldJump = crIsTruthy
			}

			if shouldJump {
				if operandIdent, ok := node.Operand.(*ast.Identifier); ok {
					return &object.Jump{TargetLabel: operandIdent.Value}
				} else {
					return newError(node, "operand for JMPC%s must be a label identifier", modifier)
				}
			}
		}
		// // Condition not met, do not jump.
		cr, _ := env.Get(currentResultVar)
		return cr

	default:
		return newError(node, "unknown IL operator: %s", op)
	}
}

// evalOperand is a helper to evaluate an IL instruction's operand, handling
// standard expressions and parenthesized IL sub-programs.
func evalOperand(operandNode ast.Expression, env *object.Environment) object.Object {
	if operandNode == nil {
		return newError(operandNode, "instruction requires an operand")
	}
	if block, ok := operandNode.(*ast.BlockStatement); ok {
		blockEnv := object.NewEnclosedEnvironment(env)
		evalIlProgram(block.Statements, blockEnv)
		operand, ok := blockEnv.Get(currentResultVar)
		if !ok {
			return newError(operandNode, "parenthesized IL expression did not produce a result")
		}
		return operand
	}
	return Eval(operandNode, env)
}

// applyNegationToOperand is a helper to apply the 'N' modifier to an operand if present.
func applyNegationToOperand(operand object.Object, modifier string, isConditional bool, node ast.Node) object.Object {
	isNegatedOperand := strings.Contains(modifier, "N") && !isConditional
	if isNegatedOperand {
		// Create a synthetic prefix expression to reuse the NOT logic.
		// It's important to pass the token from the original IL instruction
		// so that any errors generated have the correct line number.
		ilStmt := node.(*ast.IlInstructionStatement)
		syntheticNotExpr := &ast.PrefixExpression{Token: ilStmt.Token, Operator: "NOT", Right: ilStmt.Operand}

		return evalNotOperatorExpression(syntheticNotExpr, operand)
	}
	return operand
}

// evalAssignmentStatement evaluates an assignment by first evaluating the right-hand
// side value, and then setting it in the environment for the left-hand side identifier.
func evalAssignmentStatement(node *ast.AssignmentStatement, env *object.Environment) object.Object {
	val := Eval(node.Value, env) // Evaluate the right side
	if isError(val) {
		return val
	}

	if ident, ok := node.Left.(*ast.Identifier); ok {
		// When assigning, we must check if the target is a pointer-like object
		// (for VAR_IN_OUT or VAR ... AT) to update the underlying value correctly.
		// We use Get to find the variable, which may be in an outer scope (e.g.,
		// when executing a POU body in an enclosed environment).
		if existing, ok := env.Get(ident.Value); ok {
			switch v := existing.(type) {
			case *object.Constant:
				// It's a constant. Assignment is not allowed.
				return newError(node, "cannot assign to constant variable '%s'", ident.Value)

			case *object.Pointer:
				if v.Env == nil { // This signifies a located variable.
					// It's a located variable. The assignment updates the I/O map.
					ioMap[v.Name] = val // v.Name holds the address string.
				} else {
					// It's a regular VAR_IN_OUT pointer.
					v.Env.Assign(v.Name, val)
				}
				return val
			}
		}
		// It's not a pointer in the local scope, so it's a regular assignment.
		// Use Assign to update the variable in the current or an outer scope.
		env.Assign(ident.Value, val)

	} else {
		return newError(node, "assignment target must be an identifier")
	}
	return val // Assignment statements evaluate to the assigned value.
}

// evalBlockStatement evaluates a block of statements sequentially. It returns the
// value of the last statement, or propagates a ReturnValue or Error immediately.
func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		// If a statement returns a value that should halt execution (like a RETURN or an ERROR),
		// we must propagate it up immediately without executing subsequent statements.
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

// evalVarBlockStatement evaluates a `VAR ... END_VAR` block by evaluating each declaration within it.
func evalVarBlockStatement(block *ast.VarBlockDeclaration, env *object.Environment) object.Object {
	for _, decl := range block.Declarations {
		if err := Eval(decl, env); isError(err) {
			return err
		}
	}
	return NULL
}

// evalGenericVarBlock is a helper to evaluate any block of variable declarations.
func evalGenericVarBlock(decls []*ast.VarDeclStatement, env *object.Environment) object.Object {
	for _, decl := range decls {
		if err := Eval(decl, env); isError(err) {
			return err // cspell:disable-line
		}
	}
	return NULL
}

// evalVarDeclStatement handles a single variable declaration. If an initial value
// is provided, it's evaluated and set. If the type is a function block, a new
// instance of that FB is created and stored.
func evalVarDeclStatement(node *ast.VarDeclStatement, env *object.Environment) object.Object {
	// Handle located variables (AT %) first.
	if node.Location != nil {
		address := node.Location.Location.String()
		// Create a pointer-like object that holds the I/O address.
		// Use a Pointer with a nil Env to signify a located variable.
		locatedObj := &object.Pointer{Name: address, Env: nil}
		env.Set(node.Name.Value, locatedObj)

		// Set the initial value in the shared I/O map.
		if node.Value != nil {
			val := Eval(node.Value, env)
			if isError(val) {
				return val
			}
			ioMap[address] = val
		} else if _, ok := ioMap[address]; !ok {
			// If no initial value is given and the address isn't already in the map,
			// initialize it to NULL.
			ioMap[address] = NULL
		}
		return locatedObj
	}
	var val object.Object
	if node.Value != nil {
		val = Eval(node.Value, env)
		if isError(val) {
			return val
		}
		// Convert the initial value to the declared type, if necessary.
		if typeSpec, ok := node.DataType.(*ast.TypeSpecifier); ok {
			targetType := strings.ToUpper(typeSpec.TokenLiteral())
			// Avoid converting if types are already the same, or if it's not a basic type conversion.
			// This is a simple heuristic to avoid errors with complex types like structs.
			if string(val.Type()) != targetType && (object.IsIntegerType(targetType) || object.IsRealType(targetType) || object.IsBooleanType(targetType)) {
				convertedVal := object.ApplyConversion(val, string(val.Type()), targetType)
				if isError(convertedVal) {
					return convertedVal
				}
				val = convertedVal
			}
		}
	} else {
		// No initial value provided in the declaration. Check for user-defined types or FB instantiation.
		if typeSpec, ok := node.DataType.(*ast.TypeSpecifier); ok {
			typeName := typeSpec.TokenLiteral()
			// 1. Check if it's a user-defined type with a default value.
			if typeQuote, ok := env.Get("_type_" + typeName); ok {
				if quote, isQuote := typeQuote.(*object.Quote); isQuote {
					if typeDecl, isTypeDecl := quote.Node.(*ast.TypeDeclaration); isTypeDecl {
						if typeDecl.InitialValue != nil {
							val = Eval(typeDecl.InitialValue, env)
						}
					}
				}
			}

			// 2. If no value was inherited, check for FB instantiation.
			if val == nil {
				// Only attempt to evaluate the type name as a potential function block
				// if it's not a known primitive type. For primitive types, we assign
				// their default zero value.
				upperTypeName := strings.ToUpper(typeName)

				if object.IsIntegerType(upperTypeName) {
					val = &object.LInt{Value: 0}
				} else if object.IsRealType(upperTypeName) {
					val = &object.LReal{Value: 0.0}
				} else if object.IsBooleanType(upperTypeName) {
					val = FALSE
				} else if object.IsStringType(upperTypeName) {
					if upperTypeName == "STRING" {
						val = &object.String{Value: ""}
					} else { // WSTRING
						val = &object.WString{Value: ""}
					}
				} else if object.IsBitStringType(upperTypeName) {
					width, ok := object.GetBitStringWidth(upperTypeName)
					if ok {
						val = &object.BitString{Value: 0, Width: width}
					}
				} else if object.IsTimeDateKeyword(upperTypeName) {
					switch upperTypeName {
					case "TIME", "T":
						val = &object.Time{Value: 0}
					case "DATE", "D":
						val = &object.Date{Value: time.Time{}}
					case "TIME_OF_DAY", "TOD":
						val = &object.TimeOfDay{Value: time.Time{}}
					case "DATE_AND_TIME", "DT":
						val = &object.DateAndTime{Value: time.Time{}}
					}
				} else {
					// It's not a primitive, so it could be an FB instance.
					typeIdentifier := &ast.Identifier{Token: typeSpec.Token, Value: typeName}
					typeObj := Eval(typeIdentifier, env)
					if isError(typeObj) {
						return typeObj
					}

					switch typeDef := typeObj.(type) {
					case *object.FunctionBlock:
						instanceEnv := object.NewEnclosedEnvironment(typeDef.Env)
						val = &object.FunctionBlockInstance{Definition: typeDef, Env: instanceEnv}
						for _, varDecl := range typeDef.VarInputs {
							if err := evalVarDeclStatement(varDecl, instanceEnv); isError(err) {
								return err
							}
						}
						for _, varDecl := range typeDef.VarOutputs {
							if err := evalVarDeclStatement(varDecl, instanceEnv); isError(err) {
								return err
							}
						}
						for _, varDecl := range typeDef.VarInOuts {
							if err := evalVarDeclStatement(varDecl, instanceEnv); isError(err) {
								return err
							}
						}
						for _, varDecl := range typeDef.Vars {
							if err := evalVarDeclStatement(varDecl, instanceEnv); isError(err) {
								return err
							}
						}
						if sfcAST, isSFC := typeDef.Body.(*ast.SFCProgram); isSFC {
							sfcObj := evalSFCProgram(sfcAST, instanceEnv)
							instanceEnv.Set("__sfc_instance__", sfcObj)
						}
					case *object.BuiltinFunctionBlock:
						instanceEnv := object.NewEnclosedEnvironment(env)
						instanceEnv.Set("__fb_logic__", typeDef)
						val = &object.FunctionBlockInstance{Definition: nil, Env: instanceEnv}
					}
				}
			}
		}
	}
	if node.IsConstant {
		env.Set(node.Name.Value, &object.Constant{Value: val})
	} else {
		env.Set(node.Name.Value, val)
	}
	return val
}

// // evalTypeBlockDeclaration evaluates a `TYPE ... END_TYPE` block, creating runtime
// // objects for user-defined types like ENUMs and subranges and storing them in the environment.
// func evalTypeBlockDeclaration(block *ast.TypeBlockDeclaration, env *object.Environment) object.Object {
// 	for _, decl := range block.Declarations {
// 		// We are interested in enumerated type declarations here.
// 		// The parser creates an EnumDefinition for `(VAL1, VAL2, ...)`
// 		if enumDef, ok := decl.DataType.(*ast.EnumDefinition); ok {
// 			// Create an EnumeratedType object
// 			enumType := &object.EnumeratedType{
// 				Name:   decl.Name.Value,
// 				Values: make(map[string]*object.EnumeratedValue),
// 			}

// 			// Populate the values
// 			for _, valIdent := range enumDef.Values {
// 				enumValue := &object.EnumeratedValue{
// 					TypeName: decl.Name.Value,
// 					Value:    valIdent.Value,
// 				}
// 				enumType.Values[valIdent.Value] = enumValue
// 			}
// 			env.Set(decl.Name.Value, enumType)
// 		} else if subrange, ok := decl.Subrange.(*ast.InfixExpression); ok && subrange.Operator == ".." {
// 			// Validate that the base type is an integer type before evaluating bounds.
// 			baseTypeStr := decl.DataType.String()
// 			if !object.IsIntegerType(baseTypeStr) {
// 				return newError(decl, "subrange base type must be an integer type, got %s", baseTypeStr)
// 			}

// 			lower := Eval(subrange.Left, env)
// 			if isError(lower) {
// 				return lower
// 			}
// 			upper := Eval(subrange.Right, env)
// 			if isError(upper) {
// 				return upper
// 			}
// 			lowerIntVal, _, okL := object.GetIntegerObjectValue(lower)
// 			upperIntVal, _, okU := object.GetIntegerObjectValue(upper)
// 			if !okL || !okU {
// 				return newError(decl, "subrange bounds must be integers, got %s and %s", lower.Type(), upper.Type())
// 			}
// 			enumType := &object.SubrangeType{
// 				Name:       decl.Name.Value,
// 				BaseType:   object.ObjectType(strings.ToUpper(baseTypeStr)),
// 				LowerBound: lowerIntVal,
// 				UpperBound: upperIntVal,
// 			}
// 			env.Set(decl.Name.Value, enumType)
// 		}
// 	}
// 	return NULL // Type declarations don't produce a value themselves.
// }

// applyTimeDateConversion parses a string value for a time or date literal and
// creates the corresponding runtime object (Time, Date, etc.).
func applyTimeDateConversion(value, typeName string) object.Object {
	upperType := strings.ToUpper(typeName)
	switch upperType { // cspell:disable-line
	case "TIME", "T":
		duration, err := parseDuration(value)
		if err != nil {
			return object.NewBuiltinError("could not parse TIME literal: %s", err)
		}
		return &object.Time{Value: duration}
	case "DATE", "D":
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return object.NewBuiltinError("could not parse DATE literal: %s", err)
		}
		return &object.Date{Value: t}
	case "TIME_OF_DAY", "TOD":
		t, err := time.Parse("15:04:05.999999999", value)
		if err != nil {
			// Try without fractional part
			t, err = time.Parse("15:04:05", value)
		}
		if err != nil {
			return object.NewBuiltinError("could not parse TIME_OF_DAY literal: %s", err)
		}
		return &object.TimeOfDay{Value: t}
	case "DATE_AND_TIME", "DT":
		layoutWithFraction := "2006-01-02-15:04:05.999999999"
		layoutWithoutFraction := "2006-01-02-15:04:05"

		t, err := time.Parse(layoutWithFraction, value)
		if err != nil {
			t, err = time.Parse(layoutWithoutFraction, value)
		}
		if err != nil {
			return object.NewBuiltinError("could not parse DATE_AND_TIME literal: %s", err)
		}
		return &object.DateAndTime{Value: t}
	}
	return object.NewBuiltinError("unknown time/date type: %s", typeName)
}

// applyBitStringConversion parses a string value for a bit-string literal (e.g.,
// `BYTE#16#FF`) and creates a `BitString` object, performing range checking
// based on the specified width.
func applyBitStringConversion(value, typeName string) object.Object {
	width, ok := object.GetBitStringWidth(typeName)
	if !ok {
		return object.NewBuiltinError("unknown bit-string type: %s", typeName)
	}

	base := 10 // Default to decimal
	valueStr := value
	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		if len(parts) == 2 {
			parsedBase, err := strconv.Atoi(parts[0])
			if err == nil && (parsedBase == 2 || parsedBase == 8 || parsedBase == 10 || parsedBase == 16) {
				base = parsedBase
				valueStr = parts[1]
			}
		}
	}

	valueStr = strings.ReplaceAll(valueStr, "_", "")

	val, err := strconv.ParseUint(valueStr, base, width)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			return object.NewBuiltinError("value %s is out of range for type %s", valueStr, typeName)
		}
		return object.NewBuiltinError("could not parse %q as %s (base %d): %v", valueStr, typeName, base, err)
	}

	return &object.BitString{Value: val, Width: width}
}

// parseDuration parses an IEC 61131-3 duration string (e.g., "1d_12h_30m_5s_10ms"),
// which can include underscores and multiple units, into a standard Go `time.Duration`.
// into a time.Duration. This is a simplified implementation.
func parseDuration(s string) (time.Duration, error) {
	originalString := s
	isNegative := false
	if strings.HasPrefix(s, "-") {
		isNegative = true
		s = s[1:]
	}

	// Per the standard, underscores are for readability and can be ignored.
	s = strings.ReplaceAll(s, "_", "")
	// Work with lowercase for unit matching.
	s = strings.ToLower(s)

	if s == "" {
		// An empty string after the prefix (e.g., T#) is not a valid duration.
		return 0, fmt.Errorf("invalid duration string: empty")
	}

	totalDuration := time.Duration(0)
	remaining := s

	// This regex will find all number-unit pairs.
	// It finds a number (int or float) followed by letters.
	re := regexp.MustCompile(`(\d*\.?\d+)([a-z]+)`)
	matches := re.FindAllStringSubmatch(remaining, -1)

	if len(matches) == 0 && remaining != "" {
		return 0, fmt.Errorf("invalid duration format in %q", originalString)
	}

	parsedStr := ""
	for _, match := range matches {
		numPart := match[1]
		unitPart := match[2]
		parsedStr += numPart + unitPart

		val, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number %q in duration string %q", numPart, originalString)
		}

		var unitDuration time.Duration
		switch unitPart {
		case "d":
			unitDuration = 24 * time.Hour
		case "h":
			unitDuration = time.Hour
		case "m":
			unitDuration = time.Minute
		case "s":
			unitDuration = time.Second
		case "ms":
			unitDuration = time.Millisecond
		case "us":
			unitDuration = time.Microsecond
		case "ns":
			unitDuration = time.Nanosecond
		default:
			return 0, fmt.Errorf("unknown duration unit %q in string %q", unitPart, originalString)
		}
		totalDuration += time.Duration(val * float64(unitDuration))
	}

	// Check if the entire string was parsed by the regex.
	if parsedStr != s {
		return 0, fmt.Errorf("unparsed characters in duration string %q", originalString)
	}

	if isNegative {
		totalDuration = -totalDuration
	}
	return totalDuration, nil
}

// nativeBoolToBooleanObject returns one of the singleton TRUE or FALSE objects.
func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

// getResultIntegerType determines the result type for an integer infix operation
// based on the standard IEC 61131-3 type promotion rules (e.g., INT + DINT results
// in a DINT).
func getResultIntegerType(t1, t2 object.ObjectType) object.ObjectType {
	// IEC 61131-3 Type Promotion Rules for Integer Arithmetic.
	// The goal is to find the smallest type that can safely hold the result.
	// If types are the same, the result is of the same type.
	if t1 == t2 {
		return t1
	}

	// Define ranks and signedness for each integer type.
	typeInfo := map[object.ObjectType]struct {
		rank     int
		isSigned bool
	}{
		object.SINT_OBJ:  {1, true},
		object.USINT_OBJ: {1, false},
		object.INT_OBJ:   {2, true},
		object.UINT_OBJ:  {2, false},
		object.DINT_OBJ:  {3, true},
		object.UDINT_OBJ: {3, false},
		object.LINT_OBJ:  {4, true},
		object.ULINT_OBJ: {4, false},
	}

	info1, ok1 := typeInfo[t1]
	info2, ok2 := typeInfo[t2]

	// If one of the types is not a standard integer type, fallback to the other.
	if !ok1 {
		return t2
	}
	if !ok2 {
		return t1
	}

	// If ranks are the same but signedness is different, promote to the next larger signed type.
	if info1.rank == info2.rank && info1.isSigned != info2.isSigned {
		switch info1.rank {
		case 1: // SINT vs USINT -> INT
			return object.INT_OBJ
		case 2: // INT vs UINT -> DINT
			return object.DINT_OBJ
		case 3: // DINT vs UDINT -> LINT
			return object.LINT_OBJ
		case 4: // LINT vs ULINT -> LINT (cannot promote further, LINT is largest signed)
			return object.LINT_OBJ
		}
	}

	// Otherwise, promote to the type with the higher rank.
	if info1.rank > info2.rank {
		return t1
	}
	return t2
}

// abs is a helper function to get the absolute value of an int64.
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// evalCaseStatement evaluates a CASE statement by first evaluating the selector,
// then iterating through each case branch to find a match. It handles single
// values, lists of values, and ranges.
func evalCaseStatement(cs *ast.CaseStatement, env *object.Environment) object.Object {
	selector := Eval(cs.Expression, env)
	if isError(selector) {
		return selector
	}

	for _, branch := range cs.Cases {
		for _, valueNode := range branch.Values {
			matches, err := isCaseMatch(selector, valueNode, env) // Pass the correct environment
			if err != nil {
				return err // Propagate errors from case value evaluation
			}
			if matches {
				consequenceResult := Eval(branch.Consequence, env)
				if isError(consequenceResult) {
					return consequenceResult
				}
				return consequenceResult
			}
		}
	}

	if cs.Alternative != nil {
		return Eval(cs.Alternative, env)
	}

	return NULL
}

// isCaseMatch checks if a selector object matches a case value, which can be a
// single value, a subrange (e.g., 5..10), or a user-defined subrange type.
// It handles type promotion for numeric comparisons.
func isCaseMatch(selector object.Object, valueNode ast.Expression, env *object.Environment) (bool, *object.Error) {
	// Handle ranges, e.g., 5..10
	if infix, ok := valueNode.(*ast.InfixExpression); ok && infix.Operator == ".." {
		lowerBound := Eval(infix.Left, env)
		if isError(lowerBound) {
			return false, lowerBound.(*object.Error)
		}
		upperBound := Eval(infix.Right, env)
		if isError(upperBound) {
			return false, upperBound.(*object.Error)
		}
		// If all are numeric, use numeric comparison to handle type promotion (e.g., INT vs REAL).
		if object.IsNumeric(selector) && object.IsNumeric(lowerBound) && object.IsNumeric(upperBound) {
			ge := object.EvalNumericInfix(selector, lowerBound, ">=")
			if err, isErr := ge.(*object.Error); isErr {
				return false, err
			}

			le := object.EvalNumericInfix(selector, upperBound, "<=")
			if err, isErr := le.(*object.Error); isErr {
				return false, err
			}
			return ge == object.TRUE && le == object.TRUE, nil
		}

		// Fallback to generic comparison for non-numeric types.
		ge := object.EvalInfix(selector, ">=", lowerBound)
		le := object.EvalInfix(selector, "<=", upperBound)

		return ge == object.TRUE && le == object.TRUE, nil
	}

	// For single values, first evaluate the AST node to get the runtime object.
	caseValue := Eval(valueNode, env)
	if isError(caseValue) {
		return false, caseValue.(*object.Error)
	}

	// Handle subrange types as case labels
	if subrange, ok := caseValue.(*object.SubrangeType); ok {
		selectorInt, ok := selector.(*object.LInt)
		if !ok {
			// If selector is not an integer, it can't match a subrange.
			// This isn't an error, just not a match.
			return false, nil
		}
		match := selectorInt.Value >= subrange.LowerBound && selectorInt.Value <= subrange.UpperBound
		return match, nil
	}

	// For all single values, use the generic infix evaluation for equality.
	eq := object.EvalInfix(selector, "=", caseValue)
	if err, isErr := eq.(*object.Error); isErr {
		return false, err
	}
	return eq == object.TRUE, nil
}

// evalForLoopStatement evaluates a FOR loop. It creates a new enclosed environment
// for the loop control variable and iterates from the start to the end value.
func evalForLoopStatement(fls *ast.ForLoopStatement, env *object.Environment) object.Object {
	// Create an enclosed environment for the loop to isolate the control variable `i`.
	// This prevents the control variable from leaking into the outer scope.
	loopEnv := object.NewEnclosedEnvironment(env)

	// 1. Evaluate the initial assignment of the control variable in the loop's environment.
	initVal := Eval(fls.ControlVar.Value, loopEnv)
	if isError(initVal) {
		return initVal
	}
	initIntVal, _, ok := object.GetIntegerObjectValue(initVal)
	if !ok {
		return newError(fls.ControlVar, "FOR loop start value must be an integer, got %s", initVal.Type())
	}
	controlVarName := fls.ControlVar.Left.(*ast.Identifier).Value
	loopEnv.Set(controlVarName, &object.LInt{Value: initIntVal})

	// 2. Evaluate the end value in the loop's environment.
	endValObj := Eval(fls.EndValue, loopEnv)
	if isError(endValObj) {
		return endValObj
	}
	endVal, _, ok := object.GetIntegerObjectValue(endValObj)
	if !ok {
		return newError(fls.EndValue, "FOR loop end value must be an integer, got %s", endValObj.Type())
	}

	// 3. Evaluate the step value (or default to 1).  Used by the BY operator
	stepVal := int64(1)
	if fls.StepValue != nil {
		stepValObj := Eval(fls.StepValue, loopEnv)
		if isError(stepValObj) {
			return stepValObj
		}
		stepIntVal, _, ok := object.GetIntegerObjectValue(stepValObj)
		if !ok {
			return newError(fls.StepValue, "FOR loop step value must be an integer, got %s", stepValObj.Type())
		}
		stepVal = stepIntVal
	}

	// 4. Execute the loop.
	for {
		// Get the current value of the control variable.
		currentValObj, _ := loopEnv.Get(controlVarName)
		currentVal := currentValObj.(*object.LInt).Value

		// Check termination condition.
		if stepVal > 0 {
			if currentVal > endVal {
				break
			}
		} else { // stepVal <= 0
			if currentVal < endVal {
				break
			}
		}

		// Evaluate the loop body.
		result := Eval(fls.Body, loopEnv)
		if result != nil {
			if result.Type() == object.ERROR_OBJ || result.Type() == object.RETURN_VALUE_OBJ || result.Type() == object.EXIT_OBJ {
				// If EXIT, stop the loop and return NULL. Otherwise, propagate RETURN/ERROR.
				if result.Type() == object.EXIT_OBJ { // cspell:disable-line
					return NULL
				}
				return result
			}
		}

		// Increment control variable.
		loopEnv.Set(controlVarName, &object.LInt{Value: currentVal + stepVal})
	}

	return NULL
}

// evalWhileStatement evaluates a WHILE loop. It repeatedly checks the condition
// and executes the body until the condition becomes false.
func evalWhileStatement(ws *ast.WhileStatement, env *object.Environment) object.Object {
	for {
		condition := Eval(ws.Condition, env)
		if isError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break // Exit loop if condition is false
		}

		result := Eval(ws.Body, env)
		if result != nil {
			rt := result.Type()
			if rt == object.ERROR_OBJ || rt == object.RETURN_VALUE_OBJ {
				return result
			}
			if rt == object.EXIT_OBJ {
				break
			}
		}
	}
	return NULL
}

// evalRepeatStatement evaluates a REPEAT...UNTIL loop. It executes the body at
// least once, then checks the condition and continues until it becomes true.
func evalRepeatStatement(rs *ast.RepeatStatement, env *object.Environment) object.Object {
	for {
		result := Eval(rs.Body, env)
		if result != nil {
			rt := result.Type()
			if rt == object.ERROR_OBJ || rt == object.RETURN_VALUE_OBJ {
				return result
			}
			if rt == object.EXIT_OBJ {
				break
			}
		}

		condition := Eval(rs.Condition, env)
		if isError(condition) {
			return condition
		}

		if isTruthy(condition) {
			break // Exit loop if UNTIL condition is true
		}
	}

	return NULL
}

// evalWStringInfixExpression handles concatenation for wide strings.
func evalWStringInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.WString).Value
	rightVal := right.(*object.WString).Value
	return &object.WString{Value: leftVal + rightVal}
}

// evalStringInfixExpression handles concatenation for single-byte strings.
func evalStringInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	return &object.String{Value: leftVal + rightVal}
}

// evalTaskDeclaration evaluates a TASK declaration, creating a runtime Task object.
func evalTaskDeclaration(taskDecl *ast.TaskDeclaration, env *object.Environment) object.Object {
	// Evaluate interval and priority expressions.
	var interval time.Duration
	if taskDecl.Interval != nil {
		intervalObj := Eval(taskDecl.Interval, env)
		if isError(intervalObj) {
			return intervalObj
		}
		if t, ok := intervalObj.(*object.Time); ok {
			interval = t.Value
		} else {
			return newError(taskDecl, "task INTERVAL must be of type TIME, got %s", intervalObj.Type())
		}
	}

	var priority int64
	if taskDecl.Priority != nil {
		priorityObj := Eval(taskDecl.Priority, env)
		if isError(priorityObj) {
			return priorityObj
		}
		if p, ok := priorityObj.(*object.LInt); ok {
			priority = p.Value
		} else {
			return newError(taskDecl, "task PRIORITY must be of type INTEGER, got %s", priorityObj.Type())
		}
	}

	task := &object.Task{
		Name:     taskDecl.Name.Value,
		Priority: priority,
		Interval: interval,
		Trigger:  taskDecl.Single, // Store the AST expression for the trigger
	}
	return env.Set(task.Name, task)
}

// evalConfigurationDeclaration evaluates a CONFIGURATION block, setting up the
// environments for its resources, tasks, and program instances.
func evalConfigurationDeclaration(config *ast.ConfigurationDeclaration, env *object.Environment) object.Object {
	// 1. Evaluate Global Vars at the configuration level.
	for _, globalVarBlock := range config.GlobalVars {
		if err := Eval(globalVarBlock, env); isError(err) {
			return err
		}
	}
	// 2. Evaluate each resource, which creates program instances.
	for _, resNode := range config.Resources {
		if err := evalResourceDeclaration(resNode, env); isError(err) {
			return err
		}
	}
	// 3. Evaluate VAR_ACCESS to create global aliases to nested variables.
	for _, accessVarBlock := range config.AccessVars {
		if err := Eval(accessVarBlock, env); isError(err) {
			return err
		}
	}
	// 4. Evaluate VAR_CONFIG to link program variables to hardware addresses. This must be done after resources are created.
	for _, varConfigBlock := range config.VarConfigs {
		if err := Eval(varConfigBlock, env); isError(err) {
			return err
		}
	}
	return NULL
}

// evalAccessVarDeclaration evaluates a VAR_ACCESS block, creating pointers
// (aliases) in the current environment that point to variables elsewhere in the
// configuration.
func evalAccessVarDeclaration(node *ast.AccessVarDeclaration, env *object.Environment) object.Object {
	for _, decl := range node.Vars {
		// decl.Name is the local alias (e.g., BAKER)
		// decl.AccessPath is the path to the target (e.g., STATION_1.P1.x2)

		targetEnv, varName, err := resolveAccessPath(decl.AccessPath, env)
		if err != nil {
			// If the path can't be resolved (e.g., in a unit test without a full
			// configuration), treat it as a placeholder declaration.
			// This aligns with the idea that VAR_ACCESS is for linking, and if the
			// link target isn't present, we just declare the local alias.
			env.Set(decl.Name.Value, NULL) // Declare it as NULL initially.
			continue
		}

		// Check if the target variable exists in the resolved environment.
		if _, ok := targetEnv.Get(varName); !ok {
			// In a full configuration, this is an error. But for placeholder
			// behavior, we can also just declare the local alias.
			env.Set(decl.Name.Value, NULL)
			continue
		}

		// Create a pointer to the target variable.
		ptr := &object.Pointer{Name: varName, Env: targetEnv}

		// Set the local alias in the current (configuration) environment to be this pointer.
		env.Set(decl.Name.Value, ptr)
	}
	return NULL
}

// evalResourceDeclaration evaluates a RESOURCE block within a configuration,
// (aliases) in the current environment that point to variables elsewhere in the
// configuration.
// func evalAccessVarDeclaration(node *ast.AccessVarDeclaration, env *object.Environment) object.Object {
// 	for _, decl := range node.Vars {
// 		// decl.Name is the local alias (e.g., BAKER)
// 		// decl.AccessPath is the path to the target (e.g., STATION_1.P1.x2)

// 		targetEnv, varName, err := resolveAccessPath(decl.AccessPath, env)
// 		if err != nil {
// 			// If the path can't be resolved (e.g., in a unit test without a full
// 			// configuration), treat it as a placeholder declaration.
// 			// This aligns with the idea that VAR_ACCESS is for linking, and if the
// 			// link target isn't present, we just declare the local alias.
// 			env.Set(decl.Name.Value, NULL) // Declare it as NULL initially.
// 			continue
// 		}

// 		// Check if the target variable exists in the resolved environment.
// 		if _, ok := targetEnv.Get(varName); !ok {
// 			// In a full configuration, this is an error. But for placeholder
// 			// behavior, we can also just declare the local alias.
// 			env.Set(decl.Name.Value, NULL)
// 			continue
// 		}

// 		// Create a pointer to the target variable.
// 		ptr := &object.Pointer{Name: varName, Env: targetEnv}

// 		// Set the local alias in the current (configuration) environment to be this pointer.
// 		env.Set(decl.Name.Value, ptr)
// 	}
// 	return NULL
// }

// evalResourceDeclaration evaluates a RESOURCE block within a configuration,
// setting up the environment for its tasks and program instances.
func evalResourceDeclaration(res *ast.ResourceDeclaration, parentEnv *object.Environment) object.Object {
	// Each resource has its own scope, which encloses the parent (configuration) scope.
	resourceEnv := object.NewEnclosedEnvironment(parentEnv)

	// Evaluate resource-scoped global variables.
	for _, globalVarBlock := range res.GlobalVars {
		if err := Eval(globalVarBlock, resourceEnv); isError(err) {
			return err
		}
	}

	// Evaluate task declarations within the resource.
	for _, taskDecl := range res.Tasks {
		if err := Eval(taskDecl, resourceEnv); isError(err) {
			return err
		}
	}

	// Evaluate program instances within the resource.
	for _, progConfig := range res.Programs {
		if err := evalProgramConfiguration(progConfig, resourceEnv); isError(err) {
			return err
		}
	}

	// To store the resource's environment, we wrap it in an object that implements
	// the object.Object interface. A FunctionBlockInstance is a suitable container.
	resourceInstance := &object.FunctionBlockInstance{
		Env: resourceEnv,
	}

	parentEnv.Set(res.Name.Value, resourceInstance)

	return NULL
}

// evalVarConfigDeclaration evaluates a VAR_CONFIG block, linking unlocated
// variables within program instances to specific hardware addresses.
func evalVarConfigDeclaration(config *ast.ConfigVarDeclaration, env *object.Environment) object.Object {
	for _, decl := range config.Declarations {
		targetEnv, varName, err := resolveAccessPath(decl.AccessPath, env)
		if err != nil {
			return err
		}

		// The variable should exist in the target environment, but might be uninitialized (nil)
		if _, ok := targetEnv.Get(varName); !ok {
			return newError(decl, "variable '%s' in VAR_CONFIG path not found in instance", varName)
		}

		address := decl.Location.Location.String()
		locatedObj := &object.Pointer{Name: address, Env: nil} // Pointer to I/O map

		// Replace the variable in the target environment with the I/O pointer
		targetEnv.Set(varName, locatedObj)

		// Initialize the I/O map if not present
		if _, ok := ioMap[address]; !ok {
			ioMap[address] = NULL
		}
	}
	return NULL
}

// resolveAccessPath traverses an AST member access path (e.g., `Res1.P1.Sensor`)
// to find the environment of the final instance and the name of the member variable.
func resolveAccessPath(node ast.Expression, startEnv *object.Environment) (*object.Environment, string, *object.Error) {
	pathParts := []string{}
	curr := node
	for {
		member, ok := curr.(*ast.MemberAccessExpression)
		if !ok {
			ident, ok := curr.(*ast.Identifier)
			if !ok {
				return nil, "", newError(node, "invalid access path in VAR_CONFIG: must be a chain of identifiers")
			}
			pathParts = append(pathParts, ident.Value)
			break
		}
		pathParts = append(pathParts, member.Member.Value)
		curr = member.Struct
	}

	// Reverse the path parts to get the correct order (e.g., from ["c", "b", "a"] to ["a", "b", "c"])
	for i, j := 0, len(pathParts)-1; i < j; i, j = i+1, j-1 {
		pathParts[i], pathParts[j] = pathParts[j], pathParts[i]
	}

	finalVarName := pathParts[len(pathParts)-1]
	pathPrefix := pathParts[:len(pathParts)-1]

	currentEnv := startEnv
	for _, part := range pathPrefix {
		obj, ok := currentEnv.Get(part)
		if !ok {
			return nil, "", newError(node, "name '%s' not found in VAR_CONFIG path", part)
		}

		// The object must be an instance with its own environment.
		switch instance := obj.(type) {
		case *object.FunctionBlockInstance: // For resources
			currentEnv = instance.Env
		case *object.ProgramInstance:
			currentEnv = instance.Env
		default:
			return nil, "", newError(node, "path element '%s' is not a configurable instance (got %s)", part, obj.Type())
		}
	}

	return currentEnv, finalVarName, nil
}

// evalProgramConfiguration evaluates a program instance declaration within a
// resource, creating a ProgramInstance object with its own environment and applying configured parameters.
func evalProgramConfiguration(progConfig *ast.ProgramConfiguration, resourceEnv *object.Environment) object.Object {
	// 1. Find the program's definition (the template).
	progDefObj, ok := resourceEnv.Get(progConfig.TypeName.Value)
	if !ok {
		return newError(progConfig, "program type '%s' not defined", progConfig.TypeName.Value)
	}
	progDef, ok := progDefObj.(*object.Program)
	if !ok {
		return newError(progConfig, "'%s' is not a PROGRAM", progConfig.TypeName.Value)
	}

	// 2. Create a new instance with its own environment.
	instanceEnv := object.NewEnclosedEnvironment(progDef.Env)
	progInstance := &object.ProgramInstance{
		Definition: progDef,
		Env:        instanceEnv,
	}
	// Store the task name on the instance itself, if it exists.
	if progConfig.TaskName != nil {
		progInstance.TaskName = progConfig.TaskName.Value
	}

	// 3. Initialize all variables in the instance environment. This ensures they
	// exist before any VAR_CONFIG or parameter assignments are applied.
	allVarBlocks := [][]*ast.VarDeclStatement{
		progDef.VarInputs,
		progDef.VarOutputs,
		progDef.VarInOuts,
		progDef.Vars,
	}
	for _, varBlock := range allVarBlocks {
		for _, varDecl := range varBlock {
			if err := evalVarDeclStatement(varDecl, instanceEnv); isError(err) {
				return err
			}
		}
	}

	// Handle implicit VAR_EXTERNAL resolution.
	// If an external variable was not explicitly mapped via parameters,
	// search for it in the enclosing (resource) environment.
	for _, extVarBlock := range progDef.VarExternal {
		for _, extVar := range extVarBlock.Vars {
			varName := extVar.Name.Value
			// Check if the variable was already set by an explicit parameter.
			if _, alreadySet := instanceEnv.GetRaw(varName); !alreadySet {
				// It was not explicitly set, so try to find it in the resource's scope.
				// The Get method will search up the chain (resource -> config).
				// If found, we create a pointer that starts its search from the resource env.
				if _, ok := resourceEnv.Get(varName); ok {
					ptr := &object.Pointer{Name: varName, Env: resourceEnv}
					instanceEnv.Set(varName, ptr)
				}
			}
		}
	}

	// 4. Apply instance-specific parameters from the configuration.
	for _, param := range progConfig.Parameters {
		if namedArg, ok := param.(*ast.NamedArgument); ok {
			// This is an input assignment: `InputVar := Value`
			val := Eval(namedArg.Value, resourceEnv) // Evaluate value in the resource/config scope
			if isError(val) {
				return val
			}
			// Set the value inside the program instance's environment.
			instanceEnv.Set(namedArg.Name.Value, val)
		} else if outputArg, ok := param.(*ast.OutputArgument); ok {
			// This is an output mapping: `OutputVar => TargetVar`.
			// The target must be an identifier.
			targetIdent, ok := outputArg.Target.(*ast.Identifier)
			if !ok {
				return newError(outputArg, "target of an output mapping '=>' must be a variable identifier")
			}
			mapping := object.OutputMapping{
				SourceParamName: outputArg.Source.Value,
				TargetVarName:   targetIdent.Value,
			}
			progInstance.OutputMappings = append(progInstance.OutputMappings, mapping)
		}
	}

	// 5. Store the fully configured instance in the resource's environment.
	resourceEnv.Set(progConfig.InstanceName.Value, progInstance)
	return progInstance
}

// evalIfStatement evaluates an IF...THEN...ELSIF...ELSE statement by first
// evaluating the condition and then executing the appropriate block.
func evalIfStatement(
	ie *ast.IfStatement,
	env *object.Environment,
) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env) // cspell:disable-line
	} else {
		return NULL
	}
}

// evalIdentifier resolves an identifier in the environment. It checks for local
// variables, outer scope variables, built-in functions, standard function blocks,
// and type conversion functions.
func evalIdentifier(
	node *ast.Identifier,
	env *object.Environment,
) object.Object {
	// Special handling for constant values to unwrap them.
	if val, ok := env.GetRaw(node.Value); ok {
		if constant, isConst := val.(*object.Constant); isConst {
			return constant.Value
		}
	}
	if val, ok := env.Get(node.Value); ok {
		// If the identifier points to a located variable, we must fetch the
		// actual value from the shared I/O map. We check for a Pointer with a nil Env.
		if ptr, isPtr := val.(*object.Pointer); isPtr && ptr.Env == nil {
			address := ptr.Name
			if ioVal, ok := ioMap[address]; ok {
				// The value in ioMap is the final value, don't dereference further.
				return ioVal
			}
			// If the I/O address hasn't been written to yet, return NULL.
			return NULL
		}
		dereferenced := dereferencePointer(node, val)
		// If a variable is in the environment but uninitialized (its value is nil),
		// we should treat it as a NULL object within the evaluator to prevent panics.
		if dereferenced == nil {
			return NULL
		}
		return dereferenced
	}

	// Check for function definitions, which are stored with a prefix.
	if fn, ok := env.Get("_function_" + node.Value); ok {
		return fn
	}

	// --- New logic to resolve SFC step names ---
	// This is a special case to allow accessing step properties like StepName.T
	// It searches the environment for any function block instances that might contain an SFC.
	// This is a simplification; a more robust implementation might use a dedicated symbol table.
	for _, name := range env.Names() {
		if obj, ok := env.Get(name); ok {
			if fbInstance, isFB := obj.(*object.FunctionBlockInstance); isFB {
				if sfcObj, sfcOk := fbInstance.Env.Get("__sfc_instance__"); sfcOk {
					if sfc, isSFC := sfcObj.(*object.SFC); isSFC {
						if step, stepOk := sfc.Steps[node.Value]; stepOk {
							return step // Found the step object
						}
					}
				}
			}
		}
	}
	// Also check if the current environment itself has an SFC (for PROGRAM with SFC body)
	// Check if the identifier is a step name within the current POU's SFC.
	// This allows accessing step properties like `MyStep.T`.
	if sfcObj, sfcOk := env.Get("__sfc_instance__"); sfcOk {
		if sfc, isSFC := sfcObj.(*object.SFC); isSFC {
			if step, stepOk := sfc.Steps[node.Value]; stepOk {
				return step
			}
		}
	}

	if builtin, ok := object.GetBuiltinByName(node.Value); ok {
		return builtin
	}

	// Check for standard function blocks (TON, CTU, etc.)
	if fb, ok := standardFBs[node.Value]; ok {
		// We return the BuiltinFunctionBlock definition. The evaluator will then create
		// an instance when it sees a variable declaration of this type.
		return fb
	}

	// Check for generic type conversion functions like `INT_TO_REAL`
	if strings.Contains(node.Value, "_TO_") {
		parts := strings.Split(node.Value, "_TO_")
		if len(parts) == 2 {
			return object.GenericConversionBuiltin(parts[0], parts[1])
		}
	}

	// Check if the identifier is a typed literal like T#5s or BYTE#16#FF
	if strings.Contains(node.Value, "#") {
		parts := strings.SplitN(node.Value, "#", 2) // cspell:disable-line
		if len(parts) == 2 {
			typeName := parts[0]
			valueStr := parts[1]

			if object.IsTimeDateKeyword(typeName) {
				return applyTimeDateConversion(valueStr, typeName)
			}
			if object.IsBitStringType(typeName) {
				return applyBitStringConversion(valueStr, typeName)
			}
		}
	}

	return newError(node, "identifier not found: %s", node.Value)
}

// dereferencePointer recursively follows a chain of Pointer objects (used for
// VAR_IN_OUT) until it finds the final, non-pointer value. This is essential for
// nested IN_OUT parameter passing.
func dereferencePointer(node ast.Node, obj object.Object) object.Object {
	ptr, isPtr := obj.(*object.Pointer)
	if !isPtr {
		return obj // Not a pointer, return the object as is.
	}

	// It's a pointer, so get the object it points to.
	dereferenced, ok := ptr.Env.Get(ptr.Name)
	if !ok {
		return newError(node, "internal error: dangling pointer for %s", ptr.Name)
	}

	// Recursively dereference in case the target is also a pointer.
	return dereferencePointer(node, dereferenced)
}

// newError creates a new Error object with a formatted message, including line and column numbers from the AST node.
func newError(node ast.Node, format string, a ...interface{}) *object.Error {
	if node != nil {

		line, col := node.Pos()

		return &object.Error{
			Message: fmt.Sprintf("ERROR (%d:%d): %s", line, col, fmt.Sprintf(format, a...)),
		}
	}

	return &object.Error{
		Message: fmt.Sprintf("ERROR: %s", fmt.Sprintf(format, a...)),
	}
}

// isError checks if a given object is an Error object.
func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

// isTruthy checks if an object is considered "truthy" in an IEC 61131-3 context.
// This is used for evaluating conditions in IF, WHILE, JMPC, etc.
// FALSE, NULL, and numeric zero values are considered false. All other values are true.
func isTruthy(obj object.Object) bool {
	if obj == nil {
		return false
	}

	switch o := obj.(type) {
	case *object.Boolean:
		return o.Value
	case *object.Null:
		return false
	// Signed Integer types
	case *object.SInt:
		return o.Value != 0
	case *object.Int:
		return o.Value != 0
	case *object.DInt:
		return o.Value != 0
	case *object.LInt:
		return o.Value != 0
	// Unsigned Integer types
	case *object.USInt:
		return o.Value != 0
	case *object.UInt:
		return o.Value != 0
	case *object.UDInt:
		return o.Value != 0
	case *object.ULInt:
		return o.Value != 0
	// Real types
	case *object.Real:
		return o.Value != 0.0
	case *object.LReal:
		return o.Value != 0.0
	default:
		return true
	}
}

// isKnownType checks if a given type name corresponds to a known built-in IEC
// type or a user-defined type (like an ENUM or STRUCT) present in the environment.
func isKnownType(typeName string, env *object.Environment) bool {
	upper := strings.ToUpper(typeName)
	// Check built-in scalar types
	if object.IsIntegerType(upper) || object.IsRealType(upper) || object.IsBooleanType(upper) || object.IsStringType(upper) || object.IsBitStringType(upper) {
		return true
	}
	// Check built-in time/date types
	if object.IsTimeDateKeyword(upper) {
		return true
	}
	// Check user-defined types (enums, structs) in the environment
	if _, ok := env.Get(upper); ok {
		return true
	}
	// Also check for type definitions which are stored with a prefix
	if _, ok := env.Get("_type_" + upper); ok {
		return true
	}
	return false
}

// evalExpressions evaluates a slice of expressions and returns a slice of the resulting objects.
func evalExpressions(
	exps []ast.Expression,
	env *object.Environment,
) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

// outputArgMapping is an internal struct used during function evaluation to track
// the mapping of a function's output parameter (the source) to a variable in the
// calling scope (the target).
type outputArgMapping struct {
	SourceParamName string         // The name of the VAR_OUTPUT parameter (e.g., "Out1")
	TargetVarNode   ast.Expression // The AST node of the target variable in the calling scope (e.g., "Res1")
}

// applyFunction handles the invocation of all callable objects: user-defined
// functions, function blocks, built-in functions, and standard function blocks.
// It manages environment setup, argument passing (by value and by reference), and return value handling.
func applyFunction(fn object.Object, args []ast.Expression, callEnv *object.Environment, callNode ast.Node) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		// Create a new environment for the function's execution, enclosed by the function's definition environment.
		extendedEnv := object.NewEnclosedEnvironment(fn.Env)
		if fn.Name != nil {
			// Pre-declare the function name as a variable in the local scope.
			// This prevents assignments to the function name (which sets the return value)
			// from overwriting the function definition in the outer scope.
			extendedEnv.Set(fn.Name.Value, NULL)
		}

		_, outputMappings, err := extendFunctionEnv(fn, args, callEnv, extendedEnv, callNode)
		if err != nil {
			return err
		}

		// Initialize VAR and VAR_TEMP variables for this specific call.
		// This ensures statelessness for each function invocation.
		for _, varDecl := range fn.Vars {
			if err := evalVarDeclStatement(varDecl, extendedEnv); isError(err) {
				return err
			}
		}

		// Execute the function body in its new environment.
		evaluated := Eval(fn.Body, extendedEnv)
		if isError(evaluated) {
			return evaluated
		}

		// After execution, handle the output arguments (=>).
		for _, mapping := range outputMappings {
			// Get the final value of the output parameter from the function's scope.
			val, ok := extendedEnv.Get(mapping.SourceParamName)
			if !ok {
				// This should ideally not happen if parsing and declaration are correct.
				return newError(mapping.TargetVarNode, "internal error: output parameter %s not found in function scope", mapping.SourceParamName)
			}
			// Assign this value to the target variable in the *calling* scope.
			// We need to evaluate the target variable node in the calling environment.
			// For now, assuming it's an identifier.
			if targetIdent, ok := mapping.TargetVarNode.(*ast.Identifier); ok {
				callEnv.Set(targetIdent.Value, val)
			} else {
				return newError(mapping.TargetVarNode, "unsupported target for output argument: %T", mapping.TargetVarNode)
			}
		}

		// Handle the primary return value of the function.
		// This is the value assigned to the variable with the same name as the function.
		var returnValue object.Object
		var ok bool

		if fn.Name != nil {
			// For named functions, the return value is the variable with the function's name.
			returnValue, ok = extendedEnv.Get(fn.Name.Value)
		} else {
			// For anonymous functions (FunctionLiteral), the return value is the result of the last statement.
			if body, ok := fn.Body.(*ast.BlockStatement); ok {
				if len(body.Statements) > 0 {
					lastStmt := body.Statements[len(body.Statements)-1]
					returnValue = Eval(lastStmt, extendedEnv)
					if _, isReturn := returnValue.(*object.ReturnValue); isReturn {
						returnValue = unwrapReturnValue(returnValue)
					}
				} else {
					returnValue = NULL
				}
			}
			ok = true // Assume anonymous functions always "return" something, even if NULL
		}
		if !ok {
			// A function must always return a value. If not explicitly set, it's an error or has a default.
			// For simplicity, we'll return NULL, but a stricter implementation might error.
			// With the change above, this will now correctly retrieve the initial NULL value if no assignment was made.
			returnValue, _ = extendedEnv.Get(fn.Name.Value)
		}
		return returnValue

	case *object.Program:
		// Treat a program call like a function call.
		// The program `fn` has its own persistent environment `fn.Env` where static VARs live.
		// We create a new temporary environment for this specific call, enclosing the program's persistent one.
		// This gives access to static VARs but provides a clean scope for VAR_INPUT and VAR_TEMP.
		extendedEnv := object.NewEnclosedEnvironment(fn.Env)

		// Pre-declare the program name as a variable in the local scope for the return value.
		extendedEnv.Set(fn.Name.Value, NULL)

		// Programs can have VAR_INPUT, so we should handle arguments.
		_, outputMappings, err := extendFunctionEnv(fn, args, callEnv, extendedEnv, callNode)
		if err != nil {
			return err
		}

		// Initialize only VAR_TEMP variables for this specific call. Static VARs are already in fn.Env.
		for _, tempBlock := range fn.VarTemp {
			for _, tempVar := range tempBlock.Vars {
				if err := evalVarDeclStatement(tempVar, extendedEnv); isError(err) {
					return err
				}
			}
		}

		// Execute the program body.
		var evaluated object.Object
		isILProgram := false
		// Check for IL first, as it has a special execution context.
		if block, isBlock := fn.Body.(*ast.BlockStatement); isBlock && len(block.Statements) > 0 {
			if _, isIL := block.Statements[0].(*ast.IlInstructionStatement); isIL {
				isILProgram = true
				evaluated = evalIlProgram(block.Statements, extendedEnv)
			}
		}

		if !isILProgram {
			// If not IL, check for SFC.
			if sfcInstanceObj, ok := extendedEnv.Get("__sfc_instance__"); ok {
				sfcInstance, isSFC := sfcInstanceObj.(*object.SFC)
				if !isSFC {
					return newError(nil, "internal error: __sfc_instance__ is not an SFC object")
				}
				evaluated = evalSFCCycle(sfcInstance, extendedEnv)
			} else {
				// For ST programs, evaluate the whole body in the temporary call environment.
				evaluated = Eval(fn.Body, extendedEnv)
			}
		}
		if isError(evaluated) {
			return evaluated
		}

		// Handle output arguments (=>).
		for _, mapping := range outputMappings {
			val, ok := extendedEnv.Get(mapping.SourceParamName)
			if !ok {
				return newError(mapping.TargetVarNode, "internal error: output parameter %s not found in program scope", mapping.SourceParamName)
			}
			if targetIdent, ok := mapping.TargetVarNode.(*ast.Identifier); ok {
				callEnv.Set(targetIdent.Value, val)
			} else {
				return newError(mapping.TargetVarNode, "unsupported target for output argument: %T", mapping.TargetVarNode)
			}
		}

		if isILProgram {
			return evaluated
		}
		// For ST/SFC, the return value is the value assigned to the program's name.
		returnValue, _ := extendedEnv.Get(fn.Name.Value)
		return returnValue

	case *object.ProgramInstance:
		// A program instance is being called.
		// Its environment `fn.Env` is already an enclosed environment that holds its state.
		// We create a new temporary environment for this specific call, enclosing the instance's persistent one.
		extendedEnv := object.NewEnclosedEnvironment(fn.Env)

		// Pre-declare the program name as a variable in the local scope for the return value.
		extendedEnv.Set(fn.Definition.Name.Value, NULL)

		// Handle arguments.
		_, outputMappings, err := extendFunctionEnv(fn.Definition, args, callEnv, extendedEnv, callNode)
		if err != nil {
			return err
		}

		// Initialize only VAR_TEMP variables for this specific call. Static VARs are already in fn.Env.
		for _, tempBlock := range fn.Definition.VarTemp {
			for _, tempVar := range tempBlock.Vars {
				if err := evalVarDeclStatement(tempVar, extendedEnv); isError(err) {
					return err
				}
			}
		}

		// Execute the program body.
		var evaluated object.Object
		isILProgram := false
		// Check for IL first.
		if block, isBlock := fn.Definition.Body.(*ast.BlockStatement); isBlock && len(block.Statements) > 0 {
			if _, isIL := block.Statements[0].(*ast.IlInstructionStatement); isIL {
				isILProgram = true
				evaluated = evalIlProgram(block.Statements, extendedEnv)
			}
		}

		if !isILProgram {
			// If not IL, check for SFC.
			if sfcInstanceObj, ok := extendedEnv.Get("__sfc_instance__"); ok {
				sfcInstance, isSFC := sfcInstanceObj.(*object.SFC)
				if !isSFC {
					return newError(nil, "internal error: __sfc_instance__ is not an SFC object")
				}
				evaluated = evalSFCCycle(sfcInstance, extendedEnv)
			} else {
				// It's an ST program.
				evaluated = Eval(fn.Definition.Body, extendedEnv)
			}
		}
		if isError(evaluated) {
			return evaluated
		}

		// Handle output arguments (=>).
		for _, mapping := range outputMappings {
			val, ok := extendedEnv.Get(mapping.SourceParamName)
			if !ok {
				return newError(mapping.TargetVarNode, "internal error: output parameter %s not found in program scope", mapping.SourceParamName)
			}
			if targetIdent, ok := mapping.TargetVarNode.(*ast.Identifier); ok {
				callEnv.Set(targetIdent.Value, val)
			} else {
				return newError(mapping.TargetVarNode, "unsupported target for output argument: %T", mapping.TargetVarNode)
			}
		}

		if isILProgram {
			return evaluated
		}
		returnValue, _ := extendedEnv.Get(fn.Definition.Name.Value)
		return returnValue

	case *object.BuiltinFunctionBlock:
		// This case is hit when a variable is declared with a standard FB type, e.g., `MyTimer : TON;`
		// We need to create an instance of it.
		instanceEnv := object.NewEnclosedEnvironment(callEnv)
		// The 'Definition' for a built-in FB instance is the BuiltinFunctionBlock object itself.
		// We need a way to link the instance back to its execution logic.
		// A simple way is to store the function pointer in the instance's environment.
		instanceEnv.Set("__fb_logic__", fn)
		// We can reuse FunctionBlockInstance, but the Definition field will be nil.
		// The logic is now self-contained in the instance's environment.
		return &object.FunctionBlockInstance{
			Definition: nil, // Or a placeholder definition if needed
			Env:        instanceEnv,
		}

	case *object.FunctionBlockInstance:
		// For a function block, we must first process the arguments to populate its
		// internal environment and identify output mappings. This must happen before
		// we check EN, because the output mappings need to be processed even if EN is false. // cspell:disable-line
		extendedEnv, outputMappings, err := extendFunctionEnv(fn.Definition, args, callEnv, fn.Env, callNode)
		if err != nil {
			return err
		}

		// Re-initialize VAR_TEMP variables at the start of every scan cycle.
		if fn.Definition != nil {
			for _, tempVarBlock := range fn.Definition.VarTemp {
				for _, tempVar := range tempVarBlock.Vars {
					if err := evalVarDeclStatement(tempVar, extendedEnv); isError(err) {
						return err
					}
				}
			}
		}

		// Check for EN input from the now-populated environment. Defaults to TRUE.
		enValue := TRUE
		if enObj, ok := extendedEnv.Get("EN"); ok {
			if boolVal, isBool := enObj.(*object.Boolean); isBool {
				enValue = boolVal
			}
		}

		// Set ENO to the value of EN by default.
		extendedEnv.Set("ENO", enValue)

		var result object.Object

		// If EN is FALSE, do not execute the function block body.
		if enValue == FALSE {
			result = NULL // No execution, but we still handle output mappings.
		} else {
			// EN is TRUE, execute the block.
			if fn.Definition == nil {
				// Built-in FB
				logicFnObj, _ := extendedEnv.Get("__fb_logic__")
				logicFn := logicFnObj.(*object.BuiltinFunctionBlock).Fn
				result = logicFn(extendedEnv, callEnv)
			} else {
				// User-defined FB
				if sfcInstanceObj, ok := extendedEnv.Get("__sfc_instance__"); ok {
					sfcInstance, isSFC := sfcInstanceObj.(*object.SFC)
					if !isSFC {
						return newError(nil, "internal error: __sfc_instance__ is not an SFC object")
					}
					// When evaluating an SFC inside a function block, it's crucial to evaluate
					// it within the function block's own environment (`extendedEnv`). This ensures
					// that actions within the SFC can access and modify the FB's variables
					// (VAR_INPUT, VAR_OUTPUT, VAR).
					result = evalSFCCycle(sfcInstance, extendedEnv) // Pass the FB's env
				} else {
					result = Eval(fn.Definition.Body, extendedEnv)
				}
			}
		}

		// If the block execution resulted in an error, set ENO to FALSE.
		if isError(result) {
			extendedEnv.Set("ENO", FALSE)
		}

		// Handle all output arguments (=>) after execution (or non-execution).
		// This is crucial for updating the caller's scope, especially for ENO.
		for _, mapping := range outputMappings {
			val, ok := extendedEnv.Get(mapping.SourceParamName)
			if !ok {
				// If the block was disabled, the output might not have been set in this cycle.
				// It should retain its value from the previous cycle, which is already in fn.Env.
				// So, if it's not found, it's a genuine internal error.
				if enValue == TRUE {
					return newError(mapping.TargetVarNode, "internal error: output parameter %s not found in FB scope", mapping.SourceParamName)
				}
				continue // Skip mapping if block is disabled and output was never set.
			}

			switch targetNode := mapping.TargetVarNode.(type) {
			case *ast.Identifier:
				// Use Assign to correctly update variables in outer scopes.
				callEnv.Assign(targetNode.Value, val)

			case *ast.IndexExpression:
				// Handle assignment to an array element, e.g., Out => MyArray[1]
				arrayObj := Eval(targetNode.Left, callEnv)
				if isError(arrayObj) {
					return arrayObj
				}
				array, ok := arrayObj.(*object.Array) // cspell:disable-line
				if !ok {
					return newError(targetNode.Left, "left side of index expression for output argument must be an ARRAY, got %s", arrayObj.Type())
				}

				indexObj := Eval(targetNode.Index, callEnv)
				if isError(indexObj) {
					return indexObj
				}
				idx, _, ok := object.GetIntegerObjectValue(indexObj)
				if !ok {
					return newError(targetNode.Index, "array index for output argument must be an integer, got %s", indexObj.Type())
				}

				if idx < 0 || idx >= int64(len(array.Elements)) {
					return newError(targetNode, "index out of bounds for output assignment: %d", idx)
				}
				array.Elements[idx] = val

			default:
				return newError(mapping.TargetVarNode, "unsupported target for FB output argument: %T", mapping.TargetVarNode)
			}
		}

		return result

	case *object.Builtin:
		// For built-in functions, we evaluate all arguments first.
		evaluatedArgs := evalExpressions(args, callEnv)
		if len(evaluatedArgs) == 1 && isError(evaluatedArgs[0]) {
			return evaluatedArgs[0]
		}
		result := fn.Fn(evaluatedArgs...)
		return result
	default:
		// If the function object itself is an error, it would have been caught earlier.
		// This case is for when `fn` is not a callable object.
		return newError(nil, "not a function: %s", fn.Type()) // Pass nil for node as we don't have it here
	}
}

// extendFunctionEnv prepares the environment for a function or function block call.
// It handles positional and named arguments, creates pointers for VAR_IN_OUT
// parameters, and collects output mappings (`=>`) for later processing.
// with parameters based on the provided arguments.
func extendFunctionEnv(def object.Object, args []ast.Expression, callEnv *object.Environment, targetEnv *object.Environment, callNode ast.Node) (*object.Environment, []outputArgMapping, *object.Error) {
	outputMappings := []outputArgMapping{}
	positionalParamIndex := 0 // Index for positional parameters in paramDecls

	var paramDecls []*ast.VarDeclStatement
	if fbDef, ok := def.(*object.FunctionBlock); ok && fbDef != nil {
		paramDecls = fbDef.VarInputs
	} else if fDef, ok := def.(*object.Function); ok && fDef != nil {
		paramDecls = fDef.VarInputs
	} else if pDef, ok := def.(*object.Program); ok && pDef != nil {
		paramDecls = pDef.VarInputs
	}
	// If def is nil, it's a built-in FB, and we just write to the targetEnv.

	for _, argNode := range args {
		switch arg := argNode.(type) {
		case *ast.NamedArgument: // Handle `InputName := Value`
			paramName := arg.Name.Value
			// Check if this is a VAR_IN_OUT parameter
			if isInOutParam(paramName, def) {
				// The argument must be a variable identifier to be passed by reference.
				if argIdent, ok := arg.Value.(*ast.Identifier); ok {
					// Check if the argument being passed is itself a VAR_IN_OUT from the calling function's perspective.
					// If so, we need to pass the pointer, not the value.
					if val, ok := callEnv.Get(argIdent.Value); ok {
						if ptr, isPtr := val.(*object.Pointer); isPtr {
							// It's a nested IN_OUT pass. Propagate the pointer.
							targetEnv.Set(paramName, ptr)
							continue // Go to next argument
						}
					}
					// It's a direct variable, so create a new pointer to it.
					ptr := &object.Pointer{Name: argIdent.Value, Env: callEnv}
					targetEnv.Set(paramName, ptr)
				} else {
					return nil, nil, newError(arg, "argument for VAR_IN_OUT parameter '%s' must be a variable", paramName)
				}
			} else {
				// It's a VAR_INPUT, so pass by value.
				val := Eval(arg.Value, callEnv)
				if isError(val) {
					return nil, nil, val.(*object.Error)
				}
				targetEnv.Set(paramName, val)
			}

		case *ast.OutputArgument: // Handle `OutputName => TargetVar`
			// The target variable is an AST node (e.g., Identifier), not an evaluated object yet.
			// We store the AST node to resolve it in the calling environment after function execution.
			outputMappings = append(outputMappings, outputArgMapping{
				SourceParamName: arg.Source.Value,
				TargetVarNode:   arg.Target,
			})

		default: // Handle positional arguments (an expression)
			if positionalParamIndex >= len(paramDecls) { // Check against the actual parameter declarations
				return nil, nil, newError(callNode, "too many arguments in function call")
			}
			paramDecl := paramDecls[positionalParamIndex]
			val := Eval(arg, callEnv)
			if isError(val) {
				return nil, nil, val.(*object.Error)
			}

			// Check if the parameter is VAR_IN_OUT
			if isInOutParam(paramDecl.Name.Value, def) {
				// The argument must be a variable identifier to be passed by reference.
				if argIdent, ok := arg.(*ast.Identifier); ok {
					// Check if the argument being passed is itself a VAR_IN_OUT from the calling function's perspective.
					// If so, we need to pass the pointer, not the value.
					if val, ok := callEnv.Get(argIdent.Value); ok {
						if ptr, isPtr := val.(*object.Pointer); isPtr {
							// It's a nested IN_OUT pass. Propagate the pointer.
							targetEnv.Set(paramDecl.Name.Value, ptr)
							positionalParamIndex++
							continue
						}
					}
					// It's a direct variable, so create a new pointer to it.
					ptr := &object.Pointer{Name: argIdent.Value, Env: callEnv}
					targetEnv.Set(paramDecl.Name.Value, ptr)
				} else {
					return nil, nil, newError(arg, "argument for VAR_IN_OUT parameter '%s' must be a variable", paramDecl.Name.Value)
				}
			} else {
				// It's a VAR_INPUT, so pass by value.
				val := Eval(arg, callEnv) // This correctly dereferences pointers for VAR_INPUT.
				if isError(val) {
					return nil, nil, val.(*object.Error)
				}
				targetEnv.Set(paramDecl.Name.Value, val)
			}
			positionalParamIndex++
		}
	}

	return targetEnv, outputMappings, nil
}

// isInOutParam is a helper function that checks if a given parameter name is
// declared as a `VAR_IN_OUT` in a function or function block's definition.
func isInOutParam(paramName string, def object.Object) bool {
	var inOutDecls []*ast.VarDeclStatement
	if fbDef, ok := def.(*object.FunctionBlock); ok && fbDef != nil {
		inOutDecls = fbDef.VarInOuts
	} else if fDef, ok := def.(*object.Function); ok && fDef != nil {
		inOutDecls = fDef.VarInOuts
	} else if pDef, ok := def.(*object.Program); ok && pDef != nil {
		inOutDecls = pDef.VarInOuts
	}

	for _, decl := range inOutDecls {
		if decl.Name.Value == paramName {
			return true
		}
	}
	return false
}

// unwrapReturnValue extracts the underlying object from a ReturnValue wrapper.
func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}

// evalIndexExpression handles the evaluation of an index expression (e.g., `MyArray[i]`).
// It dispatches to specific handlers for arrays and hashes.
func evalIndexExpression(node ast.Node, left, index object.Object) object.Object {
	switch left.Type() {
	case object.ARRAY_OBJ:
		if _, _, ok := object.GetIntegerObjectValue(index); ok {
			return evalArrayIndexExpression(left, index)
		}
		return newError(node, "array index must be an integer, got %s", index.Type())
	case object.HASH_OBJ:
		return evalHashIndexExpression(node, left, index)
	default:
		return newError(node, "index operator not supported: %s", left.Type())
	}
}

// evalArrayIndexExpression performs the bounds check and lookup for an array index operation.
func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx, _, _ := object.GetIntegerObjectValue(index)
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

// evalHashLiteral evaluates a hash literal by evaluating all its key-value
// pairs and creating a new Hash object.
func evalHashLiteral(
	node *ast.HashLiteral,
	env *object.Environment,
) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError(keyNode, "unusable as hash key: %s", key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}

		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: pairs}
}

// evalHashIndexExpression performs the lookup for a hash index operation.
func evalHashIndexExpression(node ast.Node, hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return newError(node, "unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

// evalMemberAccessExpression handles the `.` operator for accessing members of
// runtime objects, such as the outputs of a function block instance (e.g.,
// `MyTimer.Q`) or the properties of an SFC step (e.g., `MyStep.T`).
func evalMemberAccessExpression(node *ast.MemberAccessExpression, env *object.Environment) object.Object {
	left := Eval(node.Struct, env)
	if isError(left) {
		return left
	}

	switch l := left.(type) {
	case *object.FunctionBlockInstance:
		member := node.Member.Value
		val, ok := l.Env.Get(member)
		if !ok {
			return newError(node, "member '%s' not found in function block instance '%s'", member, l.Definition.Name.Value)
		}
		return val
	case *object.Step:
		member := node.Member.Value
		switch member {
		case "T":
			if !l.IsActive || l.ActivationTime.IsZero() {
				return &object.Time{Value: 0}
			}
			elapsed := nowFunc().Sub(l.ActivationTime)
			return &object.Time{Value: elapsed}
		case "X": // The 'X' flag is equivalent to IsActive
			return nativeBoolToBooleanObject(l.IsActive)
		default:
			return newError(node, "member '%s' not found for type STEP", member)
		}
	case *object.Action:
		member := node.Member.Value
		switch member {
		case "Q":
			return nativeBoolToBooleanObject(l.IsActive)
		default:
			return newError(node, "member '%s' not found for type ACTION", member)
		}
	default:
		return newError(node, "member access not supported for type %s", left.Type())
	}
}

// applyNumericConversion parses a string value for a numeric typed literal (e.g.,
// `INT#10`, `REAL#3.14`) and creates the corresponding runtime object, performing
// range checking based on the specified type.
func applyNumericConversion(value, typeName string) object.Object {
	base := 10
	valueStr := value
	upperTypeName := strings.ToUpper(typeName)

	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		if len(parts) == 2 {
			parsedBase, err := strconv.Atoi(parts[0])
			if err == nil && (parsedBase == 2 || parsedBase == 8 || parsedBase == 10 || parsedBase == 16) {
				base = parsedBase
				valueStr = parts[1]
			}
		}
	}
	valueStr = strings.ReplaceAll(valueStr, "_", "")

	if object.IsRealType(upperTypeName) {
		val, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return object.NewBuiltinError("could not parse %q as %s: %v", value, typeName, err)
		}
		return &object.Real{Value: val}
	}

	if strings.HasPrefix(upperTypeName, "U") { // Unsigned
		// Explicitly check for a negative sign, as ParseUint will return a syntax error,
		// but we want to provide a more user-friendly "out of range" error.
		if strings.HasPrefix(valueStr, "-") {
			return object.NewBuiltinError("value %s is out of range for type %s", valueStr, typeName)
		}
		uVal, err := strconv.ParseUint(valueStr, base, 64)
		if err != nil {
			if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
				return object.NewBuiltinError("value %s is out of range for type %s", valueStr, typeName)
			}
			return object.NewBuiltinError("could not parse %q as %s: %v", value, typeName, err)
		}
		targetRange := integerTypeRanges[upperTypeName]
		if uVal > targetRange.maxUnsigned {
			return object.NewBuiltinError("value %d is out of range for type %s", uVal, typeName)
		}
		t := object.ObjectType(upperTypeName)
		switch t {
		case object.USINT_OBJ:
			return &object.USInt{Value: uint8(uVal)}
		case object.UINT_OBJ:
			return &object.UInt{Value: uint16(uVal)}
		case object.UDINT_OBJ:
			return &object.UDInt{Value: uint32(uVal)}
		case object.ULINT_OBJ:
			return &object.ULInt{Value: uVal}
		}
		return object.NewBuiltinError("internal error: unhandled unsigned integer type %s", t)
	} else { // Signed
		val, err := strconv.ParseInt(valueStr, base, 64)
		if err != nil {
			if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
				return object.NewBuiltinError("value %s is out of range for type %s", valueStr, typeName)
			}
			return object.NewBuiltinError("could not parse %q as %s: %v", value, typeName, err)
		}
		targetRange := integerTypeRanges[upperTypeName]
		if val < targetRange.minSigned || val > targetRange.maxSigned {
			return object.NewBuiltinError("value %d is out of range for type %s", val, typeName)
		}
		t := object.ObjectType(upperTypeName)
		switch t {
		case object.SINT_OBJ:
			return &object.SInt{Value: int8(val)}
		case object.INT_OBJ:
			return &object.Int{Value: int16(val)}
		case object.DINT_OBJ:
			return &object.DInt{Value: int32(val)}
		case object.LINT_OBJ:
			return &object.LInt{Value: val}
		}
		return object.NewBuiltinError("internal error: unhandled signed integer type %s", t)
	}
}

// isAnyBit checks if an object's type is part of the ANY_BIT family.
func isAnyBit(obj object.Object) bool {
	t := obj.Type() // cspell:disable-line
	return t == object.BOOLEAN_OBJ || t == object.BITSTRING_OBJ
}

// evalComparisonInfix handles comparison operations for types not covered by specific infix evaluators.
func evalComparisonInfix(node *ast.InfixExpression, left, right object.Object) object.Object {
	// This function provides a fallback for comparing types that don't have a dedicated
	// infix evaluation function, such as strings, booleans, and time types.
	// Handle NULL comparisons
	if left == NULL || right == NULL {
		if node.Operator == "=" {
			return nativeBoolToBooleanObject(left == right)
		}
		if node.Operator == "!=" {
			return nativeBoolToBooleanObject(left != right)
		}
		return newError(node, "unsupported operator for NULL: %s", node.Operator)
	}

	// Handle Boolean comparisons
	if left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ {
		leftVal := left.(*object.Boolean).Value
		rightVal := right.(*object.Boolean).Value
		switch node.Operator {
		case "=":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "!=":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
		}
	}

	// Handle String comparisons
	if left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ {
		leftVal := left.(*object.String).Value
		rightVal := right.(*object.String).Value
		// For strings, all comparison operators are valid.
		return evalGenericComparison(node.Operator, leftVal, rightVal)
	}

	// Handle WString comparisons
	if left.Type() == object.WSTRING_OBJ && right.Type() == object.WSTRING_OBJ {
		leftVal := left.(*object.WString).Value
		rightVal := right.(*object.WString).Value
		return evalGenericComparison(node.Operator, leftVal, rightVal)
	}

	// Handle Time comparisons
	if left.Type() == object.TIME_OBJ && right.Type() == object.TIME_OBJ {
		leftVal := left.(*object.Time).Value
		rightVal := right.(*object.Time).Value
		return evalGenericComparison(node.Operator, int64(leftVal), int64(rightVal))
	}

	// Handle Date comparisons
	if left.Type() == object.DATE_OBJ && right.Type() == object.DATE_OBJ {
		leftVal := left.(*object.Date).Value
		rightVal := right.(*object.Date).Value
		// Compare using Unix nanoseconds for a consistent integer-based comparison
		return evalGenericComparison(node.Operator, leftVal.UnixNano(), rightVal.UnixNano())
	}

	// Handle EnumeratedValue comparisons
	if left.Type() == object.ENUMERATED_VALUE_OBJ && right.Type() == object.ENUMERATED_VALUE_OBJ {
		leftVal := left.(*object.EnumeratedValue)
		rightVal := right.(*object.EnumeratedValue)
		// For enums, only equality and inequality are meaningful.
		// They must be of the same type and have the same value.
		isEqual := leftVal.TypeName == rightVal.TypeName && leftVal.Value == rightVal.Value
		switch node.Operator {
		case "=":
			return nativeBoolToBooleanObject(isEqual)
		case "!=":
			return nativeBoolToBooleanObject(!isEqual)
		default:
			return newError(node, "unknown operator for enumerated types: %s", node.Operator)
		}
	}

	// Handle TimeOfDay comparisons
	if left.Type() == object.TIME_OF_DAY_OBJ && right.Type() == object.TIME_OF_DAY_OBJ {
		leftVal := left.(*object.TimeOfDay).Value
		rightVal := right.(*object.TimeOfDay).Value
		// Convert to nanoseconds since midnight for comparison, ignoring date part
		leftNs := int64(leftVal.Hour())*int64(time.Hour) + int64(leftVal.Minute())*int64(time.Minute) + int64(leftVal.Second())*int64(time.Second) + int64(leftVal.Nanosecond())
		rightNs := int64(rightVal.Hour())*int64(time.Hour) + int64(rightVal.Minute())*int64(time.Minute) + int64(rightVal.Second())*int64(time.Second) + int64(rightVal.Nanosecond())
		return evalGenericComparison(node.Operator, leftNs, rightNs)
	}

	// Handle DateAndTime comparisons
	if left.Type() == object.DATE_AND_TIME_OBJ && right.Type() == object.DATE_AND_TIME_OBJ {
		leftVal := left.(*object.DateAndTime).Value
		rightVal := right.(*object.DateAndTime).Value
		// Compare using Unix nanoseconds for a consistent integer-based comparison
		return evalGenericComparison(node.Operator, leftVal.UnixNano(), rightVal.UnixNano())
	}

	// Handle EnumeratedValue comparisons
	if left.Type() == object.ENUMERATED_VALUE_OBJ && right.Type() == object.ENUMERATED_VALUE_OBJ {
		leftVal := left.(*object.EnumeratedValue)
		rightVal := right.(*object.EnumeratedValue)
		// For enums, only equality and inequality are meaningful.
		// They must be of the same type and have the same value.
		isEqual := leftVal.TypeName == rightVal.TypeName && leftVal.Value == rightVal.Value
		switch node.Operator {
		case "=":
			return nativeBoolToBooleanObject(isEqual)
		case "!=":
			return nativeBoolToBooleanObject(!isEqual)
		default:
			return newError(node, "unknown operator for enumerated types: %s", node.Operator)
		}
	}

	// If types are different, it's a type mismatch for comparison
	return newError(node, "type mismatch for comparison: %s %s %s", left.Type(), node.Operator, right.Type())
}

// NewScheduler is a placeholder for a function that would create a runtime
// scheduler from a fully evaluated configuration environment, organizing tasks
// and their associated programs.
func NewScheduler(configEnv *object.Environment) (*object.Scheduler, *object.Error) {
	scheduler := &object.Scheduler{Tasks: []*object.Task{}}

	// This assumes a single resource for simplicity. A full implementation would iterate all resources.
	// Let's find the first resource environment.
	var resourceEnv *object.Environment
	for _, name := range configEnv.Names() {
		obj, _ := configEnv.Get(name)
		// A resource is stored as a FunctionBlockInstance that holds its environment.
		if resInstance, ok := obj.(*object.FunctionBlockInstance); ok {
			// We found our resource. For now, we only support one.
			// A more advanced implementation would check the type of the instance.
			resourceEnv = resInstance.Env
			break
		}
	}
	if resourceEnv == nil {
		return nil, object.NewBuiltinError("no resource found in configuration")
	}

	// Gather all TaskDeclarations and ProgramConfigurations
	tasks := make(map[string]*object.Task)
	programs := make(map[string]*object.ProgramInstance)

	for _, name := range resourceEnv.Names() {
		obj, _ := resourceEnv.Get(name)
		switch obj := obj.(type) {
		case *object.Task: // Assuming TaskDeclarations are evaluated into object.Task
			tasks[obj.Name] = obj
		case *object.ProgramInstance:
			programs[name] = obj
		}
	}

	// Associate programs with tasks
	for _, progInstance := range programs {
		if progInstance.TaskName != "" {
			if task, ok := tasks[progInstance.TaskName]; ok {
				task.Programs = append(task.Programs, progInstance)
			}
		} else {
			// Handle programs with no task association (run once or continuously at low priority)
		}
	}

	for _, task := range tasks {
		scheduler.Tasks = append(scheduler.Tasks, task)
	}

	// Sort tasks by priority (lower number = higher priority)
	sort.Slice(scheduler.Tasks, func(i, j int) bool {
		return scheduler.Tasks[i].Priority < scheduler.Tasks[j].Priority
	})

	return scheduler, nil
}

// RunScheduler is a placeholder for a function that would start the main execution
// loop of a scheduler, triggering tasks based on their configured interval or
// event conditions.
func RunScheduler(s *object.Scheduler, env *object.Environment, scanCycle time.Duration) {
	ticker := time.NewTicker(scanCycle)
	defer ticker.Stop()

	fmt.Println("Scheduler started. Press Ctrl+C to stop.")

	for range ticker.C {
		now := time.Now()
		readyTasks := []*object.Task{}

		// 1. Check for triggers and identify ready tasks
		for _, task := range s.Tasks {
			isReady := false
			if task.Interval > 0 {
				// Periodic task
				if now.Sub(task.LastExecution) >= task.Interval {
					isReady = true
					task.LastExecution = now
				}
			} else if task.Trigger != nil {
				// Event-driven task
				triggerValObj := Eval(task.Trigger, env) // cspell:disable-line
				currentTriggerVal := isTruthy(triggerValObj)
				// Check for rising edge
				if currentTriggerVal && !task.LastTriggerValue {
					isReady = true
				}
				task.LastTriggerValue = currentTriggerVal
			}

			if isReady {
				readyTasks = append(readyTasks, task)
			}
		}

		// No need to re-sort, as the main list is already prioritized.
		// We just need to execute them in the order they appear in s.Tasks.

		// 2. Execute ready tasks according to priority
		for _, task := range s.Tasks {
			isTaskReady := false
			for _, readyTask := range readyTasks {
				if task == readyTask {
					isTaskReady = true
					break
				}
			}

			if isTaskReady {
				fmt.Printf("Executing Task: %s (Priority: %d)\n", task.Name, task.Priority)
				for _, prog := range task.Programs {
					// Execute the program body in its own instance environment
					Eval(prog.Definition.Body, prog.Env)

					// Handle output mappings (=>)
					for _, mapping := range prog.OutputMappings {
						val, _ := prog.Env.Get(mapping.SourceParamName)
						env.Set(mapping.TargetVarName, val) // Set in the global/resource scope
					}
				}
			}
		}
	}
}

// evalGenericComparison provides a centralized comparison logic for types that
// can be represented as either `string` or `int64`, handling all standard
// comparison operators.
func evalGenericComparison[T ~string | ~int64](op string, leftVal, rightVal T) object.Object {
	switch op {
	case "=":
		return nativeBoolToBooleanObject(leftVal == rightVal) // This now works for strings too
	case "!=", "<>":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		// This path should ideally not be hit if called from evalComparisonInfix,
		// but it's here for robustness.
		return object.NewBuiltinError("unknown operator '%s' for generic comparison", op)
	}
}

// evalPrefixExpression evaluates a prefix expression by first evaluating the
// right-hand side, then applying the operator (e.g., NOT, -).
func evalPrefixExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	switch node.Operator {
	case "NOT", "!":
		return evalNotOperatorExpression(node, right)
	case "-":
		return evalMinusPrefixOperatorExpression(node, right)
	default:
		return newError(node, "unknown operator: %s %s", node.Operator, right.Type())
	}
}

// evalBitStringPrefixExpression handles the bitwise `NOT` operation for BitString objects.
func evalBitStringPrefixExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	if right.Type() != object.BITSTRING_OBJ {
		return newError(node, "unknown operator: %s%s", node.Operator, right.Type())
	}

	bitString := right.(*object.BitString)
	value := bitString.Value
	width := bitString.Width

	switch node.Operator {
	case "NOT":
		var mask uint64
		if width < 64 { // For widths less than 64, create a mask of 'width' ones
			mask = (1 << width) - 1
		} else { // For 64-bit, all bits are relevant
			mask = 0xFFFFFFFFFFFFFFFF // All ones
		}
		return &object.BitString{Value: ^value & mask, Width: width}
	default:
		return newError(node, "unknown operator: %s%s", node.Operator, right.Type())
	}
}

// evalNotOperatorExpression handles the `NOT` operator for both booleans and bit-strings.
func evalNotOperatorExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Boolean:
		if right.Value {
			return FALSE
		}
		return TRUE
	case *object.BitString: // Delegate all bitstring NOT operations
		return evalBitStringPrefixExpression(node, right)
	default:
		return newError(node, "unknown operator: %s %s", node.Operator, right.Type())
	}
}

// evalMinusPrefixOperatorExpression handles the unary minus operator for numeric types.
func evalMinusPrefixOperatorExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	if !object.IsNumeric(right) {
		return newError(node, "unknown operator: - %s", right.Type())
	}

	switch val := right.(type) {
	case *object.SInt:
		return &object.SInt{Value: -val.Value}
	case *object.Int:
		return &object.Int{Value: -val.Value}
	case *object.DInt:
		return &object.DInt{Value: -val.Value}
	case *object.LInt:
		return &object.LInt{Value: -val.Value}
	case *object.Real:
		return &object.Real{Value: -val.Value}
	case *object.LReal:
		return &object.LReal{Value: -val.Value}
	// Negating an unsigned integer results in a signed integer of the same or larger size.
	// We'll promote to the next signed size.
	case *object.USInt:
		return &object.SInt{Value: -int8(val.Value)}
	case *object.UInt:
		return &object.Int{Value: -int16(val.Value)}
	}
	return newError(node, "unknown operator: -%s", right.Type())
}
