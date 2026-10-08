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
	"encoding/xml"
	"strings"
	"testing"
)

func keep(t *testing.T, xmlData []byte) string {
	t.Helper()
	iec, err := ImportToIECTextOptions(xmlData, ImportOptions{KeepDiagrams: true})
	if err != nil {
		t.Fatal(err)
	}
	return iec
}

// A ladder imported with KeepDiagrams stays a ladder, and runs as the
// lowered one does.
func TestImportKeepsALadder(t *testing.T) {
	iec := keep(t, sealIn())
	for _, want := range []string{"\tLD\n", "[ Start | Run ] /Stop ( Run )", "\tEND_LD\n"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	decl := "VAR m : SealIn; res : BOOL; END_VAR"
	if !boolValue(t, run(t, iec, decl, "m(Start := TRUE, Stop := FALSE); m(Start := FALSE, Stop := FALSE); res := m.Run; res;")) {
		t.Fatal("Run did not seal in after Start was released")
	}
	if boolValue(t, run(t, iec, decl, "m(Start := TRUE, Stop := FALSE); m(Start := FALSE, Stop := TRUE); res := m.Run; res;")) {
		t.Fatal("Stop did not drop Run")
	}
}

func TestImportKeepsAnFBD(t *testing.T) {
	iec := keep(t, counterFBD())
	for _, want := range []string{"\tFBD\n", "Cnt := SEL(Reset, ADD(Cnt, 1), 0)", "OUT := Cnt", "\tEND_FBD\n"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q in\n%s", want, iec)
		}
	}
	got := run(t, iec, "VAR c : CounterFbd; n : INT; END_VAR", "c(Reset := FALSE); c(Reset := FALSE); c(Reset := FALSE); n := c.OUT; n;")
	if n := intValue(t, got); n != 3 {
		t.Fatalf("counted %d, want 3", n)
	}
}

// A rung the text form cannot hold is lowered, and the IEC text says why.
func TestImportFallsBackToST(t *testing.T) {
	body := leftRail(1, 0, 0) +
		contact(2, 60, 40, "A", ` edge="rising"`, wire(1, "")) +
		contact(3, 60, 100, "B", "", wire(1, "")) +
		contact(4, 120, 70, "M", "", wire(2, ""), wire(3, "")) +
		contact(5, 180, 40, "C", "", wire(2, ""), wire(4, "")) +
		coil(6, 240, 40, "Out", "", wire(5, ""), wire(3, ""))
	iec := keep(t, project(fb("Bridge", "LD", vars("inputVars", "A:BOOL", "B:BOOL", "M:BOOL", "C:BOOL")+vars("outputVars", "Out:BOOL"), body)))
	if !strings.Contains(iec, "(* The LD diagram is imported as Structured Text:") || strings.Contains(iec, "\tLD\n") {
		t.Fatalf("no fallback to ST:\n%s", iec)
	}
	run(t, iec, "VAR b : Bridge; END_VAR", "b(A := TRUE, B := FALSE, M := FALSE, C := TRUE);")
}

const ladderSource = `FUNCTION_BLOCK Motor
VAR_INPUT Start, Stop, Mode : BOOL; END_VAR
VAR_OUTPUT Run, Done : BOOL; Starts : INT; END_VAR
LD
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S) ( Done )
  RUNG count
    +Run cu:CTU(PV := 100, CV => Starts)
END_LD
END_FUNCTION_BLOCK
`

// A body in the LD text form exports as a graphical ladder, declares its
// instances but not the lowering's helpers, and imports back as the same
// ladder.
func TestExportWritesALadder(t *testing.T) {
	data, err := ExportSourceToXML(ladderSource, "Plant")
	if err != nil {
		t.Fatal(err)
	}
	x := string(data)
	for _, want := range []string{"<LD>", "<leftPowerRail", "<rightPowerRail", `<contact localId=`, `instanceName="t1"`, `typeName="CTU"`, `<variable name="t1">`, `<variable name="cu">`} {
		if !strings.Contains(x, want) {
			t.Fatalf("no %q in\n%s", want, x)
		}
	}
	if strings.Contains(x, "_R_TRIG") || strings.Contains(x, "RUNG") {
		t.Fatalf("the lowering's helpers or the text form in the XML:\n%s", x)
	}
	proj, err := Import(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateProject(proj); err != nil {
		t.Fatalf("not a valid project: %v", err)
	}
	var check struct{}
	if err := xml.Unmarshal(data, &check); err != nil {
		t.Fatal(err)
	}

	iec := keep(t, data)
	for _, want := range []string{"[ Start | Run ] /Stop ( Run )", "t1:TON(PT := T#5S) ( Done )", "+Run cu:CTU(PV := 100, CV => Starts)"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q after the round trip:\n%s", want, iec)
		}
	}
	decl := "VAR m : Motor; res : BOOL; END_VAR"
	if !boolValue(t, run(t, iec, decl, "m(Start := TRUE, Stop := FALSE); m(Start := FALSE, Stop := FALSE); res := m.Run; res;")) {
		t.Fatal("the round-tripped ladder does not seal in")
	}
}

func TestExportWritesAnFBD(t *testing.T) {
	src := `FUNCTION_BLOCK Guard
VAR_INPUT Temp, Limit : REAL; END_VAR
VAR_OUTPUT Alarm : BOOL; END_VAR
FBD
  hot = GT(Temp, Limit)
  t1 : TON(IN := hot, PT := T#5S)
  Alarm := OR(t1.Q, Alarm)
END_FBD
END_FUNCTION_BLOCK
`
	data, err := ExportSourceToXML(src, "Plant")
	if err != nil {
		t.Fatal(err)
	}
	x := string(data)
	for _, want := range []string{"<FBD>", `<connector name="hot"`, `<continuation name="hot"`, `<outVariable`, `typeName="TON"`} {
		if !strings.Contains(x, want) {
			t.Fatalf("no %q in\n%s", want, x)
		}
	}
	iec := keep(t, data)
	for _, want := range []string{"t1 : TON(IN := hot, PT := T#5S)", "Alarm := OR(t1.Q, Alarm)"} {
		if !strings.Contains(iec, want) {
			t.Fatalf("no %q after the round trip:\n%s", want, iec)
		}
	}
	run(t, iec, "VAR g : Guard; END_VAR", "g(Temp := 90.0, Limit := 80.0);")
}
