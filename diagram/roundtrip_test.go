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

// roundTrip writes a text-form body as PLCopen XML, reads the XML back and
// writes it as text again; both texts must lower to the same statements,
// and the XML itself must lower to them too.
func roundTrip(t *testing.T, lang, src string, declared ...string) string {
	t.Helper()
	opt := Options{Declared: declared}
	want, err := LowerText("P", lang, src, 1, opt)
	if err != nil {
		t.Fatalf("original: %v", err)
	}
	orig, err := ParseText(lang, src, 1)
	if err != nil {
		t.Fatal(err)
	}
	x, err := XML(orig.Elems)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	elems, err := ParseXML(x)
	if err != nil {
		t.Fatalf("ParseXML: %v\n%s", err, x)
	}
	text, err := Format(lang, elems, nil)
	if err != nil {
		t.Fatalf("Format: %v\n%s", err, x)
	}
	got, err := LowerText("P", lang, text, 1, opt)
	if err != nil {
		t.Fatalf("formatted text: %v\n%s", err, text)
	}
	if got.Body != want.Body || strings.Join(got.Decls, ";") != strings.Join(want.Decls, ";") {
		t.Fatalf("the round trip changed the body\noriginal:\n%s\nformatted:\n%s\nlowered before:\n%s%v\nlowered after:\n%s%v",
			src, text, want.Body, want.Decls, got.Body, got.Decls)
	}
	if lang == "FBD" {
		opt.OneNetwork = true
	}
	opt.Declared = append(opt.Declared, instances(orig)...)
	fromXML, err := Lower("P", elems, opt)
	if err != nil {
		t.Fatalf("lowering the XML: %v\n%s", err, x)
	}
	if fromXML.Body != want.Body {
		t.Fatalf("the XML lowers differently\ntext:\n%s\nXML:\n%s", want.Body, fromXML.Body)
	}
	return text
}

// instances are the names the text declares, which a POU's XML declares in
// its interface instead.
func instances(t *Text) []string {
	var out []string
	for _, d := range t.Declares {
		out = append(out, d[0])
	}
	return out
}

func TestRoundTripLadder(t *testing.T) {
	text := roundTrip(t, "LD", `
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S) ( Done )
  RUNG count
    +Run cu:CTU(PV := 100, CV => Starts)
  RUNG coils
    A ( S X ) ( R Y ) ( P Z ) ( N W ) ( /V )
  RUNG nested
    A [ B [ C | D ] | E ] -F ( Out )
`, "Start", "Run", "Stop", "Done", "Starts", "A", "B", "C", "D", "E", "F", "X", "Y", "Z", "W", "V", "Out")
	for _, want := range []string{"[ Start | Run ] /Stop ( Run )", "t1:TON(PT := T#5S) ( Done )", "cu:CTU(PV := 100, CV => Starts)",
		"( S X ) ( R Y ) ( P Z ) ( N W ) ( /V )", "A [ B [ C | D ] | E ] -F ( Out )"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
}

func TestRoundTripFunctionContacts(t *testing.T) {
	text := roundTrip(t, "LD", `
  RUNG first
    EQ(Mode, 2) Run ( A )
  RUNG middle
    Run /GT(Temp, 80.0) ( B )
`, "Mode", "Run", "Temp", "A", "B")
	for _, want := range []string{"EQ(Mode, 2) Run ( A )", "Run /GT(Temp, 80.0) ( B )"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
}

func TestRoundTripFBD(t *testing.T) {
	text := roundTrip(t, "FBD", `
  hot = GT(Temp, Limit)
  t1 : TON(IN := hot, PT := T#5S)
  Alarm := OR(t1.Q, Alarm)
  Level := LIMIT(0, ADD(Raw, Offset), 100)
`, "Temp", "Limit", "Alarm", "Level", "Raw", "Offset")
	for _, want := range []string{"t1 : TON(IN := hot, PT := T#5S)", "Alarm := OR(t1.Q, Alarm)", "Level := LIMIT(0, ADD(Raw, Offset), 100)"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
}

// bridge is a rung whose branches cross at M: A and B each reach the coil
// two ways. edgeA makes A a rising-edge contact.
func bridge(edgeA bool) string {
	a := ""
	if edgeA {
		a = ` edge="rising"`
	}
	return `<leftPowerRail localId="1"><position x="0" y="0"/><connectionPointOut formalParameter=""/></leftPowerRail>
<contact localId="2"` + a + `><position x="40" y="20"/><connectionPointIn><connection refLocalId="1"/></connectionPointIn><connectionPointOut/><variable>A</variable></contact>
<contact localId="3"><position x="40" y="60"/><connectionPointIn><connection refLocalId="1"/></connectionPointIn><connectionPointOut/><variable>B</variable></contact>
<contact localId="4"><position x="100" y="40"/><connectionPointIn><connection refLocalId="2"/><connection refLocalId="3"/></connectionPointIn><connectionPointOut/><variable>M</variable></contact>
<contact localId="5"><position x="160" y="20"/><connectionPointIn><connection refLocalId="2"/><connection refLocalId="4"/></connectionPointIn><connectionPointOut/><variable>C</variable></contact>
<coil localId="6"><position x="220" y="20"/><connectionPointIn><connection refLocalId="5"/><connection refLocalId="3"/></connectionPointIn><connectionPointOut/><variable>Out</variable></coil>`
}

// Crossing branches of plain contacts are written by repeating the
// contacts they share, which have no state; the logic is the same
// (A·C + A·M·C + B·M·C + B).
func TestFormatExpandsABridge(t *testing.T) {
	elems, err := ParseXML(bridge(false))
	if err != nil {
		t.Fatal(err)
	}
	text, err := Format("LD", elems, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[ [ A | [ A | B ] M ] C | B ] ( Out )"; !strings.Contains(text, want) {
		t.Fatalf("no %q in\n%s", want, text)
	}
}

// An edge contact has state: repeating it would detect its edge twice, so
// a crossing that needs it twice is refused, and the caller lowers the body
// instead.
func TestFormatRefusesABridgeThroughAnEdge(t *testing.T) {
	elems, err := ParseXML(bridge(true))
	if err != nil {
		t.Fatal(err)
	}
	if text, err := Format("LD", elems, nil); err == nil {
		t.Fatalf("a rising edge was written twice:\n%s", text)
	}
}
