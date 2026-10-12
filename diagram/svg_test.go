package diagram

import (
	"encoding/xml"
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
