/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ide

import (
	"strings"
	"testing"
)

const src = `TYPE Point : STRUCT x : INT; y : INT; END_STRUCT; END_TYPE
TYPE Mode : (Idle, Running); END_TYPE
VAR_GLOBAL Level : REAL; END_VAR
FUNCTION_BLOCK Base
VAR_INPUT Enable : BOOL; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK Motor EXTENDS Base
VAR_INPUT Start, Stop : BOOL; (* push buttons *) END_VAR
VAR_OUTPUT Run : BOOL; END_VAR
VAR t1 : TON; p : Point; pts : ARRAY[1..3] OF Point; END_VAR
METHOD Reset : BOOL
VAR_INPUT hard : BOOL; END_VAR
Run := FALSE;
Reset := hard AND Enable;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM Main
VAR m : Motor; n : INT; md : Mode; END_VAR
m(Start := Level > 1.0, Stop := FALSE);
n := m.p.x + m.pts[2].y;
IF m.t1.Q THEN md := Mode#Running; END_IF;
n := ABS(n);
END_PROGRAM
PROGRAM Ladder
VAR a, b : BOOL; END_VAR
LD
  RUNG r1
    a d1:TON(PT := T#2S) ( b )
END_LD
END_PROGRAM
`

// at is the position of the n-th (1-based) occurrence of word in src,
// plus off bytes.
func at(t *testing.T, word string, n, off int) Pos {
	t.Helper()
	from := 0
	for k := 1; ; k++ {
		i := strings.Index(src[from:], word)
		if i < 0 {
			t.Fatalf("%q #%d not in the source", word, n)
		}
		i += from
		if k == n {
			i += off
			row := 1 + strings.Count(src[:i], "\n")
			return Pos{row, i - strings.LastIndex(src[:i], "\n")}
		}
		from = i + len(word)
	}
}

func TestDefinition(t *testing.T) {
	ix := Analyze(src)
	if d := ix.Diagnostics(); len(d) > 0 {
		t.Fatalf("the source does not parse: %v", d)
	}
	tests := []struct {
		name     string
		use      Pos
		declWord string
		declN    int
	}{
		{"a local", at(t, "n := m.p", 1, 0), "n : INT", 1},
		{"a global", at(t, "Level > 1.0", 1, 0), "Level : REAL", 1},
		{"a named argument: the block's input", at(t, "Start := Level", 1, 0), "Start, Stop", 1},
		{"a member of a member", at(t, "m.p.x", 1, 4), "x : INT", 1},
		{"an array element's member", at(t, "m.pts[2].y", 1, 9), "y : INT", 1},
		{"a type", at(t, "m : Motor", 1, 4), "Motor EXTENDS", 1},
		{"an inherited input in a method", at(t, "AND Enable", 1, 4), "Enable : BOOL", 1},
		{"a method's own input", at(t, "hard AND", 1, 0), "hard : BOOL", 1},
		{"an enum value", at(t, "Mode#Running", 1, 5), "Running);", 1},
		{"an instance a ladder rung declares", at(t, "d1:TON", 1, 0), "d1:TON", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ix.Definition(tt.use)
			want := at(t, tt.declWord, tt.declN, 0)
			if !ok || got.Start != want {
				t.Errorf("Definition(%v) = %v, %v; want %v", tt.use, got, ok, want)
			}
		})
	}
}

func TestStandardNames(t *testing.T) {
	ix := Analyze(src)
	sym, _, ok := ix.SymbolAt(at(t, "Q THEN", 1, 0))
	if !ok || sym.Name != "Q" || !sym.Standard || sym.Container != "TON" {
		t.Errorf("t1.Q = %+v, %v; want TON's standard output Q", sym, ok)
	}
	if _, ok := ix.Definition(at(t, "Q THEN", 1, 0)); ok {
		t.Error("a standard output has a definition in the source")
	}
	sym, _, ok = ix.SymbolAt(at(t, "ABS(n)", 1, 0))
	if !ok || sym.Kind != Builtin {
		t.Errorf("ABS = %+v, %v; want the standard function", sym, ok)
	}
	sym, _, ok = ix.SymbolAt(at(t, "PT := T#2S", 1, 0))
	if !ok || sym.Name != "PT" || sym.Container != "TON" {
		t.Errorf("PT in d1:TON(PT := ...) = %+v, %v; want TON's input", sym, ok)
	}
}

func TestReferencesAndRename(t *testing.T) {
	ix := Analyze(src)
	refs := ix.References(at(t, "Run := FALSE", 1, 0), true)
	if len(refs) != 2 || refs[0].Start != at(t, "Run : BOOL", 1, 0) {
		t.Errorf("References(Run) = %v; want its declaration and its use", refs)
	}
	if refs := ix.References(at(t, "Run := FALSE", 1, 0), false); len(refs) != 1 {
		t.Errorf("References(Run) without the declaration = %v", refs)
	}
	edits, err := ix.Rename(at(t, "m : Motor", 1, 0), "motor1")
	if err != nil || len(edits) != 5 {
		t.Errorf("Rename(m) = %v, %v; want 5 places", edits, err)
	}
	if _, err := ix.Rename(at(t, "m : Motor", 1, 0), "n"); err == nil || !strings.Contains(err.Error(), "already names") {
		t.Errorf("Rename(m, n) = %v; want refused: n is taken", err)
	}
	if _, err := ix.Rename(at(t, "m : Motor", 1, 0), "END_IF"); err == nil {
		t.Error("Rename to a keyword was not refused")
	}
	if _, err := ix.Rename(at(t, "ABS(n)", 1, 0), "Abs2"); err == nil {
		t.Error("Rename of a standard function was not refused")
	}
}

func TestHover(t *testing.T) {
	ix := Analyze(src)
	h, _, ok := ix.Hover(at(t, "Stop := FALSE", 1, 0))
	if !ok || !strings.Contains(h, "**Stop** — VAR_INPUT of Motor") || !strings.Contains(h, "(* push buttons *)") {
		t.Errorf("Hover(Stop) = %q", h)
	}
}

func TestComplete(t *testing.T) {
	ix := Analyze(src)
	labels := func(cs []Completion) string {
		var s []string
		for _, c := range cs {
			s = append(s, c.Label)
		}
		return strings.Join(s, " ")
	}
	// After "m.", Motor's members and Base's.
	got := labels(ix.Complete(at(t, "m.p.x", 1, 2)))
	for _, w := range []string{"Start", "Stop", "Run", "t1", "p", "pts", "Reset", "Enable"} {
		if !strings.Contains(" "+got+" ", " "+w+" ") {
			t.Errorf("Complete after m. = %q; lacks %s", got, w)
		}
	}
	// After "m.t1.", TON's.
	if got := labels(ix.Complete(at(t, "m.t1.Q", 1, 5))); got != "IN PT Q ET" {
		t.Errorf("Complete after m.t1. = %q", got)
	}
	// A word begun: "Le" in Main is Level, the global.
	if got := labels(ix.Complete(at(t, "Level > 1.0", 1, 2))); !strings.HasPrefix(got, "Level") {
		t.Errorf("Complete(Le) = %q", got)
	}
}

func TestOutline(t *testing.T) {
	ix := Analyze(src)
	var names []string
	for _, s := range ix.Outline() {
		names = append(names, string(s.Kind)+":"+s.Name)
	}
	want := "type:Point type:Mode variable:Level function_block:Base function_block:Motor program:Main program:Ladder"
	if strings.Join(names, " ") != want {
		t.Errorf("Outline = %v; want %s", names, want)
	}
	var motor []string
	for _, c := range ix.Outline()[4].Children {
		motor = append(motor, c.Name)
	}
	if strings.Join(motor, " ") != "Start Stop Run t1 p pts Reset" {
		t.Errorf("Motor's children = %v", motor)
	}
	var ladder []string
	for _, c := range ix.Outline()[6].Children {
		ladder = append(ladder, c.Name)
	}
	if strings.Join(ladder, " ") != "a b d1" {
		t.Errorf("Ladder's children = %v (the edge detectors a diagram generates are left out)", ladder)
	}
}

// A source with errors still answers for what parses.
func TestBrokenSource(t *testing.T) {
	broken := "PROGRAM P\nVAR x : INT; END_VAR\nx := x + ;\nEND_PROGRAM\n"
	ix := Analyze(broken)
	if len(ix.Diagnostics()) == 0 {
		t.Fatal("no error in a broken source")
	}
	if d, ok := ix.Definition(Pos{3, 1}); !ok || d.Start != (Pos{2, 5}) {
		t.Errorf("Definition(x) in a broken source = %v, %v", d, ok)
	}
}

// TestAnalyzeMalformedMethod: ide.Analyze("method") must not panic (issue #9).
func TestAnalyzeMalformedMethod(t *testing.T) {
	for _, src := range []string{"method", "PROGRAM P\nmethod\nEND_PROGRAM", "FUNCTION A method"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Analyze(%q) panicked: %v", src, r)
				}
			}()
			Analyze(src)
		}()
	}
}
