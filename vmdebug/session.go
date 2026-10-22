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
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/watch"
)

// Options configure a Session.
type Options struct {
	// Program is the PROGRAM to run; empty is the first that is not a
	// test (TEST_...), else the first.
	Program string
	// Engine runs the program: the VM (EngineVM, the default), the
	// evaluator (EngineEvaluator) or a transpiled program (EngineGo).
	Engine Engine
	// GoBuild configures EngineGo (engine_go.go); the others ignore it.
	GoBuild GoBuild
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

// Session is a PROGRAM running on an engine (the VM, the evaluator or a
// transpiled program: Options.Engine) under a debugger. A run
// (Continue, the steps) holds the session until it stops; while it runs,
// another goroutine may Pause it, set breakpoints, Force, Unforce and Set
// (applied when the next scan starts) and read Live values. The other
// methods wait for the run to stop.
type Session struct {
	mu   sync.Mutex // the engine and its state; held by a run
	opts Options

	img     *image // replaced only with mu and cmu held
	elapsed time.Duration
	scans   int
	midScan bool // a scan has started and not ended (paused in it)
	closed  bool

	// evalEnd ends the evaluator's session (engine_eval.go), which the
	// session holds from its first evaluator engine until Close.
	evalEnd func()

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
	source  string
	name    string // the PROGRAM's name, as declared
	program *ast.Program
	decl    *ast.ProgramDeclaration
	eng     engine

	vars    []watch.Variable
	decls   map[string]Variable // by upper-case name
	located map[string]Variable // by upper-case address
	userPOU map[string]bool     // the source's own POUs, upper case

	rows     map[int]bool // lines that have a statement
	pouRows  []int        // lines POUs start on
	firstRow int          // the body's first statement

	plants []*plant // the plant models, run after each scan (plant.go)
}

// Load builds source's PROGRAM on the engine and runs its initialization;
// the session is then between scans, before the first.
func Load(source string, o Options) (*Session, error) {
	if o.Interval <= 0 {
		o.Interval = 10 * time.Millisecond
	}
	if o.Budget <= 0 {
		o.Budget = 10_000_000
	}
	var err error
	if o.Engine, err = ParseEngine(string(o.Engine)); err != nil {
		return nil, err
	}
	s := &Session{opts: o, bps: map[int]*Breakpoint{}, forces: map[string]*force{}}
	if o.Realtime {
		s.SetSpeed(1)
	}
	img, err := s.build(source, o.Program)
	if err != nil {
		s.Close()
		return nil, err
	}
	s.img = img
	return s, nil
}

// Engine is the engine the session runs the program on.
func (s *Session) Engine() Engine { return s.opts.Engine }

// Close ends the session: it frees what the engine holds (the evaluator,
// a transpiled program's process). A run in progress must have stopped.
// The session is not to be used after it.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.img != nil && s.img.eng != nil {
		s.img.eng.Close()
	}
	if s.evalEnd != nil {
		s.evalEnd()
		s.evalEnd = nil
	}
}

// build makes source's PROGRAM name on the engine and runs its
// initialization; the clock is the session's.
func (s *Session) build(source, name string) (*image, error) {
	img, err := s.analyze(source, name)
	if err != nil {
		return nil, err
	}
	clock := func() time.Duration { return s.elapsed }
	switch s.opts.Engine {
	case EngineEvaluator:
		img.eng, err = newEvalEngine(img, s.opts, s.evalSession)
	case EngineGo:
		img.eng, err = newGoEngine(img, s.opts, clock)
	default:
		img.eng, err = newVMEngine(img, s.opts, clock)
	}
	if err != nil {
		return nil, err
	}
	s.index(img)
	if err := s.buildPlants(img); err != nil {
		img.eng.Close()
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
		decls: map[string]Variable{}, located: map[string]Variable{}, userPOU: map[string]bool{}}
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

// index reads what the debugger needs of img's engine: its variables by
// name and address, and the lines a breakpoint can be on (the body's and
// the source's POUs').
func (s *Session) index(img *image) {
	for _, v := range img.eng.Variables() {
		img.decls[strings.ToUpper(v.Name)] = v
		if v.Address != "" {
			img.located[strings.ToUpper(v.Address)] = v
		}
	}
	img.rows, img.firstRow = img.eng.Rows(), img.eng.FirstRow()
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
		s.hint(m)
		err := s.img.eng.Run(s.hook, m == modeInstruction)
		if errors.Is(err, errPaused) {
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

// hint tells an engine that asks (hinter) which statements the hook wants
// in the run to come: every one for a step or a pause, else the
// breakpoints' lines.
func (s *Session) hint(m mode) {
	h, ok := s.img.eng.(hinter)
	if !ok {
		return
	}
	s.cmu.Lock()
	rows := make([]int, 0, len(s.bps))
	for row := range s.bps {
		rows = append(rows, row)
	}
	s.cmu.Unlock()
	h.hints(rows, m != modeContinue || s.pauseReq.Load())
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
// applied, the engine starts the body over.
func (s *Session) beginScan() {
	s.scans++
	if s.host == nil || s.host.Elapsed == nil {
		s.elapsed += s.opts.Interval
	}
	s.holdRow = 0
	s.applyInputs()
	s.applyPending()
	s.applyForces()
	s.img.eng.Reset()
}

// between is a stop between scans.
func (s *Session) between(r Reason) Stop {
	return Stop{Reason: r, Scan: s.scans}
}

// hook is the engine's hook while a run is in progress: it stops before a
// statement of the source as the run's mode, the breakpoints and Pause
// say.
func (s *Session) hook(at site) bool {
	stopAt := func(r Reason, msg string) bool {
		s.stop = Stop{Reason: r, Scan: s.scans, Line: s.lineOut(at.row), Column: at.column, Message: msg}
		s.holdRow = at.row
		return true
	}
	if s.mode == modeInstruction {
		return stopAt(OnStep, "")
	}
	if !at.statement {
		return false
	}
	if at.row != s.holdRow {
		s.holdRow = 0
	}
	if s.pauseReq.Swap(false) || s.ctx != nil && s.ctx.Err() != nil {
		return stopAt(OnPause, "")
	}
	switch s.mode {
	case modeIn:
		if at.depth > s.stepDepth || at.row != s.stepRow {
			return stopAt(OnStep, "")
		}
	case modeOver:
		if at.depth < s.stepDepth || at.depth == s.stepDepth && at.row != s.stepRow {
			return stopAt(OnStep, "")
		}
	case modeOut:
		if at.depth < s.stepDepth {
			return stopAt(OnStep, "")
		}
	}
	if s.holdRow != 0 {
		return false
	}
	s.cmu.Lock()
	b := s.bps[at.row]
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
	// scans, and on an engine with no disassembly.
	Address int

	depth int
	row   int // Line as a line of the source (attach.go: Host.LineOffset)
	eng   frame
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
	efs := s.img.eng.Frames()
	out := make([]Frame, 0, len(efs))
	for _, f := range efs {
		out = append(out, Frame{Name: f.name, Line: s.lineOut(f.row), Column: f.column, Address: f.address,
			depth: f.depth, row: f.row, eng: f})
	}
	return out
}
