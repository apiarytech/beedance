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
	"time"
)

const dapProgram = `PROGRAM Main
VAR
    n : DINT;
    start AT %IX0.0 : BOOL;
    lamp AT %QX0.0 : BOOL;
END_VAR
n := n + 1;
lamp := start;
END_PROGRAM
`

// dapClient is a page talking to a DAP session, message by message.
type dapClient struct {
	t   *testing.T
	d   *DAP
	in  chan map[string]any
	seq int
}

func newDAPClient(t *testing.T) *dapClient {
	c := &dapClient{t: t, in: make(chan map[string]any, 100)}
	c.d = NewDAP(func(msg []byte) {
		var m map[string]any
		if err := json.Unmarshal(msg, &m); err != nil {
			t.Errorf("message %s: %v", msg, err)
			return
		}
		c.in <- m
	}, DAPYield, nil)
	t.Cleanup(c.d.Close)
	return c
}

// call sends a request and waits for its response, which must succeed.
func (c *dapClient) call(command string, args any) map[string]any {
	c.t.Helper()
	c.seq++
	b, _ := json.Marshal(map[string]any{"seq": c.seq, "type": "request", "command": command, "arguments": args})
	c.d.Send(b)
	for {
		m := c.next()
		if m["type"] == "response" && int(m["request_seq"].(float64)) == c.seq {
			if m["success"] != true {
				c.t.Fatalf("%s: %v", command, m["message"])
			}
			body, _ := m["body"].(map[string]any)
			return body
		}
	}
}

func (c *dapClient) next() map[string]any {
	c.t.Helper()
	select {
	case m := <-c.in:
		return m
	case <-time.After(10 * time.Second):
		c.t.Fatal("no message")
	}
	return nil
}

// event waits for the event name; the others before it are dropped.
func (c *dapClient) event(name string) map[string]any {
	c.t.Helper()
	for {
		if m := c.next(); m["type"] == "event" && m["event"] == name {
			body, _ := m["body"].(map[string]any)
			return body
		}
	}
}

// TestDAP: the page's debugger, from launch to a breakpoint, running free,
// the live values and pausing.
func TestDAP(t *testing.T) {
	c := newDAPClient(t)
	c.call("initialize", map[string]any{"adapterID": "beedance"})
	c.call("launch", map[string]any{"program": "main.st", "source": dapProgram, "realtime": false})
	c.event("initialized")
	line := strings.Count(dapProgram[:strings.Index(dapProgram, "lamp := start")], "\n") + 1
	bps := c.call("setBreakpoints", map[string]any{"source": map[string]any{"path": "main.st"},
		"breakpoints": []map[string]any{{"line": line}}})
	if b := bps["breakpoints"].([]any)[0].(map[string]any); b["verified"] != true {
		t.Fatalf("breakpoint %v", b)
	}
	c.call("configurationDone", nil)
	if st := c.event("stopped"); st["reason"] != "breakpoint" {
		t.Fatalf("stopped %v", st)
	}
	c.call("setBreakpoints", map[string]any{"source": map[string]any{"path": "main.st"}, "breakpoints": []any{}})
	c.call("beedance/force", map[string]any{"name": "%IX0.0", "value": "TRUE"})
	c.call("continue", map[string]any{"threadId": 1})
	time.Sleep(50 * time.Millisecond)
	live := c.call("beedance/live", nil)
	c.call("pause", map[string]any{"threadId": 1})
	if st := c.event("stopped"); st["reason"] != "pause" {
		t.Fatalf("stopped %v", st)
	}
	if live["scan"].(float64) < 2 {
		t.Errorf("live %v: the program did not run on", live)
	}
	lamp := ""
	for _, v := range live["values"].([]any) {
		if v := v.(map[string]any); v["name"] == "lamp" {
			lamp, _ = v["text"].(string)
		}
	}
	if lamp != "TRUE" {
		t.Errorf("lamp = %q with %%IX0.0 forced TRUE, in %v", lamp, live)
	}
	c.call("disconnect", nil)
}
