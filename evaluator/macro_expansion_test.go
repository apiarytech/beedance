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

	// After defining the macros, the AST should NOT be modified. The VAR block
	// should still contain all three original declarations.
	if len(program.Statements) != 1 {
		t.Fatalf("Program should have 1 statement (the VAR block). got=%d",
			len(program.Statements))
	}
	varBlock, ok := program.Statements[0].(*ast.VarBlockDeclaration)
	if !ok || len(varBlock.Declarations) != 3 {
		t.Fatalf("VAR block should still contain all 3 declarations. got=%d", len(varBlock.Declarations))
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
		{
			// Test case for when macro evaluation results in an error.
			// The macro should not be expanded, and the original call is returned.
			`VAR errorMacro : MACRO := macro() { 1 + TRUE; }; END_VAR errorMacro();`,
			`errorMacro();`,
		},
		{
			// Test case for a non-macro function call, which should not be expanded.
			// This covers the `if !ok` in `isMacroCall`.
			`LEN("test");`,
			`LEN("test");`,
		},
		{
			// Test case for a call expression where the function is not a simple identifier.
			// This covers the `if !ok` in `isMacroCall` for `exp.Function.(*ast.Identifier)`.
			`get_macro()();`,
			`get_macro()();`,
		},
	}

	for _, tt := range tests {
		expected := testParseProgram(tt.expected)
		program := testParseProgram(tt.input)

		env := object.NewEnvironment()
		DefineMacros(program, env)
		expanded := ExpandMacros(program, env)

		// The expanded AST is a full *ast.Program. We need to compare the
		// last statement of the expanded program with the first (and only)
		// statement of the expected program.
		expandedProg, ok := expanded.(*ast.Program)
		if !ok {
			t.Fatalf("expanded is not *ast.Program. got=%T", expanded)
		}

		lastExpandedStmt := expandedProg.Statements[len(expandedProg.Statements)-1]
		expectedStmt := expected.Statements[0]

		if lastExpandedStmt.String() != expectedStmt.String() {
			t.Errorf("macro expansion incorrect. want=%q, got=%q",
				expectedStmt.String(), lastExpandedStmt.String())
		}
	}
}

func TestExpandMacrosOnNonMacroCall(t *testing.T) {
	input := `myNonMacro();`
	expected := `myNonMacro();`

	program := testParseProgram(input)
	env := object.NewEnvironment()

	// Manually add a non-macro object to the environment to simulate a function
	// or other variable being present. This is to specifically test the
	// `macro, ok := obj.(*object.Macro)` type assertion inside `isMacroCall`.
	env.Set("myNonMacro", &object.LInt{Value: 123}) // Any non-macro object will do.

	// DefineMacros won't find any macros to define.
	DefineMacros(program, env)

	// ExpandMacros should ignore the call to `myNonMacro` because it's not a macro.
	expanded := ExpandMacros(program, env)

	if expanded.String() != expected {
		t.Errorf("macro expansion failed. want=%q, got=%q",
			expected, expanded.String())
	}
}

func TestIsMacroDefinition(t *testing.T) {
	t.Run("Valid Macro Definition", func(t *testing.T) {
		input := `VAR mymacro : MACRO := MACRO() { EXPR(1); }; END_VAR`
		program := testParseProgram(input)
		varBlock := program.Statements[0].(*ast.VarBlockDeclaration)
		decl := varBlock.Declarations[0]

		if !isMacroDefinition(decl) {
			t.Errorf("isMacroDefinition() returned false for a valid macro definition")
		}
	})

	t.Run("Not a Macro (regular var)", func(t *testing.T) {
		input := `VAR myvar : INT := 5; END_VAR`
		program := testParseProgram(input)
		varBlock := program.Statements[0].(*ast.VarBlockDeclaration)
		decl := varBlock.Declarations[0]

		if isMacroDefinition(decl) {
			t.Errorf("isMacroDefinition() returned true for a regular variable definition")
		}
	})

	t.Run("Not a VarDeclStatement", func(t *testing.T) {
		// This test case specifically covers the `if !ok` branch in isMacroDefinition.
		input := `5 + 5;`
		program := testParseProgram(input)
		stmt := program.Statements[0] // This will be an *ast.ExpressionStatement

		if isMacroDefinition(stmt) {
			t.Errorf("isMacroDefinition() returned true for a non-VarDeclStatement")
		}
	})
}

func TestExpandMacrosPanicsOnNonQuoteReturn(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ExpandMacros should have panicked but did not")
		} else {
			// Optionally check the panic message to be more specific.
			msg, ok := r.(string)
			if !ok || msg != "we only support returning AST-nodes from macros" {
				t.Errorf("unexpected panic message: got %v, want %q", r, "we only support returning AST-nodes from macros")
			}
		}
	}()

	// This macro's body evaluates to an Integer object, not a Quote object,
	// which should trigger the panic.
	input := `VAR nonQuoteMacro : MACRO := macro() { 1 + 2; }; END_VAR nonQuoteMacro();`
	program := testParseProgram(input)
	env := object.NewEnvironment()
	DefineMacros(program, env)
	ExpandMacros(program, env) // This line is expected to panic.
}

func testParseProgram(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}
