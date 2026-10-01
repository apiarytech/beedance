/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"testing"
	"time"

	"beedance/stdlib"
)

func TestStandardFunctionBlocks(t *testing.T) {
	// Each read of the clock moves it on by 400ms.
	saved := stdlib.Clock
	defer func() { stdlib.Clock = saved }()
	var now time.Duration
	stdlib.Clock = func() time.Duration {
		now += 400 * time.Millisecond
		return now
	}

	runVmTests(t, []vmTestCase{
		// TON: Q is set once IN has been TRUE for PT, and ET is capped at PT.
		{"VAR t : TON; a, b : BOOL; END_VAR t(IN := TRUE, PT := T#1s); t(IN := TRUE); a := t.Q; t(IN := TRUE); b := t.Q; NOT a AND b AND t.ET = T#1s;", true},
		{"VAR t : TON; END_VAR t(IN := TRUE, PT := T#1s); t(IN := FALSE); t.ET = T#0s AND NOT t.Q;", true},
		// TOF: Q stays set for PT after IN falls, and is not set before IN rises.
		{"VAR t : TOF; a, b, c : BOOL; END_VAR a := t.Q; t(IN := FALSE, PT := T#1s); a := a OR t.Q; t(IN := TRUE); b := t.Q; t(IN := FALSE); t(IN := FALSE); c := t.Q; t(IN := FALSE); NOT a AND b AND c AND NOT t.Q;", true},
		// TP: a pulse of PT on a rising edge of IN, even when IN falls.
		{"VAR t : TP; a, b : BOOL; END_VAR t(IN := TRUE, PT := T#1s); a := t.Q; t(IN := FALSE); b := t.Q; t(IN := FALSE); a AND b AND NOT t.Q AND t.ET = T#0s;", true},
		// Counters count rising edges.
		{"VAR c : CTU; END_VAR c(CU := TRUE, PV := 2); c(CU := FALSE); c(CU := TRUE); c(CU := TRUE); c.Q AND c.CV = 2;", true},
		{"VAR c : CTU; END_VAR c(CU := TRUE, PV := 2); c(R := TRUE); c.CV;", 0},
		{"VAR c : CTD; END_VAR c(LD := TRUE, PV := 2); c(LD := FALSE, CD := TRUE); c(CD := FALSE); c(CD := TRUE); c.Q AND c.CV = 0;", true},
		{"VAR c : CTUD; END_VAR c(CU := TRUE, PV := 3); c(CU := FALSE); c(CU := TRUE); c(CU := FALSE, CD := TRUE); c.CV;", 1},
		{"VAR c : CTUD; END_VAR c(LD := TRUE, PV := 3); c.QU AND NOT c.QD;", true},
		// Edge triggers.
		{"VAR r : R_TRIG; a, b : BOOL; END_VAR r(CLK := TRUE); a := r.Q; r(CLK := TRUE); b := r.Q; a AND NOT b;", true},
		{"VAR f : F_TRIG; a, b : BOOL; END_VAR f(CLK := FALSE); a := f.Q; f(CLK := TRUE); f(CLK := FALSE); b := f.Q; NOT a AND b;", true},
		// Bistables: SR is set dominant, RS reset dominant.
		{"VAR s : SR; END_VAR s(S1 := TRUE, R := TRUE); s.Q1;", true},
		{"VAR s : RS; END_VAR s(S := TRUE, R1 := TRUE); s.Q1;", false},
		{"VAR s : RS; END_VAR s(S := TRUE); s(S := FALSE); s.Q1;", true},
		// A program's own FB of the same name is used instead.
		{"FUNCTION_BLOCK TON VAR_OUTPUT Q : INT; END_VAR Q := 7; END_FUNCTION_BLOCK VAR t : TON; END_VAR t(); t.Q;", 7},
		// Standard FBs inside a user FB.
		{"FUNCTION_BLOCK Edge VAR_INPUT x : BOOL; END_VAR VAR_OUTPUT q : BOOL; END_VAR VAR r : R_TRIG; END_VAR r(CLK := x); q := r.Q; END_FUNCTION_BLOCK VAR e : Edge; END_VAR e(x := TRUE); e.q;", true},
	})
}

// TestTimeFunction checks TIME(), the CODESYS clock OSCAT's T_PLC_MS reads.
func TestTimeFunction(t *testing.T) {
	saved := stdlib.Clock
	defer func() { stdlib.Clock = saved }()
	stdlib.Clock = func() time.Duration { return 1500 * time.Millisecond }

	runVmTests(t, []vmTestCase{
		{"TIME() = T#1.5s;", true},
		{"FUNCTION T_PLC_MS : DWORD VAR tx : TIME; END_VAR tx := TIME(); T_PLC_MS := TIME_TO_DWORD(tx); END_FUNCTION T_PLC_MS() = 1500;", true},
	})
}

// TestDayDurations checks that a TIME literal may have days, as IEC 61131-3 allows.
func TestDayDurations(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"T#1d = T#24h;", true},
		{"T#1d_2h - T#2h = TIME#1d;", true},
	})
}
