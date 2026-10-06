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
	"slices"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
)

// Directly represented variables used in statements, such as
// `level := %IW0;` or `%QX0.1 := run;` (IEC 61131-3, 6.5.5). They live in
// the VM's I/O image by address, as located variables do, and have the type
// their size gives (ast.DirectType). A program's direct variables are
// listed with its variables, so a host binds them like located ones.

// directObjectType is the type of a directly represented variable.
func (c *Compiler) directObjectType(dv *ast.DirectVariable) (object.ObjectType, error) {
	typ, ok := ast.DirectType(dv.FullAddress())
	if !ok {
		return "", fmt.Errorf("%s is not a directly represented variable", dv.FullAddress())
	}
	return conversionTargets[typ], nil
}

// compileDirectRead reads a direct variable from the I/O image.
func (c *Compiler) compileDirectRead(dv *ast.DirectVariable) error {
	if _, err := c.directObjectType(dv); err != nil {
		return err
	}
	addr := dv.FullAddress()
	c.noteDirect(addr)
	c.emit(code.OpGetExternal, c.addConstant(&object.String{Value: addr}))
	return nil
}

// compileDirectWrite writes value to a direct variable. An input (%I) is
// read-only: its value comes from the field.
func (c *Compiler) compileDirectWrite(dv *ast.DirectVariable, value ast.Expression) error {
	if _, err := c.directObjectType(dv); err != nil {
		return err
	}
	addr := dv.FullAddress()
	if ast.IsInputAddress(addr) {
		return fmt.Errorf("%s is an input: a program reads it but cannot write it", addr)
	}
	if err := c.Compile(value); err != nil {
		return err
	}
	c.noteDirect(addr)
	c.emit(code.OpSetExternal, c.addConstant(&object.String{Value: addr}))
	return nil
}

// noteDirect records a direct variable the program uses.
func (c *Compiler) noteDirect(addr string) {
	if !slices.Contains(c.directUses, addr) {
		c.directUses = append(c.directUses, addr)
	}
}

// withDirect adds the direct variables the program uses to its variables,
// as located variables named by their address, unless a located variable
// declared at the same address already stands for it.
func (c *Compiler) withDirect(vars []Variable) []Variable {
	for _, addr := range c.directUses {
		declared := slices.ContainsFunc(vars, func(v Variable) bool { return strings.EqualFold(v.Address, addr) })
		if declared {
			continue
		}
		typ, _ := ast.DirectType(addr)
		vars = append(vars, Variable{Name: addr, Block: "VAR", Type: typ, Global: -1, Address: addr})
	}
	return vars
}
