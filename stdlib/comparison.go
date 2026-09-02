/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

import (
	"beedance/object"
	"time"
)

var (
	// TRUE is a singleton object representing the boolean true value.
	TRUE = &object.Boolean{Value: true}
	// FALSE is a singleton object representing the boolean false value.
	FALSE = &object.Boolean{Value: false}
)

func init() {
	object.RegisterBuiltin(BuiltinGt, "GT", comparisonBuiltin("GT"))
	object.RegisterBuiltin(BuiltinGe, "GE", comparisonBuiltin("GE"))
	object.RegisterBuiltin(BuiltinEq, "EQ", comparisonBuiltin("EQ"))
	object.RegisterBuiltin(BuiltinLe, "LE", comparisonBuiltin("LE"))
	object.RegisterBuiltin(BuiltinLt, "LT", comparisonBuiltin("LT"))
	object.RegisterBuiltin(BuiltinNe, "NE", comparisonBuiltin("NE"))
}

func comparisonBuiltin(op string) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for %s. got=%d, want=2", op, len(args))
		}
		return evalComparison(op, args[0], args[1])
	}
}

func evalComparison(op string, left, right object.Object) object.Object {
	opMap := map[string]string{"GT": ">", "GE": ">=", "EQ": "=", "LE": "<=", "LT": "<", "NE": "<>"}
	symbolicOp, ok := opMap[op]
	if !ok {
		return object.NewBuiltinError("internal error: unknown comparison operator %s", op)
	}
	return evalInfix(left, symbolicOp, right)
}

func evalInfix(left object.Object, operator string, right object.Object) object.Object {
	switch {
	case object.IsNumeric(left) && object.IsNumeric(right):
		return object.EvalNumericInfix(left, right, operator)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		leftVal := left.(*object.String).Value
		rightVal := right.(*object.String).Value
		return evalGenericComparison(operator, leftVal, rightVal)
	case left.Type() == object.WSTRING_OBJ && right.Type() == object.WSTRING_OBJ:
		leftVal := left.(*object.WString).Value
		rightVal := right.(*object.WString).Value
		return evalGenericComparison(operator, leftVal, rightVal)
	case left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ:
		leftVal := left.(*object.Boolean).Value
		rightVal := right.(*object.Boolean).Value
		switch operator {
		case "=":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "<>":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return object.NewBuiltinError("unknown operator for BOOLEAN: %s", operator)
		}
	case left.Type() == object.TIME_OBJ && right.Type() == object.TIME_OBJ:
		leftVal := left.(*object.Time).Value
		rightVal := right.(*object.Time).Value
		return evalGenericComparison(operator, int64(leftVal), int64(rightVal))
	case left.Type() == object.DATE_OBJ && right.Type() == object.DATE_OBJ:
		leftVal := left.(*object.Date).Value
		rightVal := right.(*object.Date).Value
		return evalGenericComparison(operator, leftVal.UnixNano(), rightVal.UnixNano())
	case left.Type() == object.TIME_OF_DAY_OBJ && right.Type() == object.TIME_OF_DAY_OBJ:
		leftVal := left.(*object.TimeOfDay).Value
		rightVal := right.(*object.TimeOfDay).Value
		leftNs := int64(leftVal.Hour())*int64(time.Hour) + int64(leftVal.Minute())*int64(time.Minute) + int64(leftVal.Second())*int64(time.Second) + int64(leftVal.Nanosecond())
		rightNs := int64(rightVal.Hour())*int64(time.Hour) + int64(rightVal.Minute())*int64(time.Minute) + int64(rightVal.Second())*int64(time.Second) + int64(rightVal.Nanosecond())
		return evalGenericComparison(operator, leftNs, rightNs)
	case left.Type() == object.DATE_AND_TIME_OBJ && right.Type() == object.DATE_AND_TIME_OBJ:
		leftVal := left.(*object.DateAndTime).Value
		rightVal := right.(*object.DateAndTime).Value
		return evalGenericComparison(operator, leftVal.UnixNano(), rightVal.UnixNano())
	case left.Type() != right.Type():
		return object.NewBuiltinError("type mismatch for comparison: %s %s %s", left.Type(), operator, right.Type())
	default:
		return object.NewBuiltinError("unsupported types for infix operation: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalGenericComparison[T ~string | ~int64](op string, leftVal, rightVal T) object.Object {
	switch op {
	case "=":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "<>":
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
		return object.NewBuiltinError("unknown operator '%s' for generic comparison", op)
	}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}
