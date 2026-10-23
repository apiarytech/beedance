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
	"os"
	"strings"
	"testing"
	"time"
)

// wasmOptions gives the wasm engine's tests their runtime: SIL_WASM_RUNTIME
// (e.g. "wazero run"), or one in PATH; without either they are skipped.
func wasmOptions(t *testing.T) Options {
	t.Helper()
	if testing.Short() {
		t.Skip("builds Go code for WebAssembly")
	}
	if err := GoEngineAvailable(); err != nil {
		t.Skip("the go engine cannot build here:", err)
	}
	opts := Options{WasmRuntime: strings.Fields(os.Getenv("SIL_WASM_RUNTIME"))}
	if _, err := wasmRuntime(context.Background(), opts, t.TempDir()); err != nil {
		t.Skip(err)
	}
	return opts
}

// The wasm engine runs the tests transpiled and built for WASI, and agrees
// with the evaluator and the VM scan by scan; a runaway loop is stopped by
// GoTimeout.
func TestRunWasmEngine(t *testing.T) {
	opts := wasmOptions(t)
	opts.Engines = []Engine{Evaluator, VM, Wasm}
	opts.Interval, opts.MaxScans, opts.ScanBudget, opts.GoTimeout = time.Second, 10000, 100000, 5*time.Second
	results, err := Run(context.Background(), suite, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, res := range results {
		w := res.Engines[2]
		if w.Engine != Wasm {
			t.Fatalf("%s: engines %+v", res.Name, res.Engines)
		}
		if strings.HasPrefix(w.Err, "go ") {
			t.Fatalf("%s: building the transpiled tests: %s", res.Name, w.Err)
		}
		if res.Mismatch != "" {
			t.Errorf("%s: engines differ: %s", res.Name, res.Mismatch)
		}
		if w.Passed != res.Engines[0].Passed || len(w.Scans) != len(res.Engines[0].Scans) {
			t.Errorf("%s: wasm passed=%v after %d scans, eval passed=%v after %d: %s",
				res.Name, w.Passed, len(w.Scans), res.Engines[0].Passed, len(res.Engines[0].Scans), w.Err)
		}
		switch res.Name {
		case "TEST_Runaway":
			if !strings.Contains(w.Err, "no result after 5s") {
				t.Errorf("TEST_Runaway on wasm: Err = %q", w.Err)
			}
		case "test_Timer":
			if last := w.Scans[len(w.Scans)-1]; last["b.q"] != "true" {
				t.Errorf("test_Timer on wasm: last scan %v", last)
			}
		}
	}
}

// A result far past the 64 KB a pipe holds arrives whole: the harness
// waits out a non-blocking pipe's EAGAIN.
func TestRunWasmEngineLongResult(t *testing.T) {
	opts := wasmOptions(t)
	opts.Engines = []Engine{VM, Wasm}
	opts.Interval, opts.Filter = time.Millisecond, "timer" // 2 h of TON at 1 ms: 7,200,001 scans
	opts.MaxScans = 20000
	results, err := Run(context.Background(), suite, opts)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if w := r.Engines[1]; len(w.Scans) != 20000 || r.Mismatch != "" {
		t.Errorf("wasm: %d scans, want 20000; mismatch %q; err %q", len(w.Scans), r.Mismatch, w.Err)
	}
}

// With IO, the wasm engine runs a scan per request over the module's stdin
// and stdout, as the go engine does.
func TestIOWasmEngine(t *testing.T) {
	opts := wasmOptions(t)
	k := &tank{}
	opts.IO, opts.Deterministic = k, true
	opts.Engines, opts.GoTimeout = []Engine{Evaluator, VM, Wasm}, 10*time.Second
	results, err := Run(context.Background(), tankSource, opts)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !r.Passed {
		for _, e := range r.Engines {
			t.Logf("%s: passed %v, %d scans, err %s", e.Engine, e.Passed, len(e.Scans), e.Err)
		}
		t.Fatalf("not passed, mismatch %q", r.Mismatch)
	}
	if got := strings.Join(k.begins, ","); got != "TEST_Fill/eval,TEST_Fill/vm,TEST_Fill/wasm" {
		t.Errorf("begins %s", got)
	}
	if w := r.Engines[2]; len(w.Scans) != 81 || !strings.EqualFold(w.Scans[80]["%QX0.0"], "false") {
		t.Errorf("wasm: %d scans, last %v", len(w.Scans), w.Scans[len(w.Scans)-1])
	}
}

// Without a runtime, only the wasm engine's runs fail, saying why.
func TestWasmEngineNoRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go code")
	}
	if err := GoEngineAvailable(); err != nil {
		t.Skip("the go engine cannot build here:", err)
	}
	results, err := Run(context.Background(), suite, Options{
		Engines: []Engine{VM, Wasm}, Filter: "add", WasmRuntime: []string{"no-such-wasi-runtime"},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !r.Engines[0].Passed || r.Engines[1].Passed || !strings.Contains(r.Engines[1].Err, "WASI runtime no-such-wasi-runtime") {
		t.Errorf("vm passed %v; wasm passed %v, err %q", r.Engines[0].Passed, r.Engines[1].Passed, r.Engines[1].Err)
	}
}
