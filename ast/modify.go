/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast

// ModifierFunc defines the signature for a function that can be used to modify an AST node.
type ModifierFunc func(Node) Node

// Modify traverses an AST node and its children, applying the modifier function to each node in a depth-first manner.
// It returns the potentially modified node. This function is the core of the AST rewriting capabilities,
// allowing for transformations of the tree.
func Modify(node Node, modifier ModifierFunc) Node {
	switch node := node.(type) {

	case *Program:
		for i, statement := range node.Statements {
			node.Statements[i], _ = Modify(statement, modifier).(Statement)
		}

	case *ExpressionStatement:
		if node.Expression != nil {
			node.Expression, _ = Modify(node.Expression, modifier).(Expression)
		}

	case *AssignmentStatement:
		node.Left, _ = Modify(node.Left, modifier).(Expression)
		if node.Value != nil {
			node.Value, _ = Modify(node.Value, modifier).(Expression)
		}

	case *InfixExpression:
		node.Left, _ = Modify(node.Left, modifier).(Expression)
		node.Right, _ = Modify(node.Right, modifier).(Expression)

	case *MemberAccessExpression:
		node.Struct, _ = Modify(node.Struct, modifier).(Expression)
		node.Member, _ = Modify(node.Member, modifier).(*Identifier)

	case *PrefixExpression:
		node.Right, _ = Modify(node.Right, modifier).(Expression)

	case *IndexExpression:
		node.Left, _ = Modify(node.Left, modifier).(Expression)
		node.Index, _ = Modify(node.Index, modifier).(Expression)

	case *IfStatement:
		node.Condition, _ = Modify(node.Condition, modifier).(Expression)
		node.Consequence, _ = Modify(node.Consequence, modifier).(*BlockStatement)
		if node.Alternative != nil {
			node.Alternative, _ = Modify(node.Alternative, modifier).(Statement)
		}

	case *BlockStatement:
		for i, _ := range node.Statements {
			node.Statements[i], _ = Modify(node.Statements[i], modifier).(Statement)
		}

	case *ReturnStatement:
		if node.ReturnValue != nil {
			node.ReturnValue, _ = Modify(node.ReturnValue, modifier).(Expression)
		}

	case *VarDeclStatement:
		if node.DataType != nil {
			node.DataType, _ = Modify(node.DataType, modifier).(Expression)
		}
		if node.Value != nil {
			node.Value, _ = Modify(node.Value, modifier).(Expression)
		}
		if node.Location != nil {
			node.Location, _ = Modify(node.Location, modifier).(*AtDeclaration)
		}
		if node.Subrange != nil {
			node.Subrange, _ = Modify(node.Subrange, modifier).(Expression)
		}
		if node.StringLength != nil {
			node.StringLength, _ = Modify(node.StringLength, modifier).(Expression)
		}

	case *ForLoopStatement:
		node.ControlVar, _ = Modify(node.ControlVar, modifier).(*AssignmentStatement)
		node.EndValue, _ = Modify(node.EndValue, modifier).(Expression)
		if node.StepValue != nil {
			node.StepValue, _ = Modify(node.StepValue, modifier).(Expression)
		}
		node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)

	case *WhileStatement:
		node.Condition, _ = Modify(node.Condition, modifier).(Expression)
		node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)

	case *RepeatStatement:
		node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)
		node.Condition, _ = Modify(node.Condition, modifier).(Expression)

	case *CaseStatement:
		node.Expression, _ = Modify(node.Expression, modifier).(Expression)
		for i, _ := range node.Cases {
			node.Cases[i], _ = Modify(node.Cases[i], modifier).(*CaseBranch)
		}
		if node.Alternative != nil {
			node.Alternative, _ = Modify(node.Alternative, modifier).(*BlockStatement)
		}

	case *CaseBranch:
		for i, _ := range node.Values {
			node.Values[i], _ = Modify(node.Values[i], modifier).(Expression)
		}
		node.Consequence, _ = Modify(node.Consequence, modifier).(*BlockStatement)

	case *FunctionParameter:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		if node.DataType != nil {
			node.DataType, _ = Modify(node.DataType, modifier).(Expression)
		}

	case *FunctionLiteral:
		for i, _ := range node.Parameters {
			node.Parameters[i], _ = Modify(node.Parameters[i], modifier).(*FunctionParameter)
		}
		for i, _ := range node.VarInputs {
			node.VarInputs[i], _ = Modify(node.VarInputs[i], modifier).(*VarDeclStatement)
		}
		if node.ReturnType != nil {
			node.ReturnType, _ = Modify(node.ReturnType, modifier).(Expression)
		}
		node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)

	case *CallExpression:
		node.Function, _ = Modify(node.Function, modifier).(Expression)
		for i, _ := range node.Arguments {
			node.Arguments[i], _ = Modify(node.Arguments[i], modifier).(Expression)
		}

	case *ArrayLiteral:
		for i, _ := range node.Elements {
			node.Elements[i], _ = Modify(node.Elements[i], modifier).(Expression)
		}

	case *HashLiteral:
		newPairs := make(map[Expression]Expression)
		for key, val := range node.Pairs {
			newKey, _ := Modify(key, modifier).(Expression)
			newVal, _ := Modify(val, modifier).(Expression)
			newPairs[newKey] = newVal
		}
		node.Pairs = newPairs

	case *AtDeclaration:
		node.Location, _ = Modify(node.Location, modifier).(*DirectVariable)

	case *NamedArgument:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		node.Value, _ = Modify(node.Value, modifier).(Expression)

	case *OutputArgument:
		if node.Source != nil {
			node.Source, _ = Modify(node.Source, modifier).(*Identifier)
		}
		if node.Target != nil {
			node.Target, _ = Modify(node.Target, modifier).(Expression)
		}

	case *ProgramConfiguration:
		node.InstanceName, _ = Modify(node.InstanceName, modifier).(*Identifier)
		if node.TaskName != nil {
			node.TaskName, _ = Modify(node.TaskName, modifier).(*Identifier)
		}
		node.TypeName, _ = Modify(node.TypeName, modifier).(*Identifier)
		for i := range node.Parameters {
			node.Parameters[i], _ = Modify(node.Parameters[i], modifier).(Expression)
		}
		for i := range node.FbTasks {
			node.FbTasks[i], _ = Modify(node.FbTasks[i], modifier).(*FbTaskAssociation)
		}

	case *FbTaskAssociation:
		node.FbName, _ = Modify(node.FbName, modifier).(*Identifier)
		node.TaskName, _ = Modify(node.TaskName, modifier).(*Identifier)

	case *FunctionDeclaration:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		if node.ReturnType != nil {
			node.ReturnType, _ = Modify(node.ReturnType, modifier).(*TypeSpecifier)
		}
		for i := range node.VarInputs {
			node.VarInputs[i], _ = Modify(node.VarInputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarOutputs {
			node.VarOutputs[i], _ = Modify(node.VarOutputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarInOuts {
			node.VarInOuts[i], _ = Modify(node.VarInOuts[i], modifier).(*VarDeclStatement)
		}
		for i := range node.Vars {
			node.Vars[i], _ = Modify(node.Vars[i], modifier).(*VarDeclStatement)
		}
		if node.Body != nil {
			node.Body, _ = Modify(node.Body, modifier).(Statement)
		}
	case *ReferenceType:
		node.BaseType, _ = Modify(node.BaseType, modifier).(Expression)

	case *FunctionBlockDeclaration:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		if node.Extends != nil {
			node.Extends, _ = Modify(node.Extends, modifier).(*Identifier)
		}
		for i := range node.Implements {
			node.Implements[i], _ = Modify(node.Implements[i], modifier).(*Identifier)
		}
		for i := range node.VarInputs {
			node.VarInputs[i], _ = Modify(node.VarInputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarOutputs {
			node.VarOutputs[i], _ = Modify(node.VarOutputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarInOuts {
			node.VarInOuts[i], _ = Modify(node.VarInOuts[i], modifier).(*VarDeclStatement)
		}
		for i := range node.Vars {
			node.Vars[i], _ = Modify(node.Vars[i], modifier).(*VarDeclStatement)
		}
		if node.Body != nil {
			node.Body, _ = Modify(node.Body, modifier).(Statement)
		}

	case *InterfaceDeclaration:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		for i := range node.Methods {
			node.Methods[i], _ = Modify(node.Methods[i], modifier).(*MethodDeclaration)
		}

	case *MethodDeclaration:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		if node.ReturnType != nil {
			node.ReturnType, _ = Modify(node.ReturnType, modifier).(*TypeSpecifier)
		}
		for i := range node.VarInputs {
			node.VarInputs[i], _ = Modify(node.VarInputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarOutputs {
			node.VarOutputs[i], _ = Modify(node.VarOutputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarInOuts {
			node.VarInOuts[i], _ = Modify(node.VarInOuts[i], modifier).(*VarDeclStatement)
		}

	case *DereferenceExpression:
		node.Pointer, _ = Modify(node.Pointer, modifier).(Expression)

	case *MethodImplementation:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		if node.ReturnType != nil {
			node.ReturnType, _ = Modify(node.ReturnType, modifier).(*TypeSpecifier)
		}
		for i := range node.VarInputs {
			node.VarInputs[i], _ = Modify(node.VarInputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarOutputs {
			node.VarOutputs[i], _ = Modify(node.VarOutputs[i], modifier).(*VarDeclStatement)
		}
		for i := range node.VarInOuts {
			node.VarInOuts[i], _ = Modify(node.VarInOuts[i], modifier).(*VarDeclStatement)
		}
		for i := range node.Vars {
			node.Vars[i], _ = Modify(node.Vars[i], modifier).(*VarDeclStatement)
		}
		if node.Body != nil {
			node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)
		}

	case *ThisExpression:
		// No children to modify

	case *SuperExpression:
		// No children to modify

	case *StructLiteral:
		for i := range node.Initializers {
			node.Initializers[i], _ = Modify(node.Initializers[i], modifier).(Expression)
		}

	case *PropertyDeclaration:
		node.Name, _ = Modify(node.Name, modifier).(*Identifier)
		if node.DataType != nil {
			node.DataType, _ = Modify(node.DataType, modifier).(*TypeSpecifier)
		}
		if node.Getter != nil {
			node.Getter, _ = Modify(node.Getter, modifier).(*PropertyGetter)
		}
		if node.Setter != nil {
			node.Setter, _ = Modify(node.Setter, modifier).(*PropertySetter)
		}

	case *PropertyGetter:
		node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)
	case *PropertySetter:
		node.Body, _ = Modify(node.Body, modifier).(*BlockStatement)

	}
	return modifier(node)
}
