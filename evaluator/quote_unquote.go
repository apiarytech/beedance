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
	"beedance/ast"
	"beedance/object"
	"beedance/token"
	"fmt"
)

func quote(node ast.Node, env *object.Environment) object.Object {
	node = evalUnquoteCalls(node, env)
	return &object.Quote{Node: node}
}

func evalUnquoteCalls(quoted ast.Node, env *object.Environment) ast.Node {
	return ast.Modify(quoted, func(node ast.Node) ast.Node {
		if !isUnquoteCall(node) {
			return node
		}

		call, ok := node.(*ast.CallExpression)
		if !ok {
			return node
		}

		if len(call.Arguments) != 1 {
			return node
		}

		unquoted := Eval(call.Arguments[0], env)
		return convertObjectToASTNode(unquoted)
	})
}

func isUnquoteCall(node ast.Node) bool {
	callExpression, ok := node.(*ast.CallExpression)
	if !ok {
		return false
	}

	return callExpression.Function.TokenLiteral() == "unquote"
}

func convertObjectToASTNode(obj object.Object) ast.Node {
	switch obj := obj.(type) {
	case *object.LInt:
		t := token.Token{
			Type:    token.INT,
			Literal: fmt.Sprintf("%d", obj.Value),
		}
		return &ast.IntegerLiteral{Token: t, Value: obj.Value}

	case *object.Boolean:
		var t token.Token
		if obj.Value {
			t = token.Token{Type: token.TRUE, Literal: "true"}
		} else {
			t = token.Token{Type: token.FALSE, Literal: "false"}
		}
		return &ast.Boolean{Token: t, Value: obj.Value}

	case *object.Real:
		t := token.Token{
			Type:    token.REAL,
			Literal: fmt.Sprintf("%f", obj.Value),
		}
		return &ast.RealLiteral{Token: t, Value: obj.Value}

	case *object.BitString: // New: Handle BitString object
		var tokType token.TokenType
		switch obj.Width {
		case 8:
			tokType = token.BYTE
		case 16:
			tokType = token.WORD
		case 32:
			tokType = token.DWORD
		case 64:
			tokType = token.LWORD
		default:
			// If an unsupported width is encountered, return nil or an error AST node
			return nil
		}
		t := token.Token{
			Type:    tokType,
			Literal: obj.Inspect(), // Use Inspect to get the standard literal format
		}
		return &ast.BitStringLiteral{Token: t, Value: obj.Value, Width: obj.Width}

	case *object.Quote:
		return obj.Node

	default:
		return nil
	}
}
