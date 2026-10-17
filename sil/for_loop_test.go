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

// forLoops: a FOR loop's control variable is the declared variable, in a
// program and in a function block. After the loop it is one step past the
// end value (or where EXIT left it), and the body may change it.
const forLoops = `
FUNCTION_BLOCK FB_LOOP
VAR_OUTPUT after : INT; END_VAR
VAR k : INT; END_VAR
FOR k := 1 TO 5 DO after := k; END_FOR;
after := k;
END_FUNCTION_BLOCK

PROGRAM TEST_ForVariable
VAR_OUTPUT failures : INT; done : BOOL; up, down, skip, quit, fb : INT; END_VAR
VAR i, s : INT; f : FB_LOOP; END_VAR
FOR i := 0 TO 3 DO s := s + i; END_FOR;
up := i;
FOR i := 10 TO 1 BY -2 DO s := s + i; END_FOR;
down := i;
FOR i := 0 TO 9 DO i := i + 1; END_FOR;
skip := i;
FOR i := 0 TO 9 DO IF i = 4 THEN EXIT; END_IF; END_FOR;
quit := i;
f();
fb := f.after;
IF up <> 4 OR down <> 0 OR skip <> 10 OR quit <> 4 OR fb <> 6 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`

// TestForVariableOnEveryEngine runs the loops on the evaluator, the VM and
// the transpiled Go (where the Go toolchain is), which agree.
func TestForVariableOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), forLoops, sil.Options{Engines: engines, MaxScans: 10, GoTimeout: time.Minute})
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
