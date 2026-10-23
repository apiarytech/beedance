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

// enEnoDeclared (IEC-75): a function block and a function that declare EN
// and ENO themselves, as IEC 61131-3 provides them (2003 2.5.1.2 and
// 2.5.2.1a; Ed.3 Table 18): EN FALSE, the body is not run and ENO is FALSE,
// a block's outputs keeping their values; otherwise ENO is TRUE before the
// body runs, and the body may set it FALSE. A call that does not pass EN leaves
// it as it was, as any input. The body may read EN, in any case.
const enEnoDeclared = `
FUNCTION_BLOCK FB_EnEno
VAR_INPUT EN : BOOL := TRUE; x : INT; END_VAR
VAR_OUTPUT ENO : BOOL; y : INT; sawEn : BOOL; runs : INT; END_VAR
runs := runs + 1;
sawEn := en;              (* the body reads EN, spelled in lower case *)
y := x;
IF x < 0 THEN ENO := FALSE; END_IF;
END_FUNCTION_BLOCK

FUNCTION Half : INT
VAR_INPUT EN : BOOL := TRUE; v : INT; END_VAR
VAR_OUTPUT ENO : BOOL; END_VAR
Half := v / 2;
IF v < 0 THEN ENO := FALSE; END_IF;
END_FUNCTION

PROGRAM TEST_EnEnoDeclared
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR f : FB_EnEno; h : INT; ok : BOOL; END_VAR
f(EN := TRUE, x := 7);
IF NOT f.ENO OR f.y <> 7 OR NOT f.sawEn OR f.runs <> 1 THEN failures := failures + 1; END_IF;
f(EN := FALSE, x := 5);     (* not run: y keeps 7, ENO FALSE *)
IF f.ENO OR f.y <> 7 OR f.runs <> 1 THEN failures := failures + 1; END_IF;
f(x := 9);                  (* EN keeps its last value, FALSE, as any input a call does not pass: not run *)
IF f.ENO OR f.y <> 7 OR f.runs <> 1 THEN failures := failures + 1; END_IF;
f(EN := TRUE, x := -1);     (* the body sets ENO FALSE *)
IF f.ENO OR f.y <> -1 OR f.runs <> 2 THEN failures := failures + 1; END_IF;
h := Half(EN := TRUE, v := 8, ENO => ok);
IF h <> 4 OR NOT ok THEN failures := failures + 1; END_IF;
h := Half(EN := TRUE, v := -8, ENO => ok);
IF ok THEN failures := failures + 1; END_IF;
h := Half(EN := FALSE, v := 6, ENO => ok);
IF ok THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`

// TestEnEnoDeclaredOnEveryEngine (IEC-75): the evaluator, the VM and the
// transpiled Go agree, as the standard says.
func TestEnEnoDeclaredOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if sil.GoEngineAvailable() == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), enEnoDeclared, sil.Options{Engines: engines, MaxScans: 3, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) || !r.Passed {
			t.Errorf("%s: passed %v on %d engines, mismatch %q", r.Name, r.Passed, len(r.Engines), r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v %.400s", e.Engine, e.Problems(), e.Err)
			}
		}
	}
}
