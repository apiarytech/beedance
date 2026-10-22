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
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const program = `FUNCTION_BLOCK Counter
VAR_INPUT step : INT; END_VAR
VAR_OUTPUT total : INT; END_VAR
VAR tmp : INT; END_VAR
tmp := total + step;
total := tmp;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    n : INT;
    c : Counter;
    x AT %IX0.0 : BOOL;
    y AT %QX0.0 : BOOL;
END_VAR
n := n + 1;
c(step := 2);
y := x;
END_PROGRAM
`

func lineOf(t *testing.T, src, text string) int {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), text) {
			return i + 1
		}
	}
	t.Fatalf("no line %q", text)
	return 0
}

// client is an editor talking to a Server.
type client struct {
	t      *testing.T
	w      io.Writer
	in     chan *message
	seq    int
	events []*message
	done   chan error
}

func newClient(t *testing.T) *client {
	t.Helper()
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	c := &client{t: t, w: cw, in: make(chan *message, 100), done: make(chan error, 1)}
	go func() {
		srv := NewServer(sr, sw)
		if os.Getenv("DAPLOG") != "" { // DAPLOG=1 go test: the messages, on stderr

			srv.Log = os.Stderr
		}
		c.done <- srv.Serve(context.Background())
		sw.Close()
	}()
	go func() {
		r := bufio.NewReader(cr)
		for {
			m, err := readMessage(r)
			if err != nil {
				close(c.in)
				return
			}
			c.in <- m
		}
	}()
	t.Cleanup(func() { cw.Close() })
	return c
}

func (c *client) next() *message {
	c.t.Helper()
	select {
	case m, ok := <-c.in:
		if !ok {
			c.t.Fatal("the server closed the stream")
		}
		return m
	case <-time.After(10 * time.Second):
		c.t.Fatal("no message from the server")
	}
	return nil
}

// request sends command and returns its response, keeping the events that
// come before it.
func (c *client) request(command string, args any) *message {
	c.t.Helper()
	c.seq++
	req := map[string]any{"seq": c.seq, "type": "request", "command": command}
	if args != nil {
		req["arguments"] = args
	}
	if err := writeMessage(c.w, req); err != nil {
		c.t.Fatal(err)
	}
	for {
		m := c.next()
		if m.Type == "event" {
			c.events = append(c.events, m)
			continue
		}
		if m.RequestSeq != c.seq {
			c.t.Fatalf("response to %d, want %d", m.RequestSeq, c.seq)
		}
		return m
	}
}

// ok is request, failing the test if the request fails; it decodes the
// body into body, if not nil.
func (c *client) ok(command string, args any, body any) {
	c.t.Helper()
	m := c.request(command, args)
	if !m.Success {
		c.t.Fatalf("%s: %s", command, m.Message)
	}
	if body != nil {
		if err := json.Unmarshal(m.Body, body); err != nil {
			c.t.Fatalf("%s: %v in %s", command, err, m.Body)
		}
	}
}

// event waits for the event name, returning its body.
func (c *client) event(name string) map[string]any {
	c.t.Helper()
	for {
		for i, e := range c.events {
			if e.Event == name {
				c.events = append(c.events[:i], c.events[i+1:]...)
				var body map[string]any
				json.Unmarshal(e.Body, &body)
				return body
			}
		}
		m := c.next()
		if m.Type == "event" {
			c.events = append(c.events, m)
		}
	}
}

type frames struct {
	StackFrames []stackFrame `json:"stackFrames"`
}

func (c *client) top() stackFrame {
	c.t.Helper()
	var f frames
	c.ok("stackTrace", map[string]any{"threadId": 1}, &f)
	if len(f.StackFrames) == 0 {
		c.t.Fatal("no frames")
	}
	return f.StackFrames[0]
}

// vars are the variables of the scope named name (a prefix) of frame.
func (c *client) vars(frame int, name string) (map[string]variable, int) {
	c.t.Helper()
	var sc struct{ Scopes []scope }
	c.ok("scopes", map[string]any{"frameId": frame}, &sc)
	for _, s := range sc.Scopes {
		if strings.HasPrefix(s.Name, name) {
			var vs struct{ Variables []variable }
			c.ok("variables", map[string]any{"variablesReference": s.VariablesReference}, &vs)
			out := map[string]variable{}
			for _, v := range vs.Variables {
				out[v.Name] = v
			}
			return out, s.VariablesReference
		}
	}
	c.t.Fatalf("no scope %s in %+v", name, sc.Scopes)
	return nil, 0
}

func launch(t *testing.T, c *client, args map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.st")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}
	var caps capabilities
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, &caps)
	if !caps.SupportsConfigurationDoneRequest || !caps.SupportsDisassembleRequest {
		t.Fatalf("capabilities %+v", caps)
	}
	args["program"] = path
	c.ok("launch", args, nil)
	c.event("initialized")
	return path
}

func TestSession(t *testing.T) {
	c := newClient(t)
	path := launch(t, c, map[string]any{"interval": "10ms", "realtime": false})
	incr := lineOf(t, program, "n := n + 1")

	var bps struct{ Breakpoints []breakpoint }
	c.ok("setBreakpoints", map[string]any{"source": map[string]any{"path": path},
		"breakpoints": []map[string]any{{"line": incr}, {"line": 1000}}}, &bps)
	if !bps.Breakpoints[0].Verified || bps.Breakpoints[0].Line != incr || bps.Breakpoints[1].Verified {
		t.Fatalf("breakpoints %+v", bps.Breakpoints)
	}
	c.ok("setBreakpoints", map[string]any{"source": map[string]any{"path": path + ".other"},
		"breakpoints": []map[string]any{{"line": 3}}}, &bps)
	if bps.Breakpoints[0].Verified {
		t.Error("a breakpoint in another file was verified")
	}
	c.ok("configurationDone", nil, nil)
	if st := c.event("stopped"); st["reason"] != "breakpoint" {
		t.Fatalf("stopped %v", st)
	}
	var th struct{ Threads []thread }
	c.ok("threads", nil, &th)
	if len(th.Threads) != 1 || th.Threads[0].Name != "PROGRAM Main" {
		t.Errorf("threads %+v", th.Threads)
	}
	f := c.top()
	if f.Name != "Main" || f.Line != incr || f.Source == nil || f.InstructionPointerReference == "" {
		t.Errorf("frame %+v", f)
	}

	// Variables: read, write, members, hover.
	vs, ref := c.vars(f.ID, "PROGRAM")
	if vs["n"].Value != "0" || vs["x"].Type != "BOOL AT %IX0.0" || vs["c"].VariablesReference == 0 {
		t.Errorf("program variables %+v", vs)
	}
	var set struct{ Value string }
	c.ok("setVariable", map[string]any{"variablesReference": ref, "name": "n", "value": "41"}, &set)
	if set.Value != "41" {
		t.Errorf("set n: %q", set.Value)
	}
	var members struct{ Variables []variable }
	c.ok("variables", map[string]any{"variablesReference": vs["c"].VariablesReference}, &members)
	names := []string{}
	for _, v := range members.Variables {
		names = append(names, v.Name+"="+v.Value)
	}
	if strings.Join(names, " ") != "step=0 tmp=0 total=0" {
		t.Errorf("c's members %v", names)
	}
	var ev struct{ Result string }
	c.ok("evaluate", map[string]any{"expression": "n * 2", "frameId": f.ID, "context": "hover"}, &ev)
	if ev.Result != "82" {
		t.Errorf("n * 2 = %s", ev.Result)
	}
	if m := c.request("evaluate", map[string]any{"expression": "n +", "context": "watch"}); m.Success {
		t.Error("a bad expression evaluated")
	}

	// The console's commands: forcing.
	c.ok("evaluate", map[string]any{"expression": "force x TRUE", "context": "repl"}, &ev)
	c.ok("evaluate", map[string]any{"expression": "forces", "context": "repl"}, &ev)
	if ev.Result != "x = TRUE" {
		t.Errorf("forces: %q", ev.Result)
	}
	vs, _ = c.vars(f.ID, "Forced")
	if vs["x"].Value != "TRUE" {
		t.Errorf("forced scope %+v", vs)
	}

	// Stepping into the function block, its locals.
	c.ok("next", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	if f = c.top(); f.Line != lineOf(t, program, "c(step") {
		t.Fatalf("after next: %+v", f)
	}
	c.ok("stepIn", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	if f = c.top(); f.Name != "Counter" || f.Line != lineOf(t, program, "tmp := total") {
		t.Fatalf("after stepIn: %+v", f)
	}
	locals, lref := c.vars(f.ID, "Locals")
	if locals["step"].Value != "2" || locals["total"].Value != "0" {
		t.Errorf("Counter's locals %+v", locals)
	}
	c.ok("setVariable", map[string]any{"variablesReference": lref, "name": "total", "value": "100"}, &set)

	// The disassembly view, and a step by instruction.
	var dis struct{ Instructions []disassembledInstruction }
	c.ok("disassemble", map[string]any{"memoryReference": f.InstructionPointerReference, "instructionOffset": -2, "instructionCount": 6}, &dis)
	if len(dis.Instructions) != 6 || dis.Instructions[2].Address != f.InstructionPointerReference ||
		dis.Instructions[2].Line != f.Line {
		t.Fatalf("disassembly %+v around %s", dis.Instructions, f.InstructionPointerReference)
	}
	c.ok("next", map[string]any{"threadId": 1, "granularity": "instruction"}, nil)
	c.event("stopped")
	if g := c.top(); g.InstructionPointerReference == f.InstructionPointerReference || g.Name != "Counter" {
		t.Errorf("an instruction step from %s: %+v", f.InstructionPointerReference, g)
	}

	// To the end of the scan: the writes and the force took.
	c.ok("stepOut", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	c.ok("beedance/stepScan", map[string]any{}, nil)
	if st := c.event("stopped"); st["description"] != "End of scan 1" {
		t.Errorf("stopped %v", st)
	}
	vs, _ = c.vars(1, "PROGRAM")
	if vs["n"].Value != "42" || vs["y"].Value != "TRUE" || vs["x"].Value != "TRUE  (forced)" {
		t.Errorf("after the scan: %+v", vs)
	}
	members.Variables = nil
	c.ok("variables", map[string]any{"variablesReference": vs["c"].VariablesReference}, &members)
	for _, v := range members.Variables {
		if v.Name == "total" && v.Value != "102" {
			t.Errorf("c.total = %s, want 102 (set to 100 in the call, then + 2)", v.Value)
		}
	}

	// Online change from the file; with no breakpoints, a scan of the new
	// code.
	c.ok("setBreakpoints", map[string]any{"source": map[string]any{"path": path}, "breakpoints": []map[string]any{}}, nil)
	edited := strings.Replace(program, "n := n + 1;", "n := n + 100;", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var report struct{ Kept []string }
	c.ok("beedance/onlineChange", map[string]any{}, &report)
	if strings.Join(report.Kept, ",") != "n,c,x,y" {
		t.Errorf("kept %v", report.Kept)
	}
	c.ok("evaluate", map[string]any{"expression": "scan", "context": "repl"}, nil)
	c.event("stopped")
	c.ok("evaluate", map[string]any{"expression": "n", "context": "watch"}, &ev)
	if ev.Result != "142" {
		t.Errorf("n = %s after the change and a scan, want 142", ev.Result)
	}

	// Running and pausing; live values while it runs, and the thread, which
	// VS Code asks for at once.
	c.ok("continue", map[string]any{"threadId": 1}, nil)
	th.Threads = nil
	c.ok("threads", nil, &th)
	if len(th.Threads) != 1 || th.Threads[0].Name != "PROGRAM Main" {
		t.Errorf("threads while running %+v", th.Threads)
	}
	time.Sleep(30 * time.Millisecond)
	c.ok("evaluate", map[string]any{"expression": "live", "context": "repl"}, &ev)
	if !strings.Contains(ev.Result, "n = ") {
		t.Errorf("live: %q", ev.Result)
	}
	if m := c.request("stackTrace", map[string]any{"threadId": 1}); m.Success {
		t.Error("a stack trace while running")
	}
	c.ok("pause", map[string]any{"threadId": 1}, nil)
	if st := c.event("stopped"); st["reason"] != "pause" {
		t.Errorf("stopped %v", st)
	}
	c.ok("disconnect", map[string]any{}, nil)
	select {
	case err := <-c.done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not return after disconnect")
	}
}

func TestStopOnEntryAndErrors(t *testing.T) {
	c := newClient(t)
	launch(t, c, map[string]any{"stopOnEntry": true, "realtime": false})
	c.ok("configurationDone", nil, nil)
	if st := c.event("stopped"); st["reason"] != "entry" {
		t.Fatalf("stopped %v", st)
	}
	if f := c.top(); f.Line != lineOf(t, program, "n := n + 1") || f.InstructionPointerReference != "" {
		t.Errorf("at entry: %+v", f)
	}
	for _, cmd := range []string{"force nope 1", "force n", "unforce n", "set n TRUE"} {
		if m := c.request("evaluate", map[string]any{"expression": cmd, "context": "repl"}); m.Success {
			t.Errorf("%s succeeded", cmd)
		}
	}
	if m := c.request("attach", map[string]any{}); m.Success {
		t.Error("attach succeeded")
	}
	c.ok("terminate", nil, nil)
	c.event("terminated")

	c2 := newClient(t)
	c2.ok("initialize", nil, nil)
	if m := c2.request("launch", map[string]any{"program": filepath.Join(t.TempDir(), "none.st")}); m.Success {
		t.Error("launched a file that is not there")
	}
	if m := c2.request("stackTrace", map[string]any{}); m.Success {
		t.Error("a stack trace with nothing launched")
	}
}

// TestLaunchSource: a host with no files (a web page) launches a program
// from its text, sets breakpoints on the name it gave it, and changes it
// online by sending the new text.
func TestLaunchSource(t *testing.T) {
	c := newClient(t)
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, nil)
	c.ok("launch", map[string]any{"program": "main.st", "source": program, "realtime": false, "stopOnEntry": true}, nil)
	c.event("initialized")
	incr := lineOf(t, program, "n := n + 1")
	var bps struct{ Breakpoints []breakpoint }
	c.ok("setBreakpoints", map[string]any{"source": map[string]any{"path": "main.st"},
		"breakpoints": []map[string]any{{"line": incr}}}, &bps)
	if !bps.Breakpoints[0].Verified {
		t.Fatalf("breakpoints %+v", bps.Breakpoints)
	}
	c.ok("configurationDone", nil, nil)
	if st := c.event("stopped"); st["reason"] != "entry" {
		t.Fatalf("stopped %v", st)
	}
	if m := c.request("beedance/onlineChange", nil); m.Success {
		t.Error("online change with no text, and no file, succeeded")
	}
	edited := strings.Replace(program, "n := n + 1;", "n := n + 10;", 1)
	var r struct{ Reset, Added, Removed []string }
	c.ok("beedance/onlineChange", map[string]any{"source": edited}, &r)
	c.ok("continue", map[string]any{"threadId": 1}, nil)
	if st := c.event("stopped"); st["reason"] != "breakpoint" {
		t.Fatalf("stopped %v", st)
	}
	c.ok("next", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	vs, _ := c.vars(c.top().ID, "PROGRAM")
	if vs["n"].Value != "10" {
		t.Errorf("n = %s after the changed statement, want 10", vs["n"].Value)
	}
}

// TestIOAndInputs: a simulator reads the I/O image, writes inputs a scan
// at a time, and hears the scans while the program runs.
func TestIOAndInputs(t *testing.T) {
	c := newClient(t)
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, nil)
	c.ok("launch", map[string]any{"program": "main.st", "source": program, "realtime": false,
		"stopOnEntry": true, "scanEvents": 1}, nil)
	c.event("initialized")
	c.ok("configurationDone", nil, nil)
	c.event("stopped")

	// A press and a release before a scan runs: each gets a scan.
	var q struct{ Queued int }
	c.ok("beedance/setInputs", map[string]any{"values": map[string]any{"%IX0.0": true}}, &q)
	c.ok("beedance/setInputs", map[string]any{"values": map[string]any{"%IX0.0": false}}, &q)
	if q.Queued != 2 {
		t.Fatalf("queued %d scans of inputs, want 2", q.Queued)
	}
	if m := c.request("beedance/setInputs", map[string]any{"values": map[string]any{"nothere": true}}); m.Success {
		t.Error("an input that is no variable or address was queued")
	}
	type ioReply struct {
		Scan      int
		ElapsedMs float64
		IO        map[string]any
	}
	var io ioReply
	for i, want := range []bool{true, false} {
		c.ok("beedance/stepScan", nil, nil)
		c.event("stopped")
		c.ok("beedance/io", nil, &io)
		if io.Scan != i+1 || io.ElapsedMs != float64(10*(i+1)) || io.IO["%QX0.0"] != want {
			t.Errorf("scan %d: io %+v, want %%QX0.0 = %v", i+1, io, want)
		}
	}

	c.ok("beedance/force", map[string]any{"name": "%IX0.0", "value": "TRUE"}, nil)
	c.ok("continue", map[string]any{"threadId": 1}, nil)
	ev := c.event("beedance/scan") // the first is the image at launch, scan 0
	for ev["scan"].(float64) < 3 {
		ev = c.event("beedance/scan")
	}
	c.ok("pause", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	if ev["io"].(map[string]any)["%QX0.0"] != true {
		t.Errorf("beedance/scan %v", ev)
	}
	found := false
	for _, v := range ev["values"].([]any) {
		if v := v.(map[string]any); v["name"] == "y" && v["text"] == "TRUE" && v["address"] == "%QX0.0" {
			found = true
		}
	}
	if !found {
		t.Errorf("beedance/scan values %v: no y = TRUE at %%QX0.0", ev["values"])
	}
}

// TestLaunchEngine: a program launched on the evaluator stops, steps, shows
// its variables and a function block's members, and is changed online, as
// one on the VM does; an unknown engine is refused.
func TestLaunchEngine(t *testing.T) {
	c := newClient(t)
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, nil)
	if m := c.request("launch", map[string]any{"program": "main.st", "source": program, "engine": "cobol"}); m.Success {
		t.Error("an unknown engine launched")
	}
	c.ok("launch", map[string]any{"program": "main.st", "source": program, "realtime": false, "engine": "eval"}, nil)
	c.event("initialized")
	call := lineOf(t, program, "c(step")
	c.ok("setBreakpoints", map[string]any{"source": map[string]any{"path": "main.st"},
		"breakpoints": []map[string]any{{"line": call}}}, nil)
	c.ok("configurationDone", nil, nil)
	if st := c.event("stopped"); st["reason"] != "breakpoint" {
		t.Fatalf("stopped %v", st)
	}
	c.ok("stepIn", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	if top := c.top(); top.Name != "Counter" || top.Line != lineOf(t, program, "tmp := total") {
		t.Fatalf("stepped into %+v", top)
	}
	locals, _ := c.vars(c.top().ID, "Locals")
	if locals["step"].Value != "2" {
		t.Errorf("locals %+v: step = %q, want 2", locals, locals["step"].Value)
	}
	c.ok("stepOut", map[string]any{"threadId": 1}, nil)
	c.event("stopped")
	vs, ref := c.vars(c.top().ID, "PROGRAM")
	if vs["c"].VariablesReference == 0 {
		t.Fatalf("c has no members: %+v", vs["c"])
	}
	var members struct{ Variables []variable }
	c.ok("variables", map[string]any{"variablesReference": vs["c"].VariablesReference}, &members)
	total := ""
	for _, v := range members.Variables {
		if v.Name == "total" {
			total = v.Value
		}
	}
	if total != "2" {
		t.Errorf("c.total = %q after the call, want 2 (%+v)", total, members.Variables)
	}
	var set struct{ Value string }
	c.ok("setVariable", map[string]any{"variablesReference": ref, "name": "n", "value": "40"}, &set)
	if set.Value != "40" {
		t.Errorf("set n: %+v", set)
	}
	c.ok("beedance/stepScan", nil, nil) // a change is made between scans
	c.event("stopped")
	edited := strings.Replace(program, "n := n + 1;", "n := n + 10;", 1)
	var r struct{ Kept, Reset []string }
	c.ok("beedance/onlineChange", map[string]any{"source": edited}, &r)
	if len(r.Kept) == 0 {
		t.Errorf("online change on the evaluator kept nothing: %+v", r)
	}
	c.ok("disconnect", nil, nil)
}

