/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestInputs: inputs queued for scans are taken one call a scan, so a
// pulse shorter than a scan is still seen; a force wins over them.
func TestInputs(t *testing.T) {
	var samples []Sample
	s, err := Load(source, Options{Sampled: func(smp Sample) { samples = append(samples, smp) }})
	if err != nil {
		t.Fatal(err)
	}
	s.Live()
	ctx := context.Background()
	for _, v := range []any{true, false, "TRUE"} {
		if err := s.SetInputs(map[string]any{"%IX0.0": v}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetInputs(map[string]any{"%IX0.0": 1.5}); err != nil {
		t.Fatal(err) // checked when applied: a BOOL is not 1.5
	}
	if err := s.SetInputs(map[string]any{"missing": true}); err == nil {
		t.Error("an input that is no variable was queued")
	}
	if n := s.QueuedInputs(); n != 4 {
		t.Fatalf("queued %d, want 4", n)
	}
	var ys []string
	for range 3 {
		stopped(t)(s.StepScan(ctx))
		ys = append(ys, value(t, s, "y"))
	}
	if strings.Join(ys, " ") != "TRUE FALSE TRUE" {
		t.Errorf("y over three scans: %v, want TRUE FALSE TRUE", ys)
	}
	if err := s.Force("%IX0.0", "FALSE"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetInputs(map[string]any{"%IX0.0": true}); err != nil {
		t.Fatal(err)
	}
	stopped(t)(s.StepScan(ctx)) // the bad input of 1.5, dropped
	stopped(t)(s.StepScan(ctx))
	if value(t, s, "y") != "FALSE" || s.QueuedInputs() != 0 {
		t.Errorf("y = %s with %%IX0.0 forced FALSE, %d queued", value(t, s, "y"), s.QueuedInputs())
	}
	io := s.IO()
	if io["%IX0.0"] != false || io["%QX0.0"] != false {
		t.Errorf("io %v", io)
	}
	if len(samples) != 6 || samples[3].Scan != 3 || samples[3].IO["%QX0.0"] != true {
		t.Fatalf("%d samples, the fourth %+v", len(samples), samples[min(3, len(samples)-1)])
	}
	b, _ := json.Marshal(samples[3])
	for _, want := range []string{`"scan":3`, `"elapsedMs":30`, `"io":{`, `"name":"y"`, `"address":"%QX0.0"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("sample as JSON %s: no %s", b, want)
		}
	}
}

// TestInputsBounded: past maxInputs scans, inputs join the last one.
func TestInputsBounded(t *testing.T) {
	s := load(t, source)
	for i := range maxInputs + 10 {
		if err := s.SetInputs(map[string]any{"%IX0.0": i%2 == 0}); err != nil {
			t.Fatal(err)
		}
	}
	if n := s.QueuedInputs(); n != maxInputs {
		t.Errorf("queued %d, want %d", n, maxInputs)
	}
}
