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

// fbReturn (IEC-74): RETURN in a function block ends the block's body, not
// the program that called it; its outputs keep what was set before it.
const fbReturn = `
FUNCTION_BLOCK FB_R
VAR_INPUT a : BOOL; END_VAR
VAR_OUTPUT x : INT; calls : INT; END_VAR
calls := calls + 1;
x := 1;
IF a THEN RETURN; END_IF;
x := 2;
END_FUNCTION_BLOCK

PROGRAM TEST_FbReturn
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR f : FB_R; END_VAR
f(a := TRUE);
IF f.x <> 1 THEN failures := failures + 1; END_IF;
f(a := FALSE);
IF f.x <> 2 OR f.calls <> 2 THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`

func TestFbReturnOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), fbReturn, sil.Options{Engines: engines, MaxScans: 5, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) || !r.Passed {
			t.Errorf("%s: passed %v, %d engines, mismatch %q", r.Name, r.Passed, len(r.Engines), r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v", e.Engine, e.Problems())
			}
		}
	}
}
