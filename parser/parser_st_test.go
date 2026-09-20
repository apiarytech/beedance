package parser

import (
	"fmt"
	"testing"

	"beedance/ast"
	"beedance/lexer"
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
		{"VAR myHexVar : INT := 16#FF; END_VAR", "myHexVar", "INT", int64(255), false, false, false},
		{"VAR myUintVar : UINT := 16#A; END_VAR", "myUintVar", "UINT", uint64(10), false, false, false},
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestTypeDeclarations", input)

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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStructTypeDeclaration", input)

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

func TestStructMemberParsing(t *testing.T) {
	tests := []struct {
		name          string
		input         string // Just the content of the STRUCT block
		expectedError string
		check         func(t *testing.T, def *ast.StructDefinition)
	}{
		{
			name:  "Valid member with initial value",
			input: `Field1 : INT := 10;`,
			check: func(t *testing.T, def *ast.StructDefinition) {
				if len(def.Members) != 1 {
					t.Fatalf("Expected 1 member, got %d", len(def.Members))
				}
				member1 := def.Members[0]
				if !testVarDeclStatement(t, member1, "Field1", "INT") || !testLiteralExpression(t, member1.Value, 10) {
					t.Error("Member 'Field1 : INT := 10;' not parsed correctly")
				}
			},
		},
		{
			name:  "Valid member without initial value",
			input: `Field1 : BOOL;`,
			check: func(t *testing.T, def *ast.StructDefinition) {
				if len(def.Members) != 1 {
					t.Fatalf("Expected 1 member, got %d", len(def.Members))
				}
				member1 := def.Members[0]
				if !testVarDeclStatement(t, member1, "Field1", "BOOL") {
					t.Error("Member 'Field1 : BOOL;' not parsed correctly")
				}
				if member1.Value != nil {
					t.Errorf("Expected nil initial value, got %v", member1.Value)
				}
			},
		},
		{
			name:          "Error on missing member name",
			input:         `: INT;`,
			expectedError: "expected member name (identifier), got :",
		},
		{
			name:          "Error on missing colon",
			input:         `Field1 INT;`,
			expectedError: "expected next token to be :, got INT instead",
		},
		{
			name:          "Error on invalid data type",
			input:         `Field1 : := 5;`,
			expectedError: "expected a data type, got :=",
		},
		{
			name:          "Recovery on missing semicolon",
			input:         `Field1 : INT`, // Missing semicolon before END_STRUCT
			expectedError: "expected next token to be ;, got END_STRUCT instead",
			check: func(t *testing.T, def *ast.StructDefinition) {
				// Check that the member was still parsed despite the missing semicolon
				if len(def.Members) != 1 {
					t.Fatalf("Expected 1 member after recovery, got %d", len(def.Members))
				}
				if !testVarDeclStatement(t, def.Members[0], "Field1", "INT") {
					t.Error("Member not parsed correctly after semicolon recovery")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullInput := fmt.Sprintf("TYPE MyStruct : STRUCT %s END_STRUCT; END_TYPE", tt.input)
			l := lexer.New(fullInput)
			p := New(l)
			program := p.ParseProgram()

			if tt.expectedError != "" {
				assertErrorContains(t, p.Errors(), tt.expectedError)
			} else {
				checkParserErrors(t, p, tt.name, fullInput)
			}

			if tt.check != nil {
				if len(program.Statements) == 0 {
					t.Fatal("No statements parsed")
				}
				typeBlock, ok := program.Statements[0].(*ast.TypeBlockDeclaration)
				if !ok {
					t.Fatalf("Expected ast.TypeBlockDeclaration, got %T", program.Statements[0])
				}
				if len(typeBlock.Declarations) == 0 {
					t.Fatal("No type declarations parsed in block")
				}
				typeDecl := typeBlock.Declarations[0]
				structDef, ok := typeDecl.DataType.(*ast.StructDefinition)
				if !ok {
					t.Fatalf("Expected ast.StructDefinition, got %T", typeDecl.DataType)
				}
				tt.check(t, structDef)
			}
		})
	}
}

func TestFunctionLiteralParsing(t *testing.T) {
	// This test covers the non-standard 'fn' keyword for anonymous functions,
	// which is a remnant from a previous language version and handled as a special case.
	input := `fn(x : INT, y : BOOL) : REAL { RETURN x > 0; };`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionLiteralParsing", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement is not ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	function, ok := stmt.Expression.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.FunctionLiteral. got=%T", stmt.Expression)
	}

	if len(function.Parameters) != 2 {
		t.Fatalf("function literal parameters wrong. want 2, got=%d", len(function.Parameters))
	}

	if function.Parameters[0].Name.Value != "x" || function.Parameters[0].DataType.String() != "INT" {
		t.Errorf("parameter 0 is not 'x : INT'. got=%s", function.Parameters[0].String())
	}
	if function.Parameters[1].Name.Value != "y" || function.Parameters[1].DataType.String() != "BOOL" {
		t.Errorf("parameter 1 is not 'y : BOOL'. got=%s", function.Parameters[1].String())
	}

	if function.ReturnType.String() != "REAL" {
		t.Errorf("function return type is not 'REAL'. got=%q", function.ReturnType.String())
	}

	if len(function.Body.Statements) != 1 {
		t.Fatalf("function.Body.Statements has not 1 statements. got=%d", len(function.Body.Statements))
	}

	if _, ok := function.Body.Statements[0].(*ast.ReturnStatement); !ok {
		t.Fatalf("function body stmt is not ast.ReturnStatement. got=%T", function.Body.Statements[0])
	}
}

func TestFunctionDeclarationWithAllVarBlocks(t *testing.T) {
	input := `
		FUNCTION MyFunc : INT
			VAR_INPUT
				in_var : INT;
			END_VAR
			VAR_OUTPUT
				out_var : BOOL;
			END_VAR
			VAR_IN_OUT
				inout_var : REAL;
			END_VAR
			VAR
				local_var : DINT;
			END_VAR
			MyFunc := 1;
		END_FUNCTION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionDeclarationWithAllVarBlocks", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fn, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionDeclaration. got=%T", program.Statements[0])
	}

	if len(fn.VarInputs) != 1 {
		t.Errorf("Expected 1 VAR_INPUT, got %d", len(fn.VarInputs))
	}
	testVarDeclStatement(t, fn.VarInputs[0], "in_var", "INT")

	if len(fn.VarOutputs) != 1 {
		t.Errorf("Expected 1 VAR_OUTPUT, got %d", len(fn.VarOutputs))
	}
	testVarDeclStatement(t, fn.VarOutputs[0], "out_var", "BOOL")

	if len(fn.VarInOuts) != 1 {
		t.Errorf("Expected 1 VAR_IN_OUT, got %d", len(fn.VarInOuts))
	}
	testVarDeclStatement(t, fn.VarInOuts[0], "inout_var", "REAL")

	if len(fn.Vars) != 1 {
		t.Errorf("Expected 1 VAR, got %d", len(fn.Vars))
	}
	testVarDeclStatement(t, fn.Vars[0], "local_var", "DINT")
}

func TestInvalidVarBlocksInFunction(t *testing.T) {
	tests := []struct {
		varBlockType  string
		expectedError string
	}{
		{"VAR_EXTERNAL", "VAR_EXTERNAL declarations are not allowed in a FUNCTION"},
		{"VAR_GLOBAL", "VAR_GLOBAL declarations are not allowed in a FUNCTION"},
		{"VAR_ACCESS", "VAR_ACCESS declarations are not allowed in a FUNCTION"},
		{"VAR_TEMP", "VAR_TEMP declarations are not allowed in a FUNCTION"},
	}

	for _, tt := range tests {
		t.Run(tt.varBlockType, func(t *testing.T) {
			input := fmt.Sprintf(`
				FUNCTION MyFunc : INT
					%s
						myVar : INT;
					END_VAR
					MyFunc := 1;
				END_FUNCTION`, tt.varBlockType)

			l := lexer.New(input)
			p := New(l)
			p.ParseProgram()

			if len(p.Errors()) == 0 {
				t.Fatalf("Expected an error for %s in FUNCTION, but got none", tt.varBlockType)
			}
			assertErrorContains(t, p.Errors(), tt.expectedError)
		})
	}
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestComplexTypeBlockDeclaration", input)

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
	program := p.ParseProgram()
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
	program := p.ParseProgram()
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

	testVarDeclStatement(t, stmt, "myArr", "MyIntArray")
}

func TestVarDeclWithSubrange(t *testing.T) {
	input := `
		VAR
			myLimitedInt : INT (0..100);
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestVarDeclWithSubrange", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(varBlock.Declarations) != 1 {
		t.Fatalf("varBlock.Declarations does not contain 1 statement. got=%d", len(varBlock.Declarations))
	}
	stmt := varBlock.Declarations[0]

	testVarDeclStatement(t, stmt, "myLimitedInt", "INT")

	if stmt.Subrange == nil {
		t.Fatal("stmt.Subrange is nil, expected a subrange expression")
	}

	if !testInfixExpression(t, 0, stmt.Subrange, 0, "..", 100) {
		t.Error("Invalid subrange expression.")
	}
}

func TestVarInputEdgeQualifiers(t *testing.T) {
	tests := []struct {
		name        string
		blockType   string
		input       string // Just the content of a VAR_INPUT block
		isRising    bool
		isFalling   bool
		expectedErr string
	}{
		{
			name:      "R_EDGE qualifier",
			blockType: "VAR_INPUT",
			input:     `myTrigger : BOOL;`,
			isRising:  true,
		},
		{
			name:      "F_EDGE qualifier",
			blockType: "VAR_INPUT",
			input:     `myFallingEdge : BOOL;`,
			isFalling: true,
		},
		{
			name:        "R_EDGE on non-BOOL is invalid",
			blockType:   "VAR_INPUT",
			input:       `myInvalidTrigger : INT;`,
			isRising:    true,
			expectedErr: "R_EDGE and F_EDGE qualifiers can only be applied to BOOL variables",
		},
		{
			name:        "R_EDGE in wrong block type (VAR)",
			blockType:   "VAR",
			input:       `myTrigger : BOOL;`,
			isRising:    true,
			expectedErr: "R_EDGE and F_EDGE qualifiers can only be used in VAR_INPUT blocks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qualifier := ""
			if tt.isRising {
				qualifier = "R_EDGE"
			}
			if tt.isFalling {
				qualifier = "F_EDGE"
			}
			fullInput := fmt.Sprintf("FUNCTION_BLOCK TestFB\n%s %s\n%s\nEND_VAR\nEND_FUNCTION_BLOCK", tt.blockType, qualifier, tt.input)
			l := lexer.New(fullInput)
			p := New(l)
			p.ParseProgram()

			assertErrorContains(t, p.Errors(), tt.expectedErr)
		})
	}
}

func TestTypeDeclWithStringLength(t *testing.T) {
	input := `
		TYPE
			MyShortString : STRING(10);
			MyLongWString : WSTRING(255) := "default";
		END_TYPE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestTypeDeclWithStringLength", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	typeBlock, ok := program.Statements[0].(*ast.TypeBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TypeBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(typeBlock.Declarations) != 2 {
		t.Fatalf("Expected 2 type declarations. got=%d", len(typeBlock.Declarations))
	}

	// Check MyShortString : STRING(10);
	decl1 := typeBlock.Declarations[0]
	if decl1.Name.Value != "MyShortString" {
		t.Errorf("Invalid name for declaration 1. got=%s", decl1.Name.Value)
	}
	if decl1.DataType.String() != "STRING" {
		t.Errorf("Invalid data type for declaration 1. got=%s", decl1.DataType.String())
	}
	if decl1.StringLength == nil || !testIntegerLiteral(t, decl1.StringLength, 10) {
		t.Errorf("Invalid string length for declaration 1.")
	}
	if decl1.Subrange != nil {
		t.Errorf("Subrange should be nil for string length declaration.")
	}

	// Check MyLongWString : WSTRING(255) := "default";
	decl2 := typeBlock.Declarations[1]
	if decl2.Name.Value != "MyLongWString" || decl2.DataType.String() != "WSTRING" {
		t.Errorf("Invalid name or data type for declaration 2. got=%s : %s", decl2.Name.Value, decl2.DataType.String())
	}
	if decl2.StringLength == nil || !testIntegerLiteral(t, decl2.StringLength, 255) {
		t.Errorf("Invalid string length for declaration 2.")
	}
	if decl2.InitialValue == nil || !testStringLiteral(t, decl2.InitialValue, "default") {
		t.Errorf("Invalid initial value for declaration 2.")
	}
}

func TestVarDeclWithStringLength(t *testing.T) {
	input := `
		VAR
			shortString : STRING(10);
			longString : WSTRING(255) := 'some initial value';
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestVarDeclWithStringLength", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
	}

	if len(varBlock.Declarations) != 2 {
		t.Fatalf("varBlock.Declarations does not contain 2 statements. got=%d", len(varBlock.Declarations))
	}

	// Check first declaration
	stmt1 := varBlock.Declarations[0]
	testVarDeclStatement(t, stmt1, "shortString", "STRING")

	if stmt1.StringLength == nil {
		t.Fatal("stmt1.StringLength is nil, expected a length expression")
	}
	if !testIntegerLiteral(t, stmt1.StringLength, 10) {
		t.Error("Invalid string length for shortString.")
	}
	if stmt1.Subrange != nil {
		t.Error("stmt1.Subrange should be nil for a string length declaration")
	}

	// Check second declaration
	stmt2 := varBlock.Declarations[1]
	testVarDeclStatement(t, stmt2, "longString", "WSTRING")

	if stmt2.StringLength == nil {
		t.Fatal("stmt2.StringLength is nil, expected a length expression")
	}
	if !testIntegerLiteral(t, stmt2.StringLength, 255) {
		t.Error("Invalid string length for longString.")
	}
	if !testStringLiteral(t, stmt2.Value, "some initial value") {
		t.Error("Invalid initial value for longString.")
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestVarDeclWithUserDefinedArrayType", input)

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

func TestVarAccessDeclarations1(t *testing.T) {
	tests := []struct {
		name          string
		input         string // Just the content of the VAR_ACCESS block
		expectedError string
		check         func(t *testing.T, decls []*ast.VarDeclStatement)
	}{
		{
			name:  "Valid symbolic path",
			input: `MyAlias : MyProgram.MyVar : INT;`,
			check: func(t *testing.T, decls []*ast.VarDeclStatement) {
				if len(decls) != 1 {
					t.Fatalf("Expected 1 declaration, got %d", len(decls))
				}
				decl := decls[0]
				if decl.Name.Value != "MyAlias" {
					t.Errorf("Name not 'MyAlias', got %s", decl.Name.Value)
				}
				if _, ok := decl.AccessPath.(*ast.MemberAccessExpression); !ok {
					t.Errorf("AccessPath is not MemberAccessExpression, got %T", decl.AccessPath)
				}
				if decl.DataType.String() != "INT" {
					t.Errorf("DataType not 'INT', got %s", decl.DataType.String())
				}
			},
		},
		{
			name:  "Valid direct variable path",
			input: `MyAlias : %IX1.0 : BOOL;`,
			check: func(t *testing.T, decls []*ast.VarDeclStatement) {
				if len(decls) != 1 {
					t.Fatalf("Expected 1 declaration, got %d", len(decls))
				}
				decl := decls[0]
				if decl.Name.Value != "MyAlias" {
					t.Errorf("Name not 'MyAlias', got %s", decl.Name.Value)
				}
				dv, ok := decl.AccessPath.(*ast.DirectVariable)
				if !ok {
					t.Errorf("AccessPath is not DirectVariable, got %T", decl.AccessPath)
				} else if dv.Address != "IX1.0" {
					t.Errorf("Address not 'IX1.0', got %s", dv.Address)
				}
				if decl.DataType.String() != "BOOL" {
					t.Errorf("DataType not 'BOOL', got %s", decl.DataType.String())
				}
			},
		},
		{
			name:          "Invalid AT with direct variable",
			input:         `HMI_Sensor AT %IW0 : INT;`,
			expectedError: "expected next token to be :, got AT instead",
		},
		{
			name:          "Invalid AT with symbolic path",
			input:         `HMI_Sensor AT MainInstance.PressureSensor : INT;`,
			expectedError: "expected next token to be :, got AT instead",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullInput := fmt.Sprintf("VAR_ACCESS\n%s\nEND_VAR", tt.input)
			l := lexer.New(fullInput)
			p := New(l)
			program := p.ParseProgram()

			if tt.expectedError != "" {
				assertErrorContains(t, p.Errors(), tt.expectedError)
			} else {
				checkParserErrors(t, p, tt.name, fullInput)
			}

			if tt.check != nil {
				if len(program.Statements) == 0 {
					t.Fatal("No statements parsed")
				}
				stmt, ok := program.Statements[0].(*ast.AccessVarDeclaration)
				if !ok {
					t.Fatalf("Expected ast.AccessVarDeclaration, got %T", program.Statements[0])
				}
				tt.check(t, stmt.Vars)
			}
		})
	}
}

func TestExternalVarDeclarations2(t *testing.T) {
	input := `
		VAR_ACCESS
			ExternalName1 : ResourceName.ProgInstance.InternalVar1 : INT READ_WRITE;
			ExternalName2 : ResourceName.ProgInstance.InternalVar2 : REAL READ_ONLY;
			SimpleAccess  : GlobalVar : BOOL;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestExternalVarDeclarations2", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.AccessVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.AccessVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Vars) != 3 {
		t.Fatalf("Expected 3 declarations in VAR_ACCESS block. got=%d", len(stmt.Vars))
	}

	// --- Check Declaration 1 ---
	decl1 := stmt.Vars[0]
	if decl1.Name.Value != "ExternalName1" {
		t.Errorf("decl1.Name not 'ExternalName1'. got=%s", decl1.Name.Value)
	}
	if decl1.AccessPath.String() != "ResourceName.ProgInstance.InternalVar1" {
		t.Errorf("decl1.AccessPath not correct. got=%s", decl1.AccessPath.String())
	}
	if decl1.DataType.String() != "INT" {
		t.Errorf("decl1.DataType not 'INT'. got=%s", decl1.DataType.String())
	}
	if decl1.AccessType != "READ_WRITE" {
		t.Errorf("decl1.AccessType not 'READ_WRITE'. got=%s", decl1.AccessType)
	}

	// --- Check Declaration 2 ---
	decl2 := stmt.Vars[1]
	if decl2.Name.Value != "ExternalName2" {
		t.Errorf("decl2.Name not 'ExternalName2'. got=%s", decl2.Name.Value)
	}
	if decl2.AccessPath.String() != "ResourceName.ProgInstance.InternalVar2" {
		t.Errorf("decl2.AccessPath not correct. got=%s", decl2.AccessPath.String())
	}
	if decl2.DataType.String() != "REAL" {
		t.Errorf("decl2.DataType not 'REAL'. got=%s", decl2.DataType.String())
	}
	if decl2.AccessType != "READ_ONLY" {
		t.Errorf("decl2.AccessType not 'READ_ONLY'. got=%s", decl2.AccessType)
	}

	// --- Check Declaration 3 ---
	decl3 := stmt.Vars[2]
	if decl3.Name.Value != "SimpleAccess" {
		t.Errorf("decl3.Name not 'SimpleAccess'. got=%s", decl3.Name.Value)
	}
	if decl3.AccessPath.String() != "GlobalVar" {
		t.Errorf("decl3.AccessPath not 'GlobalVar'. got=%s", decl3.AccessPath.String())
	}
	if decl3.DataType.String() != "BOOL" {
		t.Errorf("decl3.DataType not 'BOOL'. got=%s", decl3.DataType.String())
	}
	if decl3.AccessType != "" {
		t.Errorf("decl3.AccessType not empty. got=%s", decl3.AccessType)
	}
}

func TestExternalVarDeclarations3(t *testing.T) {
	input := `
		VAR_EXTERNAL CONSTANT
			External1 : INT := 1;
			External2 : BOOL := TRUE;
		END_VAR
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestExternalVarDeclarations3", input)

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

func TestNestedNamespaceDeclaration(t *testing.T) {
	input := `
		NAMESPACE MyCompany.MyLibrary
			FUNCTION MyFunc : INT
				MyFunc := 1;
			END_FUNCTION
		END_NAMESPACE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestNestedNamespaceDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	ns, ok := program.Statements[0].(*ast.NamespaceDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.NamespaceDeclaration. got=%T", program.Statements[0])
	}

	// Check the name
	if ns.Name.String() != "MyCompany.MyLibrary" {
		t.Errorf("namespace name is not 'MyCompany.MyLibrary'. got=%s", ns.Name.String())
	}

	// Check the structure of the name expression
	_, ok = ns.Name.(*ast.MemberAccessExpression)
	if !ok {
		t.Fatalf("namespace name is not a MemberAccessExpression. got=%T", ns.Name)
	}

	if len(ns.Statements) != 1 {
		t.Fatalf("namespace should contain 1 statement. got=%d", len(ns.Statements))
	}
}

func TestNamespaceDeclaration(t *testing.T) {
	input := `
		NAMESPACE MyLibrary
			// A function inside the namespace
			FUNCTION MyFunc : INT
				MyFunc := 1;
			END_FUNCTION

			TYPE MyStruct : STRUCT Field : BOOL; END_STRUCT; END_TYPE
		END_NAMESPACE
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestNamespaceDeclaration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	ns, ok := program.Statements[0].(*ast.NamespaceDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.NamespaceDeclaration. got=%T", program.Statements[0])
	}

	if ns.Name.String() != "MyLibrary" {
		t.Errorf("namespace name is not 'MyLibrary'. got=%s", ns.Name.String())
	}

	if len(ns.Statements) != 2 {
		t.Fatalf("namespace should contain 2 statements. got=%d", len(ns.Statements))
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestMixedExternalVarDeclarations", input)

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

			// The parser now creates specific literal types for time/date, so we handle them here.
			switch tt.expectedType {
			case "T", "TIME":
				lit, ok := stmt.Expression.(*ast.TimeLiteral)
				if !ok {
					t.Fatalf("stmt.Expression is not ast.TimeLiteral. got=%T for input %q", stmt.Expression, tt.input)
				}
				if lit.Value != tt.expectedVal {
					t.Errorf("TimeLiteral value not %q. got=%q", tt.expectedVal, lit.Value)
				}
			case "D", "DATE":
				lit, ok := stmt.Expression.(*ast.DateLiteral)
				if !ok {
					t.Fatalf("stmt.Expression is not ast.DateLiteral. got=%T for input %q", stmt.Expression, tt.input)
				}
				if lit.Value != tt.expectedVal {
					t.Errorf("DateLiteral value not %q. got=%q", tt.expectedVal, lit.Value)
				}
			case "TOD", "TIME_OF_DAY":
				lit, ok := stmt.Expression.(*ast.TimeOfDayLiteral)
				if !ok {
					t.Fatalf("stmt.Expression is not ast.TimeOfDayLiteral. got=%T for input %q", stmt.Expression, tt.input)
				}
				if lit.Value != tt.expectedVal {
					t.Errorf("TimeOfDayLiteral value not %q. got=%q", tt.expectedVal, lit.Value)
				}
			case "DT", "DATE_AND_TIME":
				lit, ok := stmt.Expression.(*ast.DateAndTimeLiteral)
				if !ok {
					t.Fatalf("stmt.Expression is not ast.DateAndTimeLiteral. got=%T for input %q", stmt.Expression, tt.input)
				}
				if lit.Value != tt.expectedVal {
					t.Errorf("DateAndTimeLiteral value not %q. got=%q", tt.expectedVal, lit.Value)
				}
			default:
				t.Fatalf("unhandled expected type in test table: %s", tt.expectedType)
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
		{
			"UINT#16#FF;",
			"UINT#16#FF;",
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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIfStatementWithEmptyBlocks", input)

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
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionWithMultipleVarBlocks", input)

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
		wrapper := pouWrappers[0] // Use PROGRAM for predictability in this refactored test

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

func TestIfWithNestedBlock(t *testing.T) {
	input := `
		IF x > 0 THEN
			FOR i := 1 TO 5 DO
				x := x - 1;
			END_FOR
		END_IF
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIfWithNestedBlock", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	ifStmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T", program.Statements[0])
	}

	if len(ifStmt.Consequence.Statements) != 1 {
		t.Fatalf("IF consequence should have 1 statement. got=%d", len(ifStmt.Consequence.Statements))
	}

	_, ok = ifStmt.Consequence.Statements[0].(*ast.ForLoopStatement)
	if !ok {
		t.Fatalf("Statement in IF consequence is not ast.ForLoopStatement. got=%T", ifStmt.Consequence.Statements[0])
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

func TestForLoopErrorRecovery(t *testing.T) {
	input := `
		FOR i := 1 10 DO // Missing TO
			x := x + 1;
		END_FOR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("Expected parser to have 1 error, but it had %d: %v", len(p.Errors()), p.Errors())
	}

	expectedError := "expected next token to be TO, got INT instead"
	assertErrorContains(t, p.Errors(), expectedError)

	// Check that the parser recovered and still parsed a FOR loop structure
	if len(program.Statements) != 1 {
		t.Fatalf("Parser did not recover, expected 1 statement. got=%d", len(program.Statements))
	}
	_, ok := program.Statements[0].(*ast.ForLoopStatement)
	if !ok {
		t.Fatalf("Statement is not ForLoopStatement. got=%T", program.Statements[0])
	}
}

func TestWhileStatement(t *testing.T) {
	input := `
		WHILE x < 10 DO
			x := x + 1;
		END_WHILE
		`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestComplexTypeBlockDeclaration", input)

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
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, stmt *ast.RepeatStatement)
	}{
		{
			name: "Simple REPEAT loop",
			input: `
				REPEAT
					x := x + 1;
				UNTIL x > 10 END_REPEAT`,
			check: func(t *testing.T, stmt *ast.RepeatStatement) {
				if !testInfixExpression(t, 0, stmt.Condition, "x", ">", 10) {
					return
				}
				if len(stmt.Body.Statements) != 1 {
					t.Fatalf("repeat loop body does not contain 1 statement. got=%d", len(stmt.Body.Statements))
				}
				testAssignmentStatement(t, stmt.Body.Statements[0], "x", "(x + 1)")
			},
		},
		{
			name: "REPEAT loop with multiple statements and comments",
			input: `
				REPEAT
					x := x + 1; (* increment x *)
					y := y - 1; // decrement y
				UNTIL x > y END_REPEAT`,
			check: func(t *testing.T, stmt *ast.RepeatStatement) {
				if !testInfixExpression(t, 0, stmt.Condition, "x", ">", "y") {
					return
				}
				if len(stmt.Body.Statements) != 2 {
					t.Fatalf("repeat loop body does not contain 2 statements. got=%d", len(stmt.Body.Statements))
				}
				testAssignmentStatement(t, stmt.Body.Statements[0], "x", "(x + 1)")
				testAssignmentStatement(t, stmt.Body.Statements[1], "y", "(y - 1)")
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

			stmt, ok := program.Statements[0].(*ast.RepeatStatement)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.RepeatStatement. got=%T", program.Statements[0])
			}

			tt.check(t, stmt)
		})
	}
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
	program := p.ParseProgram()
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

func TestExitStatement(t *testing.T) {
	input := `EXIT;`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
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
	if !testErrorContains(p.Errors(), expectedError) {
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
	if !testErrorContains(p.Errors(), expectedError) {
		t.Errorf("Expected error message to contain %q, got %q", expectedError, p.Errors()[0])
	}

	// Check that the parser recovered. With a missing END_IF, a reasonable
	// recovery strategy is to consume subsequent statements as part of the IF
	// body until EOF or another block-ending keyword is found.
	if len(program.Statements) != 1 {
		t.Fatalf("Parser should parse the input as a single IF statement. got=%d statements", len(program.Statements))
	}
	ifStmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("First statement should be an IfStatement after recovery. got=%T", program.Statements[0])
	}
	// The recovery should result in the second assignment being part of the IF's consequence.
	if len(ifStmt.Consequence.Statements) != 2 {
		t.Errorf("Expected IF consequence to contain 2 statements after recovery. got=%d", len(ifStmt.Consequence.Statements))
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

	assertErrorContains(t, p.Errors(), "expected next token to be END_FUNCTION_BLOCK, got VAR instead at row 9, column 3")

	// Check that the parser recovered and parsed both the FUNCTION_BLOCK and the subsequent VAR block
	if len(program.Statements) != 2 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Errorf("First statement should be a FunctionBlockDeclaration after recovery.")
	}

	_, ok = program.Statements[1].(*ast.VarBlockDeclaration)
	if !ok {
		t.Errorf("Second statement should be a VarBlockDeclaration after recovery.")
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

	assertErrorContains(t, p.Errors(), "expected next token to be END_PROGRAM, got VAR instead at row 9, column 3")

	// Check that the parser recovered and parsed both the PROGRAM and the subsequent VAR block
	if len(program.Statements) != 2 {
		t.Fatalf("Parser did not recover, expected 2 statements to be parsed. got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ProgramDeclaration); !ok {
		t.Errorf("First statement should be a ProgramDeclaration after recovery.")
	}

	if _, ok := program.Statements[1].(*ast.VarBlockDeclaration); !ok {
		t.Errorf("Second statement should be a VarBlockDeclaration after recovery.")
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

func TestFunctionBlockVarBlockTypes(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
		check         func(t *testing.T, stmt *ast.FunctionBlockDeclaration)
	}{
		{
			name: "VAR_TEMP in FUNCTION_BLOCK",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_TEMP
						tempVar : INT;
					END_VAR
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, stmt *ast.FunctionBlockDeclaration) {
				if len(stmt.VarTemp) != 1 {
					t.Fatalf("Expected 1 VAR_TEMP block, got %d", len(stmt.VarTemp))
				}
				if len(stmt.VarTemp[0].Vars) != 1 {
					t.Fatalf("Expected 1 var in VAR_TEMP block, got %d", len(stmt.VarTemp[0].Vars))
				}
				testVarDeclStatement(t, stmt.VarTemp[0].Vars[0], "tempVar", "INT")
			},
		},
		{
			name: "VAR_EXTERNAL in FUNCTION_BLOCK",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_EXTERNAL
						extVar : BOOL;
					END_VAR
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, stmt *ast.FunctionBlockDeclaration) {
				if len(stmt.VarExternal) != 1 {
					t.Fatalf("Expected 1 VAR_EXTERNAL block, got %d", len(stmt.VarExternal))
				}
				if len(stmt.VarExternal[0].Vars) != 1 {
					t.Fatalf("Expected 1 var in VAR_EXTERNAL block, got %d", len(stmt.VarExternal[0].Vars))
				}
				testVarDeclStatement(t, stmt.VarExternal[0].Vars[0], "extVar", "BOOL")
			},
		},
		{
			name: "VAR_TEMP with multiple variables",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_TEMP
						temp1 : INT;
						temp2 : REAL;
					END_VAR
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, stmt *ast.FunctionBlockDeclaration) {
				if len(stmt.VarTemp) != 1 {
					t.Fatalf("Expected 1 VAR_TEMP block, got %d", len(stmt.VarTemp))
				}
				if len(stmt.VarTemp[0].Vars) != 2 {
					t.Fatalf("Expected 2 vars in VAR_TEMP block, got %d", len(stmt.VarTemp[0].Vars))
				}
				testVarDeclStatement(t, stmt.VarTemp[0].Vars[0], "temp1", "INT")
				testVarDeclStatement(t, stmt.VarTemp[0].Vars[1], "temp2", "REAL")
			},
		},
		{
			name: "VAR_TEMP missing END_VAR",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_TEMP
						tempVar : INT;
					(* Missing END_VAR *)
					tempVar := 1;
				END_FUNCTION_BLOCK
			`,
			// The parser recovers by seeing the assignment as the start of the body.
			// It reports that END_VAR was expected before the identifier 'tempVar'.
			expectedError: "expected next token to be END_VAR, got IDENT instead",
		},
		{
			name: "Multiple VAR_TEMP blocks in FUNCTION_BLOCK",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_TEMP
						tempVar1 : INT;
					END_VAR
					VAR_TEMP
						tempVar2 : BOOL;
					END_VAR
				END_FUNCTION_BLOCK
			`,
			check: func(t *testing.T, stmt *ast.FunctionBlockDeclaration) {
				if len(stmt.VarTemp) != 2 {
					t.Fatalf("Expected 2 VAR_TEMP blocks, got %d", len(stmt.VarTemp))
				}
				// Check first block
				testVarDeclStatement(t, stmt.VarTemp[0].Vars[0], "tempVar1", "INT")
				// Check second block
				testVarDeclStatement(t, stmt.VarTemp[1].Vars[0], "tempVar2", "BOOL")
			},
		},
		{
			name: "VAR_ACCESS in FUNCTION_BLOCK (error)",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_ACCESS
						accVar : OtherFB READ_ONLY;
					END_VAR
				END_FUNCTION_BLOCK
			`,
			expectedError: "VAR_ACCESS declarations are not allowed in a FUNCTION_BLOCK",
		},
		{
			name: "VAR_GLOBAL in FUNCTION_BLOCK (error)",
			input: `
				FUNCTION_BLOCK MyFB
					VAR_GLOBAL
						gVar : REAL;
					END_VAR
				END_FUNCTION_BLOCK
			`,
			expectedError: "VAR_GLOBAL declarations are not allowed in a FUNCTION_BLOCK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()

			if tt.expectedError != "" {
				assertErrorContains(t, p.Errors(), tt.expectedError)
				return
			}

			checkParserErrors(t, p, tt.name, tt.input)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
			}

			if tt.check != nil {
				tt.check(t, stmt)
			}
		})
	}
}

func TestParseVarDeclarationsComprehensive(t *testing.T) {
	tests := []struct {
		name          string
		input         string // Input is just the content of a VAR block
		numDecls      int
		expectedError string
		check         func(t *testing.T, decls []*ast.VarDeclStatement)
	}{
		{
			name:     "RETAIN and CONSTANT qualifiers",
			input:    `RETAIN CONSTANT myVar : INT := 1;`,
			numDecls: 1,
			check: func(t *testing.T, decls []*ast.VarDeclStatement) {
				if !decls[0].IsRetain {
					t.Error("Expected IsRetain to be true")
				}
				if !decls[0].IsConstant {
					t.Error("Expected IsConstant to be true")
				}
			},
		},
		{
			name:     "AT clause before type",
			input:    `myVar AT %IX0.0 : BOOL;`,
			numDecls: 1,
			check: func(t *testing.T, decls []*ast.VarDeclStatement) {
				if decls[0].Location == nil {
					t.Fatal("Expected Location to be parsed")
				}
				if decls[0].Location.Location.Address != "IX0.0" {
					t.Errorf("Expected location address to be 'IX0.0', got %s", decls[0].Location.Location.Address)
				}
			},
		},
		{
			name:     "AT clause after type",
			input:    `myVar : BOOL AT %QX1.1;`,
			numDecls: 1,
			check: func(t *testing.T, decls []*ast.VarDeclStatement) {
				if decls[0].Location == nil {
					t.Fatal("Expected Location to be parsed")
				}
				if decls[0].Location.Location.Address != "QX1.1" {
					t.Errorf("Expected location address to be 'QX1.1', got %s", decls[0].Location.Location.Address)
				}
			},
		},
		{
			name:          "Error on missing END_VAR",
			input:         `myVar : INT; IF x THEN END_IF`,
			numDecls:      1, // It should parse the first line
			expectedError: "expected next token to be END_VAR, got IF instead",
		},
		{
			name:          "Error on using = instead of :=",
			input:         `myVar : INT = 5;`,
			numDecls:      0, // The faulty declaration is now skipped entirely by the improved recovery.
			expectedError: "expected next token to be :=, got = instead",
		},
		{
			name:     "Comment between declarations",
			input:    `var1 : INT; (* a comment *) var2 : BOOL;`,
			numDecls: 2,
			check: func(t *testing.T, decls []*ast.VarDeclStatement) {
				testVarDeclStatement(t, decls[0], "var1", "INT")
				testVarDeclStatement(t, decls[1], "var2", "BOOL")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the input in a VAR block for the parser
			fullInput := fmt.Sprintf("VAR %s END_VAR", tt.input)
			l := lexer.New(fullInput)
			p := New(l)
			program := p.ParseProgram()

			if tt.expectedError != "" {
				if len(p.Errors()) == 0 {
					t.Fatalf("Expected an error but got none")
				}
				assertErrorContains(t, p.Errors(), tt.expectedError)
			} else {
				checkParserErrors(t, p, tt.name, fullInput)
			}

			if len(program.Statements) > 0 {
				block, ok := program.Statements[0].(*ast.VarBlockDeclaration)
				if !ok {
					t.Fatalf("Expected ast.VarBlockDeclaration, got %T", program.Statements[0])
				}
				if len(block.Declarations) != tt.numDecls {
					t.Fatalf("Expected %d declarations, got %d", tt.numDecls, len(block.Declarations))
				}
				if tt.check != nil {
					tt.check(t, block.Declarations)
				}
			} else if tt.numDecls > 0 {
				t.Fatalf("Expected statements but got none")
			}
		})
	}
}
