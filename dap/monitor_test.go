/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

import (
	"strconv"
	"testing"
	"time"
)

type monitorReply struct {
	Program string
	Scan    int
	Lines   map[string][]struct {
		Column            int
		Name, Path, Value string
	}
}

// at is the value of name on line row of a monitor reply, and its path.
func (r monitorReply) at(row int, name string) (string, string) {
	for _, m := range r.Lines[strconv.Itoa(row)] {
		if m.Name == name {
			return m.Value, m.Path
		}
	}
	return "", ""
}

// TestMonitor: the values of the names on every line, read as the program
// sees them, stopped and while it runs.
func TestMonitor(t *testing.T) {
	c := newClient(t)
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, nil)
	c.ok("launch", map[string]any{"program": "main.st", "source": program, "speed": 0, "stopOnEntry": true}, nil)
	c.event("initialized")
	c.ok("configurationDone", nil, nil)
	c.event("stopped")
	for range 3 {
		c.ok("beedance/stepScan", nil, nil)
		c.event("stopped")
	}
	body := lineOf(t, program, "tmp := total + step")
	incr := lineOf(t, program, "n := n + 1")
	call := lineOf(t, program, "c(step := 2)")

	var r monitorReply
	c.ok("beedance/monitor", map[string]any{"from": 1, "to": 100}, &r)
	if r.Program != "Main" || r.Scan != 3 {
		t.Fatalf("monitor %+v", r)
	}
	// The block's body, through its instance c.
	if v, p := r.at(body, "total"); v != "6" || p != "c.total" {
		t.Errorf("total in Counter's body: %q at %q, want 6 at c.total", v, p)
	}
	if v, _ := r.at(body, "step"); v != "2" {
		t.Errorf("step in Counter's body: %q", v)
	}
	// The program's body, its own.
	if v, p := r.at(incr, "n"); v != "3" || p != "n" {
		t.Errorf("n: %q at %q", v, p)
	}
	if v, p := r.at(call, "step"); v != "2" || p != "c.step" {
		t.Errorf("step in c(step := 2): %q at %q", v, p)
	}
	// An instance has members, not a value of its own.
	if v, p := r.at(call, "c"); p != "" {
		t.Errorf("the instance c is monitored: %q at %q", v, p)
	}
	// Keywords, literals and comments name nothing.
	for _, m := range r.Lines[strconv.Itoa(incr)] {
		if m.Name != "n" {
			t.Errorf("line %d monitors %q", incr, m.Name)
		}
	}

	// Running: the values move on.
	c.ok("continue", map[string]any{"threadId": 1}, nil)
	c.ok("beedance/monitor", map[string]any{"from": incr, "to": incr}, &r)
	deadline := time.Now().Add(5 * time.Second)
	for {
		time.Sleep(30 * time.Millisecond)
		r = monitorReply{}
		c.ok("beedance/monitor", map[string]any{"from": incr, "to": incr}, &r)
		if v, _ := r.at(incr, "n"); v != "" && v != "3" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("running: %+v", r)
		}
	}
	if len(r.Lines) != 1 {
		t.Errorf("asked for one line, got %v", r.Lines)
	}
	c.ok("pause", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
}
