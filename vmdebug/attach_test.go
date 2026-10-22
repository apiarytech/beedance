/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"context"
	"errors"
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
)

// host is a program run the way beehive's logic runs one: a VM of its
// own, cycled by a task, inputs before each scan and outputs after.
type host struct {
	t        *testing.T
	source   string
	compiled *compiler.CompiledProgram
	globals  []object.Object
	scan     *vm.VM
	elapsed  time.Duration

	mu      sync.Mutex
	dbg     *Session
	cycles  int // cycles the task ran
	scans   int // scans that completed, with their outputs written
	skipped int // cycles the program was held
	errs    []error
	out     []string // the output y after each complete scan
	stop    chan struct{}
	done    chan struct{}
}

func newHost(t *testing.T, source, program string) *host {
	t.Helper()
	object.FinalizeBuiltins()
	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	comp := compiler.NewCompilerWithBuiltins(object.Builtins)
	compiled, err := comp.CompileProgramUnit(prog, program)
	if err != nil {
		t.Fatal(err)
	}
	h := &host{t: t, source: source, compiled: compiled, globals: make([]object.Object, vm.GlobalsSize),
		stop: make(chan struct{}), done: make(chan struct{})}
	init := vm.NewWithGlobalsStore(compiled.InitBytecode, h.globals)
	h.scan = vm.NewWithGlobalsStore(compiled.CyclicBytecode, h.globals)
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return h.elapsed }) {
		init.SetBuiltin(idx, b)
		h.scan.SetBuiltin(idx, b)
	}
	if err := init.Run(); err != nil {
		t.Fatal(err)
	}
	for addr, v := range init.IO() {
		h.scan.IO()[addr] = v
	}
	return h
}

// run is the task: a cycle every ms until stopped.
func (h *host) run() {
	go func() {
		defer close(h.done)
		for {
			select {
			case <-h.stop:
				return
			case <-time.After(time.Millisecond):
				h.cycle()
			}
		}
	}()
}

func (h *host) close() {
	close(h.stop)
	<-h.done
}

// cycle is one cycle of the task, as logic's runner does it with a
// debugger attached.
func (h *host) cycle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cycles++
	state := HostScan
	if h.dbg != nil {
		state = h.dbg.State()
	}
	if state == HostHold {
		h.skipped++
		return
	}
	if state == HostScan {
		h.scan.IO()["%IX0.0"] = &object.Boolean{Value: h.cycles%2 == 0} // the input
		h.elapsed += 10 * time.Millisecond
	}
	var err error
	if h.dbg != nil {
		err = h.dbg.Scan()
		if errors.Is(err, ErrDetached) {
			h.dbg, err = nil, nil
		}
	} else {
		h.scan.Reset()
		err = h.scan.Run()
	}
	if errors.Is(err, ErrStopped) {
		return // no outputs
	}
	if err != nil {
		h.errs = append(h.errs, err)
		return
	}
	h.scans++
	h.out = append(h.out, Format(h.scan.IO()["%QX0.0"]))
}

func (h *host) attach(lineOffset int) *Session {
	h.t.Helper()
	s, err := Attach(Host{Source: h.source, Program: "Main", Compiled: h.compiled, Globals: h.globals, VM: h.scan,
		Elapsed: func() time.Duration { return h.elapsed }, LineOffset: lineOffset}, Options{})
	if err != nil {
		h.t.Fatal(err)
	}
	h.mu.Lock()
	h.dbg = s
	h.mu.Unlock()
	return s
}

func (h *host) counts() (cycles, scans, skipped int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cycles, h.scans, h.skipped
}

func (h *host) attached() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dbg != nil
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func TestAttachBreakpointAndSteps(t *testing.T) {
	h := newHost(t, source, "Main")
	h.run()
	defer h.close()
	waitUntil(t, "the host to cycle", func() bool { _, scans, _ := h.counts(); return scans > 3 })

	s := h.attach(0)
	if !s.Attached() {
		t.Fatal("not attached")
	}
	if s.State() != HostScan {
		t.Fatalf("state = %v, want the host to scan", s.State())
	}
	// The program runs on under the debugger: a Continue waits.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	st, err := s.Continue(ctx)
	cancel()
	if err != nil || st.Reason != OnPause {
		t.Fatalf("Continue with no breakpoint = %+v, %v; want to wait until given up", st, err)
	}
	_, before, _ := h.counts()
	waitUntil(t, "scans under the debugger", func() bool { _, scans, _ := h.counts(); return scans > before+3 })

	// A breakpoint stops the host's scan at the line; the scan is not
	// complete, so no output is written, and the cycles after are held.
	bp := line(t, source, "c(step := 2)")
	bps := s.SetBreakpoints([]BreakpointSpec{{Line: bp}})
	if !bps[0].Verified || bps[0].Actual != bp {
		t.Fatalf("breakpoint = %+v", bps[0])
	}
	st, err = s.Continue(context.Background())
	if err != nil || st.Reason != OnBreakpoint || st.Line != bp {
		t.Fatalf("stopped = %+v, %v", st, err)
	}
	if s.State() != HostHold {
		t.Fatalf("state = %v, want held", s.State())
	}
	_, scansAtStop, _ := h.counts()
	time.Sleep(20 * time.Millisecond)
	if _, scans, skipped := h.counts(); scans != scansAtStop || skipped == 0 {
		t.Fatalf("held: scans %d -> %d, skipped %d", scansAtStop, scans, skipped)
	}
	if fs := s.Frames(); len(fs) != 1 || fs[0].Name != "Main" || fs[0].Line != bp {
		t.Fatalf("frames = %+v", fs)
	}
	n := value(t, s, "n")

	// Step into the function block: the host's next cycle resumes the scan.
	st, err = s.StepIn(context.Background())
	if err != nil || st.Reason != OnStep || st.Line != line(t, source, "tmp := total + step") {
		t.Fatalf("step in = %+v, %v", st, err)
	}
	if fs := s.Frames(); len(fs) != 2 || fs[0].Name != "Counter" {
		t.Fatalf("frames = %+v", fs)
	}
	if value(t, s, "n") != n {
		t.Fatal("n changed in a step that did not pass its statement")
	}
	// Step out, then over to the end of the scan: the outputs are written.
	if st, _ = s.StepOut(context.Background()); st.Reason != OnStep || st.Line != line(t, source, "t(IN := TRUE") {
		t.Fatalf("step out = %+v", st)
	}
	s.SetBreakpoints(nil)
	st, _ = s.StepScan(context.Background())
	if st.Reason != OnScanEnd {
		t.Fatalf("step scan = %+v", st)
	}
	waitUntil(t, "the scan's outputs", func() bool { _, scans, _ := h.counts(); return scans == scansAtStop+1 })
	if s.State() != HostHold {
		t.Fatalf("state after a scan step = %v, want held between scans", s.State())
	}
	// The clock is the host's: it moved one interval for the scan.
	if s.Elapsed() != h.elapsed {
		t.Fatalf("elapsed %v, host %v", s.Elapsed(), h.elapsed)
	}
	// Pause while free-running: between scans, before the next.
	go func() { time.Sleep(10 * time.Millisecond); s.Pause() }()
	if st, _ = s.Continue(context.Background()); st.Reason != OnPause || st.Line != 0 {
		t.Fatalf("pause = %+v", st)
	}
	// A force holds a value the program writes.
	if err := s.Force("y", "TRUE"); err != nil {
		t.Fatal(err)
	}
	st, _ = s.StepScan(context.Background())
	if st.Reason != OnScanEnd {
		t.Fatalf("step scan = %+v", st)
	}
	if v := value(t, s, "y"); v != "TRUE" {
		t.Fatalf("y = %s with y forced TRUE", v)
	}
	if err := s.Unforce("y"); err != nil {
		t.Fatal(err)
	}

	// Detach: the program runs on, the host drops the session.
	s.Detach()
	waitUntil(t, "the host to let go", func() bool { return !h.attached() })
	_, before, _ = h.counts()
	waitUntil(t, "scans without the debugger", func() bool { _, scans, _ := h.counts(); return scans > before+3 })
	if st, _ = s.Continue(context.Background()); st.Reason != OnDetach {
		t.Fatalf("Continue after Detach = %+v", st)
	}
	h.mu.Lock()
	errs := h.errs
	h.mu.Unlock()
	if len(errs) > 0 {
		t.Fatalf("host errors: %v", errs)
	}
}

func TestAttachRefusesWhatTheHostDoes(t *testing.T) {
	h := newHost(t, source, "Main")
	s := h.attach(0)
	if _, err := s.Change(source); !errors.Is(err, ErrAttached) {
		t.Fatalf("Change = %v, want ErrAttached", err)
	}
	if err := s.SetInputs(map[string]any{"%IX0.0": true}); !errors.Is(err, ErrAttached) {
		t.Fatalf("SetInputs = %v, want ErrAttached", err)
	}
	if err := load(t, source).Scan(); !errors.Is(err, ErrNotAttached) {
		t.Fatalf("Scan on a loaded session = %v", err)
	}
}

func TestAttachLineOffset(t *testing.T) {
	// The host compiled a library before the file the client shows.
	library := "FUNCTION Twice : INT\nVAR_INPUT x : INT; END_VAR\nTwice := 2 * x;\nEND_FUNCTION\n"
	full := library + source
	offset := strings.Count(library, "\n")
	h := newHost(t, full, "Main")
	h.run()
	defer h.close()
	s := h.attach(offset)

	bp := line(t, source, "c(step := 2)") // the file's line
	bps := s.SetBreakpoints([]BreakpointSpec{{Line: bp}})
	if !bps[0].Verified || bps[0].Actual != bp {
		t.Fatalf("breakpoint = %+v, want the file's line %d", bps[0], bp)
	}
	st, err := s.Continue(context.Background())
	if err != nil || st.Reason != OnBreakpoint || st.Line != bp {
		t.Fatalf("stopped = %+v, %v; want the file's line %d", st, err, bp)
	}
	if fs := s.Frames(); fs[0].Line != bp {
		t.Fatalf("frame line %d, want %d", fs[0].Line, bp)
	}
	if st, _ = s.StepOver(context.Background()); st.Line != line(t, source, "t(IN := TRUE") {
		t.Fatalf("step over = %+v", st)
	}
	for _, in := range s.Disassembly() {
		if in.Line != 0 && in.Line > strings.Count(source, "\n") {
			t.Fatalf("disassembly line %d is past the file", in.Line)
		}
	}
	s.Detach()
}

func TestAttachExceptionFaultsTheHost(t *testing.T) {
	src := `PROGRAM Main
VAR a : ARRAY[0..1] OF INT; i : INT; END_VAR
i := i + 1;
a[i] := 1;
END_PROGRAM
`
	h := newHost(t, src, "Main")
	s := h.attach(0)
	go func() {
		for i := 0; i < 5; i++ {
			h.cycle()
		}
	}()
	st, _ := s.Continue(context.Background())
	if st.Reason != OnException || !strings.Contains(st.Message, "index") {
		t.Fatalf("stopped = %+v", st)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.errs) == 0 {
		t.Fatal("the host did not see the scan's error")
	}
}
