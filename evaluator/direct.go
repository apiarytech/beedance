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

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// Directly represented variables used in statements (`level := %IW0;`,
// `%QX0.1 := run;`), in the I/O map by address as located variables are,
// with the type their size gives (ast.DirectType); the compiler's
// direct.go does the same for the VM.

// evalDirectRead reads a direct variable; one the host has not set reads as
// its type's default.
func evalDirectRead(node *ast.DirectVariable) object.Object {
	addr := node.FullAddress()
	typ, ok := ast.DirectType(addr)
	if !ok {
		return newError(node, "%s is not a directly represented variable", addr)
	}
	ioTypes[addr] = typ
	if v, ok := ioMap[addr]; ok {
		return object.AsBitString(v)
	}
	return directZero(typ)
}

// evalDirectWrite writes a direct variable; an input (%I) is read-only.
func evalDirectWrite(at ast.Node, node *ast.DirectVariable, val object.Object) object.Object {
	addr := node.FullAddress()
	typ, ok := ast.DirectType(addr)
	if !ok {
		return newError(at, "%s is not a directly represented variable", addr)
	}
	if ast.IsInputAddress(addr) {
		return newError(at, "%s is an input: a program reads it but cannot write it", addr)
	}
	ioTypes[addr] = typ
	ioMap[addr] = val
	return val
}

// directZero is a type's default as the evaluator holds it: a bit string
// of the type's width (the VM uses its own BYTE..LWORD objects).
func directZero(typ string) object.Object {
	if width, ok := object.GetBitStringWidth(typ); ok && typ != "BOOL" {
		return &object.BitString{Value: 0, Width: width}
	}
	return FALSE
}
