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

// This file is the debugger attached to a program its host runs itself:
// beehive's logic, whose programs run on royaljelly's scheduler, bound to
// a node's tags. The host keeps its VM, its clock and its cycle (inputs,
// the scan, outputs); the session only runs the scan in the host's place
// (Scan) with the debugger's hook in it, and tells the host, before each
// cycle, whether the program is stopped (State). A run (Continue, the
// steps) does not run the VM here: it asks for the program to go on and
// waits for the host's next cycles to stop it. The host's task paces the
// stepping, as an online debugger's target does.

import (
	"context"
	"errors"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/vm"
)

// The debugger attached to a host's program always runs on the VM: the
// host's.

// Host is a program its host runs itself, scan by scan, on a VM of its
// own: what Attach needs of it.
type Host struct {
	// Source is the text Compiled was compiled from.
	Source string
	// Program is the PROGRAM's name.
	Program string
	// Compiled is the program as the host compiled it.
	Compiled *compiler.CompiledProgram
	// Globals is the store the host's VMs run on (vm.NewWithGlobalsStore).
	Globals []object.Object
	// VM runs the program's body: the host's cyclic VM, which Scan runs in
	// the host's place.
	VM *vm.VM
	// Elapsed is the program's clock, as its clock builtins read it; nil
	// counts the scans run under the debugger times Options.Interval.
	Elapsed func() time.Duration
	// LineOffset is how many lines Source has before line 1 of the file
	// the debugger's client shows (a library the host put first): the
	// lines given to the session and the lines it reports are the file's.
	LineOffset int
}

// HostState is what an attached session's host does in its next cycle.
type HostState int

const (
	// HostScan: a scan as usual: the inputs, Scan, the outputs.
	HostScan HostState = iota
	// HostResume: the scan the program stopped in goes on: Scan, then the
	// outputs; the inputs stay as the scan read them.
	HostResume
	// HostHold: the program is stopped: no inputs, no scan, no outputs,
	// and its clock stands still, as under a hold.
	HostHold
)

// The reasons and errors of an attached session.
const (
	// OnDetach: the session was detached (Detach, or the host let go of
	// the program); the program runs on without the debugger.
	OnDetach Reason = "detached"
)

var (
	// ErrStopped is returned by Scan when the program stopped in the scan:
	// it is not complete, and the host must not write its outputs.
	ErrStopped = errors.New("vmdebug: the program stopped under the debugger")
	// ErrDetached is returned by Scan once the session is detached, after
	// the scan it completes: the host runs on without it.
	ErrDetached = errors.New("vmdebug: the debugger is detached")
	// ErrAttached is returned by what an attached session cannot do: an
	// online change (the host makes those), inputs set by a simulator.
	ErrAttached = errors.New("vmdebug: not for a program attached to; its host does that")
	// ErrNotAttached is returned by Scan on a session of Load's.
	ErrNotAttached = errors.New("vmdebug: the session is not attached to a host")
)

// Attach returns a session on a program its host runs (see Host): the
// debugger's view of it, with no VM or clock of its own. The host then
// calls State before each cycle and Scan in place of running its VM, and
// Detach when it lets the program go (a stop, a restart). The program is
// running when attached; Options.Program, Realtime, Budget and Yield do
// not apply.
func Attach(h Host, o Options) (*Session, error) {
	if h.Compiled == nil || h.VM == nil || h.Globals == nil {
		return nil, errors.New("vmdebug: Attach needs the compiled program, its globals and its VM")
	}
	if o.Interval <= 0 {
		o.Interval = 10 * time.Millisecond
	}
	o.Engine = EngineVM
	s := &Session{opts: o, bps: map[int]*Breakpoint{}, forces: map[string]*force{}, stops: make(chan Stop, 1)}
	host := h
	s.host = &host
	object.FinalizeBuiltins()
	img, err := s.analyze(h.Source, h.Program)
	if err != nil {
		return nil, err
	}
	img.eng = attachVMEngine(img, h.Compiled, h.Globals, h.VM)
	s.index(img)
	s.img = img
	return s, nil
}

// hostVM is the attached host's VM engine.
func (s *Session) hostVM() *vmEngine { return s.img.eng.(*vmEngine) }

// Attached reports whether the session is on a host's program (Attach)
// rather than a VM of its own (Load).
func (s *Session) Attached() bool { return s.host != nil }

// State is what the host does in its next cycle (see HostState). It may
// be called at any time.
func (s *Session) State() HostState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state()
}

func (s *Session) state() HostState {
	switch {
	case s.host == nil:
		return HostScan
	case s.stopped && !s.detaching:
		return HostHold
	case s.midScan:
		return HostResume // stopped in a scan and asked to go on, or detaching
	}
	return HostScan
}

// Scan runs one scan of the program for its host, in place of the host
// resetting and running its VM: the writes and forces are applied, then
// the body runs with the debugger in it (a scan the program stopped in
// goes on where it stopped). It returns nil when the scan is complete,
// whether or not the program stops between scans after it (State says);
// ErrStopped when the program stopped in the scan; the VM's error when
// the scan failed, which the host handles as it would; and ErrDetached,
// after a complete scan, once the session is detached.
func (s *Session) Scan() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.host == nil {
		return ErrNotAttached
	}
	if s.host.Elapsed != nil {
		s.elapsed = s.host.Elapsed()
	}
	if s.detaching {
		// The last scan: a scan in progress goes on without the hook.
		if !s.midScan {
			s.scans++
			s.img.eng.Reset()
		}
		s.midScan = true
		err := s.img.eng.Run(nil, false)
		s.midScan = false
		if err != nil {
			return err
		}
		return ErrDetached
	}
	if s.stopped {
		return ErrStopped
	}
	if !s.midScan {
		if s.pauseReq.Swap(false) {
			return s.stopAttached(s.between(OnPause)) // before the scan: it is not started
		}
		s.beginScan()
	}
	s.midScan = true
	var hook func(site) bool
	if s.needsHook() {
		hook = s.hook
	}
	err := s.img.eng.Run(hook, false)
	if errors.Is(err, errPaused) {
		return s.stopAttached(s.stop)
	}
	if err != nil {
		st := Stop{Reason: OnException, Scan: s.scans, Message: err.Error()}
		if fs := s.frames(); len(fs) > 0 {
			st.Line, st.Column = fs[0].Line, fs[0].Column
		}
		s.midScan = false
		s.stopAttached(st)
		return err
	}
	s.midScan = false
	s.applyForces()
	s.record()
	s.sample()
	if s.mode != modeContinue {
		// A step that found no statement to stop at before the scan ended.
		s.stopAttached(s.between(OnScanEnd))
	}
	return nil
}

// needsHook reports whether the next scan needs the debugger's hook in
// it: a breakpoint, a step, or a pause asked for. Without one the scan
// runs at the VM's full speed, and a pause takes effect before the next
// scan.
func (s *Session) needsHook() bool {
	if s.mode != modeContinue || s.pauseReq.Load() {
		return true
	}
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return len(s.bps) > 0
}

// stopAttached holds the program and hands st to the run waiting for a
// stop, if one is; mu is held. It returns ErrStopped for Scan.
func (s *Session) stopAttached(st Stop) error {
	s.stopped = true
	select {
	case s.stops <- st:
	default:
	}
	return ErrStopped
}

// runAttached is run for an attached session: it lets the program go on
// as mode says, and waits for the host's scans to stop it, or for ctx.
func (s *Session) runAttached(ctx context.Context, m mode) (Stop, error) {
	s.mu.Lock()
	if s.detaching {
		s.mu.Unlock()
		return Stop{Reason: OnDetach}, nil
	}
	s.ctx, s.mode = ctx, m
	s.stepDepth, s.stepRow = 1, 0
	if s.midScan {
		if fs := s.frames(); len(fs) > 0 {
			s.stepDepth, s.stepRow = fs[0].depth, fs[0].row
		}
	}
	select {
	case <-s.stops: // a stop no run waited for: stale
	default:
	}
	s.stopped = false
	s.mu.Unlock()

	var st Stop
	select {
	case st = <-s.stops:
	case <-ctx.Done():
		// The run was given up (a disconnect): the program goes on.
		st = Stop{Reason: OnPause, Scan: s.Scans()}
	}
	s.mu.Lock()
	s.ctx, s.mode = nil, modeContinue
	s.mu.Unlock()
	return st, nil
}

// Detach ends the debugger's hold on the program: it runs on, a scan it
// is stopped in completed by the host's next cycle, the breakpoints and
// forces no longer applied, and a run waiting for a stop gets OnDetach.
// The host drops the session when Scan returns ErrDetached, or at once
// if it is not cycling. Detach may be called more than once.
func (s *Session) Detach() {
	if s.host == nil {
		return
	}
	s.mu.Lock()
	s.detaching, s.stopped = true, false
	s.hostVM().unhook()
	s.mu.Unlock()
	select {
	case s.stops <- Stop{Reason: OnDetach}:
	default:
	}
}

// Detached reports whether Detach was called.
func (s *Session) Detached() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detaching
}

// lineOut is a line of the source as the client sees it (Host.LineOffset).
func (s *Session) lineOut(line int) int {
	if line == 0 || s.host == nil {
		return line
	}
	return line - s.host.LineOffset
}

// lineIn is a client's line as a line of the source.
func (s *Session) lineIn(line int) int {
	if line == 0 || s.host == nil {
		return line
	}
	return line + s.host.LineOffset
}
