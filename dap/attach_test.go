/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
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
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
	"github.com/apiarytech/beedance/vmdebug"
)

// plc is a host running the program on a task of its own, as beehive's
// logic does, with a debugger attached through the adapter.
type plc struct {
	mu       sync.Mutex
	compiled *compiler.CompiledProgram
	globals  []object.Object
	scan     *vm.VM
	elapsed  time.Duration
	dbg      *vmdebug.Session
	scans    int
	stop     chan struct{}
	done     chan struct{}
}

func newPLC(t *testing.T, source string) *plc {
	t.Helper()
	object.FinalizeBuiltins()
	ps := parser.New(lexer.New(source))
	prog := ps.ParseProgram()
	if errs := ps.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	compiled, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(prog, "Main")
	if err != nil {
		t.Fatal(err)
	}
	p := &plc{compiled: compiled, globals: make([]object.Object, vm.GlobalsSize), stop: make(chan struct{}), done: make(chan struct{})}
	init := vm.NewWithGlobalsStore(compiled.InitBytecode, p.globals)
	p.scan = vm.NewWithGlobalsStore(compiled.CyclicBytecode, p.globals)
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return p.elapsed }) {
		init.SetBuiltin(idx, b)
		p.scan.SetBuiltin(idx, b)
	}
	if err := init.Run(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.done)
		for {
			select {
			case <-p.stop:
				return
			case <-time.After(time.Millisecond):
				p.cycle()
			}
		}
	}()
	t.Cleanup(func() { close(p.stop); <-p.done })
	return p
}

func (p *plc) cycle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := vmdebug.HostScan
	if p.dbg != nil {
		state = p.dbg.State()
	}
	if state == vmdebug.HostHold {
		return
	}
	if state == vmdebug.HostScan {
		p.elapsed += 10 * time.Millisecond
	}
	var err error
	if p.dbg != nil {
		err = p.dbg.Scan()
		if errors.Is(err, vmdebug.ErrDetached) {
			p.dbg, err = nil, nil
		}
	} else {
		p.scan.Reset()
		err = p.scan.Run()
	}
	if err == nil {
		p.scans++
	}
}

func (p *plc) attach(a AttachArgs) (*vmdebug.Session, error) {
	if a.Program != "Main" {
		return nil, errors.New("no program " + a.Program)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := vmdebug.Attach(vmdebug.Host{Source: program, Program: "Main", Compiled: p.compiled, Globals: p.globals, VM: p.scan,
		Elapsed: func() time.Duration { return p.elapsed }}, vmdebug.Options{})
	if err != nil {
		return nil, err
	}
	p.dbg = s
	return s, nil
}

func (p *plc) attached() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dbg != nil
}

func (p *plc) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scans
}

// newAttachClient is newClient with the server's Attach set.
func newAttachClient(t *testing.T, attach Attacher) *client {
	t.Helper()
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	c := &client{t: t, w: cw, in: make(chan *message, 100), done: make(chan error, 1)}
	go func() {
		srv := NewServer(sr, sw)
		srv.Attach = attach
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

func TestAttachRefusedWithoutHost(t *testing.T) {
	c := newClient(t)
	c.request("initialize", nil)
	rsp := c.request("attach", map[string]any{"program": "Main"})
	if rsp.Success || !strings.Contains(rsp.Message, "host") {
		t.Fatalf("attach without a host: %+v", rsp)
	}
}

func TestAttachToHost(t *testing.T) {
	p := newPLC(t, program)
	c := newAttachClient(t, p.attach)
	c.request("initialize", nil)
	if rsp := c.request("attach", map[string]any{"program": "Nope"}); rsp.Success {
		t.Fatal("attached to a program the host has not got")
	}
	rsp := c.request("attach", map[string]any{"program": "Main", "file": "C:/plant/main.st"})
	if !rsp.Success {
		t.Fatalf("attach: %s", rsp.Message)
	}
	c.event("initialized")
	bp := lineOf(t, program, "c(step := 2)")
	rsp = c.request("setBreakpoints", map[string]any{"source": map[string]any{"path": "C:/plant/main.st"},
		"breakpoints": []map[string]any{{"line": bp}}})
	var bps struct{ Breakpoints []breakpoint }
	json.Unmarshal(rsp.Body, &bps)
	if len(bps.Breakpoints) != 1 || !bps.Breakpoints[0].Verified {
		t.Fatalf("breakpoints = %+v", bps)
	}
	c.request("configurationDone", nil)

	// The program, running on the host, stops at the breakpoint.
	if body := c.event("stopped"); body["reason"] != "breakpoint" {
		t.Fatalf("stopped: %v", body)
	}
	rsp = c.request("stackTrace", map[string]any{"threadId": 1})
	var st struct {
		StackFrames []struct {
			Name   string
			Line   int
			Source struct{ Path string }
		}
	}
	json.Unmarshal(rsp.Body, &st)
	if len(st.StackFrames) != 1 || st.StackFrames[0].Line != bp || !strings.HasSuffix(st.StackFrames[0].Source.Path, "main.st") {
		t.Fatalf("stack = %+v", st)
	}
	scans := p.count()
	time.Sleep(20 * time.Millisecond)
	if p.count() != scans {
		t.Fatal("the host scanned while the program was stopped")
	}

	// What the host does is refused; forcing is allowed.
	if rsp = c.request("beedance/onlineChange", map[string]any{}); rsp.Success || !strings.Contains(rsp.Message, "host") {
		t.Fatalf("online change on an attached program: %+v", rsp)
	}
	if rsp = c.request("beedance/setInputs", map[string]any{"values": map[string]any{"%IX0.0": true}}); rsp.Success {
		t.Fatal("inputs set on an attached program")
	}
	if rsp = c.request("beedance/speed", map[string]any{"factor": 2}); rsp.Success {
		t.Fatal("speed set on an attached program")
	}
	if rsp = c.request("beedance/force", map[string]any{"name": "y", "value": "TRUE"}); !rsp.Success {
		t.Fatalf("force: %s", rsp.Message)
	}

	// A step runs in the host's next cycle.
	c.request("next", map[string]any{"threadId": 1})
	if body := c.event("stopped"); body["reason"] != "step" {
		t.Fatalf("after next: %v", body)
	}
	c.request("setBreakpoints", map[string]any{"source": map[string]any{"path": "C:/plant/main.st"}, "breakpoints": []map[string]any{}})
	c.request("continue", map[string]any{"threadId": 1})
	waitFor(t, "scans to go on", func() bool { return p.count() > scans+3 })

	// Disconnecting detaches: the program runs on without the debugger.
	c.request("disconnect", nil)
	waitFor(t, "the host to let go", func() bool { return !p.attached() })
	scans = p.count()
	waitFor(t, "scans without the debugger", func() bool { return p.count() > scans+3 })
}

func TestAttachStopOnEntryAndHostDetach(t *testing.T) {
	p := newPLC(t, program)
	c := newAttachClient(t, p.attach)
	c.request("initialize", nil)
	if rsp := c.request("attach", map[string]any{"program": "Main", "stopOnEntry": true}); !rsp.Success {
		t.Fatalf("attach: %s", rsp.Message)
	}
	c.request("configurationDone", nil)
	if body := c.event("stopped"); body["reason"] != "pause" {
		t.Fatalf("stop on entry: %v", body)
	}
	rsp := c.request("stackTrace", map[string]any{"threadId": 1})
	if !strings.Contains(string(rsp.Body), "Main.st") {
		t.Fatalf("the source without a file: %s", rsp.Body)
	}
	c.request("continue", map[string]any{"threadId": 1})
	// The host lets the program go (a stop, a restart): the session ends.
	p.mu.Lock()
	p.dbg.Detach()
	p.mu.Unlock()
	c.event("terminated")
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}
