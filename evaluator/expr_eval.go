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

// quote is the entry point for handling the `EXPR` built-in. It takes an AST
// node, processes any nested `EVAL` calls within it, and returns the resulting
// node wrapped in a `Quote` object. This prevents the evaluator from executing
// the code, treating it as a data structure instead.
func quote(node ast.Node, env *object.Environment) object.Object {
	node = evalUnquoteCalls(node, env)
	return &object.Quote{Node: node}
}

// evalUnquoteCalls traverses a given AST node (`quoted`) and searches for `EVAL`
// calls. When an `EVAL` call is found, it evaluates the argument of the call and
// replaces the `EVAL` call site with the resulting AST node. This is the mechanism
// that allows for injecting computed values into a quoted AST.
func evalUnquoteCalls(quoted ast.Node, env *object.Environment) ast.Node {
	return ast.Modify(quoted, func(node ast.Node) ast.Node {
		if !isEvalCall(node) {
			return node
		}

		// isEvalCall ensures this is a *ast.CallExpression, so a direct assertion is safe.
		call := node.(*ast.CallExpression)
		if len(call.Arguments) != 1 {
			return node
		}

		unquoted := Eval(call.Arguments[0], env)
		return convertObjectToASTNode(unquoted)
	})
}

// isEvalCall is a helper function that checks if a given AST node is a
// `CallExpression` to the `EVAL` function.
func isEvalCall(node ast.Node) bool {
	callExpression, ok := node.(*ast.CallExpression)
	if !ok {
		return false
	}

	return callExpression.Function.TokenLiteral() == "EVAL"
}

// convertObjectToASTNode converts a runtime `object.Object` back into its
// `ast.Node` representation. This is a critical part of the `EVAL` mechanism,
// as it allows the result of an evaluated expression to be re-inserted into the
// AST. It handles various object types like integers, booleans, and even other quotes.
func convertObjectToASTNode(obj object.Object) ast.Node {
	switch obj := obj.(type) {
	case *object.SInt:
		t := token.Token{Type: token.INT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.IntegerLiteral{Token: t, Value: int64(obj.Value)}
	case *object.Int:
		t := token.Token{Type: token.INT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.IntegerLiteral{Token: t, Value: int64(obj.Value)}
	case *object.DInt:
		t := token.Token{Type: token.INT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.IntegerLiteral{Token: t, Value: int64(obj.Value)}
	case *object.LInt:
		t := token.Token{
			Type:    token.INT,
			Literal: fmt.Sprintf("%d", obj.Value),
		}
		return &ast.IntegerLiteral{Token: t, Value: obj.Value}
	case *object.USInt:
		t := token.Token{Type: token.UINT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.UnsignedIntegerLiteral{Token: t, Value: uint64(obj.Value)}
	case *object.UInt:
		t := token.Token{Type: token.UINT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.UnsignedIntegerLiteral{Token: t, Value: uint64(obj.Value)}
	case *object.UDInt:
		t := token.Token{Type: token.UINT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.UnsignedIntegerLiteral{Token: t, Value: uint64(obj.Value)}
	case *object.ULInt:
		t := token.Token{Type: token.UINT, Literal: fmt.Sprintf("%d", obj.Value)}
		return &ast.UnsignedIntegerLiteral{Token: t, Value: obj.Value}

	case *object.Boolean:
		var t token.Token
		if obj.Value {
			t = token.Token{Type: token.TRUE, Literal: "TRUE"}
		} else {
			t = token.Token{Type: token.FALSE, Literal: "FALSE"}
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

	case *object.String:
		t := token.Token{
			Type:    token.STRING_LITERAL,
			Literal: obj.Value,
		}
		return &ast.StringLiteral{Token: t, Value: obj.Value}

	case *object.WString:
		t := token.Token{
			Type:    token.WSTRING_LITERAL,
			Literal: obj.Value,
		}
		return &ast.WStringLiteral{Token: t, Value: obj.Value}

	case *object.Time:
		t := token.Token{Type: token.TIME, Literal: obj.Inspect()}
		return &ast.Identifier{Token: t, Value: obj.Inspect()}

	case *object.Date:
		t := token.Token{Type: token.DATE, Literal: obj.Inspect()}
		return &ast.Identifier{Token: t, Value: obj.Inspect()}

	case *object.TimeOfDay:
		t := token.Token{Type: token.TIME_OF_DAY, Literal: obj.Inspect()}
		return &ast.Identifier{Token: t, Value: obj.Inspect()}

	case *object.Quote:
		return obj.Node

	default:
		// If an object type cannot be converted back to an AST node (e.g., a function object),
		// return nil. This will likely cause an error further up the call stack, which is the desired behavior.
		// It's better to return an error node or nil than to panic.
		// Returning nil will likely cause an error further up the call stack, which is acceptable.
		return nil
	}
}
