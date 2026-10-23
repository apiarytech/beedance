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

// ilPOUs (IEC-70): IL bodies in a FUNCTION, a FUNCTION_BLOCK and a METHOD,
// with a loop by JMPC, a RETC and a CAL, on every engine (the Go engine
// transpiled IL only in a PROGRAM).
const ilPOUs = `
FUNCTION SumTo : INT
VAR_INPUT n : INT; END_VAR
VAR i : INT; END_VAR
    LD 0
    ST SumTo
    ST i
AGAIN:
    LD i
    GE n
    RETC
    LD i
    ADD 1
    ST i
    LD SumTo
    ADD i
    ST SumTo
    JMP AGAIN
END_FUNCTION

FUNCTION_BLOCK FB_Count
VAR_INPUT pulse : BOOL; END_VAR
VAR_OUTPUT count : INT; on : BOOL; END_VAR
VAR t : TON; END_VAR
    LD pulse
    JMPCN SKIP
    LD count
    ADD 1
    ST count
SKIP:
    CAL t(IN := pulse, PT := T#20ms)
    LD t.Q
    ST on
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Math
VAR_OUTPUT calls : INT; END_VAR
METHOD Double : INT
VAR_INPUT x : INT; END_VAR
    LD x
    MUL 2
    ST Double
END_METHOD
calls := calls + 1;
END_FUNCTION_BLOCK

FUNCTION Inc : INT
VAR_INPUT a : INT; END_VAR
START: LD a      (* a label nothing jumps to: Go refuses one unless used *)
    ADD 1
    ST Inc
END_FUNCTION

FUNCTION Clamp : INT
VAR_INPUT v, hi : INT; END_VAR
    LD v
    ST Clamp
    GT hi
    RETCN
    LD hi
    ST Clamp
    RET
END_FUNCTION

PROGRAM TEST_IlPOUs
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR c : FB_Count; m : FB_Math; n : INT; s : INT; END_VAR
n := n + 1;
s := SumTo(n := 4);
IF s <> 10 THEN failures := failures + 1; END_IF;
c(pulse := n <= 4);
IF n <= 4 AND c.count <> n THEN failures := failures + 1; END_IF;
IF n = 6 AND c.count <> 4 THEN failures := failures + 1; END_IF;
IF n = 3 AND NOT c.on THEN failures := failures + 1; END_IF;
IF m.Double(x := 21) <> 42 THEN failures := failures + 1; END_IF;
IF Clamp(v := 7, hi := 5) <> 5 OR Clamp(v := 3, hi := 5) <> 3 OR Inc(a := 1) <> 2 THEN failures := failures + 1; END_IF;
IF n = 6 THEN done := TRUE; END_IF;
END_PROGRAM
`

// TestIlPOUsOnEveryEngine (IEC-70) runs them on the evaluator, the VM and the
// transpiled Go, which agree.
func TestIlPOUsOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if sil.GoEngineAvailable() == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), ilPOUs, sil.Options{Engines: engines, MaxScans: 10, GoTimeout: time.Minute})
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
