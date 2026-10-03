package parser

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/token"
	"strings"
	"testing"
)

func TestRuntimePanicRecovery(t *testing.T) {
	// This test verifies the parser's top-level panic recovery mechanism.
	// Specifically, it tests the case where a panic occurs that is NOT a custom `parseError`.
	// The parser's recover block should catch this, see it's not a `parseError`, and re-panic.

	// We expect this whole function to panic with the message we inject.
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic as expected")
		} else {
			expectedPanicMsg := "injected runtime panic"
			if msg, ok := r.(string); !ok || msg != expectedPanicMsg {
				t.Errorf("recovered unexpected panic value. want=%q, got=%v", expectedPanicMsg, r)
			}
		}
	}()

	// To trigger a runtime panic inside the parser's recovery scope, we'll
	// inject a temporary, malicious prefix parsing function for a token that
	// normally doesn't have one (like '+').
	input := `+ 5;`
	l := lexer.New(input)
	p := New(l)

	// Inject the panicking function.
	p.registerPrefix(token.PLUS, func() ast.Expression {
		panic("injected runtime panic")
	})

	// This call will trigger our panicking prefix function. The recover block in
	// ParseProgram will catch it, see it's not a *parseError, and re-panic.
	// Our test's defer block above will then catch the re-panic and pass the test.
	p.ParseProgram()
}

func TestParseErrorPanicRecoveryAndSynchronization(t *testing.T) {
	// This test verifies that if a parsing function panics with a `*parseError`,
	// the main `ParseProgram` loop recovers, calls `synchronizeParser`, and
	// successfully continues parsing subsequent statements.

	// We'll inject a prefix function that panics with a `*parseError`.
	// The input has valid statements before and after the error token.
	input := `
		VAR x : INT; END_VAR
		= (* This equals sign will trigger our injected panic *)
		VAR y : BOOL; END_VAR
	`
	l := lexer.New(input)
	p := New(l)

	// Inject a panicking function for the '=' token, which normally has no prefix handler.
	p.registerPrefix(token.EQ, func() ast.Expression {
		p.currentError("injected parse error on '='")
		// This simulates a critical parsing failure that uses panic for control flow.
		panic(&parseError{msg: "injected parse error"})
	})

	// This call should recover from the panic and parse the whole program.
	program := p.ParseProgram()

	// 1. Check that the injected error was logged.
	if len(p.Errors()) == 0 {
		t.Fatal("Expected parser to have errors, but it had none.")
	}
	if !strings.Contains(p.Errors()[0], "injected parse error on '='") {
		t.Errorf("Expected error message not found. got=%v", p.Errors())
	}

	// 2. Check that the parser recovered and parsed statements *after* the error.
	// The panic happens on '='. `synchronizeParser` should run, skip to the next
	// statement keyword (`VAR y`), and parsing should continue.
	// We expect 2 statements in the final program.
	if len(program.Statements) != 2 {
		t.Fatalf("Parser did not recover correctly. Expected 2 statements, got %d", len(program.Statements))
	}

	// 3. Verify the statements before and after the error were parsed correctly.
	if _, ok := program.Statements[0].(*ast.VarBlockDeclaration); !ok {
		t.Errorf("First statement (before error) was not parsed correctly. got=%T", program.Statements[0])
	}
	if _, ok := program.Statements[1].(*ast.VarBlockDeclaration); !ok {
		t.Errorf("Second statement (after error) was not parsed correctly. got=%T", program.Statements[1])
	}
}
