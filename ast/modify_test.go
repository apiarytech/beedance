package ast

import (
	"beedance/token"
	"reflect"
	"testing"
)

func TestModify(t *testing.T) {
	one := func() Expression { return &IntegerLiteral{Value: 1} }
	two := func() Expression { return &IntegerLiteral{Value: 2} }

	turnOneIntoTwo := func(node Node) Node {
		integer, ok := node.(*IntegerLiteral)
		if !ok {
			return node
		}

		if integer.Value != 1 {
			return node
		}

		integer.Value = 2
		return integer
	}

	tests := []struct {
		input    Node
		expected Node
	}{
		{
			one(),
			two(),
		},
		{
			&Program{
				Statements: []Statement{
					&ExpressionStatement{Expression: one()},
				},
			},
			&Program{
				Statements: []Statement{
					&ExpressionStatement{Expression: two()},
				},
			},
		},
		{
			&InfixExpression{Left: one(), Operator: "+", Right: two()},
			&InfixExpression{Left: two(), Operator: "+", Right: two()},
		},
		{
			&InfixExpression{Left: two(), Operator: "+", Right: one()},
			&InfixExpression{Left: two(), Operator: "+", Right: two()},
		},
		{
			&PrefixExpression{Operator: "-", Right: one()},
			&PrefixExpression{Operator: "-", Right: two()},
		},
		{
			&IndexExpression{Left: one(), Index: one()},
			&IndexExpression{Left: two(), Index: two()},
		},
		{
			&IfStatement{
				Condition: one(),
				Consequence: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
				Alternative: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
			},
			&IfStatement{
				Condition: two(),
				Consequence: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
				Alternative: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
			},
		},
		{
			&ReturnStatement{ReturnValue: one()},
			&ReturnStatement{ReturnValue: two()},
		},
		{
			&VarDeclStatement{
				DataType: one(),
				Value:    one(),
			},
			&VarDeclStatement{
				DataType: two(),
				Value:    two(),
			},
		},
		{
			&FunctionLiteral{
				Parameters: []*FunctionParameter{
					{
						Name:     &Identifier{Value: "x"},
						DataType: one(),
					},
				},
				ReturnType: one(),
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
			},
			&FunctionLiteral{
				Parameters: []*FunctionParameter{
					{
						Name:     &Identifier{Value: "x"},
						DataType: two(),
					},
				},
				ReturnType: two(),
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
			},
		},
		{
			&ArrayLiteral{Elements: []Expression{one(), one()}},
			&ArrayLiteral{Elements: []Expression{two(), two()}},
		},
		{
			&AssignmentStatement{Left: one(), Value: one()},
			&AssignmentStatement{Left: two(), Value: two()},
		},
		{
			&MemberAccessExpression{Struct: one(), Member: &Identifier{Value: "field"}},
			&MemberAccessExpression{Struct: two(), Member: &Identifier{Value: "field"}},
		},
		{
			&ForLoopStatement{
				ControlVar: &AssignmentStatement{Left: &Identifier{Value: "i"}, Value: one()},
				EndValue:   one(),
				StepValue:  one(),
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
			},
			&ForLoopStatement{
				ControlVar: &AssignmentStatement{Left: &Identifier{Value: "i"}, Value: two()},
				EndValue:   two(),
				StepValue:  two(),
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
			},
		},
		{
			&WhileStatement{
				Condition: one(),
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
			},
			&WhileStatement{
				Condition: two(),
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
			},
		},
		{
			&RepeatStatement{
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
				Condition: one(),
			},
			&RepeatStatement{
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
				Condition: two(),
			},
		},
		{
			&CaseStatement{
				Expression: one(),
				Cases: []*CaseBranch{
					{
						Values:      []Expression{one()},
						Consequence: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: one()}}},
					},
				},
				Alternative: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: one()}}},
			},
			&CaseStatement{
				Expression: two(),
				Cases: []*CaseBranch{
					{
						Values:      []Expression{two()},
						Consequence: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: two()}}},
					},
				},
				Alternative: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: two()}}},
			},
		},
		{
			&CallExpression{
				Function:  one(),
				Arguments: []Expression{one(), &NamedArgument{Value: one()}, &OutputArgument{Target: one()}},
			},
			&CallExpression{
				Function:  two(),
				Arguments: []Expression{two(), &NamedArgument{Value: two()}, &OutputArgument{Target: two()}},
			},
		},
		{
			&FunctionBlockDeclaration{
				Name:    &Identifier{Value: "fb"},
				Extends: &Identifier{Value: "Base"},
				Implements: []Expression{
					&Identifier{Value: "IBase"},
				},
				VarInputs: []*VarDeclStatement{
					{Value: one()},
				},
				Properties: []*PropertyDeclaration{
					{Getter: &PropertyGetter{Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: one()}}}}},
				},
				VarTemp: []*TempVarDeclaration{
					{Vars: []*VarDeclStatement{{Value: one()}}},
				},
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: one()},
					},
				},
			},
			&FunctionBlockDeclaration{
				Name:    &Identifier{Value: "fb"},
				Extends: &Identifier{Value: "Base"},
				Implements: []Expression{
					&Identifier{Value: "IBase"},
				},
				VarInputs: []*VarDeclStatement{
					{Value: two()},
				},
				Properties: []*PropertyDeclaration{
					{Getter: &PropertyGetter{Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: two()}}}}},
				},
				VarTemp: []*TempVarDeclaration{
					{Vars: []*VarDeclStatement{{Value: two()}}},
				},
				Body: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: two()},
					},
				},
			},
		},
		{
			&InterfaceDeclaration{
				Name: &Identifier{Value: "iface"},
				Methods: []*MethodDeclaration{
					{
						Name:       &Identifier{Value: "m"},
						ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
						VarInputs:  []*VarDeclStatement{{Value: one()}},
					},
				},
				Properties: []*PropertyDeclaration{
					{Name: &Identifier{Value: "p"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}},
				},
			},
			&InterfaceDeclaration{
				Name: &Identifier{Value: "iface"},
				Methods: []*MethodDeclaration{
					{
						Name:       &Identifier{Value: "m"},
						ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
						VarInputs:  []*VarDeclStatement{{Value: two()}},
					},
				},
				Properties: []*PropertyDeclaration{
					{Name: &Identifier{Value: "p"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}},
				},
			},
		},
		{
			&MethodDeclaration{
				ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "one"}},
				VarInputs:  []*VarDeclStatement{{Value: one()}},
				VarOutputs: []*VarDeclStatement{{Value: one()}},
				VarInOuts:  []*VarDeclStatement{{Value: one()}},
			},
			&MethodDeclaration{
				ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "one"}},
				VarInputs:  []*VarDeclStatement{{Value: two()}},
				VarOutputs: []*VarDeclStatement{{Value: two()}},
				VarInOuts:  []*VarDeclStatement{{Value: two()}},
			},
		},
		{
			&MethodImplementation{
				ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "one"}},
				VarInputs:  []*VarDeclStatement{{Value: one()}},
				Body:       &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: one()}}},
			},
			&MethodImplementation{
				ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "one"}},
				VarInputs:  []*VarDeclStatement{{Value: two()}},
				Body:       &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: two()}}},
			},
		},
		{
			&PropertyDeclaration{
				DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
				Getter:   &PropertyGetter{Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: one()}}}},
				Setter:   &PropertySetter{Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: one()}}}},
			},
			&PropertyDeclaration{
				DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
				Getter:   &PropertyGetter{Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: two()}}}},
				Setter:   &PropertySetter{Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: two()}}}},
			},
		},
		{
			&DereferenceExpression{Pointer: one()},
			&DereferenceExpression{Pointer: two()},
		},
		{
			&NamespaceDeclaration{
				Name:       one(),
				Statements: []Statement{&ExpressionStatement{Expression: one()}},
			},
			&NamespaceDeclaration{
				Name:       two(),
				Statements: []Statement{&ExpressionStatement{Expression: two()}},
			},
		},
		{
			&ReferenceType{BaseType: one()},
			&ReferenceType{BaseType: two()},
		},
		{
			&ExternalVarDeclaration{
				Vars: []*VarDeclStatement{{Value: one()}},
			},
			&ExternalVarDeclaration{
				Vars: []*VarDeclStatement{{Value: two()}},
			},
		},
		{
			&TempVarDeclaration{
				Vars: []*VarDeclStatement{{Value: one()}},
			},
			&TempVarDeclaration{
				Vars: []*VarDeclStatement{{Value: two()}},
			},
		},
	}

	for _, tt := range tests {
		modified := Modify(tt.input, turnOneIntoTwo)

		equal := reflect.DeepEqual(modified, tt.expected)
		if !equal {
			t.Errorf("modification failed.\ngot =%#v\nwant=%#v",
				modified, tt.expected)
		}
	}

	// Another separate test because `reflect.DeepEqual` with maps and
	// interfaces and pointers is DeeplyWeird™
	hashLiteral := &HashLiteral{
		Pairs: map[Expression]Expression{
			one(): one(),
		},
	}

	Modify(hashLiteral, turnOneIntoTwo)

	for key, val := range hashLiteral.Pairs {
		key, _ := key.(*IntegerLiteral)
		if key.Value != 2 {
			t.Errorf("value is not %d, got=%d", 2, key.Value)
		}
		val, _ := val.(*IntegerLiteral)
		if val.Value != 2 {
			t.Errorf("value is not %d, got=%d", 2, val.Value)
		}
	}

	// Test for AtDeclaration
	atInput := &AtDeclaration{
		Location: &DirectVariable{Address: "old"},
	}
	atExpected := &AtDeclaration{
		Location: &DirectVariable{Address: "new"},
	}

	atModifier := func(node Node) Node {
		if dv, ok := node.(*DirectVariable); ok {
			if dv.Address == "old" {
				dv.Address = "new"
			}
		}
		return node
	}

	modified := Modify(atInput, atModifier)

	if !reflect.DeepEqual(modified, atExpected) {
		t.Errorf("AtDeclaration modification failed. got=%#v, want=%#v", modified, atExpected)
	}

	// Test for OutputArgument.Source
	oaInput := &OutputArgument{
		Source: &Identifier{Value: "old_source"},
	}
	oaExpected := &OutputArgument{
		Source: &Identifier{Value: "new_source"},
	}

	identModifier := func(node Node) Node {
		if id, ok := node.(*Identifier); ok {
			if id.Value == "old_source" {
				id.Value = "new_source"
			}
		}
		return node
	}

	modified = Modify(oaInput, identModifier)

	if !reflect.DeepEqual(modified, oaExpected) {
		t.Errorf("OutputArgument.Source modification failed. got=%#v, want=%#v", modified, oaExpected)
	}

	// Test for VarDeclStatement.Location
	vdsInput := &VarDeclStatement{
		Location: &AtDeclaration{
			Location: &DirectVariable{Address: "old_loc"},
		},
	}
	vdsExpected := &VarDeclStatement{
		Location: &AtDeclaration{
			Location: &DirectVariable{Address: "new_loc"},
		},
	}

	locModifier := func(node Node) Node {
		if dv, ok := node.(*DirectVariable); ok {
			if dv.Address == "old_loc" {
				dv.Address = "new_loc"
			}
		}
		return node
	}

	modified = Modify(vdsInput, locModifier)
	if !reflect.DeepEqual(modified, vdsExpected) {
		t.Errorf("VarDeclStatement.Location modification failed. got=%#v, want=%#v", modified, vdsExpected)
	}
}
