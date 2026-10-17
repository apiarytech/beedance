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

// stJumps (IEC-68): JMP label; and label: in Structured Text, as IL has
// them: a forward jump out of an IF to the end of a block (the alarm
// blocks' CLEAR_COMMANDS pattern), a backward jump making a loop, a jump
// out of a FOR, and one out of nested IFs in a function.
const stJumps = `
FUNCTION_BLOCK FB_ALARM
VAR_INPUT IN_ENABLE : BOOL; CMD_ACK : BOOL; END_VAR
VAR_OUTPUT active : BOOL; acks : INT; cleared : INT; END_VAR
IF NOT IN_ENABLE THEN
  JMP CLEAR_COMMANDS;
END_IF;
active := TRUE;
IF CMD_ACK THEN acks := acks + 1; END_IF;
CLEAR_COMMANDS:
cleared := cleared + 1;
IF NOT IN_ENABLE THEN active := FALSE; END_IF;
END_FUNCTION_BLOCK

FUNCTION FirstAbove : INT
VAR_INPUT limit : INT; END_VAR
VAR k : INT; END_VAR
FirstAbove := -1;
FOR k := 1 TO 100 DO
  IF k * k > limit THEN
    IF k > 0 THEN
      FirstAbove := k;
      JMP DONE;
    END_IF;
  END_IF;
END_FOR;
FirstAbove := -2;
DONE:
END_FUNCTION

PROGRAM TEST_StJumps
VAR_OUTPUT failures : INT; done : BOOL; loops : INT; outOfFor : INT; first : INT; END_VAR
VAR a : FB_ALARM; i : INT; k : INT; n : INT; END_VAR
n := n + 1;
(* the alarm block, disabled then enabled *)
a(IN_ENABLE := n > 1, CMD_ACK := n = 3);
IF n = 1 AND (a.active OR a.acks <> 0 OR a.cleared <> 1) THEN failures := failures + 1; END_IF;
IF n = 3 AND (NOT a.active OR a.acks <> 1 OR a.cleared <> 3) THEN failures := failures + 1; END_IF;
(* a backward jump: a loop of five *)
i := 0;
AGAIN:
i := i + 1;
IF i < 5 THEN JMP AGAIN; END_IF;
loops := i;
(* out of a FOR *)
outOfFor := 0;
FOR k := 1 TO 10 DO
  outOfFor := k;
  IF k = 3 THEN JMP AFTER_FOR; END_IF;
END_FOR;
outOfFor := 99;
AFTER_FOR:
first := FirstAbove(limit := 20);
IF loops <> 5 OR outOfFor <> 3 OR first <> 5 THEN failures := failures + 1; END_IF;
IF n = 4 THEN done := TRUE; END_IF;
END_PROGRAM
`

// TestStJumpsOnEveryEngine (IEC-68) runs the jumps on the evaluator, the VM
// and the transpiled Go (where the Go toolchain is), which agree scan by
// scan.
func TestStJumpsOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), stJumps, sil.Options{Engines: engines, MaxScans: 20, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("%d tests, want 1", len(results))
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) {
			t.Errorf("%s ran on %d engines, want %d", r.Name, len(r.Engines), len(engines))
		}
		if !r.Passed {
			t.Errorf("%s failed: mismatch %q", r.Name, r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v", e.Engine, e.Problems())
			}
		}
	}
}
