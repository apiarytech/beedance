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
	"fmt"
	"github.com/apiarytech/beedance/object"
)

func init() {
	// Generic Type Conversions
	types := []string{"BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT", "REAL", "LREAL", "TIME", "DATE", "TOD", "DT", "STRING", "WSTRING", "BYTE", "WORD", "DWORD", "LWORD", "ANY_INT", "ANY_REAL", "BCD"}
	for _, from := range types {
		for _, to := range types {
			if from == to {
				continue
			}
			funcName := fmt.Sprintf("%s_TO_%s", from, to)
			index, ok := BuiltinNameToIndex[funcName]
			if !ok {
				// This indicates a mismatch between the generated function names and the iota constants.
				// It should not happen if indices.go and this loop are in sync.
				panic("missing builtin index for: " + funcName)
			}
			object.RegisterBuiltin(index, funcName, object.GenericConversionBuiltin(from, to).Fn)
		}
	}
}
