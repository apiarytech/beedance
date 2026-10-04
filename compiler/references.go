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

// This file compiles references: variables declared REF_TO <type>, or
// POINTER TO <type>, which beedance treats as the same typed reference.
// REF(x) and ADR(x) refer to a variable, an array element or a member; r^ is
// the variable r refers to; NULL refers to nothing. There is no pointer
// arithmetic.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
)

// isReferenceCall reports whether a call is REF(x) or ADR(x), unless the
// program declares something of that name.
func (c *Compiler) isReferenceCall(call *ast.CallExpression) bool {
	ident, ok := call.Function.(*ast.Identifier)
	if !ok {
		return false
	}
	switch strings.ToUpper(ident.Value) {
	case "REF", "ADR":
	default:
		return false
	}
	if _, declared := c.symbolTable.Resolve(ident.Value); declared {
		return false
	}
	_, declared := c.resolveTypeName(ident.Value)
	return !declared
}

// compileReference compiles REF(x) or ADR(x): x is compiled as it is read,
// and the instruction that reads it becomes one that refers to it.
func (c *Compiler) compileReference(call *ast.CallExpression) error {
	name := strings.ToUpper(call.Function.(*ast.Identifier).Value)
	if len(call.Arguments) != 1 {
		return fmt.Errorf("%s takes one argument, got %d", name, len(call.Arguments))
	}
	target := call.Arguments[0]
	if deref, ok := target.(*ast.DereferenceExpression); ok && !isInstanceSelf(deref.Pointer) {
		return c.Compile(deref.Pointer) // REF(r^) is r itself
	}
	start := len(c.currentInstructions())
	if err := c.Compile(target); err != nil {
		return err
	}
	ops := decodeFrom(c.currentInstructions(), start)
	// A value read is copied when it is an array or a structure; the
	// reference refers to the variable itself.
	if n := len(ops); n > 0 && ops[n-1].op == code.OpCopy {
		ops = ops[:n-1]
	}
	if len(ops) == 0 {
		return fmt.Errorf("%s needs a variable, got %s", name, target.String())
	}
	last := ops[len(ops)-1]
	c.truncateInstructions(last.pos, ops[:len(ops)-1])
	switch last.op {
	case code.OpGetGlobal:
		c.emit(code.OpRef, code.RefGlobal, last.operand)
	case code.OpGetLocal:
		c.emit(code.OpRef, code.RefLocal, last.operand)
	case code.OpGetFree:
		c.emit(code.OpRef, code.RefFree, last.operand)
	case code.OpGetExternal:
		c.emit(code.OpRef, code.RefExternal, last.operand)
	case code.OpIndex:
		c.emit(code.OpRefIndex)
	default:
		return fmt.Errorf("%s needs a variable, an array element or a member, got %s", name, target.String())
	}
	return nil
}

// isInstanceSelf reports whether e is THIS or SUPER, for which ^ is the
// instance itself, not a reference.
func isInstanceSelf(e ast.Expression) bool {
	switch e.(type) {
	case *ast.ThisExpression, *ast.SuperExpression:
		return true
	}
	return false
}

// compileDereference compiles r^, the variable the reference r refers to.
func (c *Compiler) compileDereference(node *ast.DereferenceExpression) error {
	if err := c.Compile(node.Pointer); err != nil {
		return err
	}
	c.emit(code.OpDeref, c.referenceBoundsOperand(node.Pointer))
	return nil
}

// referenceBoundsOperand returns OpDeref's operand for a reference: 1 + the
// constant index of the lower bounds of the array type the reference is
// declared to refer to, or 0 when it is not declared to refer to an array.
// A type indexed from 0 needs them too, as the array referred to may be
// indexed from 1.
func (c *Compiler) referenceBoundsOperand(ref ast.Expression) int {
	rt, ok := c.declaredTypeOf(ref).(*ast.RefToType)
	if !ok {
		return 0
	}
	def := c.arrayTypeOf(rt.BaseType)
	if def == nil {
		return 0
	}
	bounds, _ := c.arrayLowerBounds(def)
	if len(bounds) == 0 {
		return 0
	}
	elements := make([]object.Object, len(bounds))
	for i, b := range bounds {
		elements[i] = &object.LInt{Value: b}
	}
	return c.addConstant(&object.Array{Elements: elements}) + 1
}

// compileSetDereference compiles r^ := value.
func (c *Compiler) compileSetDereference(target *ast.DereferenceExpression, value ast.Expression) error {
	if err := c.Compile(target.Pointer); err != nil {
		return err
	}
	if err := c.Compile(value); err != nil {
		return err
	}
	c.emit(code.OpSetDeref)
	return nil
}

// decodedOp is an instruction decoded from compiled code.
type decodedOp struct {
	op      code.Opcode
	pos     int
	operand int // the first operand, if any
}

// decodeFrom decodes the instructions from position start on.
func decodeFrom(ins code.Instructions, start int) []decodedOp {
	var ops []decodedOp
	for pos := start; pos < len(ins); {
		def, err := code.Lookup(ins[pos])
		if err != nil {
			return ops
		}
		operands, read := code.ReadOperands(def, ins[pos+1:])
		d := decodedOp{op: code.Opcode(ins[pos]), pos: pos}
		if len(operands) > 0 {
			d.operand = operands[0]
		}
		ops = append(ops, d)
		pos += 1 + read
	}
	return ops
}

// truncateInstructions drops the instructions from position pos on; before
// holds the instructions that remain since the expression started, so that
// the last instruction is known again.
func (c *Compiler) truncateInstructions(pos int, before []decodedOp) {
	scope := &c.scopes[c.scopeIndex]
	scope.instructions = scope.instructions[:pos]
	scope.lastInstruction, scope.previousInstruction = EmittedInstruction{}, EmittedInstruction{}
	if n := len(before); n > 0 {
		scope.lastInstruction = EmittedInstruction{Opcode: before[n-1].op, Position: before[n-1].pos}
	}
	if n := len(before); n > 1 {
		scope.previousInstruction = EmittedInstruction{Opcode: before[n-2].op, Position: before[n-2].pos}
	}
}

// checkReferenceValue checks the value given to a variable declared with the
// reference type declared: REF(x) or ADR(x) of a variable of the type it
// refers to, another reference of that type, or NULL. A reference refers to
// a whole variable of its type: POINTER TO BYTE cannot refer to a STRING,
// as beedance has no byte access through references.
func (c *Compiler) checkReferenceValue(name string, declared ast.Expression, value ast.Expression) error {
	rt, ok := declared.(*ast.RefToType)
	if !ok {
		return nil
	}
	var target ast.Expression // the type the value refers to, when known
	switch v := value.(type) {
	case *ast.Identifier:
		if strings.EqualFold(v.Value, "NULL") {
			if _, declared := c.symbolTable.Resolve(v.Value); !declared {
				return nil
			}
		}
	case *ast.CallExpression:
		if c.isReferenceCall(v) && len(v.Arguments) == 1 {
			target = c.referencedType(v.Arguments[0])
			if target == nil {
				return nil
			}
			if object.ReferenceTypeKey(target) != object.ReferenceTypeKey(rt.BaseType) {
				return fmt.Errorf("%s refers to a %s, but %s is declared %s: a reference refers to a whole variable of its type, and beedance references have no byte access",
					v.String(), object.ReferenceTypeKey(target), name, rt.String())
			}
			return nil
		}
	}
	valueType := c.declaredTypeOf(value)
	if valueType == nil {
		switch value.(type) {
		case *ast.CallExpression, *ast.DereferenceExpression, *ast.IndexExpression, *ast.MemberAccessExpression, *ast.Identifier:
			return nil // not known here, such as a function's result
		}
		// A literal or a computed value is never a reference.
		return fmt.Errorf("%s is declared %s; it takes REF(), ADR() or NULL, not %s", name, rt.String(), value.String())
	}
	other, isRef := valueType.(*ast.RefToType)
	if !isRef {
		return fmt.Errorf("%s is declared %s; it takes REF(), ADR() or NULL, not %s", name, rt.String(), value.String())
	}
	if object.ReferenceTypeKey(other.BaseType) != object.ReferenceTypeKey(rt.BaseType) {
		return fmt.Errorf("%s is declared %s, but %s is %s", name, rt.String(), value.String(), other.String())
	}
	return nil
}

// referencedType returns the declared type of the variable, array element
// or member REF(target) refers to, or nil when it is not known.
func (c *Compiler) referencedType(target ast.Expression) ast.Expression {
	switch t := target.(type) {
	case *ast.IndexExpression:
		if def := c.arrayTypeOf(c.referencedType(t.Left)); def != nil {
			return def.DataType
		}
		return nil
	case *ast.DereferenceExpression:
		if rt, ok := c.referencedType(t.Pointer).(*ast.RefToType); ok {
			return rt.BaseType
		}
		return nil
	}
	return c.declaredTypeOf(target)
}

// referenceExpressionType returns the type the type checker gives a
// reference expression: a reference, REF(x), ADR(x) and NULL are of any
// type, as they are checked by checkReferenceValue; r^ is of the type r
// refers to, and pt^[i] of its element type. ok is false for other
// expressions.
func (c *Compiler) referenceExpressionType(expr ast.Expression) (object.ObjectType, bool) {
	switch e := expr.(type) {
	case *ast.Identifier:
		if _, declared := c.symbolTable.Resolve(e.Value); !declared && strings.EqualFold(e.Value, "NULL") {
			return anyType, true
		}
		if _, isRef := c.declaredTypeOf(e).(*ast.RefToType); isRef {
			return anyType, true
		}
	case *ast.MemberAccessExpression:
		if _, isRef := c.declaredTypeOf(e).(*ast.RefToType); isRef {
			return anyType, true
		}
	case *ast.CallExpression:
		if c.isReferenceCall(e) {
			return anyType, true
		}
	case *ast.DereferenceExpression:
		if rt, isRef := c.referencedType(e.Pointer).(*ast.RefToType); isRef {
			return typeOrAny(c.flattenExpressionToString(rt.BaseType)), true
		}
	case *ast.IndexExpression:
		if deref, ok := e.Left.(*ast.DereferenceExpression); ok {
			if rt, isRef := c.referencedType(deref.Pointer).(*ast.RefToType); isRef {
				if def := c.arrayTypeOf(rt.BaseType); def != nil {
					return typeOrAny(c.flattenExpressionToString(def.DataType)), true
				}
				return anyType, true
			}
		}
	}
	return "", false
}

// typeOrAny returns a type name as the type checker's type, or anyType when
// it is not known.
func typeOrAny(name string) object.ObjectType {
	if name == "" {
		return anyType
	}
	return object.ObjectType(name)
}

// declaredTypeOf returns the declared type of a variable or member, as
// declaredVarType does, also finding the parameters and variables of the
// function being compiled.
func (c *Compiler) declaredTypeOf(e ast.Expression) ast.Expression {
	if t := c.declaredVarType(e); t != nil {
		return t
	}
	ident, ok := e.(*ast.Identifier)
	if !ok {
		return nil
	}
	if fn := c.currentFunction(); fn != nil {
		for _, block := range [][]*ast.VarDeclStatement{fn.VarInputs, fn.VarOutputs, fn.VarInOuts, fn.Vars} {
			if d := findParameter(block, ident.Value); d != nil {
				return d.DataType
			}
		}
	}
	return nil
}

// checkReferenceArguments checks the arguments of a call of fn given to its
// inputs declared with a reference type, as checkReferenceValue checks an
// assignment: positional arguments are its inputs, then its VAR_IN_OUTs, in
// order.
func (c *Compiler) checkReferenceArguments(fn *ast.FunctionDeclaration, args []ast.Expression) error {
	params := append(append([]*ast.VarDeclStatement{}, fn.VarInputs...), fn.VarInOuts...)
	positional := 0
	for _, arg := range args {
		var param *ast.VarDeclStatement
		value := arg
		if named, ok := arg.(*ast.NamedArgument); ok {
			param, value = findParameter(params, named.Name.Value), named.Value
		} else {
			if positional < len(params) {
				param = params[positional]
			}
			positional++
		}
		if param == nil {
			continue
		}
		name := fmt.Sprintf("input %s of function %s", param.Name.Value, fn.Name.Value)
		if err := c.checkReferenceValue(name, param.DataType, value); err != nil {
			return err
		}
	}
	return nil
}
