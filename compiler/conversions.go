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
