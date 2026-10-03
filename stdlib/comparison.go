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
	"github.com/apiarytech/beedance/object"
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

		left, right := args[0], args[1]
		result := object.EvalInfix(left, symbolicOp, right)

		// If EvalInfix returns a generic "unsupported operator" error, replace it
		// with a more specific "type mismatch for comparison" error. This can happen
		// when comparing incompatible types like TIME and INT.
		if err, ok := result.(*object.Error); ok {
			if strings.Contains(err.Message, "unsupported operator") {
				return object.NewBuiltinError("type mismatch for comparison: %s %s %s", left.Type(), symbolicOp, right.Type())
			}
			// Propagate other errors as-is.
			return err
		}
		return result
	}
}

// nativeBoolToBooleanObject is a helper to avoid depending on the evaluator's singletons.
func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}
