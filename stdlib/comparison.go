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
	"strings"
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
	object.RegisterBuiltin(BuiltinGreat, "GREAT", comparisonBuiltin("GREAT"))
}

func comparisonBuiltin(op string) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for %s. got=%d, want=2", op, len(args))
		}
		opMap := map[string]string{"GT": ">", "GE": ">=", "EQ": "=", "LE": "<=", "LT": "<", "NE": "<>", "GREAT": ">"}
		symbolicOp := opMap[op]
		result := object.EvalInfix(args[0], symbolicOp, args[1])
		// If EvalInfix returns a generic "unsupported operator" error, replace it
		// with a more specific "type mismatch for comparison" error. This can happen
		// when comparing incompatible types like TIME and INT.
		if err, ok := result.(*object.Error); ok {
			if strings.Contains(err.Message, "unsupported operator") {
				return object.NewBuiltinError("type mismatch for comparison: %s %s %s", args[0].Type(), symbolicOp, args[1].Type())
			}
		}
		return result
	}
}
