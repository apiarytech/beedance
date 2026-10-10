/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram_test

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/diagram"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// sfcSample has a selection, a simultaneous branch and a loop back to the
// initial step.
const sfcSample = `
PROGRAM Press
VAR Go, Up, Down, Clamped, Fault : BOOL; Strokes : INT; END_VAR
ACTION Count: Strokes := Strokes + 1; END_ACTION
INITIAL_STEP Ready: END_STEP
STEP Lower: Down(N); Count(P); END_STEP
STEP Clamp: Clamped(S); END_STEP
STEP Raise: Up(N); Clamped(R); END_STEP
STEP Alarm: Fault(SD, T#500ms); END_STEP
TRANSITION FROM Ready TO (Lower, Clamp) := Go; END_TRANSITION
TRANSITION FROM (Lower, Clamp) TO Raise := Strokes > 3; END_TRANSITION
TRANSITION FROM Raise TO Ready := NOT Go; END_TRANSITION
TRANSITION FROM Raise TO Alarm := Go AND Fault; END_TRANSITION
TRANSITION FROM Alarm TO Ready := NOT Fault; END_TRANSITION
END_PROGRAM
`

func parseChart(t *testing.T, src string) *diagram.Chart {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v\n%s", errs, src)
	}
	for _, st := range prog.Statements {
		if d, ok := st.(*ast.ProgramDeclaration); ok {
			if sfc, ok := d.Body.(*ast.SFCProgram); ok {
				return diagram.ChartOf(sfc)
			}
		}
	}
	t.Fatalf("no SFC body in\n%s", src)
	return nil
}

func TestSFCSVG(t *testing.T) {
	c := parseChart(t, sfcSample)
	svg := diagram.SFCSVG(c)
	got := texts(t, svg)
	for _, w := range []string{"Ready", "Lower", "Clamp", "Raise", "Alarm", "Down", "Count", "P", "S", "R", "SD ", "T#500ms", "Go", "NOT Go"} {
		if !slices.Contains(got, w) {
			t.Errorf("no text %q in %q", w, got)
		}
	}
	// The loops back to Ready are jumps, arrows naming it, not wires.
	if n := strings.Count(svg, `<path class="j"`); n != 2 {
		t.Errorf("%d jump arrows, want 2", n)
	}
	if n := strings.Count(svg, "<rect"); n != 5+1+6*2 { // 5 steps, the initial one doubled, 6 actions of two cells
		t.Errorf("%d boxes:\n%s", n, svg)
	}
	if strings.Contains(svg, "<script") || strings.Contains(svg, "href") {
		t.Error("a drawing has no scripts or links")
	}
	// SVG_OUT=dir writes the drawing as a page, to look at.
	if dir := os.Getenv("SVG_OUT"); dir != "" {
		page := "<html><body style='background:#fff;margin:0'>" + svg + "</body></html>"
		if err := os.WriteFile(filepath.Join(dir, "sfc.html"), []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Text writes what the parser reads back as the same chart.
func TestSFCTextRoundTrip(t *testing.T) {
	c := parseChart(t, sfcSample)
	c.Actions = []diagram.Action{{Name: "Count", Body: "Strokes := Strokes + 1;"}}
	text, err := c.Text("")
	if err != nil {
		t.Fatal(err)
	}
	again := parseChart(t, "PROGRAM Press\nVAR Go, Up, Down, Clamped, Fault : BOOL; Strokes : INT; END_VAR\n"+text+"END_PROGRAM\n")
	again.Actions = c.Actions
	a, _ := again.Text("")
	if a != text {
		t.Errorf("read back differently:\n%s\n---\n%s", text, a)
	}
}

func TestSFCTextRefused(t *testing.T) {
	for _, tt := range []struct {
		c    diagram.Chart
		want string
	}{
		{diagram.Chart{Steps: []diagram.Step{{Name: "A"}}}, "no initial step"},
		{diagram.Chart{Steps: []diagram.Step{{Name: "A", Initial: true}, {Name: "a"}}}, "two steps are named"},
		{diagram.Chart{Steps: []diagram.Step{{Name: "A B", Initial: true}}}, "not an identifier"},
		{diagram.Chart{Steps: []diagram.Step{{Name: "A", Initial: true}}, Transitions: []diagram.Transition{{From: []string{"A"}, To: []string{"B"}, Condition: "TRUE"}}}, "no step B"},
		{diagram.Chart{Steps: []diagram.Step{{Name: "A", Initial: true}}, Transitions: []diagram.Transition{{From: []string{"A"}, To: []string{"A"}}}}, "has no condition"},
	} {
		if _, err := tt.c.Text(""); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("expected an error containing %q, got %v", tt.want, err)
		}
	}
}

// A chart without an initial step, or with a transition to nowhere, still
// draws: a preview shows what is there.
func TestSFCSVGPartial(t *testing.T) {
	c := &diagram.Chart{
		Steps:       []diagram.Step{{Name: "A"}, {Name: "B"}},
		Transitions: []diagram.Transition{{From: []string{"A"}, To: []string{"Nowhere"}, Condition: "x"}, {From: []string{"B"}, To: []string{"A"}, Condition: strings.Repeat("y AND ", 20) + "z"}},
	}
	got := texts(t, diagram.SFCSVG(c))
	if !slices.Contains(got, "A") || !slices.Contains(got, "B") {
		t.Errorf("texts %q", got)
	}
	for _, g := range got {
		if len([]rune(g)) > 48 {
			t.Errorf("a condition is not cut short: %q", g)
		}
	}
}

// texts returns an SVG's text contents, checking it is well-formed XML.
func texts(t *testing.T, svg string) []string {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(svg))
	var out []string
	depth := 0 // inside <text>, counting its <tspan>s
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("not well-formed: %v\n%s", err, svg)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "text" || depth > 0 {
				depth++
			}
		case xml.CharData:
			if depth > 0 {
				out = append(out, string(v))
			}
		case xml.EndElement:
			if depth > 0 {
				depth--
			}
		}
	}
}
