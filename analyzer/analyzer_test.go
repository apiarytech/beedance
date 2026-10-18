/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func check(t *testing.T, src string) []Finding {
	t.Helper()
	fs, errs := CheckSource(src, Options{})
	if len(errs) > 0 {
		t.Fatalf("the source does not parse: %v", errs)
	}
	return fs
}

func dump(fs []Finding) string {
	var b strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&b, "\n\t%s", f)
	}
	return b.String()
}

// expect checks that rule is reported at each row, and nowhere else.
func expect(t *testing.T, fs []Finding, rule string, rows ...int) {
	t.Helper()
	want := map[int]bool{}
	for _, r := range rows {
		want[r] = true
	}
	got := map[int]bool{}
	for _, f := range fs {
		if f.Rule == rule {
			got[f.Row] = true
		}
	}
	for r := range want {
		if !got[r] {
			t.Errorf("%s not reported at row %d; findings:%s", rule, r, dump(fs))
		}
	}
	for r := range got {
		if !want[r] {
			t.Errorf("%s reported at row %d, which it should not be; findings:%s", rule, r, dump(fs))
		}
	}
}

func TestUnusedVariables(t *testing.T) {
	fs := check(t, `FUNCTION_BLOCK FB
VAR_INPUT a, b : INT; END_VAR
VAR_OUTPUT q : INT; r : INT; END_VAR
VAR x : INT; y : INT; END_VAR
VAR_TEMP tmp : INT; END_VAR
q := a + y;
tmp := 1;
END_FUNCTION_BLOCK
VAR_GLOBAL unusedGlobal : INT; usedGlobal : INT; END_VAR
PROGRAM P
VAR_EXTERNAL usedGlobal : INT; END_VAR
VAR fb : FB; idle : TON; END_VAR
fb(a := usedGlobal);
END_PROGRAM
`)
	expect(t, fs, "unused-input", 2)
	expect(t, fs, "unset-output", 3)
	expect(t, fs, "unused-variable", 4, 9, 12)
	expect(t, fs, "unread-variable", 5)
}

func TestFunctionResultNeverSet(t *testing.T) {
	fs := check(t, `FUNCTION F : INT
VAR_INPUT a : INT; END_VAR
IF a > 0 THEN RETURN; END_IF;
END_FUNCTION
FUNCTION G : INT
VAR_INPUT a : INT; END_VAR
G := a;
END_FUNCTION
`)
	expect(t, fs, "unset-output", 1)
}

func TestUnreachableCode(t *testing.T) {
	fs := check(t, `FUNCTION F : INT
VAR_INPUT a : INT; END_VAR
F := a;
RETURN;
F := 2;
END_FUNCTION
PROGRAM P
VAR i, k : INT; END_VAR
IF FALSE THEN
  i := 1;
END_IF;
IF TRUE THEN i := 2; ELSE
  i := 3;
END_IF;
JMP skip;
i := 4;
skip:
i := 5;
WHILE FALSE DO
  i := 6;
END_WHILE;
FOR k := 10 TO 1 DO
  i := 7;
END_FOR;
i := F(a := i);
END_PROGRAM
`)
	expect(t, fs, "unreachable-code", 5, 10, 13, 16, 20, 23)
}

// An input declared CONSTANT is not a constant: its caller gives it.
func TestInputConstantIsNotConstant(t *testing.T) {
	fs := check(t, `FUNCTION F : INT
VAR_INPUT CONSTANT sign : INT := 0; END_VAR
VAR CONSTANT off : BOOL := FALSE; END_VAR
F := 1;
IF sign > 0 THEN F := 2; END_IF;
IF off THEN
  F := 3;
END_IF;
END_FUNCTION
`)
	expect(t, fs, "unreachable-code", 7)
}

func TestNarrowing(t *testing.T) {
	fs := check(t, `FUNCTION Half : INT
VAR_INPUT a : INT; END_VAR
Half := a / 2;
END_FUNCTION
TYPE Level : INT; END_TYPE
PROGRAM P
VAR i : INT; d : DINT; r : REAL; u : UINT; l : LREAL; b : BYTE; s : SINT; lv : Level; t : TON; END_VAR
i := d;
d := i;
u := i;
i := r;
r := l;
l := r;
i := 70000;
s := 100;
b := 300;
i := 2.5;
i := DINT_TO_INT(d);
r := i;
r := d;
lv := d;
i := Half(a := d);
t(IN := TRUE, PT := i);
i := Half(d);
d := i * 2 + d;
END_PROGRAM
`)
	expect(t, fs, "narrowing", 8, 10, 11, 12, 17, 20, 21, 22, 23, 24)
	expect(t, fs, "literal-range", 14, 16)
}

func TestMultipleWrites(t *testing.T) {
	fs := check(t, `VAR_GLOBAL g : INT; END_VAR
PROGRAM A
VAR_EXTERNAL g : INT; END_VAR
VAR q AT %QX0.0 : BOOL; END_VAR
q := TRUE;
IF g > 0 THEN q := FALSE; END_IF;
%QX0.1 := TRUE;
g := 1;
END_PROGRAM
PROGRAM B
VAR_EXTERNAL g : INT; END_VAR
g := 2;
%QX0.1 := FALSE;
END_PROGRAM
`)
	expect(t, fs, "multiple-output-writes", 6, 13)
	expect(t, fs, "global-writers", 12)
}

func TestLoops(t *testing.T) {
	fs := check(t, `PROGRAM P
VAR i, k : INT; n, j : DINT; x : BOOL; arr : ARRAY[0..9] OF INT; END_VAR
WHILE TRUE DO k := k + 1; END_WHILE;
WHILE TRUE DO k := k + 1; IF k > 10 THEN EXIT; END_IF; END_WHILE;
WHILE i < 10 DO arr[0] := i; END_WHILE;
REPEAT k := k + 1; UNTIL k > 5 END_REPEAT;
FOR i := 0 TO 32767 DO k := 1; END_FOR;
FOR i := 0 TO 10 BY 0 DO k := 1; END_FOR;
FOR j := 0 TO n DO k := 1; END_FOR;
FOR i := 0 TO 999 DO
  FOR k := 0 TO 999 DO x := TRUE; END_FOR;
END_FOR;
FOR i := 0 TO 9 DO i := i + 1; END_FOR;
FOR i := 0 TO 99 DO arr[i MOD 10] := i; END_FOR;
END_PROGRAM
`)
	var errs, warns []int
	for _, f := range fs {
		if f.Rule != "unbounded-loop" {
			continue
		}
		if f.Severity == Error {
			errs = append(errs, f.Row)
		} else {
			warns = append(warns, f.Row)
		}
	}
	if fmt.Sprint(errs) != "[3 5 7 8]" || fmt.Sprint(warns) != "[4 6]" {
		t.Errorf("unbounded-loop errors at rows %v, warnings at %v; want [3 5 7 8] and [4 6]; findings:%s", errs, warns, dump(fs))
	}
	expect(t, fs, "loop-iterations", 9, 11)
	expect(t, fs, "for-counter-modified", 13)
}

func TestGuidelines(t *testing.T) {
	fs := check(t, `VAR_GLOBAL shared : INT; END_VAR
PROGRAM P
VAR r : REAL; tm : TIME; tn : TON; x : BOOL; shared : INT; w : W; END_VAR
x := r = 1.0;
x := tm = T#1s;
tn(IN := x, PT := T#1s);
tn(IN := NOT x, PT := T#1s);
w(a := shared + Rec(n := 3));
x := r > 0.5;
END_PROGRAM
FUNCTION Rec : INT
VAR_INPUT n : INT; END_VAR
Rec := Rec(n := n - 1);
END_FUNCTION
FUNCTION_BLOCK W
VAR_INPUT a : INT; END_VAR
VAR_EXTERNAL shared : INT; END_VAR
a := 1;
shared := a;
END_FUNCTION_BLOCK
`)
	expect(t, fs, "float-equality", 4)
	expect(t, fs, "time-equality", 5)
	expect(t, fs, "instance-calls", 7)
	expect(t, fs, "shadowed-global", 3)
	expect(t, fs, "recursion", 13)
	expect(t, fs, "input-write", 18)
	expect(t, fs, "external-in-fb", 17)
}

// A program written to the guidelines has no warnings and no errors.
func TestCleanProgram(t *testing.T) {
	fs := check(t, `FUNCTION_BLOCK Blink
VAR_INPUT run : BOOL; period : TIME; END_VAR
VAR_OUTPUT q : BOOL; END_VAR
VAR t : TON; END_VAR
t(IN := run AND NOT t.Q, PT := period);
IF t.Q THEN q := NOT q; ELSE q := q; END_IF;
END_FUNCTION_BLOCK
PROGRAM Main
VAR b : Blink; lamp AT %QX0.0 : BOOL; level : REAL; count : INT; END_VAR
b(run := TRUE, period := T#500ms);
lamp := b.q;
FOR count := 1 TO 10 DO level := level * 0.5 + INT_TO_REAL(count); END_FOR;
END_PROGRAM
`)
	for _, f := range fs {
		if f.Severity >= Warning {
			t.Errorf("unexpected %s", f)
		}
	}
}

// The parser lowers LD and FBD to ST, and the analyzer reads SFC's own
// elements: their variables are used.
func TestGraphicalBodies(t *testing.T) {
	fs := check(t, `PROGRAM Lad
VAR Start, Stop, Run, Done, Fault : BOOL; Mode : INT; END_VAR
LD
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S) ( Done )
  RUNG cmp
    EQ(Mode, 2) ( S Fault )
END_LD
END_PROGRAM
PROGRAM Chart
VAR_OUTPUT failures : INT; done : BOOL; n : INT; END_VAR
ACTION Count: n := n + 1; END_ACTION
ACTION Finish: done := TRUE; END_ACTION
INITIAL_STEP S0: Count(N); END_STEP
TRANSITION FROM S0 TO S1 := n >= 2; END_TRANSITION
STEP S1: Finish(N); END_STEP
END_PROGRAM
`)
	for _, f := range fs {
		if f.Rule == "unused-variable" {
			t.Errorf("unexpected %s", f)
		}
	}
}

func TestDisableAndRules(t *testing.T) {
	src := "PROGRAM P\nVAR i : INT; d : DINT; END_VAR\ni := d;\nEND_PROGRAM\n"
	fs, _ := CheckSource(src, Options{Disable: map[string]bool{"narrowing": true}})
	for _, f := range fs {
		if f.Rule == "narrowing" {
			t.Errorf("a disabled rule reported: %s", f)
		}
	}
	ids := map[string]bool{}
	for _, r := range Rules() {
		if ids[r.ID] {
			t.Errorf("rule %s listed twice", r.ID)
		}
		ids[r.ID] = true
	}
	if _, errs := CheckSource("PROGRAM P x := ; END_PROGRAM", Options{}); len(errs) == 0 {
		t.Error("a source that does not parse gave no parser errors")
	}
}

// Every example in the repository is analyzed without a panic.
func TestExamples(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "examples", "*", "*.st"))
	if len(files) == 0 {
		t.Skip("no examples")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		CheckSource(string(src), Options{})
	}
}
