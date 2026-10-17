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

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/token"
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

// wrappedInt is an assignment's value that wraps into the range of a narrow
// integer type once computed (OpWrapInt with code).
type wrappedInt struct {
	ast.Expression
	code int
}

// wrapAssignedInteger returns the assignment with its value wrapped into
// the range of a target declared SINT, INT, DINT, USINT, UINT or UDINT (a
// variable, a member, or an element of an array of them), as an
// overflowing PLC integer wraps; or the assignment itself otherwise.
func (c *Compiler) wrapAssignedInteger(node *ast.AssignmentStatement) *ast.AssignmentStatement {
	if _, done := node.Value.(*wrappedInt); done {
		return node
	}
	var dataType ast.Expression
	switch target := node.Left.(type) {
	case *ast.Identifier, *ast.MemberAccessExpression:
		dataType = c.declaredVarType(target)
	case *ast.IndexExpression:
		array := target.Left
		for inner, ok := array.(*ast.IndexExpression); ok; inner, ok = array.(*ast.IndexExpression) {
			array = inner.Left
		}
		if def, isArray := c.declaredVarType(array).(*ast.ArrayDefinition); isArray && def.DataType != nil {
			dataType = def.DataType
		}
	}
	if dataType == nil {
		return node
	}
	typeName := c.flattenExpressionToString(dataType)
	code := object.IntegerWrapCode(typeName)
	if code == 0 {
		return node
	}
	// A constant in range needs no wrapping, nor a variable of a type that
	// fits (INT := INT, DINT := SINT); arithmetic may overflow.
	if n, err := c.evaluateConstantInteger(node.Value); err == nil && object.WrapInteger(n, typeName) == n {
		return node
	}
	switch node.Value.(type) {
	case *ast.Identifier, *ast.MemberAccessExpression, *ast.IndexExpression:
		if from := c.valueTypeName(node.Value); from != "" && object.IntegerFits(from, typeName) {
			return node
		}
	}
	wrapped := *node
	wrapped.Value = &wrappedInt{Expression: node.Value, code: code}
	return &wrapped
}

// valueTypeName returns the declared type name of a variable, a member or
// an element of an array, or "".
func (c *Compiler) valueTypeName(exp ast.Expression) string {
	var dataType ast.Expression
	switch e := exp.(type) {
	case *ast.Identifier, *ast.MemberAccessExpression:
		dataType = c.declaredVarType(e)
	case *ast.IndexExpression:
		array := e.Left
		for inner, ok := array.(*ast.IndexExpression); ok; inner, ok = array.(*ast.IndexExpression) {
			array = inner.Left
		}
		if def, isArray := c.declaredVarType(array).(*ast.ArrayDefinition); isArray && def.DataType != nil {
			dataType = def.DataType
		}
	}
	if dataType == nil {
		return ""
	}
	return c.flattenExpressionToString(dataType)
}

// cutAssignedString returns the assignment with its value cut to the length
// of a variable declared STRING(n) or WSTRING(n), LEFT(value, n), as IEC
// 61131-3 keeps only n characters; or the assignment itself otherwise.
func (c *Compiler) cutAssignedString(node *ast.AssignmentStatement) *ast.AssignmentStatement {
	target, ok := node.Left.(*ast.Identifier)
	if !ok {
		return node
	}
	decl := c.declarationOf(target.Value)
	if decl == nil {
		return node
	}
	value := stringCut(decl, node.Value)
	if value == node.Value {
		return node
	}
	cut := *node
	cut.Value = value
	return &cut
}

// stringCut returns value as LEFT(value, n) when decl declares a STRING(n)
// or WSTRING(n), else value itself.
func stringCut(decl *ast.VarDeclStatement, value ast.Expression) ast.Expression {
	if decl.StringLength == nil || value == nil || decl.DataType == nil {
		return value
	}
	if name := strings.ToUpper(decl.DataType.String()); name != "STRING" && name != "WSTRING" {
		return value
	}
	left := &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "LEFT"}, Value: "LEFT"}
	return &ast.CallExpression{Token: left.Token, Function: left, Arguments: []ast.Expression{value, decl.StringLength}}
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

// convertAssignedToReal returns the assignment of an integer to a target
// declared REAL or LREAL with its value converted, TO_REAL(value), as an
// integer converts to a real implicitly; or the assignment itself.
func (c *Compiler) convertAssignedToReal(node *ast.AssignmentStatement) *ast.AssignmentStatement {
	var dataType ast.Expression
	switch target := node.Left.(type) {
	case *ast.Identifier, *ast.MemberAccessExpression:
		dataType = c.declaredVarType(target)
	case *ast.IndexExpression:
		array := target.Left
		for inner, ok := array.(*ast.IndexExpression); ok; inner, ok = array.(*ast.IndexExpression) {
			array = inner.Left
		}
		if def, isArray := c.declaredVarType(array).(*ast.ArrayDefinition); isArray && def.DataType != nil {
			dataType = def.DataType
		}
	}
	if dataType == nil {
		return node
	}
	typeName := strings.ToUpper(c.flattenExpressionToString(dataType))
	if typeName != "REAL" && typeName != "LREAL" {
		return node
	}
	valueType, err := c.getExpressionType(node.Value)
	if err != nil || !object.IsIntegerType(string(valueType)) {
		return node
	}
	tok := node.Token
	converted := *node
	converted.Value = &ast.CallExpression{Token: tok, Function: &ast.Identifier{Token: tok, Value: "TO_" + typeName}, Arguments: []ast.Expression{node.Value}}
	return &converted
}

// namesIdentifier reports whether node uses the identifier name, ignoring
// case.
func namesIdentifier(node ast.Node, name string) bool {
	found := false
	ast.Modify(node, func(n ast.Node) ast.Node {
		if ident, ok := n.(*ast.Identifier); ok && strings.EqualFold(ident.Value, name) {
			found = true
		}
		return n
	})
	return found
}
