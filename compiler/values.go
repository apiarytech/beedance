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

// This file makes arrays and structures values, as in IEC 61131-3: an
// assignment `b := a` copies a, so changing b leaves a alone, and a function
// or method gets its own copy of an array or structure input. The VM's
// OpCopy makes the copy. Function block instances are not copied.

import (
	"fmt"
	"strings"

	"beedance/ast"
	"beedance/code"
	"beedance/object"
)

// copiedValue is an assignment's value that is copied once computed.
type copiedValue struct {
	ast.Expression
}

// copyAssignedValue returns the assignment with its value copied when the
// target holds an array or structure, or the assignment itself otherwise.
func (c *Compiler) copyAssignedValue(node *ast.AssignmentStatement) *ast.AssignmentStatement {
	switch node.Value.(type) {
	case *copiedValue, *ast.ArrayLiteral, *ast.StructLiteral:
		// Already copied, or a new value.
		return node
	}
	if !c.holdsAggregate(node.Left) {
		return node
	}
	copied := *node
	copied.Value = &copiedValue{Expression: node.Value}
	return &copied
}

// holdsAggregate reports whether an assignment target was declared as an
// array or structure: a variable, a member, or an element of an array.
func (c *Compiler) holdsAggregate(target ast.Expression) bool {
	indices := 0
	for {
		index, ok := target.(*ast.IndexExpression)
		if !ok {
			break
		}
		indices++
		target = index.Left
	}
	dataType := c.declaredVarType(target)
	for indices > 0 {
		def := c.arrayTypeOf(dataType)
		if def == nil {
			return false
		}
		if indices < len(def.Ranges) {
			return true // A row of a multi-dimensional array.
		}
		indices -= len(def.Ranges)
		if def.DataType == nil {
			return false
		}
		dataType = def.DataType
	}
	return c.isAggregateType(dataType)
}

// isAggregateType reports whether a data type is an array or structure,
// looking through named TYPEs.
func (c *Compiler) isAggregateType(dataType ast.Expression) bool {
	for seen := 0; dataType != nil && seen < 16; seen++ {
		switch dataType.(type) {
		case *ast.ArrayDefinition, *ast.StructDefinition:
			return true
		}
		node, ok := c.resolveTypeNode(dataType)
		if !ok {
			return false
		}
		td, ok := node.(*ast.TypeDeclaration)
		if !ok {
			return false
		}
		dataType = td.DataType
	}
	return false
}

// declaredVarType returns the data type a variable, a function block's
// variable, or a member of a structure or function block was declared with,
// or nil.
func (c *Compiler) declaredVarType(target ast.Expression) ast.Expression {
	switch t := target.(type) {
	case *ast.Identifier:
		for i := c.scopeIndex; i >= 0; i-- {
			for name, decl := range c.scopes[i].varDecls {
				if strings.EqualFold(name, t.Value) {
					return decl.DataType
				}
			}
		}
		if c.currentFB != nil {
			if decl, _ := c.findVarDeclOnFBChain(c.currentFB, t.Value); decl != nil {
				return decl.DataType
			}
		}
	case *ast.MemberAccessExpression:
		return c.memberDataType(t)
	}
	return nil
}

// prepareParameters gives a function or method its own copy of each array
// or structure input as it starts, and sets the declared lower bounds on
// each array input, so an array passed in is indexed like the parameter.
func (c *Compiler) prepareParameters(params []*ast.VarDeclStatement) error {
	for _, p := range params {
		if !c.isAggregateType(p.DataType) {
			continue
		}
		symbol, ok := c.symbolTable.Resolve(p.Name.Value)
		if !ok {
			continue
		}
		c.loadSymbol(symbol)
		c.emit(code.OpCopy)
		if def := c.arrayTypeOf(p.DataType); def != nil {
			if err := c.emitArrayBounds(def); err != nil {
				return err
			}
		}
		// The input is read-only to the body, not to its own set-up.
		writable := symbol
		writable.IsReadOnly = false
		if err := c.setSymbol(writable); err != nil {
			return err
		}
	}
	return nil
}

// bitSetValue is the value a bit assignment `flags.3 := value` stores in
// its target: the target with the bit set to value.
type bitSetValue struct {
	*ast.BitAccessExpression
	value ast.Expression
}

// assignBit rewrites a bit assignment, `flags.3 := value`, as an assignment
// to the whole target, `flags := <flags with bit 3 set to value>`, which
// stores it like any other assignment. Other assignments are unchanged.
func assignBit(node *ast.AssignmentStatement) *ast.AssignmentStatement {
	bit, ok := node.Left.(*ast.BitAccessExpression)
	if !ok {
		return node
	}
	rewritten := *node
	rewritten.Left = bit.Target
	rewritten.Value = &bitSetValue{BitAccessExpression: bit, value: node.Value}
	return &rewritten
}

// compileBitAccess compiles a bit read, or the value of a bit write.
func (c *Compiler) compileBitAccess(bit *ast.BitAccessExpression, value ast.Expression) error {
	if bit.Bit < 0 || bit.Bit > 63 {
		return fmt.Errorf("bit %d is outside any integer, in %s", bit.Bit, bit.String())
	}
	// The target's declared type, which the VM may hold in a wider integer.
	typeOperand := 0
	if typ, err := c.getExpressionType(bit.Target); err == nil && typ != "" {
		typeOperand = c.addConstant(&object.String{Value: string(typ)}) + 1
	}
	if err := c.Compile(bit.Target); err != nil {
		return err
	}
	if value == nil {
		c.emit(code.OpGetBit, int(bit.Bit), typeOperand)
		return nil
	}
	if err := c.Compile(value); err != nil {
		return err
	}
	c.emit(code.OpSetBit, int(bit.Bit), typeOperand)
	return nil
}
