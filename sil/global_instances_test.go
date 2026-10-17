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
	"os/exec"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// globalInstances: function block instances declared VAR_GLOBAL, alone,
// with an initializer, standard, and in an array, start as a program's
// instances do: with their initial values, their initializer and EN TRUE.
const globalInstances = `
FUNCTION_BLOCK FB_G
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT := 1; END_VAR
y := y + x;
END_FUNCTION_BLOCK

VAR_GLOBAL
  gf : FB_G;
  gi : FB_G := (x := 2);
  gt : TON := (PT := T#20ms);
  ga : ARRAY[0..1] OF FB_G;
END_VAR

PROGRAM TEST_GlobalInstances
VAR_OUTPUT failures : INT; done : BOOL; n : INT; END_VAR
n := n + 1;
gf(x := 3); gi(); gt(IN := TRUE); ga[1](x := 1);
IF n = 1 AND (gf.y <> 4 OR gi.y <> 3 OR ga[1].y <> 2 OR ga[0].y <> 1 OR gt.Q) THEN failures := failures + 1; END_IF;
IF n = 5 THEN
  IF NOT gt.Q THEN failures := failures + 1; END_IF;
  done := TRUE;
END_IF;
END_PROGRAM
`

// TestGlobalInstancesOnEveryEngine runs the global instances on the
// evaluator, the VM and the transpiled Go (where the Go toolchain is),
// which agree scan by scan.
func TestGlobalInstancesOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), globalInstances, sil.Options{Engines: engines, MaxScans: 100, GoTimeout: time.Minute})
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
