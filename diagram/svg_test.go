package diagram

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ldSample = `
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S, ET => Elapsed) ( Done ) ( S Latched )
  RUNG pulse
    +Button [ EQ(Mode, 2) | /Auto ] ( P Pulse )
`

const fbdSample = `
  above = GE(L, SP)
  hold : TON(IN := above, PT := T#3S)
  High := OR(hold.Q, AND(High, NOT(LT(L, SUB(SP, 5.0)))))
  Msg := CONCAT('a<b', "&")
`

// texts returns an SVG's text contents, checking it is well-formed XML.
func texts(t *testing.T, svg string) []string {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(svg))
	var out []string
	inText := false
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
			inText = v.Name.Local == "text"
		case xml.CharData:
			if inText {
				out = append(out, string(v))
			}
		case xml.EndElement:
			inText = false
		}
	}
}

func TestSVG(t *testing.T) {
	for _, c := range []struct {
		lang, src string
		want      []string
	}{
		{"LD", ldSample, []string{"RUNG sealin", "Start", "Stop", "TON", "t1", "Elapsed", "S", "P", "EQ", "EN", "ENO", "Auto"}},
		{"FBD", fbdSample, []string{"GE", "above", "hold", "TON", "OR", "NOT", "'a<b'", `"&"`, "CONCAT", "Msg"}},
	} {
		body, err := ParseText(c.lang, c.src, 1)
		if err != nil {
			t.Fatal(err)
		}
		svg := SVG(body.Elems)
		got := texts(t, svg)
		for _, w := range c.want {
			found := false
			for _, g := range got {
				if g == w {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no text %q in %q", c.lang, w, got)
			}
		}
		if strings.Contains(svg, "<script") || strings.Contains(svg, "href") {
			t.Errorf("%s: a drawing has no scripts or links", c.lang)
		}
		// SVG_OUT=dir writes the drawings as pages, to look at.
		if dir := os.Getenv("SVG_OUT"); dir != "" {
			page := "<html><body style='background:#fff;margin:0'>" + svg + "</body></html>"
			if err := os.WriteFile(filepath.Join(dir, strings.ToLower(c.lang)+".html"), []byte(page), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// A PLCopen XML body draws too, from its elements.
func TestSVGFromXML(t *testing.T) {
	inner := `<leftPowerRail localId="1"><position x="0" y="0"/></leftPowerRail>
<contact localId="2"><position x="50" y="20"/><connectionPointIn><connection refLocalId="1"/></connectionPointIn><variable>A</variable></contact>
<coil localId="3" negated="true"><position x="150" y="20"/><connectionPointIn><connection refLocalId="2"/></connectionPointIn><variable>Q</variable></coil>`
	elems, err := ParseXML(inner)
	if err != nil {
		t.Fatal(err)
	}
	got := texts(t, SVG(elems))
	if strings.Join(got, " ") != "A / Q" {
		t.Errorf("texts %q", got)
	}
}

// Contacts, coils and variables name their variable, so a page can show
// them live (drone L-4); a negated one says so.
func TestSVGNamesLiveVariables(t *testing.T) {
	body, err := ParseText("LD", "RUNG r\n [ Start | Run ] /Stop ( Run )\n", 1)
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(body.Elems)
	for _, want := range []string{
		`<g class="el contact" data-var="Start" data-neg="false">`,
		`<g class="el contact" data-var="Stop" data-neg="true">`,
		`<g class="el coil" data-var="Run" data-neg="false">`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s in\n%s", want, svg)
		}
	}
	if strings.Count(svg, "<g ") != strings.Count(svg, "</g>") {
		t.Error("groups not closed")
	}
}

// A block's pins show their values at run time (drone L-38): an instance's
// inputs and outputs, and a variable written inside a block.
func TestSVGValueSlots(t *testing.T) {
	body, err := ParseText("LD", "\n  RUNG high_delay\n    GT(WellLevel, 85.0) t1:TON(PT := T#5S) ( HighAlarm )\n", 1)
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(body.Elems)
	for _, want := range []string{`data-vals="=WellLevel"`, `data-vals="=t1.IN"`, `data-vals="=t1.Q"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s in %s", want, svg)
		}
	}
	if strings.Contains(svg, `data-vals="=t1.EN`) || strings.Contains(svg, `data-vals="=85.0"`) {
		t.Error("a slot for power flow or a literal")
	}
}

// TestSVGWires: each drawn wire names the value it carries, as the
// lowering computes it (drone L-4: power flow drawn from live values).
func TestSVGWires(t *testing.T) {
	body, err := ParseText("LD", ldSample, 1)
	if err != nil {
		t.Fatal(err)
	}
	low, err := LowerText("Main", "LD", ldSample, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	svg := SVGWires(body.Elems, low.Wires)
	texts(t, svg) // well-formed
	var got []string
	for _, m := range strings.Split(svg, `data-expr="`)[1:] {
		got = append(got, m[:strings.IndexByte(m, '"')])
	}
	for _, want := range []string{"TRUE", "Start", "Run", "(Start OR Run) AND NOT(Stop)", "t1.Q", "_R_TRIG1.Q", "(_R_TRIG1.Q AND (Mode = 2))"} {
		found := false
		for _, g := range got {
			found = found || g == want
		}
		if !found {
			t.Errorf("no wire carries %q; wires: %q", want, got)
		}
	}
	if strings.Count(svg, "<polyline") != len(got) {
		t.Errorf("%d wires, %d named", strings.Count(svg, "<polyline"), len(got))
	}
	if SVG(body.Elems) == svg || strings.Contains(SVG(body.Elems), "data-expr") {
		t.Error("SVG names the wires too")
	}
}

func TestSVGWiresFBD(t *testing.T) {
	body, err := ParseText("FBD", fbdSample, 1)
	if err != nil {
		t.Fatal(err)
	}
	low, err := LowerText("Main", "FBD", fbdSample, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	svg := SVGWires(body.Elems, low.Wires)
	texts(t, svg)
	if n, named := strings.Count(svg, "<polyline"), strings.Count(svg, "data-expr="); n == 0 || n != named {
		t.Errorf("%d wires, %d named:\n%s", n, named, svg)
	}
	if !strings.Contains(svg, `data-expr="hold.Q"`) {
		t.Error("no wire from hold.Q")
	}
}

// TestSVGDataValues: every block shows its outputs (a function's result,
// held by no variable, as the expression the page works out), an analog
// result shows again at the input it goes into, output variables show
// their values, and timers and counters their progress (drone L-39, L-40).
func TestSVGDataValues(t *testing.T) {
	draw := func(lang, src string) string {
		t.Helper()
		body, err := ParseText(lang, src, 1)
		if err != nil {
			t.Fatal(err)
		}
		low, err := LowerText("Main", lang, src, 1, Options{})
		if err != nil {
			t.Fatal(err)
		}
		svg := SVGWires(body.Elems, low.Wires)
		texts(t, svg)
		if strings.Contains(SVG(body.Elems), "data-val-expr") {
			t.Error("SVG has value slots")
		}
		return svg
	}
	fbd := draw("FBD", fbdSample)
	// SUB's result at its output and at LT's input; a timer's output is
	// its own value, never an expression; output variables show theirs
	if n := strings.Count(fbd, `data-val-expr="(SP - 5.0)"`); n != 2 {
		t.Errorf("SUB's result shown %d times, want 2:\n%s", n, fbd)
	}
	for _, want := range []string{`data-val-expr="(L &gt;= SP)"`, `class="el pin" data-var="hold.Q"`, `data-vals="=hold.Q"`, `data-vals="=High"`, `data-vals="=Msg"`, `data-prog="hold.ET hold.PT"`} {
		if !strings.Contains(fbd, want) {
			t.Errorf("no %s", want)
		}
	}
	// every data input of a function has its value (L-41): a timer's Q into
	// OR, a comparison's BOOL into NOT
	for _, want := range []string{`data-val-expr="hold.Q"`, `data-val-expr="(L &lt; (SP - 5.0))"`} {
		if !strings.Contains(fbd, want) {
			t.Errorf("no input value %s:\n%s", want, fbd)
		}
	}
	if strings.Count(fbd, `data-val-expr="hold.Q"`) != 1 {
		t.Error("a timer's output shown as an expression at its own output")
	}
	out := draw("FBD", "  Total := MUL(ADD(a, b), 2.0)\n")
	for want, n := range map[string]int{`data-val-expr="(a + b)"`: 2, `data-val-expr="((a + b) * 2.0)"`: 1, `data-vals="=Total"`: 1} {
		if got := strings.Count(out, want); got != n {
			t.Errorf("%s %d times, want %d:\n%s", want, got, n, out)
		}
	}
	ld := draw("LD", ldSample)
	if !strings.Contains(ld, `data-prog="t1.ET t1.PT"`) || !strings.Contains(ld, `data-var="t1.Q"`) {
		t.Error("the LD timer's status")
	}
}

func TestCondExprs(t *testing.T) {
	for _, c := range []struct {
		cond string
		want []string
	}{
		{"WellLevel > 80.0", nil},
		{"WellLevel + 5.0 > Limit", []string{"WellLevel + 5.0"}},
		{"(a * 2 >= b - 1) AND NOT Done", []string{"a * 2", "b - 1"}},
		{"GT(ADD(a, b), 10) OR rest.Q", []string{"ADD(a, b)"}},
		{"GT(IN1 := ADD(a, b), IN2 := c)", []string{"ADD(a, b)"}},
		{"MAX(a, b) = c AND x <> y * 2 AND p - 1 < q AND r / 2 > s", []string{"MAX(a, b)", "y * 2", "p - 1", "r / 2"}},
		// each computed part's own computed inputs (L-41)
		{"MAX(a, b * 2) > c", []string{"MAX(a, b * 2)", "b * 2"}},
		{"(a + b) * 2 > c", []string{"(a + b) * 2", "(a + b)"}},
		{"LIMIT(0, x - 1.5E-3, 10) >= -y", []string{"LIMIT(0, x - 1.5E-3, 10)", "x - 1.5E-3", "-y"}},
		{"InRange(a * 2, b)", []string{"a * 2"}},
		{"a + b + c + d + e + f + g > 1", []string{"a + b + c + d + e + f + g"}},
		{"name = 'a AND b'", nil},
		{"Ready", nil},
	} {
		got := condExprs(c.cond)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%q: %q, want %q", c.cond, got, c.want)
		}
	}
}

func TestSFCValueExprs(t *testing.T) {
	c := &Chart{Steps: []Step{{Name: "Fill", Initial: true}, {Name: "Drain"}},
		Transitions: []Transition{{From: []string{"Fill"}, To: []string{"Drain"}, Condition: "WellLevel + 5.0 > Limit"},
			{From: []string{"Drain"}, To: []string{"Fill"}, Condition: "WellLevel < 20.0"}}}
	svg := SFCSVG(c)
	texts(t, svg)
	for _, want := range []string{`data-vals="=_sfc_Fill_E.ET"`, `data-expr="WellLevel &lt; 20.0"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s", want)
		}
	}
	if !strings.Contains(svg, `data-val-exprs="WellLevel + 5.0"`) || strings.Count(svg, "data-val-exprs=") != 1 {
		t.Errorf("values of computed parts:\n%s", svg)
	}
}

// TestSFCActionQualifier: an action's row names its step and qualifier, so
// a page can hold a P action green when the step becomes active (drone L-42).
func TestSFCActionQualifier(t *testing.T) {
	c := &Chart{Steps: []Step{{Name: "Fill", Initial: true}, {Name: "Pump", Actions: []Association{{Action: "ShowPump"}, {Action: "CountCycle", Qualifier: "p"}}}},
		Transitions: []Transition{{From: []string{"Fill"}, To: []string{"Pump"}, Condition: "TRUE"}, {From: []string{"Pump"}, To: []string{"Fill"}, Condition: "TRUE"}},
		ActionIndex: map[string]int{"SHOWPUMP": 0, "COUNTCYCLE": 1}}
	svg := SFCSVG(c)
	for _, want := range []string{`data-var="_sfc_a1_Q" data-neg="false" data-step="Pump" data-qual="P"`, `data-var="_sfc_a0_Q" data-neg="false" data-step="Pump" data-qual=""`} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s:\n%s", want, svg)
		}
	}
}

// TestSFCActionTimers: an action's own time shows only where it is not its
// step's (SD, DS, SL), with room for it in full; D and L are the step's
// time, shown under the step (drone L-43).
func TestSFCActionTimers(t *testing.T) {
	c := &Chart{Steps: []Step{{Name: "Pump", Initial: true, Actions: []Association{
		{Action: "PumpTooLong", Qualifier: "D", Duration: "T#60S"}, {Action: "Purge", Qualifier: "SD", Duration: "T#5S"}, {Action: "Lamp", Qualifier: "N"}}}},
		Transitions: []Transition{{From: []string{"Pump"}, To: []string{"Pump"}, Condition: "FALSE"}},
		Actions:     []Action{{Name: "PumpTooLong", Body: "x := TRUE;"}, {Name: "Purge", Body: "y := TRUE;"}},
		ActionIndex: map[string]int{"PUMPTOOLONG": 0, "PURGE": 1, "LAMP": 2}}
	svg := SFCSVG(c)
	if strings.Contains(svg, "elapsed=_sfc_a0_t.ET") {
		t.Error("D shows its own time, the step's")
	}
	for _, want := range []string{`data-vals="elapsed=_sfc_a1_t.ET"`, `data-vals="Lamp"`, `data-vals="=_sfc_Pump_E.ET"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s", want)
		}
	}
	if strings.Contains(svg, `data-vals="PumpTooLong"`) {
		t.Error("an action with a body shown as a variable")
	}
	// the drawing is wide enough for the values in full
	width, x := 0.0, 0.0
	fmt.Sscanf(svg[strings.Index(svg, `width="`)+7:], "%f", &width)
	label := strings.LastIndex(svg[:strings.Index(svg, `data-vals="elapsed=`)], `x="`)
	fmt.Sscanf(svg[label+3:], "%f", &x)
	if end := x + charW*10/12*float64(len("elapsed = T#1h23m45s678ms")); end > width {
		t.Errorf("the action's time ends at %.0f, past the drawing's width %.0f", end, width)
	}
}

// TestSFCStepTimeFits: a step's time, right aligned left of its link, fits
// inside the drawing at its longest (drone L-44).
func TestSFCStepTimeFits(t *testing.T) {
	c := &Chart{Steps: []Step{{Name: "A", Initial: true}, {Name: "B"}, {Name: "C"}},
		Transitions: []Transition{{From: []string{"A"}, To: []string{"B", "C"}, Condition: "go"}, {From: []string{"B", "C"}, To: []string{"A"}, Condition: "back"}}}
	svg := SFCSVG(c)
	n := 0
	for _, part := range strings.Split(svg, `data-vals="=_sfc_`)[1:] {
		_ = part
		n++
	}
	if n != 3 {
		t.Fatalf("%d step times, want 3", n)
	}
	for i := 0; ; {
		at := strings.Index(svg[i:], `text-anchor="end" data-vals="=_sfc_`)
		if at < 0 {
			break
		}
		at += i
		x := 0.0
		fmt.Sscanf(svg[strings.LastIndex(svg[:at], `x="`)+3:], "%f", &x)
		if start := x - float64(len(longestTime))*charW*10/12; start < 0 {
			t.Errorf("a step's time starts at %.0f, left of the drawing", start)
		}
		i = at + 1
	}
}
