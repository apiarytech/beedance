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

// bistables (IEC-1): SR is set dominant, Q1 := S1 OR (NOT R AND Q1); RS is
// reset dominant, Q1 := NOT R1 AND (S OR Q1). Each scan drives both with
// the same set and reset, through set, hold, reset, hold and both at once.
// A variable named like a block type (sr, rs, ctr, ignoring case) does not
// hide the type from an instance declared with it.
const bistables = `
FUNCTION_BLOCK CTR
VAR_OUTPUT n : INT; END_VAR
n := n + 1;
END_FUNCTION_BLOCK

PROGRAM TEST_Bistables
VAR_OUTPUT failures : INT; done : BOOL; sr, rs : BOOL; ctr : INT; END_VAR
VAR
  a : SR; b : RS; c : CTR;
  scan : INT;
  set, reset, wantSR, wantRS : BOOL;
END_VAR
CASE scan OF
  0: set := TRUE;  reset := FALSE; wantSR := TRUE;  wantRS := TRUE;
  1: set := FALSE; reset := FALSE; wantSR := TRUE;  wantRS := TRUE;
  2: set := FALSE; reset := TRUE;  wantSR := FALSE; wantRS := FALSE;
  3: set := FALSE; reset := FALSE; wantSR := FALSE; wantRS := FALSE;
  4: set := TRUE;  reset := TRUE;  wantSR := TRUE;  wantRS := FALSE;
  5: set := FALSE; reset := FALSE; wantSR := TRUE;  wantRS := FALSE;
END_CASE;
a(S1 := set, R := reset);
b(S := set, R1 := reset);
c();
sr := a.Q1;
rs := b.Q1;
ctr := c.n;
IF sr <> wantSR OR rs <> wantRS OR ctr <> scan + 1 THEN failures := failures + 1; END_IF;
scan := scan + 1;
done := scan > 5;
END_PROGRAM
`

// TestBistablesOnEveryEngine runs SR and RS on the evaluator, the VM and
// the transpiled Go (where the Go toolchain is), which agree scan by scan.
func TestBistablesOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if sil.GoEngineAvailable() == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), bistables, sil.Options{Engines: engines, MaxScans: 20, GoTimeout: time.Minute})
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
