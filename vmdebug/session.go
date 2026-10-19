/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package vmdebug debugs a PROGRAM running on the VM, scan by scan, as a
// PLC runs it: breakpoints (with conditions) on the lines of the source,
// stepping statement by statement (over, into and out of calls) or scan by
// scan, pausing, reading the program's variables and the locals of each
// call, writing them, forcing them and the I/O image, and changing the
// program online, between scans, with its variables carried over.
//
//	s, _ := vmdebug.Load(source, vmdebug.Options{})
//	s.SetBreakpoints([]vmdebug.BreakpointSpec{{Line: 12, Condition: "n > 3"}})
//	stop, _ := s.Continue(ctx)           // stop.Reason == vmdebug.OnBreakpoint
//	frames := s.Frames()                 // where, innermost first
//	s.Force("%IX0.0", "TRUE")            // before and after every scan
//	stop, _ = s.StepScan(ctx)            // to the end of the scan
//	report, _ := s.Change(edited)        // online change
//
// The clock is simulated: each scan starts Interval after the last, so a
// program stopped at a breakpoint finds its timers where it left them. A
// debugger front end speaks to a Session, e.g. over the Debug Adapter
// Protocol (package dap).
package vmdebug

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
	"github.com/apiarytech/beedance/watch"
)

// Options configure a Session.
type Options struct {
	// Program is the PROGRAM to run; empty is the first that is not a
	// test (TEST_...), else the first.
	Program string
	// Interval is the simulated time from one scan to the next; default
	// 10 ms.
	Interval time.Duration
	// Realtime makes Continue wait Interval of wall-clock time between
	// scans, so a program runs at its pace while it is watched.
	Realtime bool
	// Budget bounds the instructions of a scan, so a runaway loop stops
	// with an exception; default 10 million, as package sil.
	Budget int64
	// Watch chooses the members Watched lists.
	Watch watch.Options
	// AllowInputWrites lets a POU assign its own inputs, as CODESYS does
	// (see package inputwrites).
	AllowInputWrites bool
	// Notify, if set, is told what went wrong with a write or force made
	// while a run was in progress, when it was applied.
	Notify func(msg string)
	// YieldEvery, if positive, makes a run that is not Realtime give up
	// the processor between scans once this much wall-clock time has gone
	// since it last did, so a single-threaded host (a web page, GOOS=js)
	// can deliver Pause, writes and forces while scans run as fast as they
	// go. Zero never yields, as before.
	YieldEvery time.Duration
	// Yield gives up the processor; nil sleeps a millisecond. Under
	// GOOS=js the host's event loop runs only once every goroutine is
	// blocked with no timer due, so a shorter sleep (or runtime.Gosched)
	// would not let it in. A host may give a function that blocks until
	// its event loop has turned.
	Yield func()
	// Sampled, if set, is given the Sample of each scan's end once Live
	// has been called, from the goroutine running the scans, while the
	// session is held: it must not call the session's methods that wait
	// for a run to stop.
	Sampled func(Sample)
}

// Reason is why a session stopped.
type Reason string

// The reasons a session stops.
const (
	// OnEntry: loaded, before the first scan.
	OnEntry Reason = "entry"
	// OnBreakpoint: before a statement on a breakpoint's line.
	OnBreakpoint Reason = "breakpoint"
	// OnStep: a statement step ended.
	OnStep Reason = "step"
	// OnPause: Pause was called, or the context of the run was done.
	OnPause Reason = "pause"
	// OnScanEnd: between scans, after StepScan or a step out of the body.
	OnScanEnd Reason = "scan"
	// OnException: the scan failed; Message says why.
	OnException Reason = "exception"
)

// Stop is where and why a run stopped.
type Stop struct {
	Reason Reason
	// Scan is the scan stopped in, or, between scans, the last one run.
	Scan int
	// Line and Column are the statement that runs next (or failed); 0
	// between scans.
	Line, Column int
	// Message is an exception's error, or why a breakpoint's condition
	// could not be evaluated.
	Message string
}

// ErrMidScan is returned by what may only be done between scans (Change).
var ErrMidScan = errors.New("vmdebug: a scan is in progress; step to its end first")

// Session is a PROGRAM running on the VM under a debugger. A run
// (Continue, the steps) holds the session until it stops; while it runs,
// another goroutine may Pause it, set breakpoints, Force, Unforce and Set
// (applied when the next scan starts) and read Live values. The other
// methods wait for the run to stop.
type Session struct {
	mu   sync.Mutex // the VM and its state; held by a run
	opts Options

	img     *image // replaced only with mu and cmu held
	elapsed time.Duration
	scans   int
	midScan bool // a scan has started and not ended (paused in it)

	cmu     sync.Mutex          // what may change while a run is in progress:
	bps     map[int]*Breakpoint // the breakpoints, by the line they are set on
	forces  map[string]*force   // the forces, by upper-case name or address
	pending []write             // writes for the next scan
	inputs  [][]write           // inputs for the scans to come, one entry a scan (io.go)
	changes []*changeRequest    // online changes for the next scan boundary

	lmu        sync.Mutex
	live       Sample
	liveWanted atomic.Bool

	trace   atomic.Pointer[tracer]   // the signals recorded every scan (trace.go)
	sampled atomic.Pointer[[]string] // more names each Sample keeps (SetSampled)

	// The last values of the sampled names, and when they were read: a
	// run reads them at most every ExtraEvery (held by the run).
	extra    map[string]string
	extraFor *[]string
	extraAt  time.Time
	speed    atomic.Uint64 // a float64's bits: SetSpeed

	// While a run is in progress: what stops it.
	ctx       context.Context
	mode      mode
	stepDepth int // the frame depth a step started at
	stepRow   int // the line it started on; 0 for any
	holdRow   int // a breakpoint's line just stopped at, not to stop at again at once
	pauseReq  atomic.Bool
	stop      Stop

	// An attached session (attach.go): the host runs the scans.
	host      *Host
	stopped   bool      // the debugger holds the program: the host skips its cycles
	detaching bool      // Detach was called: the host lets go after its next scan
	stops     chan Stop // the stop a waiting run gets
}

type mode int

const (
	modeContinue mode = iota
	modeIn
	modeOver
	modeOut
	modeScan
	modeInstruction
)

// image is what Load or Change made of a source.
type image struct {
	source   string
	name     string // the PROGRAM's name, as declared
	program  *ast.Program
	decl     *ast.ProgramDeclaration
	compiled *compiler.CompiledProgram
	globals  []object.Object
	vm       *vm.VM

	vars    []watch.Variable
	decls   map[string]compiler.Variable // by upper-case name
	located map[string]compiler.Variable // by upper-case address
	userPOU map[string]bool              // the source's own POUs, upper case

	marks    map[*object.CompiledFunction]map[int]object.SourceLine // statement starts; nil: not the source's
	rows     map[int]bool                                           // lines that have a statement
	pouRows  []int                                                  // lines POUs start on
	firstRow int                                                    // the body's first statement

	blocks    []block // the code at its addresses (code.go)
	blockBase map[*object.CompiledFunction]int

	plants []*plant // the plant models, run after each scan (plant.go)
}

// Load compiles source's PROGRAM and runs its initialization; the session
// is then between scans, before the first.
func Load(source string, o Options) (*Session, error) {
	if o.Interval <= 0 {
		o.Interval = 10 * time.Millisecond
	}
	if o.Budget <= 0 {
		o.Budget = 10_000_000
	}
	s := &Session{opts: o, bps: map[int]*Breakpoint{}, forces: map[string]*force{}}
	if o.Realtime {
		s.SetSpeed(1)
	}
	img, err := s.build(source, o.Program)
	if err != nil {
		return nil, err
	}
	s.img = img
	return s, nil
}

// build compiles source's PROGRAM name and runs its initialization on a
// store of its own; the clock is the session's.
func (s *Session) build(source, name string) (*image, error) {
	object.FinalizeBuiltins()
	img, err := s.analyze(source, name)
	if err != nil {
		return nil, err
	}
	comp := compiler.NewCompilerWithBuiltins(object.Builtins)
	comp.AllowInputWrites = s.opts.AllowInputWrites
	compiled, err := comp.CompileProgramUnit(img.program, img.name)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	img.compiled = compiled

	img.globals = make([]object.Object, vm.GlobalsSize)
	initVM := vm.NewWithGlobalsStore(compiled.InitBytecode, img.globals)
	img.vm = vm.NewWithGlobalsStore(compiled.CyclicBytecode, img.globals)
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return s.elapsed }) {
		initVM.SetBuiltin(idx, b)
		img.vm.SetBuiltin(idx, b)
	}
	initVM.SetBudget(s.opts.Budget)
	if err := initVM.Run(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	for addr, v := range initVM.IO() {
		img.vm.IO()[addr] = v
	}
	img.vm.SetBudget(s.opts.Budget)
	s.index(img)
	if err := s.buildPlants(img); err != nil {
		return nil, err
	}
	return img, nil
}

// analyze parses source and picks its PROGRAM name: the image before it
// is compiled, with what the debugger reads from the syntax tree.
func (s *Session) analyze(source, name string) (*image, error) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("parse: %s", strings.Join(errs, "; "))
	}
	decl, err := pick(program, name)
	if err != nil {
		return nil, err
	}
	img := &image{source: source, name: decl.Name.Value, program: program, decl: decl,
		decls: map[string]compiler.Variable{}, located: map[string]compiler.Variable{},
		userPOU: map[string]bool{}, marks: map[*object.CompiledFunction]map[int]object.SourceLine{},
		rows: map[int]bool{}}
	// The source's own POUs, before compiling adds the standard ones.
	for _, st := range program.Statements {
		var pou string
		switch n := st.(type) {
		case *ast.FunctionBlockDeclaration:
			pou = n.Name.Value
		case *ast.FunctionDeclaration:
			pou = n.Name.Value
		case *ast.ProgramDeclaration:
			pou = n.Name.Value
		default:
			continue
		}
		img.userPOU[strings.ToUpper(pou)] = true
		if row, _ := st.Pos(); row > 0 {
			img.pouRows = append(img.pouRows, row)
		}
	}
	img.vars = watch.Variables(program, decl, s.opts.Watch)
	return img, nil
}

// index reads what the debugger needs of img's compiled program: its
// variables by name and address, and the lines a breakpoint can be on
// (the body's and the source's POUs').
func (s *Session) index(img *image) {
	compiled := img.compiled
	for _, v := range compiled.Variables {
		img.decls[strings.ToUpper(v.Name)] = v
		if v.Address != "" {
			img.located[strings.ToUpper(v.Address)] = v
		}
	}
	for _, l := range compiled.CyclicBytecode.Lines {
		img.rows[l.Row] = true
		if img.firstRow == 0 {
			img.firstRow = l.Row
		}
	}
	for _, c := range compiled.CyclicBytecode.Constants {
		if fn, ok := c.(*object.CompiledFunction); ok && img.isUser(fn) {
			for _, l := range fn.Lines {
				img.rows[l.Row] = true
			}
		}
	}
}

// pick finds the PROGRAM named name, or, when name is empty, the first that
// is neither a test nor a plant model, else the first.
func pick(program *ast.Program, name string) (*ast.ProgramDeclaration, error) {
	var all []*ast.ProgramDeclaration
	for _, st := range program.Statements {
		if pd, ok := st.(*ast.ProgramDeclaration); ok && pd.Name != nil {
			all = append(all, pd)
		}
	}
	if len(all) == 0 {
		return nil, errors.New("the source declares no PROGRAM")
	}
	for _, pd := range all {
		if name == "" && !strings.HasPrefix(strings.ToUpper(pd.Name.Value), "TEST_") && !IsPlant(pd.Name.Value) ||
			name != "" && strings.EqualFold(pd.Name.Value, name) {
			return pd, nil
		}
	}
	if name == "" {
		return all[0], nil
	}
	return nil, fmt.Errorf("no PROGRAM %s", name)
}

// isUser reports whether fn is the code of one of the source's own POUs
// (or of the program's body), whose lines are the source's; a standard
// block's are not.
func (img *image) isUser(fn *object.CompiledFunction) bool {
	if fn.Name == "" {
		return true
	}
	pou, _, _ := strings.Cut(fn.Name, ".")
	return img.userPOU[strings.ToUpper(pou)]
}

// markAt is the statement starting at ip in fn, if fn is the source's.
func (img *image) markAt(fn *object.CompiledFunction, ip int) (object.SourceLine, bool) {
	m, seen := img.marks[fn]
	if !seen {
		if img.isUser(fn) && len(fn.Lines) > 0 {
			m = make(map[int]object.SourceLine, len(fn.Lines))
			for _, l := range fn.Lines {
				m[l.Offset] = l
			}
		}
		img.marks[fn] = m
	}
	l, ok := m[ip]
	return l, ok
}

// Program is the name of the PROGRAM being debugged. It may be called
// while a run is in progress.
func (s *Session) Program() string {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return s.img.name
}

// Source is the source the session runs (the last Change's). It may be
// called while a run is in progress.
func (s *Session) Source() string {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return s.img.source
}

// Scans is the number of scans started.
func (s *Session) Scans() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scans
}

// Elapsed is the simulated time at the last scan's start.
func (s *Session) Elapsed() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.elapsed
}

// Continue runs scan after scan until a breakpoint, Pause, ctx's end or
// an exception.
func (s *Session) Continue(ctx context.Context) (Stop, error) { return s.run(ctx, modeContinue) }

// StepOver runs to the next statement of the call it is stopped in, or of
// its callers; between scans, to the first statement of the next scan.
func (s *Session) StepOver(ctx context.Context) (Stop, error) { return s.run(ctx, modeOver) }

// StepIn runs to the next statement, in a call the current one makes if
// it makes one.
func (s *Session) StepIn(ctx context.Context) (Stop, error) { return s.run(ctx, modeIn) }

// StepOut runs until the call it is stopped in returns, to the statement
// after the call; out of the program's body, to the end of the scan.
func (s *Session) StepOut(ctx context.Context) (Stop, error) { return s.run(ctx, modeOut) }

// StepScan runs to the end of the scan in progress, or, between scans,
// runs the next scan; a breakpoint on the way stops it there.
func (s *Session) StepScan(ctx context.Context) (Stop, error) { return s.run(ctx, modeScan) }

// Pause stops a run at its next statement, or between scans. It may be
// called from another goroutine while Continue or a step runs; called
// while none does, it stops the next one at once.
func (s *Session) Pause() { s.pauseReq.Store(true) }

func (s *Session) run(ctx context.Context, m mode) (Stop, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.host != nil {
		return s.runAttached(ctx, m)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx, s.mode = ctx, m
	defer func() {
		s.ctx = nil
		s.serveChanges() // between scans, made; in one, refused
	}()
	s.stepDepth, s.stepRow = 1, 0
	if s.midScan {
		if fs := s.frames(); len(fs) > 0 {
			s.stepDepth, s.stepRow = fs[0].depth, fs[0].row
		}
	}
	begun := false // this run has started a scan
	yielded := time.Now()
	due := time.Now() // when the next scan is due, at a speed
	for {
		if !s.midScan {
			if begun && s.opts.YieldEvery > 0 && time.Since(yielded) >= s.opts.YieldEvery {
				if s.opts.Yield != nil {
					s.opts.Yield()
				} else {
					sleep(ctx, time.Millisecond)
				}
				yielded = time.Now()
			}
			s.serveChanges()
			if s.pauseReq.Swap(false) || ctx.Err() != nil {
				return s.between(OnPause), nil
			}
			if speed := s.Speed(); begun && speed > 0 && m == modeContinue {
				// A scan every Interval/speed of wall-clock time; behind by
				// more than a moment (a slow host), it carries on from now
				// rather than racing to catch up.
				due = due.Add(time.Duration(float64(s.opts.Interval) / speed))
				if wait := time.Until(due); wait > 0 {
					t := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						t.Stop()
						return s.between(OnPause), nil
					case <-t.C:
					}
					yielded = time.Now()
				} else if wait < -100*time.Millisecond {
					due = time.Now()
				}
			}
			s.beginScan()
			begun = true
		}
		s.midScan = true
		s.img.vm.SetHook(s.hook)
		err := s.img.vm.Run()
		if errors.Is(err, vm.ErrPaused) {
			return s.stop, nil
		}
		if err != nil {
			st := Stop{Reason: OnException, Scan: s.scans, Message: err.Error()}
			if fs := s.frames(); len(fs) > 0 {
				st.Line, st.Column = fs[0].Line, fs[0].Column
			}
			s.midScan = false
			return st, nil
		}
		s.midScan = false
		s.applyForces()
		if err := s.runPlants(); err != nil {
			return Stop{Reason: OnException, Scan: s.scans, Message: "plant model " + err.Error()}, nil
		}
		s.record()
		s.sample()
		switch m {
		case modeScan, modeOut, modeInstruction:
			return s.between(OnScanEnd), nil
		case modeIn, modeOver:
			if begun {
				// A whole scan ran with no statement of the source to stop at.
				return s.between(OnScanEnd), nil
			}
			s.stepDepth, s.stepRow = 1, 0 // the step goes on into the next scan
		}
	}
}

// sleep blocks the run for d or until ctx is done, so the scheduler runs
// the other goroutines and, under GOOS=js, returns to the host's event
// loop, which delivers its callbacks (Pause, writes) meanwhile.
// runtime.Gosched would not: it never leaves Go.
func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		t.Stop()
	case <-t.C:
	}
}

// beginScan starts the next scan: the clock moves on, the forces are
// applied, the VM starts its body over.
func (s *Session) beginScan() {
	s.scans++
	if s.host == nil || s.host.Elapsed == nil {
		s.elapsed += s.opts.Interval
	}
	s.holdRow = 0
	s.applyInputs()
	s.applyPending()
	s.applyForces()
	s.img.vm.Reset()
}

// between is a stop between scans.
func (s *Session) between(r Reason) Stop {
	return Stop{Reason: r, Scan: s.scans}
}

// hook is the VM's hook while a run is in progress: it stops before a
// statement of the source as the run's mode, the breakpoints and Pause
// say.
func (s *Session) hook(fn *object.CompiledFunction, ip, depth int) bool {
	if s.mode == modeInstruction {
		if !s.img.isUser(fn) {
			return false
		}
		l, _ := object.LineAt(fn.Lines, ip)
		s.stop = Stop{Reason: OnStep, Scan: s.scans, Line: s.lineOut(l.Row), Column: l.Column}
		s.holdRow = l.Row
		return true
	}
	line, ok := s.img.markAt(fn, ip)
	if !ok {
		return false
	}
	if line.Row != s.holdRow {
		s.holdRow = 0
	}
	stopAt := func(r Reason, msg string) bool {
		s.stop = Stop{Reason: r, Scan: s.scans, Line: s.lineOut(line.Row), Column: line.Column, Message: msg}
		s.holdRow = line.Row
		return true
	}
	if s.pauseReq.Swap(false) || s.ctx != nil && s.ctx.Err() != nil {
		return stopAt(OnPause, "")
	}
	switch s.mode {
	case modeIn:
		if depth > s.stepDepth || line.Row != s.stepRow {
			return stopAt(OnStep, "")
		}
	case modeOver:
		if depth < s.stepDepth || depth == s.stepDepth && line.Row != s.stepRow {
			return stopAt(OnStep, "")
		}
	case modeOut:
		if depth < s.stepDepth {
			return stopAt(OnStep, "")
		}
	}
	if s.holdRow != 0 {
		return false
	}
	s.cmu.Lock()
	b := s.bps[line.Row]
	s.cmu.Unlock()
	if b != nil {
		if holds, msg := s.holds(b); holds {
			s.cmu.Lock()
			b.Hits++
			s.cmu.Unlock()
			return stopAt(OnBreakpoint, msg)
		}
	}
	return false
}

// Frame is a call the session is stopped in.
type Frame struct {
	// Name is the POU: the program's name for its body, a function
	// block's for its body, "FB.Method" for a method.
	Name string
	// Line and Column are the statement it is at: the one that runs next
	// in the innermost frame, the call in the others.
	Line, Column int
	// Address is the instruction it is at (see Disassembly); 0 between
	// scans.
	Address int

	depth int
	row   int // Line as a line of the source (attach.go: Host.LineOffset)
	vm    vm.FrameInfo
}

// Frames are the calls the session is stopped in, innermost first; between
// scans, the program's body, at its first statement. A standard function
// block's code is not shown: it has no place in the source.
func (s *Session) Frames() []Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frames()
}

func (s *Session) frames() []Frame {
	if !s.midScan {
		return []Frame{{Name: s.img.name, Line: s.lineOut(s.img.firstRow), Column: 1, depth: 1, row: s.img.firstRow}}
	}
	vfs := s.img.vm.Frames()
	out := make([]Frame, 0, len(vfs))
	for i := len(vfs) - 1; i >= 0; i-- {
		f := vfs[i]
		if !s.img.isUser(f.Fn) {
			continue
		}
		fr := Frame{Name: f.Fn.Name, depth: i + 1, vm: f}
		if fr.Name == "" {
			fr.Name = s.img.name
		}
		if l, ok := object.LineAt(f.Fn.Lines, f.IP); ok {
			fr.Line, fr.Column, fr.row = s.lineOut(l.Row), l.Column, l.Row
		}
		fr.Address = s.img.address(f.Fn, f.IP)
		out = append(out, fr)
	}
	return out
}
