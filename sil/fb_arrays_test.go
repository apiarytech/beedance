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

// fbArrays: arrays of function block instances, ARRAY [..] OF MyFB and
// ARRAY [..] OF TON, in a function block and in a program. Each element is
// an instance of its own, starts with the block's initial values and EN
// TRUE, is called as steps[i](...) and read as steps[i].out.
const fbArrays = `
FUNCTION_BLOCK FB_ACC
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT := 100; q : BOOL; END_VAR
VAR t : TON; END_VAR
t(IN := TRUE, PT := T#20ms);
q := t.Q;
y := y + x;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_BANK
VAR_OUTPUT s : INT; q1 : BOOL; tq : BOOL; corner : INT; END_VAR
VAR
  acc : ARRAY[0..3] OF FB_ACC;
  tons : ARRAY[1..2] OF TON;
  grid : ARRAY[0..1, 1..2] OF FB_ACC;
  k : INT;
END_VAR
FOR k := 0 TO 3 DO acc[k](x := k); END_FOR;
tons[2](IN := TRUE, PT := T#30ms);
grid[1, 2](x := 5);
tq := tons[2].Q;
s := acc[3].y;
q1 := acc[1].q;
corner := grid[1, 2].y;
END_FUNCTION_BLOCK

PROGRAM TEST_FBArrayInFB
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR b : FB_BANK; n : INT; END_VAR
b();
n := n + 1;
IF n = 1 AND (b.s <> 103 OR b.corner <> 105 OR b.q1 OR b.tq) THEN failures := failures + 1; END_IF;
IF n = 2 AND (b.s <> 106 OR b.corner <> 110) THEN failures := failures + 1; END_IF;
IF n = 6 THEN
  IF NOT b.q1 OR NOT b.tq THEN failures := failures + 1; END_IF;
  done := TRUE;
END_IF;
END_PROGRAM

PROGRAM TEST_FBArrayInProgram
VAR_OUTPUT failures : INT; done : BOOL; y2 : INT; END_VAR
VAR acc : ARRAY[1..3] OF FB_ACC; END_VAR
acc[2](x := 2);
acc[2]();
y2 := acc[2].y;
IF y2 <> 104 OR acc[1].y <> 100 THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`

// TestFunctionBlockArraysOnEveryEngine runs the arrays of instances on the
// evaluator, the VM and the transpiled Go (where the Go toolchain is),
// which agree scan by scan.
func TestFunctionBlockArraysOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), fbArrays, sil.Options{Engines: engines, MaxScans: 100, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("%d tests, want 2", len(results))
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
