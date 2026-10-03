package parser

import (
	"fmt" // cspell:disable-line
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/token"
)

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
	program := p.ParseProgram()
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

// Comments nest, as IEC 61131-3 allows and CODESYS does, so code with its
// own comments can be commented out.
func TestNestedComments(t *testing.T) {
	input := `
		(* outer (* middle (* inner *) *) outer *)
		x := 1; (* old code
		IF x > 1 THEN (* a note *) x := 2; END_IF;
		*)
		y := 2;
	`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements outside the comments, got %d: %s", len(program.Statements), program.String())
	}
	// A comment whose nested comments close but which does not is unterminated.
	p = New(lexer.New("(* outer (* inner *) never closed\nx := 1;"))
	p.ParseProgram()
	assertErrorContains(t, p.Errors(), "unterminated comment")
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

	if dataType != "" {
		if s.DataType.String() != dataType && s.DataType.TokenLiteral() != dataType {
			t.Errorf("s.DataType.String() not '%s'. got=%s",
				dataType, s.DataType.String())
			return false
		}
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
	case uint64:
		// The parser may produce a signed IntegerLiteral even for an unsigned value
		// if the token type from the lexer is generic (e.g., token.INT).
		// We check for unsigned first, then fall back to signed to make the test robust.
		if testUnsignedIntegerLiteral(t, exp, v) {
			return true
		}
		return testIntegerLiteral(t, exp, int64(v))
	}
	t.Errorf("type of exp not handled. got=%T", exp)
	return false
}

func testUnsignedIntegerLiteral(t *testing.T, uil ast.Expression, value uint64) bool {
	t.Helper()
	integ, ok := uil.(*ast.UnsignedIntegerLiteral)
	if !ok {
		// Don't fail immediately. Return false so the caller can try another type.
		if testing.Verbose() {
			t.Logf("uil not *ast.UnsignedIntegerLiteral. got=%T", uil)
		}
		return false
	}

	if integ.Value != value {
		t.Errorf("integ.Value not %d. got=%d", value, integ.Value)
		return false
	}

	return true
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

func testStringLiteral(t *testing.T, sl ast.Expression, value string) bool {
	t.Helper()
	var strLitValue string
	var ok bool
	if strLit, isString := sl.(*ast.StringLiteral); isString {
		strLitValue = strLit.Value
		ok = true
	} else if wstrLit, isWString := sl.(*ast.WStringLiteral); isWString {
		strLitValue = wstrLit.Value
		ok = true
	}

	if !ok {
		t.Errorf("sl not *ast.StringLiteral or *ast.WStringLiteral. got=%T", sl)
		return false
	}

	if strLitValue != value {
		t.Errorf("strLit.Value not %q. got=%q", value, strLitValue)
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

func TestPeek2TokenIs(t *testing.T) {
	input := `VAR myVar : INT := 5;`
	l := lexer.New(input)
	p := New(l)

	// After New(), tokens are:
	// curToken:  VAR
	// peekToken: myVar (IDENT)
	// peek2Token: : (COLON)

	if !p.curTokenIs(token.VAR) {
		t.Fatalf("Expected curToken to be VAR, got %s", p.curToken.Type)
	}
	if !p.peekTokenIs(token.IDENT) {
		t.Fatalf("Expected peekToken to be IDENT, got %s", p.peekToken.Type)
	}
	if !p.peek2TokenIs(token.COLON) {
		t.Fatalf("Expected peek2Token to be COLON, but peek2TokenIs returned false. Token is %s", p.peek2Token.Type)
	}

	// Advance tokens by one
	p.nextToken()

	// New state:
	// curToken:  myVar (IDENT)
	// peekToken: : (COLON)
	// peek2Token: INT
	if !p.peek2TokenIs(token.INT) {
		t.Fatalf("After nextToken(), expected peek2Token to be INT, but peek2TokenIs returned false. Token is %s", p.peek2Token.Type)
	}
}

func TestParseAccessDeclarationsErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		input         string // Just the content of a VAR_ACCESS block
		expectedError string
		numDecls      int // Expected number of declarations parsed after recovery
	}{
		{
			name:          "Missing END_VAR",
			input:         `accVar : OtherFB; PROGRAM MyProg END_PROGRAM`,
			expectedError: "expected next token to be END_VAR, got IDENT instead",
			numDecls:      1,
		},
		{
			name:          "Missing identifier for local name",
			input:         `: OtherFB; END_VAR`,
			expectedError: "expected identifier for local access variable name, got :",
			numDecls:      0,
		},
		{
			name:          "Missing colon after local name",
			input:         `accVar OtherFB; END_VAR`,
			expectedError: "expected next token to be :, got IDENT instead",
			numDecls:      0,
		},
		{
			name:          "Invalid access path",
			input:         `accVar : ; END_VAR`,
			expectedError: "no prefix parse function for ; found",
			numDecls:      0, // No declaration should be parsed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullInput := fmt.Sprintf("VAR_ACCESS %s", tt.input)
			l := lexer.New(fullInput)
			p := New(l)
			program := p.ParseProgram()

			if len(p.Errors()) == 0 {
				t.Fatalf("Expected an error but got none for input: %s", fullInput)
			}
			assertErrorContains(t, p.Errors(), tt.expectedError)

			if len(program.Statements) == 0 {
				if tt.numDecls > 0 {
					t.Fatalf("Expected %d declarations, but no statements were parsed.", tt.numDecls)
				}
				return
			}

			// The first statement should be the AccessVarDeclaration block
			accessBlock, ok := program.Statements[0].(*ast.AccessVarDeclaration)
			if !ok {
				t.Fatalf("Expected ast.AccessVarDeclaration as the first statement, got %T", program.Statements[0])
			}

			if len(accessBlock.Vars) != tt.numDecls {
				t.Errorf("Expected %d declarations in the VAR_ACCESS block after recovery, got %d", tt.numDecls, len(accessBlock.Vars))
			}
		})
	}
}

func TestParseSpecialVarBlocks(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
		check         func(t *testing.T, program *ast.Program)
	}{
		{
			name: "VAR_EXTERNAL happy path",
			input: `PROGRAM MyProg
						VAR_EXTERNAL
							extVar : BOOL;
						END_VAR
					END_PROGRAM`,
			check: func(t *testing.T, program *ast.Program) {
				progDecl := program.Statements[0].(*ast.ProgramDeclaration)
				if len(progDecl.VarExternal) != 1 {
					t.Fatalf("Expected 1 VAR_EXTERNAL block, got %d", len(progDecl.VarExternal))
				}
				extBlock := progDecl.VarExternal[0]
				if len(extBlock.Vars) != 1 {
					t.Fatalf("Expected 1 var in VAR_EXTERNAL, got %d", len(extBlock.Vars))
				}
				testVarDeclStatement(t, extBlock.Vars[0], "extVar", "BOOL")
			},
		},
		{
			name:          "VAR_EXTERNAL missing END_VAR",
			input:         `PROGRAM MyProg VAR_EXTERNAL extVar : BOOL; PROGRAM OtherProg END_PROGRAM`,
			expectedError: "expected next token to be END_VAR, got PROGRAM instead",
		},
		{
			name: "VAR_ACCESS with READ_WRITE",
			input: `PROGRAM MyProg
						VAR_ACCESS
							accVar : OtherFB READ_WRITE;
						END_VAR
					END_PROGRAM`,
			check: func(t *testing.T, program *ast.Program) {
				progDecl := program.Statements[0].(*ast.ProgramDeclaration)
				if len(progDecl.VarAccess) != 1 {
					t.Fatalf("Expected 1 VAR_ACCESS block, got %d", len(progDecl.VarAccess))
				}
				accBlock := progDecl.VarAccess[0]
				if len(accBlock.Vars) != 1 {
					t.Fatalf("Expected 1 var in VAR_ACCESS, got %d", len(accBlock.Vars))
				}
				decl := accBlock.Vars[0]
				if decl.Name.Value != "accVar" || decl.AccessPath.String() != "OtherFB" || decl.AccessType != "READ_WRITE" {
					t.Errorf("VAR_ACCESS declaration parsed incorrectly")
				}
			},
		},
		{
			name: "VAR_ACCESS with no access type",
			input: `PROGRAM MyProg
						VAR_ACCESS
							accVar : OtherFB;
						END_VAR
					END_PROGRAM`,
			check: func(t *testing.T, program *ast.Program) {
				progDecl := program.Statements[0].(*ast.ProgramDeclaration)
				if len(progDecl.VarAccess) != 1 {
					t.Fatalf("Expected 1 VAR_ACCESS block, got %d", len(progDecl.VarAccess))
				}
				accBlock := progDecl.VarAccess[0]
				if len(accBlock.Vars) != 1 {
					t.Fatalf("Expected 1 var in VAR_ACCESS, got %d", len(accBlock.Vars))
				}
				decl := accBlock.Vars[0]
				if decl.AccessType != "" {
					t.Errorf("Expected empty access type, got %s", decl.AccessType)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()

			if tt.expectedError != "" {
				assertErrorContains(t, p.Errors(), tt.expectedError)
			} else {
				checkParserErrors(t, p, tt.name, tt.input)
			}

			if tt.check != nil {
				if len(program.Statements) == 0 {
					t.Fatal("No statements parsed")
				}
				tt.check(t, program)
			}
		})
	}
}

func TestProgramDeclarationWithComments(t *testing.T) {
	input := `
		PROGRAM MyProgWithComments
			VAR_INPUT
				In1 : BOOL;
			END_VAR
			(* This is a multi-line comment *)
			VAR_OUTPUT
				Out1 : INT;
			END_VAR
			// This is a single-line comment
			VAR
				Local1 : REAL;
			END_VAR
		END_PROGRAM
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramDeclarationWithComments", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	prog, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	if len(prog.VarInputs) != 1 {
		t.Errorf("Expected 1 VAR_INPUT block, got %d", len(prog.VarInputs))
	}
	if len(prog.VarOutputs) != 1 {
		t.Errorf("Expected 1 VAR_OUTPUT block, got %d", len(prog.VarOutputs))
	}
	if len(prog.Vars) != 1 {
		t.Errorf("Expected 1 VAR block, got %d", len(prog.Vars))
	}

	testVarDeclStatement(t, prog.VarInputs[0], "In1", "BOOL")
	testVarDeclStatement(t, prog.VarOutputs[0], "Out1", "INT")
	testVarDeclStatement(t, prog.Vars[0], "Local1", "REAL")
}

func TestVarBlockQualifiers(t *testing.T) {
	tests := []struct {
		input       string
		isConstant  bool
		isRetain    bool
		isNonRetain bool
	}{
		{"VAR CONSTANT myVar : INT; END_VAR", true, false, false},
		{"VAR RETAIN myVar : INT; END_VAR", false, true, false},
		{"VAR NON_RETAIN myVar : INT; END_VAR", false, false, true},
		{"VAR CONSTANT RETAIN myVar : INT; END_VAR", true, true, false},
		{"VAR CONSTANT NON_RETAIN myVar : INT; END_VAR", true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p, "TestVarBlockQualifiers", tt.input)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
			}

			block, ok := program.Statements[0].(*ast.VarBlockDeclaration)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.VarBlockDeclaration. got=%T", program.Statements[0])
			}

			if len(block.Declarations) != 1 {
				t.Fatalf("block.Declarations does not contain 1 statement. got=%d", len(block.Declarations))
			}
			stmt := block.Declarations[0]

			if stmt.IsConstant != tt.isConstant {
				t.Errorf("IsConstant wrong. want=%v, got=%v", tt.isConstant, stmt.IsConstant)
			}
			if stmt.IsRetain != tt.isRetain {
				t.Errorf("IsRetain wrong. want=%v, got=%v", tt.isRetain, stmt.IsRetain)
			}
		})
	}
}

func assertErrorContains(t *testing.T, errors []string, expected string, index ...int) {
	t.Helper()
	if expected == "" {
		if len(errors) > 0 {
			t.Errorf("Expected no errors, but got %d: %v", len(errors), errors)
		}
		return
	}

	if len(index) > 0 { // Check a specific error by index
		idx := index[0]
		if idx >= len(errors) {
			t.Errorf("Error index %d out of bounds. Only %d errors reported: %v", idx, len(errors), errors)
			return
		}
		if !strings.Contains(errors[idx], expected) {
			t.Errorf("Expected error at index %d to contain %q, got %q", idx, expected, errors[idx])
		}
	} else { // Search all errors for the expected string
		found := false
		for _, err := range errors {
			if strings.Contains(err, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find error %q in parser errors, but did not. Errors: %v", expected, errors)
		}
	}
}

func TestInfiniteLoopRecovery(t *testing.T) {
	// This test ensures the parser's infinite loop detection and recovery works.
	// We inject a faulty prefix function for the ASTERISK token that doesn't advance the parser.
	input := `
		VAR x : INT; END_VAR
		* 5;
		VAR y : BOOL; END_VAR
	`
	l := lexer.New(input)
	p := New(l)

	// Inject a faulty function that doesn't consume the token, which should trigger the infinite loop detector.
	p.registerPrefix(token.ASTERISK, func() ast.Expression {
		// This is intentionally wrong. A prefix function MUST advance the token.
		// By not calling p.nextToken(), we simulate a bug that would cause an infinite loop.
		// Returning a valid node makes the test case stronger, as it forces
		// parseExpressionStatement to continue and fail on the missing semicolon.
		return &ast.Identifier{Token: p.curToken, Value: "faulty"}
	})

	program := p.ParseProgram()

	// Check the sequence of errors.
	// 1. The parser tries to parse `* 5;`. It parses `*` as "faulty", then fails
	//    because the next token is `5` (INT) instead of `;`.
	assertErrorContains(t, p.Errors(), "expected next token to be ;, got INT instead", 0)
	// 2. Because an error occurred, the main loop does not advance the token inside the anonymous func.
	//    On the next iteration, the infinite loop detector fires because the
	//    token is still `*`.
	assertErrorContains(t, p.Errors(), "parser stuck on token * (*), forcing advance to recover", 1)

	// Check that the parser recovered and parsed the statements before and after the faulty one.
	// 1. The first VAR block.
	// 2. The expression statement from `* 5;`, which fails but still produces a statement node.
	// 3. The recovery mechanism advances past the `*` to the `5`, then the loop continues.
	//    The `5` is parsed as another (erroneous) expression statement.
	// 4. The second VAR block.
	// The final result is 4 statements. The test is updated to reflect this correct recovery behavior.
	if len(program.Statements) != 4 {
		t.Fatalf("Expected 4 statements after recovery, got %d", len(program.Statements))
	}
}

func TestIsValidIecDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Valid cases
		{"5s", true},
		{"1h_30m", true},
		{"-10s_500ms", true},
		{"1.5s", true},
		{"1d_2h_3m_4s_5ms", true},
		{"1D_2H_3M_4S_5MS", true}, // Case-insensitivity

		// Invalid cases
		{"5z", false},      // Invalid unit
		{"1h30m", false},   // Missing underscore
		{"_5s", false},     // Leading underscore
		{"5s_", false},     // Trailing underscore
		{"s5", false},      // Unit before number
		{"", false},        // Empty string
		{"1_h", false},     // Underscore before unit
		{"1h_m", false},    // Missing number for a unit
		{"1.5.5s", false},  // Invalid number format
		{"1h__30m", false}, // Double underscore
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// The function under test uses strings.ToLower, so the test is inherently case-insensitive.
			if got := isValidIecDuration(tt.input); got != tt.expected {
				t.Errorf("isValidIecDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// testErrorContains is a helper to check if a slice of error strings contains a specific substring.
func testErrorContains(errors []string, expected string) bool {
	for _, err := range errors {
		if strings.Contains(err, expected) {
			return true
		}
	}
	return false
}
