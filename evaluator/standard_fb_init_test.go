package evaluator

import (
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// TestStandardFBOutputsBeforeFirstCall: a standard block's outputs read
// before its first call have their initial values instead of failing (an
// SFC step's timer is read in the transition before it is updated).
func TestStandardFBOutputsBeforeFirstCall(t *testing.T) {
	src := `PROGRAM P
VAR
  t : TON; c : CTUD; r : R_TRIG; s : SR;
  q : BOOL; et : TIME; cv : LINT; qu : BOOL; rq : BOOL; q1 : BOOL; bad : BOOL;
END_VAR
q := t.Q; et := t.ET; cv := c.CV; qu := c.QU; rq := r.Q; q1 := s.Q1;
END_PROGRAM
P();`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	object.FinalizeBuiltins()
	env := object.NewEnvironment()
	if out := Eval(prog, env); isError(out) {
		t.Fatalf("reading outputs before the first call: %s", out.Inspect())
	}

	// A member a standard block does not have is an error, not a crash.
	bad := parser.New(lexer.New(`PROGRAM Q VAR t : TON; x : BOOL; END_VAR x := t.NOPE; END_PROGRAM Q();`)).ParseProgram()
	out := Eval(bad, object.NewEnvironment())
	if !isError(out) {
		t.Fatalf("t.NOPE: %v, want an error", out)
	}
}

// TestStandardFBHeldInputs: an input held TRUE from a variable is one edge,
// not one per call. The variable's BOOL is not the TRUE object, and the
// blocks' memories were compared with it by identity (BUG-25).
func TestStandardFBHeldInputs(t *testing.T) {
	src := `FUNCTION_BLOCK W
VAR_INPUT X : BOOL; END_VAR
VAR_OUTPUT Q, FQ : BOOL; N : INT; END_VAR
VAR H : BOOL; r : R_TRIG; f : F_TRIG; c : CTU; END_VAR
H := X OR H;
r(CLK := H); Q := r.Q;
f(CLK := NOT H); FQ := f.Q;
c(CU := H, PV := 10); N := c.CV;
END_FUNCTION_BLOCK
VAR w : W; q, fq : BOOL; n : INT; END_VAR
w(X := TRUE); w(X := FALSE); w(X := FALSE);
q := w.Q; fq := w.FQ; n := w.N;`
	for name, want := range map[string]string{"q": "false", "fq": "false", "n": "1"} {
		p := parser.New(lexer.New(src + " " + name + ";"))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		object.FinalizeBuiltins()
		if out := Eval(prog, object.NewEnvironment()); out.Inspect() != want {
			t.Errorf("%s = %s, want %s", name, out.Inspect(), want)
		}
	}
}
