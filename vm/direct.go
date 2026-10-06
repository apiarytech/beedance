/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// directZero is the starting value of an address in the I/O image nothing
// has set yet: its size's type's default for a directly represented
// variable (ast.DirectType), NULL for anything else.
func directZero(address string) object.Object {
	typ, ok := ast.DirectType(address)
	if !ok {
		return Null
	}
	switch typ {
	case "BYTE":
		return &object.Byte{}
	case "WORD":
		return &object.Word{}
	case "DWORD":
		return &object.DWord{}
	case "LWORD":
		return &object.LWord{}
	}
	return False
}

// asBitString is object.AsBitString: I/O values compute as bit strings.
func asBitString(o object.Object) object.Object { return object.AsBitString(o) }
