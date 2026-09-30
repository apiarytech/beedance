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

// This file compiles calls of function block instances, e.g.
// `timer(IN := start, PT := T#5s, Q => done)`. An instance is a hash whose
// fields are the function block's variables; its class holds a "main"
// closure, the function block's body, which takes the instance as THIS.
//
// A call sets the given inputs on the instance, runs the body (unless EN is
// FALSE), then assigns the requested outputs. A VAR_IN_OUT is passed by
// copy-in/copy-out: the argument's value is copied into the instance before
// the body runs and back into the argument afterwards. This gives the same
// result as passing a reference, except when the same variable is passed to
// two in-outs of one call.

import (
	"fmt"
	"strings"

	"beedance/ast"
	"beedance/code"
	"beedance/object"
)

// calleeFunctionBlock returns the function block an expression is an
// instance of, or nil if it is not a function block instance.
func (c *Compiler) calleeFunctionBlock(callee ast.Expression) *ast.FunctionBlockDeclaration {
	typeName, ok := c.instanceTypeName(callee)
	if !ok {
		return nil
	}
	node, ok := c.resolveTypeName(typeName)
	if !ok {
		return nil
	}
	fbDef, _ := node.(*ast.FunctionBlockDeclaration)
	return fbDef
}

// functionBlockParameters returns the inputs, in-outs and outputs of a
// function block, its ancestors' first.
func (c *Compiler) functionBlockParameters(fbDef *ast.FunctionBlockDeclaration) (inputs, inOuts, outputs []*ast.VarDeclStatement) {
	chain := []*ast.FunctionBlockDeclaration{}
	for fb, seen := fbDef, map[*ast.FunctionBlockDeclaration]bool{}; fb != nil && !seen[fb]; {
		seen[fb] = true
		chain = append([]*ast.FunctionBlockDeclaration{fb}, chain...)
		if fb.Extends == nil {
			break
		}
		parent, ok := c.resolveTypeNode(fb.Extends)
		if !ok {
			break
		}
		fb, _ = parent.(*ast.FunctionBlockDeclaration)
	}
	for _, fb := range chain {
		inputs = append(inputs, fb.VarInputs...)
		inOuts = append(inOuts, fb.VarInOuts...)
		outputs = append(outputs, fb.VarOutputs...)
	}
	return inputs, inOuts, outputs
}

// findParameter finds a declaration by name, ignoring case.
func findParameter(decls []*ast.VarDeclStatement, name string) *ast.VarDeclStatement {
	for _, d := range decls {
		if strings.EqualFold(d.Name.Value, name) {
			return d
		}
	}
	return nil
}

// isAssignable reports whether an expression can be assigned to, as a
// VAR_IN_OUT argument must be.
func isAssignable(exp ast.Expression) bool {
	switch exp.(type) {
	case *ast.Identifier, *ast.IndexExpression, *ast.MemberAccessExpression:
		return true
	}
	return false
}

// compileFunctionBlockCall compiles a call of a function block instance.
// The call's value is NULL, as a function block has no result.
func (c *Compiler) compileFunctionBlockCall(node *ast.CallExpression, fbDef *ast.FunctionBlockDeclaration) error {
	instance := node.Function
	field := func(name string) ast.Expression {
		return &ast.MemberAccessExpression{Token: node.Token, Struct: instance, Member: &ast.Identifier{Token: node.Token, Value: name}}
	}
	assign := func(left, value ast.Expression) error {
		return c.Compile(&ast.AssignmentStatement{Token: node.Token, Left: left, Value: value})
	}

	inputs, inOuts, outputs := c.functionBlockParameters(fbDef)
	params := append(append([]*ast.VarDeclStatement{}, inputs...), inOuts...)
	var enable, enableTarget ast.Expression
	copyBacks := []*ast.AssignmentStatement{}
	outputAssigns := []*ast.AssignmentStatement{}

	setInput := func(param *ast.VarDeclStatement, value ast.Expression) error {
		if findParameter(inOuts, param.Name.Value) != nil {
			if !isAssignable(value) {
				return fmt.Errorf("argument for VAR_IN_OUT '%s' of %s must be a variable, got %s", param.Name.Value, describePOU(fbDef), value.String())
			}
			copyBacks = append(copyBacks, &ast.AssignmentStatement{Token: node.Token, Left: value, Value: field(param.Name.Value)})
		}
		return assign(field(param.Name.Value), value)
	}

	positional := 0
	for _, arg := range node.Arguments {
		switch a := arg.(type) {
		case *ast.NamedArgument:
			if strings.EqualFold(a.Name.Value, "EN") {
				enable = a.Value
				continue
			}
			param := findParameter(params, a.Name.Value)
			if param == nil {
				return fmt.Errorf("%s has no input '%s'", describePOU(fbDef), a.Name.Value)
			}
			if err := setInput(param, a.Value); err != nil {
				return err
			}
		case *ast.OutputArgument:
			if strings.EqualFold(a.Source.Value, "ENO") {
				enableTarget = a.Target
				continue
			}
			output := findParameter(outputs, a.Source.Value)
			if output == nil {
				return fmt.Errorf("%s has no output '%s'", describePOU(fbDef), a.Source.Value)
			}
			outputAssigns = append(outputAssigns, &ast.AssignmentStatement{Token: node.Token, Left: a.Target, Value: field(output.Name.Value)})
		default:
			if positional >= len(params) {
				return fmt.Errorf("%s takes %d inputs, got more", describePOU(fbDef), len(params))
			}
			if err := setInput(params[positional], arg); err != nil {
				return err
			}
			positional++
		}
	}

	// Run the body, unless EN is FALSE.
	skip := -1
	if enable != nil {
		if err := c.Compile(enable); err != nil {
			return err
		}
		skip = c.emit(code.OpJumpNotTruthy, 9999)
	}
	if err := c.Compile(instance); err != nil {
		return err
	}
	c.emitConstant(c.addConstant(&object.String{Value: "main"}))
	c.emit(code.OpIndex)
	c.emit(code.OpCall, 0)
	c.emit(code.OpPop)
	if skip >= 0 {
		c.changeOperand(skip, len(c.currentInstructions()))
	}

	// In-outs are copied back and outputs assigned whether or not the body ran.
	for _, stmt := range append(copyBacks, outputAssigns...) {
		if err := c.Compile(stmt); err != nil {
			return err
		}
	}
	// ENO follows EN: TRUE unless the body was skipped.
	if enableTarget != nil {
		value := enable
		if value == nil {
			value = &ast.Boolean{Token: node.Token, Value: true}
		}
		if err := assign(enableTarget, value); err != nil {
			return err
		}
	}
	c.emit(code.OpNull)
	return nil
}

// instanceTypeName returns the declared type name of a variable, or of a
// member of a structure or function block instance such as `THIS.inner` or
// `line.start`.
func (c *Compiler) instanceTypeName(exp ast.Expression) (string, bool) {
	member, isMember := exp.(*ast.MemberAccessExpression)
	if !isMember {
		return c.getExpressionTypeName(exp)
	}
	ownerType, ok := c.instanceTypeName(member.Struct)
	if !ok {
		return "", false
	}
	owner, ok := c.resolveTypeName(ownerType)
	if !ok {
		return "", false
	}
	var decl *ast.VarDeclStatement
	switch o := owner.(type) {
	case *ast.FunctionBlockDeclaration:
		decl, _ = c.findVarDeclOnFBChain(o, member.Member.Value)
	case *ast.TypeDeclaration:
		if def, isStruct := o.DataType.(*ast.StructDefinition); isStruct {
			decl = findParameter(def.Members, member.Member.Value)
		}
	}
	if decl == nil || decl.DataType == nil {
		return "", false
	}
	return c.flattenExpressionToString(decl.DataType), true
}

// functionInOutCopyBacks returns, for each VAR_IN_OUT of a function call, an
// output argument that copies its final value back into the argument. Every
// in-out must be given a variable.
func (c *Compiler) functionInOutCopyBacks(fn *ast.FunctionDeclaration, args []ast.Expression, call *ast.CallExpression) ([]*ast.OutputArgument, error) {
	if len(fn.VarInOuts) == 0 {
		return nil, nil
	}
	copyBacks := []*ast.OutputArgument{}
	supplied := map[string]bool{}
	positional := 0
	for _, arg := range args {
		var param *ast.VarDeclStatement
		value := arg
		if named, ok := arg.(*ast.NamedArgument); ok {
			param = findParameter(fn.VarInOuts, named.Name.Value)
			value = named.Value
		} else {
			if i := positional - len(fn.VarInputs); i >= 0 && i < len(fn.VarInOuts) {
				param = fn.VarInOuts[i]
			}
			positional++
		}
		if param == nil {
			continue
		}
		if !isAssignable(value) {
			return nil, fmt.Errorf("argument for VAR_IN_OUT '%s' of function '%s' must be a variable, got %s", param.Name.Value, fn.Name.Value, value.String())
		}
		supplied[strings.ToUpper(param.Name.Value)] = true
		copyBacks = append(copyBacks, &ast.OutputArgument{Token: call.Token, Source: &ast.Identifier{Token: call.Token, Value: param.Name.Value}, Target: value})
	}
	for _, p := range fn.VarInOuts {
		if !supplied[strings.ToUpper(p.Name.Value)] {
			return nil, fmt.Errorf("the call to '%s' must supply its VAR_IN_OUT '%s'", fn.Name.Value, p.Name.Value)
		}
	}
	return copyBacks, nil
}

// arrayLowerBounds returns the constant lower bound of each dimension of an
// array type, and whether any of them is not zero.
func (c *Compiler) arrayLowerBounds(def *ast.ArrayDefinition) ([]int64, bool) {
	bounds := []int64{}
	nonZero := false
	for _, rng := range def.Ranges {
		infix, ok := rng.(*ast.InfixExpression)
		if !ok || infix.Operator != ".." {
			return nil, false
		}
		low, err := c.evaluateConstantInteger(infix.Left)
		if err != nil {
			return nil, false
		}
		bounds = append(bounds, low)
		nonZero = nonZero || low != 0
	}
	return bounds, nonZero
}

// emitArrayBounds sets the declared lower bounds on the array on top of the
// stack. An array indexed from 0 in every dimension needs nothing.
func (c *Compiler) emitArrayBounds(def *ast.ArrayDefinition) error {
	bounds, nonZero := c.arrayLowerBounds(def)
	if !nonZero {
		return nil
	}
	elements := make([]object.Object, len(bounds))
	for i, b := range bounds {
		elements[i] = &object.LInt{Value: b}
	}
	c.emit(code.OpArrayBounds, c.addConstant(&object.Array{Elements: elements}))
	return nil
}

// declaredArrayType returns the array type an assignment target was declared
// with: a variable, a function block's variable, or a member of a structure
// or function block. It returns nil for anything else.
func (c *Compiler) declaredArrayType(target ast.Expression) *ast.ArrayDefinition {
	return c.arrayTypeOf(c.declaredVarType(target))
}

// memberDataType returns the data type of a structure or function block
// member, or nil.
func (c *Compiler) memberDataType(member *ast.MemberAccessExpression) ast.Expression {
	ownerType, ok := c.instanceTypeName(member.Struct)
	if !ok {
		return nil
	}
	owner, ok := c.resolveTypeName(ownerType)
	if !ok {
		return nil
	}
	var decl *ast.VarDeclStatement
	switch o := owner.(type) {
	case *ast.FunctionBlockDeclaration:
		decl, _ = c.findVarDeclOnFBChain(o, member.Member.Value)
	case *ast.TypeDeclaration:
		if def, isStruct := o.DataType.(*ast.StructDefinition); isStruct {
			decl = findParameter(def.Members, member.Member.Value)
		}
	}
	if decl == nil {
		return nil
	}
	return decl.DataType
}

// arrayTypeOf returns the array definition of a data type, looking through
// a named TYPE, or nil if it is not an array type.
func (c *Compiler) arrayTypeOf(dataType ast.Expression) *ast.ArrayDefinition {
	for seen := 0; dataType != nil && seen < 16; seen++ {
		if def, ok := dataType.(*ast.ArrayDefinition); ok {
			return def
		}
		node, ok := c.resolveTypeNode(dataType)
		if !ok {
			return nil
		}
		td, ok := node.(*ast.TypeDeclaration)
		if !ok {
			return nil
		}
		dataType = td.DataType
	}
	return nil
}

// isFunctionBlockVar reports whether name is a variable of the function block
// being compiled.
func (c *Compiler) isFunctionBlockVar(name string) bool {
	if c.currentFB == nil {
		return false
	}
	decl, _ := c.findVarDeclOnFBChain(c.currentFB, name)
	return decl != nil
}

// hidesSymbol reports whether a variable of the function block being
// compiled hides symbol, a built-in function or global of the same name,
// such as a variable `first` or a variable `counter` of type Counter.
func (c *Compiler) hidesSymbol(symbol Symbol, name string) bool {
	if c.currentFB == nil || (symbol.Scope != BuiltinScope && symbol.Scope != GlobalScope) {
		return false
	}
	decl, _ := c.findVarDeclOnFBChain(c.currentFB, name)
	return decl != nil && !strings.EqualFold(decl.Scope, "VAR_EXTERNAL")
}
