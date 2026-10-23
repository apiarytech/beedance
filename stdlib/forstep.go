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

import "github.com/apiarytech/beedance/object"

// $FORSTEP is a FOR loop's step on the VM: the compiler writes BY s as
// $FORSTEP(s) when s is not a constant other than 0. It returns s, or, for a
// step of 0, an error, which the VM raises: FOR ... BY 0 never ends, so it is
// a runtime error on every engine (the owner's choice, issue #15).

func init() {
	object.RegisterBuiltin(BuiltinForStep, "$FORSTEP", forStepFn)
}

func forStepFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("$FORSTEP: got %d arguments, want 1", len(args))
	}
	if n, _, ok := object.GetIntegerObjectValue(args[0]); ok && n == 0 {
		return object.NewBuiltinError("FOR loop step (BY) is 0")
	}
	return args[0]
}
