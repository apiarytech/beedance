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

// The assignment attempt (IEC 61131-3 3rd edition), target ?= value, is
// target := $ATTEMPT(value, isRef, FB1, FB2, ...): the function blocks are
// those of the target's type (an interface they implement, or the function
// block a REF_TO refers to and those extending it), worked out here; the
// built-in checks the instance's class against them at run time.

import (
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
)

// attemptAssignment returns target ?= value as an ordinary assignment of
// $ATTEMPT's result.
func (c *Compiler) attemptAssignment(node *ast.AssignmentStatement) *ast.AssignmentStatement {
	declared := c.declaredVarType(node.Left)
	isRef := false
	typeName := ""
	if rt, ok := declared.(*ast.RefToType); ok {
		isRef = true
		typeName = c.flattenExpressionToString(rt.BaseType)
	} else if declared != nil {
		typeName = c.flattenExpressionToString(declared)
	}

	var names []string
	for _, n := range c.typeInfo {
		fb, ok := n.(*ast.FunctionBlockDeclaration)
		if !ok || typeName == "" || !c.fbIsA(fb, typeName) {
			continue
		}
		// Only a function block compiled with the program has instances.
		if _, compiled := c.symbolTable.Resolve(fb.Name.Value); compiled {
			names = append(names, fb.Name.Value)
		}
	}
	sort.Strings(names)

	tok := node.Token
	args := []ast.Expression{node.Value, &ast.Boolean{Token: tok, Value: isRef}}
	for _, n := range names {
		args = append(args, &ast.Identifier{Token: tok, Value: n})
	}
	assign := *node
	assign.Token = token.Token{Type: token.ASSIGN, Literal: ":=", Row: tok.Row, Column: tok.Column, Pos: tok.Pos}
	assign.Value = &ast.CallExpression{Token: tok, Function: &ast.Identifier{Token: tok, Value: "$ATTEMPT"}, Arguments: args}
	return &assign
}

// fbIsA reports whether instances of a function block are of the type
// typeName: the block or one it extends, or an interface one of those
// implements (or one such an interface extends).
func (c *Compiler) fbIsA(fb *ast.FunctionBlockDeclaration, typeName string) bool {
	var interfaceIs func(e ast.Expression, depth int) bool
	interfaceIs = func(e ast.Expression, depth int) bool {
		name := c.flattenExpressionToString(e)
		if strings.EqualFold(name, typeName) {
			return true
		}
		if depth > 16 {
			return false
		}
		if iface, ok := c.typeInfo[strings.ToUpper(name)].(*ast.InterfaceDeclaration); ok {
			for _, parent := range iface.Extends {
				if interfaceIs(parent, depth+1) {
					return true
				}
			}
		}
		return false
	}
	for depth := 0; fb != nil && depth < 16; depth++ {
		if strings.EqualFold(fb.Name.Value, typeName) {
			return true
		}
		for _, impl := range fb.Implements {
			if interfaceIs(impl, 0) {
				return true
			}
		}
		if fb.Extends == nil {
			return false
		}
		parent, _ := c.resolveTypeNode(fb.Extends)
		fb, _ = parent.(*ast.FunctionBlockDeclaration)
	}
	return false
}
