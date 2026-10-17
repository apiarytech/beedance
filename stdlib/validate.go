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

// The validate functions (IEC 61131-3 3rd edition, Table 39): IS_VALID of a
// real is FALSE for an infinity or a NaN, and IS_VALID_BCD of a bit string
// is FALSE if a digit is more than 9.

import (
	"math"

	"github.com/apiarytech/beedance/object"
)

func init() {
	object.RegisterBuiltin(BuiltinIsValid, "IS_VALID", isValidFn)
	object.RegisterBuiltin(BuiltinIsValidBcd, "IS_VALID_BCD", isValidBCDFn)
}

var isValidFn = func(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("wrong number of arguments for IS_VALID. got=%d, want=1", len(args))
	}
	switch args[0].(type) {
	case *object.Real, *object.LReal:
	default:
		return object.NewBuiltinError("argument to `IS_VALID` must be a REAL or LREAL, got %s", args[0].Type())
	}
	f, _ := object.GetFloat64Value(args[0])
	return &object.Boolean{Value: !math.IsNaN(f) && !math.IsInf(f, 0)}
}

var isValidBCDFn = func(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("wrong number of arguments for IS_VALID_BCD. got=%d, want=1", len(args))
	}
	var v uint64
	if b, ok := args[0].(*object.BitString); ok {
		v = b.Value
	} else if n, _, ok := object.GetIntegerObjectValue(args[0]); ok && n >= 0 {
		v = uint64(n)
	} else {
		return object.NewBuiltinError("argument to `IS_VALID_BCD` must be a bit string, got %s", args[0].Type())
	}
	for ; v != 0; v >>= 4 {
		if v&0xF > 9 {
			return &object.Boolean{Value: false}
		}
	}
	return &object.Boolean{Value: true}
}
