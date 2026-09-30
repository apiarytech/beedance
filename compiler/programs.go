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

// This file compiles PROGRAM declarations. As in the evaluator and IEC
// 61131-3, declaring a program defines it and a call such as
// `Pg(i := 4, o => res)` runs it. A program is compiled as a function block
// class with a single instance named after the program, so a call is a
// function block call, its variables keep their values between calls, and
// two programs' variables do not clash. Its VAR_GLOBAL, VAR_EXTERNAL and
// VAR_ACCESS blocks remain global.
//
// A program with an SFC body is still compiled in place, producing the
// description of its chart, since the VM does not run charts.

import (
	"fmt"
	"strings"

	"beedance/ast"
	"beedance/code"
	"beedance/object"
)

// programClassName is the name of the function block class of a program.
// IEC 61131-3 identifiers cannot contain two underscores in a row, so it
// never clashes with a user's name.
func programClassName(program string) string {
	return "__program_" + program
}

// compileCallableProgram compiles a program as a class and its instance.
func (c *Compiler) compileCallableProgram(node *ast.ProgramDeclaration) error {
	for _, block := range node.VarGlobal {
		if err := c.Compile(block); err != nil {
			return err
		}
	}
	for _, block := range node.VarExternal {
		if err := c.Compile(block); err != nil {
			return err
		}
	}
	for _, block := range node.VarAccess {
		if err := c.Compile(block); err != nil {
			return err
		}
	}

	className := &ast.Identifier{Token: node.Name.Token, Value: programClassName(node.Name.Value)}
	class := &ast.FunctionBlockDeclaration{
		Token:      node.Token,
		Name:       className,
		VarInputs:  node.VarInputs,
		VarOutputs: node.VarOutputs,
		VarInOuts:  node.VarInOuts,
		Vars:       node.Vars,
		VarTemp:    node.VarTemp,
		Body:       node.Body,
	}
	c.typeInfo[strings.ToUpper(className.Value)] = class
	if err := c.Compile(class); err != nil {
		return err
	}
	instance := &ast.VarDeclStatement{Token: node.Token, Name: node.Name, DataType: className}
	return c.Compile(instance)
}

// storeTopInto stores the value on top of the stack into a variable, which
// may be a field of the function block (or program) being compiled.
func (c *Compiler) storeTopInto(name string) error {
	if symbol, ok := c.symbolTable.Resolve(name); ok && !c.hidesSymbol(symbol, name) {
		return c.setSymbol(symbol)
	}
	if !c.isFunctionBlockVar(name) {
		return fmt.Errorf("undefined variable %s", name)
	}
	// THIS.name := value, with the value already on the stack.
	this, _ := c.symbolTable.Resolve("THIS")
	c.loadSymbol(this)
	c.emit(code.OpSwap)
	c.emitConstant(c.addConstant(&object.String{Value: name}))
	c.emit(code.OpSwap)
	c.emit(code.OpSetIndex)
	return nil
}

// describePOU names a function block, or the program it was compiled from,
// for an error message.
func describePOU(fbDef *ast.FunctionBlockDeclaration) string {
	if name, isProgram := strings.CutPrefix(fbDef.Name.Value, programClassName("")); isProgram {
		return fmt.Sprintf("program '%s'", name)
	}
	return fmt.Sprintf("function block '%s'", fbDef.Name.Value)
}
