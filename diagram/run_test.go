package diagram_test

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"

	_ "github.com/apiarytech/beedance/stdlib" // registers SEL, MUX, LIMIT, ... with the evaluator
)

// Beremiz's counter test (tests/projects/iec61131_lang_test), in the text
// forms: three counts, a reset to 17, two counts give 19.
const counters = `
FUNCTION_BLOCK CounterLD
VAR_INPUT Reset : BOOL; END_VAR
VAR_OUTPUT Out : INT; END_VAR
VAR Cnt : INT; END_VAR
LD
  RUNG count (* not reset: one more *)
    /Reset t1:MOVE_INT(IN := ADD(Cnt, 1), OUT => Cnt)
  RUNG reset
    Reset t2:MOVE_INT(IN := 17, OUT => Cnt)
  RUNG out
    t3:MOVE_INT(IN := Cnt, OUT => Out)
END_LD
END_FUNCTION_BLOCK

FUNCTION_BLOCK MOVE_INT
VAR_INPUT IN : INT; END_VAR
VAR_OUTPUT OUT : INT; END_VAR
OUT := IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CounterFBD
VAR_INPUT Reset : BOOL; END_VAR
VAR_OUTPUT Out : INT; END_VAR
VAR Cnt : INT; END_VAR
FBD
  // the count, or the reset value
  Cnt := SEL(Reset, ADD(Cnt, 1), 17)
  Out := Cnt
END_FBD
END_FUNCTION_BLOCK
`

func run(t *testing.T, src, decl, script string) object.Object {
	t.Helper()
	all := src + "\n" + decl + "\n" + script
	p := parser.New(lexer.New(all))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v\n%s", errs, all)
	}
	object.FinalizeBuiltins()
	out := evaluator.Eval(prog, object.NewEnvironment())
	if e, ok := out.(*object.Error); ok {
		t.Fatalf("evaluate: %s\n%s", e.Inspect(), all)
	}
	vmSrc := src + "\nPROGRAM Harness\n" + decl + "\n" + strings.TrimSuffix(strings.TrimSpace(script), ";") + ";\nEND_PROGRAM\n"
	vp := parser.New(lexer.New(vmSrc))
	vprog := vp.ParseProgram()
	if errs := vp.Errors(); len(errs) > 0 {
		t.Fatalf("parse for the VM: %v", errs)
	}
	if _, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(vprog, "Harness"); err != nil {
		t.Fatalf("compile for the VM: %v", err)
	}
	return out
}

func TestTextCounters(t *testing.T) {
	for _, fb := range []string{"CounterLD", "CounterFBD"} {
		decl := "VAR c : " + fb + "; v : INT; END_VAR"
		got := run(t, counters, decl, "c(Reset := FALSE); c(Reset := FALSE); c(Reset := FALSE); c(Reset := TRUE); c(Reset := FALSE); c(Reset := FALSE); v := c.Out; v;")
		if n, _, _ := object.GetIntegerObjectValue(got); n != 19 {
			t.Errorf("%s: %s, want 19", fb, got.Inspect())
		}
	}
}

func TestTextSealInAndTimer(t *testing.T) {
	src := `
FUNCTION_BLOCK Motor
VAR_INPUT Start, Stop : BOOL; END_VAR
VAR_OUTPUT Run, Pulse : BOOL; Starts : INT; END_VAR
LD
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
  RUNG count
    +Run cu:CTU(PV := 100, CV => Starts)
  RUNG pulse
    Run ( P Pulse )
END_LD
END_FUNCTION_BLOCK
`
	script := `m(Start := TRUE); m(Start := FALSE); a := m.Run; b := m.Pulse;
m(Stop := TRUE); c := m.Run; m(Stop := FALSE); m(Start := TRUE); m(Start := FALSE); n := m.Starts;`
	run(t, src, "VAR m : Motor; a, b, c : BOOL; n : INT; END_VAR", script+" a;")
	for name, want := range map[string]string{"a": "true", "b": "false", "c": "false", "n": "2"} {
		got := run(t, src, "VAR m : Motor; a, b, c : BOOL; n : INT; END_VAR", script+" "+name+";")
		if got.Inspect() != want {
			t.Errorf("%s = %s, want %s", name, got.Inspect(), want)
		}
	}
}

// Errors in a diagram body name its source line.
func TestTextErrorLines(t *testing.T) {
	src := "PROGRAM P\nVAR A : BOOL; END_VAR\nLD\n  RUNG one\n    A ( B )\n  RUNG two\n    A B\nEND_LD\nEND_PROGRAM\n"
	p := parser.New(lexer.New(src))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) != 1 || !strings.Contains(errs[0], "line 6") {
		t.Fatalf("errors %v, want one at line 6", errs)
	}
	// An IL program whose first instruction is LD is still IL.
	il := "PROGRAM Q\nVAR A, B : BOOL; END_VAR\nLD A\nST B\nEND_PROGRAM\n"
	p = parser.New(lexer.New(il))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("IL: %v", errs)
	}
}

// The examples of doc/language.md parse and run.
func TestLanguageDocExamples(t *testing.T) {
	src := `
FUNCTION_BLOCK Motor
VAR_INPUT Start, Stop : BOOL; END_VAR
VAR_OUTPUT Run, Done : BOOL; Starts : INT; END_VAR
LD
  RUNG sealin (* start, hold, stop *)
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S) ( Done )
  RUNG count
    +Run cu:CTU(PV := 100, CV => Starts)
END_LD
END_FUNCTION_BLOCK

FUNCTION_BLOCK Level
VAR_INPUT L, SP : REAL; END_VAR
VAR_OUTPUT High : BOOL; Speed : REAL; END_VAR
FBD
  above = GE(L, SP)                     // a wire, named
  hold : TON(IN := above, PT := T#3S)   // an instance, declared and called
  High := OR(hold.Q, AND(High, NOT(LT(L, SUB(SP, 5.0)))))
  Speed := SEL(High, MUL(L, 0.5), 0.0)
END_FBD
END_FUNCTION_BLOCK
`
	got := run(t, src, "VAR m : Motor; v : Level; speed : REAL; END_VAR", "m(Start := TRUE); v(L := 10.0, SP := 50.0); speed := v.Speed; speed;")
	if got.Inspect() != "5.000000" {
		t.Errorf("Speed = %s, want 5", got.Inspect())
	}
}
