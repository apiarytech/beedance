/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

// These tests cover what a host needs to run a compiled PROGRAM every scan:
// one VM reused across scans, a clock of the host's choosing, and the
// program's variables by name.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/stdlib"
)

func compileProgram(t *testing.T, input string) *compiler.CompiledProgram {
	t.Helper()
	object.FinalizeBuiltins()
	program := parse(t, input)
	var name string
	for _, s := range program.Statements {
		if pd, ok := s.(*ast.ProgramDeclaration); ok {
			name = pd.Name.Value
		}
	}
	compiled, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, name)
	if err != nil {
		t.Fatalf("CompileProgram: %s", err)
	}
	return compiled
}

func variable(t *testing.T, p *compiler.CompiledProgram, name string) compiler.Variable {
	t.Helper()
	for _, v := range p.Variables {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("no variable %s in %+v", name, p.Variables)
	return compiler.Variable{}
}

func TestResetReusesVMAcrossScans(t *testing.T) {
	p := compileProgram(t, `PROGRAM Counter
		VAR count : DINT; END_VAR
		count := count + 1;
	END_PROGRAM`)
	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
	for i := 0; i < 3; i++ {
		scan.Reset()
		if err := scan.Run(); err != nil {
			t.Fatalf("scan %d: %s", i, err)
		}
	}
	count := variable(t, p, "count")
	if v, _, ok := object.GetIntegerObjectValue(globals[count.Global]); !ok || v != 3 {
		t.Fatalf("count = %v after three scans, want 3", globals[count.Global])
	}

	// A reused VM allocates only what the program itself creates.
	allocs := testing.AllocsPerRun(100, func() {
		scan.Reset()
		scan.Run()
	})
	if allocs > 10 {
		t.Errorf("a reused scan allocates %v times", allocs)
	}
}

func TestHostClock(t *testing.T) {
	p := compileProgram(t, `PROGRAM Delay
		VAR t : TON; q : BOOL; END_VAR
		t(IN := TRUE, PT := T#100ms);
		q := t.Q;
	END_PROGRAM`)
	var now time.Duration
	globals := make([]object.Object, GlobalsSize)
	init := NewWithGlobalsStore(p.InitBytecode, globals)
	scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return now }) {
		init.SetBuiltin(idx, b)
		scan.SetBuiltin(idx, b)
	}
	if err := init.Run(); err != nil {
		t.Fatal(err)
	}
	q := variable(t, p, "q")
	run := func() bool {
		t.Helper()
		scan.Reset()
		if err := scan.Run(); err != nil {
			t.Fatal(err)
		}
		b, ok := globals[q.Global].(*object.Boolean)
		return ok && b.Value
	}
	if run() {
		t.Fatal("Q at 0 ms")
	}
	now = 50 * time.Millisecond
	if run() {
		t.Fatal("Q at 50 ms")
	}
	// The process clock moves on; only the host's clock counts.
	now = 150 * time.Millisecond
	if !run() {
		t.Fatal("no Q at 150 ms")
	}
}

func TestProgramVariables(t *testing.T) {
	p := compileProgram(t, `PROGRAM Mixer
		VAR_INPUT start : BOOL; END_VAR
		VAR_OUTPUT running : BOOL; END_VAR
		VAR RETAIN batches : DINT; END_VAR
		VAR CONSTANT limit : INT := 10; END_VAR
		VAR motor AT %QX0.1 : BOOL; t : TON; END_VAR
		running := start;
		motor := running;
	END_PROGRAM`)
	start, running, batches := variable(t, p, "start"), variable(t, p, "running"), variable(t, p, "batches")
	limit, motor, timer := variable(t, p, "limit"), variable(t, p, "motor"), variable(t, p, "t")
	switch {
	case start.Block != "VAR_INPUT" || start.Type != "BOOL" || start.Global < 0:
		t.Errorf("start = %+v", start)
	case running.Block != "VAR_OUTPUT":
		t.Errorf("running = %+v", running)
	case !batches.Retain || batches.Type != "DINT":
		t.Errorf("batches = %+v", batches)
	case !limit.Constant:
		t.Errorf("limit = %+v", limit)
	case motor.Address != "%QX0.1" || motor.Global != -1:
		t.Errorf("motor = %+v", motor)
	case timer.Type != "TON" || timer.Global < 0:
		t.Errorf("t = %+v", timer)
	}

	// A host writes an input into its slot and reads outputs back.
	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	globals[start.Global] = True
	scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
	if err := scan.Run(); err != nil {
		t.Fatal(err)
	}
	if globals[running.Global] != True {
		t.Errorf("running = %v", globals[running.Global])
	}
	if b, ok := scan.IO()["%QX0.1"].(*object.Boolean); !ok || !b.Value {
		t.Errorf("motor in the I/O image = %v", scan.IO()["%QX0.1"])
	}
}

// TestProgramUnit: a program uses a function block declared beside it in the
// same source; other programs in the source are left out.
func TestProgramUnit(t *testing.T) {
	p := compileProgram(t, `FUNCTION_BLOCK Doubler
		VAR_INPUT x : DINT; END_VAR
		VAR_OUTPUT y : DINT; END_VAR
		y := x * 2;
	END_FUNCTION_BLOCK
	PROGRAM Main
		VAR d : Doubler; out : DINT; END_VAR
		d(x := 21);
		out := d.y;
	END_PROGRAM`)
	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	if err := NewWithGlobalsStore(p.CyclicBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	if v, _, ok := object.GetIntegerObjectValue(globals[variable(t, p, "out").Global]); !ok || v != 42 {
		t.Fatalf("out = %v, want 42", globals[variable(t, p, "out").Global])
	}
	object.FinalizeBuiltins()
	if _, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(parse(t, "PROGRAM A VAR x : INT; END_VAR x := 1; END_PROGRAM"), "B"); err == nil {
		t.Error("compiled a program that is not in the source")
	}
}

// TestBudget: a scan that loops for ever is stopped by the budget; the next
// scan, after Reset, gets a fresh one.
func TestBudget(t *testing.T) {
	p := compileProgram(t, `PROGRAM Spin
		VAR_INPUT forever : BOOL; END_VAR
		VAR_OUTPUT n : DINT; END_VAR
		WHILE forever DO n := n + 1; END_WHILE;
		n := n + 1;
	END_PROGRAM`)
	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
	scan.SetBudget(10000)
	for i := 0; i < 3; i++ { // a normal scan, many times, stays within it
		scan.Reset()
		if err := scan.Run(); err != nil {
			t.Fatalf("scan %d: %v", i, err)
		}
	}
	globals[variable(t, p, "forever").Global] = True
	scan.Reset()
	if err := scan.Run(); !errors.Is(err, ErrBudget) {
		t.Fatalf("an endless loop: err = %v, want ErrBudget", err)
	}
}

// A return in a program's body (IL RET, ST RETURN) ends that scan, as the
// end of the body does; the next scan starts at the top. It popped the
// main frame and failed with "index out of range [-1]".
func TestReturnEndsTheScan(t *testing.T) {
	for _, src := range []string{
		"PROGRAM P\nVAR n, after : DINT; END_VAR\n  LD n\n  ADD 1\n  ST n\n  RET\n  LD 1\n  ST after\nEND_PROGRAM",
		"PROGRAM P\nVAR n, after : DINT; END_VAR\nn := n + 1;\nRETURN;\nafter := 1;\nEND_PROGRAM",
	} {
		p := compileProgram(t, src)
		globals := make([]object.Object, GlobalsSize)
		if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
			t.Fatal(err)
		}
		scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
		for i := 0; i < 3; i++ {
			scan.Reset()
			if err := scan.Run(); err != nil {
				t.Fatalf("scan %d: %v\n%s", i+1, err, src)
			}
		}
		n, after := variable(t, p, "n"), variable(t, p, "after")
		if v, _, _ := object.GetIntegerObjectValue(globals[n.Global]); v != 3 {
			t.Errorf("n = %v after three scans, want 3\n%s", globals[n.Global], src)
		}
		if v, _, _ := object.GetIntegerObjectValue(globals[after.Global]); v != 0 {
			t.Errorf("after = %v: the code after the return ran\n%s", globals[after.Global], src)
		}
	}
}

// IEC 61131-3 names are not case-sensitive: an FB body reads its input RST
// as rst, and a program reads the output Q as b.q (OSCAT's SRAMP faulted
// on `IF rst OR NOT init`). DWORD_TO_REAL converts the number (OSCAT's
// TC_S).
func TestNamesIgnoreCase(t *testing.T) {
	src := "FUNCTION_BLOCK B\nVAR_INPUT RST : BOOL; END_VAR\nVAR_OUTPUT Q : BOOL; N : DINT; END_VAR\nq := rst OR FALSE;\nn := n + 1;\nEND_FUNCTION_BLOCK\n" +
		"PROGRAM P\nVAR b : B; on : BOOL; count : DINT; r : REAL; w : DWORD := 16#10; END_VAR\nb(RST := TRUE);\non := b.q AND TRUE;\ncount := b.n;\nr := DWORD_TO_REAL(w) * 0.5;\nEND_PROGRAM"
	p := compileProgram(t, src)
	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
	for i := 0; i < 2; i++ {
		scan.Reset()
		if err := scan.Run(); err != nil {
			t.Fatalf("scan %d: %v", i+1, err)
		}
	}
	if on := globals[variable(t, p, "on").Global]; !strings.EqualFold(on.Inspect(), "TRUE") {
		t.Errorf("on = %v, want TRUE", on.Inspect())
	}
	if v, _, _ := object.GetIntegerObjectValue(globals[variable(t, p, "count").Global]); v != 2 {
		t.Errorf("count = %v, want 2", globals[variable(t, p, "count").Global])
	}
	if f, _ := object.GetFloat64Value(globals[variable(t, p, "r").Global]); f != 8 {
		t.Errorf("r = %v, want 8", globals[variable(t, p, "r").Global])
	}
}

// An integer given to a DWORD is a DWORD (OSCAT's ONTIME sets ms := 0 and
// then reads DWORD_TO_ULINT(ms)): LibDemo faulted with "ULINT ERROR" the
// first time its pump ran.
func TestIntegerToBitString(t *testing.T) {
	src := "FUNCTION_BLOCK M\nVAR_INPUT IN : BOOL; END_VAR\nVAR_OUTPUT S : UDINT; END_VAR\nVAR ms, tx, last : DWORD; init : BOOL; END_VAR\n" +
		"tx := DWORD#16#FFFFFF00;\nIF NOT init THEN init := TRUE; last := tx; ms := 0; END_IF;\n" +
		"IF IN THEN ms := ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last) + DWORD_TO_ULINT(ms) + 1000) MOD 4294967296);\n" +
		"IF ms >= 1000 THEN S := S + 1; ms := ULINT_TO_DWORD(DWORD_TO_ULINT(ms) - 1000); END_IF; END_IF;\nlast := tx;\nEND_FUNCTION_BLOCK\n" +
		"PROGRAM P\nVAR m : M; s : UDINT; d : DWORD; u : ULINT; END_VAR\nm(IN := TRUE);\ns := m.S;\nd := 5;\nu := DWORD_TO_ULINT(d) + 1;\nEND_PROGRAM"
	p := compileProgram(t, src)
	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(p.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	scan := NewWithGlobalsStore(p.CyclicBytecode, globals)
	for i := 0; i < 3; i++ {
		scan.Reset()
		if err := scan.Run(); err != nil {
			t.Fatalf("scan %d: %v", i+1, err)
		}
	}
	if d := globals[variable(t, p, "d").Global]; d.Type() != object.BITSTRING_OBJ {
		t.Errorf("d := 5 holds %s", d.Type())
	}
	if v, _, _ := object.GetIntegerObjectValue(globals[variable(t, p, "s").Global]); v != 3 {
		t.Errorf("s = %v, want 3", globals[variable(t, p, "s").Global])
	}
	if v, _, _ := object.GetIntegerObjectValue(globals[variable(t, p, "u").Global]); v != 6 {
		t.Errorf("u = %v, want 6", globals[variable(t, p, "u").Global])
	}
}
