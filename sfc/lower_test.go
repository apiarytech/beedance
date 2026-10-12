/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sfc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/sfc"
	"github.com/apiarytech/beedance/sil"
)

// Charts run on the VM as the ST they lower to, and on the evaluator as
// charts; the two must agree scan by scan (software in the loop compares
// every output after every scan).
const charts = `
PROGRAM TEST_Qualifiers
VAR_OUTPUT failures : INT; done : BOOL; scans, n, s, p, d, l, sd, ds, sl : INT; END_VAR
ACTION Tick: scans := scans + 1; END_ACTION
ACTION AN: n := n + 1; END_ACTION
ACTION AS: s := s + 1; END_ACTION
ACTION AP: p := p + 1; END_ACTION
ACTION AD: d := d + 1; END_ACTION
ACTION AL: l := l + 1; END_ACTION
ACTION ASD: sd := sd + 1; END_ACTION
ACTION ADS: ds := ds + 1; END_ACTION
ACTION ASL: sl := sl + 1; END_ACTION
ACTION Finish: done := TRUE; END_ACTION
INITIAL_STEP S0: Tick(N); AN(N); AS(S); AP(P); AD(D, T#50MS); AL(L, T#50MS); ASD(SD, T#30MS); ADS(DS, T#30MS); ASL(SL, T#40MS); END_STEP
TRANSITION FROM S0 TO S1 := scans >= 10; END_TRANSITION
STEP S1: Tick(N); AP(P); END_STEP
TRANSITION FROM S1 TO S2 := scans >= 20; END_TRANSITION
STEP S2: Tick(N); AS(R); ASD(R); ASL(R); END_STEP
TRANSITION FROM S2 TO S3 := scans >= 30; END_TRANSITION
STEP S3: Tick(N); Finish(N); END_STEP
END_PROGRAM

PROGRAM TEST_Branches
VAR_OUTPUT failures : INT; done : BOOL; k, a, b, c, laps, pulses : INT; Lamp : BOOL; END_VAR
ACTION Count: k := k + 1; END_ACTION
ACTION InA: a := a + 1; END_ACTION
ACTION InB: b := b + 1; END_ACTION
ACTION InC: c := c + 1; END_ACTION
ACTION Lap: laps := laps + 1; END_ACTION
ACTION Pulse: pulses := pulses + 1; END_ACTION
ACTION Fin: done := TRUE; END_ACTION
INITIAL_STEP Idle: Count(N); Pulse(P); END_STEP
TRANSITION FROM Idle TO (A, B) := k MOD 4 = 3; END_TRANSITION
STEP A: Count(N); InA(N); Lamp(N); END_STEP
STEP B: InB(N); END_STEP
TRANSITION FROM A TO A2 := k MOD 4 = 1; END_TRANSITION
STEP A2: Count(N); END_STEP
TRANSITION FROM (A2, B) TO C := TRUE; END_TRANSITION
STEP C: Count(N); InC(N); Lap(P); END_STEP
TRANSITION FROM C TO Idle := laps < 3; END_TRANSITION
TRANSITION FROM C TO Stop := laps >= 3; END_TRANSITION
STEP Stop: Fin(N); END_STEP
END_PROGRAM

PROGRAM TEST_Done
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR w : TON; END_VAR
ACTION Wait: w(IN := TRUE, PT := T#200MS); IF w.Q THEN done := TRUE; END_IF; END_ACTION
INITIAL_STEP Init: Wait(N); END_STEP
END_PROGRAM
`

func TestChartsAgreeWithTheEvaluator(t *testing.T) {
	results, err := sil.Run(context.Background(), charts, sil.Options{MaxScans: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("%d results", len(results))
	}
	for _, r := range results {
		for _, e := range r.Engines {
			if e.Err != "" {
				t.Errorf("%s on %s: %s", r.Name, e.Engine, e.Err)
			}
		}
		if r.Mismatch != "" {
			t.Errorf("%s: engines differ: %s", r.Name, r.Mismatch)
		}
		if !r.Passed {
			for _, e := range r.Engines {
				t.Errorf("%s on %s: %v", r.Name, e.Engine, e.Problems())
			}
		}
	}
	// A few values, so agreeing is not agreeing on nothing.
	last := func(name, v string) string {
		for _, r := range results {
			if r.Name == name {
				if sc := r.Engines[1].Scans; len(sc) > 0 {
					return sc[len(sc)-1][v]
				}
			}
		}
		return ""
	}
	for _, c := range []struct{ test, v, want string }{
		{"TEST_Branches", "laps", "3"},
		{"TEST_Branches", "pulses", "3"}, // P on every entry to Idle
		{"TEST_Qualifiers", "s", "20"},   // stored until R in S2
	} {
		if got := last(c.test, c.v); got != c.want {
			t.Errorf("%s.%s = %s, want %s", c.test, c.v, got, c.want)
		}
	}
}

func TestLowerRefuses(t *testing.T) {
	for src, want := range map[string]string{
		"PROGRAM P\nSTEP Init: END_STEP\nEND_PROGRAM":                                                                  "no initial step",
		"PROGRAM P\nINITIAL_STEP Init: END_STEP\nTRANSITION FROM Init TO Nowhere := TRUE; END_TRANSITION\nEND_PROGRAM": "not defined",
		"PROGRAM P\nACTION A: ; END_ACTION\nINITIAL_STEP Init: A(D); END_STEP\nEND_PROGRAM":                            "needs a time",
	} {
		p := parser.New(lexer.New(src))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("%s: %v", src, p.Errors())
		}
		_, err := sfc.Program(prog)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", src, err, want)
		}
	}
}

// The source program is left as it was: the evaluator still runs the chart.
func TestProgramLeavesTheSourceAlone(t *testing.T) {
	p := parser.New(lexer.New("PROGRAM P\nVAR x : INT; END_VAR\nACTION A: x := x + 1; END_ACTION\nINITIAL_STEP Init: A(N); END_STEP\nEND_PROGRAM"))
	prog := p.ParseProgram()
	low, err := sfc.Program(prog)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := prog.Statements[0].(*ast.ProgramDeclaration).Body.(*ast.SFCProgram); !ok {
		t.Error("the source program was changed")
	}
	if _, ok := low.Statements[0].(*ast.ProgramDeclaration).Body.(*ast.BlockStatement); !ok {
		t.Error("the chart was not lowered")
	}
}
