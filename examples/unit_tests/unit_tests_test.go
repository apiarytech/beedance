/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package unittests runs the example's Structured Text unit tests from Go
// with package sil, as `beedance -test` does from the command line.
package unittests

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// source joins ST files into one source, as beedance -test does.
func source(t *testing.T, files ...string) string {
	t.Helper()
	var b strings.Builder
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(src)
		b.WriteString("\n")
	}
	return b.String()
}

// engines are the evaluator and the VM, and the transpiler unless -short
// or without a Go toolchain (it builds Go code, and downloads royaljelly
// the first time).
func engines() []sil.Engine {
	if _, err := exec.LookPath("go"); err != nil || testing.Short() {
		return []sil.Engine{sil.Evaluator, sil.VM}
	}
	return []sil.Engine{sil.Evaluator, sil.VM, sil.Go}
}

// The example's tests pass on every engine, and the engines agree.
func TestMotor(t *testing.T) {
	results, err := sil.Run(context.Background(), source(t, "motor.st", "motor_tests.st"), sil.Options{Engines: engines()})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("%d tests, want 4", len(results))
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

// The failing examples fail the way README.md shows.
func TestFailing(t *testing.T) {
	results, err := sil.Run(context.Background(), source(t, "motor.st", "failing.st"),
		sil.Options{Engines: engines(), MaxScans: 2000, GoTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"TEST_WrongExpectation": "1 failed checks, last: expected 99 %",
		"TEST_NeverDone":        "not done after 2000 scans",
		"TEST_RunawayLoop":      "", // the message differs by engine
		"TEST_NoFailuresOutput": "a test program needs VAR_OUTPUT failures : INT",
	}
	for _, r := range results {
		if r.Passed {
			t.Errorf("%s passed", r.Name)
		}
		for _, e := range r.Engines {
			if p := strings.Join(e.Problems(), "; "); !strings.Contains(p, want[r.Name]) || p == "" {
				t.Errorf("%s on %s: %q, want %q", r.Name, e.Engine, p, want[r.Name])
			}
		}
	}
}

// REAL precision: the engines agree within the default tolerance.
func TestPrecision(t *testing.T) {
	results, err := sil.Run(context.Background(), source(t, "motor.st", "precision.st"), sil.Options{Engines: engines()})
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Passed {
		t.Errorf("TEST_RealPrecision: %+v", results[0])
	}
}
