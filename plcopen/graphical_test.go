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
	"strings"
	"testing"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"

	_ "github.com/apiarytech/beedance/stdlib" // registers SEL, MUX, LIMIT, ... with the evaluator
)

// --- drawing helpers: our own fixtures, element by element ---

func pos(x, y int) string { return fmt.Sprintf(`<position x="%d" y="%d"/>`, x, y) }

func wire(ref int, param string) string {
	if param != "" {
		return fmt.Sprintf(`<connection refLocalId="%d" formalParameter="%s"/>`, ref, param)
	}
	return fmt.Sprintf(`<connection refLocalId="%d"/>`, ref)
}

func pointIn(wires ...string) string {
	return "<connectionPointIn>" + strings.Join(wires, "") + "</connectionPointIn>"
}

func inVar(id, x, y int, expr string) string {
	return fmt.Sprintf(`<inVariable localId="%d">%s<connectionPointOut/><expression>%s</expression></inVariable>`, id, pos(x, y), expr)
}

func outVar(id, x, y int, expr string, wires ...string) string {
	return fmt.Sprintf(`<outVariable localId="%d">%s%s<expression>%s</expression></outVariable>`, id, pos(x, y), pointIn(wires...), expr)
}

func inOutVar(id, x, y int, expr string, wires ...string) string {
	return fmt.Sprintf(`<inOutVariable localId="%d">%s%s<connectionPointOut/><expression>%s</expression></inOutVariable>`, id, pos(x, y), pointIn(wires...), expr)
}

// pin is a block input: "PARAM=wire".
type pin struct {
	param string
	wire  string
	attrs string
}

func block(id, x, y int, typ, inst string, outs []string, ins ...pin) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<block localId="%d" typeName="%s"`, id, typ)
	if inst != "" {
		fmt.Fprintf(&b, ` instanceName="%s"`, inst)
	}
	b.WriteString(">" + pos(x, y) + "<inputVariables>")
	for _, p := range ins {
		fmt.Fprintf(&b, `<variable formalParameter="%s"%s>%s</variable>`, p.param, p.attrs, pointIn(p.wire))
	}
	b.WriteString("</inputVariables><inOutVariables/><outputVariables>")
	for _, o := range outs {
		fmt.Fprintf(&b, `<variable formalParameter="%s"><connectionPointOut/></variable>`, o)
	}
	b.WriteString("</outputVariables></block>")
	return b.String()
}

func leftRail(id, x, y int) string {
	return fmt.Sprintf(`<leftPowerRail localId="%d">%s<connectionPointOut formalParameter=""/></leftPowerRail>`, id, pos(x, y))
}

func contact(id, x, y int, v, attrs string, wires ...string) string {
	return fmt.Sprintf(`<contact localId="%d"%s>%s%s<connectionPointOut/><variable>%s</variable></contact>`, id, attrs, pos(x, y), pointIn(wires...), v)
}

func coil(id, x, y int, v, attrs string, wires ...string) string {
	return fmt.Sprintf(`<coil localId="%d"%s>%s%s<connectionPointOut/><variable>%s</variable></coil>`, id, attrs, pos(x, y), pointIn(wires...), v)
}

// fb is a function block POU with a graphical body.
func fb(name, lang, vars, body string) string {
	return fmt.Sprintf(`<pou name="%s" pouType="functionBlock"><interface>%s</interface><body><%s>%s</%s></body></pou>`,
		name, vars, lang, body, lang)
}

var elementary = map[string]bool{"BOOL": true, "INT": true, "DINT": true, "REAL": true, "TIME": true}

func vars(section string, decls ...string) string {
	var b strings.Builder
	b.WriteString("<" + section + ">")
	for _, d := range decls {
		name, typ, _ := strings.Cut(d, ":")
		if elementary[typ] {
			fmt.Fprintf(&b, `<variable name="%s"><type><%s/></type></variable>`, name, typ)
		} else { // a function block or user type
			fmt.Fprintf(&b, `<variable name="%s"><type><derived name="%s"/></type></variable>`, name, typ)
		}
	}
	b.WriteString("</" + section + ">")
	return b.String()
}

func project(pous ...string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://www.plcopen.org/xml/tc6_0201">
<fileHeader companyName="test" productName="test" productVersion="1" creationDateTime="2026-10-05T00:00:00"/>
<contentHeader name="test"><coordinateInfo><fbd><scaling x="1" y="1"/></fbd><ld><scaling x="1" y="1"/></ld><sfc><scaling x="1" y="1"/></sfc></coordinateInfo></contentHeader>
<types><dataTypes/><pous>` + strings.Join(pous, "") + `</pous></types>
<instances><configurations/></instances>
</project>`)
}

// lower imports a project and returns its IEC text.
func lower(t *testing.T, xmlData []byte) string {
	t.Helper()
	iec, err := ImportToIECText(xmlData)
	if err != nil {
		t.Fatal(err)
	}
	return iec
}

// run evaluates iec followed by script and returns the script's last value.
// It also compiles a PROGRAM made of the script for the VM, so both
// engines accept what the lowering writes.
func run(t *testing.T, iec, decl, script string) object.Object {
	t.Helper()
	src := iec + "\n" + decl + "\n" + script
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v\n%s", errs, src)
	}
	object.FinalizeBuiltins()
	out := evaluator.Eval(prog, object.NewEnvironment())
	if e, ok := out.(*object.Error); ok {
		t.Fatalf("evaluate: %s\n%s", e.Inspect(), src)
	}

	vmSrc := iec + "\nPROGRAM Harness\n" + decl + "\n" + strings.TrimSuffix(strings.TrimSpace(script), ";") + ";\nEND_PROGRAM\n"
	vp := parser.New(lexer.New(vmSrc))
	vprog := vp.ParseProgram()
	if errs := vp.Errors(); len(errs) > 0 {
		t.Fatalf("parse for the VM: %v\n%s", errs, vmSrc)
	}
	if _, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(vprog, "Harness"); err != nil {
		t.Fatalf("compile for the VM: %v\n%s", err, vmSrc)
	}
	return out
}

func intValue(t *testing.T, o object.Object) int64 {
	t.Helper()
	v, _, ok := object.GetIntegerObjectValue(o)
	if !ok {
		t.Fatalf("not an integer: %T %v", o, o)
	}
	return v
}

func boolValue(t *testing.T, o object.Object) bool {
	t.Helper()
	b, ok := o.(*object.Boolean)
	if !ok {
		t.Fatalf("not a BOOL: %T %v", o, o)
	}
	return b.Value
}

// --- FBD ---

// counterFBD: Cnt := SEL(Reset, ADD(Cnt, 1), 0); OUT := Cnt. Cnt is read
// and written, a feedback through a variable, as drawing tools write it.
func counterFBD() []byte {
	body := inVar(1, 20, 40, "Reset") +
		inVar(2, 20, 120, "1") +
		inVar(3, 20, 200, "0") +
		block(4, 150, 100, "ADD", "", []string{"OUT"},
			pin{param: "IN1", wire: wire(5, "")}, pin{param: "IN2", wire: wire(2, "")}) +
		inOutVar(5, 420, 40, "Cnt", wire(6, "OUT")) +
		block(6, 280, 40, "SEL", "", []string{"OUT"},
			pin{param: "G", wire: wire(1, "")}, pin{param: "IN0", wire: wire(4, "OUT")}, pin{param: "IN1", wire: wire(3, "")}) +
		outVar(7, 560, 40, "OUT", wire(5, ""))
	return project(fb("CounterFbd", "FBD",
		vars("inputVars", "Reset:BOOL")+vars("outputVars", "OUT:INT")+vars("localVars", "Cnt:INT"), body))
}

func TestFBDCounter(t *testing.T) {
	iec := lower(t, counterFBD())
	for _, want := range []string{"Cnt := SEL(Reset, (Cnt + 1), 0);", "OUT := Cnt;"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	if strings.Index(iec, "Cnt := ") > strings.Index(iec, "OUT := Cnt") {
		t.Fatalf("OUT is assigned before Cnt:\n%s", iec)
	}
	decl := "VAR c : CounterFbd; o : INT; END_VAR"
	got := run(t, iec, decl, "c(Reset := FALSE); c(Reset := FALSE); c(Reset := FALSE); o := c.OUT; o;")
	if v := intValue(t, got); v != 3 {
		t.Fatalf("three scans counted %d", v)
	}
	got = run(t, iec, decl, "c(Reset := FALSE); c(Reset := FALSE); c(Reset := TRUE); o := c.OUT; o;")
	if v := intValue(t, got); v != 0 {
		t.Fatalf("after a reset: %d", v)
	}
}

func TestFBDFunctionBlockWithENO(t *testing.T) {
	body := inVar(1, 20, 20, "Enable") +
		inVar(2, 20, 60, "Pulse") +
		inVar(3, 20, 100, "3") +
		block(4, 150, 20, "CTU", "c1", []string{"ENO", "Q", "CV"},
			pin{param: "EN", wire: wire(1, "")}, pin{param: "CU", wire: wire(2, "")}, pin{param: "PV", wire: wire(3, "")}) +
		outVar(5, 300, 20, "Done", wire(4, "Q")) +
		outVar(6, 300, 60, "Count", wire(4, "CV"))
	iec := lower(t, project(fb("Counting", "FBD",
		vars("inputVars", "Enable:BOOL", "Pulse:BOOL")+vars("outputVars", "Done:BOOL", "Count:INT")+vars("localVars", "c1:CTU"), body)))
	for _, want := range []string{"c1(EN := Enable, CU := Pulse, PV := 3, ENO => _ENO4);", "IF _ENO4 THEN Done := c1.Q; END_IF;", "_ENO4 : BOOL;"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	decl := "VAR k : Counting; n : INT; END_VAR"
	// Three rising edges while enabled count 3; while disabled, nothing.
	got := run(t, iec, decl, `k(Enable := TRUE, Pulse := TRUE); k(Enable := TRUE, Pulse := FALSE);
k(Enable := TRUE, Pulse := TRUE); k(Enable := TRUE, Pulse := FALSE);
k(Enable := FALSE, Pulse := TRUE); k(Enable := FALSE, Pulse := FALSE);
k(Enable := TRUE, Pulse := TRUE); n := k.Count; n;`)
	if v := intValue(t, got); v != 3 {
		t.Fatalf("counted %d, want 3 (disabled edges are not counted)", v)
	}
}

func TestFBDOrder(t *testing.T) {
	// Network 1 (top) writes X from A; network 2 (below) writes Y from X.
	// Inside network 2 the block is drawn below its output and still runs
	// first: the data flow orders a network, its position does not.
	body := outVar(10, 300, 300, "Y", wire(12, "Q")) +
		inVar(11, 20, 340, "X") +
		block(12, 150, 340, "R_TRIG", "edge", []string{"Q"}, pin{param: "CLK", wire: wire(11, "")}) +
		inVar(1, 20, 20, "A") +
		outVar(2, 300, 20, "X", wire(1, ""))
	low, err := LowerGraphical("Order", body, nil, []string{"A", "X", "Y", "edge"})
	if err != nil {
		t.Fatal(err)
	}
	want := "X := A;\nedge(CLK := X);\nY := edge.Q;\n"
	if low.Body != want {
		t.Fatalf("order:\n%s\nwant:\n%s", low.Body, want)
	}
	if fmt.Sprint(low.Lines) != "[2 12 10]" {
		t.Errorf("lines map to elements %v", low.Lines)
	}

	// With executionOrderId on every statement, the file decides.
	explicit := strings.Replace(strings.Replace(strings.Replace(body,
		`<outVariable localId="10"`, `<outVariable localId="10" executionOrderId="3"`, 1),
		`<block localId="12"`, `<block localId="12" executionOrderId="1"`, 1),
		`<outVariable localId="2"`, `<outVariable localId="2" executionOrderId="2"`, 1)
	low, err = LowerGraphical("Order", explicit, nil, []string{"A", "X", "Y", "edge"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "edge(CLK := X);\nX := A;\nY := edge.Q;\n"; low.Body != want {
		t.Fatalf("explicit order:\n%s\nwant:\n%s", low.Body, want)
	}
}

// --- LD ---

// sealIn: one rung, (Start OR Run) AND NOT Stop -> Run.
func sealIn() []byte {
	body := leftRail(1, 0, 0) +
		contact(2, 60, 40, "Start", "", wire(1, "")) +
		contact(3, 60, 100, "Run", "", wire(1, "")) +
		contact(4, 160, 40, "Stop", ` negated="true"`, wire(2, ""), wire(3, "")) +
		coil(5, 260, 40, "Run", "", wire(4, ""))
	return project(fb("SealIn", "LD", vars("inputVars", "Start:BOOL", "Stop:BOOL")+vars("outputVars", "Run:BOOL"), body))
}

func TestLDSealIn(t *testing.T) {
	iec := lower(t, sealIn())
	if want := "Run := (Start OR Run) AND NOT(Stop);"; !strings.Contains(iec, want) {
		t.Fatalf("no %q in\n%s", want, iec)
	}
	decl := "VAR m : SealIn; res : BOOL; END_VAR"
	if !boolValue(t, run(t, iec, decl, "m(Start := TRUE, Stop := FALSE); m(Start := FALSE, Stop := FALSE); res := m.Run; res;")) {
		t.Fatal("Run did not seal in after Start was released")
	}
	if boolValue(t, run(t, iec, decl, "m(Start := TRUE, Stop := FALSE); m(Start := FALSE, Stop := TRUE); res := m.Run; res;")) {
		t.Fatal("Stop did not drop Run")
	}
}

func TestLDCoilsAndEdges(t *testing.T) {
	body := leftRail(1, 0, 0) +
		contact(2, 60, 40, "Go", ` edge="rising"`, wire(1, "")) +
		coil(3, 200, 40, "Latched", ` storage="set"`, wire(2, "")) +
		coil(4, 260, 40, "Count", ` storage="none"`, wire(3, "")) + // power continues through a coil
		contact(5, 60, 120, "Clear", "", wire(1, "")) +
		coil(6, 200, 120, "Latched", ` storage="reset"`, wire(5, "")) +
		contact(7, 60, 200, "Latched", ` negated="true"`, wire(1, "")) +
		coil(8, 200, 200, "Idle", "", wire(7, ""))
	iec := lower(t, project(fb("Coils", "LD",
		vars("inputVars", "Go:BOOL", "Clear:BOOL")+vars("outputVars", "Latched:BOOL", "Idle:BOOL", "Count:BOOL"), body)))
	for _, want := range []string{"_R_TRIG1(CLK := Go);", "IF _R_TRIG1.Q THEN Latched := TRUE; END_IF;",
		"IF Clear THEN Latched := FALSE; END_IF;", "Idle := NOT(Latched);", "_R_TRIG1 : R_TRIG;"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	decl := "VAR c : Coils; a : BOOL; b : BOOL; END_VAR"
	// A held Go sets Latched once; Clear resets it; rungs run top to bottom.
	got := run(t, iec, decl, "c(Go := TRUE, Clear := FALSE); c(Go := TRUE, Clear := TRUE); a := c.Latched; a;")
	if boolValue(t, got) {
		t.Fatal("Latched set again by a held Go, or not reset by Clear")
	}
	got = run(t, iec, decl, "c(Go := TRUE, Clear := FALSE); b := c.Idle; b;")
	if boolValue(t, got) {
		t.Fatal("Idle is not NOT(Latched) of the same scan")
	}
}

func TestLDTimerInRung(t *testing.T) {
	body := leftRail(1, 0, 0) +
		contact(2, 60, 40, "Run", "", wire(1, "")) +
		inVar(3, 60, 100, "T#5s") +
		block(4, 160, 40, "TON", "t1", []string{"Q", "ET"},
			pin{param: "IN", wire: wire(2, "")}, pin{param: "PT", wire: wire(3, "")}) +
		coil(5, 300, 40, "Alarm", "", wire(4, "Q"))
	iec := lower(t, project(fb("Delay", "LD",
		vars("inputVars", "Run:BOOL")+vars("outputVars", "Alarm:BOOL")+vars("localVars", "t1:TON"), body)))
	for _, want := range []string{"t1(IN := Run, PT := T#5s);", "Alarm := t1.Q;"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	if strings.Index(iec, "t1(IN") > strings.Index(iec, "Alarm := t1.Q") {
		t.Fatalf("the coil reads the timer before it runs:\n%s", iec)
	}
	run(t, iec, "VAR d : Delay; END_VAR", "d(Run := TRUE);")
}

func TestUnnamedStandardBlockGetsAnInstance(t *testing.T) {
	body := inVar(1, 20, 20, "Pulse") +
		block(2, 120, 20, "R_TRIG", "", []string{"Q"}, pin{param: "CLK", wire: wire(1, "")}) +
		outVar(3, 240, 20, "Rose", wire(2, "Q"))
	low, err := LowerGraphical("P", body, nil, []string{"Pulse", "Rose"})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(low.Decls) != "[_R_TRIG2 : R_TRIG;]" || !strings.Contains(low.Body, "_R_TRIG2(CLK := Pulse);") {
		t.Fatalf("decls %v, body:\n%s", low.Decls, low.Body)
	}
}

func TestGraphicalErrors(t *testing.T) {
	for name, body := range map[string]string{
		"dangling wire": outVar(1, 0, 0, "X", wire(9, "")),
		"jump":          `<jump localId="1" label="L1"><position x="0" y="0"/></jump>`,
		"duplicate id":  inVar(1, 0, 0, "A") + outVar(1, 0, 0, "X", wire(1, "")),
		"open contact":  contact(1, 0, 0, "A", "") + coil(2, 0, 0, "X", "", wire(1, "")),
		"guarded result into a block": inVar(1, 0, 0, "A") + inVar(4, 0, 0, "B") +
			block(2, 0, 0, "ADD", "", []string{"OUT"}, pin{param: "EN", wire: wire(1, "")}, pin{param: "IN1", wire: wire(4, "")}, pin{param: "IN2", wire: wire(4, "")}) +
			block(3, 0, 0, "MUL", "", []string{"OUT"}, pin{param: "IN1", wire: wire(2, "OUT")}, pin{param: "IN2", wire: wire(4, "")}) +
			outVar(5, 0, 0, "X", wire(3, "OUT")),
	} {
		if _, err := LowerGraphical("P", body, nil, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestGraphicalSFCStillRefused(t *testing.T) {
	data := project(`<pou name="S" pouType="program"><interface/><body><SFC></SFC></body></pou>`)
	if _, err := ImportToIECText(data); err == nil || !strings.Contains(err.Error(), "SFC") {
		t.Fatalf("graphical SFC: %v", err)
	}
}

// TestFBDFunctionWithEN: MOVE with EN is a conditional assignment, and its
// ENO passes the enable on, as drawing tools use it.
func TestFBDFunctionWithEN(t *testing.T) {
	body := inVar(1, 20, 20, "Take") +
		inVar(2, 20, 60, "Sample") +
		block(3, 150, 20, "MOVE", "", []string{"ENO", "OUT"},
			pin{param: "EN", wire: wire(1, "")}, pin{param: "IN", wire: wire(2, "")}) +
		outVar(4, 300, 20, "Held", wire(3, "OUT")) +
		outVar(5, 300, 60, "Taken", wire(3, "ENO"))
	iec := lower(t, project(fb("Hold", "FBD",
		vars("inputVars", "Take:BOOL", "Sample:INT")+vars("outputVars", "Held:INT", "Taken:BOOL"), body)))
	for _, want := range []string{"IF Take THEN Held := Sample; END_IF;", "Taken := Take;"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	decl := "VAR h : Hold; v : INT; END_VAR"
	got := run(t, iec, decl, "h(Take := TRUE, Sample := 7); h(Take := FALSE, Sample := 9); v := h.Held; v;")
	if n := intValue(t, got); n != 7 {
		t.Fatalf("held %d, want 7 (the value taken while EN was TRUE)", n)
	}
}
