package compiler

import (
	"errors"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// A compile error carries the position of the innermost node that failed,
// and its message is unchanged.
func TestCompileErrorPosition(t *testing.T) {
	src := "PROGRAM P\nVAR x : INT; END_VAR\nIF x > 0 THEN\n  x := nope + 1;\nEND_IF;\nEND_PROGRAM\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	_, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(prog, "P")
	if err == nil {
		t.Fatal("compiled a program reading an undeclared variable")
	}
	var pe *PositionError
	if !errors.As(err, &pe) {
		t.Fatalf("%v has no position", err)
	}
	if pe.Row != 4 {
		t.Errorf("error %q at row %d, column %d; want row 4", err, pe.Row, pe.Column)
	}
	if err.Error() != pe.Err.Error() {
		t.Errorf("message changed: %q", err.Error())
	}
}
