package parser

import (
	"beedance/ast"
	"beedance/lexer"
	"testing"
)

func TestReferenceToTypeDeclaration(t *testing.T) {
	input := `
		FUNCTION_BLOCK MyFB
			VAR_INPUT
				DataSource : REFERENCE TO BigDataStructure;
			END_VAR
		END_FUNCTION_BLOCK
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestReferenceToTypeDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(fb.VarInputs) != 1 {
		t.Fatalf("Expected 1 VAR_INPUT declaration, got %d", len(fb.VarInputs))
	}

	decl := fb.VarInputs[0]
	if decl.Name.Value != "DataSource" {
		t.Errorf("Expected variable name 'DataSource', got %s", decl.Name.Value)
	}

	refType, ok := decl.DataType.(*ast.ReferenceType)
	if !ok {
		t.Fatalf("Expected DataType to be *ast.ReferenceType, got %T", decl.DataType)
	}

	if refType.BaseType.String() != "BigDataStructure" {
		t.Errorf("Expected reference base type to be 'BigDataStructure', got %s", refType.BaseType.String())
	}
}

func TestFunctionBlockExtends(t *testing.T) {
	input := `
		FUNCTION_BLOCK DerivedFB EXTENDS BaseFB
			VAR_INPUT
				NewInput : BOOL;
			END_VAR
		END_FUNCTION_BLOCK
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionBlockExtends", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if fb.Name.Value != "DerivedFB" {
		t.Errorf("Function block name is not 'DerivedFB'. got=%s", fb.Name.Value)
	}

	if fb.Extends == nil {
		t.Fatal("fb.Extends is nil, expected a base FB name")
	}

	if fb.Extends.String() != "BaseFB" {
		t.Errorf("Expected Extends name to be 'BaseFB', got %s", fb.Extends.String())
	}

	if len(fb.VarInputs) != 1 || fb.VarInputs[0].Name.Value != "NewInput" {
		t.Errorf("Failed to parse VAR_INPUT block in derived FB.")
	}
}

func TestFunctionBlockImplements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, fb *ast.FunctionBlockDeclaration)
	}{
		{
			name: "Implements single interface",
			input: `
				FUNCTION_BLOCK MyFB IMPLEMENTS IMyInterface
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, fb *ast.FunctionBlockDeclaration) {
				if fb.Name.Value != "MyFB" {
					t.Errorf("Name not 'MyFB', got %s", fb.Name.Value)
				}
				if len(fb.Implements) != 1 {
					t.Fatalf("Expected 1 implemented interface, got %d", len(fb.Implements))
				}
				if fb.Implements[0].String() != "IMyInterface" {
					t.Errorf("Expected interface 'IMyInterface', got %s", fb.Implements[0].String())
				}
				if fb.Extends != nil {
					t.Errorf("Extends should be nil")
				}
			},
		},
		{
			name: "Implements multiple interfaces",
			input: `
				FUNCTION_BLOCK MyFB IMPLEMENTS IMyInterface1, IMyInterface2
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, fb *ast.FunctionBlockDeclaration) {
				if len(fb.Implements) != 2 {
					t.Fatalf("Expected 2 implemented interfaces, got %d", len(fb.Implements))
				}
				if fb.Implements[0].String() != "IMyInterface1" || fb.Implements[1].String() != "IMyInterface2" {
					t.Errorf("Incorrect interfaces parsed. Got %s, %s", fb.Implements[0].String(), fb.Implements[1].String())
				}
			},
		},
		{
			name: "Extends and Implements",
			input: `
				FUNCTION_BLOCK DerivedFB EXTENDS BaseFB IMPLEMENTS IMyInterface
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, fb *ast.FunctionBlockDeclaration) {
				if fb.Extends == nil || fb.Extends.String() != "BaseFB" {
					t.Errorf("Expected Extends 'BaseFB', got %v", fb.Extends)
				}
				if len(fb.Implements) != 1 || fb.Implements[0].String() != "IMyInterface" {
					t.Errorf("Expected Implements 'IMyInterface', got %v", fb.Implements)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p, tt.name, tt.input)
			fb := program.Statements[0].(*ast.FunctionBlockDeclaration)
			tt.check(t, fb)
		})
	}
}

func TestOOPFeaturesThisAndSuper(t *testing.T) {
	input := `
		FUNCTION_BLOCK Derived EXTENDS Base
			METHOD DoIt : INT
				// CORRECT: Use THIS^. to dereference and access own member
				THIS^.y := 10;
				// CORRECT: Use SUPER^.MethodName() to call the parent's logic
				DoIt := SUPER^.DoIt() + THIS^.y;
			END_METHOD
		END_FUNCTION_BLOCK
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestOOPFeaturesThisAndSuper", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	// The body of a FB is a BlockStatement containing methods, etc.
	body, ok := fb.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("FB body is not a BlockStatement. got=%T", fb.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Expected 1 method in FB body, got %d", len(body.Statements))
	}

	methodImpl, ok := body.Statements[0].(*ast.MethodImplementation)
	if !ok {
		t.Fatalf("Statement 0 is not a MethodImplementation. got=%T", body.Statements[0])
	}

	if len(methodImpl.Body.Statements) != 2 {
		t.Fatalf("Expected 2 statements in method body, got %d", len(methodImpl.Body.Statements))
	}

	// Check the assignment to THIS^.y
	assign1, ok := methodImpl.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Statement 0 is not an AssignmentStatement. got=%T", methodImpl.Body.Statements[0])
	}
	memberAccess, ok := assign1.Left.(*ast.MemberAccessExpression)
	if !ok {
		t.Fatalf("LHS of first assignment is not MemberAccessExpression. got=%T", assign1.Left)
	}
	derefExp, ok := memberAccess.Struct.(*ast.DereferenceExpression)
	if !ok {
		t.Fatalf("Struct part of member access is not DereferenceExpression. got=%T", memberAccess.Struct)
	}
	if _, ok := derefExp.Pointer.(*ast.ThisExpression); !ok {
		t.Errorf("Pointer of dereference is not ThisExpression. got=%T", derefExp.Pointer)
	}

	// Check the call to SUPER^.DoIt()
	assign2 := methodImpl.Body.Statements[1].(*ast.AssignmentStatement)
	infix, _ := assign2.Value.(*ast.InfixExpression)
	call, _ := infix.Left.(*ast.CallExpression)
	memberAccess2, ok := call.Function.(*ast.MemberAccessExpression)
	if !ok {
		t.Fatalf("Function in call is not MemberAccessExpression. got=%T", call.Function)
	}
	derefExp2, ok := memberAccess2.Struct.(*ast.DereferenceExpression)
	if !ok {
		t.Fatalf("Struct part of member access is not DereferenceExpression. got=%T", memberAccess2.Struct)
	}
	if _, ok := derefExp2.Pointer.(*ast.SuperExpression); !ok {
		t.Errorf("Pointer of dereference is not SuperExpression. got=%T", derefExp2.Pointer)
	}
}

func TestInterfaceDeclaration(t *testing.T) {
	input := `
		INTERFACE IMyInterface
			METHOD MyMethod1 : BOOL
				VAR_INPUT
					In1 : INT;
				END_VAR
			END_METHOD

			METHOD MyMethod2 : REAL
				VAR_IN_OUT
					InOut1 : LREAL;
				END_VAR
			END_METHOD
		END_INTERFACE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestInterfaceDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.InterfaceDeclaration. got=%T", program.Statements[0])
	}

	if iface.Name.Value != "IMyInterface" {
		t.Errorf("Interface name is not 'IMyInterface'. got=%s", iface.Name.Value)
	}

	if len(iface.Methods) != 2 {
		t.Fatalf("Expected 2 methods, got %d", len(iface.Methods))
	}

	// Check first method
	method1 := iface.Methods[0]
	if method1.Name.Value != "MyMethod1" {
		t.Errorf("Method 1 name is not 'MyMethod1'. got=%s", method1.Name.Value)
	}
	if method1.ReturnType == nil || method1.ReturnType.String() != "BOOL" {
		t.Errorf("Method 1 return type is not 'BOOL'. got=%v", method1.ReturnType)
	}
	if len(method1.VarInputs) != 1 {
		t.Fatalf("Expected 1 VAR_INPUT for method 1, got %d", len(method1.VarInputs))
	}
	if method1.VarInputs[0].Name.Value != "In1" {
		t.Errorf("Failed to parse VAR_INPUT for method 1.")
	}

	// Check second method
	method2 := iface.Methods[1]
	if method2.Name.Value != "MyMethod2" {
		t.Errorf("Method 2 name is not 'MyMethod2'. got=%s", method2.Name.Value)
	}
	if method2.ReturnType == nil || method2.ReturnType.String() != "REAL" {
		t.Errorf("Method 2 return type is not 'REAL'. got=%v", method2.ReturnType)
	}
	if len(method2.VarInOuts) != 1 {
		t.Fatalf("Expected 1 VAR_IN_OUT for method 2, got %d", len(method2.VarInOuts))
	}
	if method2.VarInOuts[0].Name.Value != "InOut1" {
		t.Errorf("Failed to parse VAR_IN_OUT for method 2.")
	}
}

func TestAbstractFunctionBlock(t *testing.T) {
	input := `
		FUNCTION_BLOCK ABSTRACT MyAbstractFB
			METHOD ABSTRACT DoIt : INT
				VAR_INPUT
					In : BOOL;
				END_VAR
			END_METHOD

			PROPERTY ABSTRACT Value : REAL
			END_PROPERTY

			PROPERTY ConcreteValue : INT
				GET
					ConcreteValue := 10;
				END_GET
			END_PROPERTY
		END_FUNCTION_BLOCK
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestAbstractFunctionBlock", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if !fb.IsAbstract {
		t.Errorf("Function block should be abstract")
	}
	if fb.Name.Value != "MyAbstractFB" {
		t.Errorf("Function block name is not 'MyAbstractFB'. got=%s", fb.Name.Value)
	}

	body, ok := fb.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("FB body is not a BlockStatement. got=%T", fb.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Expected 1 method in FB body, got %d", len(body.Statements))
	}

	// Check abstract method
	method, ok := body.Statements[0].(*ast.MethodImplementation)
	if !ok {
		t.Fatalf("Statement 0 is not a MethodImplementation. got=%T", body.Statements[0])
	}
	if !method.IsAbstract {
		t.Errorf("Method 'DoIt' should be abstract")
	}
	if method.Name.Value != "DoIt" {
		t.Errorf("Method name is not 'DoIt'. got=%s", method.Name.Value)
	}
	if method.Body != nil {
		t.Errorf("Abstract method should not have a body")
	}
	if len(method.VarInputs) != 1 {
		t.Errorf("Abstract method should have its VAR_INPUTs parsed")
	}

	// Check properties
	if len(fb.Properties) != 2 {
		t.Fatalf("Expected 2 properties, got %d", len(fb.Properties))
	}

	// Check abstract property
	prop1 := fb.Properties[0]
	if !prop1.IsAbstract {
		t.Errorf("Property 'Value' should be abstract")
	}
	if prop1.Name.Value != "Value" {
		t.Errorf("Property 1 name is not 'Value'. got=%s", prop1.Name.Value)
	}
	if prop1.Getter != nil || prop1.Setter != nil {
		t.Errorf("Abstract property should not have GET or SET blocks")
	}

	// Check concrete property
	prop2 := fb.Properties[1]
	if prop2.IsAbstract {
		t.Errorf("Property 'ConcreteValue' should not be abstract")
	}
	if prop2.Getter == nil {
		t.Errorf("Concrete property should have a GET block")
	}
}

func TestNamespaceDeclarationParsing(t *testing.T) {
	input := `
		NAMESPACE MyLib
			INTERFACE IGreeter
				METHOD Greet : STRING;
			END_INTERFACE

			FUNCTION_BLOCK Greeter IMPLEMENTS IGreeter
				METHOD Greet : STRING
					Greet := THIS.Greeting;
				END_METHOD
			END_FUNCTION_BLOCK
		END_NAMESPACE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestNamespaceDeclarationParsing", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	ns, ok := program.Statements[0].(*ast.NamespaceDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.NamespaceDeclaration. got=%T", program.Statements[0])
	}

	if ns.Name.String() != "MyLib" {
		t.Errorf("namespace name is not 'MyLib'. got=%s", ns.Name.String())
	}

	if len(ns.Statements) != 2 {
		t.Fatalf("namespace should contain 2 statements. got=%d", len(ns.Statements))
	}

	// Check the interface
	iface, ok := ns.Statements[0].(*ast.InterfaceDeclaration)
	if !ok {
		t.Fatalf("Statement 0 is not InterfaceDeclaration. got=%T", ns.Statements[0])
	}
	if iface.Name.Value != "IGreeter" {
		t.Errorf("Interface name is not 'IGreeter'. got=%s", iface.Name.Value)
	}

	// Check the function block
	fb, ok := ns.Statements[1].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("Statement 1 is not FunctionBlockDeclaration. got=%T", ns.Statements[1])
	}
	if fb.Name.Value != "Greeter" {
		t.Errorf("Function block name is not 'Greeter'. got=%s", fb.Name.Value)
	}
}

func TestAbstractErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "Abstract function",
			input:         `ABSTRACT FUNCTION MyFunc : INT; END_FUNCTION`,
			expectedError: "no prefix parse function for ABSTRACT found",
		},
		{
			name:          "Abstract method in concrete FB",
			input:         `FUNCTION_BLOCK MyFB METHOD ABSTRACT MyMethod : INT; END_METHOD END_FUNCTION_BLOCK`,
			expectedError: "abstract members are not allowed in a non-abstract function block",
		},
		{
			name:          "Abstract method with body",
			input:         `FUNCTION_BLOCK ABSTRACT MyFB METHOD ABSTRACT MyMethod : INT MyMethod := 1; END_METHOD END_FUNCTION_BLOCK`,
			expectedError: "abstract method cannot have a body",
		},
		{
			name:          "Abstract property with GET block",
			input:         `FUNCTION_BLOCK ABSTRACT MyFB PROPERTY ABSTRACT MyProp : INT GET END_GET END_PROPERTY END_FUNCTION_BLOCK`,
			expectedError: "abstract property cannot have GET or SET implementations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			assertErrorContains(t, p.Errors(), tt.expectedError)
		})
	}
}

func TestFunctionBlockProperty(t *testing.T) {
	input := `
		FUNCTION_BLOCK MyFB
			VAR
				internalvar : INT;
			END_VAR

			PROPERTY MyProp : INT
				GET
					MyProp := THIS.internalvar;
				END_GET
				SET
					THIS.internalvar := value;
				END_SET
			END_PROPERTY
		END_FUNCTION_BLOCK
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionBlockProperty", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(fb.Properties) != 1 {
		t.Fatalf("Expected 1 property, got %d", len(fb.Properties))
	}

	prop := fb.Properties[0]
	if prop.Name.Value != "MyProp" {
		t.Errorf("Property name is not 'MyProp'. got=%s", prop.Name.Value)
	}
	if prop.DataType.String() != "INT" {
		t.Errorf("Property data type is not 'INT'. got=%s", prop.DataType.String())
	}

	// Check GET block
	if prop.Getter == nil {
		t.Fatal("Property getter is nil")
	}
	if len(prop.Getter.Body.Statements) != 1 {
		t.Fatalf("Expected 1 statement in GET body, got %d", len(prop.Getter.Body.Statements))
	}
	getStmt, ok := prop.Getter.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok || getStmt.Left.String() != "MyProp" || getStmt.Value.String() != "THIS.internalvar" {
		t.Errorf("Incorrect statement in GET body. want='MyProp := THIS.internalvar', got='%s := %s'", getStmt.Left.String(), getStmt.Value.String())
	}

	// Check SET block
	if prop.Setter == nil {
		t.Fatal("Property setter is nil")
	}
	if len(prop.Setter.Body.Statements) != 1 {
		t.Fatalf("Expected 1 statement in SET body, got %d", len(prop.Setter.Body.Statements))
	}
	setStmt, ok := prop.Setter.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok || setStmt.Left.String() != "THIS.internalvar" || setStmt.Value.String() != "value" {
		t.Errorf("Incorrect statement in SET body. want='THIS.internalvar := value', got='%s := %s'", setStmt.Left.String(), setStmt.Value.String())
	}
}

func TestFunctionBlockWithMethodCall(t *testing.T) {
	input := `
			FUNCTION_BLOCK MyFBWithMethod
				VAR
					Internalvar: INT := 1;
				END_VAR

				METHOD MyMethod : INT
					VAR_INPUT
						MethodIn : INT;
					END_VAR
					MyMethod := MethodIn * 2;
				END_METHOD
				
				Internalvar := THIS^.MyMethod(Internalvar) + 1;
			END_FUNCTION_BLOCK
			`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionBlockWithMethodCall", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if fb.Name.Value != "MyFBWithMethod" {
		t.Errorf("FB name is not 'MyFBWithMethod'. got=%s", fb.Name.Value)
	}

	// The body of a FB is a BlockStatement containing methods and then the logic
	body, ok := fb.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("FB body is not a BlockStatement. got=%T", fb.Body)
	}

	if len(body.Statements) != 2 {
		t.Fatalf("Expected 2 statements in FB body (method + assignment), got %d", len(body.Statements))
	}

	// Check METHOD
	methodImpl, ok := body.Statements[0].(*ast.MethodImplementation)
	if !ok {
		t.Fatalf("Statement 0 is not a MethodImplementation. got=%T", body.Statements[0])
	}
	if methodImpl.Name.Value != "MyMethod" {
		t.Errorf("Method name is not 'MyMethod'. got=%s", methodImpl.Name.Value)
	}

	// Check main logic assignment
	assignStmt, ok := body.Statements[1].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Statement 1 is not an AssignmentStatement. got=%T", body.Statements[1])
	}

	// Check RHS of assignment: THIS^.MyMethod(Internal) + 1
	infix, ok := assignStmt.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("Assignment value is not InfixExpression. got=%T", assignStmt.Value)
	}

	// Check call expression: THIS^.MyMethod(Internal)
	call, ok := infix.Left.(*ast.CallExpression)
	if !ok {
		t.Fatalf("LHS of infix is not CallExpression. got=%T", infix.Left)
	}

	// Check function being called: THIS^.MyMethod
	memberAccess, ok := call.Function.(*ast.MemberAccessExpression)
	if !ok {
		t.Fatalf("Function of call is not MemberAccessExpression. got=%T", call.Function)
	}

	// Check THIS^
	deref, ok := memberAccess.Struct.(*ast.DereferenceExpression)
	if !ok {
		t.Fatalf("Struct part of member access is not DereferenceExpression. got=%T", memberAccess.Struct)
	}
	if _, ok := deref.Pointer.(*ast.ThisExpression); !ok {
		t.Fatalf("Pointer of dereference is not ThisExpression. got=%T", deref.Pointer)
	}
}
