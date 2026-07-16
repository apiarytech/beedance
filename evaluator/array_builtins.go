/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package evaluator

import "beedance/object"

func init() {
	builtins["LOWER_BOUND"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for LOWER_BOUND. got=%d, want=2", len(args))
			}
			// The first argument must be an array instance.
			if _, ok := args[0].(*object.Array); !ok {
				return newBuiltinError("argument 1 to `LOWER_BOUND` must be of type ARRAY, got %s", args[0].Type())
			}
			// The second argument is the dimension. IEC standard is 1-based.
			dim, _, ok := getIntegerObjectValue(args[1])
			if !ok {
				return newBuiltinError("argument 2 to `LOWER_BOUND` must be of type INT, got %s", args[1].Type())
			}

			// For a simple 1D array as implemented, the dimension is always 1 and the lower bound is 0.
			if dim != 1 {
				return newBuiltinError("invalid dimension %d for 1D array", dim)
			}

			return &object.LInt{Value: 0} // In this implementation, arrays are 0-indexed.
		},
	}

	builtins["UPPER_BOUND"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for UPPER_BOUND. got=%d, want=2", len(args))
			}
			arr, ok := args[0].(*object.Array)
			if !ok {
				return newBuiltinError("argument 1 to `UPPER_BOUND` must be of type ARRAY, got %s", args[0].Type())
			}
			dim, _, ok := getIntegerObjectValue(args[1])
			if !ok {
				return newBuiltinError("argument 2 to `UPPER_BOUND` must be of type INT, got %s", args[1].Type())
			}

			if dim != 1 {
				return newBuiltinError("invalid dimension %d for 1D array", dim)
			}

			// The upper bound is the number of elements minus 1.
			upper := int64(len(arr.Elements) - 1)
			return &object.LInt{Value: upper}
		},
	}
}
