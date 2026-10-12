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
)

func init() {
	// Every type conversion X_TO_Y is one generic built-in: the compiler
	// calls $CONVERT(value, 'X', 'Y'), and the evaluator looks X_TO_Y up by
	// name (object.GetBuiltinByName). Registering the 552 pairs one by one
	// took most of a microcontroller's heap.
	object.RegisterBuiltin(BuiltinConvert, object.ConvertBuiltinName, object.ConvertBuiltin)
}
