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
