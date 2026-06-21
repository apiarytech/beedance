package evaluator

import (
	"beedance/ast"
	"beedance/object"
	"beedance/token"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	NULL = &object.Null{}
	// TRUE and FALSE are singletons to optimize memory and comparison.
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	EXIT  = &object.Exit{}

	// currentResultVar is the internal name for the IL accumulator (Current Result).
	currentResultVar = "__CURRENT_RESULT__"

	// nowFunc is a variable that can be overridden for testing purposes.
	nowFunc = time.Now
)

// standardFBs holds the definitions for standard function blocks like TON, CTU, etc.
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

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {

	// Statements
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.SFCProgram:
		return evalSFCProgram(node, env)

	case *ast.ConfigurationDeclaration:
		return evalConfigurationDeclaration(node, env)

	case *ast.TaskDeclaration:
		return evalTaskDeclaration(node, env)

	case *ast.BlockStatement:
		// Check if this is an IL program body
		if len(node.Statements) > 0 {
			if _, ok := node.Statements[0].(*ast.IlInstructionStatement); ok {
				return evalIlProgram(node.Statements, env)
			}
		}
		return evalBlockStatement(node, env)

	case *ast.VarBlockDeclaration:
		return evalVarBlockStatement(node, env)

	// SFC elements are handled within the context of a program/function block body, not as standalone statements.

	case *ast.TypeBlockDeclaration:
		return evalTypeBlockDeclaration(node, env)

	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

	case *ast.FunctionDeclaration:
		fn := &object.Function{
			Name:       node.Name,
			VarInputs:  node.VarInputs,
			VarOutputs: node.VarOutputs,
			VarInOuts:  node.VarInOuts,
			Vars:       node.Vars,
			Body:       node.Body,
			Env:        env,
		}
		env.Set(node.Name.Value, fn)
		return fn

	case *ast.FunctionBlockDeclaration:
		fb := &object.FunctionBlock{
			Name:       node.Name,
			VarInputs:  node.VarInputs,
			VarOutputs: node.VarOutputs,
			VarInOuts:  node.VarInOuts,
			Vars:       node.Vars,
			Body:       node.Body,
			Env:        env, // The environment where the FB is declared
		}
		env.Set(node.Name.Value, fb)
		return fb

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.TypedLiteral:
		// The parser gives us a TypedLiteral for constructs like `INT#10` or `DATE#'2023-01-01'`.
		// The `Value` field of the AST node is an expression that needs to be evaluated.
		// For `INT#10`, `node.Value` is an `IntegerLiteral`.
		// For `DATE#'...'`, `node.Value` is an `Identifier` with the date string.
		valueObj := Eval(node.Value, env)
		if isError(valueObj) {
			return valueObj
		}

		targetTypeName := node.TypeName
		// The source type is derived from the evaluated object.

		// 1. Handle enumerated typed literals (e.g., COLOR#RED)
		if enumTypeObj, ok := env.Get(targetTypeName); ok {
			if enumType, isEnumType := enumTypeObj.(*object.EnumeratedType); isEnumType {
				if enumVal, isEnumVal := valueObj.(*object.EnumeratedValue); isEnumVal {
					if _, exists := enumType.Values[enumVal.Value]; exists {
						return &object.EnumeratedValue{TypeName: targetTypeName, Value: enumVal.Value}
					}
					return newError(node, "enumerated value '%s' not found in type '%s'", enumVal.Value, targetTypeName)
				}
				return newError(node, "expected enumerated value, got %s", valueObj.Type())
			}
		}

		// 2. Handle time/date typed literals (e.g., DATE#'2023-01-01')
		if timeDateObj := applyTimeDateConversion(valueObj.Inspect(), targetTypeName); timeDateObj.Type() != object.ERROR_OBJ {
			return timeDateObj
		}

		// 3. Handle other typed literals (e.g., INT#10, REAL#1.23)
		return applyConversion(valueObj, string(valueObj.Type()), targetTypeName)

	// Expressions
	case *ast.IntegerLiteral:
		switch node.Token.Type {
		case token.SINT:
			return &object.SInt{Value: int8(node.Value)}
		case token.INT:
			return &object.Int{Value: int16(node.Value)}
		case token.DINT:
			return &object.DInt{Value: int32(node.Value)}
		case token.LINT:
			return &object.LInt{Value: node.Value}
		default:
			return &object.LInt{Value: node.Value}
		}
	case *ast.UnsignedIntegerLiteral:
		switch node.Token.Type {
		case token.USINT:
			return &object.USInt{Value: uint8(node.Value)}
		case token.UINT:
			return &object.UInt{Value: uint16(node.Value)}
		case token.UDINT:
			return &object.UDInt{Value: uint32(node.Value)}
		case token.ULINT:
			return &object.ULInt{Value: node.Value}
		}

	case *ast.RealLiteral:
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
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		// Handle NOT for BitStrings
		if node.Operator == "NOT" && right.Type() == object.BITSTRING_OBJ {
			return evalBitStringPrefixExpression(node, right)
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

		return evalInfixExpression(node, left, right)

	case *ast.MemberAccessExpression:
		return evalMemberAccessExpression(node, env)

	case *ast.IfStatement:
		return evalIfStatement(node, env)
	case *ast.ForLoopStatement:
		return evalForLoopStatement(node, env)
	case *ast.WhileStatement:
		return evalWhileStatement(node, env)
	case *ast.RepeatStatement:
		return evalRepeatStatement(node, env)
	case *ast.ExitStatement:
		// EXIT statements simply return a special EXIT object
		// that loop evaluators will catch.
		return EXIT
	case *ast.CaseStatement:
		return evalCaseStatement(node, env)

	case *ast.VarDeclStatement:
		return evalVarDeclStatement(node, env)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.FunctionLiteral:
		// Convert the simple identifiers from the function literal into
		// VarDeclStatements to match the structure of a formal Function object.
		varInputs := make([]*ast.VarDeclStatement, len(node.Parameters))
		for i, p := range node.Parameters {
			varInputs[i] = &ast.VarDeclStatement{
				Name: p,
				// DataType would be ANY or inferred in a more advanced system.
			}
		}
		body := node.Body
		return &object.Function{
			VarInputs: varInputs, Env: env, Body: body,
		}

	case *ast.CallExpression:
		// Special handling for 'quote' macro
		if node.Function.TokenLiteral() == "quote" {
			return quote(node.Arguments[0], env)
		}

		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}

		// Pass raw AST arguments to applyFunction for proper handling of named/output args
		return applyFunction(function, node.Arguments, env)

	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}

	case *ast.IndexExpression:
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

	case *ast.IlInstructionStatement:
		return evalIlInstructionStatement(node, env)

	}

	return nil
}

// evalSFCProgram is the entry point for SFC evaluation. It builds the runtime
// SFC object from the AST, initializes it, and stores it in the environment.
// In a real PLC, a scheduler would then call evalSFCCycle repeatedly.
// For our purposes, the initial call might run one cycle to set initial states.
// The returned object is the SFC instance itself, which can be cycled further.
func evalSFCProgram(program *ast.SFCProgram, env *object.Environment) object.Object {
	sfc := &object.SFC{
		Steps:       make(map[string]*object.Step),
		Transitions: []*object.Transition{},
		Actions:     make(map[string]*object.Action),
		ActiveSteps: make(map[string]bool),
	}

	// 1. Build the SFC structure from the AST
	for _, element := range program.Elements {
		switch elem := element.(type) {
		case *ast.StepStatement: // Initial steps are also regular steps
			if elem.IsInitial {
				sfc.InitialStepName = elem.Name.Value
			}
			step := &object.Step{Name: elem.Name, Actions: elem.Actions, IsActive: false}
			sfc.Steps[elem.Name.Value] = step
			for _, actionBlock := range elem.Actions {
				actionName := actionBlock.ActionName.Value
				if _, ok := sfc.Actions[actionName]; !ok {
					// This assumes the action is defined elsewhere, e.g., as a boolean variable.
					// A full implementation would need to look up the action definition.
					// For now, we create a placeholder.
					sfc.Actions[actionName] = &object.Action{Name: actionBlock.ActionName}
				}
				sfc.Actions[actionName].AssociatedSteps = append(sfc.Actions[actionName].AssociatedSteps, step)
				// If a duration is specified in the AST, evaluate it and store it.
				if actionBlock.Duration != nil {
					durationObj := Eval(actionBlock.Duration, env)
					if isError(durationObj) {
						// This should probably be a fatal error during setup
						return durationObj
					}
					if timeObj, ok := durationObj.(*object.Time); ok {
						sfc.Actions[actionName].Duration = timeObj.Value
					} else {
						return newError(actionBlock, "action qualifier duration must be of type TIME, got %s", durationObj.Type())
					}
				}
			}
		case *ast.TransitionStatement:
			sfc.Transitions = append(sfc.Transitions, &object.Transition{
				FromSteps: elem.From,
				ToSteps:   elem.To,
				Condition: elem.Condition,
			})
		case *ast.ActionStatement:
			// An action can be referenced in a STEP before it is fully defined.
			// We need to find the existing action object and update its body.
			action, ok := sfc.Actions[elem.Name.Value]
			if !ok {
				// This case is unlikely if steps are parsed correctly, but it's safe to handle.
				action = &object.Action{Name: elem.Name}
				sfc.Actions[elem.Name.Value] = action
			}
			// The parser ensures the body of an ACTION is a BlockStatement.
			action.Body, _ = elem.Body.(*ast.BlockStatement)

		}
	}

	// 2. Initialize the SFC state
	if sfc.InitialStepName == "" {
		return newError(program, "SFC program has no initial step") //
	}
	sfc.ActiveSteps[sfc.InitialStepName] = true
	sfc.Steps[sfc.InitialStepName].IsActive = true

	// Return the initialized SFC object. The caller (e.g., a test or a scheduler) is responsible for cycling it.
	return sfc
}

func evalSFCCycle(sfc *object.SFC, env *object.Environment) object.Object {
	// Phase 1: Evaluate Action Control Logic
	for _, action := range sfc.Actions {
		evaluateAction(action)
	}
	// Phase 2: Evaluate Transitions
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

	// Phase 3: Update Step States
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

	// Phase 4: Execute Action Bodies
	for _, action := range sfc.Actions {
		if action.IsActive && action.Body != nil {
			evaluated := Eval(action.Body, env)
			if isError(evaluated) {
				return evaluated // Propagate errors
			}
		}
		// Also handle boolean variable actions
		if boolAction, ok := env.Get(action.Name.Value); ok && boolAction.Type() == object.BOOLEAN_OBJ {
			env.Set(action.Name.Value, nativeBoolToBooleanObject(action.IsActive))
		}
	}

	return NULL // A single cycle completes successfully
}

// evaluateAction determines the state of a single action based on its associated active steps and qualifiers.
func evaluateAction(action *object.Action) {
	qualifier, isStepActive := getHighestPriorityActiveQualifier(action)

	// The action is not influenced by any active step in this cycle.
	// For non-stored actions, this means they become inactive.
	// For stored actions, they maintain their state unless reset by another step.
	if !isStepActive {
		switch qualifier {
		case "N", "P", "D", "L":
			action.IsActive = false
		}
		// Reset timers and pulse counts for non-stored actions when their steps deactivate.
		if qualifier == "D" || qualifier == "L" {
			action.TimerStart = time.Time{}
		}
		if qualifier == "P" {
			action.ActivationCount = 0
		}
		return
	}

	// Apply action control logic based on the highest priority active qualifier.
	switch qualifier {
	case "R":
		action.IsActive = false
		// Resetting a stored action should also reset its timer.
		action.TimerStart = time.Time{}
	case "S":
		action.IsActive = true
	case "N":
		action.IsActive = true
	case "P":
		// Activate only on the first scan cycle that the step is active.
		if action.ActivationCount == 0 {
			action.IsActive = true
		} else {
			action.IsActive = false
		}
		action.ActivationCount++
	case "D":
		if action.TimerStart.IsZero() {
			action.TimerStart = nowFunc()
		}
		action.IsActive = time.Since(action.TimerStart) >= action.Duration
	case "L":
		if action.TimerStart.IsZero() {
			action.TimerStart = nowFunc()
		}
		action.IsActive = time.Since(action.TimerStart) < action.Duration
	case "SD", "DS": // Stored and Delayed (DS is functionally identical in this model)
		if action.TimerStart.IsZero() {
			action.TimerStart = nowFunc()
		}
		if time.Since(action.TimerStart) >= action.Duration {
			action.IsActive = true
		}
	case "SL":
		if action.TimerStart.IsZero() {
			action.TimerStart = nowFunc()
		}
		// The action becomes active immediately but is stored. It will only be deactivated
		// by an 'R' qualifier or when the time limit expires.
		if time.Since(action.TimerStart) >= action.Duration {
			action.IsActive = false
		} else {
			action.IsActive = true
		}
	}
}

// getHighestPriorityActiveQualifier finds the highest priority qualifier for an action among all its active associated steps.
// IEC 61131-3 specifies the precedence: R > S > (all others).
func getHighestPriorityActiveQualifier(action *object.Action) (qualifier string, isStepActive bool) {
	qualifierPrecedence := map[string]int{"R": 2, "S": 1}
	highestQualifier := "N" // Default qualifier
	highestPrecedence := 0
	anyStepActive := false

	for _, step := range action.AssociatedSteps {
		if step.IsActive {
			anyStepActive = true
			for _, actionBlock := range step.Actions {
				if actionBlock.ActionName.Value == action.Name.Value {
					q := "N" // Default
					if actionBlock.Qualifier != nil {
						q = actionBlock.Qualifier.Value
					}
					if precedence, ok := qualifierPrecedence[q]; ok && precedence > highestPrecedence {
						highestPrecedence = precedence
						highestQualifier = q
					} else if highestPrecedence == 0 { // If no R or S found yet, take the current one.
						highestQualifier = q
					}
				}
			}
		}
	}
	return highestQualifier, anyStepActive
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NULL // Default to NULL

	for _, statement := range program.Statements {
		stmtResult := Eval(statement, env)

		if stmtResult != nil {
			switch res := stmtResult.(type) {
			case *object.ReturnValue:
				return res.Value
			case *object.Error:
				return res
			case *object.Null:
				// Do nothing, don't let NULL from VAR blocks overwrite a previous valid result.
			default:
				result = stmtResult // Update the result with the value of the last non-null statement
			}
		}
	}

	return result
}

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
			// The final return value of the POU will be whatever is in the function name variable.
			return result
		}

		pc++ // Increment PC for next instruction
	}
	return result
}

func evalIlInstructionStatement(node *ast.IlInstructionStatement, env *object.Environment) object.Object {
	// 1. Handle conditional execution (C modifier)
	// This applies to JMP, CAL, RET.
	isConditional := strings.Contains(node.Modifier, "C")
	if isConditional {
		crObj, ok := env.Get(currentResultVar)
		isNegatedConditional := strings.Contains(node.Modifier, "N") // e.g., JMPCN

		// If CR is not set, it's considered FALSE.
		crIsTruthy := ok && isTruthy(crObj)

		// If JMPC and CR is FALSE, skip.
		// If JMPCN and CR is TRUE, skip.
		if (isConditional && !isNegatedConditional && !crIsTruthy) || (isNegatedConditional && crIsTruthy) {
			return NULL // Skip instruction
		}
	}

	// Handle JMP separately as it affects control flow, not data.
	if strings.ToUpper(node.Operator) == "JMP" {
		if operandIdent, ok := node.Operand.(*ast.Identifier); ok {
			// We don't evaluate the operand, we just need its name as the label.
			// The actual jump is handled by the program loop.
			// We return a special Jump object to signal this.
			return &object.Jump{TargetLabel: operandIdent.Value}
		} else {
			return newError(node, "operand for JMP must be a label identifier")
		}
	}

	// 2. Evaluate the operand, if it exists
	var operand object.Object
	if node.Operand != nil {
		// Special handling for parenthesized IL expressions, where the operand is a block.
		if block, ok := node.Operand.(*ast.BlockStatement); ok {
			// The result of the parenthesized block is its own final Current Result.
			// We evaluate it in a temporary enclosed environment to not pollute the main CR.
			blockEnv := object.NewEnclosedEnvironment(env)
			evalIlProgram(block.Statements, blockEnv) // This will populate __CURRENT_RESULT__ in blockEnv
			operand, ok = blockEnv.Get(currentResultVar)
			if !ok {
				// If the block is empty or doesn't produce a result, it's an error.
				return newError(node, "parenthesized IL expression did not produce a result")
			}
		} else {
			operand = Eval(node.Operand, env)
		}
		if isError(operand) {
			return operand
		}
	}

	// 3. Handle operand negation (N modifier for non-conditional ops)
	isNegatedOperand := strings.Contains(node.Modifier, "N") && !isConditional
	if isNegatedOperand {
		// We create a temporary prefix expression to reuse the NOT logic
		notExpr := &ast.PrefixExpression{Operator: "NOT", Right: node.Operand}
		operand = evalNotOperatorExpression(notExpr, operand)
		if isError(operand) {
			return operand
		}
	}

	// 4. Execute the operator logic
	switch strings.ToUpper(node.Operator) {
	case "LD":
		env.Set(currentResultVar, operand)
		return operand

	case "ST":
		crObj, ok := env.Get(currentResultVar)
		if !ok {
			return newError(node, "ST instruction executed but Current Result is not set")
		}
		// The operand of ST must be a variable identifier
		if targetIdent, ok := node.Operand.(*ast.Identifier); ok {
			env.Set(targetIdent.Value, crObj)
			return crObj
		}
		return newError(node, "operand for ST must be a variable identifier")

	case "S": // Set (Operand is a BOOL variable)
		if targetIdent, ok := node.Operand.(*ast.Identifier); ok {
			env.Set(targetIdent.Value, TRUE)
			return TRUE
		}
		return newError(node, "operand for S must be a boolean variable")

	case "R": // Reset (Operand is a BOOL variable)
		if targetIdent, ok := node.Operand.(*ast.Identifier); ok {
			env.Set(targetIdent.Value, FALSE)
			return FALSE
		}
		return newError(node, "operand for R must be a boolean variable")

	// Arithmetic and Logic operators that update the CR
	case "ADD", "SUB", "MUL", "DIV", "AND", "OR", "XOR":
		crObj, ok := env.Get(currentResultVar)
		if !ok {
			return newError(node, "%s instruction executed but Current Result is not set", node.Operator)
		}
		if operand == nil {
			return newError(node, "%s instruction requires an operand", node.Operator)
		}

		// Reuse the infix evaluation logic
		op := node.Operator
		if op == "SUB" {
			op = "-"
		} // Map to standard operators if needed
		infixNode := &ast.InfixExpression{Operator: op}
		result := evalInfixExpression(infixNode, crObj, operand)
		if isError(result) {
			return result
		}

		env.Set(currentResultVar, result) // Update CR
		return result

	case "CAL":
		// The operand for CAL is a CallExpression to a function block instance.
		// The 'operand' variable already holds the evaluated result of this call.
		if isError(operand) {
			return operand
		}

		// After a CAL instruction, the Current Result (CR) is updated with the
		// result of the function block execution. The `applyFunction` logic
		// already returns the FB's primary output.
		env.Set(currentResultVar, operand)
		return operand

	case "RET":
		// Conditional check is handled at the top. If we are here, we should return.
		return &object.Return{}

	case "JMP": // Jump and Call operators (placeholders for now)
		// In a real evaluator, this would modify the program counter.
		// For now, we just acknowledge it.
		return NULL
	default:
		return newError(node, "unknown IL operator: %s", node.Operator)
	}
}

func evalBlockStatement(
	block *ast.BlockStatement,
	env *object.Environment,
) object.Object {
	var result object.Object = NULL // Default to NULL

	for _, statement := range block.Statements {
		stmtResult := Eval(statement, env)

		if stmtResult != nil {
			switch res := stmtResult.(type) {
			case *object.ReturnValue:
				return res // Propagate return values immediately
			case *object.Error:
				return res // Propagate errors immediately
			case *object.Null:
				// Do nothing, don't let NULL overwrite a previous valid result.
			default:
				result = stmtResult // Update the result with the value of the last non-null statement
			}
		}
	}

	return result
}

func evalVarBlockStatement(block *ast.VarBlockDeclaration, env *object.Environment) object.Object {
	for _, decl := range block.Declarations {
		if err := Eval(decl, env); isError(err) {
			return err
		}
	}
	return NULL
}

func evalTypeBlockDeclaration(block *ast.TypeBlockDeclaration, env *object.Environment) object.Object {
	for _, decl := range block.Declarations {
		// We are interested in enumerated type declarations here.
		// The parser creates an EnumDefinition for `(VAL1, VAL2, ...)`
		if enumDef, ok := decl.DataType.(*ast.EnumDefinition); ok {
			// Create an EnumeratedType object
			enumType := &object.EnumeratedType{
				Name:   decl.Name.Value,
				Values: make(map[string]*object.EnumeratedValue),
			}

			// Populate the values
			for _, valIdent := range enumDef.Values {
				enumValue := &object.EnumeratedValue{
					TypeName: decl.Name.Value,
					Value:    valIdent.Value,
				}
				enumType.Values[valIdent.Value] = enumValue
			}
			env.Set(decl.Name.Value, enumType)
		} else if subrange, ok := decl.Subrange.(*ast.InfixExpression); ok && subrange.Operator == ".." {
			// This is a subrange type declaration, e.g., TYPE MyRange : INT(0..100); END_TYPE
			lower := Eval(subrange.Left, env)
			if isError(lower) {
				return lower
			}
			upper := Eval(subrange.Right, env)
			if isError(upper) {
				return upper
			}
			lowerInt, okL := lower.(*object.LInt)
			upperInt, okU := upper.(*object.LInt)
			if !okL || !okU {
				return newError(decl, "subrange bounds must be integers")
			}

			// Validate that the base type is an integer type
			baseTypeStr := decl.DataType.String()
			if !isIntegerTypeName(baseTypeStr) {
				return newError(decl, "subrange base type must be an integer type, got %s", baseTypeStr)
			}

			enumType := &object.SubrangeType{
				Name:       decl.Name.Value,
				BaseType:   object.ObjectType(baseTypeStr),
				LowerBound: lowerInt.Value,
				UpperBound: upperInt.Value,
			}
			env.Set(decl.Name.Value, enumType)
		}
	}
	return NULL // Type declarations don't produce a value themselves.
}

func applyTimeDateConversion(value, typeName string) object.Object {
	upperType := strings.ToUpper(typeName)
	switch upperType {
	case "TIME", "T":
		duration, err := parseDuration(value)
		if err != nil {
			return newBuiltinError("could not parse TIME literal: %s", err)
		}
		return &object.Time{Value: duration}
	case "DATE", "D":
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return newBuiltinError("could not parse DATE literal: %s", err)
		}
		return &object.Date{Value: t}
	case "TIME_OF_DAY", "TOD":
		t, err := time.Parse("15:04:05.999999999", value)
		if err != nil {
			// Try without fractional part
			t, err = time.Parse("15:04:05", value)
		}
		if err != nil {
			return newBuiltinError("could not parse TIME_OF_DAY literal: %s", err)
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
			return newBuiltinError("could not parse DATE_AND_TIME literal: %s", err)
		}
		return &object.DateAndTime{Value: t}
	}
	return newBuiltinError("unknown time/date type: %s", typeName)
}

func evalVarDeclStatement(node *ast.VarDeclStatement, env *object.Environment) object.Object {
	var val object.Object
	if node.Value != nil {
		val = Eval(node.Value, env)
		if isError(val) {
			return val
		}
	}
	env.Set(node.Name.Value, val)
	return val
}

// parseDuration parses an IEC 61131-3 duration string (e.g., "1d_12h_30m_5s_10ms")
// into a time.Duration. This is a simplified implementation.
func parseDuration(s string) (time.Duration, error) {
	isNegative := false
	if strings.HasPrefix(s, "-") {
		isNegative = true
		s = s[1:] // Strip the negative sign for parsing
	}

	s = strings.ToLower(s)
	totalDuration := time.Duration(0)

	// A more robust implementation would use a regex, but for now, we can split by '_'
	parts := strings.Split(s, "_")
	// TODO: Add validation to ensure parts are in the correct order (d, h, m, s, ms)
	// and that only the last part has a fractional value.

	for _, part := range parts {
		if strings.Contains(part, "d") {
			val, err := strconv.ParseFloat(strings.TrimSuffix(part, "d"), 64)
			if err != nil {
				return 0, err
			}
			totalDuration += time.Duration(val * 24 * float64(time.Hour))
		} else if strings.Contains(part, "h") {
			val, err := strconv.ParseFloat(strings.TrimSuffix(part, "h"), 64)
			if err != nil {
				return 0, err
			}
			totalDuration += time.Duration(val * float64(time.Hour))
		} else if strings.Contains(part, "ms") {
			val, err := strconv.ParseFloat(strings.TrimSuffix(part, "ms"), 64)
			if err != nil {
				return 0, err
			}
			totalDuration += time.Duration(val * float64(time.Millisecond))
		} else if strings.Contains(part, "m") {
			val, err := strconv.ParseFloat(strings.TrimSuffix(part, "m"), 64)
			if err != nil {
				return 0, err
			}
			totalDuration += time.Duration(val * float64(time.Minute))
		} else if strings.Contains(part, "s") {
			// This must be last to avoid matching 'ms'
			dur, err := time.ParseDuration(part)
			if err != nil {
				return 0, err
			}
			totalDuration += dur
		}
	}

	if isNegative {
		totalDuration = -totalDuration
	}
	return totalDuration, nil
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	switch {
	// Handle all REAL, LREAL, and mixed INTEGER operations here.
	case isNumeric(left) && isNumeric(right):
		return evalNumericInfixExpression(node, left, right)
	case left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ:
		return evalBooleanInfixExpression(node, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(node, left, right)
	case left.Type() == object.BITSTRING_OBJ && right.Type() == object.BITSTRING_OBJ: // New: BitString operations
		return evalBitStringInfixExpression(node, left, right)
	case isComparisonOperator(node.Operator):
		return evalComparisonInfix(node, left, right)
	case left.Type() != right.Type():
		return newError(node, "type mismatch: %s %s %s",
			left.Type(), node.Operator, right.Type())
	default:
		return newError(node, "unknown operator: %s %s %s",
			left.Type(), node.Operator, right.Type())
	}
}

func evalPrefixExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	switch node.Operator {
	case "NOT", "!":
		return evalNotOperatorExpression(node, right)
	case "-":
		return evalMinusPrefixOperatorExpression(node, right)
	default:
		return newError(node, "unknown operator: %s%s", node.Operator,
			right.Type())
	}
}

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
		return newError(node, "unknown operator: %s%s", node.Operator, right.Type())
	}
}

func evalMinusPrefixOperatorExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	if !isNumeric(right) {
		return newError(node, "unknown operator: -%s", right.Type())
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

func evalBooleanInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Boolean).Value
	rightVal := right.(*object.Boolean).Value

	switch node.Operator {
	case "AND", "&":
		return nativeBoolToBooleanObject(leftVal && rightVal)
	case "OR":
		return nativeBoolToBooleanObject(leftVal || rightVal)
	case "XOR":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "=":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "<>", "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
}

// evalNumericInfixExpression handles all numeric operations, including type promotion.
func evalNumericInfixExpression(node *ast.InfixExpression, left, right object.Object) object.Object {
	// If either operand is LREAL, the result is LREAL.
	if left.Type() == object.LREAL_OBJ || right.Type() == object.LREAL_OBJ {
		leftVal, okL := getFloat64Value(left)
		rightVal, okR := getFloat64Value(right)
		if !okL || !okR {
			return newError(node, "type mismatch in LREAL expression")
		}
		return evalFloatInfixExpression(node, leftVal, rightVal, true)
	}

	// If either is REAL (and none are LREAL), the result is REAL.
	if left.Type() == object.REAL_OBJ || right.Type() == object.REAL_OBJ {
		leftVal, okL := getFloat64Value(left)
		rightVal, okR := getFloat64Value(right)
		if !okL || !okR {
			return newError(node, "type mismatch in REAL expression")
		}
		return evalFloatInfixExpression(node, leftVal, rightVal, false)
	}

	// Otherwise, both are integer types.
	return evalIntegerInfixExpression(node, left, right)
}

// evalFloatInfixExpression performs the actual operation for REAL and LREAL types.
func evalFloatInfixExpression(node *ast.InfixExpression, leftVal, rightVal float64, isLReal bool) object.Object {
	var result object.Object
	switch node.Operator {
	case "+":
		result = &object.Real{Value: leftVal + rightVal}
	case "-":
		result = &object.Real{Value: leftVal - rightVal}
	case "*":
		result = &object.Real{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0.0 {
			return newError(node, "division by zero")
		}
		result = &object.Real{Value: leftVal / rightVal}
	case "<", "LT":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">", "GT":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "=", "EQ":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=", "NE":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=", "LE":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=", "GE":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError(node, "unknown operator for REAL/LREAL: %s", node.Operator)
	}

	// If the result should be LREAL, convert it.
	if isLReal {
		return &object.LReal{Value: result.(*object.Real).Value}
	}
	return result
}

func evalBitStringInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftBitString := left.(*object.BitString)
	rightBitString := right.(*object.BitString)

	// IEC 61131-3 requires operands of bitwise operations to be of the same type (same width).
	if leftBitString.Width != rightBitString.Width {
		return newError(node, "type mismatch: bitstring operands must have same width, got %d and %d", leftBitString.Width, rightBitString.Width)
	}

	leftVal := leftBitString.Value
	rightVal := rightBitString.Value
	width := leftBitString.Width

	switch node.Operator {
	case "AND", "&":
		return &object.BitString{Value: leftVal & rightVal, Width: width}
	case "OR":
		return &object.BitString{Value: leftVal | rightVal, Width: width}
	case "XOR", "XNOR":
		return &object.BitString{Value: leftVal ^ rightVal, Width: width}
	case "NAND":
		var mask uint64
		if width < 64 {
			mask = (1 << width) - 1
		} else {
			mask = 0xFFFFFFFFFFFFFFFF
		}
		return &object.BitString{Value: ^(leftVal & rightVal) & mask, Width: width}
	case "NOR":
		var mask uint64
		if width < 64 {
			mask = (1 << width) - 1
		} else {
			mask = 0xFFFFFFFFFFFFFFFF
		}
		return &object.BitString{Value: ^(leftVal | rightVal) & mask, Width: width}
	case "=":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=", "<>":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
}

// evalIntegerInfixExpression handles arithmetic for all integer types, including promotion and overflow checking.
func evalIntegerInfixExpression(node *ast.InfixExpression, left, right object.Object) object.Object {
	leftType := left.Type()
	rightType := right.Type()
	resultType := getResultIntegerType(leftType, rightType)

	// Convert both operands to the result type for the operation.
	leftVal, isLeftUnsigned, ok := getIntegerObjectValue(left)
	if !ok {
		return newError(node, "could not get value from left operand of type %s", left.Type())
	}
	rightVal, isRightUnsigned, ok := getIntegerObjectValue(right)
	if !ok {
		return newError(node, "could not get value from right operand of type %s", right.Type())
	}

	// Perform the operation
	var resultValue int64
	var uResultValue uint64
	resultIsUnsigned := isLeftUnsigned && isRightUnsigned

	// If both are unsigned, use unsigned arithmetic.
	if resultIsUnsigned {
		uLeft, uRight := uint64(leftVal), uint64(rightVal)
		switch node.Operator {
		case "+":
			if math.MaxUint64-uLeft < uRight {
				return newError(node, "unsigned integer overflow")
			}
			uResultValue = uLeft + uRight
		case "-":
			if uLeft < uRight {
				return newError(node, "unsigned integer underflow")
			}
			uResultValue = uLeft - uRight
		case "*":
			if uRight > 0 && uLeft > math.MaxUint64/uRight {
				return newError(node, "unsigned integer overflow")
			}
			uResultValue = uLeft * uRight
		case "/":
			if uRight == 0 {
				return newError(node, "division by zero")
			}
			uResultValue = uLeft / uRight
		case "MOD":
			if uRight == 0 {
				return newError(node, "division by zero in MOD")
			}
			uResultValue = uLeft % uRight
		case "<":
			return nativeBoolToBooleanObject(uLeft < uRight)
		case ">":
			return nativeBoolToBooleanObject(uLeft > uRight)
		case "<=":
			return nativeBoolToBooleanObject(uLeft <= uRight)
		case ">=":
			return nativeBoolToBooleanObject(uLeft >= uRight)
		case "=":
			return nativeBoolToBooleanObject(uLeft == uRight)
		case "<>", "!=":
			return nativeBoolToBooleanObject(uLeft != uRight)
		default:
			return newError(node, "unknown operator for unsigned integers: %s", node.Operator)
		}
	} else {
		// If one or both are signed, use signed arithmetic.
		switch node.Operator {
		case "+":
			if (rightVal > 0 && leftVal > math.MaxInt64-rightVal) || (rightVal < 0 && leftVal < math.MinInt64-rightVal) {
				return newError(node, "signed integer overflow")
			}
			resultValue = leftVal + rightVal
		case "-":
			if (rightVal > 0 && leftVal < math.MinInt64+rightVal) || (rightVal < 0 && leftVal > math.MaxInt64+rightVal) {
				return newError(node, "signed integer underflow")
			}
			resultValue = leftVal - rightVal
		case "*":
			// Special case for MinInt64 to avoid overflow on negation
			if leftVal == math.MinInt64 || rightVal == math.MinInt64 {
				return newError(node, "signed integer overflow on multiplication with MinInt64")
			}
			if rightVal != 0 && leftVal > math.MaxInt64/abs(rightVal) {
				return newError(node, "signed integer overflow")
			}
			if rightVal != 0 && leftVal < math.MinInt64/abs(rightVal) {
				return newError(node, "signed integer underflow")
			}
			resultValue = leftVal * rightVal
		case "/":
			if rightVal == 0 {
				return newError(node, "division by zero")
			}
			// Special case for MinInt64 / -1
			if leftVal == math.MinInt64 && rightVal == -1 {
				return newError(node, "signed integer overflow (MinInt64 / -1)")
			}
			resultValue = leftVal / rightVal
		case "MOD":
			if rightVal == 0 {
				return newError(node, "division by zero in MOD")
			}
			resultValue = leftVal % rightVal
		case "<":
			return nativeBoolToBooleanObject(leftVal < rightVal)
		case ">":
			return nativeBoolToBooleanObject(leftVal > rightVal)
		case "<=":
			return nativeBoolToBooleanObject(leftVal <= rightVal)
		case ">=":
			return nativeBoolToBooleanObject(leftVal >= rightVal)
		case "=":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "<>", "!=":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return newError(node, "unknown operator for signed integers: %s", node.Operator)
		}
	}

	if resultIsUnsigned {
		return checkAndCreateIntegerObject(node, resultType, 0, uResultValue, true)
	} else {
		return checkAndCreateIntegerObject(node, resultType, resultValue, 0, false)
	}
}

// getResultIntegerType determines the result type for an integer infix operation.
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

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// getIntegerObjectValue safely extracts an int64 from any integer-like object.
func getIntegerObjectValue(obj object.Object) (val int64, isUnsigned bool, success bool) {
	switch o := obj.(type) {
	case *object.SInt:
		return int64(o.Value), false, true
	case *object.Int:
		return int64(o.Value), false, true
	case *object.DInt:
		return int64(o.Value), false, true
	case *object.LInt:
		return o.Value, false, true
	case *object.USInt:
		return int64(o.Value), true, true
	case *object.UInt:
		return int64(o.Value), true, true
	case *object.UDInt:
		return int64(o.Value), true, true
	case *object.ULInt:
		// This can lose precision if the ULINT value is > MaxInt64,
		// but it's necessary for mixed-sign arithmetic.
		return int64(o.Value), true, true
	default:
		return 0, false, false
	}
}

// checkAndCreateIntegerObject validates the computed value against the target type's bounds and creates the object.
func checkAndCreateIntegerObject(node ast.Node, t object.ObjectType, val int64, uval uint64, isUnsigned bool) object.Object {
	switch t {
	case object.SINT_OBJ:
		if val < math.MinInt8 {
			return newError(node, "SINT underflow: %d", val)
		} else if val > math.MaxInt8 {
			return newError(node, "SINT overflow: %d", val)
		}
		return &object.SInt{Value: int8(val)}
	case object.INT_OBJ:
		if val < math.MinInt16 {
			return newError(node, "INT underflow: %d", val)
		} else if val > math.MaxInt16 {
			return newError(node, "INT overflow: %d", val)
		}
		return &object.Int{Value: int16(val)}
	case object.DINT_OBJ:
		if val < math.MinInt32 {
			return newError(node, "DINT underflow: %d", val)
		} else if val > math.MaxInt32 {
			return newError(node, "DINT overflow: %d", val)
		}
		return &object.DInt{Value: int32(val)}
	case object.LINT_OBJ:
		// Overflow/underflow for LINT is handled before the operation.
		return &object.LInt{Value: val}
	case object.USINT_OBJ:
		if isUnsigned {
			if uval > math.MaxUint8 { // Check against uint64 value
				return newError(node, "USINT overflow: %d", uval)
			}
			return &object.USInt{Value: uint8(uval)}
		}
		// Result from a signed operation being cast to unsigned
		if val < 0 {
			return newError(node, "USINT underflow: %d", val)
		} else if val > math.MaxUint8 {
			return newError(node, "USINT overflow: %d", val)
		}
		return &object.USInt{Value: uint8(val)}
	case object.UINT_OBJ:
		if isUnsigned {
			if uval > math.MaxUint16 { // Check against uint64 value
				return newError(node, "UINT overflow: %d", uval)
			}
			return &object.UInt{Value: uint16(uval)}
		}
		if val < 0 {
			return newError(node, "UINT underflow: %d", val)
		} else if val > math.MaxUint16 {
			return newError(node, "UINT overflow: %d", val)
		}
		return &object.UInt{Value: uint16(val)}
	case object.UDINT_OBJ:
		if isUnsigned {
			if uval > math.MaxUint32 { // Check against uint64 value
				return newError(node, "UDINT overflow: %d", uval)
			}
			return &object.UDInt{Value: uint32(uval)}
		}
		if val < 0 {
			return newError(node, "UDINT underflow: %d", val)
		} else if val > math.MaxUint32 {
			return newError(node, "UDINT overflow: %d", val)
		}
		return &object.UDInt{Value: uint32(val)}
	case object.ULINT_OBJ:
		if !isUnsigned && val < 0 {
			return newError(node, "ULINT underflow: %d", val)
		}
		// Overflow is handled before the operation for uint64
		return &object.ULInt{Value: uval}
	}
	// Fallback to generic Integer for safety, though this path should ideally not be taken.
	return &object.LInt{Value: val}
}

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
				return Eval(branch.Consequence, env)
			}
		}
	}

	if cs.Alternative != nil {
		return Eval(cs.Alternative, env)
	}

	return NULL
}

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

		// Check selector >= lowerBound
		ge := evalInfixExpression(&ast.InfixExpression{Operator: ">="}, selector, lowerBound)
		if err, isErr := ge.(*object.Error); isErr {
			return false, err
		}

		// Check selector <= upperBound
		le := evalInfixExpression(&ast.InfixExpression{Operator: "<="}, selector, upperBound)
		if err, isErr := le.(*object.Error); isErr {
			return false, err
		}

		return ge == TRUE && le == TRUE, nil
	}

	// Handle single values
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

	// For numeric types, use the dedicated numeric comparison logic.
	if isNumeric(selector) && isNumeric(caseValue) {
		eq := evalNumericInfixExpression(&ast.InfixExpression{Operator: "="}, selector, caseValue)
		return eq == TRUE, nil
	}

	// For non-numeric types, use the generic comparison logic.
	eq := evalComparisonInfix(&ast.InfixExpression{Operator: "="}, selector, caseValue)
	if err, isErr := eq.(*object.Error); isErr {
		return false, err
	}
	return eq == TRUE, nil
}

func evalForLoopStatement(fls *ast.ForLoopStatement, env *object.Environment) object.Object {
	// The control variable and loop body execute in an enclosed environment.
	loopEnv := object.NewEnclosedEnvironment(env)

	// 1. Evaluate the initial assignment of the control variable.
	initVal := Eval(fls.ControlVar.Value, loopEnv)
	if isError(initVal) {
		return initVal
	}
	initInt, ok := initVal.(*object.LInt)
	if !ok {
		return newError(fls.ControlVar, "FOR loop start value must be an integer, got %s", initVal.Type())
	}
	controlVarName := fls.ControlVar.Left.(*ast.Identifier).Value
	loopEnv.Set(controlVarName, initInt)

	// 2. Evaluate the end value.
	endValObj := Eval(fls.EndValue, loopEnv)
	if isError(endValObj) {
		return endValObj
	}
	endVal, ok := endValObj.(*object.LInt)
	if !ok {
		return newError(fls.EndValue, "FOR loop end value must be an integer, got %s", endValObj.Type())
	}

	// 3. Evaluate the step value (or default to 1).
	stepVal := int64(1)
	if fls.StepValue != nil {
		stepValObj := Eval(fls.StepValue, loopEnv)
		if isError(stepValObj) {
			return stepValObj
		}
		stepInt, ok := stepValObj.(*object.LInt)
		if !ok {
			return newError(fls.StepValue, "FOR loop step value must be an integer, got %s", stepValObj.Type())
		}
		stepVal = stepInt.Value
	}

	// 4. Execute the loop.
	for {
		// Get the current value of the control variable.
		currentValObj, _ := loopEnv.Get(controlVarName)
		currentVal := currentValObj.(*object.LInt).Value

		// Check termination condition.
		if stepVal > 0 {
			if currentVal > endVal.Value {
				break
			}
		} else { // stepVal <= 0
			if currentVal < endVal.Value {
				break
			}
		}

		// Evaluate the loop body.
		result := Eval(fls.Body, loopEnv)
		if result != nil {
			if result.Type() == object.ERROR_OBJ || result.Type() == object.RETURN_VALUE_OBJ || result.Type() == object.EXIT_OBJ {
				// If EXIT, stop the loop and return NULL. Otherwise, propagate RETURN/ERROR.
				if result.Type() == object.EXIT_OBJ {
					return NULL
				}
				return result
			}
		}

		// Increment the control variable.
		loopEnv.Set(controlVarName, &object.LInt{Value: currentVal + stepVal})
	}

	return NULL
}

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

func evalStringInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	if node.Operator != "+" {
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	return &object.String{Value: leftVal + rightVal}
}

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

func evalConfigurationDeclaration(config *ast.ConfigurationDeclaration, env *object.Environment) object.Object {
	// Create a new environment for the configuration to hold its resources and globals.
	configEnv := object.NewEnclosedEnvironment(env)

	// 1. Evaluate Global Vars first, so they are available to resources.
	for _, globalVarBlock := range config.GlobalVars {
		Eval(globalVarBlock, configEnv)
	}

	// 2. Evaluate each resource.
	for _, resNode := range config.Resources {
		evalResourceDeclaration(resNode, configEnv)
	}

	// In a real runtime, the configuration object would be returned and managed by a scheduler.
	return NULL
}

func evalResourceDeclaration(res *ast.ResourceDeclaration, configEnv *object.Environment) object.Object {
	// Each resource has its own scope within the configuration.
	resourceEnv := object.NewEnclosedEnvironment(configEnv)

	// Evaluate task declarations within the resource.
	for _, taskDecl := range res.Tasks {
		Eval(taskDecl, resourceEnv)
	}

	// Evaluate program instances within the resource.
	for _, progConfig := range res.Programs {
		evalProgramConfiguration(progConfig, resourceEnv)
	}

	// To store the resource's environment, we wrap it in an object that implements
	// the object.Object interface. A FunctionBlockInstance is a suitable container.
	resourceInstance := &object.FunctionBlockInstance{
		Env: resourceEnv,
	}

	configEnv.Set(res.Name.Value, resourceInstance)

	return NULL
}

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
		// Store the task name on the instance itself.
		TaskName: progConfig.TaskName.Value,
		Env:      instanceEnv,
	}

	// 3. Initialize default values for all variables in the instance.
	// (This would be a more complex function that iterates all VAR blocks in progDef).
	// for _, varDecl := range progDef.AllVars {
	//     instanceEnv.Set(varDecl.Name.Value, getDefaultValue(varDecl.DataType))
	// }

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
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalIdentifier(
	node *ast.Identifier,
	env *object.Environment,
) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return dereferencePointer(node, val)
	}

	if builtin, ok := builtins[node.Value]; ok {
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
			return genericConversionBuiltin(parts[0], parts[1])
		}
	}

	return newError(node, "identifier not found: %s", node.Value)
}

// dereferencePointer recursively follows a chain of pointers until it finds a non-pointer object.
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

func isTruthy(obj object.Object) bool {
	if obj == nil || obj == NULL || obj == FALSE {
		return false
	}
	if obj == TRUE {
		return true
	}
	// IEC 61131-3 requires the condition of an IF statement to be a boolean expression.
	// Any non-boolean result is implicitly not "truthy". A stricter implementation
	// could return an error here if the type is not BOOLEAN. For now, we treat
	// non-booleans as false to prevent unexpected execution of the consequence.
	return false
}

func newError(node ast.Node, format string, a ...interface{}) *object.Error {
	line, col := node.Pos()
	return &object.Error{
		Message: fmt.Sprintf("ERROR (%d:%d): %s", line, col, fmt.Sprintf(format, a...)),
	}
}

func newBuiltinError(format string, a ...interface{}) *object.Error {
	return &object.Error{
		Message: fmt.Sprintf("BUILTIN ERROR: %s", fmt.Sprintf(format, a...)),
	}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

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

// outputArgMapping stores the information needed to map a function's output
// parameter back to a variable in the calling scope.
type outputArgMapping struct {
	SourceParamName string         // The name of the VAR_OUTPUT parameter (e.g., "Out1")
	TargetVarNode   ast.Expression // The AST node of the target variable in the calling scope (e.g., "Res1")
}

func applyFunction(fn object.Object, args []ast.Expression, callEnv *object.Environment) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		// Create a new environment for the function's execution, enclosed by the function's definition environment.
		extendedEnv := object.NewEnclosedEnvironment(fn.Env)
		_, outputMappings, err := extendFunctionEnv(fn, args, callEnv, extendedEnv)
		if err != nil {
			return err
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
		}
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
		// Check for EN input. If not provided, it defaults to TRUE.
		enValue := TRUE
		for _, arg := range args {
			if namedArg, ok := arg.(*ast.NamedArgument); ok && namedArg.Name.Value == "EN" {
				evaluatedEn := Eval(namedArg.Value, callEnv)
				if isError(evaluatedEn) {
					return evaluatedEn
				}
				if boolVal, ok := evaluatedEn.(*object.Boolean); ok {
					enValue = boolVal
				} else {
					return newError(arg, "EN input must be of type BOOL, got %s", evaluatedEn.Type())
				}
				break
			}
		}

		// Set ENO to the value of EN by default.
		fn.Env.Set("ENO", enValue)

		// If EN is FALSE, do not execute the function block body.
		if enValue == FALSE {
			// Return the primary output of the FB if it exists, otherwise NULL.
			// The outputs are not updated.
			if primaryOutput, ok := fn.Env.Get(fn.Definition.Name.Value); ok {
				return primaryOutput
			}
			return NULL
		}

		var result object.Object
		//var err *object.Error

		// For built-in FBs, the definition is nil, and logic is stored in the env.
		if fn.Definition == nil {
			// It's a built-in FB instance.
			// 1. Copy input arguments into the instance environment.
			_, _, err := extendFunctionEnv(nil, args, callEnv, fn.Env)
			if err != nil {
				return err
			}

			// 2. Execute the built-in logic.
			logicFnObj, _ := fn.Env.Get("__fb_logic__")
			logicFn := logicFnObj.(*object.BuiltinFunctionBlock).Fn
			return logicFn(fn.Env, callEnv)

		} else {
			// It's a user-defined FB instance.
			extendedEnv, outputMappings, err := extendFunctionEnv(fn.Definition, args, callEnv, fn.Env)
			if err != nil {
				return err
			}

			// Execute the function block body.
			result = Eval(fn.Definition.Body, extendedEnv)

			// Handle output arguments (=>).
			for _, mapping := range outputMappings {
				val, ok := extendedEnv.Get(mapping.SourceParamName)
				if !ok {
					return newError(mapping.TargetVarNode, "internal error: output parameter %s not found in FB scope", mapping.SourceParamName)
				}

				if targetIdent, ok := mapping.TargetVarNode.(*ast.Identifier); ok {
					callEnv.Set(targetIdent.Value, val)
				} else {
					return newError(mapping.TargetVarNode, "unsupported target for FB output argument: %T", mapping.TargetVarNode)
				}
			}
		}

		// If the block execution resulted in an error, set ENO to FALSE.
		if isError(result) {
			fn.Env.Set("ENO", FALSE)
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
		// This case is for when `function` is not a Function or Builtin object.
		return newError(nil, "not a function: %s", fn.Type()) // Pass nil for node as we don't have it here
	}
}

// extendFunctionEnv creates a new environment for a function call, populating it
// with parameters based on the provided arguments.
func extendFunctionEnv(def object.Object, args []ast.Expression, callEnv *object.Environment, targetEnv *object.Environment) (*object.Environment, []outputArgMapping, *object.Error) {
	outputMappings := []outputArgMapping{}
	positionalParamIndex := 0 // Index for positional parameters in paramDecls

	var paramDecls []*ast.VarDeclStatement
	if fbDef, ok := def.(*object.FunctionBlock); ok {
		paramDecls = fbDef.VarInputs
	} else if fDef, ok := def.(*object.Function); ok {
		paramDecls = fDef.VarInputs
	}
	// If def is nil, it's a built-in FB, and we just write to the targetEnv.

	for _, argNode := range args {
		switch arg := argNode.(type) {
		case *ast.NamedArgument: // Handle `InputName := Value`
			val := Eval(arg.Value, callEnv)
			if isError(val) {
				return nil, nil, val.(*object.Error)
			}
			targetEnv.Set(arg.Name.Value, val)

		case *ast.OutputArgument: // Handle `OutputName => TargetVar`
			// The target variable is an AST node (e.g., Identifier), not an evaluated object yet.
			// We store the AST node to resolve it in the calling environment after function execution.
			outputMappings = append(outputMappings, outputArgMapping{
				SourceParamName: arg.Source.Value,
				TargetVarNode:   arg.Target,
			})

		default: // Handle positional arguments (an expression)
			if positionalParamIndex >= len(paramDecls) { // Check against the actual parameter declarations
				return nil, nil, newError(argNode, "too many arguments in function call") //
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

// isInOutParam checks if a parameter name is declared as VAR_IN_OUT in a function/FB definition.
func isInOutParam(paramName string, def object.Object) bool {
	var inOutDecls []*ast.VarDeclStatement
	if fbDef, ok := def.(*object.FunctionBlock); ok {
		inOutDecls = fbDef.VarInOuts
	} else if fDef, ok := def.(*object.Function); ok {
		inOutDecls = fDef.VarInOuts
	}

	for _, decl := range inOutDecls {
		if decl.Name.Value == paramName {
			return true
		}
	}
	return false
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}

func evalIndexExpression(node ast.Node, left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.LINT_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(node, left, index)
	default:
		return newError(node, "index operator not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.LInt).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

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

// evalMemberAccessExpression handles access to members of objects (e.g., function block instances).
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

// isComparisonOperator checks if a given operator string is a comparison operator.
func isComparisonOperator(op string) bool {
	switch op {
	case "=", "!=", "<", ">", "<=", ">=":
		return true
	default:
		return false
	}
}

// isAnyBit checks if an object's type is part of the ANY_BIT family.
func isAnyBit(obj object.Object) bool {
	t := obj.Type()
	return t == object.BOOLEAN_OBJ || t == object.BITSTRING_OBJ
}

// evalComparisonInfix handles comparison operations for types not covered by specific infix evaluators.
func evalComparisonInfix(node *ast.InfixExpression, left, right object.Object) object.Object {
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

// NewScheduler creates a scheduler from a fully evaluated configuration environment.
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
		return nil, newBuiltinError("no resource found in configuration")
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

// Run starts the scheduler's main execution loop.
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
				triggerValObj := Eval(task.Trigger, env)
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

// evalGenericComparison centralizes comparison logic for types that can be represented as int64.
func evalGenericComparison[T ~string | ~int64](op string, leftVal, rightVal T) object.Object {
	switch op {
	case "=":
		return nativeBoolToBooleanObject(leftVal == rightVal) // This now works for strings too
	case "!=":
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
		return newBuiltinError("unknown operator '%s' for generic comparison", op)
	}
}

// isNumeric checks if an object is one of the numeric types.
func isNumeric(obj object.Object) bool {
	t := obj.Type()
	return t == object.SINT_OBJ || t == object.INT_OBJ || t == object.DINT_OBJ || t == object.LINT_OBJ ||
		t == object.USINT_OBJ || t == object.UINT_OBJ || t == object.UDINT_OBJ || t == object.ULINT_OBJ ||
		t == object.REAL_OBJ || t == object.LREAL_OBJ
}

// getFloat64Value extracts a float64 from any numeric object type for calculations.
func getFloat64Value(obj object.Object) (float64, bool) {
	switch o := obj.(type) {
	case *object.SInt:
		return float64(o.Value), true
	case *object.Int:
		return float64(o.Value), true
	case *object.DInt:
		return float64(o.Value), true
	case *object.LInt:
		return float64(o.Value), true
	case *object.USInt:
		return float64(o.Value), true
	case *object.UInt:
		return float64(o.Value), true
	case *object.UDInt:
		return float64(o.Value), true
	case *object.ULInt:
		return float64(o.Value), true
	case *object.Real:
		return o.Value, true
	case *object.LReal:
		return o.Value, true
	default:
		return 0, false
	}
}

// isIntegerTypeName checks if a string corresponds to an IEC 61131-3 integer type keyword.
func isIntegerTypeName(name string) bool {
	return name == "SINT" || name == "INT" || name == "DINT" || name == "LINT" ||
		name == "USINT" || name == "UINT" || name == "UDINT" || name == "ULINT"
}
