package parser

import (
	"fmt"
	"strings"
	"testing"

	"beedance/ast"
	"beedance/lexer"
	"beedance/token"
)

func TestSingleVarDeclStatement(t *testing.T) {
	input := `VAR myVar : INT := 5;`

	l := lexer.New(input)
	p := New(l) // cspell:disable-line
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestSingleVarDeclStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Declarations) != 1 {
		t.Fatalf("Expected 1 var declaration. got=%d", len(stmt.Declarations))
	}

	decl := stmt.Declarations[0]

	if decl.Name.Value != "myVar" {
		t.Errorf("decl.Name.Value not 'myVar'. got=%s", decl.Name.Value)
	}

	ts, ok := decl.DataType.(*ast.TypeSpecifier)
	if !ok {
		t.Fatalf("decl.DataType is not *ast.TypeSpecifier. got=%T", decl.DataType)
	}

	if ts.TokenLiteral() != "INT" {
		t.Errorf("decl.DataType not 'INT'. got=%s", decl.DataType)
	}

	val, ok := decl.Value.(*ast.IntegerLiteral)
	if !ok || val.Value != 5 {
		t.Errorf("decl.Value is not 5. got=%v", decl.Value)
	}
}

func TestNamedArgumentParsing(t *testing.T) {
	input := `MyFunc(In1 := 10, Out1 => Res1);`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestNamedArgumentParsing", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	call, ok := stmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.CallExpression. got=%T", stmt.Expression)
	}

	arg1, ok := call.Arguments[0].(*ast.NamedArgument)
	if !ok || arg1.Name.Value != "In1" || arg1.Value.(*ast.IntegerLiteral).Value != 10 {
		t.Errorf("First argument is not a correct NamedArgument")
	}

	arg2, ok := call.Arguments[1].(*ast.OutputArgument)
	if !ok || arg2.Source.Value != "Out1" || arg2.Target.(*ast.Identifier).Value != "Res1" {
		t.Errorf("Second argument is not a correct OutputArgument")
	}
}

func TestVarDeclStatements(t *testing.T) {
	tests := []struct {
		input              string
		expectedIdentifier string
		expectedDataType   string
		expectedValue      interface{}
		isConstant         bool
		isRetain           bool
		isNonRetain        bool
	}{
		{"VAR myVar : INT := 5; END_VAR", "myVar", "INT", 5, false, false, false},
		{"VAR myFlag : BOOL := true; END_VAR", "myFlag", "BOOL", true, false, false, false},
		{"VAR anotherVar : REAL; END_VAR", "anotherVar", "REAL", nil, false, false, false},
		{"VAR CONSTANT MyConst : INT := 100; END_VAR", "MyConst", "INT", 100, true, false, false},
		{"VAR RETAIN RetainVar : INT := 42; END_VAR", "RetainVar", "INT", 42, false, true, false},
		{"VAR NON_RETAIN NonRetainVar : BOOL; END_VAR", "NonRetainVar", "BOOL", nil, false, false, true},
		{`VAR myVar AT %IX0.0 : BOOL; END_VAR`, "myVar", "BOOL", nil, false, false, false},
		{"VAR myUpperBool : BOOL := TRUE; END_VAR", "myUpperBool", "BOOL", true, false, false, false},
		{`VAR
			MultiVar1 : INT;               //Test 1
			MultiVar2 : BOOL := TRUE;      //Test 2
		END_VAR`,
			"MultiVar1", "INT", nil, false, false, false,
		},
	}

	for i, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Errorf("test case %d: parser has %d errors", i, len(p.Errors()))
			for _, msg := range p.Errors() {
				t.Errorf("test case %d: parser error: %q", i, msg)
			}
			t.FailNow()
		}

		if len(program.Statements) != 1 {
			t.Fatalf("test case %d: program.Statements does not contain 1 statement. got=%d", i, len(program.Statements))
		}

		block, ok := program.Statements[0].(*ast.VarBlockDeclaration)
		if !ok {
			t.Fatalf("test case %d: program.Statements[0] is not ast.VarBlockDeclaration. got=%T", i, program.Statements[0])
		}

		// For the multi-line test case, we expect 2 declarations.
		if tt.expectedIdentifier == "MultiVar1" {
			if len(block.Declarations) != 2 {
				t.Fatalf("test case %d: block.Declarations does not contain 2 statements for multi-line test. got=%d", i, len(block.Declarations))
			}
			// Test first declaration in multi-line block
			if !testVarDeclStatement(t, block.Declarations[0], "MultiVar1", "INT") {
				return
			}
			// Test second declaration in multi-line block
			if !testVarDeclStatement(t, block.Declarations[1], "MultiVar2", "BOOL") || !testLiteralExpression(t, block.Declarations[1].Value, true) {
				return
			}
			continue // Skip the generic checks below for this specific multi-line case
		} else if len(block.Declarations) != 1 {
			t.Fatalf("test case %d: block.Declarations does not contain 1 statement. got=%d", i, len(block.Declarations))
		}
		stmt := block.Declarations[0] // We'll check the first declaration for all tests

		if !testVarDeclStatement(t, stmt, tt.expectedIdentifier, tt.expectedDataType) {
			return
		}

		if stmt.IsConstant != tt.isConstant {
			t.Fatalf("test case %d: stmt.IsConstant is not %v. got=%v", i, tt.isConstant, stmt.IsConstant)
		}

		if stmt.IsRetain != tt.isRetain {
			t.Fatalf("test case %d: stmt.IsRetain is not %v. got=%v", i, tt.isRetain, stmt.IsRetain)
		}

		if stmt.IsNonRetain != tt.isNonRetain {
			t.Fatalf("test case %d: stmt.IsNonRetain is not %v. got=%v", i, tt.isNonRetain, stmt.IsNonRetain)
		}

		val := stmt.Value
		if tt.expectedValue == nil {
			continue
		}
		if !testLiteralExpression(t, val, tt.expectedValue) {
			return
		}
	}
}

func TestTypeDeclarations(t *testing.T) {
	input := `
		TYPE
			MyInt : INT;
			MyBool : BOOL;
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestStepStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.TypeBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TypeBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Declarations) != 2 {
		t.Fatalf("Expected 2 type declarations. got=%d", len(stmt.Declarations))
	}

	decl1 := stmt.Declarations[0]
	if decl1.Name.Value != "MyInt" || decl1.DataType.String() != "INT" {
		t.Errorf("Invalid type declaration 1. got=%s", decl1.String())
	}

	decl2 := stmt.Declarations[1]
	if decl2.Name.Value != "MyBool" || decl2.DataType.String() != "BOOL" {
		t.Errorf("Invalid type declaration 2. got=%s", decl2.String())
	}
}

func TestStructTypeDeclaration(t *testing.T) {
	input := `
		TYPE
			MyStruct : STRUCT
				Field1 : INT;
				Field2 : BOOL;
			END_STRUCT;
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestWhileStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	typeBlock, ok := program.Statements[0].(*ast.TypeBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TypeBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(typeBlock.Declarations) != 1 {
		t.Fatalf("Expected 1 type declaration. got=%d", len(typeBlock.Declarations))
	}

	typeDecl := typeBlock.Declarations[0]
	if typeDecl.Name.Value != "MyStruct" {
		t.Fatalf("Type name is not 'MyStruct'. got=%s", typeDecl.Name.Value)
	}

	structDef, ok := typeDecl.DataType.(*ast.StructDefinition)
	if !ok {
		t.Fatalf("DataType is not ast.StructDefinition. got=%T", typeDecl.DataType)
	}

	if len(structDef.Members) != 2 {
		t.Fatalf("Struct does not have 2 members. got=%d", len(structDef.Members))
	}

	// Test first member
	member1 := structDef.Members[0]
	if !testVarDeclStatement(t, member1, "Field1", "INT") {
		return
	}

	// Test second member
	member2 := structDef.Members[1]
	if !testVarDeclStatement(t, member2, "Field2", "BOOL") {
		return
	}
}

func TestStructMemberInitializationError(t *testing.T) {
	input := `
		TYPE
			MyStruct : STRUCT
				Field1 : INT;
				Field2 : BOOL := TRUE;
			END_STRUCT;
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("Expected an error for struct member initialization, but got none")
	}

	expectedError := "initialization is not allowed for struct members"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}
}

func TestComplexTypeBlockDeclaration(t *testing.T) {
	input := `
		TYPE
			MyInteger   : INT := 10;
			MySubrange  : INT (0..100);
			MyArray     : ARRAY [1..10] OF BOOL;
			MyStruct    : STRUCT 
				Field1:INT; 
			END_STRUCT;
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestForLoopStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	typeBlock, ok := program.Statements[0].(*ast.TypeBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TypeBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(typeBlock.Declarations) != 4 {
		t.Fatalf("Expected 4 type declarations. got=%d", len(typeBlock.Declarations))
	}

	// 1. Test MyInteger : INT := 10;
	decl1 := typeBlock.Declarations[0]
	if decl1.Name.Value != "MyInteger" {
		t.Errorf("Invalid name for declaration 1. got=%s", decl1.Name.Value)
	}
	if decl1.DataType.String() != "INT" {
		t.Errorf("Invalid data type for declaration 1. got=%s", decl1.DataType.String())
	}
	if !testIntegerLiteral(t, decl1.InitialValue, 10) {
		t.Errorf("Invalid initial value for declaration 1.")
	}

	// 2. Test MySubrange : INT (0..100);
	decl2 := typeBlock.Declarations[1]
	if decl2.Name.Value != "MySubrange" {
		t.Errorf("Invalid name for declaration 2. got=%s", decl2.Name.Value)
	}
	if decl2.DataType.String() != "INT" {
		t.Errorf("Invalid data type for declaration 2. got=%s", decl2.DataType.String())
	}
	if !testInfixExpression(t, 0, decl2.Subrange, 0, "..", 100) {
		t.Errorf("Invalid subrange for declaration 2.")
	}

	// 3. Test MyArray : ARRAY [1..10] OF BOOL;
	decl3 := typeBlock.Declarations[2]
	if decl3.Name.Value != "MyArray" {
		t.Errorf("Invalid name for declaration 3. got=%s", decl3.Name.Value)
	}
	if _, ok := typeBlock.Declarations[2].DataType.(*ast.ArrayDefinition); !ok {
		t.Errorf("DataType for declaration 3 is not ArrayDefinition. got=%T", decl3.DataType)
	}

	// 4. Test MyStruct : STRUCT Field1:INT; END_STRUCT;
	decl4 := typeBlock.Declarations[3]
	if decl4.Name.Value != "MyStruct" {
		t.Errorf("Invalid name for declaration 4. got=%s", decl4.Name.Value)
	}
	structDef, ok := decl4.DataType.(*ast.StructDefinition)
	if !ok {
		t.Errorf("DataType for declaration 4 is not StructDefinition. got=%T", decl4.DataType)
	}
	if len(structDef.Members) != 1 {
		t.Errorf("Expected 1 member in MyStruct. got=%d", len(structDef.Members))
	}
}

func TestArrayTypeDeclaration(t *testing.T) {
	input := `
		TYPE
			MyArray : ARRAY [1..5, 1..10, 1..20] OF INT;
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestArrayTypeDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	typeBlock, ok := program.Statements[0].(*ast.TypeBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TypeBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(typeBlock.Declarations) != 1 {
		t.Fatalf("Expected 1 type declaration. got=%d", len(typeBlock.Declarations))
	}

	typeDecl := typeBlock.Declarations[0]
	if typeDecl.Name.Value != "MyArray" {
		t.Fatalf("Type name is not 'MyArray'. got=%s", typeDecl.Name.Value)
	}

	arrayDef, ok := typeDecl.DataType.(*ast.ArrayDefinition)
	if !ok {
		t.Fatalf("DataType is not ast.ArrayDefinition. got=%T", typeDecl.DataType)
	}

	if len(arrayDef.Ranges) != 3 {
		t.Fatalf("Expected 3 ranges for multi-dimensional array. got=%d", len(arrayDef.Ranges))
	}
}

func TestVarDeclWithUserDefinedType(t *testing.T) {
	input := `
		TYPE
			MyStruct : STRUCT
				Field1 : INT;
			END_STRUCT;
		END_TYPE

		VAR
			myVar : MyStruct;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestVarDeclWithUserDefinedArrayType", input)

	if len(program.Statements) != 2 {
		t.Fatalf("program.Statements does not contain 2 statements. got=%d", len(program.Statements))
	}

	varBlock, ok := program.Statements[1].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[1] is not ast.VarBlockDeclaration. got=%T", program.Statements[1])
	}

	if len(varBlock.Declarations) != 1 {
		t.Fatalf("varBlock.Declarations does not contain 1 statement. got=%d", len(varBlock.Declarations))
	}
	stmt := varBlock.Declarations[0]

	testVarDeclStatement(t, stmt, "myVar", "MyStruct")
}

func TestVarDeclWithUserDefinedArrayType(t *testing.T) {
	input := `
		TYPE
			MyIntArray : ARRAY [1..5] OF INT;
		END_TYPE

		VAR
			myArr : MyIntArray;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestVarDeclWithUserDefinedType", input)

	if len(program.Statements) != 2 {
		t.Fatalf("program.Statements does not contain 2 statements. got=%d", len(program.Statements))
	}

	varBlock, ok := program.Statements[1].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[1] is not ast.VarBlockDeclaration. got=%T", program.Statements[1])
	}

	if len(varBlock.Declarations) != 1 {
		t.Fatalf("varBlock.Declarations does not contain 1 statement. got=%d", len(varBlock.Declarations))
	}
	stmt := varBlock.Declarations[0]

	testVarDeclStatement(t, stmt, "myArr", "MyIntArray")
}

func TestConfigVarDeclarations(t *testing.T) {
	input := `
		VAR_CONFIG
			Config1 : INT;
			Config2 : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestIfStatementWithEmptyBlocks", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConfigVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Vars) != 2 {
		t.Fatalf("Expected 2 config variables. got=%d", len(stmt.Vars))
	}

	if !testVarDeclStatement(t, stmt.Vars[0], "Config1", "INT") {
		return
	}

	if !testVarDeclStatement(t, stmt.Vars[1], "Config2", "BOOL") {
		return
	}
}

func TestTempVarDeclarations(t *testing.T) {
	input := `
		VAR_TEMP
			Temp1 : INT;
			Temp2 : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestTempVarDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.TempVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TempVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Vars) != 2 {
		t.Fatalf("Expected 2 temp variables. got=%d", len(stmt.Vars))
	}

	if !testVarDeclStatement(t, stmt.Vars[0], "Temp1", "INT") {
		return
	}

	if !testVarDeclStatement(t, stmt.Vars[1], "Temp2", "BOOL") {
		return
	}
}

func TestAccessVarDeclarations(t *testing.T) {
	input := `
		VAR_ACCESS
			Path1 : MyFB READ_ONLY;
			Path2 : OtherFB READ_WRITE;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestAccessVarDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.AccessVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.AccessVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Vars) != 2 {
		t.Fatalf("Expected 2 access variables. got=%d", len(stmt.Vars))
	}

	if !testVarDeclStatement(t, stmt.Vars[0], "Path1", "MyFB") {
		return
	}

	if !testVarDeclStatement(t, stmt.Vars[1], "Path2", "OtherFB") {
		return
	}
}

func TestExternalVarDeclarations(t *testing.T) {
	input := `
		VAR_EXTERNAL
			External1 : INT;           // Standard external variable (read/write)
		END_VAR

		VAR_EXTERNAL CONSTANT
			External2 : BOOL;          // External constant (read-only)
		END_VAR
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestExternalVarDeclarations", input)

	if len(program.Statements) != 2 {
		t.Fatalf("program.Statements does not contain 2 statements. got=%d", len(program.Statements))
	}

	// Test first block: VAR_EXTERNAL
	stmt1, ok := program.Statements[0].(*ast.ExternalVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExternalVarDeclaration. got=%T", program.Statements[0])
	}
	if len(stmt1.Vars) != 1 {
		t.Fatalf("Expected 1 external variable in first block. got=%d", len(stmt1.Vars))
	}
	if !testVarDeclStatement(t, stmt1.Vars[0], "External1", "INT") {
		return
	}
	if stmt1.Vars[0].IsConstant {
		t.Errorf("External1 should not be constant")
	}

	// Test second block: VAR_EXTERNAL CONSTANT
	stmt2, ok := program.Statements[1].(*ast.ExternalVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[1] is not ast.ExternalVarDeclaration. got=%T", program.Statements[1])
	}
	if len(stmt2.Vars) != 1 {
		t.Fatalf("Expected 1 external variable in second block. got=%d", len(stmt2.Vars))
	}
	if !testVarDeclStatement(t, stmt2.Vars[0], "External2", "BOOL") {
		return
	}
	if !stmt2.Vars[0].IsConstant {
		t.Errorf("External2 should be constant")
	}
}

func TestMixedExternalVarDeclarations(t *testing.T) {
	input := `
		VAR_EXTERNAL CONSTANT
			External1 : INT := 1; 
			External2 : BOOL := TRUE;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestAccessVarDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExternalVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExternalVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Vars) != 2 {
		t.Fatalf("Expected 2 external variables. got=%d", len(stmt.Vars))
	}

	if !testVarDeclStatement(t, stmt.Vars[0], "External1", "INT") {
		return
	}
	if !stmt.Vars[0].IsConstant {
		t.Errorf("External1 should be constant")
	}

	if !testVarDeclStatement(t, stmt.Vars[1], "External2", "BOOL") {
		return
	}
	if !stmt.Vars[1].IsConstant {
		t.Errorf("External2 should be constant")
	}
}

func TestGlobalVarDeclarations(t *testing.T) {
	input := `
		VAR_GLOBAL RETAIN
			Global1 : INT;
			Global2 : BOOL := TRUE;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestConfigVarDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.GlobalVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.GlobalVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Vars) != 2 {
		t.Fatalf("Expected 2 global variables. got=%d", len(stmt.Vars))
	}

	if !testVarDeclStatement(t, stmt.Vars[0], "Global1", "INT") {
		return
	}

	if !testVarDeclStatement(t, stmt.Vars[1], "Global2", "BOOL") {
		return
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input         string
		expectedValue interface{}
	}{
		{"RETURN 5;", 5},
		{"RETURN TRUE;", true},
		{"RETURN foobar;", "foobar"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestReturnStatements", tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain 1 statements. got=%d",
				len(program.Statements))
		}

		stmt := program.Statements[0]
		returnStmt, ok := stmt.(*ast.ReturnStatement)
		if !ok {
			t.Fatalf("stmt not *ast.ReturnStatement. got=%T", stmt)
		}
		if returnStmt.TokenLiteral() != "RETURN" {
			t.Fatalf("returnStmt.TokenLiteral not 'RETURN', got %q",
				returnStmt.TokenLiteral())
		}
		if testLiteralExpression(t, returnStmt.ReturnValue, tt.expectedValue) {
			return
		}
	}
}

func TestIdentifierExpression(t *testing.T) {
	input := "foobar;"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIntegerLiteralExpression", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	ident, ok := stmt.Expression.(*ast.Identifier)
	if !ok {
		t.Fatalf("exp not *ast.Identifier. got=%T", stmt.Expression)
	}
	if ident.Value != "foobar" {
		t.Errorf("ident.Value not %s. got=%s", "foobar", ident.Value)
	}
	if ident.TokenLiteral() != "foobar" {
		t.Errorf("ident.TokenLiteral not %s. got=%s", "foobar",
			ident.TokenLiteral())
	}
}

func TestIntegerLiteralExpression(t *testing.T) {
	input := "5;"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIdentifierExpression", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	literal, ok := stmt.Expression.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("exp not *ast.IntegerLiteral. got=%T", stmt.Expression)
	}
	if literal.Value != 5 {
		t.Errorf("literal.Value not %d. got=%d", 5, literal.Value)
	}
	if literal.TokenLiteral() != "5" {
		t.Errorf("literal.TokenLiteral not %s. got=%s", "5",
			literal.TokenLiteral())
	}
}

func TestRealLiteralExpression(t *testing.T) {
	tests := []struct {
		input             string
		expectedValue     float64
		expectedPrecision int
	}{
		{"1.23E4;", 12300.0, 32},
		{"1.23e-4;", 0.000123, 32},
		{"REAL#3.14;", 3.14, 32},
		{"LREAL#2.718;", 2.718, 64},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestRealLiteralExpression", tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("program has not enough statements. got=%d", len(program.Statements))
		}
		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
		}

		literal, ok := stmt.Expression.(*ast.RealLiteral)
		if !ok {
			t.Fatalf("exp not *ast.RealLiteral. got=%T", stmt.Expression)
		}
		// Use a small tolerance (epsilon) for float comparison to avoid precision issues.
		const epsilon = 1e-6
		if diff := literal.Value - tt.expectedValue; diff < -epsilon || diff > epsilon {
			t.Errorf("literal.Value not %g. got=%g", tt.expectedValue, literal.Value)
		}
		if literal.Precision != tt.expectedPrecision {
			t.Errorf("literal.Precision not %d. got=%d", tt.expectedPrecision, literal.Precision)
		}
	}
}

func TestParsingPrefixExpressions(t *testing.T) {
	prefixTests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"NOT 5;", "NOT", 5},
		{"-15;", "-", 15},
		{"-foobar;", "-", "foobar"},
		{"NOT TRUE;", "NOT", true},
		{"NOT FALSE;", "NOT", false},
	}

	for _, tt := range prefixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestParsingInfixExpressions", tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
				1, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
				program.Statements[0])
		}

		exp, ok := stmt.Expression.(*ast.PrefixExpression)
		if !ok {
			t.Fatalf("stmt is not ast.PrefixExpression. got=%T", stmt.Expression)
		}
		if exp.Operator != tt.operator {
			t.Fatalf("exp.Operator is not '%s'. got=%s",
				tt.operator, exp.Operator)
		}
		if !testLiteralExpression(t, exp.Right, tt.value) {
			return
		}
	}
}

func TestParsingInfixExpressions(t *testing.T) {
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"5 + 5;", 5, "+", 5},
		{"5 - 5;", 5, "-", 5},
		{"5 * 5;", 5, "*", 5},
		{"5 / 5;", 5, "/", 5},
		{"5 > 5;", 5, ">", 5},
		{"5 < 5;", 5, "<", 5},
		{"5 = 5;", 5, "=", 5},
		{"5 <> 5;", 5, "<>", 5},
		{"foobar + barfoo;", "foobar", "+", "barfoo"},
		{"foobar - barfoo;", "foobar", "-", "barfoo"},
		{"foobar * barfoo;", "foobar", "*", "barfoo"},
		{"foobar / barfoo;", "foobar", "/", "barfoo"},
		{"foobar > barfoo;", "foobar", ">", "barfoo"},
		{"foobar < barfoo;", "foobar", "<", "barfoo"},
		{"foobar = barfoo;", "foobar", "=", "barfoo"},
		{"foobar <> barfoo;", "foobar", "<>", "barfoo"},
		{"TRUE = TRUE;", true, "=", true},
		{"TRUE <> FALSE;", true, "<>", false},
		{"FALSE = FALSE;", false, "=", false},
	}

	for i, tt := range infixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestParsingPrefixExpressions", tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("tests[%d] - program.Statements does not contain %d statements. got=%d\n",
				i, 1, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("tests[%d] - program.Statements[0] is not ast.ExpressionStatement. got=%T",
				i, program.Statements[0])
		}

		if !testInfixExpression(t, i, stmt.Expression, tt.leftValue,
			tt.operator, tt.rightValue) {
			return
		}
	}
}

func TestOperatorPrecedenceParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"-a * b;",
			"((-a) * b);",
		},
		{
			"a + b + c;",
			"((a + b) + c);",
		},
		{
			"a + b - c;",
			"((a + b) - c);",
		},
		{
			"a * b * c;",
			"((a * b) * c);",
		},
		{
			"a * b / c;",
			"((a * b) / c);",
		},
		{
			"a + b / c;",
			"(a + (b / c));",
		},
		{
			"a + b * c + d / e - f;",
			"(((a + (b * c)) + (d / e)) - f);",
		},
		{
			"3 + 4; -5 * 5;",
			"(3 + 4);((-5) * 5);",
		},
		{
			"5 > 4 = 3 < 4;",
			"((5 > 4) = (3 < 4));",
		},
		{
			"5 < 4 <> 3 > 4;",
			"((5 < 4) <> (3 > 4));",
		},
		{
			"3 + 4 * 5 = 3 * 1 + 4 * 5;",
			"((3 + (4 * 5)) = ((3 * 1) + (4 * 5)));",
		},
		{
			"TRUE;",
			"TRUE;",
		},
		{
			"FALSE;",
			"FALSE;",
		},
		{
			"3 > 5 = FALSE;",
			"((3 > 5) = FALSE);",
		},
		{
			"3 < 5 = TRUE;",
			"((3 < 5) = TRUE);",
		},
		{
			"1 + (2 + 3) + 4;",
			"((1 + (2 + 3)) + 4);",
		},
		{
			"(5 + 5) * 2;",
			"((5 + 5) * 2);",
		},
		{
			"2 / (5 + 5);",
			"(2 / (5 + 5));",
		},
		{
			"(5 + 5) * 2 * (5 + 5);",
			"(((5 + 5) * 2) * (5 + 5));",
		},
		{
			"-(5 + 5);",
			"(-(5 + 5));",
		},
		{
			"a + add(b * c) + d;",
			"((a + add((b * c))) + d);",
		},
		{
			"add(a, b, 1, 2 * 3, 4 + 5, add(6, 7 * 8));",
			"add(a, b, 1, (2 * 3), (4 + 5), add(6, (7 * 8)));",
		},
		{
			"add(a + b + c * d / f + g);",
			"add((((a + b) + ((c * d) / f)) + g));",
		},
		{
			"a * [1, 2, 3, 4][b * c] * d;",
			"((a * ([1, 2, 3, 4][(b * c)])) * d);",
		},
		{
			"add(a * b[2], b[1], 2 * [1, 2][1]);",
			"add((a * (b[2])), (b[1]), (2 * ([1, 2][1])));",
		},
		{
			"a + b AND c * d;",
			"((a + b) AND (c * d));",
		},
		{
			"a AND b OR c;",
			"((a AND b) OR c);",
		},
		{
			"a OR b AND c;",
			"(a OR (b AND c));",
		},
		{
			"a & b OR c;",
			"((a & b) OR c);",
		},
		{
			"a XOR b & c;",
			"(a XOR (b & c));",
		},
		{
			"a ** b + c;",
			"((a ** b) + c);",
		},
		{
			"a + b ** c;",
			"(a + (b ** c));",
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestOperatorPrecedenceParsing", tt.input)

		actual := program.String()
		if actual != tt.expected {
			t.Errorf("expected=%q, got=%q", tt.expected, actual)
		}
	}
}

func TestBooleanExpression(t *testing.T) {
	tests := []struct {
		input           string
		expectedBoolean bool
	}{
		{"TRUE;", true},
		{"FALSE;", false},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestBooleanExpression", tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("program has not enough statements. got=%d",
				len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
				program.Statements[0])
		}

		boolean, ok := stmt.Expression.(*ast.Boolean)
		if !ok {
			t.Fatalf("exp not *ast.Boolean. got=%T", stmt.Expression)
		}
		if boolean.Value != tt.expectedBoolean {
			t.Errorf("boolean.Value not %t. got=%t", tt.expectedBoolean,
				boolean.Value)
		}
	}
}

func TestIfStatement(t *testing.T) {
	input := `
		IF x < y THEN 
			x; 
		END_IF
		`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestMacroLiteralParsing", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T",
			program.Statements[0])
	}

	if stmt.TokenLiteral() != "IF" {
		t.Fatalf("stmt.TokenLiteral not 'IF', got %q", stmt.TokenLiteral())
	}

	if !testInfixExpression(t, 0, stmt.Condition, "x", "<", "y") {
		return
	}

	if len(stmt.Consequence.Statements) != 1 {
		t.Errorf("consequence is not 1 statements. got=%d\n",
			len(stmt.Consequence.Statements))
	}

	consequence, ok := stmt.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Statements[0] is not ast.ExpressionStatement. got=%T",
			stmt.Consequence.Statements[0])
	}

	if !testIdentifier(t, consequence.Expression, "x") {
		return
	}

	if stmt.Alternative != nil {
		t.Errorf("stmt.Alternative.Statements was not nil. got=%+v", stmt.Alternative)
	}
}

func TestIfElseStatement(t *testing.T) {
	input := `
		IF x < y THEN 
			x; 
		ELSE 
			y; 
		END_IF
		`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestCallExpressionParsing", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	if stmt.TokenLiteral() != "IF" {
		t.Fatalf("stmt.TokenLiteral not 'IF', got %q", stmt.TokenLiteral())
	}

	if !testInfixExpression(t, 0, stmt.Condition, "x", "<", "y") {
		return
	}

	if len(stmt.Consequence.Statements) != 1 {
		t.Errorf("consequence is not 1 statements. got=%d\n",
			len(stmt.Consequence.Statements))
	}

	consequence, ok := stmt.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Statements[0] is not ast.ExpressionStatement. got=%T",
			stmt.Consequence.Statements[0])
	}

	if !testIdentifier(t, consequence.Expression, "x") {
		return
	}

	alt, ok := stmt.Alternative.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("stmt.Alternative is not *ast.BlockStatement. got=%T", stmt.Alternative)
	}

	if len(alt.Statements) != 1 {
		t.Errorf("stmt.Alternative.Statements does not contain 1 statements. got=%d\n",
			len(alt.Statements))
	}

	alternative, ok := alt.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Statements[0] is not ast.ExpressionStatement. got=%T",
			alt.Statements[0])
	}

	if !testIdentifier(t, alternative.Expression, "y") {
		return
	}
}

func TestIfElsifElseStatement(t *testing.T) {
	input := `
		IF x < y THEN
			x;
		ELSIF x > y THEN
			y;
		ELSE
			z;
		END_IF
		`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIfElsifElseStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statements. got=%d\n", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	// Test IF part
	if !testInfixExpression(t, 0, stmt.Condition, "x", "<", "y") {
		return
	}
	consequence, ok := stmt.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Consequence statement is not ast.ExpressionStatement. got=%T", stmt.Consequence.Statements[0])
	}
	if !testIdentifier(t, consequence.Expression, "x") {
		return
	}

	// Test ELSIF part (which is a nested IfStatement)
	elsifStmt, ok := stmt.Alternative.(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt.Alternative is not ast.IfStatement. got=%T", stmt.Alternative)
	}

	if !testInfixExpression(t, 0, elsifStmt.Condition, "x", ">", "y") {
		return
	}
	elsifConsequence, ok := elsifStmt.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Elsif consequence statement is not ast.ExpressionStatement. got=%T", elsifStmt.Consequence.Statements[0])
	}
	if !testIdentifier(t, elsifConsequence.Expression, "y") {
		return
	}

	// Test ELSE part
	elseStmt, ok := elsifStmt.Alternative.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("elsifStmt.Alternative is not ast.BlockStatement. got=%T", elsifStmt.Alternative)
	}
	testIdentifier(t, elseStmt.Statements[0].(*ast.ExpressionStatement).Expression, "z")
}

func TestIfStatementWithEmptyBlocks(t *testing.T) {
	input := `
		IF x < y THEN
			(* This block is empty *)
		ELSIF x > y THEN
			do_something;
		ELSE
			(* This block is also empty *)
		END_IF
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestStructTypeDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	ifStmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	// Check that the main IF consequence is empty
	if len(ifStmt.Consequence.Statements) != 0 {
		t.Errorf("Expected empty consequence for IF block, but found %d statements.", len(ifStmt.Consequence.Statements))
	}

	// Check the ELSIF part
	elsifStmt, ok := ifStmt.Alternative.(*ast.IfStatement)
	if !ok {
		t.Fatalf("Alternative is not an ELSIF (IfStatement). got=%T", ifStmt.Alternative)
	}
	if len(elsifStmt.Consequence.Statements) != 1 {
		t.Errorf("Expected 1 statement in ELSIF block, but found %d.", len(elsifStmt.Consequence.Statements))
	}

	// Check that the ELSE part is empty
}

func TestFunctionDeclaration(t *testing.T) {
	input := `
		FUNCTION MyFunction : INT
			VAR_INPUT
				A : INT;
				B : INT;
			END_VAR
			VAR
				C : INT;
			END_VAR

			C := A + B;
			MyFunction := C * 2;
		END_FUNCTION
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionDeclaration. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyFunction" {
		t.Fatalf("Function name is not 'MyFunction'. got=%s", stmt.Name.Value)
	}

	if stmt.ReturnType.String() != "INT" {
		t.Fatalf("Function return type is not 'INT'. got=%s", stmt.ReturnType.String())
	}

	if len(stmt.VarInputs) != 2 {
		t.Fatalf("Expected 2 VAR_INPUTs. got=%d", len(stmt.VarInputs))
	}
	testVarDeclStatement(t, stmt.VarInputs[0], "A", "INT")
	testVarDeclStatement(t, stmt.VarInputs[1], "B", "INT")

	if len(stmt.Vars) != 1 {
		t.Fatalf("Expected 1 VAR. got=%d", len(stmt.Vars))
	}
	testVarDeclStatement(t, stmt.Vars[0], "C", "INT")

	if len(stmt.Body.Statements) != 2 {
		t.Fatalf("Function body does not have 2 statements. got=%d", len(stmt.Body.Statements))
	}
	// Test first statement in body: C := A + B;
	stmt1, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement 1 is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}
	testIdentifier(t, stmt1.Left, "C")

	// Test second statement in body: MyFunction := C * 2;
	stmt2, ok := stmt.Body.Statements[1].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement 2 is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[1])
	}
	testIdentifier(t, stmt2.Left, "MyFunction")
}

func TestFunctionWithMultipleVarBlocks(t *testing.T) {
	input := `
		FUNCTION MyFunc : BOOL
			VAR_INPUT
				In1 : INT;
			END_VAR
			VAR_INPUT
				In2 : BOOL;
			END_VAR
			VAR_OUTPUT
				Out1 : REAL;
			END_VAR

			MyFunc := TRUE;
		END_FUNCTION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestTypeDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.VarInputs) != 2 {
		t.Fatalf("Expected 2 VAR_INPUT declarations. got=%d", len(stmt.VarInputs))
	}
	testVarDeclStatement(t, stmt.VarInputs[0], "In1", "INT")
	testVarDeclStatement(t, stmt.VarInputs[1], "In2", "BOOL")

	if len(stmt.VarOutputs) != 1 {
		t.Fatalf("Expected 1 VAR_OUTPUT declaration. got=%d", len(stmt.VarOutputs))
	}
	testVarDeclStatement(t, stmt.VarOutputs[0], "Out1", "REAL")

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("Function body should have 1 statement. got=%d", len(stmt.Body.Statements))
	}
}

func TestCallExpressionParsing(t *testing.T) {
	input := "add(1, 2 * 3, 4 + 5);"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIfStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	exp, ok := stmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.CallExpression. got=%T",
			stmt.Expression)
	}

	if !testIdentifier(t, exp.Function, "add") {
		return
	}

	if len(exp.Arguments) != 3 {
		t.Fatalf("wrong length of arguments. got=%d", len(exp.Arguments))
	}

	testLiteralExpression(t, exp.Arguments[0], 1)
	testInfixExpression(t, 0, exp.Arguments[1], 2, "*", 3)
	testInfixExpression(t, 0, exp.Arguments[2], 4, "+", 5)
}

func TestCallExpressionParameterParsing(t *testing.T) {
	tests := []struct {
		input         string
		expectedIdent string
		expectedArgs  []string
	}{
		{
			input:         "add();",
			expectedIdent: "add",
			expectedArgs:  []string{},
		},
		{
			input:         "add(1);",
			expectedIdent: "add",
			expectedArgs:  []string{"1"},
		},
		{
			input:         "add(1, 2 * 3, 4 + 5);",
			expectedIdent: "add",
			expectedArgs:  []string{"1", "(2 * 3)", "(4 + 5)"},
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestCallExpressionParameterParsing", tt.input)

		stmt := program.Statements[0].(*ast.ExpressionStatement)
		exp, ok := stmt.Expression.(*ast.CallExpression)
		if !ok {
			t.Fatalf("stmt.Expression is not ast.CallExpression. got=%T",
				stmt.Expression)
		}

		if !testIdentifier(t, exp.Function, tt.expectedIdent) {
			return
		}

		if len(exp.Arguments) != len(tt.expectedArgs) {
			t.Fatalf("wrong number of arguments. want=%d, got=%d",
				len(tt.expectedArgs), len(exp.Arguments))
		}

		for i, arg := range tt.expectedArgs {
			if exp.Arguments[i].String() != arg {
				t.Errorf("argument %d wrong. want=%q, got=%q", i,
					arg, exp.Arguments[i].String())
			}
		}
	}
}

func TestStringLiteralExpression(t *testing.T) {
	input := `"hello world";`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStringLiteralExpression", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	literal, ok := stmt.Expression.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("exp not *ast.StringLiteral. got=%T", stmt.Expression)
	}

	if literal.Value != "hello world" {
		t.Errorf("literal.Value not %q. got=%q", "hello world", literal.Value)
	}
}

func TestTimeLiteralExpression(t *testing.T) {
	tests := []struct {
		input        string
		expectedType interface{}
		expectedVal  string
	}{
		{"T#5m_10s;", (*ast.TimeLiteral)(nil), "T#5m_10s"},
		{"TIME#1h_30m;", (*ast.TimeLiteral)(nil), "TIME#1h_30m"},
		{"D#2026-05-09;", (*ast.DateLiteral)(nil), "D#2026-05-09"},
		{"DATE#1999-12-31;", (*ast.DateLiteral)(nil), "DATE#1999-12-31"},
		{"TOD#22:45:00;", (*ast.TimeOfDayLiteral)(nil), "TOD#22:45:00"},
		{"TIME_OF_DAY#08:30:15.5;", (*ast.TimeOfDayLiteral)(nil), "TIME_OF_DAY#08:30:15.5"},
		{"DT#2026-05-09-22:45:00;", (*ast.DateAndTimeLiteral)(nil), "DT#2026-05-09-22:45:00"},
		{"DATE_AND_TIME#1999-12-31-23:59:59;", (*ast.DateAndTimeLiteral)(nil), "DATE_AND_TIME#1999-12-31-23:59:59"},
	}

	for i, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, fmt.Sprintf("TestTimeLiteralExpression[%d]", i), tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("Test[%d] - program has not enough statements for input '%s'. got=%d", i, tt.input, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("Test[%d] - program.Statements[0] is not ast.ExpressionStatement for input '%s'. got=%T", i, tt.input, program.Statements[0])
		}

		if !isExpectedType(t, tt.expectedType, stmt.Expression) {
			t.Fatalf("Test[%d] - Expression is not of expected type for input '%s'. got=%T", i, tt.input, stmt.Expression)
		}

		// Check the raw literal value stored in the AST node
		val := stmt.Expression.(ast.Node).TokenLiteral()
		if val != tt.expectedVal {
			t.Errorf("Test[%d] - literal.Value not %q for input '%s'. got=%q", i, tt.expectedVal, tt.input, val)
		}
	}
}

func TestParsingEmptyArrayLiterals(t *testing.T) {
	input := "[];"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingEmptyArrayLiterals", input)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	array, ok := stmt.Expression.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.ArrayLiteral. got=%T", stmt.Expression)
	}

	if len(array.Elements) != 0 {
		t.Errorf("len(array.Elements) not 0. got=%d", len(array.Elements))
	}
}

func TestParsingArrayLiterals(t *testing.T) {
	input := "[1, 2 * 2, 3 + 3];"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingArrayLiterals", input)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	array, ok := stmt.Expression.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.ArrayLiteral. got=%T", stmt.Expression)
	}

	if len(array.Elements) != 3 {
		t.Fatalf("len(array.Elements) not 3. got=%d", len(array.Elements))
	}

	testIntegerLiteral(t, array.Elements[0], 1)
	testInfixExpression(t, 0, array.Elements[1], 2, "*", 2)
	testInfixExpression(t, 1, array.Elements[2], 3, "+", 3)
}

func TestParsingIndexExpressions(t *testing.T) {
	input := "myArray[1 + 1];"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingIndexExpressions", input)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	indexExp, ok := stmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("exp not *ast.IndexExpression. got=%T", stmt.Expression)
	}

	if !testIdentifier(t, indexExp.Left, "myArray") {
		return
	}

	if !testInfixExpression(t, 0, indexExp.Index, 1, "+", 1) {
		return
	}
}

func TestParsingEmptyHashLiteral(t *testing.T) {
	input := "{};"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingEmptyHashLiteral", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	hash, ok := stmt.Expression.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("exp is not ast.HashLiteral. got=%T", stmt.Expression)
	}

	if len(hash.Pairs) != 0 {
		t.Errorf("hash.Pairs has wrong length. got=%d", len(hash.Pairs))
	}
}

func TestParsingHashLiteralsStringKeys(t *testing.T) {
	input := `{"one": 1, "two": 2, "three": 3};`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingHashLiteralsStringKeys", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	hash, ok := stmt.Expression.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("exp is not ast.HashLiteral. got=%T", stmt.Expression)
	}

	expected := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	if len(hash.Pairs) != len(expected) {
		t.Errorf("hash.Pairs has wrong length. got=%d", len(hash.Pairs))
	}

	for key, value := range hash.Pairs {
		literal, ok := key.(*ast.StringLiteral)
		if !ok {
			t.Errorf("key is not ast.StringLiteral. got=%T", key)
			continue
		}

		expectedValue := expected[literal.String()]
		testIntegerLiteral(t, value, expectedValue)
	}
}

func TestParsingHashLiteralsBooleanKeys(t *testing.T) {
	input := `{true: 1, false: 2};`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingHashLiteralsBooleanKeys", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	hash, ok := stmt.Expression.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("exp is not ast.HashLiteral. got=%T", stmt.Expression)
	}

	expected := map[string]int64{
		"true":  1,
		"false": 2,
	}

	if len(hash.Pairs) != len(expected) {
		t.Errorf("hash.Pairs has wrong length. got=%d", len(hash.Pairs))
	}

	for key, value := range hash.Pairs {
		boolean, ok := key.(*ast.Boolean)
		if !ok {
			t.Errorf("key is not ast.BooleanLiteral. got=%T", key)
			continue
		}

		expectedValue := expected[boolean.String()]
		testIntegerLiteral(t, value, expectedValue)
	}
}

func TestParsingHashLiteralsIntegerKeys(t *testing.T) {
	input := `{1: 1, 2: 2, 3: 3};`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingHashLiteralsIntegerKeys", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	hash, ok := stmt.Expression.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("exp is not ast.HashLiteral. got=%T", stmt.Expression)
	}

	expected := map[string]int64{
		"1": 1,
		"2": 2,
		"3": 3,
	}

	if len(hash.Pairs) != len(expected) {
		t.Errorf("hash.Pairs has wrong length. got=%d", len(hash.Pairs))
	}

	for key, value := range hash.Pairs {
		integer, ok := key.(*ast.IntegerLiteral)
		if !ok {
			t.Errorf("key is not ast.IntegerLiteral. got=%T", key)
			continue
		}

		expectedValue := expected[integer.String()]

		testIntegerLiteral(t, value, expectedValue)
	}
}

func TestParsingHashLiteralsWithExpressions(t *testing.T) {
	input := `{"one": 0 + 1, "two": 10 - 8, "three": 15 / 5};`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestParsingHashLiteralsWithExpressions", input)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	hash, ok := stmt.Expression.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("exp is not ast.HashLiteral. got=%T", stmt.Expression)
	}

	if len(hash.Pairs) != 3 {
		t.Errorf("hash.Pairs has wrong length. got=%d", len(hash.Pairs))
	}

	tests := map[string]func(ast.Expression){
		"one": func(e ast.Expression) {
			testInfixExpression(t, 1, e, 0, "+", 1)
		},
		"two": func(e ast.Expression) {
			testInfixExpression(t, 2, e, 10, "-", 8)
		},
		"three": func(e ast.Expression) {
			testInfixExpression(t, 3, e, 15, "/", 5)
		},
	}

	for key, value := range hash.Pairs {
		literal, ok := key.(*ast.StringLiteral)
		if !ok {
			t.Errorf("key is not ast.StringLiteral. got=%T", key)
			continue
		}

		testFunc, ok := tests[literal.String()]
		if !ok {
			t.Errorf("No test function for key %q found", literal.String())
			continue
		}

		testFunc(value)
	}
}

func TestMacroLiteralParsing(t *testing.T) {
	input := `macro(x, y) { x + y; };`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIfElseStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	macro, ok := stmt.Expression.(*ast.MacroLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.MacroLiteral. got=%T",
			stmt.Expression)
	}

	if len(macro.Parameters) != 2 {
		t.Fatalf("macro literal parameters wrong. want 2, got=%d\n",
			len(macro.Parameters))
	}

	testLiteralExpression(t, macro.Parameters[0], "x")
	testLiteralExpression(t, macro.Parameters[1], "y")

	if len(macro.Body.Statements) != 1 {
		t.Fatalf("macro.Body.Statements has not 1 statements. got=%d\n",
			len(macro.Body.Statements))
	}

	bodyStmt, ok := macro.Body.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("macro body stmt is not ast.ExpressionStatement. got=%T",
			macro.Body.Statements[0])
	}

	testInfixExpression(t, 0, bodyStmt.Expression, "x", "+", "y")
}

func TestNestedIfStatement(t *testing.T) {
	input := `
		IF x < y THEN
			IF a > b THEN
				a;
			ELSIF a = b THEN
				c;
			ELSE
				b;
			END_IF
		ELSE
			y;
		END_IF`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestNestedIfStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	// Test outer IF
	if !testInfixExpression(t, 0, stmt.Condition, "x", "<", "y") {
		return
	}

	if len(stmt.Consequence.Statements) != 1 {
		t.Fatalf("Outer consequence is not 1 statement. got=%d", len(stmt.Consequence.Statements))
	}

	// Test nested IF in the consequence
	nestedIf, ok := stmt.Consequence.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("Statement in outer consequence is not ast.IfStatement. got=%T", stmt.Consequence.Statements[0])
	}

	if !testInfixExpression(t, 0, nestedIf.Condition, "a", ">", "b") {
		return
	}

	// Test nested ELSIF
	nestedElsif, ok := nestedIf.Alternative.(*ast.IfStatement)
	if !ok {
		t.Fatalf("Nested alternative is not ast.IfStatement for ELSIF. got=%T", nestedIf.Alternative)
	}

	if !testInfixExpression(t, 0, nestedElsif.Condition, "a", "=", "b") {
		return
	}

	// Test nested ELSE
	nestedElse, ok := nestedElsif.Alternative.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Nested ELSIF alternative is not ast.BlockStatement. got=%T", nestedElsif.Alternative)
	}
	if !testIdentifier(t, nestedElse.Statements[0].(*ast.ExpressionStatement).Expression, "b") {
		return
	}

	// Test outer ELSE
	alt, ok := stmt.Alternative.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Outer alternative is not ast.BlockStatement. got=%T", stmt.Alternative)
	}
	if !testIdentifier(t, alt.Statements[0].(*ast.ExpressionStatement).Expression, "y") {
		return
	}
}

func TestForLoopStatement(t *testing.T) {
	input := `
		FOR i := 1 TO 10 BY 2 DO 
			x := x + 1; 
		END_FOR
		`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestForLoopStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ForLoopStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ForLoopStatement. got=%T", program.Statements[0])
	}

	// Test the control variable assignment: i := 1
	if stmt.ControlVar == nil {
		t.Fatalf("ForLoopStatement.ControlVar is nil")
	}
	if !testIdentifier(t, stmt.ControlVar.Left, "i") {
		return
	}
	if !testIntegerLiteral(t, stmt.ControlVar.Value, 1) {
		return
	}

	if !testIntegerLiteral(t, stmt.EndValue, 10) {
		return
	}

	if !testIntegerLiteral(t, stmt.StepValue, 2) {
		return
	}

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("for loop body does not contain 1 statement. got=%d", len(stmt.Body.Statements))
	}

	bodyStmt, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("for loop body statement is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}
	if !testIdentifier(t, bodyStmt.Left, "x") {
		return
	}
	testInfixExpression(t, 0, bodyStmt.Value, "x", "+", 1)
}

func TestWhileStatement(t *testing.T) {
	input := `
		WHILE x < 10 DO 
			x := x + 1; 
		END_WHILE
		`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestInitialStepStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.WhileStatement. got=%T", program.Statements[0])
	}

	if !testInfixExpression(t, 0, stmt.Condition, "x", "<", 10) {
		return
	}

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("while loop body does not contain 1 statement. got=%d", len(stmt.Body.Statements))
	}

	bodyStmt, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("while loop body statement is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}

	if !testIdentifier(t, bodyStmt.Left, "x") {
		return
	}
	testInfixExpression(t, 0, bodyStmt.Value, "x", "+", 1)
}

func TestRepeatUntilStatement(t *testing.T) {
	input := `
		REPEAT x := x + 1; 
		UNTIL x > 10 
		END_REPEAT`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestRepeatUntilStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.RepeatStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.RepeatStatement. got=%T", program.Statements[0])
	}

	if !testInfixExpression(t, 0, stmt.Condition, "x", ">", 10) {
		return
	}

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("repeat loop body does not contain 1 statement. got=%d", len(stmt.Body.Statements))
	}

	bodyStmt, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("repeat loop body statement is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}

	if !testIdentifier(t, bodyStmt.Left, "x") {
		return
	}
	testInfixExpression(t, 0, bodyStmt.Value, "x", "+", 1)
}

func TestCaseStatement(t *testing.T) {
	input := `
		CASE myVar OF
			1: x := 1;
			2, 3: x := 2;
		ELSE
			x := 3;
		END_CASE
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestCaseStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.CaseStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.CaseStatement. got=%T", program.Statements[0])
	}

	if !testIdentifier(t, stmt.Expression, "myVar") {
		return
	}

	if len(stmt.Cases) != 2 {
		t.Fatalf("case statement does not have 2 cases. got=%d", len(stmt.Cases))
	}

	// Test first case: 1: x := 1;
	case1 := stmt.Cases[0]
	if len(case1.Values) != 1 || !testIntegerLiteral(t, case1.Values[0], 1) {
		t.Errorf("incorrect values for case 1. got=%v", case1.Values)
	}
	consequence1, ok := case1.Consequence.(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("consequence for case 1 is not AssignmentStatement. got=%T", case1.Consequence)
	}
	testIdentifier(t, consequence1.Left, "x")
	testIntegerLiteral(t, consequence1.Value, 1)

	// Test second case: 2, 3: x := 2;
	case2 := stmt.Cases[1]
	if len(case2.Values) != 2 || !testIntegerLiteral(t, case2.Values[0], 2) || !testIntegerLiteral(t, case2.Values[1], 3) {
		t.Errorf("incorrect values for case 2. got=%v", case2.Values)
	}
	consequence2, ok := case2.Consequence.(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("consequence for case 2 is not AssignmentStatement. got=%T", case2.Consequence)
	}
	testIdentifier(t, consequence2.Left, "x")
	testIntegerLiteral(t, consequence2.Value, 2)

	// Test ELSE part
	if stmt.Alternative == nil {
		t.Fatal("case statement alternative (ELSE) is nil")
	}
	if len(stmt.Alternative.Statements) != 1 {
		t.Fatalf("else block does not have 1 statement. got=%d", len(stmt.Alternative.Statements))
	}
	elseConsequence, ok := stmt.Alternative.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("else consequence is not AssignmentStatement. got=%T", stmt.Alternative.Statements[0])
	}
	testIdentifier(t, elseConsequence.Left, "x")
	testIntegerLiteral(t, elseConsequence.Value, 3)

}

func TestFunctionBlockDeclaration(t *testing.T) {
	input := `
		FUNCTION_BLOCK MyFB
			VAR_INPUT
				In1 : BOOL;
			END_VAR
			VAR_OUTPUT
				Out1 : INT;
			END_VAR
			VAR
				Internal1 : REAL;
			END_VAR

			Out1 := In1 + Internal1;
		END_FUNCTION_BLOCK
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestFunctionBlockDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyFB" {
		t.Fatalf("Function block name is not 'MyFB'. got=%s", stmt.Name.Value)
	}

	if len(stmt.VarInputs) != 1 {
		t.Fatalf("Expected 1 VAR_INPUT. got=%d", len(stmt.VarInputs))
	}
	testVarDeclStatement(t, stmt.VarInputs[0], "In1", "BOOL")

	if len(stmt.VarOutputs) != 1 {
		t.Fatalf("Expected 1 VAR_OUTPUT. got=%d", len(stmt.VarOutputs))
	}
	testVarDeclStatement(t, stmt.VarOutputs[0], "Out1", "INT")

	if len(stmt.Vars) != 1 {
		t.Fatalf("Expected 1 VAR. got=%d", len(stmt.Vars))
	}
	testVarDeclStatement(t, stmt.Vars[0], "Internal1", "REAL")

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("Function block body does not have 1 statement. got=%d", len(stmt.Body.Statements))
	}

	bodyStmt, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}
	testIdentifier(t, bodyStmt.Left, "Out1")
	testInfixExpression(t, 0, bodyStmt.Value, "In1", "+", "Internal1")
}

func TestProgramDeclaration(t *testing.T) {
	input := `
		PROGRAM MyProgram
			VAR_INPUT
				ProgIn : BOOL;
			END_VAR
			VAR_OUTPUT
				ProgOut : INT;
			END_VAR
			VAR_IN_OUT
				ProgInOut : REAL;
			END_VAR
			VAR
				LocalVar : INT;
			END_VAR

			LocalVar := 10;
		END_PROGRAM
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestProgramDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyProgram" {
		t.Fatalf("Program name is not 'MyProgram'. got=%s", stmt.Name.Value)
	}

	if len(stmt.VarInputs) != 1 {
		t.Fatalf("Expected 1 VAR_INPUT. got=%d", len(stmt.VarInputs))
	}
	testVarDeclStatement(t, stmt.VarInputs[0], "ProgIn", "BOOL")

	if len(stmt.VarOutputs) != 1 {
		t.Fatalf("Expected 1 VAR_OUTPUT. got=%d", len(stmt.VarOutputs))
	}
	testVarDeclStatement(t, stmt.VarOutputs[0], "ProgOut", "INT")

	if len(stmt.VarInOuts) != 1 {
		t.Fatalf("Expected 1 VAR_IN_OUT. got=%d", len(stmt.VarInOuts))
	}
	testVarDeclStatement(t, stmt.VarInOuts[0], "ProgInOut", "REAL")

	if len(stmt.Vars) != 1 {
		t.Fatalf("Expected 1 VAR. got=%d", len(stmt.Vars))
	}
	testVarDeclStatement(t, stmt.Vars[0], "LocalVar", "INT")

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("Program body does not have 1 statement. got=%d", len(stmt.Body.Statements))
	}

	bodyStmt, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}
	testIdentifier(t, bodyStmt.Left, "LocalVar")
	testIntegerLiteral(t, bodyStmt.Value, 10)
}

func TestActionStatement(t *testing.T) {
	input := `
		ACTION MyAction
			x := x + 1;
		END_ACTION
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestActionStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ActionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ActionStatement. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyAction" {
		t.Fatalf("Action name is not 'MyAction'. got=%s", stmt.Name.Value)
	}

	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("Action body does not have 1 statement. got=%d", len(stmt.Body.Statements))
	}

	bodyStmt, ok := stmt.Body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", stmt.Body.Statements[0])
	}

	testIdentifier(t, bodyStmt.Left, "x")
	testInfixExpression(t, 0, bodyStmt.Value, "x", "+", 1)
}

func TestExitStatement(t *testing.T) {
	input := `EXIT;`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestExitStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExitStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExitStatement. got=%T", program.Statements[0])
	}

	if stmt.TokenLiteral() != "EXIT" {
		t.Errorf("stmt.TokenLiteral not 'EXIT'. got=%q", stmt.TokenLiteral())
	}
}

func TestTransitionStatement(t *testing.T) {
	input := `
		TRANSITION FROM Step1, Step2 TO Step3 := Condition1 AND Condition2; END_TRANSITION
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestTransitionStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.TransitionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TransitionStatement. got=%T", program.Statements[0])
	}

	if len(stmt.From) != 2 {
		t.Fatalf("Expected 2 'FROM' identifiers. got=%d", len(stmt.From))
	}
	if stmt.From[0].Value != "Step1" || stmt.From[1].Value != "Step2" {
		t.Errorf("Incorrect 'FROM' identifiers. got=%s, %s", stmt.From[0].Value, stmt.From[1].Value)
	}

	if len(stmt.To) != 1 {
		t.Fatalf("Expected 1 'TO' identifier. got=%d", len(stmt.To))
	}
	if stmt.To[0].Value != "Step3" {
		t.Errorf("Incorrect 'TO' identifier. got=%s", stmt.To[0].Value)
	}

	if !testInfixExpression(t, 0, stmt.Condition, "Condition1", "AND", "Condition2") {
		return
	}
}

func TestStepStatement(t *testing.T) {
	input := `
		STEP MyStep:
			Action1(N);
			Action2(P);
		END_STEP
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestComplexTypeBlockDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.StepStatement. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyStep" {
		t.Errorf("Step name is not 'MyStep'. got=%s", stmt.Name.Value)
	}

	if stmt.IsInitial {
		t.Errorf("Step should not be initial")
	}

	if len(stmt.Actions) != 2 {
		t.Fatalf("Expected 2 action associations. got=%d", len(stmt.Actions))
	}

	if stmt.Actions[0].ActionName.Value != "Action1" || stmt.Actions[0].Qualifier.Value != "N" {
		t.Errorf("Incorrect first action association. got=%s(%s)", stmt.Actions[0].ActionName.Value, stmt.Actions[0].Qualifier.Value)
	}

	if stmt.Actions[1].ActionName.Value != "Action2" || stmt.Actions[1].Qualifier.Value != "P" {
		t.Errorf("Incorrect second action association. got=%s(%s)", stmt.Actions[1].ActionName.Value, stmt.Actions[1].Qualifier.Value)
	}
}

func TestInitialStepStatement(t *testing.T) {
	input := `
		INITIAL_STEP InitStep:
			InitAction(N);
		END_STEP
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestArrayTypeDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.StepStatement. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "InitStep" {
		t.Errorf("Step name is not 'InitStep'. got=%s", stmt.Name.Value)
	}

	if !stmt.IsInitial {
		t.Errorf("Step should be initial")
	}

	if len(stmt.Actions) != 1 {
		t.Fatalf("Expected 1 action association. got=%d", len(stmt.Actions))
	}
}

func TestConfigurationDeclaration(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			VAR_GLOBAL
				Global1 : BOOL;
			END_VAR

			RESOURCE Res1 ON PLC1
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
				PROGRAM Prog1 WITH Task1 : ProgType1;
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestTempVarDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyConfig" {
		t.Errorf("Configuration name is not 'MyConfig'. got=%s", stmt.Name.Value)
	}

	if len(stmt.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(stmt.Resources))
	}
}

func TestComments(t *testing.T) {
	input := `
		VAR // This is a variable block
			myVar : INT; (* This is a variable declaration *)
			myArray : ARRAY[1..10] OF REAL; // An array declaration
		END_VAR

		// This is a function call
		MyFunction (
			In1 := 10, (* Input parameter 1 *)
			In2 := 20, // Input parameter 2
			Out1 => Res1 // Output parameter
		);
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestComments", input)

	if len(program.Statements) != 2 {
		t.Fatalf("program.Statements does not contain 2 statements. got=%d", len(program.Statements))
	}

	// Check VAR block
	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
	}
	if len(varBlock.Declarations) != 2 {
		t.Fatalf("VarBlockDeclaration does not contain 2 declarations. got=%d", len(varBlock.Declarations))
	}
	// Check myVar declaration
	if !testVarDeclStatement(t, varBlock.Declarations[0], "myVar", "INT") {
		return
	}
	// Check myArray declaration
	arrayDecl := varBlock.Declarations[1]

	if !testVarDeclStatement(t, arrayDecl, "myArray", "ARRAY [1 .. 10] OF REAL") {
		return
	}
	_, isArrayDef := arrayDecl.DataType.(*ast.ArrayDefinition)
	if !isArrayDef {
		t.Fatalf("myArray's DataType is not *ast.ArrayDefinition. got=%T", arrayDecl.DataType)
	}

	// Check function call
	exprStmt, ok := program.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[1] is not ast.ExpressionStatement. got=%T", program.Statements[1])
	}
	call, ok := exprStmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("Expression is not ast.CallExpression. got=%T", exprStmt.Expression)
	}
	if len(call.Arguments) != 3 { // Updated to expect 3 arguments (2 input, 1 output)
		t.Fatalf("Expected 3 arguments in function call. got=%d", len(call.Arguments))
	}

	// Test first argument: In1 := 10
	arg1, ok := call.Arguments[0].(*ast.NamedArgument)
	if !ok {
		t.Fatalf("First argument is not *ast.NamedArgument. got=%T", call.Arguments[0])
	}
	if arg1.Name.Value != "In1" || !testIntegerLiteral(t, arg1.Value, 10) {
		t.Errorf("First argument (In1 := 10) is incorrect. Name: %s, Value: %v", arg1.Name.Value, arg1.Value)
	}

	// Test second argument: In2 := 20
	arg2, ok := call.Arguments[1].(*ast.NamedArgument)
	if !ok {
		t.Fatalf("Second argument is not *ast.NamedArgument. got=%T", call.Arguments[1])
	}
	if arg2.Name.Value != "In2" || !testIntegerLiteral(t, arg2.Value, 20) {
		t.Errorf("Second argument (In2 := 20) is incorrect. Name: %s, Value: %v", arg2.Name.Value, arg2.Value)
	}

	// Test third argument: Out1 => Res1
	arg3, ok := call.Arguments[2].(*ast.OutputArgument)
	if !ok || arg3.Source.Value != "Out1" || !testIdentifier(t, arg3.Target, "Res1") {
		t.Errorf("Third argument (Out1 => Res1) is incorrect. Source: %s, Target: %v", arg3.Source.Value, arg3.Target)
	}
}

func TestNestedComments(t *testing.T) {
	input := `
		VAR
			myVar : INT; (* outer (* middle (* inner *) middle *) outer *)
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("Expected an error for nested comments, but got none")
	}

	expectedError := "illegal character \"nested comment\""
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}
}

func TestUnterminatedCommentErrorRecovery(t *testing.T) {
	input := `
		VAR
			myVar : INT; (* This comment is not closed...
			anotherVar : BOOL;
		END_VAR

		// The parser should recover and parse this statement
		myVar := 10;
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("Expected parser to have errors, but it had none.")
	}

	// Check that the unterminated comment error was reported
	expectedError := "unterminated comment"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed the statement after the error
	if len(program.Statements) < 1 {
		t.Fatalf("Parser did not recover, expected at least 1 statement to be parsed. got=%d", len(program.Statements))
	}
}

func TestMissingSemicolonErrorRecovery(t *testing.T) {
	input := `
		VAR
			myVar : INT := 5 // Missing semicolon
			anotherVar : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 1 error, but it had %d", len(p.Errors()))
	}

	expectedError := "expected next token to be ;, got IDENT instead at row 4, column 4"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed the rest of the block
	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(varBlock.Declarations) != 2 {
		t.Fatalf("Parser did not recover, expected 2 declarations to be parsed. got=%d", len(varBlock.Declarations))
	}
}

func TestMissingThenErrorRecovery(t *testing.T) {
	input := `
		IF x < y // Missing THEN
			x := 1;
		ELSE
			y := 1;
		END_IF
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 4 {
		t.Fatalf("Expected parser to have 4 errors, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "expected next token to be THEN, got IDENT instead at row 3, column 4"
	expectedError2 := "no prefix parse function for ; found"
	assertErrorContains(t, p.Errors(), expectedError)
	assertErrorContains(t, p.Errors(), expectedError2)

	// Check that the parser recovered and parsed the full IF statement
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 1 statement to be parsed. got=%d", len(program.Statements))
	}

	ifStmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	// The parser should recover from the missing THEN and still parse the consequence.
	if ifStmt.Consequence == nil || len(ifStmt.Consequence.Statements) != 1 {
		t.Errorf("IF statement consequence should have 1 statement after recovery, but has %d", len(ifStmt.Consequence.Statements))
	}

	// The parser should recover and parse the ELSE block.
	altBlock, ok := ifStmt.Alternative.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("ifStmt.Alternative is not *ast.BlockStatement. got=%T", ifStmt.Alternative)
	}
	if len(altBlock.Statements) != 1 {
		t.Fatalf("Alternative block should have 1 statement. got=%d", len(altBlock.Statements))
	}
	altStmt, ok := altBlock.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Alternative statement is not *ast.AssignmentStatement. got=%T", altBlock.Statements[0])
	}
	testIdentifier(t, altStmt.Left, "y")
	testIntegerLiteral(t, altStmt.Value, 1)
}

func TestMissingDoErrorRecovery(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
		expectedErrs  int
	}{
		{
			"Missing DO in FOR loop",
			`FOR i := 1 TO 10
				x := x + 1;
			 END_FOR`,
			"expected next token to be DO, got IDENT instead",
			3,
		},
		{
			"Missing DO in WHILE loop",
			`WHILE x < 10
				x := x + 1;
			 END_WHILE`,
			"missing 'DO' in WHILE loop",
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != tt.expectedErrs {
				t.Fatalf("Expected parser to have %d error(s), but it had %d: %v", tt.expectedErrs, len(p.Errors()), p.Errors())
			}

			// Check that the primary expected error is present among the reported errors.
			assertErrorContains(t, p.Errors(), tt.expectedError)

			// Check that the parser recovered and parsed the loop body
			if len(program.Statements) != 1 {
				t.Fatalf("Parser did not recover, expected 1 statement to be parsed. got=%d", len(program.Statements))
			}

			// A simple check to see if the body was parsed at all
			if program.Statements[0].String() == "" {
				t.Error("Loop statement was not parsed correctly after recovery.")
			}
		})
	}
}

func TestMissingEndBlockErrorRecovery(t *testing.T) {
	input := `
		IF x < y THEN
			x := 1;
		// Missing END_IF here

		y := 2; // This should still be parsed
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 2 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "missing 'END_IF' for IF statement starting at row 2 at row 7, column 3"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the IF and the subsequent assignment
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Errorf("First statement should be an IfStatement after recovery.")
	}
}

func TestMissingEndVarErrorRecovery(t *testing.T) {
	input := `
		VAR
			myVar : INT;
		// Missing END_VAR here

		IF myVar > 0 THEN
			myVar := 0;
		END_IF
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 1 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "expected next token to be END_VAR, got IF instead at row 6, column 3"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the VAR block and the IF statement
	if len(program.Statements) != 2 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok || len(varBlock.Declarations) != 1 {
		t.Errorf("First statement should be a VarBlockDeclaration with 1 declaration.")
	}

	_, ok = program.Statements[1].(*ast.IfStatement)
	if !ok {
		t.Errorf("Second statement should be an IfStatement after recovery.")
	}
}

func TestMissingEndFunctionBlockErrorRecovery(t *testing.T) {
	input := `
		FUNCTION_BLOCK MyFB
			VAR
				x : INT;
			END_VAR
			x := 1;
		// Missing END_FUNCTION_BLOCK here

		VAR
			y : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 1 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "expected next token to be END_FUNCTION_BLOCK, got EOF instead at row 12, column 3"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the FUNCTION_BLOCK and the subsequent VAR block
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Errorf("First statement should be a FunctionBlockDeclaration after recovery.")
	}
}

func TestMissingEndProgramErrorRecovery(t *testing.T) {
	input := `
		PROGRAM MyProg
			VAR
				x : INT;
			END_VAR
			x := 1;
		// Missing END_PROGRAM here

		VAR
			y : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 1 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "expected next token to be END_PROGRAM, got EOF instead at row 12, column 3"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the PROGRAM and the subsequent VAR block
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ProgramDeclaration); !ok {
		t.Errorf("First statement should be a ProgramDeclaration after recovery.")
	}
}

func testVarDeclStatement(t *testing.T, s *ast.VarDeclStatement, name string, dataType string) bool {
	if s.TokenLiteral() != name && s.Token.Type != token.VAR {
		t.Errorf("s.TokenLiteral not '%s' or 'VAR'. got=%q", name, s.TokenLiteral())
		return false
	}

	if s.Name.Value != name {
		t.Errorf("s.Name.Value not '%s'. got=%s", name, s.Name.Value)
		return false
	}

	if s.Name.TokenLiteral() != name {
		t.Errorf("s.Name.TokenLiteral() not '%s'. got=%s",
			name, s.Name.TokenLiteral())
		return false
	}

	if s.DataType.String() != dataType && s.DataType.TokenLiteral() != dataType {
		t.Errorf("s.DataType.String() not '%s'. got=%s",
			dataType, s.DataType.String())
		return false
	}

	return true
}

func testInfixExpression(t *testing.T, i int, exp ast.Expression, left interface{},
	operator string, right interface{}) bool {

	opExp, ok := exp.(*ast.InfixExpression)
	if !ok {
		t.Errorf("test[%d] - exp is not ast.InfixExpression. got=%T(%s)", i, exp, exp)
		return false
	}

	if !testLiteralExpression(t, opExp.Left, left) {
		return false
	}

	if opExp.Operator != operator {
		t.Errorf("exp.Operator is not '%s'. got=%q", operator, opExp.Operator)
		return false
	}

	if !testLiteralExpression(t, opExp.Right, right) {
		return false
	}

	return true
}

func testLiteralExpression(
	t *testing.T,
	exp ast.Expression,
	expected interface{},
) bool {
	switch v := expected.(type) {
	case int:
		return testIntegerLiteral(t, exp, int64(v))
	case int64:
		return testIntegerLiteral(t, exp, v)
	case string:
		return testIdentifier(t, exp, v)
	case bool:
		return testBooleanLiteral(t, exp, v)
	}
	t.Errorf("type of exp not handled. got=%T", exp)
	return false
}

func testIntegerLiteral(t *testing.T, il ast.Expression, value int64) bool {
	integ, ok := il.(*ast.IntegerLiteral)
	if !ok {
		t.Errorf("il not *ast.IntegerLiteral. got=%T", il)
		return false
	}

	if integ.Value != value {
		t.Errorf("integ.Value not %d. got=%d", value, integ.Value)
		return false
	}

	if integ.TokenLiteral() != fmt.Sprintf("%d", value) {
		t.Errorf("integ.TokenLiteral not %d. got=%s", value,
			integ.TokenLiteral())
		return false
	}

	return true
}

func testIdentifier(t *testing.T, exp ast.Expression, value string) bool {
	ident, ok := exp.(*ast.Identifier)
	if !ok {
		t.Errorf("exp not *ast.Identifier. got=%T", exp)
		return false
	}

	if ident.Value != value {
		t.Errorf("ident.Value not %s. got=%s", value, ident.Value)
		return false
	}

	if ident.TokenLiteral() != value {
		t.Errorf("ident.TokenLiteral not %s. got=%s", value,
			ident.TokenLiteral())
		return false
	}

	return true
}

func testBooleanLiteral(t *testing.T, exp ast.Expression, value bool) bool {
	bo, ok := exp.(*ast.Boolean)
	if !ok {
		t.Errorf("exp not *ast.Boolean. got=%T", exp)
		return false
	}

	if bo.Value != value {
		t.Errorf("bo.Value not %t. got=%t", value, bo.Value)
		return false
	}

	if strings.ToLower(bo.TokenLiteral()) != fmt.Sprintf("%t", value) {
		t.Errorf("bo.TokenLiteral not case-insensitive for %t. got=%s",
			value, bo.TokenLiteral(),
		)
		return false
	}

	return true
}

func checkParserErrors(t *testing.T, p *Parser, testName string, input string) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("FAIL: %s - parser has %d errors for input:\n%s", testName, len(errors), input)
	for _, msg := range errors {
		t.Errorf("parser error: %s", msg)
	}
	t.FailNow()
}

func isExpectedType(t *testing.T, expectedType interface{}, actual interface{}) bool {
	expected := fmt.Sprintf("%T", expectedType)
	result := fmt.Sprintf("%T", actual)
	return expected == result
}

func assertErrorContains(t *testing.T, errors []string, expected string) {
	t.Helper()
	for _, err := range errors {
		if strings.Contains(err, expected) {
			return // Found it
		}
	}
	t.Errorf("Expected to find error %q in parser errors, but did not. Errors: %v", expected, errors)
}
