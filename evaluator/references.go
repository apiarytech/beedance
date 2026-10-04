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

// This file evaluates references: variables declared REF_TO <type>, or
// POINTER TO <type>, which beedance treats as the same typed reference.
// REF(x) and ADR(x) refer to a variable, r^ is the variable r refers to,
// and NULL refers to nothing. There is no pointer arithmetic: a reference
// refers to a whole variable, an array element or a member, never to bytes.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// referenceBuiltins are the operators this file evaluates, unless a program
// declares a function of the same name.
var referenceBuiltins = map[string]bool{"REF": true, "ADR": true}

// evalReferenceCall evaluates a call of REF or ADR, and reports whether the
// call is one.
func evalReferenceCall(node *ast.CallExpression, env *object.Environment) (object.Object, bool) {
	ident, ok := node.Function.(*ast.Identifier)
	if !ok || !referenceBuiltins[strings.ToUpper(ident.Value)] {
		return nil, false
	}
	if _, declared := env.Get("_function_" + ident.Value); declared {
		return nil, false
	}
	if _, declared := env.Get(ident.Value); declared {
		return nil, false
	}
	if len(node.Arguments) != 1 {
		return newError(node, "%s takes one argument, got %d", strings.ToUpper(ident.Value), len(node.Arguments)), true
	}
	return makeReference(node, node.Arguments[0], env), true
}

// makeReference returns a reference to the variable target names: a
// variable, an array element or a member of a structure or function block
// instance.
func makeReference(node ast.Node, target ast.Expression, env *object.Environment) object.Object {
	switch t := target.(type) {
	case *ast.Identifier:
		scope, name, ok := env.Owner(t.Value)
		if !ok {
			return newError(node, "identifier not found: %s", t.Value)
		}
		raw, _ := scope.GetRaw(name)
		// A VAR_IN_OUT, or a located variable, stands for another variable.
		if ptr, isPtr := raw.(*object.Pointer); isPtr {
			if ptr.Env == nil {
				return &object.Reference{IO: ioMap, Name: ptr.Name}
			}
			return &object.Reference{Env: ptr.Env, Name: ptr.Name}
		}
		return &object.Reference{Env: scope, Name: name}
	case *ast.IndexExpression:
		container := Eval(t.Left, env)
		if isError(container) {
			return container
		}
		index := Eval(t.Index, env)
		if isError(index) {
			return index
		}
		ref, err := object.NewElementReference(container, index)
		if err != nil {
			return newError(node, "%s", err)
		}
		return ref
	case *ast.MemberAccessExpression:
		owner := Eval(t.Struct, env)
		if isError(owner) {
			return owner
		}
		switch o := owner.(type) {
		case *object.FunctionBlockInstance:
			if _, ok := o.Env.Get(t.Member.Value); !ok {
				return newError(node, "%s has no member '%s'", t.Struct.String(), t.Member.Value)
			}
			scope, name, _ := o.Env.Owner(t.Member.Value)
			return &object.Reference{Env: scope, Name: name}
		case *object.Hash:
			key, ok := hashMemberKey(o, t.Member.Value)
			if !ok {
				return newError(node, "structure has no member '%s'", t.Member.Value)
			}
			ref, err := object.NewElementReference(o, o.Pairs[key].Key)
			if err != nil {
				return newError(node, "%s", err)
			}
			return ref
		}
		return newError(node, "REF needs a variable; %s is a %s", t.String(), owner.Type())
	case *ast.DereferenceExpression:
		// REF(r^) is r itself.
		return Eval(t.Pointer, env)
	}
	return newError(node, "REF needs a variable, got %s", target.String())
}

// isRefToType reports whether a declared type is a reference type.
func isRefToType(dataType ast.Expression) (*ast.RefToType, bool) {
	rt, ok := dataType.(*ast.RefToType)
	return rt, ok
}

// nullReference returns the NULL value of a reference type: a reference to
// nothing that keeps the view of the type it refers to.
func nullReference(rt *ast.RefToType, env *object.Environment) *object.Reference {
	ref := &object.Reference{}
	if def, ok := rt.BaseType.(*ast.ArrayDefinition); ok {
		ref.Lower = declaredLowerBounds(def, env)
	}
	return ref
}

// bindReference gives val, a value for a variable declared with the
// reference type rt, that type's view; val must be a reference or NULL.
func bindReference(node ast.Node, name string, rt *ast.RefToType, val object.Object, env *object.Environment) object.Object {
	switch val.(type) {
	case *object.Reference, *object.Null:
		return nullReference(rt, env).Retarget(val)
	}
	return newError(node, "%s is declared %s; it takes REF(), ADR() or NULL, not a %s", name, rt.String(), val.Type())
}

// findParamDecl returns the declaration of the parameter name of a function,
// function block or program.
func findParamDecl(def object.Object, name string) *ast.VarDeclStatement {
	for _, d := range getParamDecls(def) {
		if strings.EqualFold(d.Name.Value, name) {
			return d
		}
	}
	return nil
}

// bindParameter returns the value an input parameter declared decl takes
// for the argument val: a reference takes the view of its declared type.
func bindParameter(node ast.Node, decl *ast.VarDeclStatement, val object.Object, env *object.Environment) object.Object {
	if decl != nil {
		if rt, ok := isRefToType(decl.DataType); ok {
			return bindReference(node, decl.Name.Value, rt, val, env)
		}
	}
	return object.CopyValue(val)
}

// evalReferenceDeref returns the variable a reference refers to, seen
// through its declared type.
func evalReferenceDeref(node ast.Node, ref *object.Reference) object.Object {
	val, err := ref.Get()
	if err != nil {
		return newError(node, "%s", err)
	}
	if val == nil {
		return NULL
	}
	return object.ArrayView(val, ref.Lower)
}

// assignThroughReference writes val to the variable a reference refers to.
func assignThroughReference(node ast.Node, ref *object.Reference, val object.Object) object.Object {
	if current, err := ref.Get(); err == nil {
		keepArrayBounds(current, val)
	}
	if err := ref.Set(val); err != nil {
		return newError(node, "%s", err)
	}
	return val
}
