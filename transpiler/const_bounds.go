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

// Go needs an array's length when it declares the array. An array bound or
// a string length given by a named constant, ARRAY[0..NMAX] or STRING(LEN),
// is written as the constant's value before code is generated.

import (
	"strconv"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
)

// inlineConstantBounds returns program with the named integer constants in
// array bounds and string lengths replaced by their values. The constants
// are those declared CONSTANT with an initial value, global or in a POU.
func inlineConstantBounds(program *ast.Program) *ast.Program {
	consts := map[string]ast.Expression{}
	collect := func(decls []*ast.VarDeclStatement) {
		for _, d := range decls {
			if d != nil && d.IsConstant && d.Value != nil && d.Name != nil {
				consts[strings.ToUpper(d.Name.Value)] = d.Value
			}
		}
	}
	for _, s := range program.Statements {
		switch d := s.(type) {
		case *ast.GlobalVarDeclaration:
			collect(d.Vars)
		case *ast.ProgramDeclaration:
			collect(d.Vars)
			for _, g := range d.VarGlobal {
				collect(g.Vars)
			}
		case *ast.FunctionBlockDeclaration:
			collect(d.Vars)
		case *ast.FunctionDeclaration:
			collect(d.Vars)
		}
	}
	if len(consts) == 0 {
		return program
	}
	var value func(e ast.Expression, depth int) (int64, bool)
	value = func(e ast.Expression, depth int) (int64, bool) {
		if depth > 16 {
			return 0, false
		}
		switch x := e.(type) {
		case *ast.Identifier:
			if c, ok := consts[strings.ToUpper(x.Value)]; ok {
				return value(c, depth+1)
			}
		case *ast.PrefixExpression:
			n, ok := value(x.Right, depth+1)
			switch x.Operator {
			case "-":
				return -n, ok
			case "+":
				return n, ok
			}
		case *ast.InfixExpression:
			l, okL := value(x.Left, depth+1)
			r, okR := value(x.Right, depth+1)
			if !okL || !okR {
				return 0, false
			}
			switch x.Operator {
			case "+":
				return l + r, true
			case "-":
				return l - r, true
			case "*":
				return l * r, true
			case "/":
				if r != 0 {
					return l / r, true
				}
			}
		default:
			return constantInteger(e)
		}
		return 0, false
	}
	literal := func(e ast.Expression) ast.Expression {
		if _, plain := constantInteger(e); plain {
			return e
		}
		n, ok := value(e, 0)
		if !ok {
			return e
		}
		text := strconv.FormatInt(n, 10)
		return &ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: text}, Value: n}
	}
	ast.Modify(program, func(n ast.Node) ast.Node {
		switch x := n.(type) {
		case *ast.ArrayDefinition:
			for _, r := range x.Ranges {
				if infix, ok := r.(*ast.InfixExpression); ok && infix.Operator == ".." {
					infix.Left = literal(infix.Left)
					infix.Right = literal(infix.Right)
				}
			}
		case *ast.VarDeclStatement:
			if x.StringLength != nil {
				x.StringLength = literal(x.StringLength)
			}
		}
		return n
	})
	return program
}
