/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const suite = `
FUNCTION_BLOCK Blink
VAR_INPUT run : BOOL; END_VAR
VAR_OUTPUT q : BOOL; END_VAR
VAR t : TON; END_VAR
t(IN := run AND NOT t.Q, PT := T#2h);
IF t.Q THEN q := NOT q; END_IF;
END_FUNCTION_BLOCK

PROGRAM TEST_Add
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR x : INT; r : REAL; END_VAR
x := 2 + 3;
r := 1.0 / 3.0;
IF x <> 5 THEN failures := failures + 1; message := 'add'; END_IF;
END_PROGRAM

PROGRAM test_Timer
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR b : Blink; END_VAR
b(run := TRUE);
IF b.q THEN done := TRUE; END_IF;
END_PROGRAM

PROGRAM TEST_Fails
VAR_OUTPUT failures : INT; message : STRING; END_VAR
failures := 1; message := 'expected failure';
END_PROGRAM

PROGRAM TEST_Runaway
VAR_OUTPUT failures : INT; END_VAR
VAR i : INT; END_VAR
WHILE TRUE DO i := i + 1; END_WHILE;
END_PROGRAM

PROGRAM TEST_NeverDone
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
END_PROGRAM

PROGRAM TEST_NoFailures
VAR x : INT; END_VAR
x := 1;
END_PROGRAM

PROGRAM NotATest
VAR x : INT; END_VAR
x := 1;
END_PROGRAM
`

func TestRun(t *testing.T) {
	results, err := Run(context.Background(), suite, Options{Interval: time.Second, MaxScans: 10000, ScanBudget: 100000})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Result{}
	var names []string
	for _, r := range results {
		byName[r.Name] = r
		names = append(names, r.Name)
	}
	if got, want := strings.Join(names, ","), "TEST_Add,test_Timer,TEST_Fails,TEST_Runaway,TEST_NeverDone,TEST_NoFailures"; got != want {
		t.Fatalf("tests = %s, want %s", got, want)
	}

	for _, tc := range []struct {
		name   string
		passed bool
		scans  int    // on each engine; -1 to skip
		err    string // in each engine's Err
	}{
		{"TEST_Add", true, 1, ""},
		{"test_Timer", true, 7201, ""}, // 2 h at 1 s per scan, plus the first
		{"TEST_Fails", false, 1, ""},
		{"TEST_Runaway", false, 0, "budget exceeded"},
		{"TEST_NeverDone", false, 10000, ""},
		{"TEST_NoFailures", false, 0, "needs VAR_OUTPUT failures"},
	} {
		res := byName[tc.name]
		if res.Passed != tc.passed {
			t.Errorf("%s: Passed = %v, want %v: %+v", tc.name, res.Passed, tc.passed, res.Engines)
		}
		if len(res.Engines) != 2 || res.Engines[0].Engine != Evaluator || res.Engines[1].Engine != VM {
			t.Fatalf("%s: engines %+v", tc.name, res.Engines)
		}
		for _, r := range res.Engines {
			if len(r.Scans) != tc.scans {
				t.Errorf("%s on %s: %d scans, want %d", tc.name, r.Engine, len(r.Scans), tc.scans)
			}
			if !strings.Contains(r.Err, tc.err) || (tc.err == "") != (r.Err == "") {
				t.Errorf("%s on %s: Err = %q, want %q", tc.name, r.Engine, r.Err, tc.err)
			}
		}
		if res.Mismatch != "" {
			t.Errorf("%s: engines differ: %s", tc.name, res.Mismatch)
		}
	}

	// A function block instance's outputs are recorded under dotted names
	// (issue #5) and compared between engines.
	timer := byName["test_Timer"]
	for _, r := range timer.Engines {
		if last := r.Scans[len(r.Scans)-1]; last["b.q"] != "true" {
			t.Errorf("test_Timer on %s: last scan b.q = %q, want true; scan %v", r.Engine, last["b.q"], last)
		}
	}

	fails := byName["TEST_Fails"].Engines[1]
	if fails.Failures != 1 || fails.Message != "expected failure" {
		t.Errorf("TEST_Fails: failures %d, message %q", fails.Failures, fails.Message)
	}
	if p := byName["TEST_NeverDone"].Engines[0].Problems(); len(p) != 1 || !strings.Contains(p[0], "not done after 10000 scans") {
		t.Errorf("TEST_NeverDone problems = %q", p)
	}
}

func TestRunOneEngineAndFilter(t *testing.T) {
	for _, e := range []Engine{Evaluator, VM} {
		results, err := Run(context.Background(), suite, Options{Engines: []Engine{e}, Filter: "add"})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || results[0].Name != "TEST_Add" || !results[0].Passed ||
			len(results[0].Engines) != 1 || results[0].Engines[0].Engine != e {
			t.Errorf("%s: %+v", e, results)
		}
	}
}

func TestRunErrors(t *testing.T) {
	_, err := Run(context.Background(), "PROGRAM TEST_X VAR x : INT END_VAR END_PROGRAM", Options{})
	var pe *ParseError
	if !errors.As(err, &pe) || len(pe.Errors) == 0 {
		t.Errorf("bad source: err = %v, want a *ParseError", err)
	}
	if _, err := Run(context.Background(), suite, Options{Filter: "nothing"}); !errors.Is(err, ErrNoTests) {
		t.Errorf("no match: err = %v, want ErrNoTests", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, suite, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v", err)
	}
}

func TestCompare(t *testing.T) {
	vars := []Variable{{Name: "r", Type: "REAL"}, {Name: "s", Type: "STRING"}, {Name: "b", Type: "BOOL"}}
	a := EngineResult{Engine: Evaluator, Scans: []Scan{{"r": "1.0000", "s": "'x'", "b": "TRUE"}, {"r": "2", "b": "true"}}}
	b := EngineResult{Engine: VM, Scans: []Scan{{"r": "1.0004", "s": "x", "b": "true"}, {"r": "2", "b": "false"}}}
	if got, want := compare(a, b, vars, 1e-3), "scan 2 b: eval true, vm false"; got != want {
		t.Errorf("compare = %q, want %q", got, want)
	}
	b.Scans = b.Scans[:1]
	if got, want := compare(a, b, vars, 1e-3), "eval ran 2 scans, vm 1"; got != want {
		t.Errorf("compare = %q, want %q", got, want)
	}
	a.Scans = a.Scans[:1]
	if got := compare(a, EngineResult{Scans: []Scan{{"r": "1.1"}}}, vars[:1], 1e-3); !strings.Contains(got, "scan 1 r") {
		t.Errorf("REAL out of tolerance: compare = %q", got)
	}
}

func TestParseEngines(t *testing.T) {
	got, err := ParseEngines(" VM, eval ,vm")
	if err != nil || len(got) != 2 || got[0] != VM || got[1] != Evaluator {
		t.Errorf("ParseEngines = %v, %v", got, err)
	}
	if _, err := ParseEngines("jit"); err == nil {
		t.Error("ParseEngines(jit): no error")
	}
}
