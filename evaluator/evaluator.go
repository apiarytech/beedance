package evaluator

import (
	"beedance/ast"
	"beedance/object"
	"beedance/token"
	"fmt"
	"math"
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

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {

	// Statements
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

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

	// Expressions
	case *ast.IntegerLiteral:
		switch node.Type {
		case token.SINT:
			return &object.SInt{Value: int8(node.Value)}
		case token.INT:
			return &object.Int{Value: int16(node.Value)}
		case token.DINT:
			return &object.DInt{Value: int32(node.Value)}
		case token.LINT:
			return &object.LInt{Value: node.Value}
		default:
			// Fallback for generic integer literals
			return &object.Integer{Value: node.Value}
		}
	case *ast.UnsignedIntegerLiteral:
		switch node.Type {
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

	case *ast.EnumeratedValueLiteral:
		// Look up the type definition in the environment.
		enumTypeObj, ok := env.Get(node.TypeName.Value)
		if !ok {
			return newError(node, "enumerated type '%s' not defined", node.TypeName.Value)
		}
		enumType, ok := enumTypeObj.(*object.EnumeratedType)
		if !ok {
			return newError(node, "'%s' is not an enumerated type", node.TypeName.Value)
		}

		// Look up the specific value within the type.
		enumValue, ok := enumType.Values[node.Value.Value]
		if !ok {
			return newError(node, "value '%s' is not a member of enumerated type '%s'", node.Value.Value, node.TypeName.Value)
		}

		return enumValue

	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.TimeLiteral:
		parts := strings.SplitN(node.Value, "#", 2)
		if len(parts) != 2 {
			return newError(node, "invalid TIME literal format: %q", node.Value)
		}
		duration, err := parseDuration(parts[1])
		if err != nil {
			return newError(node, "could not parse TIME literal: %s", err)
		}
		return &object.Time{Value: duration}

	case *ast.DateLiteral:
		parts := strings.SplitN(node.Value, "#", 2)
		if len(parts) != 2 {
			return newError(node, "invalid DATE literal format: %q", node.Value)
		}
		t, err := time.Parse("2006-01-02", parts[1])
		if err != nil {
			return newError(node, "could not parse DATE literal: %s", err)
		}
		return &object.Date{Value: t}

	case *ast.TimeOfDayLiteral:
		parts := strings.SplitN(node.Value, "#", 2)
		if len(parts) != 2 {
			return newError(node, "invalid TIME_OF_DAY literal format: %q", node.Value)
		}
		t, err := time.Parse("15:04:05.999999999", parts[1])
		if err != nil {
			return newError(node, "could not parse TIME_OF_DAY literal: %s", err)
		}
		return &object.TimeOfDay{Value: t}

	case *ast.DateAndTimeLiteral:
		parts := strings.SplitN(node.Value, "#", 2)
		if len(parts) != 2 {
			return newError(node, "invalid DATE_AND_TIME literal format: %q", node.Value)
		}
		t, err := time.Parse("2006-01-02-15:04:05.999999999", parts[1])
		if err != nil {
			return newError(node, "could not parse DATE_AND_TIME literal: %s", err)
		}
		return &object.DateAndTime{Value: t}

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
		params := node.Parameters
		body := node.Body
		return &object.Function{
			Parameters: params, Env: env, Body: body,
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

	case *ast.BlockStatement:
		// Check if this is an IL program body
		if len(node.Statements) > 0 {
			if _, ok := node.Statements[0].(*ast.IlInstructionStatement); ok {
				return evalIlProgram(node.Statements, env)
			}
		}
		return evalBlockStatement(node, env)
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
		case *ast.InitialStepStatement: // Initial steps are also regular steps
			sfc.InitialStepName = elem.Name.Value
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
		case *ast.StepStatement:
			step := &object.Step{Name: elem.Name, Actions: elem.Actions, IsActive: false}
			sfc.Steps[elem.Name.Value] = step
			for _, actionBlock := range elem.Actions {
				actionName := actionBlock.ActionName.Value
				if _, ok := sfc.Actions[actionName]; !ok {
					sfc.Actions[actionName] = &object.Action{Name: actionBlock.ActionName}
				}
				sfc.Actions[actionName].AssociatedSteps = append(sfc.Actions[actionName].AssociatedSteps, step)
				if actionBlock.Duration != nil {
					durationObj := Eval(actionBlock.Duration, env)
					if isError(durationObj) {
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
			// Store the full action definition
			action := &object.Action{Name: elem.Name, Body: elem.Body}
			sfc.Actions[elem.Name.Value] = action

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
		isAssociatedStepActive := false
		var activeQualifier string

		// Find if any associated step is active and get its qualifier for this action
		for _, step := range action.AssociatedSteps {
			if step.IsActive {
				isAssociatedStepActive = true
				for _, actionBlock := range step.Actions {
					if actionBlock.ActionName.Value == action.Name.Value {
						if actionBlock.Qualifier != nil {
							activeQualifier = actionBlock.Qualifier.Value
						} else {
							activeQualifier = "N" // Default qualifier
						}
						break
					}
				}
				break
			}
		}

		// Apply action control logic based on the standard
		switch activeQualifier {
		case "N":
			action.IsActive = isAssociatedStepActive
		case "S":
			if isAssociatedStepActive {
				action.IsActive = true
			}
		case "R":
			if isAssociatedStepActive {
				action.IsActive = false
			}
		case "P":
			if isAssociatedStepActive && action.ActivationCount == 0 {
				action.IsActive = true
				action.ActivationCount++
			} else {
				action.IsActive = false
			}
			if !isAssociatedStepActive {
				action.ActivationCount = 0 // Reset for next activation
			}
		case "D": // Delayed
			if isAssociatedStepActive {
				if action.TimerStart.IsZero() {
					action.TimerStart = nowFunc()
				}
				if time.Since(action.TimerStart) >= action.Duration {
					action.IsActive = true
				}
			} else {
				action.IsActive = false
				action.TimerStart = time.Time{} // Reset timer
			}
		case "L": // Time-Limited
			if isAssociatedStepActive {
				if action.TimerStart.IsZero() {
					action.TimerStart = nowFunc()
				}
				if time.Since(action.TimerStart) < action.Duration {
					action.IsActive = true
				} else {
					action.IsActive = false // Time limit expired
				}
			} else {
				action.IsActive = false
				action.TimerStart = time.Time{} // Reset timer
			}
		case "SD": // Stored and Delayed
			if isAssociatedStepActive {
				if action.TimerStart.IsZero() {
					action.TimerStart = nowFunc()
				}
				if time.Since(action.TimerStart) >= action.Duration {
					action.IsActive = true
				}
			} // Note: No else clause, so IsActive is not reset on step deactivation
		case "DS": // Delayed and Stored
			// This is effectively the same as SD for our implementation.
			// The standard makes a subtle distinction that is hard to model without a full PLC scan cycle.
			if isAssociatedStepActive {
				if action.TimerStart.IsZero() {
					action.TimerStart = nowFunc()
				}
				if time.Since(action.TimerStart) >= action.Duration {
					action.IsActive = true
				}
			}
		case "SL": // Stored and Time-Limited
			if isAssociatedStepActive {
				if action.TimerStart.IsZero() {
					action.TimerStart = nowFunc()
				}
				action.IsActive = time.Since(action.TimerStart) < action.Duration
			} // No else clause, so IsActive is not reset on step deactivation
		default:
			action.IsActive = isAssociatedStepActive // Default to 'N' behavior
		}
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
			sfc.Steps[fromStep.Value].IsActive = false
		}
		for _, toStep := range transition.ToSteps {
			sfc.ActiveSteps[toStep.Value] = true
			sfc.Steps[toStep.Value].IsActive = true
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

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
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
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
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
			lowerInt, okL := lower.(*object.Integer)
			upperInt, okU := upper.(*object.Integer)
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
	s = strings.ToLower(s)
	totalDuration := time.Duration(0)

	// A more robust implementation would use a regex, but for now, we can split by '_'
	parts := strings.Split(s, "_")

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
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(node, left, right) // Keep this for pure integer operations

	// Handle all REAL, LREAL, and mixed INTEGER operations here.
	case isNumeric(left) && isNumeric(right):
		// If either operand is LREAL, the result is LREAL.
		if left.Type() == object.LREAL_OBJ || right.Type() == object.LREAL_OBJ {
			return evalLRealInfixExpression(node, left, right)
		}
		// If either is REAL (and none are LREAL), the result is REAL.
		if left.Type() == object.REAL_OBJ || right.Type() == object.REAL_OBJ {
			return evalRealInfixExpression(node, left, right)
		}
		// If we are here, both must be some form of integer.
		// This case is already handled above, but we keep it for logical completeness.
		// The logic can be simplified if evalIntegerInfixExpression is merged,
		// but this separation is also clear.
		return evalIntegerInfixExpression(node, left, right)

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

func evalNotOperatorExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	switch right.Type() {
	case object.BOOLEAN_OBJ:
		if right == TRUE {
			return FALSE
		}
		return TRUE
	case object.BITSTRING_OBJ:
		return evalBitStringPrefixExpression(node, right)
	default:
		// As per TestBangOperator, NOT on a non-boolean (like an integer) should evaluate to false.
		// This is a simplification; a strict implementation might error.
		return nativeBoolToBooleanObject(!isTruthy(right))
	}
}
func evalMinusPrefixOperatorExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	if right.Type() != object.INTEGER_OBJ {
		return newError(node, "unknown operator: -%s", right.Type())
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value}
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

// evalLRealInfixExpression handles operations where at least one operand is an LREAL.
// The result is always an LREAL.
func evalLRealInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal, ok := getFloat64Value(left)
	if !ok {
		return newError(node, "type mismatch: expected numeric type for left operand, got %s", left.Type())
	}
	rightVal, ok := getFloat64Value(right)
	if !ok {
		return newError(node, "type mismatch: expected numeric type for right operand, got %s", right.Type())
	}

	switch node.Operator {
	case "+":
		return &object.LReal{Value: leftVal + rightVal}
	case "-":
		return &object.LReal{Value: leftVal - rightVal}
	case "*":
		return &object.LReal{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0.0 {
			return newError(node, "division by zero")
		}
		return &object.LReal{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
}

func evalRealInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal, ok := getFloat64Value(left)
	if !ok {
		return newError(node, "type mismatch: expected numeric type for left operand, got %s", left.Type())
	}
	rightVal, ok := getFloat64Value(right)
	if !ok {
		return newError(node, "type mismatch: expected numeric type for right operand, got %s", right.Type())
	}

	switch node.Operator {
	case "+":
		return &object.Real{Value: leftVal + rightVal}
	case "-":
		return &object.Real{Value: leftVal - rightVal}
	case "*":
		return &object.Real{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0.0 {
			return newError(node, "division by zero")
		}
		return &object.Real{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
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
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
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
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
}

func evalIntegerInfixExpression(node *ast.InfixExpression, left, right object.Object) object.Object {
	// Determine the result type based on IEC 61131-3 type promotion rules.
	// The highest rank type determines the result type.
	resultType := getResultIntegerType(left.Type(), right.Type())

	// Convert both operands to the result type for the operation.
	leftVal, leftIsUnsigned, ok := getIntegerObjectValue(left)
	if !ok {
		return newError(node, "could not get value from left operand of type %s", left.Type())
	}
	rightVal, rightIsUnsigned, ok := getIntegerObjectValue(right)
	if !ok {
		return newError(node, "could not get value from right operand of type %s", right.Type())
	}

	// Perform the operation
	var resultValue int64
	var uResultValue uint64
	var resultIsUnsigned bool

	// If both are unsigned, use unsigned arithmetic.
	if leftIsUnsigned && rightIsUnsigned {
		resultIsUnsigned = true
		uLeft, uRight := uint64(leftVal), uint64(rightVal)
		switch node.Operator {
		case "+":
			uResultValue = uLeft + uRight
		case "-":
			if uLeft < uRight {
				// This would underflow. The overflow check later will catch this
				// by converting the negative signed result to a large unsigned one.
				resultValue = leftVal - rightVal
				resultIsUnsigned = false // Treat result as signed for overflow check
			} else {
				uResultValue = uLeft - uRight
			}
		case "*":
			uResultValue = uLeft * uRight
		case "/":
			if uRight == 0 {
				return newError(node, "division by zero")
			}
			uResultValue = uLeft / uRight
		}
	} else {
		// If one or both are signed, use signed arithmetic.
		resultIsUnsigned = false
		switch node.Operator {
		case "+":
			resultValue = leftVal + rightVal
		case "-":
			resultValue = leftVal - rightVal
		case "*":
			resultValue = leftVal * rightVal
		case "/":
			if rightVal == 0 {
				return newError(node, "division by zero")
			}
			resultValue = leftVal / rightVal
		}
	}

	switch node.Operator {
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
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
	}

	// Check for overflow and return a new object of the correct, promoted type.
	switch resultType {
	case object.SINT_OBJ:
		if resultValue < math.MinInt8 || resultValue > math.MaxInt8 {
			return newError(node, "SINT overflow: %d", resultValue)
		}
		return &object.SInt{Value: int8(resultValue)}
	case object.INT_OBJ:
		if resultValue < math.MinInt16 || resultValue > math.MaxInt16 {
			return newError(node, "INT overflow: %d", resultValue)
		}
		return &object.Int{Value: int16(resultValue)}
	case object.DINT_OBJ:
		if resultValue < math.MinInt32 || resultValue > math.MaxInt32 {
			return newError(node, "DINT overflow: %d", resultValue)
		}
		return &object.DInt{Value: int32(resultValue)}
	case object.LINT_OBJ:
		// No overflow check needed as we are using int64
		return &object.LInt{Value: resultValue}
	case object.USINT_OBJ:
		if resultIsUnsigned {
			if uResultValue > math.MaxUint8 {
				return newError(node, "USINT overflow: %d", uResultValue)
			}
		} else if resultValue < 0 || resultValue > math.MaxUint8 {
			return newError(node, "USINT overflow: %d", resultValue)
		}
		return &object.USInt{Value: uint8(resultValue)}
	case object.UINT_OBJ:
		if resultIsUnsigned {
			if uResultValue > math.MaxUint16 {
				return newError(node, "UINT overflow: %d", uResultValue)
			}
		} else if resultValue < 0 || resultValue > math.MaxUint16 {
			return newError(node, "UINT overflow: %d", resultValue)
		}
		return &object.UInt{Value: uint16(resultValue)}
	case object.UDINT_OBJ:
		if resultIsUnsigned {
			if uResultValue > math.MaxUint32 {
				return newError(node, "UDINT overflow: %d", uResultValue)
			}
		} else if resultValue < 0 || resultValue > math.MaxUint32 {
			return newError(node, "UDINT overflow: %d", resultValue)
		}
		return &object.UDInt{Value: uint32(resultValue)}
	case object.ULINT_OBJ:
		if !resultIsUnsigned && resultValue < 0 {
			return newError(node, "ULINT underflow: %d", resultValue)
		}
		uResultValue = uint64(resultValue)
		// No overflow check needed for addition/multiplication as we are using uint64
		return &object.ULInt{Value: uResultValue}
	default:
		// Fallback to generic Integer for safety, though this path should ideally not be taken.
		return &object.Integer{Value: resultValue}
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
		// Generic INTEGER is treated like DINT for promotion.
		object.INTEGER_OBJ: {3, true},
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

// getIntegerObjectValue safely extracts an int64 from any integer-like object.
func getIntegerObjectValue(obj object.Object) (val int64, isUnsigned bool, success bool) {
	switch o := obj.(type) {
	case *object.Integer:
		return o.Value, false, true
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

func evalCaseStatement(cs *ast.CaseStatement, env *object.Environment) object.Object {
	selector := Eval(cs.Expression, env)
	if isError(selector) {
		return selector
	}

	for _, branch := range cs.Cases {
		for _, valueNode := range branch.Values {
			matches, err := isCaseMatch(selector, valueNode, env)
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
			return false, lowerBound
		}
		upperBound := Eval(infix.Right, env)
		if isError(upperBound) {
			return false, upperBound
		}

		// Check selector >= lowerBound
		ge := evalComparisonInfix(&ast.InfixExpression{Operator: ">="}, selector, lowerBound)
		if err, isErr := ge.(*object.Error); isErr {
			return false, err
		}

		// Check selector <= upperBound
		le := evalComparisonInfix(&ast.InfixExpression{Operator: "<="}, selector, upperBound)
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
		selectorInt, ok := selector.(*object.Integer)
		if !ok {
			// If selector is not an integer, it can't match a subrange.
			// This isn't an error, just not a match.
			return false, nil
		}
		match := selectorInt.Value >= subrange.LowerBound && selectorInt.Value <= subrange.UpperBound
		return match, nil
	}

	eq := evalComparisonInfix(&ast.InfixExpression{Operator: "=="}, selector, caseValue)
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
	initInt, ok := initVal.(*object.Integer)
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
	endVal, ok := endValObj.(*object.Integer)
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
		stepInt, ok := stepValObj.(*object.Integer)
		if !ok {
			return newError(fls.StepValue, "FOR loop step value must be an integer, got %s", stepValObj.Type())
		}
		stepVal = stepInt.Value
	}

	// 4. Execute the loop.
	for {
		// Get the current value of the control variable.
		currentValObj, _ := loopEnv.Get(controlVarName)
		currentVal := currentValObj.(*object.Integer).Value

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
		loopEnv.Set(controlVarName, &object.Integer{Value: currentVal + stepVal})
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
		return val
	}

	if builtin, ok := builtins[node.Value]; ok {
		return builtin
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

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
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
		// Create the function's execution environment, mapping arguments to parameters.
		extendedEnv, outputMappings, err := extendFunctionEnv(fn, args, callEnv, fn.VarInputs) // Pass fn.VarInputs as parameter declarations
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
			if len(fn.Body.Statements) > 0 {
				lastStmt := fn.Body.Statements[len(fn.Body.Statements)-1]
				returnValue = Eval(lastStmt, extendedEnv)
				if _, isReturn := returnValue.(*object.ReturnValue); isReturn {
					returnValue = unwrapReturnValue(returnValue)
				}
			} else {
				returnValue = NULL
			}
			ok = true // Assume anonymous functions always "return" something, even if NULL
		}
		if !ok {
			// A function must always return a value. If not explicitly set, it's an error or has a default.
			// For simplicity, we'll return NULL, but a stricter implementation might error.
		}
		return returnValue

	case *object.Builtin:
		// For built-in functions, we evaluate all arguments first.
		evalArgs := evalExpressions(args, callEnv)
		if len(evalArgs) == 1 && isError(evalArgs[0]) {
			return evalArgs[0]
		}
		return fn.Fn(evalArgs...)
	default:
		// If the function object itself is an error, it would have been caught earlier.
		// This case is for when `function` is not a Function or Builtin object.
		return newError(nil, "not a function: %s", fn.Type()) // Pass nil for node as we don't have it here
	}
}

// extendFunctionEnv creates a new environment for a function call, populating it
// with parameters based on the provided arguments.
func extendFunctionEnv(fn object.Object, args []ast.Expression, callEnv *object.Environment, paramDecls []*ast.VarDeclStatement) (*object.Environment, []outputArgMapping, *object.Error) {
	env := object.NewEnclosedEnvironment(fn.Env)
	outputMappings := []outputArgMapping{}
	positionalParamIndex := 0 // Index for positional parameters in paramDecls

	for _, argNode := range args {
		switch arg := argNode.(type) {
		case *ast.NamedArgument: // Handle `InputName := Value`
			val := Eval(arg.Value, callEnv)
			if isError(val) {
				return nil, nil, val.(*object.Error)
			}
			env.Set(arg.Name.Value, val)

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

			// Special handling for VAR_IN_OUT: pass by reference.
			// For now, we'll treat it as pass-by-value for simplicity,
			// but a proper implementation would involve storing a reference.
			// The `paramDecls` here contains both VAR_INPUT and VAR_IN_OUT.
			// We need to distinguish them. For now, we just set the value.
			// If `paramDecl` has a `VarInOut` flag, we could handle it differently.
			// Since `ast.VarDeclStatement` doesn't have a `VarInOut` flag,
			// we'll just set the value.
			env.Set(paramDecl.Name.Value, val) // This is pass-by-value

			// TODO: Implement proper VAR_IN_OUT (pass by reference)
			// This would involve storing a reference to the variable in the callEnv.
			positionalParamIndex++
		}
	}

	return env, outputMappings, nil
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}

func evalIndexExpression(node ast.Node, left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(node, left, index)
	default:
		return newError(node, "index operator not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
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
	default:
		return newError(node, "member access not supported for type %s", left.Type())
	}
}

// isComparisonOperator checks if a given operator string is a comparison operator.
func isComparisonOperator(op string) bool {
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		return true
	default:
		return false
	}
}

// evalComparisonInfix handles comparison operations for types not covered by specific infix evaluators.
func evalComparisonInfix(node *ast.InfixExpression, left, right object.Object) object.Object {
	// Handle NULL comparisons
	if left == NULL || right == NULL {
		if node.Operator == "==" {
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
		case "==":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "!=":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
		}
	}

	// Handle Time comparisons
	if left.Type() == object.TIME_OBJ && right.Type() == object.TIME_OBJ {
		leftVal := left.(*object.Time).Value
		rightVal := right.(*object.Time).Value
		switch node.Operator {
		case "==":
			return nativeBoolToBooleanObject(leftVal == rightVal)
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
			return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
		}
	}

	// Handle Date comparisons
	if left.Type() == object.DATE_OBJ && right.Type() == object.DATE_OBJ {
		leftVal := left.(*object.Date).Value
		rightVal := right.(*object.Date).Value
		switch node.Operator {
		case "==":
			return nativeBoolToBooleanObject(leftVal.Equal(rightVal))
		case "!=":
			return nativeBoolToBooleanObject(!leftVal.Equal(rightVal))
		case "<":
			return nativeBoolToBooleanObject(leftVal.Before(rightVal))
		case ">":
			return nativeBoolToBooleanObject(leftVal.After(rightVal))
		case "<=":
			return nativeBoolToBooleanObject(leftVal.Before(rightVal) || leftVal.Equal(rightVal))
		case ">=":
			return nativeBoolToBooleanObject(leftVal.After(rightVal) || leftVal.Equal(rightVal))
		default:
			return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
		}
	}

	// Handle TimeOfDay comparisons
	if left.Type() == object.TIME_OF_DAY_OBJ && right.Type() == object.TIME_OF_DAY_OBJ {
		leftVal := left.(*object.TimeOfDay).Value
		rightVal := right.(*object.TimeOfDay).Value
		// Convert to nanoseconds since midnight for comparison, ignoring date part
		leftNs := int64(leftVal.Hour())*int64(time.Hour) + int64(leftVal.Minute())*int64(time.Minute) + int64(leftVal.Second())*int64(time.Second) + int64(leftVal.Nanosecond())
		rightNs := int64(rightVal.Hour())*int64(time.Hour) + int64(rightVal.Minute())*int64(time.Minute) + int64(rightVal.Second())*int64(time.Second) + int64(rightVal.Nanosecond())

		switch node.Operator {
		case "==":
			return nativeBoolToBooleanObject(leftNs == rightNs)
		case "!=":
			return nativeBoolToBooleanObject(leftNs != rightNs)
		case "<":
			return nativeBoolToBooleanObject(leftNs < rightNs)
		case ">":
			return nativeBoolToBooleanObject(leftNs > rightNs)
		case "<=":
			return nativeBoolToBooleanObject(leftNs <= rightNs)
		case ">=":
			return nativeBoolToBooleanObject(leftNs >= rightNs)
		default:
			return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
		}
	}

	// Handle DateAndTime comparisons
	if left.Type() == object.DATE_AND_TIME_OBJ && right.Type() == object.DATE_AND_TIME_OBJ {
		leftVal := left.(*object.DateAndTime).Value
		rightVal := right.(*object.DateAndTime).Value
		switch node.Operator {
		case "==":
			return nativeBoolToBooleanObject(leftVal.Equal(rightVal))
		case "!=":
			return nativeBoolToBooleanObject(!leftVal.Equal(rightVal))
		case "<":
			return nativeBoolToBooleanObject(leftVal.Before(rightVal))
		case ">":
			return nativeBoolToBooleanObject(leftVal.After(rightVal))
		case "<=":
			return nativeBoolToBooleanObject(leftVal.Before(rightVal) || leftVal.Equal(rightVal))
		case ">=":
			return nativeBoolToBooleanObject(leftVal.After(rightVal) || leftVal.Equal(rightVal))
		default:
			return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
		}
	}

	// Handle EnumeratedValue comparisons
	if left.Type() == object.ENUMERATED_VALUE_OBJ && right.Type() == object.ENUMERATED_VALUE_OBJ {
		leftVal := left.(*object.EnumeratedValue)
		rightVal := right.(*object.EnumeratedValue)
		// For enums, only equality and inequality are meaningful.
		// They must be of the same type and have the same value.
		isEqual := leftVal.TypeName == rightVal.TypeName && leftVal.Value == rightVal.Value
		switch node.Operator {
		case "==":
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

// isNumeric checks if an object is one of the numeric types.
func isNumeric(obj object.Object) bool {
	t := obj.Type()
	return t == object.INTEGER_OBJ ||
		t == object.SINT_OBJ || t == object.INT_OBJ || t == object.DINT_OBJ || t == object.LINT_OBJ ||
		t == object.USINT_OBJ || t == object.UINT_OBJ || t == object.UDINT_OBJ || t == object.ULINT_OBJ ||
		t == object.REAL_OBJ || t == object.LREAL_OBJ
}

// getFloat64Value extracts a float64 from any numeric object type for calculations.
func getFloat64Value(obj object.Object) (float64, bool) {
	switch o := obj.(type) {
	case *object.Integer:
		return float64(o.Value), true
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
