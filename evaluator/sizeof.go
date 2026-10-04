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

// SIZEOF(x) is the size of x in bytes (see object.SizeOfType), from x's
// declaration as the compiler computes it. Without a declaration (a
// VAR_IN_OUT passed in, a function's parameter), the value is measured:
// exact for bit strings and arrays and structures of them, while a number
// held in a 64-bit value counts by its runtime type and a STRING by its
// current length.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// isSizeofCall reports whether call is SIZEOF(x), unless the program
// declares a function of that name.
func isSizeofCall(call *ast.CallExpression, env *object.Environment) bool {
	ident, ok := call.Function.(*ast.Identifier)
	if !ok || !strings.EqualFold(ident.Value, "SIZEOF") || len(call.Arguments) != 1 {
		return false
	}
	_, declared := env.Get("_function_" + ident.Value)
	return !declared
}

// sizeResolver gives object.SizeOfType the constants and types of env.
func sizeResolver(env *object.Environment) object.SizeResolver {
	return object.SizeResolver{
		ConstInt: func(e ast.Expression) (int64, error) {
			v := Eval(e, env)
			if c, ok := v.(*object.Constant); ok {
				v = c.Value
			}
			if n, _, ok := object.GetIntegerObjectValue(v); ok {
				return n, nil
			}
			return 0, fmt.Errorf("%s is not a constant integer", e.String())
		},
		Type: func(name string) (*ast.TypeDeclaration, bool) { return lookupTypeDeclaration(name, env) },
	}
}

// evalSizeof returns SIZEOF(x) as an integer.
func evalSizeof(call *ast.CallExpression, env *object.Environment) object.Object {
	arg := call.Arguments[0]
	r := sizeResolver(env)
	var decl *ast.VarDeclStatement
	switch x := arg.(type) {
	case *ast.TypeSpecifier:
		if n, ok := object.ElementarySize(x.Token.Literal); ok {
			return &object.LInt{Value: n}
		}
	case *ast.Identifier:
		decl, _ = env.Declaration(x.Value)
		if decl == nil {
			if n, ok := object.ElementarySize(x.Value); ok {
				return &object.LInt{Value: n} // a type name, such as LREAL
			}
		}
	case *ast.MemberAccessExpression:
		if id, ok := x.Struct.(*ast.Identifier); ok {
			if owner, ok := env.Declaration(id.Value); ok {
				decl = object.StructMember(owner.DataType, x.Member.Value, r.Type)
			}
		}
	}
	if decl != nil {
		if n, err := object.SizeOfDeclaration(decl, r); err == nil {
			return &object.LInt{Value: n}
		}
	}
	v := Eval(arg, env)
	if isError(v) {
		return v
	}
	n, ok := sizeOfValue(v)
	if !ok {
		return newError(call, "SIZEOF(%s): cannot tell the size of a %s", arg.String(), v.Type())
	}
	return &object.LInt{Value: n}
}

// objectSizes are the sizes in bytes of the runtime types.
var objectSizes = map[object.ObjectType]int64{
	object.BOOLEAN_OBJ: 1, object.SINT_OBJ: 1, object.USINT_OBJ: 1,
	object.INT_OBJ: 2, object.UINT_OBJ: 2,
	object.DINT_OBJ: 4, object.UDINT_OBJ: 4, object.REAL_OBJ: 4,
	object.TIME_OBJ: 4, object.DATE_OBJ: 4, object.TIME_OF_DAY_OBJ: 4, object.DATE_AND_TIME_OBJ: 4,
	object.LINT_OBJ: 8, object.ULINT_OBJ: 8, object.LREAL_OBJ: 8,
}

// sizeOfValue returns the size of a value in bytes.
func sizeOfValue(v object.Object) (int64, bool) {
	switch x := v.(type) {
	case *object.Constant:
		return sizeOfValue(x.Value)
	case *object.BitString:
		return int64(x.Width / 8), true
	case *object.String:
		return int64(len(x.Value)) + 1, true
	case *object.WString:
		return 2 * (int64(len([]rune(x.Value))) + 1), true
	case *object.Array:
		var total int64
		for _, e := range x.Elements {
			n, ok := sizeOfValue(e)
			if !ok {
				return 0, false
			}
			total += n
		}
		return total, true
	case *object.Hash:
		// A structure: its members, without the evaluator's own keys.
		var total int64
		for _, p := range x.Pairs {
			if k, ok := p.Key.(*object.String); ok && strings.HasPrefix(k.Value, "__") {
				continue
			}
			n, ok := sizeOfValue(p.Value)
			if !ok {
				return 0, false
			}
			total += n
		}
		return total, true
	}
	n, ok := objectSizes[v.Type()]
	return n, ok
}
