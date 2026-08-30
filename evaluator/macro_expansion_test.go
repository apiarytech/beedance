package evaluator

import (
	"beedance/ast"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"testing"
)

func TestDefineMacros(t *testing.T) {
	input := `
		VAR
			number : INT := 1;
			mymacro : MACRO := macro(x, y) { x + y; };
			mymacroTwo : MACRO := macro(x, y) { x + y; };
		END_VAR
	`

	env := object.NewEnvironment()
	program := testParseProgram(input)

	DefineMacros(program, env)

	// After defining the macros, the VAR block should still exist but only contain 'number'.
	if len(program.Statements) != 1 {
		t.Fatalf("Program should have 1 statement (the VAR block). got=%d",
			len(program.Statements))
	}
	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok || len(varBlock.Declarations) != 1 {
		t.Fatalf("VAR block should contain exactly one declaration ('number') after macro removal.")
	}

	// Check that the macros were added to the environment.
	obj, ok := env.Get("mymacro")
	if !ok {
		t.Fatalf("macro not in environment.")
	}
	macro, ok := obj.(*object.Macro)
	if !ok {
		t.Fatalf("object is not Macro. got=%T (%+v)", obj, obj)
	}
	if len(macro.Parameters) != 2 {
		t.Fatalf("Wrong number of macro parameters. got=%d",
			len(macro.Parameters))
	}
	if macro.Parameters[0].String() != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", macro.Parameters[0])
	}
	if macro.Parameters[1].String() != "y" {
		t.Fatalf("parameter is not 'y'. got=%q", macro.Parameters[1])
	}
	expectedBody := "(x + y);"
	if macro.Body.String() != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, macro.Body.String())
	}

	// Check that the second macro was also defined.
	_, ok = env.Get("mymacroTwo")
	if !ok {
		t.Fatalf("macro 'mymacroTwo' not in environment.")
	}
}

func TestExpandMacros(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`VAR 
				infixExpression : MACRO := macro() { EXPR(1 + 2); }; 
			END_VAR 
			infixExpression();`,
			`(1 + 2);`,
		},
		{
			`VAR 
				reverse : MACRO := macro(a, b) { EXPR(EVAL(b) - EVAL(a)); }; 
			END_VAR 
			reverse(2 + 2, 10 - 5);`,
			`((10 - 5) - (2 + 2));`,
		},
	}

	for _, tt := range tests {
		expected := testParseProgram(tt.expected)
		program := testParseProgram(tt.input)

		env := object.NewEnvironment()
		DefineMacros(program, env)
		expanded := ExpandMacros(program, env)

		if expanded.String() != expected.String() {
			t.Errorf("not equal. want=%q, got=%q",
				expected.String(), expanded.String())
		}
	}
}

func testParseProgram(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}
