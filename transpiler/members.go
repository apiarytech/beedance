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

// This file finds the declarations of members, x.member, wherever x comes
// from: a local or global variable, an element of an array, or another
// member; and whatever declares the member: a structure, named or declared
// in place, or a function block.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// valueDataType returns the data type an expression's value was declared
// with, or nil when it is not known.
func (t *Transpiler) valueDataType(exp ast.Expression) ast.Expression {
	switch e := exp.(type) {
	case *ast.Identifier:
		if td := t.varInfo[e.Value]; td != nil {
			return td.DataType
		}
		// A function's result variable.
		if t.currentFunc != nil && strings.EqualFold(t.currentFunc.Name.Value, e.Value) {
			return t.currentFunc.ReturnType
		}
		if decl := t.globalDecl(e.Value); decl != nil {
			return decl.DataType
		}
	case *ast.IndexExpression:
		// An element of an array, indexed in all its dimensions. A scope
		// may record an array variable by its element type already.
		dataType := t.valueDataType(e.Left)
		if def := t.arrayDefinitionOf(dataType); def != nil {
			return def.DataType
		}
		return dataType
	case *ast.MemberAccessExpression:
		if decl := t.memberDecl(e.Struct, e.Member.Value); decl != nil {
			return decl.DataType
		}
	}
	return nil
}

// memberDecl returns the declaration of member in the structure or function
// block value owner, ignoring case, or nil.
func (t *Transpiler) memberDecl(owner ast.Expression, member string) *ast.VarDeclStatement {
	dataType := t.valueDataType(owner)
	for depth := 0; dataType != nil && depth < 16; depth++ {
		switch dt := dataType.(type) {
		case *ast.StructDefinition:
			return decl0(dt.Members, member)
		}
		if fb := t.lookupFunctionBlock(dataType); fb != nil {
			return t.findFunctionBlockVar(fb, member)
		}
		td := t.lookupTypeDeclaration(dataType)
		if td == nil {
			return nil
		}
		dataType = td.DataType
	}
	return nil
}

// inputGoType returns the elementary Go type of an input of the function
// block instance fb, so that the value passed to it is converted, or "".
func (t *Transpiler) inputGoType(fb ast.Expression, input *ast.Identifier) string {
	return t.exprGoType(&ast.MemberAccessExpression{Struct: fb, Member: input})
}

// globalDecl returns the declaration of a global variable, or nil.
func (t *Transpiler) globalDecl(name string) *ast.VarDeclStatement {
	for _, decl := range t.globalDecls {
		if decl.Name.Value == name {
			return decl
		}
	}
	for _, decl := range t.globalDecls {
		if strings.EqualFold(decl.Name.Value, name) {
			return decl
		}
	}
	return nil
}
