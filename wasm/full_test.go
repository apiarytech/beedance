//go:build !lite

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

import (
	"strings"
	"testing"
)

func TestTest(t *testing.T) {
	reply := call(t, New(), map[string]any{"op": "test", "source": source, "intervalMs": 100, "scans": true})
	noError(t, reply)
	if reply["passed"] != false || reply["failed"] != 1.0 {
		t.Errorf("passed %v, failed %v: want false, 1", reply["passed"], reply["failed"])
	}
	results := reply["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("%d results, want 3", len(results))
	}
	want := map[string]bool{"TEST_Add": true, "TEST_Blink": true, "TEST_Fails": false}
	for _, r := range results {
		r := r.(map[string]any)
		name := r["name"].(string)
		if r["passed"] != want[name] {
			t.Errorf("%s passed %v, want %v: %v", name, r["passed"], want[name], r)
		}
		engines := r["engines"].([]any)
		if len(engines) != 2 {
			t.Fatalf("%s ran on %d engines, want eval and vm", name, len(engines))
		}
		for _, e := range engines {
			e := e.(map[string]any)
			if n := e["scanCount"].(float64); int(n) != len(e["scans"].([]any)) {
				t.Errorf("%s on %s: scanCount %v, %d scans", name, e["engine"], n, len(e["scans"].([]any)))
			}
		}
		if name == "TEST_Blink" {
			// TON of 1 s at 100 ms scans: q is TRUE on the scan after it.
			if n := engines[0].(map[string]any)["scanCount"].(float64); n < 10 || n > 12 {
				t.Errorf("TEST_Blink ran %v scans, want about 11", n)
			}
		}
	}

	reply = call(t, New(), map[string]any{"op": "test", "source": source, "filter": "add", "engines": []string{"vm"}})
	noError(t, reply)
	if results := reply["results"].([]any); len(results) != 1 || reply["passed"] != true {
		t.Errorf("filtered run: %v", reply)
	}
}

func TestTestErrors(t *testing.T) {
	h := New()
	reply := call(t, h, map[string]any{"op": "test", "source": "PROGRAM P VAR x : INT END_VAR"})
	if reply["error"] != "parse errors" || len(reply["errors"].([]any)) == 0 {
		t.Errorf("parse error reply %v", reply)
	}
	reply = call(t, h, map[string]any{"op": "test", "source": "PROGRAM P END_PROGRAM"})
	if reply["error"] == nil {
		t.Errorf("a source without tests: %v", reply)
	}
	reply = call(t, h, map[string]any{"op": "test", "source": source, "engines": []string{"cobol"}})
	if reply["error"] == nil {
		t.Errorf("an unknown engine: %v", reply)
	}
}

func TestTranspile(t *testing.T) {
	reply := call(t, New(), map[string]any{"op": "transpile", "source": source})
	noError(t, reply)
	if src := reply["go"].(string); !strings.HasPrefix(src, "package main") {
		t.Errorf("transpiled source starts %.40q", src)
	}
}
