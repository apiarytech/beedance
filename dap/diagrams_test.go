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
	"testing"
	"time"
)

const ldProgram = `FUNCTION_BLOCK Motor
VAR_INPUT Start, Stop : BOOL; END_VAR
VAR_OUTPUT Run : BOOL; END_VAR
LD
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
END_LD
END_FUNCTION_BLOCK

PROGRAM Main
VAR m : Motor; go : BOOL; halt : BOOL; lamp : BOOL; END_VAR
m(Start := go, Stop := halt);
lamp := m.Run;
END_PROGRAM
`

type liveReply struct {
	Program  string
	Diagrams []struct {
		POU, Lang, Prefix, Error string
		Live                     *struct {
			Values map[string]string
			Exprs  map[string]string
		}
	}
}

// TestDiagramsLive: an LD body's values, seen through its instance, when
// stopped and while running.
func TestDiagramsLive(t *testing.T) {
	c := newClient(t)
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, nil)
	c.ok("launch", map[string]any{"program": "main.st", "source": ldProgram, "speed": 0, "stopOnEntry": true}, nil)
	c.event("initialized")
	c.ok("configurationDone", nil, nil)
	c.event("stopped")
	c.ok("beedance/force", map[string]any{"name": "go", "value": "TRUE"}, nil)
	c.ok("beedance/stepScan", nil, nil)
	c.event("stopped")

	var r liveReply
	c.ok("beedance/diagrams", nil, &r)
	if r.Program != "Main" || len(r.Diagrams) != 1 {
		t.Fatalf("diagrams %+v", r)
	}
	d := r.Diagrams[0]
	if d.POU != "Motor" || d.Prefix != "m." || d.Live == nil {
		t.Fatalf("diagram %+v", d)
	}
	if d.Live.Values["Start"] != "TRUE" || d.Live.Values["Run"] != "TRUE" || d.Live.Values["Stop"] != "FALSE" {
		t.Errorf("stopped: values %v", d.Live.Values)
	}
	on := 0
	for _, v := range d.Live.Exprs {
		if v == "TRUE" {
			on++
		}
	}
	if on == 0 {
		t.Errorf("stopped: no wire on in %v", d.Live.Exprs)
	}

	// Running: the first request asks for the names, later ones get them.
	c.ok("beedance/unforce", map[string]any{"name": "go"}, nil)
	c.ok("beedance/force", map[string]any{"name": "halt", "value": "TRUE"}, nil)
	c.ok("continue", map[string]any{"threadId": 1}, nil)
	c.ok("beedance/diagrams", map[string]any{"prefixes": map[string]string{"Motor": "m"}}, &r)
	deadline := time.Now().Add(5 * time.Second)
	for {
		time.Sleep(20 * time.Millisecond)
		c.ok("beedance/diagrams", map[string]any{"prefixes": map[string]string{"Motor": "m"}}, &r)
		if l := r.Diagrams[0].Live; l != nil && l.Values["Stop"] == "TRUE" && l.Values["Run"] == "FALSE" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("running: values %+v", r.Diagrams[0].Live)
		}
	}
	c.ok("pause", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
}
