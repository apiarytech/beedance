/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

// SIZEOF(x) is the size of x in bytes, as CODESYS and TwinCAT give it (see
// object.SizeOfType). The compiler knows it from x's declaration, so a call
// compiles to a constant.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// isSizeofCall reports whether call is SIZEOF(x), unless the program
// declares a function of that name.
func (c *Compiler) isSizeofCall(call *ast.CallExpression) bool {
	ident, ok := call.Function.(*ast.Identifier)
	if !ok || !strings.EqualFold(ident.Value, "SIZEOF") || len(call.Arguments) != 1 {
		return false
	}
	_, declared := c.resolveTypeName(ident.Value)
	return !declared
}

// compileSizeof compiles SIZEOF(x) to the constant size of x.
func (c *Compiler) compileSizeof(call *ast.CallExpression) error {
	n, err := c.sizeOfExpression(call.Arguments[0])
	if err != nil {
		return fmt.Errorf("SIZEOF(%s): %w", call.Arguments[0].String(), err)
	}
	c.emitConstant(c.addConstant(&object.LInt{Value: n}))
	return nil
}

// sizeResolver gives object.SizeOfType the compiler's constants and types.
func (c *Compiler) sizeResolver() object.SizeResolver {
	return object.SizeResolver{
		ConstInt: c.evaluateConstantInteger,
		Type: func(name string) (*ast.TypeDeclaration, bool) {
			node, ok := c.resolveTypeName(name)
			if !ok {
				return nil, false
			}
			td, ok := node.(*ast.TypeDeclaration)
			return td, ok
		},
	}
}

// sizeOfExpression returns the size of a variable, a member, an element or
// a type name.
func (c *Compiler) sizeOfExpression(e ast.Expression) (int64, error) {
	r := c.sizeResolver()
	switch x := e.(type) {
	case *ast.Identifier:
		if decl := c.declarationOf(x.Value); decl != nil {
			return object.SizeOfDeclaration(decl, r)
		}
		return object.SizeOfType(x, nil, r)
	case *ast.TypeSpecifier:
		return object.SizeOfType(x, nil, r)
	case *ast.IndexExpression:
		def, ok := c.arrayDefinitionOf(x.Left)
		if !ok {
			return 0, fmt.Errorf("cannot tell the type of %s", x.Left.String())
		}
		return object.SizeOfType(def.DataType, nil, r)
	case *ast.MemberAccessExpression:
		if decl := c.memberDeclaration(x); decl != nil {
			return object.SizeOfDeclaration(decl, r)
		}
	}
	return 0, fmt.Errorf("cannot tell the size of %s", e.String())
}

// declarationOf returns the declaration of a variable visible here: in the
// scopes from the innermost out, or in the current function block's chain.
func (c *Compiler) declarationOf(name string) *ast.VarDeclStatement {
	for i := c.scopeIndex; i >= 0; i-- {
		for n, decl := range c.scopes[i].varDecls {
			if strings.EqualFold(n, name) {
				return decl
			}
		}
	}
	if c.currentFB != nil {
		if decl, _ := c.findVarDeclOnFBChain(c.currentFB, name); decl != nil {
			return decl
		}
	}
	return nil
}

// arrayDefinitionOf returns the array definition of a declared array.
func (c *Compiler) arrayDefinitionOf(e ast.Expression) (*ast.ArrayDefinition, bool) {
	var decl *ast.VarDeclStatement
	switch x := e.(type) {
	case *ast.Identifier:
		decl = c.declarationOf(x.Value)
	case *ast.MemberAccessExpression:
		decl = c.memberDeclaration(x)
	}
	if decl == nil {
		return nil, false
	}
	def, ok := decl.DataType.(*ast.ArrayDefinition)
	return def, ok
}

// memberDeclaration returns the declaration of a structure's member.
func (c *Compiler) memberDeclaration(m *ast.MemberAccessExpression) *ast.VarDeclStatement {
	id, ok := m.Struct.(*ast.Identifier)
	if !ok {
		return nil
	}
	owner := c.declarationOf(id.Value)
	if owner == nil {
		return nil
	}
	return object.StructMember(owner.DataType, m.Member.Value, c.sizeResolver().Type)
}
