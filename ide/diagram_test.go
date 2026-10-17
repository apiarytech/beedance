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

const ldSrc = `PROGRAM Lad
VAR Start, Stop, Run, Done, Fault : BOOL; Mode : INT; END_VAR
LD
  RUNG sealin (* start, hold, stop *)
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S) ( Done )
  RUNG cmp
    EQ(Mode, 2) ( S Fault )
END_LD
END_PROGRAM
`

const fbdSrc = `PROGRAM Blocks
VAR L, SP, Speed : REAL; High : BOOL; END_VAR
FBD
  above = GE(L, SP)
  hold : TON(IN := above, PT := T#3S)
  High := OR(hold.Q, High)
  Speed := SEL(High, 0.0, L)
END_FBD
END_PROGRAM
`

const sfcSrc = `PROGRAM Seq
VAR go, done : BOOL; n : INT; END_VAR
ACTION Count: n := n + 1; END_ACTION
INITIAL_STEP Idle: END_STEP
TRANSITION FROM Idle TO Busy := go; END_TRANSITION
STEP Busy: Count(N); END_STEP
TRANSITION FROM Busy TO Idle := done; END_TRANSITION
END_PROGRAM
`

// pos is where the n-th (1-based) occurrence of word starts in s.
func pos(t *testing.T, s, word string, n int) Pos {
	t.Helper()
	from := 0
	for k := 1; ; k++ {
		i := strings.Index(s[from:], word)
		if i < 0 {
			t.Fatalf("%q #%d not in the source", word, n)
		}
		i += from
		if k == n {
			return Pos{1 + strings.Count(s[:i], "\n"), i - strings.LastIndex(s[:i], "\n")}
		}
		from = i + len(word)
	}
}

func TestLDModel(t *testing.T) {
	ix := Analyze(ldSrc)
	ds := ix.Diagrams()
	if len(ds) != 1 || ds[0].Lang != "LD" || ds[0].POU != "Lad" || ds[0].Error != "" {
		t.Fatalf("diagrams = %+v", ds)
	}
	d := ds[0]
	if len(d.Rungs) != 3 || d.Rungs[0].Name != "sealin" || d.Rungs[2].Name != "cmp" {
		t.Fatalf("rungs = %+v", d.Rungs)
	}
	var kinds []string
	for _, e := range d.Rungs[0].Elements {
		kinds = append(kinds, e.Kind+":"+e.Mode+e.Name)
	}
	if strings.Join(kinds, " ") != "branch: contact:/Stop coil:Run" {
		t.Errorf("rung sealin = %v", kinds)
	}
	br := d.Rungs[0].Elements[0]
	if len(br.Legs) != 2 || br.Legs[0][0].Name != "Start" || br.Legs[1][0].Name != "Run" || br.Text != "[ Start | Run ]" {
		t.Errorf("the branch = %+v", br)
	}
	blk := d.Rungs[1].Elements[1]
	if blk.Kind != "block" || blk.Name != "t1" || blk.Type != "TON" || len(blk.Args) != 1 || blk.Args[0].Pin != "PT" || blk.Args[0].Value != "T#5S" {
		t.Errorf("the block = %+v %+v", blk, blk.Args)
	}
	fn := d.Rungs[2].Elements[0]
	coil := d.Rungs[2].Elements[1]
	if fn.Kind != "function" || fn.Type != "EQ" || len(fn.Args) != 2 || fn.Args[1].Pin != "2" || coil.Mode != "S" || coil.Name != "Fault" {
		t.Errorf("rung cmp = %+v, %+v", fn, coil)
	}
	if want := pos(t, ldSrc, "/Stop", 1); d.Rungs[0].Elements[1].At != want {
		t.Errorf("/Stop at %v, want %v", d.Rungs[0].Elements[1].At, want)
	}
	// The drawing names each element's place.
	for _, e := range append(d.Rungs[0].Elements[1:], d.Rungs[1].Elements...) {
		if e.Ref == "" || !strings.Contains(d.SVG, `data-at="`+e.Ref+`"`) {
			t.Errorf("the SVG does not name %s at %q", e.Text, e.Ref)
		}
	}
}

func TestLDGestures(t *testing.T) {
	ix := Analyze(ldSrc)
	tests := []struct {
		name string
		g    Gesture
		old  string // the line before
		want string // and after
	}{
		{"a contact after", Gesture{Op: "ld.contact", At: pos(t, ldSrc, "/Stop", 1), Name: "Fault", Mode: "/"},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] /Stop /Fault ( Run )"},
		{"a contact before", Gesture{Op: "ld.contact", At: pos(t, ldSrc, "/Stop", 1), Where: "before", Name: "Done"},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] Done /Stop ( Run )"},
		{"a leg added to a branch", Gesture{Op: "ld.contact", At: pos(t, ldSrc, "[ Start", 1), Where: "parallel", Name: "Done"},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run | Done ] /Stop ( Run )"},
		{"a contact in parallel", Gesture{Op: "ld.contact", At: pos(t, ldSrc, "/Stop", 1), Where: "parallel", Name: "Fault"},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] [ /Stop | Fault ] ( Run )"},
		{"negated", Gesture{Op: "ld.mode", At: pos(t, ldSrc, "/Stop", 1)},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] Stop ( Run )"},
		{"a set coil reset", Gesture{Op: "ld.mode", At: pos(t, ldSrc, "( S Fault", 1), Mode: "R"},
			"EQ(Mode, 2) ( S Fault )", "EQ(Mode, 2) ( R Fault )"},
		{"a contact renamed", Gesture{Op: "ld.rename", At: pos(t, ldSrc, "/Stop", 1), Name: "Fault"},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] /Fault ( Run )"},
		{"a block renamed", Gesture{Op: "ld.rename", At: pos(t, ldSrc, "t1:TON", 1), Name: "delay1"},
			"Run t1:TON(PT := T#5S) ( Done )", "Run delay1:TON(PT := T#5S) ( Done )"},
		{"a leg's contact deleted", Gesture{Op: "ld.delete", At: pos(t, ldSrc, "Run ]", 1)},
			"[ Start | Run ] /Stop ( Run )", "Start /Stop ( Run )"},
		{"a contact deleted", Gesture{Op: "ld.delete", At: pos(t, ldSrc, "/Stop", 1)},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] ( Run )"},
		{"a coil added", Gesture{Op: "ld.coil", At: pos(t, ldSrc, "RUNG delay", 1), Name: "Fault", Mode: "S"},
			"Run t1:TON(PT := T#5S) ( Done )", "Run t1:TON(PT := T#5S) ( Done ) ( S Fault )"},
		{"a block added", Gesture{Op: "ld.block", At: pos(t, ldSrc, "/Stop", 1), Name: "c1", Type: "CTU", Text: "PV := 10"},
			"[ Start | Run ] /Stop ( Run )", "[ Start | Run ] /Stop c1:CTU(PV := 10) ( Run )"},
		{"a function contact negated", Gesture{Op: "ld.mode", At: pos(t, ldSrc, "EQ(", 1), Mode: "/"},
			"EQ(Mode, 2) ( S Fault )", "/EQ(Mode, 2) ( S Fault )"},
		{"a range in parallel", Gesture{Op: "ld.parallel", At: pos(t, ldSrc, "[ Start", 1), To: pos(t, ldSrc, "/Stop", 1), Name: "Fault"},
			"[ Start | Run ] /Stop ( Run )", "[ [ Start | Run ] /Stop | Fault ] ( Run )"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ix.Apply(tt.g)
			if err != nil {
				t.Fatal(err)
			}
			line := func(s string) string { return "\n    " + s + "\n" }
			if !strings.Contains("\n"+r.Source, line(tt.want)) || strings.Count("\n"+r.Source, line(tt.old)) != strings.Count("\n"+ldSrc, line(tt.old))-1 {
				t.Errorf("got\n%s\nwant the line %q", r.Source, tt.want)
			}
		})
	}
}

func TestLDRungGestures(t *testing.T) {
	ix := Analyze(ldSrc)
	r, err := ix.Apply(Gesture{Op: "ld.rung", At: pos(t, ldSrc, "RUNG delay", 1), Name: "alarm", Text: "Fault ( Done )"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Source, "( Done )\n  RUNG alarm\n    Fault ( Done )\n  RUNG cmp") {
		t.Errorf("a rung added:\n%s", r.Source)
	}
	r, err = ix.Apply(Gesture{Op: "ld.deleteRung", At: pos(t, ldSrc, "RUNG delay", 1)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Source, "delay") || strings.Contains(r.Source, "t1:TON") || !strings.Contains(r.Source, "( Run )\n  RUNG cmp") {
		t.Errorf("a rung deleted:\n%s", r.Source)
	}
	r, err = ix.Apply(Gesture{Op: "ld.moveRung", At: pos(t, ldSrc, "RUNG delay", 1), Where: "up"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Source, "LD\n  RUNG delay\n    Run t1:TON(PT := T#5S) ( Done )\n  RUNG sealin (* start, hold, stop *)\n    [ Start | Run ] /Stop ( Run )\n  RUNG cmp") {
		t.Errorf("a rung moved up:\n%s", r.Source)
	}
	if len(r.Edits) != 1 {
		t.Errorf("%d edits; a move is one", len(r.Edits))
	}
}

func TestLDRefused(t *testing.T) {
	ix := Analyze(ldSrc)
	tests := []struct {
		name string
		g    Gesture
		want string
	}{
		{"no element", Gesture{Op: "ld.mode", At: Pos{5, 1}}, "no element"},
		{"after a coil", Gesture{Op: "ld.contact", At: pos(t, ldSrc, "( Run )", 1), Name: "X"}, "coils come last"},
		{"a bad name", Gesture{Op: "ld.contact", At: pos(t, ldSrc, "/Stop", 1), Name: "a b"}, "not a variable"},
		{"injected text", Gesture{Op: "ld.block", At: pos(t, ldSrc, "/Stop", 1), Name: "c", Type: "CTU", Text: "PV := 1); x := 1; (*"}, "has a"},
		{"a rung left without a coil or block", Gesture{Op: "ld.delete", At: pos(t, ldSrc, "( Run )", 1)}, "would make the source not read"},
		{"a bad coil mode", Gesture{Op: "ld.mode", At: pos(t, ldSrc, "( Run )", 1), Mode: "X"}, "a coil is"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ix.Apply(tt.g)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v; want %q", err, tt.want)
			}
		})
	}
}

func TestFBDModelAndGestures(t *testing.T) {
	ix := Analyze(fbdSrc)
	ds := ix.Diagrams()
	if len(ds) != 1 || ds[0].Lang != "FBD" || ds[0].Error != "" {
		t.Fatalf("diagrams = %+v", ds)
	}
	var kinds []string
	for _, s := range ds[0].Statements {
		kinds = append(kinds, s.Kind+":"+s.Target+":"+s.Call)
	}
	if strings.Join(kinds, " ") != "wire:above:GE call:hold:TON assign:High:OR assign:Speed:SEL" {
		t.Errorf("statements = %v", kinds)
	}
	tests := []struct {
		name      string
		g         Gesture
		old, want string
	}{
		{"an input set", Gesture{Op: "fbd.input", At: pos(t, fbdSrc, "hold :", 1), Pin: "PT", Text: "T#10S"},
			"hold : TON(IN := above, PT := T#3S)", "hold : TON(IN := above, PT := T#10S)"},
		{"a positional input set", Gesture{Op: "fbd.input", At: pos(t, fbdSrc, "above =", 1), Pin: "2", Text: "50.0"},
			"above = GE(L, SP)", "above = GE(L, 50.0)"},
		{"a positional input added", Gesture{Op: "fbd.input", At: pos(t, fbdSrc, "High :=", 1), Pin: "3", Text: "above"},
			"High := OR(hold.Q, High)", "High := OR(hold.Q, High, above)"},
		{"a value negated", Gesture{Op: "fbd.negate", At: pos(t, fbdSrc, "High :=", 1)},
			"High := OR(hold.Q, High)", "High := NOT(OR(hold.Q, High))"},
		{"an input negated", Gesture{Op: "fbd.negate", At: pos(t, fbdSrc, "hold :", 1), Pin: "IN"},
			"hold : TON(IN := above, PT := T#3S)", "hold : TON(IN := NOT(above), PT := T#3S)"},
		{"a target renamed", Gesture{Op: "fbd.target", At: pos(t, fbdSrc, "above =", 1), Name: "over"},
			"above = GE(L, SP)", "over = GE(L, SP)"},
		{"a statement added", Gesture{Op: "fbd.statement", At: pos(t, fbdSrc, "above =", 1), Text: "slow = LT(L, 1.0)"},
			"above = GE(L, SP)\n  hold", "above = GE(L, SP)\n  slow = LT(L, 1.0)\n  hold"},
		{"a statement moved down", Gesture{Op: "fbd.move", At: pos(t, fbdSrc, "High :=", 1), Where: "down"},
			"High := OR(hold.Q, High)\n  Speed", "Speed := SEL(High, 0.0, L)\n  High := OR(hold.Q, High)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ix.Apply(tt.g)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(r.Source, tt.want) {
				t.Errorf("got\n%s\nwant %q", r.Source, tt.want)
			}
		})
	}
	if _, err := ix.Apply(Gesture{Op: "fbd.input", At: pos(t, fbdSrc, "above =", 1), Pin: "4", Text: "x"}); err == nil {
		t.Error("a 4th input of a 2-input call was not refused")
	}
	if _, err := ix.Apply(Gesture{Op: "fbd.statement", Text: "x := 1; y := 2"}); err == nil {
		t.Error("two statements in one gesture were not refused")
	}
}

func TestSFCModelAndGestures(t *testing.T) {
	ix := Analyze(sfcSrc)
	ds := ix.Diagrams()
	if len(ds) != 1 || ds[0].Lang != "SFC" {
		t.Fatalf("diagrams = %+v", ds)
	}
	d := ds[0]
	if len(d.Steps) != 2 || !d.Steps[0].Initial || d.Steps[1].Name != "Busy" || len(d.Steps[1].Actions) != 1 || d.Steps[1].Actions[0].Qualifier != "N" {
		t.Errorf("steps = %+v", d.Steps)
	}
	if len(d.Transits) != 2 || d.Transits[0].Condition != "go" || d.Transits[1].From[0] != "Busy" {
		t.Errorf("transitions = %+v", d.Transits)
	}
	if len(d.Actions) != 1 || d.Actions[0].Name != "Count" || !strings.Contains(d.SVG, "Idle") {
		t.Errorf("actions = %+v", d.Actions)
	}
	tests := []struct {
		name string
		g    Gesture
		want []string // lines that must be there
		gone []string // and not
	}{
		{"a condition", Gesture{Op: "sfc.condition", At: pos(t, sfcSrc, "TRANSITION FROM Idle", 1), Text: "go AND NOT done"},
			[]string{"TRANSITION FROM Idle TO Busy := go AND NOT done; END_TRANSITION"}, nil},
		{"a step after Busy", Gesture{Op: "sfc.step", At: pos(t, sfcSrc, "STEP Busy", 1), Name: "Wait", Text: "n > 3"},
			[]string{"STEP Busy: Count(N); END_STEP\nTRANSITION FROM Busy TO Wait := n > 3; END_TRANSITION\nSTEP Wait: END_STEP\n"}, nil},
		{"made initial", Gesture{Op: "sfc.initial", At: pos(t, sfcSrc, "STEP Busy", 1)},
			[]string{"INITIAL_STEP Busy:", "\nSTEP Idle:"}, nil},
		{"an action qualified", Gesture{Op: "sfc.qualify", At: pos(t, sfcSrc, "INITIAL_STEP Idle", 1), Name: "Count", Mode: "P"},
			[]string{"INITIAL_STEP Idle: Count(P); END_STEP"}, nil},
		{"a qualifier changed", Gesture{Op: "sfc.qualify", At: pos(t, sfcSrc, "STEP Busy", 1), Name: "Count", Mode: "L", Text: "T#2S"},
			[]string{"STEP Busy: Count(L, T#2S); END_STEP"}, nil},
		{"an action unqualified", Gesture{Op: "sfc.unqualify", At: pos(t, sfcSrc, "STEP Busy", 1), Name: "Count"},
			[]string{"STEP Busy: END_STEP"}, nil},
		{"a step renamed everywhere", Gesture{Op: "rename", At: pos(t, sfcSrc, "Busy", 1), Name: "Working"},
			[]string{"TO Working :=", "STEP Working:", "FROM Working TO"}, []string{"Busy"}},
		{"a step deleted with its transitions", Gesture{Op: "sfc.delete", At: pos(t, sfcSrc, "STEP Busy", 1)},
			nil, []string{"Busy"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ix.Apply(tt.g)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(r.Source, w) {
					t.Errorf("got\n%s\nlacks %q", r.Source, w)
				}
			}
			for _, w := range tt.gone {
				if strings.Contains(r.Source, w) {
					t.Errorf("got\n%s\nstill has %q", r.Source, w)
				}
			}
		})
	}
	if _, err := ix.Apply(Gesture{Op: "sfc.qualify", At: pos(t, sfcSrc, "STEP Busy", 1), Name: "Count", Mode: "D"}); err == nil {
		t.Error("D without a time was not refused")
	}
}

// Line endings are kept: a CRLF source gets CRLF lines.
func TestCRLF(t *testing.T) {
	src := strings.ReplaceAll(ldSrc, "\n", "\r\n")
	ix := Analyze(src)
	r, err := ix.Apply(Gesture{Op: "ld.rung", At: pos(t, src, "RUNG cmp", 1), Name: "x", Text: "Start ( Done )"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Source, "( S Fault )\r\n  RUNG x\r\n    Start ( Done )\r\nEND_LD") {
		t.Errorf("got %q", r.Source)
	}
}

// An element on the LD keyword's own line is drawn at its column in the
// source, as the model places it.
func TestElementsOnTheKeywordLine(t *testing.T) {
	src := "PROGRAM P\nVAR a, b, c : BOOL; END_VAR\nLD RUNG one a /b ( c )\n  RUNG two\n    c ( a )\nEND_LD\nEND_PROGRAM\n"
	d := Analyze(src).Diagrams()[0]
	if d.Error != "" || len(d.Rungs) != 2 {
		t.Fatalf("diagram = %+v", d)
	}
	for _, r := range d.Rungs {
		for _, e := range r.Elements {
			if !strings.Contains(d.SVG, `data-at="`+e.Ref+`"`) {
				t.Errorf("%s at %q is not in the drawing", e.Text, e.Ref)
			}
		}
	}
	if want := pos(t, src, "/b", 1); d.Rungs[0].Elements[1].At != want {
		t.Errorf("/b at %v, want %v", d.Rungs[0].Elements[1].At, want)
	}
}
