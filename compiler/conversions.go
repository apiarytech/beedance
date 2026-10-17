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

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
)

// conversionCall reports whether call is a standard type conversion X_TO_Y,
// which compiles to the one generic built-in $CONVERT, and between which
// types. A function the program declares under such a name is its own.
func (c *Compiler) conversionCall(call *ast.CallExpression) (from, to string, ok bool) {
	ident, isIdent := call.Function.(*ast.Identifier)
	if !isIdent {
		return "", "", false
	}
	if from, to, ok = object.ParseConversion(ident.Value); !ok {
		return "", "", false
	}
	if _, declared := c.symbolTable.Resolve(ident.Value); declared {
		return "", "", false
	}
	if _, declared := c.resolveTypeName(ident.Value); declared {
		return "", "", false
	}
	return from, to, true
}

// compileConversion compiles X_TO_Y(v) as $CONVERT(v, 'X', 'Y'). A named
// argument (X_TO_Y(IN := v)) passes its value.
func (c *Compiler) compileConversion(call *ast.CallExpression, from, to string) error {
	convert, ok := c.symbolTable.Resolve(object.ConvertBuiltinName)
	if !ok {
		return fmt.Errorf("%s_TO_%s: the conversion built-in %s is not registered (import the stdlib package)", from, to, object.ConvertBuiltinName)
	}
	c.loadSymbol(convert)
	for _, arg := range call.Arguments {
		if named, isNamed := arg.(*ast.NamedArgument); isNamed {
			arg = named.Value
		}
		if err := c.Compile(arg); err != nil {
			return err
		}
	}
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: from}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: to}))
	c.emit(code.OpCall, len(call.Arguments)+2)
	return nil
}

// timeFunctionCall reports whether call is one of the time functions
// (object.IsTimeFunction), and which; a function the program declares under
// such a name is its own.
func (c *Compiler) timeFunctionCall(call *ast.CallExpression) (string, bool) {
	ident, isIdent := call.Function.(*ast.Identifier)
	if !isIdent || !object.IsTimeFunction(ident.Value) {
		return "", false
	}
	if _, declared := c.resolveTypeName(ident.Value); declared {
		return "", false
	}
	return strings.ToUpper(ident.Value), true
}

// compileTimeFunction compiles NAME(args) as $TIMEFN('NAME', args).
func (c *Compiler) compileTimeFunction(call *ast.CallExpression, name string) error {
	fn, ok := c.symbolTable.Resolve(object.TimeFunctionBuiltinName)
	if !ok {
		return fmt.Errorf("%s: the built-in %s is not registered (import the stdlib package)", name, object.TimeFunctionBuiltinName)
	}
	c.loadSymbol(fn)
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: name}))
	for _, arg := range call.Arguments {
		if named, isNamed := arg.(*ast.NamedArgument); isNamed {
			arg = named.Value
		}
		if err := c.Compile(arg); err != nil {
			return err
		}
	}
	c.emit(code.OpCall, len(call.Arguments)+1)
	return nil
}

// endianCall reports whether call is TO_BIG_ENDIAN or another function of
// byte order (stdlib/endian.go) of one argument.
func (c *Compiler) endianCall(call *ast.CallExpression) bool {
	ident, isIdent := call.Function.(*ast.Identifier)
	if !isIdent || len(call.Arguments) != 1 {
		return false
	}
	switch strings.ToUpper(ident.Value) {
	case "TO_BIG_ENDIAN", "TO_LITTLE_ENDIAN", "FROM_BIG_ENDIAN", "FROM_LITTLE_ENDIAN":
	default:
		return false
	}
	_, declared := c.resolveTypeName(ident.Value)
	return !declared
}

// typedEndianCall is TO_BIG_ENDIAN(v) as TO_BIG_ENDIAN(v, 'T'), T the declared
// type of v: the VM holds an INT as a LINT, whose bytes are not the INT's.
func (c *Compiler) typedEndianCall(call *ast.CallExpression) *ast.CallExpression {
	t, err := c.getExpressionType(call.Arguments[0])
	if err != nil || t == "" {
		return call
	}
	typed := *call
	typed.Arguments = []ast.Expression{call.Arguments[0], &ast.StringLiteral{Value: string(c.aliasedType(t))}}
	return &typed
}
