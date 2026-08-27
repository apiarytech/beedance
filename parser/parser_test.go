package parser

import (
	"fmt" // cspell:disable-line
	"math/rand"
	"strings"
	"testing"
	"time"

	"beedance/ast"
	"beedance/lexer"
	"beedance/token"
)

func TestSingleVarDeclStatement(t *testing.T) {
	input := `VAR myVar : INT := 5; END_VAR`

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

func TestEnumTypeDeclaration(t *testing.T) {
	input := `
		TYPE
			COLOR : (RED, GREEN, BLUE);
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestEnumTypeDeclaration", input)

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
	if typeDecl.Name.Value != "COLOR" {
		t.Fatalf("Type name is not 'COLOR'. got=%s", typeDecl.Name.Value)
	}

	enumDef, ok := typeDecl.DataType.(*ast.EnumDefinition)
	if !ok {
		t.Fatalf("DataType is not ast.EnumDefinition. got=%T", typeDecl.DataType)
	}

	expectedValues := []string{"RED", "GREEN", "BLUE"}
	if len(enumDef.Values) != len(expectedValues) {
		t.Fatalf("Enum does not have %d values. got=%d", len(expectedValues), len(enumDef.Values))
	}

}

func TestStructTypeDeclaration(t *testing.T) {
	input := `
		TYPE
			MyStruct : STRUCT
				Field1 : INT := 10;
				Field2 : BOOL := TRUE;
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
	if !testVarDeclStatement(t, member1, "Field1", "INT") || !testLiteralExpression(t, member1.Value, 10) {
		return
	}

	// Test second member
	member2 := structDef.Members[1]
	if !testVarDeclStatement(t, member2, "Field2", "BOOL") || !testLiteralExpression(t, member2.Value, true) {
		return
	}
}

func TestVarInFunctionError(t *testing.T) {
	input := `
	FUNCTION MyFunc : INT
		VAR_EXTERNAL
			x : INT;
		END_VAR
		MyFunc := x;
	END_FUNCTION
	`
	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("Expected parser error for VAR in FUNCTION, but got none")
	}

	expectedError := "VAR_EXTERNAL declarations are not allowed in a FUNCTION at row 3, column 3"
	assertErrorContains(t, p.Errors(), expectedError)
}

func TestComplexTypeBlockDeclaration(t *testing.T) {
	input := `
		TYPE
			MyInteger   : INT := 10;
			MySubrange  : INT (0..100);
			MyArray     : ARRAY [1..10] OF BOOL;
			MyStruct    : STRUCT 
				Field1 : INT; 
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
		VAR_CONFIG MyProg
			Config1 : INT;
			Config2 : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestConfigVarDeclarations", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConfigVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigVarDeclaration. got=%T", program.Statements[0])
	}

	if stmt.ProgramInstanceName == nil || stmt.ProgramInstanceName.Value != "MyProg" {
		t.Fatalf("Expected ProgramInstanceName to be 'MyProg'. got=%v", stmt.ProgramInstanceName)
	}

	if len(stmt.Declarations) != 2 {
		t.Fatalf("Expected 2 config variables. got=%d", len(stmt.Declarations))
	}

	if !testVarDeclStatement(t, stmt.Declarations[0], "Config1", "INT") {
		return
	}

	if !testVarDeclStatement(t, stmt.Declarations[1], "Config2", "BOOL") {
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

	decl1 := stmt.Vars[0]
	if decl1.Name.Value != "Path1" {
		t.Errorf("decl1.Name.Value not 'Path1'. got=%s", decl1.Name.Value)
	}
	if decl1.AccessPath.String() != "MyFB" {
		t.Errorf("decl1.AccessPath not 'MyFB'. got=%s", decl1.AccessPath.String())
	}
	if decl1.AccessType != "READ_ONLY" {
		t.Errorf("decl1.AccessType not 'READ_ONLY'. got=%s", decl1.AccessType)
	}
	if decl1.DataType != nil {
		t.Errorf("decl1.DataType should be nil for VAR_ACCESS. got=%v", decl1.DataType)
	}

	decl2 := stmt.Vars[1]
	if decl2.Name.Value != "Path2" {
		t.Errorf("decl2.Name.Value not 'Path2'. got=%s", decl2.Name.Value)
	}
	if decl2.AccessPath.String() != "OtherFB" {
		t.Errorf("decl2.AccessPath not 'OtherFB'. got=%s", decl2.AccessPath.String())
	}
	if decl2.AccessType != "READ_WRITE" {
		t.Errorf("decl2.AccessType not 'READ_WRITE'. got=%s", decl2.AccessType)
	}
	if decl2.DataType != nil {
		t.Errorf("decl2.DataType should be nil for VAR_ACCESS. got=%v", decl2.DataType)
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestGlobalVarDeclarations", input)

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

func TestBitStringLiteralParsing(t *testing.T) {
	tests := []struct { // cspell:disable-line
		input        string
		expectedType string
		expectedVal  string
	}{
		{"BYTE#16#A5;", "BYTE", "16#A5"},
		{"WORD#16#1234;", "WORD", "16#1234"},
		{"DWORD#16#ABCDEF12;", "DWORD", "16#ABCDEF12"},
		{"LWORD#16#1234567890ABCDEF;", "LWORD", "16#1234567890ABCDEF"},
		// Test with different bases
		{"BYTE#10#165;", "BYTE", "10#165"},
		{"BYTE#8#245;", "BYTE", "8#245"},
		{"BYTE#2#1010_0101;", "BYTE", "2#1010_0101"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestBitStringLiteralParsing", tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
		}

		typedLit, ok := stmt.Expression.(*ast.TypedLiteral)
		if !ok {
			t.Fatalf("stmt.Expression is not ast.TypedLiteral. got=%T", stmt.Expression)
		}

		if typedLit.TypeName != tt.expectedType {
			t.Errorf("TypeName not %q. got=%q", tt.expectedType, typedLit.TypeName)
		}

		valIdent, _ := typedLit.Value.(*ast.Identifier)
		if valIdent.Value != tt.expectedVal {
			t.Errorf("Value not %q. got=%q", tt.expectedVal, valIdent.Value)
		}
	}
}

func TestTypedTimeDateLiterals(t *testing.T) {
	tests := []struct {
		input        string
		expectedType string
		expectedVal  string
	}{
		// TIME literals
		{"T#5s;", "T", "5s"},
		{"TIME#1h_30m;", "TIME", "1h_30m"},
		{"T#-10s_500ms;", "T", "-10s_500ms"},

		// DATE literals
		{"D#2026-05-21;", "D", "2026-05-21"},
		{"DATE#2026-05-21;", "DATE", "2026-05-21"},

		// TIME_OF_DAY literals
		{"TOD#14:30:00;", "TOD", "14:30:00"},
		{"TIME_OF_DAY#14:30:00.123;", "TIME_OF_DAY", "14:30:00.123"},

		// DATE_AND_TIME literals
		{"DT#2026-05-21-14:30:00;", "DT", "2026-05-21-14:30:00"},
		{"DATE_AND_TIME#2026-05-21-14:30:00.5;", "DATE_AND_TIME", "2026-05-21-14:30:00.5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p, "TestTypedTimeDateLiterals", tt.input)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
			}

			typedLit, ok := stmt.Expression.(*ast.TypedLiteral)
			if !ok {
				t.Fatalf("stmt.Expression is not ast.TypedLiteral. got=%T for input %q", stmt.Expression, tt.input)
			}

			if typedLit.TypeName != tt.expectedType {
				t.Errorf("TypeName not %q. got=%q", tt.expectedType, typedLit.TypeName)
			}

			valIdent, _ := typedLit.Value.(*ast.Identifier)
			if valIdent.Value != tt.expectedVal {
				t.Errorf("Value not %q. got=%q", tt.expectedVal, valIdent.Value)
			}
		})
	}
}

func TestRealLiteralExpression(t *testing.T) {
	tests := []struct {
		input         string
		expectedValue float64
	}{
		{"3.14;", 3.14},
		{"1.23E4;", 12300.0},
		{"-1.23e-4;", -0.000123},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestRealLiteralExpression", tt.input) // cspell:disable-line

		if len(program.Statements) != 1 {
			t.Fatalf("program has not enough statements. got=%d", len(program.Statements))
		}
		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0]) // cspell:disable-line
		}

		testRealLiteral(t, stmt.Expression, tt.expectedValue)
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

func testTypedRealLiteral(t *testing.T, exp ast.Expression, typeName string, value float64) bool {
	t.Helper()
	typedLit, ok := exp.(*ast.TypedLiteral)
	if !ok {
		t.Errorf("exp not *ast.TypedLiteral. got=%T", exp)
		return false
	}

	if typedLit.TypeName != typeName {
		t.Errorf("typedLit.TypeName not %q. got=%q", typeName, typedLit.TypeName)
		return false
	}

	return testRealLiteral(t, typedLit.Value, value)
}

func testRealLiteral(t *testing.T, exp ast.Expression, value float64) bool {
	t.Helper()
	realLit, ok := exp.(*ast.RealLiteral)
	if !ok {
		// It might be a prefix expression for negative numbers
		if prefix, ok := exp.(*ast.PrefixExpression); ok && prefix.Operator == "-" {
			return testRealLiteral(t, prefix.Right, -value)
		}
		t.Errorf("exp not *ast.RealLiteral. got=%T", exp)
		return false
	}

	// Use a small tolerance (epsilon) for float comparison to avoid precision issues.
	const epsilon = 1e-9
	if diff := realLit.Value - value; diff < -epsilon || diff > epsilon {
		t.Errorf("realLit.Value not %g. got=%g", value, realLit.Value)
		return false
	}
	return true
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
	checkParserErrors(t, p, "TestIfElseStatement", input)

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

func TestProgramWithSFCBody(t *testing.T) {
	input := `
		PROGRAM MySFCProgram
			VAR
				cond : BOOL;
				x : INT := 0;
			END_VAR

			ACTION Step1Action:
				x := x + 1;
				cond := TRUE;
			END_ACTION

			ACTION Step2Action:
				x := x * 2;
				cond := FALSE;
			END_ACTION

			INITIAL_STEP S1: Step1Action(); END_STEP

			TRANSITION FROM S1 TO S2 := cond; END_TRANSITION

			STEP S2: Step2Action(); END_STEP
		END_PROGRAM
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramWithSFCBody", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	progDecl, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	if progDecl.Name.Value != "MySFCProgram" {
		t.Fatalf("Program name is not 'MySFCProgram'. got=%s", progDecl.Name.Value)
	}

	sfcBody, ok := progDecl.Body.(*ast.SFCProgram)
	if !ok {
		t.Fatalf("Program body is not ast.SFCProgram. got=%T", progDecl.Body)
	}

	if len(sfcBody.Elements) != 5 {
		t.Fatalf("SFC body does not have 5 elements. got=%d", len(sfcBody.Elements))
	}

	// --- Detailed check of the first element: INITIAL_STEP S1 ---
	initialStep, ok := sfcBody.Elements[2].(*ast.StepStatement)
	if !ok {
		t.Fatalf("Element 2 is not ast.StepStatement. got=%T", sfcBody.Elements[2])
	}
	if !initialStep.IsInitial {
		t.Error("First step should be initial.")
	}
	if initialStep.Name.Value != "S1" {
		t.Errorf("Initial step name is not 'S1'. got=%s", initialStep.Name.Value)
	}

	// --- Detailed check of the second element: TRANSITION ---
	transition, ok := sfcBody.Elements[3].(*ast.TransitionStatement)
	if !ok {
		t.Fatalf("Element 3 is not ast.TransitionStatement. got=%T", sfcBody.Elements[3])
	}
	if len(transition.From) != 1 || transition.From[0].Value != "S1" {
		t.Errorf("Transition 'FROM' is not 'S1'. got=%v", transition.From)
	}
	if len(transition.To) != 1 || transition.To[0].Value != "S2" {
		t.Errorf("Transition 'TO' is not 'S2'. got=%v", transition.To)
	}
	testIdentifier(t, transition.Condition, "cond")

	// --- Detailed check of the third element: STEP S2 ---
	step2, ok := sfcBody.Elements[4].(*ast.StepStatement)
	if !ok {
		t.Fatalf("Element 4 is not ast.StepStatement. got=%T", sfcBody.Elements[4])
	}
	if step2.Name.Value != "S2" {
		t.Errorf("Step name is not 'S2'. got=%s", step2.Name.Value)
	}
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

			MyFunction := (A + B) * 2;
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

	if len(stmt.Vars) != 0 {
		t.Fatalf("Expected 0 VARs. got=%d", len(stmt.Vars))
	}

	body, ok := stmt.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Function body is not a BlockStatement. got=%T", stmt.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Function body does not have 1 statement. got=%d", len(body.Statements))
	}
	// Test statement in body: MyFunction := (A + B) * 2;
	stmt1, ok := body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement 1 is not ast.AssignmentStatement. got=%T", body.Statements[0])
	}
	testIdentifier(t, stmt1.Left, "MyFunction")
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

	body, ok := stmt.Body.(*ast.BlockStatement)
	if len(body.Statements) != 1 {
		t.Fatalf("Function body should have 1 statement. got=%d", len(body.Statements))
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
	input := `'hello world';`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStringLiteralExpression", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d", len(program.Statements))
	}

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	literal, ok := stmt.Expression.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("exp not *ast.StringLiteral. got=%T", stmt.Expression)
	}

	if literal.Value != "hello world" { // cspell:disable-line
		t.Errorf("literal.Value not %q. got=%q", "hello world", literal.Value) // cspell:disable-line
	}
}

func TestWStringLiteralExpression(t *testing.T) {
	input := `"hello world";` // cspell:disable-line

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestWStringLiteralExpression", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d", len(program.Statements))
	}

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	_, ok := stmt.Expression.(*ast.WStringLiteral)
	if !ok {
		t.Fatalf("exp not *ast.WStringLiteral. got=%T", stmt.Expression)
	}
}

func TestAllVarBlockTypes(t *testing.T) {
	// Seed the random number generator for varied testing
	// cspell:disable-next-line
	source := rand.NewSource(time.Now().UnixNano())
	// cspell:disable-next-line
	rng := rand.New(source)

	// Define POU wrappers to randomly place VAR blocks inside them
	pouWrappers := []struct {
		start        string
		end          string
		expectedType interface{}
	}{
		{"PROGRAM TestProg ", " END_PROGRAM", &ast.ProgramDeclaration{}},
		{"FUNCTION TestFunc : INT ", " END_FUNCTION", &ast.FunctionDeclaration{}},
		{"FUNCTION_BLOCK TestFB ", " END_FUNCTION_BLOCK", &ast.FunctionBlockDeclaration{}},
	}

	tests := []struct {
		varBlock      string
		getVarDecls   func(stmt ast.Statement) []*ast.VarDeclStatement
		isBlock       bool // True if it's a block like VAR_GLOBAL, not a simple VAR_INPUT
		blockExpected interface{}
	}{
		{
			"VAR_INPUT myVar : INT; END_VAR", func(stmt ast.Statement) []*ast.VarDeclStatement {
				switch s := stmt.(type) {
				case *ast.ProgramDeclaration:
					return s.VarInputs
				case *ast.FunctionDeclaration:
					return s.VarInputs
				case *ast.FunctionBlockDeclaration:
					return s.VarInputs
				}
				return nil
			}, false, nil,
		},
		{
			"VAR_OUTPUT myVar : SINT; END_VAR", func(stmt ast.Statement) []*ast.VarDeclStatement {
				switch s := stmt.(type) {
				case *ast.ProgramDeclaration:
					return s.VarOutputs
				case *ast.FunctionDeclaration:
					return s.VarOutputs
				case *ast.FunctionBlockDeclaration:
					return s.VarOutputs
				}
				return nil
			}, false, nil,
		},
		{
			"VAR_IN_OUT myVar : DINT; END_VAR", func(stmt ast.Statement) []*ast.VarDeclStatement {
				switch s := stmt.(type) {
				case *ast.ProgramDeclaration:
					return s.VarInOuts
				case *ast.FunctionDeclaration:
					return s.VarInOuts
				case *ast.FunctionBlockDeclaration:
					return s.VarInOuts
				}
				return nil
			}, false, nil,
		},
		{
			"VAR myVar : LINT; END_VAR", func(stmt ast.Statement) []*ast.VarDeclStatement {
				switch s := stmt.(type) {
				case *ast.ProgramDeclaration:
					return s.Vars
				case *ast.FunctionDeclaration:
					return s.Vars
				case *ast.FunctionBlockDeclaration:
					return s.Vars
				}
				return nil
			}, false, nil,
		},
	}

	for i, tt := range tests {
		// Randomly select a POU wrapper for the current test case
		wrapper := pouWrappers[rng.Intn(len(pouWrappers))]
		// Skip invalid combinations: VAR blocks are not allowed in FUNCTIONs
		if strings.Contains(wrapper.start, "FUNCTION ") && strings.HasPrefix(tt.varBlock, "VAR ") {
			continue
		}

		input := wrapper.start + tt.varBlock + wrapper.end

		l := lexer.New(input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, "TestAllVarBlockTypes", input)

		if len(program.Statements) != 1 {
			t.Fatalf("test[%d] input: %s\nprogram.Statements does not contain 1 statement. got=%d", i, input, len(program.Statements))
		}

		// Check that the top-level statement is the correct POU type
		if !isExpectedType(t, wrapper.expectedType, program.Statements[0]) {
			t.Fatalf("test[%d] input: %s\nprogram.Statements[0] is not of expected type %T. got=%T", i, input, wrapper.expectedType, program.Statements[0])
		}

		// Use the accessor function to get the list of var declarations
		decls := tt.getVarDecls(program.Statements[0])
		if decls == nil {
			t.Fatalf("test[%d] input: %s\ngetVarDecls returned nil for type %T", i, input, program.Statements[0])
		}

		if len(decls) != 1 {
			t.Fatalf("test[%d] input: %s\nExpected 1 var declaration inside the POU. got=%d", i, input, len(decls))
		}
	}
}

func TestParsingEmptyArrayLiterals(t *testing.T) { // cspell:disable-line
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
	input := `{'one': 1, 'two': 2, 'three': 3};`

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
		var keyStr string
		switch k := key.(type) {
		case *ast.StringLiteral:
			keyStr = k.Value
		case *ast.WStringLiteral:
			keyStr = k.Value
		default:
			t.Errorf("key is not a string literal. got=%T", key)
			continue
		}

		expectedValue, ok := expected[keyStr]
		if !ok {
			t.Errorf("unexpected key in hash: %s", keyStr)
		}
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

func TestParsingArrayLiteralsWithRepetition(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"[3(0)];",
			"[3(0)];",
		},
		{
			"[2(1, 2, 3)];",
			"[2(1, 2, 3)];",
		},
		{
			"[3(0), 2(1)];",
			"[3(0), 2(1)];",
		},
		{
			"[1, 3(0), 2, 2(5, 6)];",
			"[1, 3(0), 2, 2(5, 6)];",
		},
		{
			"[2(myVar + 1)];",
			"[2((myVar + 1))];",
		},
	}

	for i, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p, fmt.Sprintf("TestParsingArrayLiteralsWithRepetition[%d]", i), tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("Test[%d] - program.Statements does not contain 1 statement. got=%d", i, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("Test[%d] - program.Statements[0] is not ast.ExpressionStatement. got=%T", i, program.Statements[0])
		}

		arrayLit, ok := stmt.Expression.(*ast.ArrayLiteral)
		if !ok {
			t.Fatalf("Test[%d] - stmt.Expression is not *ast.ArrayLiteral. got=%T", i, stmt.Expression)
		}

		actual := arrayLit.String() + ";" // Add semicolon back for comparison with expected
		if actual != tt.expected {
			t.Errorf("Test[%d] - ArrayLiteral.String() wrong. want=%q, got=%q", i, tt.expected, actual)
		}
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
		var keyStr string
		switch k := key.(type) {
		case *ast.StringLiteral:
			keyStr = k.Value
		case *ast.WStringLiteral:
			keyStr = k.Value
		default:
			t.Errorf("key is not a string literal. got=%T", key)
			continue
		}

		testFunc, ok := tests[keyStr]
		if !ok {
			t.Errorf("No test function for key %q found", keyStr)
			continue
		}

		testFunc(value)
	}
}

func TestMacroLiteralParsing(t *testing.T) {
	input := `MACRO(x, y) { x + y; };`

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
		VAR
			iMachineState : INT;
			iSpeed : INT;
		END_VAR

		CASE iMachineState OF
			0 : 
				(* Single Label: Handled exactly when iMachineState = 0 *)
				iSpeed := 0;
				
			1, 2, 3 : 
				(* Comma-Separated Labels: Handled when iMachineState is 1, 2, or 3 *)
				iSpeed := 50;
				
			4..7 : 
				(* Subrange Labels: Handled when iMachineState is between 4 and 7 (inclusive) *)
				iSpeed := 100;
				
			ELSE 
				(* Catch-all: Handled if iMachineState does not match any label above *)
				iSpeed := -1; 
		END_CASE
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestCaseStatement", input)

	if len(program.Statements) != 2 {
		t.Fatalf("program.Statements does not contain 2 statements. got=%d", len(program.Statements))
	}

	// The first statement is the VAR block, which we can ignore for this test's purpose.
	stmt, ok := program.Statements[1].(*ast.CaseStatement)
	if !ok {
		t.Fatalf("program.Statements[1] is not ast.CaseStatement. got=%T", program.Statements[1])
	}

	if !testIdentifier(t, stmt.Expression, "iMachineState") {
		return
	}

	if len(stmt.Cases) != 3 {
		t.Fatalf("case statement does not have 3 cases. got=%d", len(stmt.Cases))
	}

	// Test first case: 0 : iSpeed := 0;
	case1 := stmt.Cases[0]
	if len(case1.Values) != 1 || !testIntegerLiteral(t, case1.Values[0], 0) {
		t.Errorf("incorrect values for case 1. got=%v", case1.Values)
	}
	if len(case1.Consequence.Statements) != 1 {
		t.Fatalf("Consequence for case 1 should have 1 statement, got %d", len(case1.Consequence.Statements))
	}
	consequence1, ok := case1.Consequence.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("consequence for case 1 is not AssignmentStatement. got=%T", case1.Consequence.Statements[0])
	}
	testIdentifier(t, consequence1.Left, "iSpeed")
	testIntegerLiteral(t, consequence1.Value, 0)

	// Test second case: 1, 2, 3 : iSpeed := 50;
	case2 := stmt.Cases[1]
	if len(case2.Values) != 3 || !testIntegerLiteral(t, case2.Values[0], 1) || !testIntegerLiteral(t, case2.Values[1], 2) || !testIntegerLiteral(t, case2.Values[2], 3) {
		t.Errorf("incorrect values for case 2. got=%v", case2.Values)
	}
	if len(case2.Consequence.Statements) != 1 {
		t.Fatalf("Consequence for case 2 should have 1 statement, got %d", len(case2.Consequence.Statements))
	}
	consequence2, ok := case2.Consequence.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("consequence for case 2 is not AssignmentStatement. got=%T", case2.Consequence.Statements[0])
	}
	testIdentifier(t, consequence2.Left, "iSpeed")
	testIntegerLiteral(t, consequence2.Value, 50)

	// Test third case: 4..7 : iSpeed := 100;
	case3 := stmt.Cases[2]
	if len(case3.Values) != 1 {
		t.Errorf("incorrect values for case 3. got=%v", case3.Values)
	}
	if !testInfixExpression(t, 0, case3.Values[0], 4, "..", 7) {
		return
	}
	if len(case3.Consequence.Statements) != 1 {
		t.Fatalf("Consequence for case 3 should have 1 statement, got %d", len(case3.Consequence.Statements))
	}
	consequence3, ok := case3.Consequence.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("consequence for case 3 is not AssignmentStatement. got=%T", case3.Consequence.Statements[0])
	}
	testIdentifier(t, consequence3.Left, "iSpeed")
	testIntegerLiteral(t, consequence3.Value, 100)

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
	testIdentifier(t, elseConsequence.Left, "iSpeed")
	testIntegerLiteral(t, elseConsequence.Value, -1)
}
func TestCaseStatementWithEnums(t *testing.T) {
	input := `
		TYPE
			COLOR : (RED, GREEN, BLUE);
		END_TYPE

		VAR
			myColor : COLOR;
			x : INT;
		END_VAR

		CASE myColor OF
			COLOR#RED : x := 1;
			COLOR#GREEN, COLOR#BLUE : x := 2;
		END_CASE
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestCaseStatementWithEnums", input)

	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[2].(*ast.CaseStatement)
	if !ok {
		t.Fatalf("program.Statements[2] is not ast.CaseStatement. got=%T", program.Statements[2])
	}

	if !testIdentifier(t, stmt.Expression, "myColor") {
		return
	}

	if len(stmt.Cases) != 2 {
		t.Fatalf("case statement does not have 2 cases. got=%d", len(stmt.Cases))
	}

	// Test first case: COLOR#RED: x := 1;
	case1 := stmt.Cases[0]
	if len(case1.Values) != 1 {
		t.Fatalf("incorrect number of values for case 1. got=%d", len(case1.Values))
	}
	testTypedLiteral(t, case1.Values[0], "COLOR", "RED")

	// Test second case: COLOR#GREEN, COLOR#BLUE: x := 2;
	case2 := stmt.Cases[1]
	if len(case2.Values) != 2 {
		t.Fatalf("incorrect number of values for case 2. got=%d", len(case2.Values))
	}
	testTypedLiteral(t, case2.Values[0], "COLOR", "GREEN")
	testTypedLiteral(t, case2.Values[1], "COLOR", "BLUE")
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

	body, ok := stmt.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Function block body is not a BlockStatement. got=%T", stmt.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Function block body does not have 1 statement. got=%d", len(body.Statements))
	}

	bodyStmt, ok := body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", body.Statements[0])
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

	body, ok := stmt.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Program body is not a BlockStatement. got=%T", stmt.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Program body does not have 1 statement. got=%d", len(body.Statements))
	}

	bodyStmt, ok := body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", body.Statements[0])
	}
	testIdentifier(t, bodyStmt.Left, "LocalVar")
	testIntegerLiteral(t, bodyStmt.Value, 10)
}

func TestProgramDeclarationWithGlobalVar(t *testing.T) {
	input := `
		PROGRAM MyProgramWithGlobals
			VAR_GLOBAL
				Global1 : INT;
			END_VAR

			VAR
				Local1 : BOOL;
			END_VAR

			Local1 := TRUE;
		END_PROGRAM
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramDeclarationWithGlobalVar", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	progDecl, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	if progDecl.Name.Value != "MyProgramWithGlobals" {
		t.Errorf("Program name is not 'MyProgramWithGlobals'. got=%s", progDecl.Name.Value)
	}

	// Check for VAR_GLOBAL block
	if len(progDecl.VarGlobal) != 1 {
		t.Fatalf("Expected 1 VAR_GLOBAL block. got=%d", len(progDecl.VarGlobal))
	}

	globalBlock := progDecl.VarGlobal[0]
	if len(globalBlock.Vars) != 1 {
		t.Fatalf("Expected 1 global variable declaration. got=%d", len(globalBlock.Vars))
	}
	if !testVarDeclStatement(t, globalBlock.Vars[0], "Global1", "INT") {
		return
	}

	// Check for local VAR block
	if len(progDecl.Vars) != 1 {
		t.Fatalf("Expected 1 local VAR declaration. got=%d", len(progDecl.Vars))
	}
	if !testVarDeclStatement(t, progDecl.Vars[0], "Local1", "BOOL") {
		return
	}

	// Check body
	body, ok := progDecl.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Program body is not a BlockStatement. got=%T", progDecl.Body)
	}
	if len(body.Statements) != 1 {
		t.Fatalf("Program body does not have 1 statement. got=%d", len(body.Statements))
	}
}

func TestFunctionBlockDeclarationWithInOut(t *testing.T) {
	input := `
		FUNCTION_BLOCK MyFBWithInOut
			VAR_IN_OUT
				InOutVar : REAL;
			END_VAR

			InOutVar := InOutVar + 1.0;
		END_FUNCTION_BLOCK
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionBlockDeclarationWithInOut", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if fb.Name.Value != "MyFBWithInOut" {
		t.Errorf("Function block name is not 'MyFBWithInOut'. got=%s", fb.Name.Value)
	}

	if len(fb.VarInOuts) != 1 {
		t.Fatalf("Expected 1 VAR_IN_OUT declaration. got=%d", len(fb.VarInOuts))
	}

	inOutVar := fb.VarInOuts[0]
	if !testVarDeclStatement(t, inOutVar, "InOutVar", "REAL") {
		return
	}

	body, ok := fb.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Function block body is not a BlockStatement. got=%T", fb.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Function block body does not have 1 statement. got=%d", len(body.Statements))
	}

	testAssignmentStatement(t, body.Statements[0], "InOutVar", "(InOutVar + 1.0)")
}

func TestFunctionBlockWithSFCBody(t *testing.T) {
	input := `
		FUNCTION_BLOCK MySFC_FB
			VAR
				cond : BOOL;
			END_VAR

			INITIAL_STEP S1:
			END_STEP

			TRANSITION FROM S1 TO S2 := cond; END_TRANSITION

			STEP S2:
			END_STEP
		END_FUNCTION_BLOCK
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionBlockWithSFCBody", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if fb.Name.Value != "MySFC_FB" {
		t.Errorf("Function block name is not 'MySFC_FB'. got=%s", fb.Name.Value)
	}

	sfcBody, ok := fb.Body.(*ast.SFCProgram)
	if !ok {
		t.Fatalf("Function block body is not *ast.SFCProgram. got=%T", fb.Body)
	}

	if len(sfcBody.Elements) != 3 {
		t.Fatalf("SFC body should have 3 elements. got=%d", len(sfcBody.Elements))
	}

	// Check the first element (INITIAL_STEP)
	initialStep, ok := sfcBody.Elements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("SFC element 0 is not *ast.StepStatement. got=%T", sfcBody.Elements[0])
	}
	if !initialStep.IsInitial {
		t.Error("First step should be an initial step.")
	}
	if initialStep.Name.Value != "S1" {
		t.Errorf("Initial step name is not 'S1'. got=%s", initialStep.Name.Value)
	}
}

func TestIlProgramParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, body *ast.BlockStatement)
	}{
		{
			"Basic IL Instructions",
			`
				FUNCTION_BLOCK MyIlProgram
					VAR
						Start : BOOL;
						Counter : INT;
						Result : INT;
					END_VAR

					LD    Start
					ADD   Counter
					ST    Result
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 3 {
					t.Fatalf("Function block body does not have 3 statements. got=%d", len(body.Statements))
				}

				// Test instruction 1: LD Start
				inst1 := testIlInstruction(t, body.Statements[0], "LD", "Start")
				// Test instruction 2: ADD Counter
				inst2 := testIlInstruction(t, body.Statements[1], "ADD", "Counter")
				// Test instruction 3: ST Result
				inst3 := testIlInstruction(t, body.Statements[2], "ST", "Result")

				if inst1 == nil || inst2 == nil || inst3 == nil {
					t.Fatalf("One or more IL instructions failed to parse correctly.")
				}
			},
		},
		{
			"Simple LD/ST",
			`
				FUNCTION_BLOCK TestFB
					VAR A, B : INT; END_VAR
					LD    A
					ST    B
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(body.Statements))
				}
				testIlInstruction(t, body.Statements[0], "LD", "A")
				testIlInstruction(t, body.Statements[1], "ST", "B")
			},
		},
		{
			"Labels and Modifiers",
			`
				FUNCTION_BLOCK TestFB
					VAR C : BOOL; END_VAR
					Loop: LDN   C
					JMPC  Loop
					RET
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 3 {
					t.Fatalf("Expected 3 statements, got %d", len(body.Statements))
				}
				inst1 := testIlInstruction(t, body.Statements[0], "LD", "C", "N")
				if inst1.Label == nil || inst1.Label.Value != "Loop" {
					t.Errorf("Instruction 1 has incorrect label. want='Loop', got=%v", inst1.Label)
				}
				testIlInstruction(t, body.Statements[1], "JMP", "Loop", "C")
				testIlInstruction(t, body.Statements[2], "RET", "")
			},
		},
		{
			"Parenthesized Expression",
			`
				FUNCTION_BLOCK TestFB
					VAR A, B, C : INT; END_VAR
					LD(
						LD A
						ADD B )
					ST C
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(body.Statements))
				}
				// Manually check the LD( instruction because testIlInstruction is too simple for it.
				ldInst, ok := body.Statements[0].(*ast.IlInstructionStatement)
				if !ok {
					t.Fatalf("Statement 0 is not IlInstructionStatement. got=%T", body.Statements[0])
				}
				if ldInst.Operator != "LD" {
					t.Errorf("Operator is not LD. got=%s", ldInst.Operator)
				}
				if ldInst.Modifier != "(" {
					t.Errorf("Modifier is not '('. got=%s", ldInst.Modifier)
				}
				parenBlock, ok := ldInst.Operand.(*ast.BlockStatement)
				if !ok {
					t.Fatalf("Operand of LD( is not a BlockStatement. got=%T", ldInst.Operand)
				}
				if len(parenBlock.Statements) != 2 {
					t.Fatalf("Parenthesized block should have 2 statements. got=%d", len(parenBlock.Statements))
				}
				testIlInstruction(t, parenBlock.Statements[0], "LD", "A")
				testIlInstruction(t, parenBlock.Statements[1], "ADD", "B")

				testIlInstruction(t, body.Statements[1], "ST", "C")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p, tt.name, tt.input)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
			}

			fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
			}

			body, ok := fb.Body.(*ast.BlockStatement)
			if !ok {
				t.Fatalf("Function block body is not a BlockStatement. got=%T", fb.Body)
			}

			tt.check(t, body)
		})
	}
}

func TestActionStatement(t *testing.T) {
	input := `
		ACTION MyAction: 
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

	body, ok := stmt.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Action body is not a BlockStatement. got=%T", stmt.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Action body does not have 1 statement. got=%d", len(body.Statements))
	}

	bodyStmt, ok := body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", body.Statements[0])
	}

	testIdentifier(t, bodyStmt.Left, "x")
	testInfixExpression(t, 0, bodyStmt.Value, "x", "+", 1)
}

func testIlInstruction(t *testing.T, s ast.Statement, operator, operand string, modifiers ...string) *ast.IlInstructionStatement {
	t.Helper()
	inst, ok := s.(*ast.IlInstructionStatement)
	if !ok {
		t.Fatalf("Statement is not IlInstructionStatement. got=%T", s)
		return nil
	}

	if inst.Operator != operator {
		t.Errorf("Instruction operator is incorrect. want=%q, got=%q", operator, inst.Operator)
	}

	if operand != "" {
		if inst.Operand == nil {
			t.Errorf("Instruction operand is nil, want=%q", operand)
		} else if inst.Operand.String() != operand {
			t.Errorf("Instruction operand is incorrect. want=%q, got=%q", operand, inst.Operand.String())
		}
	} else {
		if inst.Operand != nil {
			t.Errorf("Instruction operand is not nil, want=nil, got=%q", inst.Operand.String())
		}
	}

	expectedModifier := ""
	if len(modifiers) > 0 {
		expectedModifier = modifiers[0]
	}

	if inst.Modifier != expectedModifier {
		t.Errorf("Instruction modifier is incorrect. want=%q, got=%q", expectedModifier, inst.Modifier)
	}

	return inst
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStepStatement", input)

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

	action1 := stmt.Actions[0]
	if action1.ActionName.Value != "Action1" {
		t.Errorf("Incorrect first action name. Expected 'Action1', got %s", action1.ActionName.Value)
	}
	if action1.Qualifier == nil || action1.Qualifier.Value != "N" {
		t.Errorf("Incorrect first action qualifier. Expected 'N', got %v", action1.Qualifier)
	}

	action2 := stmt.Actions[1]
	if action2.ActionName.Value != "Action2" {
		t.Errorf("Incorrect second action name. Expected 'Action2', got %s", action2.ActionName.Value)
	}
	if action2.Qualifier == nil || action2.Qualifier.Value != "P" {
		t.Errorf("Incorrect second action qualifier. Expected 'P', got %v", action2.Qualifier)
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
	checkParserErrors(t, p, "TestInitialStepStatement", input)

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
		t.Fatalf("Expected 1 action association in step body. got=%d", len(stmt.Actions))
	}

	action := stmt.Actions[0]
	if action.ActionName.Value != "InitAction" {
		t.Errorf("Incorrect action name. Expected 'InitAction', got %s", action.ActionName.Value)
	}
	if action.Qualifier == nil || action.Qualifier.Value != "N" {
		t.Errorf("Incorrect action qualifier. Expected 'N', got %v", action.Qualifier)
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

			VAR_CONFIG Prog1
				Input1 : INT := 42;
			END_VAR
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestConfigurationDeclaration", input)

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

	if len(stmt.VarConfigs) != 1 {
		t.Fatalf("Expected 1 VAR_CONFIG block. got=%d", len(stmt.VarConfigs))
	}

	cfgVar := stmt.VarConfigs[0]
	if cfgVar.ProgramInstanceName.Value != "Prog1" {
		t.Errorf("VAR_CONFIG instance name is not 'Prog1'. got=%s", cfgVar.ProgramInstanceName.Value)
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

func TestNestedCommentsAreIllegal(t *testing.T) {
	input := `
		(* outer (* middle *) outer *)
	`
	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("Expected an error for nested comments, but got none")
	}

	expectedError := "illegal character \"nested comment\""
	assertErrorContains(t, p.Errors(), expectedError)
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
			myVar : INT := 5
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
		IF x < y
			x := 1;
		ELSE
			y := 1;
		END_IF
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	// 1. Check that exactly one error was reported.
	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 1 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	// 2. Check that the error is the one we expect.
	expectedError := "expected next token to be THEN, got IDENT instead"
	assertErrorContains(t, p.Errors(), expectedError)

	// 3. Check that the parser recovered and parsed the full IF statement structure.
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 1 statement to be parsed. got=%d", len(program.Statements))
	}

	ifStmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	// 4. Verify the condition was parsed.
	if !testInfixExpression(t, 0, ifStmt.Condition, "x", "<", "y") {
		return
	}

	// 5. Verify the consequence was parsed, even with the missing THEN.
	if ifStmt.Consequence == nil || len(ifStmt.Consequence.Statements) != 1 {
		t.Fatalf("IF statement consequence should have 1 statement after recovery, but has %d", len(ifStmt.Consequence.Statements))
	}
	consequenceStmt, ok := ifStmt.Consequence.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Consequence statement is not *ast.AssignmentStatement. got=%T", ifStmt.Consequence.Statements[0])
	}
	testIdentifier(t, consequenceStmt.Left, "x")
	testIntegerLiteral(t, consequenceStmt.Value, 1)

	// 6. Verify the alternative (ELSE) was parsed.
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
			"expected next token to be DO, got IDENT instead at row 2, column 5",
			1,
		},
		{
			"Missing DO in WHILE loop",
			`WHILE x < 10
				x := x + 1;
			END_WHILE`,
			"expected next token to be DO, got IDENT instead at row 2, column 5",
			1,
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
		t.Fatalf("Expected parser to have 1 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "missing 'END_IF' for IF statement starting at row 2"
	if !strings.HasPrefix(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the IF and the subsequent assignment
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}
	// The parser should parse the IF statement (partially) and then the next statement.
	_, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Errorf("First statement should be an IfStatement after recovery.")
	}
}

func TestMissingEndVarErrorRecovery(t *testing.T) {
	input := `
		VAR
			myVar : INT;
			myOtherVar : BOOL;
		// Missing END_VAR

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

	expectedError := "expected next token to be END_VAR, got IF instead"
	assertErrorContains(t, p.Errors(), expectedError)

	// Check that the parser recovered and parsed both statements
	if len(program.Statements) != 2 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	// Check first statement (VAR block)
	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Errorf("First statement should be a VarBlockDeclaration. got=%T", program.Statements[0])
	} else if len(varBlock.Declarations) != 2 {
		t.Errorf("Expected VAR block to have 2 declarations. got=%d", len(varBlock.Declarations))
	}

	// Check second statement (IF block)
	_, ok = program.Statements[1].(*ast.IfStatement)
	if !ok {
		t.Errorf("Second statement should be an IfStatement. got=%T", program.Statements[1])
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

	expectedError := "expected next token to be END_FUNCTION_BLOCK, got EOF instead at row 12, column 2"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the FUNCTION_BLOCK and the subsequent VAR block
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 1 statements to be parsed. got=%d", len(program.Statements))
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

	expectedError := "expected next token to be END_PROGRAM, got EOF instead at row 12, column 2"
	if !strings.Contains(p.Errors()[0], expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered and parsed both the PROGRAM and the subsequent VAR block
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 1 statements to be parsed. got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ProgramDeclaration); !ok {
		t.Errorf("First statement should be a ProgramDeclaration after recovery.")
	}
}

func TestDirectVariableParsing(t *testing.T) {
	input := `
		VAR
			myInput AT %IX0.1 : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestDirectVariableParsing", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(varBlock.Declarations) != 1 {
		t.Fatalf("Expected 1 declaration. got=%d", len(varBlock.Declarations))
	}

	decl := varBlock.Declarations[0]
	if decl.Location == nil {
		t.Fatalf("Expected AT clause to be parsed, but it was nil.")
	}

	// No type assertions are needed, as the compiler guarantees the concrete types
	// for both decl.Location and atDecl.Location from the struct definitions.
	atDecl := decl.Location
	directVar := atDecl.Location
	if directVar == nil {
		t.Fatalf("atDecl.Location is nil, expected *ast.DirectVariable")
	}

	if directVar.Address != "IX0.1" { // cspell:disable-line
		t.Errorf("DirectVariable address is not 'IX0.1'. got=%q", directVar.Address) // cspell:disable-line
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
	case float64:
		return testRealLiteral(t, exp, v)
	case string:
		return testIdentifier(t, exp, v)
	case bool:
		return testBooleanLiteral(t, exp, v)
	}
	t.Errorf("type of exp not handled. got=%T", exp)
	return false
}

func testAssignmentStatement(t *testing.T, stmt ast.Statement, left, rightValueString string) bool {
	t.Helper()
	assignStmt, ok := stmt.(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("stmt not *ast.AssignmentStatement. got=%T", stmt)
		return false
	}
	if assignStmt.Left.String() != left {
		t.Errorf("assignStmt.Left.String() not %q. got=%q", left, assignStmt.Left.String())
		return false
	}
	if assignStmt.Value.String() != rightValueString {
		t.Errorf("assignStmt.Value.String() not %q. got=%q", rightValueString, assignStmt.Value.String())
		return false
	}
	return true
}

func testIntegerLiteral(t *testing.T, il ast.Expression, value int64) bool {
	integ, ok := il.(*ast.IntegerLiteral)
	if !ok {
		// Handle the case where a negative number is parsed as a prefix expression
		if prefix, isPrefix := il.(*ast.PrefixExpression); isPrefix && prefix.Operator == "-" {
			// Recursively call with the right-hand side and negated value
			return testIntegerLiteral(t, prefix.Right, -value)
		}

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

func testTypedLiteral(t *testing.T, exp ast.Expression, typeName string, value string) bool {
	t.Helper()
	typedLit, ok := exp.(*ast.TypedLiteral)
	if !ok {
		t.Errorf("exp not *ast.TypedLiteral. got=%T", exp)
		return false
	}

	if typedLit.TypeName != typeName {
		t.Errorf("TypedLiteral.TypeName not %q. got=%q", typeName, typedLit.TypeName)
		return false
	}

	valueIdent, ok := typedLit.Value.(*ast.Identifier)
	if !ok {
		t.Errorf("TypedLiteral.Value not *ast.Identifier. got=%T", typedLit.Value)
		return false
	}

	return testIdentifier(t, valueIdent, value)
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
	t.Helper()
	expected := fmt.Sprintf("%T", expectedType)
	result := fmt.Sprintf("%T", actual)
	return expected == result
}

func assertErrorContains(t *testing.T, errors []string, expected string, index ...int) {
	t.Helper()
	if len(index) > 0 {
		idx := index[0]
		if idx >= len(errors) {
			t.Errorf("Error index %d out of bounds. Only %d errors reported: %v", idx, len(errors), errors)
			return
		}
		if !strings.Contains(errors[index[0]], expected) {
			t.Errorf("Expected error at index %d to contain %q, got %q", index[0], expected, errors[index[0]])
		}
		return
	}
	// If no index is provided, search all errors for the expected string.
	for _, err := range errors {
		if strings.Contains(err, expected) {
			return // Found it
		}
	}
	t.Errorf("Expected to find error %q in parser errors, but did not. Errors: %v", expected, errors)
}
