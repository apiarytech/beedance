/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

import (
	"encoding/json"
	"strings"
	"testing"
)

const source = `
FUNCTION_BLOCK Blink
VAR_INPUT run : BOOL; END_VAR
VAR_OUTPUT q : BOOL; END_VAR
VAR t : TON; END_VAR
t(IN := run AND NOT t.Q, PT := T#1s);
IF t.Q THEN q := NOT q; END_IF;
END_FUNCTION_BLOCK

PROGRAM Main
VAR start AT %IX0.0 : BOOL; lamp AT %QX0.0 : BOOL; END_VAR
VAR count : INT; limit : INT := 3; b : Blink; END_VAR
b(run := start);
lamp := b.q;
IF start AND count < limit THEN count := count + 1; END_IF;
END_PROGRAM

PROGRAM TEST_Add
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR x : INT; END_VAR
x := 2 + 3;
IF x <> 5 THEN failures := failures + 1; message := 'add'; END_IF;
END_PROGRAM

PROGRAM TEST_Blink
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR b : Blink; END_VAR
b(run := TRUE);
IF b.q THEN done := TRUE; END_IF;
END_PROGRAM

PROGRAM TEST_Fails
VAR_OUTPUT failures : INT; message : STRING; END_VAR
failures := 1; message := 'expected failure';
END_PROGRAM
`

// call sends req through Handle, as a module's host does, and decodes the
// reply.
func call(t *testing.T, h *Host, req map[string]any) map[string]any {
	t.Helper()
	in, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var reply map[string]any
	if err := json.Unmarshal(h.Handle(in), &reply); err != nil {
		t.Fatal(err)
	}
	return reply
}

func noError(t *testing.T, reply map[string]any) {
	t.Helper()
	if e, ok := reply["error"]; ok {
		t.Fatalf("error %v: %v", e, reply["errors"])
	}
}

func TestVersion(t *testing.T) {
	reply := call(t, New(), map[string]any{"op": "version"})
	noError(t, reply)
	if !strings.HasPrefix(reply["goVersion"].(string), "go") || len(reply["ops"].([]any)) == 0 {
		t.Errorf("version reply %v", reply)
	}
}

func TestCheck(t *testing.T) {
	reply := call(t, New(), map[string]any{"op": "check", "source": source})
	if reply["ok"] != true {
		t.Fatalf("check: %v", reply)
	}
	if got := len(reply["programs"].([]any)); got != 4 {
		t.Errorf("%d programs, want 4", got)
	}
	if got := len(reply["tests"].([]any)); got != 3 {
		t.Errorf("%d tests, want 3", got)
	}

	reply = call(t, New(), map[string]any{"op": "check", "source": "PROGRAM P VAR x : INT END_VAR"})
	if reply["ok"] != false || len(reply["errors"].([]any)) == 0 {
		t.Errorf("check of a bad source: %v", reply)
	}
}

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

func TestSession(t *testing.T) {
	h := New()
	reply := call(t, h, map[string]any{"op": "load", "source": source, "intervalMs": 250})
	noError(t, reply)
	if reply["program"] != "Main" {
		t.Errorf("loaded %v, want Main, the first PROGRAM that is not a test", reply["program"])
	}
	id := reply["session"]
	if v := reply["values"].(map[string]any); v["limit"] != 3.0 || v["count"] != 0.0 {
		t.Errorf("after initialization: %v", v)
	}

	// Without start, nothing counts.
	reply = call(t, h, map[string]any{"op": "scan", "session": id, "count": 2})
	noError(t, reply)
	if reply["scans"] != 2.0 || reply["timeMs"] != 500.0 || reply["values"].(map[string]any)["count"] != 0.0 {
		t.Errorf("two scans without start: %v", reply)
	}

	// Start by address; count stops at limit; the lamp blinks after 1 s.
	reply = call(t, h, map[string]any{"op": "scan", "session": id, "count": 5, "values": map[string]any{"%ix0.0": true}})
	noError(t, reply)
	v := reply["values"].(map[string]any)
	if v["count"] != 3.0 || v["start"] != true || v["b.q"] != true || v["lamp"] != true {
		t.Errorf("five scans with start: %v", v)
	}
	if io := reply["io"].(map[string]any); io["%QX0.0"] != true {
		t.Errorf("I/O image %v: want %%QX0.0 TRUE", io)
	}

	// set by name converts to the declared type and does not scan.
	reply = call(t, h, map[string]any{"op": "set", "session": id, "values": map[string]any{"limit": 10, "start": false}})
	noError(t, reply)
	if reply["scans"] != 7.0 || reply["values"].(map[string]any)["limit"] != 10.0 {
		t.Errorf("set: %v", reply)
	}
	reply = call(t, h, map[string]any{"op": "read", "session": id})
	noError(t, reply)
	if reply["values"].(map[string]any)["start"] != false {
		t.Errorf("read after set: %v", reply)
	}

	for _, bad := range []map[string]any{
		{"nosuch": 1},
		{"limit": "three"},
		{"%Z1": true},
	} {
		if reply := call(t, h, map[string]any{"op": "set", "session": id, "values": bad}); reply["error"] == nil {
			t.Errorf("set %v: no error", bad)
		}
	}

	noError(t, call(t, h, map[string]any{"op": "unload", "session": id}))
	if reply := call(t, h, map[string]any{"op": "read", "session": id}); reply["error"] == nil {
		t.Errorf("read after unload: %v", reply)
	}
}

func TestLoadByName(t *testing.T) {
	h := New()
	reply := call(t, h, map[string]any{"op": "load", "source": source, "program": "test_add"})
	noError(t, reply)
	if reply["program"] != "TEST_Add" {
		t.Errorf("loaded %v", reply["program"])
	}
	reply = call(t, h, map[string]any{"op": "scan", "session": reply["session"]})
	noError(t, reply)
	if v := reply["values"].(map[string]any); v["x"] != 5.0 || v["failures"] != 0.0 {
		t.Errorf("TEST_Add after a scan: %v", v)
	}
	if reply := call(t, h, map[string]any{"op": "load", "source": source, "program": "Nope"}); reply["error"] == nil {
		t.Errorf("load of a missing program: %v", reply)
	}
}

func TestBadRequests(t *testing.T) {
	h := New()
	for _, in := range []string{`not json`, `{}`, `{"op":"fly"}`, `{"op":"scan","session":42}`} {
		var reply map[string]any
		if err := json.Unmarshal(h.Handle([]byte(in)), &reply); err != nil {
			t.Fatalf("%s: reply is not JSON: %v", in, err)
		}
		if reply["error"] == nil {
			t.Errorf("%s: no error in %v", in, reply)
		}
	}
}
