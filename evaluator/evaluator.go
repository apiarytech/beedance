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
	NULL  = &object.Null{}
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

	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

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

	case *ast.IfStatement:
		return evalIfStatement(node, env)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: env, Body: body}

	case *ast.CallExpression:
		if node.Function.TokenLiteral() == "quote" {
			return quote(node.Arguments[0], env)
		}

		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}

		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}

		return applyFunction(function, args)

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
	case node.Operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case node.Operator == "!=":
		return nativeBoolToBooleanObject(left != right)
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
	case "AND":
		return &object.BitString{Value: leftVal & rightVal, Width: width}
	case "OR":
		return &object.BitString{Value: leftVal | rightVal, Width: width}
	case "XOR":
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

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {

	case *object.Function:
		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv) // fn.Body is an ast.Node
		return unwrapReturnValue(evaluated)

	case *object.Builtin:
		// Builtins don't have an AST node to pass, so we can't easily add line numbers here.
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(
	fn *object.Function,
	args []object.Object,
) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}

	return env
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
