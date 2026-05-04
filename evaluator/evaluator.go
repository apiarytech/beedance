package evaluator

import (
	"beedance/ast"
	"beedance/object"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	NULL = &object.Null{}
	// TRUE and FALSE are singletons to optimize memory and comparison.
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {

	// Statements
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

	case *ast.SFCProgram:
		return evalSFCProgram(node, env)

	case *ast.InitialStepStatement:
		return newError(node, "SFC InitialStepStatement not yet implemented for direct evaluation")

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
		return &object.Integer{Value: node.Value}

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

	}

	return nil
}

// evalSFCProgram evaluates an SFC program.
func evalSFCProgram(program *ast.SFCProgram, env *object.Environment) object.Object {
	sfc := &object.SFC{
		Steps:       make(map[string]*object.Step),
		Transitions: []*object.Transition{},
		ActiveSteps: make(map[string]bool),
	}

	// 1. Build the SFC structure from the AST
	for _, element := range program.Elements {
		switch elem := element.(type) {
		case *ast.InitialStepStatement: // Initial steps are also regular steps
			sfc.InitialStepName = elem.Name.Value
			sfc.Steps[elem.Name.Value] = &object.Step{Name: elem.Name, Actions: elem.Actions, IsActive: false}
		case *ast.StepStatement:
			sfc.Steps[elem.Name.Value] = &object.Step{Name: elem.Name, Actions: elem.Actions, IsActive: false}
		case *ast.TransitionStatement:
			sfc.Transitions = append(sfc.Transitions, &object.Transition{
				FromSteps: elem.From,
				ToSteps:   elem.To,
				Condition: elem.Condition,
			})
		}
	}

	// 2. Initialize the SFC state
	if sfc.InitialStepName == "" {
		return newError(program, "SFC program has no initial step") //
	}
	sfc.ActiveSteps[sfc.InitialStepName] = true
	sfc.Steps[sfc.InitialStepName].IsActive = true

	// This would typically be part of a larger execution loop (e.g., PLC scan cycle)
	// For this basic implementation, we'll just run one evaluation cycle.
	return evalSFCCycle(sfc, env)
}

func evalSFCCycle(sfc *object.SFC, env *object.Environment) object.Object {
	// Create a snapshot of active steps before evaluation
	stepsToEvaluate := make([]string, 0, len(sfc.ActiveSteps))
	for stepName := range sfc.ActiveSteps {
		stepsToEvaluate = append(stepsToEvaluate, stepName)
	}

	// Rule 1: Evaluate actions of all active steps
	for _, stepName := range stepsToEvaluate {
		step := sfc.Steps[stepName]
		for _, action := range step.Actions {
			// For a basic implementation, we just evaluate the action body
			// In a full implementation, action qualifiers (N, P, L, etc.) would be handled here.
			evaluated := Eval(action.Body, env)
			if isError(evaluated) {
				return evaluated // Propagate errors
			}
		}
	}

	// Rule 2 & 3: Evaluate transitions and update step states
	transitionsToClear := []*object.Transition{}
	for _, transition := range sfc.Transitions {
		// Check if the transition is enabled (all preceding steps are active)
		isEnabled := true
		for _, fromStepName := range transition.FromSteps { //
			if !sfc.ActiveSteps[fromStepName] {
				isEnabled = false
				break
			}
		}

		if isEnabled {
			conditionResult := Eval(transition.Condition, env)
			if isTruthy(conditionResult) { //
				transitionsToClear = append(transitionsToClear, transition)
			}
		}
	}

	// Rule 4: Deactivate old steps and activate new ones
	for _, transition := range transitionsToClear {
		for _, fromStep := range transition.FromSteps { //
			delete(sfc.ActiveSteps, fromStep)
			sfc.Steps[fromStep].IsActive = false
		}
		for _, toStep := range transition.ToSteps { //
			sfc.ActiveSteps[toStep] = true
			sfc.Steps[toStep].IsActive = true
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

func evalPrefixExpression(node *ast.PrefixExpression, right object.Object) object.Object {
	switch node.Operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(node, right)
	default:
		return newError(node, "unknown operator: %s%s", node.Operator, right.Type())
	}
}

func evalInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(node, left, right)
	case left.Type() == object.REAL_OBJ && right.Type() == object.REAL_OBJ:
		return evalRealInfixExpression(node, left, right)
	// IEC 61131-3 Type Promotion: INT -> REAL
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.REAL_OBJ:
		leftReal := &object.Real{Value: float64(left.(*object.Integer).Value)}
		return evalRealInfixExpression(node, leftReal, right)
	case left.Type() == object.REAL_OBJ && right.Type() == object.INTEGER_OBJ:
		rightReal := &object.Real{Value: float64(right.(*object.Integer).Value)}
		return evalRealInfixExpression(node, left, rightReal)
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

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
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

func evalRealInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Real).Value
	rightVal := right.(*object.Real).Value

	switch node.Operator {
	case "+":
		return &object.Real{Value: leftVal + rightVal}
	case "-":
		return &object.Real{Value: leftVal - rightVal}
	case "*":
		return &object.Real{Value: leftVal * rightVal}
	case "/":
		// IEC 61131-3 Annex E specifies an error for division by zero.
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
	case "XOR", "XOR":
		return &object.BitString{Value: leftVal ^ rightVal, Width: width}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
}

func evalIntegerInfixExpression(
	node *ast.InfixExpression,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch node.Operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		// IEC 61131-3 Annex E specifies an error for division by zero.
		if rightVal == 0 {
			return newError(node, "division by zero")
		}
		return &object.Integer{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError(node, "unknown operator: %s %s %s", left.Type(), node.Operator, right.Type())
	}
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

	// If types are different, it's a type mismatch for comparison
	return newError(node, "type mismatch for comparison: %s %s %s", left.Type(), node.Operator, right.Type())
}
