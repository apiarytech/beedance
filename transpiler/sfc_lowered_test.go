package transpiler

import (
	"regexp"
	"strings"
	"testing"
)

var stepName = regexp.MustCompile(`(?m)^\s*(?:INITIAL_)?STEP\s+([A-Za-z_][A-Za-z0-9_]*)`)

// checkLoweredChart transpiles a program whose body is a chart: it goes
// to Go as the Structured Text package sfc lowers it to, each step a BOOL
// _sfc_<step>_X, with none of the transpiler's earlier chart machinery.
func checkLoweredChart(t *testing.T, input string) {
	t.Helper()
	got, err := transpileSource(t, input)
	if err != nil {
		t.Fatalf("transpile: %v", err)
	}
	if strings.Contains(got, "sfcActiveSteps") {
		t.Errorf("the chart was not lowered:\n%s", got)
	}
	steps := stepName.FindAllStringSubmatch(input, -1)
	if len(steps) == 0 {
		t.Fatal("no steps in the input")
	}
	for _, s := range steps {
		if !strings.Contains(got, "_sfc_"+s[1]+"_X") {
			t.Errorf("no flag for step %s:\n%s", s[1], got)
		}
	}
}

// TestChartQualifiersTranspile: every qualifier a chart may use goes to Go,
// P1 and P0 (IEC 61131-3 3rd edition) among them.
func TestChartQualifiersTranspile(t *testing.T) {
	checkLoweredChart(t, `PROGRAM Q VAR n : INT; END_VAR
ACTION A: n := n + 1; END_ACTION
INITIAL_STEP S1: A(N); A(P1); A(P0); A(DS, T#1s); A(SD, T#1s); A(SL, T#1s); A(D, T#1s); A(L, T#1s); END_STEP
TRANSITION FROM S1 TO S2 := TRUE; END_TRANSITION
STEP S2: A(R); A(S); A(P); END_STEP
TRANSITION FROM S2 TO S1 := TRUE; END_TRANSITION
END_PROGRAM`)
}
