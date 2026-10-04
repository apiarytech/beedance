/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// This file transpiles references: REF_TO <type>, or POINTER TO <type>,
// which beedance treats as the same typed reference, is a Go pointer *T;
// REF(x) and ADR(x) are &x, r^ is *r and NULL is nil. A reference to an
// array is a slice instead, as POINTER TO ARRAY[0..32000] OF REAL refers to
// arrays of any length: ADR(a) is a[:], and pt^[i] indexes the slice from
// the lower bound of the array type pt refers to.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// refToGoType returns the Go type of a reference type.
func (t *Transpiler) refToGoType(rt *ast.RefToType) string {
	if def := t.arrayDefinitionOf(rt.BaseType); def != nil {
		inner := t.mapIecTypeToGo(def.DataType)
		if len(def.Ranges) > 1 {
			// The other dimensions keep their lengths: ARRAY[0..9, 1..3]
			// is a slice of [3] arrays.
			rest := *def
			rest.Ranges = def.Ranges[1:]
			inner = t.mapIecTypeToGo(&rest)
		}
		return "[]" + inner
	}
	return "*" + t.mapIecTypeToGo(rt.BaseType)
}

// refToArray returns the array type the reference ref is declared to refer
// to, or nil when ref is not a reference to an array.
func (t *Transpiler) refToArray(ref ast.Expression) *ast.ArrayDefinition {
	rt, ok := t.valueDataType(ref).(*ast.RefToType)
	if !ok {
		return nil
	}
	return t.arrayDefinitionOf(rt.BaseType)
}

// isNull reports whether an identifier is NULL, and not a variable named so.
func (t *Transpiler) isNull(ident *ast.Identifier) bool {
	if !strings.EqualFold(ident.Value, "NULL") {
		return false
	}
	name := ident.Value
	return !(t.localVars[name] || t.varInfo[name] != nil || t.globalVars[name] || t.tempVars[name] ||
		t.inOutVars[name] || t.accessVars[name] || t.lookupType(name) != nil)
}

// isReferenceCall reports whether a call is REF(x) or ADR(x), unless the
// program declares a function of that name.
func (t *Transpiler) isReferenceCall(call *ast.CallExpression) bool {
	ident, ok := call.Function.(*ast.Identifier)
	if !ok {
		return false
	}
	switch strings.ToUpper(ident.Value) {
	case "REF", "ADR":
		return t.lookupType(ident.Value) == nil
	}
	return false
}

// transpileReference transpiles REF(x) or ADR(x): &x, or x[:] for an array.
func (t *Transpiler) transpileReference(call *ast.CallExpression) error {
	if len(call.Arguments) != 1 {
		return fmt.Errorf("%s takes one argument, got %d", strings.ToUpper(call.Function.String()), len(call.Arguments))
	}
	target := call.Arguments[0]
	if deref, ok := target.(*ast.DereferenceExpression); ok {
		return t.transpileExpression(deref.Pointer) // REF(r^) is r itself
	}
	if t.arrayDefinitionOf(t.valueDataType(target)) != nil {
		t.write("(")
		if err := t.transpileExpression(target); err != nil {
			return err
		}
		t.write(")[:]")
		return nil
	}
	t.write("&")
	return t.transpileExpression(target)
}
