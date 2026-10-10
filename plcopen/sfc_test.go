/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/diagram"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// --- SFC drawing helpers: our own fixtures, element by element ---

func sfcStep(id, x, y int, name string, initial bool, wires ...string) string {
	in := ""
	if len(wires) > 0 {
		in = pointIn(wires...)
	}
	return fmt.Sprintf(`<step localId="%d" name="%s" initialStep="%t">%s%s<connectionPointOut formalParameter=""/><connectionPointOutAction formalParameter=""/></step>`,
		id, name, initial, pos(x, y), in)
}

// sfcTransition is a transition; cond is the content of its <condition>.
func sfcTransition(id, x, y int, attrs, cond string, wires ...string) string {
	return fmt.Sprintf(`<transition localId="%d"%s>%s%s<connectionPointOut/><condition%s</condition></transition>`,
		id, attrs, pos(x, y), pointIn(wires...), cond)
}

func inlineST(text string) string {
	return fmt.Sprintf(`><inline name=""><ST><xhtml:p xmlns:xhtml="http://www.w3.org/1999/xhtml">%s</xhtml:p></ST></inline>`, text)
}

func reference(name string) string { return fmt.Sprintf(`><reference name="%s"/>`, name) }

func sfcNode(kind string, id, x, y int, outs int, ins ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<%s localId="%d">%s`, kind, id, pos(x, y))
	for _, in := range ins {
		b.WriteString(pointIn(in))
	}
	for i := range outs {
		fmt.Fprintf(&b, `<connectionPointOut formalParameter="%d"/>`, i)
	}
	fmt.Fprintf(&b, `</%s>`, kind)
	return b.String()
}

// sfcAction is one action of an action block: a reference, or inline ST
// when body is set.
type sfcAct struct{ qualifier, duration, name, body string }

func actionBlock(id, x, y, step int, acts ...sfcAct) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<actionBlock localId="%d">%s%s`, id, pos(x, y), pointIn(wire(step, "")))
	for i, a := range acts {
		fmt.Fprintf(&b, `<action localId="%d" qualifier="%s"`, id*100+i, a.qualifier)
		if a.duration != "" {
			fmt.Fprintf(&b, ` duration="%s"`, a.duration)
		}
		b.WriteString(`><relPosition x="0" y="0"/>`)
		if a.body != "" {
			fmt.Fprintf(&b, `<inline><ST><xhtml:p xmlns:xhtml="http://www.w3.org/1999/xhtml">%s</xhtml:p></ST></inline>`, a.body)
		} else {
			fmt.Fprintf(&b, `<reference name="%s"/>`, a.name)
		}
		b.WriteString(`<connectionPointOut/></action>`)
	}
	b.WriteString(`</actionBlock>`)
	return b.String()
}

func stBody(text string) string {
	return fmt.Sprintf(`<body><ST><xhtml:p xmlns:xhtml="http://www.w3.org/1999/xhtml">%s</xhtml:p></ST></body>`, text)
}

// mixerSFC is a batch mixer:
//
//	Idle --Start (priority 2)--> Fill, --Abort (priority 1)--> jump Idle
//	Fill --Level >= 30 (a wired variable)--> Heat || Stir
//	Heat --HotEnough (an LD transition)--> HeatDone
//	HeatDone, Stir --Stirred (a wired contact)--> Done
//	Done --NOT Busy--> Idle
func mixerSFC() []byte {
	body := sfcStep(1, 200, 20, "Idle", true, wire(20, "")) +
		actionBlock(2, 300, 20, 1, sfcAct{qualifier: "N", name: "Ready"}) +
		sfcNode("selectionDivergence", 3, 200, 60, 2, wire(1, "")) +
		sfcTransition(4, 200, 80, ` priority="2"`, inlineST("Start"), wire(3, "")) +
		sfcTransition(5, 400, 80, ` priority="1"`, reference("Abort"), wire(3, "")) +
		sfcStep(6, 200, 120, "Fill", false, wire(4, "")) +
		actionBlock(7, 300, 120, 6, sfcAct{qualifier: "N", body: "Level := Level + 10;"}) +
		`<jumpStep localId="8" targetName="Idle">` + pos(400, 120) + pointIn(wire(5, "")) + `</jumpStep>` +
		inVar(30, 20, 160, "Level &gt;= 30") +
		sfcTransition(9, 200, 160, "", ">"+pointIn(wire(30, "")), wire(6, "")) +
		sfcNode("simultaneousDivergence", 10, 200, 180, 2, wire(9, "")) +
		sfcStep(11, 200, 220, "Heat", false, wire(10, "")) +
		actionBlock(12, 300, 220, 11, sfcAct{qualifier: "N", name: "Heater"}, sfcAct{qualifier: "L", duration: "PT2S", name: "Lamp"}) +
		sfcStep(13, 500, 220, "Stir", false, wire(10, "")) +
		actionBlock(14, 600, 220, 13, sfcAct{qualifier: "N", name: "Mix"}) +
		sfcTransition(15, 200, 260, "", reference("HotEnough"), wire(11, "")) +
		sfcStep(16, 200, 300, "HeatDone", false, wire(15, "")) +
		sfcNode("simultaneousConvergence", 17, 200, 340, 0, wire(16, ""), wire(13, "")) +
		leftRail(40, 20, 360) +
		contact(41, 60, 360, "Stirred", "", wire(40, "")) +
		sfcTransition(18, 200, 360, "", ">"+pointIn(wire(41, "")), wire(17, "")) +
		sfcStep(19, 200, 400, "Done", false, wire(18, "")) +
		sfcTransition(20, 200, 440, "", ` negated="true"`+inlineST("Busy"), wire(19, ""))

	hotEnough := leftRail(1, 20, 20) + contact(2, 60, 20, "Hot", "", wire(1, "")) + coil(3, 120, 20, "HotEnough", "", wire(2, ""))
	pou := `<pou name="Mixer" pouType="functionBlock"><interface>` +
		vars("inputVars", "Start:BOOL", "Fault:BOOL", "Hot:BOOL", "Stirred:BOOL", "Busy:BOOL") +
		vars("outputVars", "Ready:BOOL", "Heater:BOOL", "Lamp:BOOL", "Level:INT", "Mixed:INT") +
		`</interface>` +
		`<actions><action name="Mix">` + stBody("Mixed := Mixed + 1;") + `</action></actions>` +
		`<transitions>` +
		`<transition name="Abort">` + stBody(":= Fault;") + `</transition>` +
		`<transition name="HotEnough"><body><LD>` + hotEnough + `</LD></body></transition>` +
		`</transitions>` +
		`<body><SFC>` + body + `</SFC></body></pou>`
	return project(pou)
}

func TestSFCImportText(t *testing.T) {
	iec := lower(t, mixerSFC())
	for _, want := range []string{
		"\tACTION Mix:\n\t\tMixed := Mixed + 1;\n\tEND_ACTION\n",
		"\tACTION _Fill_1:\n\t\tLevel := Level + 10;\n\tEND_ACTION\n",
		"\tINITIAL_STEP Idle:\n\t\tReady(N);\n\tEND_STEP\n",
		"\tSTEP Fill:\n\t\t_Fill_1(N);\n\tEND_STEP\n",
		"\tSTEP Heat:\n\t\tHeater(N);\n\t\tLamp(L, T#2s);\n\tEND_STEP\n",
		"\tSTEP Done: END_STEP\n",
		// Abort has priority 1: Start clears only when Abort does not.
		"TRANSITION FROM Idle TO Idle := Fault; END_TRANSITION",
		"TRANSITION FROM Idle TO Fill := Start AND NOT Fault; END_TRANSITION",
		"TRANSITION FROM Fill TO (Heat, Stir) := (Level >= 30); END_TRANSITION",
		"TRANSITION FROM Heat TO HeatDone := Hot; END_TRANSITION",
		"TRANSITION FROM (HeatDone, Stir) TO Done := Stirred; END_TRANSITION",
		"TRANSITION FROM Done TO Idle := NOT (Busy); END_TRANSITION",
	} {
		if !strings.Contains(iec, want) {
			t.Errorf("no %q in\n%s", want, iec)
		}
	}
	p := parser.New(lexer.New(iec))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v\n%s", errs, iec)
	}
}

func TestSFCImportRuns(t *testing.T) {
	iec := lower(t, mixerSFC())
	decl := "VAR m : Mixer; b : BOOL; i : INT; END_VAR"

	// Start fills in steps of 10 until Level >= 30, then heats and stirs.
	script := "m(Start := TRUE); m(Start := FALSE); m(); m(); m(); m(); "
	if !boolValue(t, run(t, iec, decl, script+"b := m.Heater; b;")) {
		t.Errorf("not heating after filling:\n%s", iec)
	}
	if v := intValue(t, run(t, iec, decl, script+"i := m.Level; i;")); v < 30 {
		t.Errorf("filled to %d", v)
	}
	if v := intValue(t, run(t, iec, decl, script+"m(); i := m.Mixed; i;")); v == 0 {
		t.Errorf("not stirring in parallel with heating")
	}
	// Hot and Stirred join the branches in Done; Heater, an N action, drops.
	done := script + "m(Hot := TRUE); m(Hot := FALSE, Stirred := TRUE); m(Stirred := FALSE, Busy := TRUE); "
	if boolValue(t, run(t, iec, decl, done+"b := m.Heater; b;")) {
		t.Errorf("still heating in Done")
	}
	// Fault has priority over Start: the chart stays in Idle, unfilled.
	if v := intValue(t, run(t, iec, decl, "m(Start := TRUE, Fault := TRUE); m(Start := TRUE, Fault := TRUE); m(); i := m.Level; i;")); v != 0 {
		t.Errorf("Start cleared over the higher-priority Abort: Level %d", v)
	}
}

func TestSFCImportRefused(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{sfcStep(1, 0, 0, "A", true) + `<macroStep localId="2">` + pos(0, 40) + `</macroStep>`, "macro steps are not supported yet"},
		{sfcStep(1, 0, 0, "A", true) + sfcStep(2, 0, 40, "B", true), "both initial"},
		{sfcStep(1, 0, 0, "A", true) + sfcTransition(2, 0, 20, "", reference("Missing"), wire(1, "")) + sfcStep(3, 0, 40, "B", false, wire(2, "")), "no transition named Missing"},
		{sfcStep(1, 0, 0, "A", true) + sfcTransition(2, 0, 20, "", inlineST("x := 1; y := 2;"), wire(1, "")) + sfcStep(3, 0, 40, "B", false, wire(2, "")), "not an expression"},
		{sfcStep(1, 0, 0, "A", true) + sfcTransition(2, 0, 20, "", inlineST("TRUE"), wire(1, "")), "leads to no step"},
		{sfcStep(1, 0, 0, "A", true) + actionBlock(2, 40, 0, 9, sfcAct{qualifier: "N", name: "X"}), "not wired to a step"},
	} {
		data := project(`<pou name="P" pouType="program"><interface/><body><SFC>` + tt.body + `</SFC></body></pou>`)
		if _, err := ImportToIECText(data); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("expected an error containing %q, got %v", tt.want, err)
		}
	}
}

// ReadSFC's chart is the one a drawing takes.
func TestReadSFCDraws(t *testing.T) {
	proj, err := Import(mixerSFC())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := ReadSFC(proj.Types.Pous.Pous[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	svg := diagram.SFCSVG(sc.Chart)
	for _, want := range []string{">Idle<", ">HeatDone<", ">Heater<", ">T#2s<", ">HotEnough<"} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s in the drawing", want)
		}
	}
	// SVG_OUT=dir writes the drawing as a page, to look at.
	if dir := os.Getenv("SVG_OUT"); dir != "" {
		page := "<html><body style='background:#fff;margin:0'>" + svg + "</body></html>"
		if err := os.WriteFile(filepath.Join(dir, "mixer.html"), []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// pressSFC has a selection, a simultaneous branch and loops back to its
// initial step.
const pressSFC = `PROGRAM Press
VAR Go, Up, Down, Clamped, Fault : BOOL; Strokes : INT; END_VAR
ACTION Count:
	(* one more stroke *)
	Strokes := Strokes + 1;
END_ACTION
INITIAL_STEP Ready: END_STEP
STEP Lower: Down(N); Count(P); END_STEP
STEP Clamp: Clamped(S); END_STEP
STEP Raise: Up(N); Clamped(R); END_STEP
STEP Alarm: Fault(SD, T#500ms); END_STEP
TRANSITION FROM Ready TO (Lower, Clamp) := Go; END_TRANSITION
TRANSITION FROM (Lower, Clamp) TO Raise := Strokes > 3 (* as written *); END_TRANSITION
TRANSITION FROM Raise TO Ready := NOT Go; END_TRANSITION
TRANSITION FROM Raise TO Alarm := Go AND Fault; END_TRANSITION
TRANSITION FROM Alarm TO Ready := NOT Fault; END_TRANSITION
END_PROGRAM
`

// A chart in the text form exports as a graphical chart the schema
// accepts, and imports back as the same chart.
func TestSFCExportRoundTrip(t *testing.T) {
	data, err := ExportSourceToXML(pressSFC, "press")
	if err != nil {
		t.Fatal(err)
	}
	xml := string(data)
	for _, want := range []string{`<step localId=`, `initialStep="true"`, `<simultaneousDivergence `, `<simultaneousConvergence `,
		`<selectionDivergence `, `<jumpStep `, `targetName="Ready"`, `priority="2"`, `<actionBlock `, `qualifier="SD" duration="T#500ms"`,
		`<action name="Count">`, "Strokes &gt; 3 (* as written *)"} {
		if !strings.Contains(xml, want) && !strings.Contains(xml, strings.ReplaceAll(want, "&gt;", ">")) {
			t.Errorf("no %s in\n%s", want, xml)
		}
	}
	if strings.Contains(xml, "<ST>") && strings.Contains(xml, "INITIAL_STEP") {
		t.Errorf("the chart is exported as text:\n%s", xml)
	}
	if err := ValidateWithXSD(data, ""); err != nil {
		t.Fatalf("the export is not valid TC6: %v", err)
	}
	if !XSDValidatorAvailable() {
		t.Log("xmllint is not installed: only the structural checks ran")
	}

	iec := lower(t, data)
	for _, want := range []string{
		"ACTION Count:\n\t\t(* one more stroke *)\n\t\tStrokes := Strokes + 1;\n\tEND_ACTION",
		"INITIAL_STEP Ready: END_STEP",
		"STEP Lower:\n\t\tDown(N);\n\t\tCount(P);\n\tEND_STEP",
		"STEP Alarm:\n\t\tFault(SD, T#500ms);\n\tEND_STEP",
		"TRANSITION FROM Ready TO (Lower, Clamp) := Go; END_TRANSITION",
		"TRANSITION FROM (Lower, Clamp) TO Raise := Strokes > 3 (* as written *); END_TRANSITION",
		"TRANSITION FROM Raise TO Ready := NOT Go; END_TRANSITION",
		// A selection imports with IEC's priorities: the chart's order.
		"TRANSITION FROM Raise TO Alarm := (Go AND Fault) AND NOT (NOT Go); END_TRANSITION",
		"TRANSITION FROM Alarm TO Ready := NOT Fault; END_TRANSITION",
	} {
		if !strings.Contains(iec, want) {
			t.Errorf("no %q in\n%s", want, iec)
		}
	}
	p := parser.New(lexer.New(iec))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v\n%s", errs, iec)
	}
}

// A chart the graphical form cannot hold keeps the text form.
func TestSFCExportKeepsText(t *testing.T) {
	src := "PROGRAM P\nVAR x : INT; END_VAR\nINITIAL_STEP A: x := 1; END_STEP\nSTEP B: END_STEP\nTRANSITION FROM A TO B := x > 0; END_TRANSITION\nEND_PROGRAM\n"
	data, err := ExportSourceToXML(src, "p")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "<SFC>") || !strings.Contains(string(data), "INITIAL_STEP A") {
		t.Errorf("a step with statements was not kept as text:\n%s", data)
	}
}
