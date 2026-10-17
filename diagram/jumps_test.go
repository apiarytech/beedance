/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram

import (
	"strings"
	"testing"
)

const ldJumps = `
  RUNG off
    /Enable >>SKIP
  RUNG
    A ( Q )
  SKIP:
  RUNG
    A ( Seen ) >>DONE
  RUNG
    /A RETURN
  RUNG
    >>DONE
  DONE:
  RUNG
    Enable ( Last )
`

const fbdJumps = `
  i := 0
  AGAIN:
  i := ADD(i, 1)
  >>AGAIN(LT(i, k))
  RETURN(stop)
  >>OUT
  after := i
  OUT:
`

// IEC-71: jumps, labels and returns lower to ST jumps, in LD and FBD.
func TestLowerJumps(t *testing.T) {
	low, err := LowerText("P", "LD", ldJumps, 1, Options{Declared: []string{"Enable", "A", "Q", "Seen", "Last"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "IF NOT(Enable) THEN JMP SKIP; END_IF;\nQ := A;\nSKIP:\nSeen := A;\nIF A THEN JMP DONE; END_IF;\nIF NOT(A) THEN RETURN; END_IF;\nJMP DONE;\nDONE:\nLast := Enable;\n"
	if low.Body != want {
		t.Fatalf("LD lowered to\n%s\nwant\n%s", low.Body, want)
	}
	low, err = LowerText("P", "FBD", fbdJumps, 1, Options{Declared: []string{"i", "k", "stop", "after"}})
	if err != nil {
		t.Fatal(err)
	}
	want = "i := 0;\nAGAIN:\ni := (i + 1);\nIF (i < k) THEN JMP AGAIN; END_IF;\nIF stop THEN RETURN; END_IF;\nJMP OUT;\nafter := i;\nOUT:\n"
	if low.Body != want {
		t.Fatalf("FBD lowered to\n%s\nwant\n%s", low.Body, want)
	}
	for name, src := range map[string]string{
		"jump without a label": "\n  RUNG\n    A >>\n",
		"FBD jump no label":    "\n  >>\n",
	} {
		lang := "LD"
		if strings.HasPrefix(name, "FBD") {
			lang = "FBD"
		}
		if _, err := LowerText("P", lang, src, 1, Options{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// IEC-71: they survive PLCopen XML and the text form both ways, and are
// drawn as IEC 61131-3 draws them.
func TestJumpsRoundTripAndDrawing(t *testing.T) {
	roundTrip(t, "LD", ldJumps, "Enable", "A", "Q", "Seen", "Last")
	roundTrip(t, "FBD", fbdJumps, "i", "k", "stop", "after")
	text, err := ParseText("LD", ldJumps, 1)
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(text.Elems)
	for _, want := range []string{"&gt;&gt;SKIP", "&lt;RETURN&gt;", "SKIP:", "DONE:", "&gt;&gt;DONE"} {
		if !strings.Contains(svg, want) {
			t.Errorf("drawing lacks %q", want)
		}
	}
	x, err := XML(text.Elems)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<jump localId=`, `label="SKIP"`, `<return localId=`, `<label localId=`} {
		if !strings.Contains(x, want) {
			t.Errorf("XML lacks %q:\n%s", want, x)
		}
	}
}
