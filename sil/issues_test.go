/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil_test

import (
	"context"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// The programs below are the reports of GitHub issues, run on every engine:
// the evaluator, the VM and the transpiled Go (where the Go toolchain is).

// runOnEveryEngine runs the TEST_ programs in src and fails on a failed
// check or a difference between the engines.
func runOnEveryEngine(t *testing.T, src string) {
	t.Helper()
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if sil.GoEngineAvailable() == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), src, sil.Options{Engines: engines, MaxScans: 200, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("%s failed: mismatch %q", r.Name, r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v", e.Engine, e.Problems())
			}
		}
	}
}

// Issue #17: untyped literals above LINT's range are ULINT or LWORD values,
// and -9223372036854775808 is LINT's minimum.
const bigLiterals = `
PROGRAM TEST_BigLiterals
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR v : ULINT; w : LWORD; t : ULINT := ULINT#18446744073709551615; END_VAR
v := 16#FFFFFFFFFFFFFFFF;
IF v <> t THEN failures := failures + 1; message := 'based'; END_IF;
v := 18446744073709551615;
IF v <> t THEN failures := failures + 1; message := 'decimal'; END_IF;
w := 16#FFFFFFFFFFFFFFFF;
IF w <> LWORD#16#FFFFFFFFFFFFFFFF THEN failures := failures + 1; message := 'lword'; END_IF;
v := 9223372036854775808;
IF v <> ULINT#9223372036854775808 THEN failures := failures + 1; message := '2^63'; END_IF;
END_PROGRAM
PROGRAM TEST_LintMin
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR l : LINT; END_VAR
l := -9223372036854775808;
IF l <> LINT#-9223372036854775808 THEN failures := failures + 1; message := 'min'; END_IF;
l := -9223372036854775807;
IF l + 1 <> -9223372036854775806 THEN failures := failures + 1; message := 'near min'; END_IF;
END_PROGRAM
`

func TestBigLiteralsIssue17(t *testing.T) { runOnEveryEngine(t, bigLiterals) }

// Issue #16: a power of literals assigned to a narrower integer compiles in
// the transpiled Go and gives the VM's value.
const powerLiterals = `
PROGRAM TEST_PowerLiterals
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR p1 : DINT; p2 : INT; a : DINT := 2; r : LREAL; END_VAR
p1 := 2 ** 3;
IF p1 <> 8 THEN failures := failures + 1; message := '2**3'; END_IF;
p1 := (-2) ** 2;
IF p1 <> 4 THEN failures := failures + 1; message := '(-2)**2'; END_IF;
p2 := 3 ** 2;
IF p2 <> 9 THEN failures := failures + 1; message := 'INT'; END_IF;
p1 := a ** 2;
IF p1 <> 4 THEN failures := failures + 1; message := 'a**2'; END_IF;
r := 2 ** 0.5;
IF r < 1.414 OR r > 1.415 THEN failures := failures + 1; message := 'real'; END_IF;
END_PROGRAM
`

func TestPowerLiteralsIssue16(t *testing.T) { runOnEveryEngine(t, powerLiterals) }

// Issue #14: a FOR control variable keeps its type past the end of its
// range (it wraps, as an assignment does) on every engine, and a negative
// step on an unsigned control variable compiles in the transpiled Go.
const forAtTypeLimits = `
PROGRAM TEST_AssignWraps
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR i : SINT := 127; END_VAR
i := i + 1;
IF i <> -128 THEN failures := 1; message := 'assign'; END_IF;
END_PROGRAM

PROGRAM TEST_ForSintMax
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR i : SINT; n : INT; END_VAR
FOR i := 120 TO 127 DO n := n + 1; IF n > 20 THEN EXIT; END_IF; END_FOR;
IF n <> 21 THEN failures := 1; message := 'iterations'; END_IF;
END_PROGRAM

PROGRAM TEST_ForUsintDown
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR i : USINT; n : INT; END_VAR
FOR i := 255 TO 0 BY -1 DO n := n + 1; IF n > 300 THEN EXIT; END_IF; END_FOR;
IF n <> 301 THEN failures := 1; message := 'iterations'; END_IF;
END_PROGRAM
`

func TestForAtTypeLimitsIssue14(t *testing.T) { runOnEveryEngine(t, forAtTypeLimits) }

// CTU, CTD and CTUD count past PV, up to PVmax and down to PVmin, their
// type's limits (IEC 61131-3: CV < PVmax, CV > PVmin); the evaluator and the
// VM used to stop at PV and 0, while royaljelly (the Go engine) counts on.
const countersPastPV = `
PROGRAM TEST_CtuCountsPastPV
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR c : CTU; d : CTD; ud : CTUD; c2 : CTU_DINT; scans : INT; END_VAR
scans := scans + 1;
c(CU := (scans MOD 2) = 0, R := FALSE, PV := 3);
d(CD := (scans MOD 2) = 0, LD := scans = 1, PV := 2);
ud(CU := (scans MOD 2) = 0, CD := FALSE, R := FALSE, LD := FALSE, PV := 3);
c2(CU := (scans MOD 2) = 0, R := FALSE, PV := 3);
IF scans >= 20 THEN
  done := TRUE;
  IF c.CV <> 10 OR NOT c.Q THEN failures := failures + 1; message := 'CTU'; END_IF;
  IF d.CV <> -8 OR NOT d.Q THEN failures := failures + 1; message := 'CTD'; END_IF;
  IF ud.CV <> 10 OR NOT ud.QU THEN failures := failures + 1; message := 'CTUD'; END_IF;
  IF c2.CV <> 10 THEN failures := failures + 1; message := 'CTU_DINT'; END_IF;
END_IF;
END_PROGRAM

`

func TestCountersPastPV(t *testing.T) { runOnEveryEngine(t, countersPastPV) }

// TP is not retriggerable: an edge of IN during a pulse is ignored, and ET
// stays PT after the pulse while IN is TRUE. royaljelly's TP does this from
// v0.3.3 (its issue #31); until RoyaljellyVersion is that, the test runs on
// the evaluator and the VM.
const tpNotRetriggerable = `
PROGRAM TEST_TpNotRetriggerable
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR p : TP; scans : INT; inp : BOOL; END_VAR
scans := scans + 1;
(* 10 ms scans: IN high 1-20, low 21-40, high again from 41. PT = 1 s. *)
inp := scans <= 20 OR scans > 40;
p(IN := inp, PT := T#1s);
IF scans = 50 AND (NOT p.Q OR p.ET <> T#490ms) THEN failures := failures + 1; message := 'retriggered'; END_IF;
IF scans = 105 AND (p.Q OR p.ET <> T#1s) THEN failures := failures + 1; message := 'ET after pulse'; END_IF;
IF scans >= 150 THEN done := TRUE; END_IF;
END_PROGRAM
`

func TestTPNotRetriggerable(t *testing.T) {
	results, err := sil.Run(context.Background(), tpNotRetriggerable, sil.Options{Engines: []sil.Engine{sil.Evaluator, sil.VM}, MaxScans: 200})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("%s failed: mismatch %q", r.Name, r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v", e.Engine, e.Problems())
			}
		}
	}
}

// Issues #10 and #13: a conversion rounds a REAL half to even on every engine,
// and one that does not fit its target is a runtime error on every engine.
const conversionsInRange = `
PROGRAM TEST_InRange
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR i : INT := 300; s : SINT; r : REAL := 2.5; w : WORD; d : DINT; u : USINT; t : TIME := T#1s500ms; b : BYTE; l : LREAL := -2.5; END_VAR
s := INT_TO_SINT(INT#100);
IF s <> 100 THEN failures := failures + 1; message := 'int'; END_IF;
i := REAL_TO_INT(r);
IF i <> 2 THEN failures := failures + 1; message := 'half even'; END_IF;
i := LREAL_TO_INT(l);
IF i <> -2 THEN failures := failures + 1; message := 'half even neg'; END_IF;
w := INT_TO_WORD(INT#5);
IF w <> WORD#5 THEN failures := failures + 1; message := 'word'; END_IF;
d := TIME_TO_DINT(t);
IF d <> 1500 THEN failures := failures + 1; message := 'time'; END_IF;
u := WORD_TO_USINT(WORD#200);
IF u <> 200 THEN failures := failures + 1; message := 'word usint'; END_IF;
b := DWORD_TO_BYTE(DWORD#16#1FF);
IF b <> BYTE#16#FF THEN failures := failures + 1; message := 'mask'; END_IF;
END_PROGRAM

`

func TestConversionsInRangeIssue13(t *testing.T) { runOnEveryEngine(t, conversionsInRange) }

func TestConversionOutOfRangeIsAnErrorIssue10(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if sil.GoEngineAvailable() == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	for _, conv := range []string{"i := REAL_TO_INT(40000.0);", "s := INT_TO_SINT(n);", "w := INT_TO_WORD(m);", "i := TIME_TO_INT(T#40s);", "i := LREAL_TO_INT(LREAL#1.0E30);"} {
		src := "PROGRAM TEST_Out\nVAR_OUTPUT failures : INT; END_VAR\nVAR i : INT; s : SINT; w : WORD; n : INT := 300; m : INT := -5; END_VAR\n" + conv + "\nEND_PROGRAM\n"
		results, err := sil.Run(context.Background(), src, sil.Options{Engines: engines, MaxScans: 1, GoTimeout: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range results {
			for _, e := range r.Engines {
				if e.Err == "" {
					t.Errorf("%s on %s: no runtime error", conv, e.Engine)
				}
			}
		}
	}
}
