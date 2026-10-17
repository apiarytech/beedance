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
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// inputWrites (IEC-69): a function block clearing its own one-shot request,
// CMD_ACK := FALSE, as beeguard's alarm blocks and OSCAT do. IEC 61131-3
// makes an input read-only in its POU, so every engine refuses it by
// default, alike; with AllowInputWrites (CODESYS's dialect) the input is
// the instance's own copy: a write inside stays until a call passes the
// input again, and a call that does not pass it keeps the written value.
const inputWrites = `
FUNCTION_BLOCK FB_REQ
VAR_INPUT CMD_ACK : BOOL; END_VAR
VAR_OUTPUT acks : INT; seen : BOOL; END_VAR
seen := CMD_ACK;
IF CMD_ACK THEN
  acks := acks + 1;
  CMD_ACK := FALSE;
END_IF;
END_FUNCTION_BLOCK

PROGRAM TEST_InputWrites
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR r : FB_REQ; END_VAR
(* the caller requests between calls; the block clears it once handled *)
r.CMD_ACK := TRUE;
r();
IF r.acks <> 1 OR NOT r.seen OR r.CMD_ACK THEN failures := failures + 1; END_IF;
(* a call that does not pass the input keeps the value written inside *)
r();
IF r.acks <> 1 OR r.seen OR r.CMD_ACK THEN failures := failures + 1; END_IF;
(* a call that passes it sets it again *)
r(CMD_ACK := TRUE);
IF r.acks <> 2 OR NOT r.seen OR r.CMD_ACK THEN failures := failures + 1; END_IF;
r();
IF r.acks <> 2 OR r.seen THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`

func inputWriteEngines() []sil.Engine {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	return engines
}

// TestInputWritesRefusedOnEveryEngine (IEC-69): by default every engine
// refuses the write, with the same message.
func TestInputWritesRefusedOnEveryEngine(t *testing.T) {
	engines := inputWriteEngines()
	results, err := sil.Run(context.Background(), inputWrites, sil.Options{Engines: engines, MaxScans: 3, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) {
			t.Fatalf("%d engines, want %d", len(r.Engines), len(engines))
		}
		for _, e := range r.Engines {
			if !strings.Contains(e.Err, "cannot assign to read-only variable 'CMD_ACK'") {
				t.Errorf("%s: %q, want the write refused", e.Engine, e.Err)
			}
		}
	}
}

// TestInputWritesAllowedOnEveryEngine (IEC-69): with the option, the engines
// agree on CODESYS's semantics.
func TestInputWritesAllowedOnEveryEngine(t *testing.T) {
	engines := inputWriteEngines()
	results, err := sil.Run(context.Background(), inputWrites, sil.Options{Engines: engines, MaxScans: 3, GoTimeout: time.Minute, AllowInputWrites: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) || !r.Passed {
			t.Errorf("%s: passed %v on %d engines, mismatch %q", r.Name, r.Passed, len(r.Engines), r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v %s", e.Engine, e.Problems(), e.Err)
			}
		}
	}
}
