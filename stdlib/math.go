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
	"math"
	"strings"
)

func init() {
	object.RegisterBuiltin(BuiltinSin, "SIN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for SIN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `SIN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Sin(val)}
	})

	object.RegisterBuiltin(BuiltinCos, "COS", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for COS. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `COS` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Cos(val)}
	})

	object.RegisterBuiltin(BuiltinTan, "TAN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for TAN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `TAN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Tan(val)}
	})

	object.RegisterBuiltin(BuiltinAsin, "ASIN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ASIN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `ASIN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val < -1.0 || val > 1.0 {
			return object.NewBuiltinError("argument to `ASIN` must be between -1 and 1, got %f", val)
		}
		return &object.Real{Value: math.Asin(val)}
	})

	object.RegisterBuiltin(BuiltinAcos, "ACOS", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ACOS. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `ACOS` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val < -1.0 || val > 1.0 {
			return object.NewBuiltinError("argument to `ACOS` must be between -1 and 1, got %f", val)
		}
		return &object.Real{Value: math.Acos(val)}
	})

	object.RegisterBuiltin(BuiltinAtan, "ATAN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ATAN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `ATAN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Atan(val)}
	})

	object.RegisterBuiltin(BuiltinAtan2, "ATAN2", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for ATAN2. got=%d, want=2", len(args))
		}
		y, okY := object.GetFloat64Value(args[0])
		if !okY {
			return object.NewBuiltinError("argument 1 to `ATAN2` must be INTEGER or REAL, got %s", args[0].Type())
		}
		x, okX := object.GetFloat64Value(args[1])
		if !okX {
			return object.NewBuiltinError("argument 2 to `ATAN2` must be INTEGER or REAL, got %s", args[1].Type())
		}
		return &object.Real{Value: math.Atan2(y, x)}
	})

	object.RegisterBuiltin(BuiltinLn, "LN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for LN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `LN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val <= 0 {
			return object.NewBuiltinError("argument to `LN` must be positive, got %f", val)
		}
		return &object.Real{Value: math.Log(val)}
	})

	object.RegisterBuiltin(BuiltinLog, "LOG", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for LOG. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `LOG` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val <= 0 {
			return object.NewBuiltinError("argument to `LOG` must be positive, got %f", val)
		}
		return &object.Real{Value: math.Log10(val)}
	})

	object.RegisterBuiltin(BuiltinExp, "EXP", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for EXP. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `EXP` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Exp(val)}
	})

	object.RegisterBuiltin(BuiltinSqrt, "SQRT", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for SQRT. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `SQRT` not supported, got %s", args[0].Type())
		}
		if val < 0 {
			return object.NewBuiltinError("argument to `SQRT` must be non-negative, got %f", val)
		}
		return &object.Real{Value: math.Sqrt(val)}
	})

	object.RegisterBuiltin(BuiltinRound, "ROUND", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ROUND. got=%d, want=1", len(args))
		}
		var val float64
		var ok bool
		switch arg := args[0].(type) {
		case *object.Real:
			val, ok = arg.Value, true
		case *object.LReal:
			val, ok = arg.Value, true
		default:
			ok = false
		}
		if !ok {
			return object.NewBuiltinError("argument to `ROUND` must be REAL, got %s", args[0].Type())
		}
		return &object.LInt{Value: int64(math.Round(val))}
	})

	object.RegisterBuiltin(BuiltinAbs, "ABS", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ABS. got=%d, want=1", len(args))
		}
		arg := args[0]
		switch v := arg.(type) {
		case *object.SInt:
			if v.Value < 0 {
				return &object.SInt{Value: -v.Value}
			}
			return v
		case *object.Int:
			if v.Value < 0 {
				return &object.Int{Value: -v.Value}
			}
			return v
		case *object.DInt:
			if v.Value < 0 {
				return &object.DInt{Value: -v.Value}
			}
			return v
		case *object.LInt:
			if v.Value < 0 {
				return &object.LInt{Value: -v.Value}
			}
			return v
		case *object.USInt, *object.UInt, *object.UDInt, *object.ULInt:
			return v
		case *object.Real:
			return &object.Real{Value: math.Abs(v.Value)}
		case *object.LReal:
			return &object.LReal{Value: math.Abs(v.Value)}
		default:
			return object.NewBuiltinError("argument to `ABS` not supported, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin(BuiltinTrunc, "TRUNC", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for TRUNC. got=%d, want=1", len(args))
		}
		var val float64
		var ok bool
		switch arg := args[0].(type) {
		case *object.Real:
			val, ok = arg.Value, true
		case *object.LReal:
			val, ok = arg.Value, true
		default:
			ok = false
		}
		if !ok {
			return object.NewBuiltinError("argument to `TRUNC` must be REAL, got %s", args[0].Type())
		}
		return &object.LInt{Value: int64(math.Trunc(val))}
	})

	object.RegisterBuiltin(BuiltinAdd, "ADD", addBuiltin)
	object.RegisterBuiltin(BuiltinSub, "SUB", subBuiltin)
	object.RegisterBuiltin(BuiltinMul, "MUL", mulBuiltin)
	object.RegisterBuiltin(BuiltinDiv, "DIV", divBuiltin)
	object.RegisterBuiltin(BuiltinMod, "MOD", modBuiltin)
	object.RegisterBuiltin(BuiltinExpt, "EXPT", exptBuiltin)

	object.RegisterBuiltin(BuiltinMin, "MIN", func(args ...object.Object) object.Object {
		return minMaxBuiltin("MIN", args...)
	})
	object.RegisterBuiltin(BuiltinMax, "MAX", func(args ...object.Object) object.Object {
		return minMaxBuiltin("MAX", args...)
	})
}

func addBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for ADD. got=%d, want=2", len(args))
	}
	left, right := args[0], args[1]
	result := object.EvalInfix(left, "+", right)
	if _, ok := result.(*object.Error); ok {
		return object.NewBuiltinError("unsupported argument types for ADD: %s + %s", left.Type(), right.Type())
	}
	return result
}

func subBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for SUB. got=%d, want=2", len(args))
	}
	left, right := args[0], args[1]
	result := object.EvalInfix(left, "-", right)
	if _, ok := result.(*object.Error); ok {
		return object.NewBuiltinError("unsupported argument types for SUB: %s - %s", left.Type(), right.Type())
	}
	return result
}

func mulBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for MUL. got=%d, want=2", len(args))
	}
	left, right := args[0], args[1]
	result := object.EvalInfix(left, "*", right)
	if _, ok := result.(*object.Error); ok {
		return object.NewBuiltinError("unsupported argument types for MUL: %s * %s", left.Type(), right.Type())
	}
	return result
}

func divBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for DIV. got=%d, want=2", len(args))
	}
	left, right := args[0], args[1]
	result := object.EvalInfix(left, "/", right)
	if err, ok := result.(*object.Error); ok {
		// Preserve "division by zero" error from EvalInfix
		if strings.Contains(err.Message, "division by zero") {
			return err
		}
		return object.NewBuiltinError("unsupported argument types for DIV: %s / %s", left.Type(), right.Type())
	}
	return result
}

func modBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for MOD. got=%d, want=2", len(args))
	}
	_, _, ok1 := object.GetIntegerObjectValue(args[0])
	_, _, ok2 := object.GetIntegerObjectValue(args[1])
	if !ok1 || !ok2 {
		return object.NewBuiltinError("arguments to `MOD` must be INTEGER, got %s and %s", args[0].Type(), args[1].Type())
	}

	return object.EvalIntegerInfix(args[0], "MOD", args[1])
}

func exptBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for EXPT. got=%d, want=2", len(args))
	}
	base, ok1 := object.GetFloat64Value(args[0])
	if !ok1 {
		return object.NewBuiltinError("argument 1 to `EXPT` must be numeric, got %s", args[0].Type())
	}
	exponent, ok2 := object.GetFloat64Value(args[1])
	if !ok2 {
		return object.NewBuiltinError("argument 2 to `EXPT` must be numeric, got %s", args[1].Type())
	}
	result := math.Pow(base, exponent)
	return &object.Real{Value: result}
}

func minMaxBuiltin(op string, args ...object.Object) object.Object {
	if len(args) == 0 {
		return object.NewBuiltinError("wrong number of arguments for %s. got=0, want>=1", op)
	}

	result := args[0]
	firstType := result.Type()
	isFirstNumeric := object.IsNumeric(result)

	// Check if the type is orderable by the logic in EvalInfix.
	switch firstType {
	case object.SINT_OBJ, object.INT_OBJ, object.DINT_OBJ, object.LINT_OBJ,
		object.USINT_OBJ, object.UINT_OBJ, object.UDINT_OBJ, object.ULINT_OBJ,
		object.REAL_OBJ, object.LREAL_OBJ, object.STRING_OBJ, object.WSTRING_OBJ,
		object.TIME_OBJ, object.DATE_OBJ, object.TIME_OF_DAY_OBJ, object.DATE_AND_TIME_OBJ:
		// These types are orderable.
	default:
		return object.NewBuiltinError("arguments to `%s` must be of an orderable elementary type, got %s", op, firstType)
	}

	for i := 1; i < len(args); i++ {
		nextArg := args[i]
		isNextNumeric := object.IsNumeric(nextArg)

		if isFirstNumeric {
			if !isNextNumeric {
				return object.NewBuiltinError("all arguments to `%s` must be INTEGER or REAL, got %s", op, nextArg.Type())
			}
		} else if nextArg.Type() != firstType {
			return object.NewBuiltinError("all arguments to `%s` must be of the same type, got %s and %s", op, firstType, nextArg.Type())
		}

		var comparisonOp string
		if op == "MIN" {
			comparisonOp = "<" // If nextArg < result, we'll update result
		} else { // MAX
			comparisonOp = ">" // If nextArg > result, we'll update result
		}

		var isCompareTrue object.Object
		// HACK: The stdlib doesn't have access to the full evaluator's infix logic.
		// We handle the types from the failing tests directly here to avoid a circular dependency.
		switch firstType {
		case object.STRING_OBJ:
			leftVal := nextArg.(*object.String).Value
			rightVal := result.(*object.String).Value
			var res bool
			if comparisonOp == "<" {
				res = leftVal < rightVal
			} else {
				res = leftVal > rightVal
			}
			isCompareTrue = nativeBoolToBooleanObject(res)
		case object.TIME_OBJ:
			leftVal := nextArg.(*object.Time).Value
			rightVal := result.(*object.Time).Value
			var res bool
			if comparisonOp == "<" {
				res = leftVal < rightVal
			} else {
				res = leftVal > rightVal
			}
			isCompareTrue = nativeBoolToBooleanObject(res)
		default:
			isCompareTrue = object.EvalInfix(nextArg, comparisonOp, result)
		}

		if err, ok := isCompareTrue.(*object.Error); ok {
			return err
		}

		if boolResult, ok := isCompareTrue.(*object.Boolean); ok && boolResult.Value {
			result = nextArg
		}
	}

	return result
}
