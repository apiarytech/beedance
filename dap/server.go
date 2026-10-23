/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package dap is a debug adapter for beedance: it speaks the Debug Adapter
// Protocol (DAP) of VS Code and other editors, and debugs an IEC 61131-3
// PROGRAM on the VM through package vmdebug. `beedance -dap` serves it on
// stdin and stdout.
//
// An editor launches a .st file ("program"); the adapter runs its PROGRAM
// scan by scan, and offers breakpoints (with conditions), stepping over,
// into and out of calls, by instruction in the disassembly view, the
// call stack, the program's variables and each call's locals (writable),
// hover and watch expressions, and pausing. A PLC's own operations are
// custom requests, and commands in the debug console:
//
//	force NAME VALUE    beedance/force    keep VALUE on a variable or address
//	unforce NAME        beedance/unforce
//	forces              beedance/forces
//	set NAME VALUE      (setVariable)     write once
//	scan                beedance/stepScan run to the end of the scan
//	change              beedance/onlineChange  re-read the file, change online
//	live                beedance/live     the watched values, while running
//
// For a simulator's virtual field devices: beedance/io reads the I/O
// image, beedance/setInputs queues inputs a scan at a time, and, when
// launched with "scanEvents", a beedance/scan event brings the watched
// values and the I/O image while the program runs. beedance/speed sets how
// fast simulated time runs, beedance/trace records signals every scan, and
// the plant models (PROGRAM PLANT_...) the session runs after each scan
// are scopes of their own.
package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apiarytech/beedance/ide"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/token"
	"github.com/apiarytech/beedance/vmdebug"
)

// Server is a debug adapter on one stream: one debug session.
type Server struct {
	r   *bufio.Reader
	w   io.Writer
	wmu sync.Mutex
	seq int

	// Log, if set, receives every message, for debugging the adapter.
	Log io.Writer
	// YieldEvery is the sessions' vmdebug.Options.YieldEvery: set it when
	// the host is single-threaded (a web page), so requests are answered
	// while the program runs as fast as it goes.
	YieldEvery time.Duration
	// Yield is the sessions' vmdebug.Options.Yield.
	Yield func()
	// Attach, if set, answers the attach request: a host with programs
	// of its own (beehive's logic) gives the session attached to the one
	// asked for (attach.go). Without it, attach is refused.
	Attach Attacher

	ctx    context.Context
	cancel context.CancelFunc

	sess    *vmdebug.Session
	path    string // the program's file
	args    launchArgs
	pending []sourceBreakpoint // set before the launch
	bpIDs   int

	rmu       sync.Mutex
	running   bool
	cancelRun context.CancelFunc

	refs    map[int]varRef
	nextRef int

	lastScanEvent time.Time // when beedance/scan was last emitted

	// beedance/diagrams and beedance/monitor (diagrams.go, monitor.go):
	// the running source analyzed and lexed, and the names the session
	// samples for each.
	ideSource     string
	ideIndex      *ide.Index
	monitorSource string
	monitorToks   []token.Token
	sampled       map[string][]string
}

// NewServer is a debug adapter reading requests from r and writing
// responses and events to w.
func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{r: bufio.NewReader(r), w: w, refs: map[int]varRef{}}
}

// Serve answers requests until the editor disconnects, r ends or ctx is
// done.
func (s *Server) Serve(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()
	for {
		m, err := readMessage(s.r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if s.Log != nil {
			fmt.Fprintf(s.Log, "<- %s %s %s\n", m.Type, m.Command, m.Arguments)
		}
		if m.Type != "request" {
			continue
		}
		if s.handle(m) {
			return nil
		}
	}
}

func (s *Server) send(v any) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.seq++
	switch m := v.(type) {
	case *response:
		m.Seq = s.seq
	case *event:
		m.Seq = s.seq
	}
	if s.Log != nil {
		b, _ := json.Marshal(v)
		fmt.Fprintf(s.Log, "-> %s\n", b)
	}
	_ = writeMessage(s.w, v)
}

func (s *Server) reply(m *message, body any) {
	s.send(&response{Type: "response", RequestSeq: m.Seq, Success: true, Command: m.Command, Body: body})
}

func (s *Server) fail(m *message, err error) {
	s.send(&response{Type: "response", RequestSeq: m.Seq, Command: m.Command, Message: err.Error(),
		Body: map[string]any{"error": map[string]any{"id": 1, "format": err.Error(), "showUser": true}}})
}

func (s *Server) emit(name string, body any) {
	s.send(&event{Type: "event", Event: name, Body: body})
}

// output writes text to the editor's debug console.
func (s *Server) output(category, text string) {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	s.emit("output", map[string]any{"category": category, "output": text})
}

func (s *Server) isRunning() bool {
	s.rmu.Lock()
	defer s.rmu.Unlock()
	return s.running
}

var errNotLaunched = errors.New("no program is being debugged: launch one")
var errRunning = errors.New("the program is running: pause it first")

// handle answers m; it reports true when the session is over.
func (s *Server) handle(m *message) (done bool) {
	defer func() {
		if r := recover(); r != nil {
			s.fail(m, fmt.Errorf("internal error: %v", r))
		}
	}()
	args := func(v any) bool {
		if len(m.Arguments) == 0 {
			return true
		}
		if err := json.Unmarshal(m.Arguments, v); err != nil {
			s.fail(m, err)
			return false
		}
		return true
	}
	needs := func(stopped bool) bool {
		if s.sess == nil {
			s.fail(m, errNotLaunched)
			return false
		}
		if stopped && s.isRunning() {
			s.fail(m, errRunning)
			return false
		}
		return true
	}
	switch m.Command {
	case "initialize":
		s.reply(m, capabilities{
			SupportsConfigurationDoneRequest: true,
			SupportsConditionalBreakpoints:   true,
			SupportsEvaluateForHovers:        true,
			SupportsSetVariable:              true,
			SupportsTerminateRequest:         true,
			SupportsDisassembleRequest:       true,
			SupportsSteppingGranularity:      true,
		})
	case "launch":
		var a launchArgs
		if args(&a) {
			if err := s.launch(a); err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, nil)
			s.emit("initialized", nil)
		}
	case "attach":
		var a AttachArgs
		if args(&a) {
			if err := s.attach(a); err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, nil)
			s.emit("initialized", nil)
		}
	case "setBreakpoints":
		var a setBreakpointsArgs
		if args(&a) {
			s.reply(m, map[string]any{"breakpoints": s.setBreakpoints(a)})
		}
	case "setExceptionBreakpoints", "setFunctionBreakpoints":
		s.reply(m, map[string]any{"breakpoints": []breakpoint{}})
	case "configurationDone":
		if !needs(false) {
			return
		}
		s.reply(m, nil)
		switch {
		case s.args.StopOnEntry && !s.args.NoDebug && s.sess.Attached():
			s.sess.Pause() // it is running: stopped before its next scan
			s.start(s.sess.Continue)
		case s.args.StopOnEntry && !s.args.NoDebug:
			s.stopped(vmdebug.Stop{Reason: vmdebug.OnEntry})
		default:
			s.start(s.sess.Continue)
		}
	case "threads":
		name := "PROGRAM"
		if s.sess != nil {
			name = "PROGRAM " + s.name()
		}
		s.reply(m, map[string]any{"threads": []thread{{ID: 1, Name: name}}})
	case "stackTrace":
		if needs(true) {
			s.reply(m, s.stackTrace())
		}
	case "scopes":
		var a frameArgs
		if args(&a) && needs(true) {
			s.reply(m, map[string]any{"scopes": s.scopes(a.FrameID - 1)})
		}
	case "variables":
		var a variablesArgs
		if args(&a) && needs(true) {
			s.reply(m, map[string]any{"variables": s.variables(a.VariablesReference)})
		}
	case "setVariable":
		var a setVariableArgs
		if args(&a) && needs(true) {
			v, err := s.setVariable(a)
			if err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, map[string]any{"value": v})
		}
	case "continue":
		if needs(true) {
			s.reply(m, map[string]any{"allThreadsContinued": true})
			s.start(s.sess.Continue)
		}
	case "next", "stepIn", "stepOut":
		var a stepArgs
		if args(&a) && needs(true) {
			s.reply(m, nil)
			switch {
			case a.Granularity == "instruction" && m.Command != "stepOut":
				s.start(s.sess.StepInstruction)
			case m.Command == "next":
				s.start(s.sess.StepOver)
			case m.Command == "stepIn":
				s.start(s.sess.StepIn)
			default:
				s.start(s.sess.StepOut)
			}
		}
	case "pause":
		if needs(false) {
			s.sess.Pause()
			s.reply(m, nil)
		}
	case "evaluate":
		var a evaluateArgs
		if args(&a) && needs(false) {
			body, err := s.evaluate(a)
			if err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, body)
		}
	case "disassemble":
		var a disassembleArgs
		if args(&a) && needs(true) {
			body, err := s.disassemble(a)
			if err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, body)
		}
	case "beedance/force", "beedance/unforce":
		var a forceArgs
		if args(&a) && needs(false) {
			var err error
			if m.Command == "beedance/force" {
				err = s.sess.Force(a.Name, a.Value)
			} else {
				err = s.sess.Unforce(a.Name)
			}
			if err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, map[string]any{"forces": s.sess.Forced()})
			s.invalidate()
		}
	case "beedance/forces":
		if needs(false) {
			s.reply(m, map[string]any{"forces": s.sess.Forced()})
		}
	case "beedance/stepScan":
		if needs(true) {
			s.reply(m, nil)
			s.start(s.sess.StepScan)
		}
	case "beedance/onlineChange":
		var a onlineChangeArgs
		if args(&a) && needs(false) {
			if s.sess.Attached() {
				s.fail(m, errors.New("the program runs on its host: make the change there (beehive: deploy it, LogicService.Apply), which restarts the program and ends this session"))
				return
			}
			r, err := s.onlineChangeTo(a.Source)
			if err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, r)
		}
	case "beedance/live":
		if needs(false) {
			s.reply(m, s.sess.Live())
		}
	case "beedance/io":
		if needs(false) {
			s.reply(m, s.io())
		}
	case "beedance/speed":
		var a speedArgs
		if args(&a) && needs(false) {
			if s.sess.Attached() {
				s.fail(m, errors.New("the host's task paces a program attached to"))
				return
			}
			s.sess.SetSpeed(a.Factor)
			s.reply(m, map[string]any{"speed": s.sess.Speed()})
		}
	case "beedance/trace":
		var a traceArgs
		if args(&a) && needs(false) {
			if a.Signals != nil {
				if err := s.sess.SetTrace(*a.Signals, a.Depth); err != nil {
					s.fail(m, err)
					return
				}
			}
			s.reply(m, s.sess.TraceData(a.Scans))
		}
	case "beedance/monitor":
		var a monitorArgs
		if args(&a) && needs(false) {
			s.reply(m, s.monitor(a))
		}
	case "beedance/diagrams":
		var a diagramsArgs
		if args(&a) && needs(false) {
			s.reply(m, s.diagrams(a))
		}
	case "beedance/setInputs":
		var a setInputsArgs
		if args(&a) && needs(false) {
			if err := s.sess.SetInputs(a.Values); err != nil {
				s.fail(m, err)
				return
			}
			s.reply(m, map[string]any{"queued": s.sess.QueuedInputs()})
		}
	case "disconnect", "terminate":
		s.cancel()       // no stop is reported after this
		s.closeSession() // an attached program runs on without the debugger
		s.reply(m, nil)
		if m.Command == "terminate" {
			s.emit("terminated", nil)
			return false
		}
		return true
	default:
		s.fail(m, fmt.Errorf("%s is not supported", m.Command))
	}
	return false
}

func (s *Server) name() string { return s.sess.Program() }

// launch loads the program a names.
func (s *Server) launch(a launchArgs) error {
	if a.Program == "" {
		return errors.New(`launch: give "program", the .st file to debug`)
	}
	var path string
	var src []byte
	var err error
	if a.Source != "" {
		// No file: the path only names the program, and may not resolve
		// (a web page has no working directory).
		path, src = filepath.Clean(a.Program), []byte(a.Source)
	} else {
		if path, err = filepath.Abs(a.Program); err != nil {
			return err
		}
		if src, err = os.ReadFile(path); err != nil {
			return fmt.Errorf("launch: %w", err)
		}
	}
	engine, err := vmdebug.ParseEngine(a.Engine)
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	o := vmdebug.Options{Program: a.ProgramName, Engine: engine, Realtime: a.Realtime == nil || *a.Realtime,
		AllowInputWrites: a.AllowInputWrites, YieldEvery: s.YieldEvery, Yield: s.Yield,
		Notify:  func(msg string) { s.output("stderr", msg) },
		GoBuild: vmdebug.GoBuild{Replace: a.GoReplace}}
	if a.ScanEvents > 0 {
		every := time.Duration(a.ScanEvents) * time.Millisecond
		o.Sampled = func(smp vmdebug.Sample) {
			if now := time.Now(); now.Sub(s.lastScanEvent) >= every {
				s.lastScanEvent = now
				s.emit("beedance/scan", smp)
			}
		}
	}
	if a.Interval != "" {
		if o.Interval, err = time.ParseDuration(a.Interval); err != nil {
			return fmt.Errorf("launch: interval: %w", err)
		}
	}
	s.closeSession() // a session launched before this one
	sess, err := vmdebug.Load(string(src), o)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if a.Speed != nil {
		sess.SetSpeed(*a.Speed)
	}
	s.sess, s.path, s.args = sess, path, a
	sess.Live() // sample from the first scan on, for the live values
	if len(s.pending) > 0 {
		s.applyBreakpoints(s.pending)
	}
	clock := "a simulated clock"
	if o.Realtime {
		clock = "a simulated clock kept at the wall clock's pace"
	}
	interval := o.Interval
	if interval == 0 {
		interval = 10 * time.Millisecond
	}
	s.output("console", fmt.Sprintf("beedance: PROGRAM %s of %s on %s, a scan every %v on %s. "+
		"In this console: force NAME VALUE, unforce NAME, forces, scan, change, live, or an expression.",
		sess.Program(), filepath.Base(path), engineName(sess.Engine()), interval, clock))
	return nil
}

// engineName is an engine as the console names it.
func engineName(e vmdebug.Engine) string {
	switch e {
	case vmdebug.EngineEvaluator:
		return "the evaluator"
	case vmdebug.EngineGo:
		return "the transpiled program (Go)"
	}
	return "the VM"
}

// closeSession ends the session, if there is one, once its run has
// stopped; an attached program runs on.
func (s *Server) closeSession() {
	if s.sess == nil {
		return
	}
	s.detach()
	s.stopRun()
	s.sess.Close()
	s.sess = nil
}

// samePath reports whether a and b name one file.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (s *Server) setBreakpoints(a setBreakpointsArgs) []breakpoint {
	src := source{Name: filepath.Base(a.Source.Path), Path: a.Source.Path}
	if s.sess == nil {
		s.pending = a.Breakpoints
		var out []breakpoint
		for _, b := range a.Breakpoints {
			s.bpIDs++
			out = append(out, breakpoint{ID: s.bpIDs, Line: b.Line, Message: "the program is not loaded yet", Source: src})
		}
		return out
	}
	if !samePath(a.Source.Path, s.path) {
		out := []breakpoint{}
		for _, b := range a.Breakpoints {
			s.bpIDs++
			out = append(out, breakpoint{ID: s.bpIDs, Line: b.Line, Message: "not the program being debugged", Source: src})
		}
		return out
	}
	return s.applyBreakpoints(a.Breakpoints)
}

func (s *Server) applyBreakpoints(bps []sourceBreakpoint) []breakpoint {
	specs := make([]vmdebug.BreakpointSpec, len(bps))
	for i, b := range bps {
		specs[i] = vmdebug.BreakpointSpec{Line: b.Line, Condition: b.Condition}
	}
	if s.args.NoDebug {
		specs = nil
	}
	set := s.sess.SetBreakpoints(specs)
	out := make([]breakpoint, 0, len(bps))
	for i, b := range bps {
		s.bpIDs++
		bp := breakpoint{ID: s.bpIDs, Line: b.Line, Source: source{Name: filepath.Base(s.path), Path: s.path}}
		if i < len(set) {
			bp.Verified, bp.Message = set[i].Verified, set[i].Message
			if set[i].Verified {
				bp.Line = set[i].Actual
			}
		}
		out = append(out, bp)
	}
	return out
}

// start runs op in the background and reports where it stops.
func (s *Server) start(op func(context.Context) (vmdebug.Stop, error)) {
	s.refs, s.nextRef = map[int]varRef{}, 0
	ctx, cancel := context.WithCancel(s.ctx)
	s.rmu.Lock()
	s.running, s.cancelRun = true, cancel
	s.rmu.Unlock()
	go func() {
		st, err := op(ctx)
		s.rmu.Lock()
		s.running, s.cancelRun = false, nil
		s.rmu.Unlock()
		cancel()
		if s.ctx.Err() != nil {
			return // disconnected
		}
		if err != nil {
			s.output("stderr", err.Error())
		}
		s.stopped(st)
	}()
}

// stopRun stops a run in progress and waits for it.
func (s *Server) stopRun() {
	s.rmu.Lock()
	cancel := s.cancelRun
	s.rmu.Unlock()
	if cancel != nil {
		cancel()
	}
	for i := 0; i < 500 && s.isRunning(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
}

// io is beedance/io's reply: the I/O image, and the scan and time it is
// of. Running, it is as the last scan left it; stopped, as it is now, in
// the middle of a scan if the program stopped there.
func (s *Server) io() map[string]any {
	if s.isRunning() {
		smp := s.sess.Live()
		return map[string]any{"scan": smp.Scan, "elapsedMs": ms(smp.Elapsed), "io": smp.IO}
	}
	return map[string]any{"scan": s.sess.Scans(), "elapsedMs": ms(s.sess.Elapsed()), "io": s.sess.IO()}
}

// ms is d in milliseconds.
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// stopped tells the editor the program stopped, and why.
func (s *Server) stopped(st vmdebug.Stop) {
	if st.Reason == vmdebug.OnDetach {
		// The host let the program go (a stop, a restart, a detach): the
		// session is over, and the program runs on.
		s.output("console", "beedance: detached: the program runs on without the debugger")
		s.emit("terminated", nil)
		return
	}
	body := map[string]any{"threadId": 1, "allThreadsStopped": true}
	switch st.Reason {
	case vmdebug.OnScanEnd:
		body["reason"] = "step"
		body["description"] = fmt.Sprintf("End of scan %d", st.Scan)
	case vmdebug.OnException:
		body["reason"] = "exception"
		body["description"] = "Scan " + strconv.Itoa(st.Scan) + " failed"
		body["text"] = st.Message
		s.output("stderr", fmt.Sprintf("scan %d failed at line %d: %s", st.Scan, st.Line, st.Message))
	default:
		body["reason"] = string(st.Reason)
		if st.Scan > 0 {
			body["description"] = fmt.Sprintf("Paused on %s, scan %d", st.Reason, st.Scan)
		}
	}
	if st.Message != "" && st.Reason == vmdebug.OnBreakpoint {
		s.output("stderr", st.Message)
	}
	s.emit("stopped", body)
}

// invalidate asks the editor to read the variables again.
func (s *Server) invalidate() {
	if !s.isRunning() {
		s.refs, s.nextRef = map[int]varRef{}, 0
		s.emit("invalidated", map[string]any{"areas": []string{"variables"}})
	}
}

func (s *Server) stackTrace() map[string]any {
	src := &source{Name: filepath.Base(s.path), Path: s.path}
	var frames []stackFrame
	for i, f := range s.sess.Frames() {
		sf := stackFrame{ID: i + 1, Name: f.Name, Source: src, Line: f.Line, Column: max(f.Column, 1)}
		if f.Address != 0 {
			sf.InstructionPointerReference = fmt.Sprintf("0x%x", f.Address)
		}
		if f.Line == 0 {
			sf.Source = nil
		}
		frames = append(frames, sf)
	}
	return map[string]any{"stackFrames": frames, "totalFrames": len(frames)}
}

// varRef is what a variablesReference stands for.
type varRef struct {
	kind  int // refProgram, refLocals, refMembers, refForces
	frame int
	obj   object.Object // refMembers: the structure, instance or array
	path  string        // refMembers: its dotted name, if it is the program's
}

const (
	refProgram = iota
	refLocals
	refMembers
	refForces
	refPlant // frame: the plant model's index
)

func (s *Server) ref(r varRef) int {
	s.nextRef++
	s.refs[s.nextRef] = r
	return s.nextRef
}

func (s *Server) scopes(frame int) []scope {
	var out []scope
	if frame > 0 || len(s.sess.Locals(frame)) > 0 {
		out = append(out, scope{Name: "Locals", PresentationHint: "locals", VariablesReference: s.ref(varRef{kind: refLocals, frame: frame})})
	}
	out = append(out, scope{Name: "PROGRAM " + s.name(), VariablesReference: s.ref(varRef{kind: refProgram})})
	for i, name := range s.sess.Plants() {
		out = append(out, scope{Name: "PLANT " + name, VariablesReference: s.ref(varRef{kind: refPlant, frame: i})})
	}
	if len(s.sess.Forced()) > 0 {
		out = append(out, scope{Name: "Forced", VariablesReference: s.ref(varRef{kind: refForces})})
	}
	return out
}

func (s *Server) variables(ref int) []variable {
	r, ok := s.refs[ref]
	if !ok {
		return []variable{}
	}
	out := []variable{}
	add := func(v vmdebug.Value, path string) {
		dv := variable{Name: v.Name, Value: v.Text(), Type: v.Type, EvaluateName: path}
		if v.Address != "" {
			dv.Type += " AT " + v.Address
		}
		if v.Forced {
			dv.Value += "  (forced)"
		}
		if len(vmdebug.Members(v.Object)) > 0 {
			dv.VariablesReference = s.ref(varRef{kind: refMembers, obj: v.Object, path: path})
		}
		out = append(out, dv)
	}
	switch r.kind {
	case refProgram:
		for _, v := range s.sess.Globals() {
			add(v, v.Name)
		}
	case refLocals:
		for _, v := range s.sess.Locals(r.frame) {
			add(v, v.Name)
		}
	case refMembers:
		for _, v := range vmdebug.Members(r.obj) {
			path := ""
			if r.path != "" && !strings.HasPrefix(v.Name, "[") {
				path = r.path + "." + v.Name
			}
			add(v, path)
		}
	case refPlant:
		plant := s.sess.Plants()[r.frame]
		for _, v := range s.sess.PlantGlobals(r.frame) {
			path := ""
			if v.Address == "" {
				path = plant + "." + v.Name
			}
			add(v, path)
		}
	case refForces:
		forces := s.sess.Forced()
		names := make([]string, 0, len(forces))
		for n := range forces {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			out = append(out, variable{Name: n, Value: forces[n]})
		}
	}
	return out
}

func (s *Server) setVariable(a setVariableArgs) (string, error) {
	r, ok := s.refs[a.VariablesReference]
	if !ok {
		return "", errors.New("the variables have changed; look again")
	}
	switch r.kind {
	case refProgram:
		if err := s.sess.Set(a.Name, a.Value); err != nil {
			return "", err
		}
		for _, v := range s.sess.Globals() {
			if strings.EqualFold(v.Name, a.Name) {
				return v.Text(), nil
			}
		}
	case refLocals:
		o, err := s.sess.SetLocal(r.frame, a.Name, a.Value)
		if err != nil {
			return "", err
		}
		return vmdebug.Format(o), nil
	case refMembers:
		o, err := s.sess.SetIn(r.obj, a.Name, a.Value)
		if err != nil {
			return "", err
		}
		return vmdebug.Format(o), nil
	case refForces:
		if err := s.sess.Force(a.Name, a.Value); err != nil {
			return "", err
		}
		return a.Value, nil
	case refPlant:
		return "", errors.New("a plant model's variables are its own: change the plant's source, or force its inputs and outputs")
	}
	return a.Value, nil
}

// evaluate answers a hover, a watch or the debug console: an expression,
// or, in the console, one of the commands of the package comment.
func (s *Server) evaluate(a evaluateArgs) (map[string]any, error) {
	text := strings.TrimSpace(a.Expression)
	if a.Context == "repl" {
		if out, ok, err := s.command(text); ok {
			if err != nil {
				return nil, err
			}
			return map[string]any{"result": out, "variablesReference": 0}, nil
		}
	}
	if s.isRunning() {
		// Running: the values the last scan left.
		for _, v := range s.sess.Live().Values {
			if strings.EqualFold(v.Name, text) {
				return map[string]any{"result": v.Text, "type": v.Type, "variablesReference": 0}, nil
			}
		}
		return nil, errors.New("while it runs, only watched variables can be shown; pause to evaluate")
	}
	o, err := s.sess.Evaluate(text, max(a.FrameID-1, 0))
	if err != nil {
		return nil, err
	}
	body := map[string]any{"result": vmdebug.Format(o), "variablesReference": 0}
	if len(vmdebug.Members(o)) > 0 {
		body["variablesReference"] = s.ref(varRef{kind: refMembers, obj: o})
	}
	return body, nil
}

// command runs a debug console command; ok is false if text is not one.
func (s *Server) command(text string) (out string, ok bool, err error) {
	word, rest, _ := strings.Cut(text, " ")
	rest = strings.TrimSpace(rest)
	// NAME VALUE, or NAME := VALUE
	nameValue := func() (string, string, error) {
		name, value, found := strings.Cut(rest, ":=")
		if !found {
			name, value, found = strings.Cut(rest, " ")
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !found || name == "" || value == "" {
			return "", "", fmt.Errorf("%s NAME VALUE", word)
		}
		return name, strings.TrimSuffix(value, ";"), nil
	}
	switch strings.ToLower(word) {
	case "force":
		name, value, err := nameValue()
		if err == nil {
			err = s.sess.Force(name, value)
		}
		if err != nil {
			return "", true, err
		}
		s.invalidate()
		return fmt.Sprintf("%s forced to %s", name, value), true, nil
	case "unforce":
		if rest == "" {
			return "", true, errors.New("unforce NAME")
		}
		if err := s.sess.Unforce(rest); err != nil {
			return "", true, err
		}
		s.invalidate()
		return rest + " unforced", true, nil
	case "forces":
		forces := s.sess.Forced()
		if len(forces) == 0 {
			return "nothing is forced", true, nil
		}
		var lines []string
		for n, v := range forces {
			lines = append(lines, n+" = "+v)
		}
		sort.Strings(lines)
		return strings.Join(lines, "\n"), true, nil
	case "set":
		name, value, err := nameValue()
		if err == nil {
			err = s.sess.Set(name, value)
		}
		if err != nil {
			return "", true, err
		}
		s.invalidate()
		return fmt.Sprintf("%s set to %s", name, value), true, nil
	case "scan":
		if s.isRunning() {
			return "", true, errRunning
		}
		s.start(s.sess.StepScan)
		return "running to the end of the scan", true, nil
	case "change":
		r, err := s.onlineChange()
		if err != nil {
			return "", true, err
		}
		return summary(r), true, nil
	case "live":
		smp := s.sess.Live()
		lines := []string{fmt.Sprintf("scan %d, %v", smp.Scan, smp.Elapsed)}
		for _, v := range smp.Values {
			l := v.Name + " = " + v.Text
			if v.Forced {
				l += "  (forced)"
			}
			lines = append(lines, l)
		}
		return strings.Join(lines, "\n"), true, nil
	}
	return "", false, nil
}

// onlineChange reads the program's file again and changes the running
// program to it.
func (s *Server) onlineChange() (vmdebug.Report, error) { return s.onlineChangeTo("") }

// onlineChangeTo changes the running program to source, or, when source is
// empty, to its file as it is now. A program launched from its text (no
// file) can only be changed to a text given.
func (s *Server) onlineChangeTo(source string) (vmdebug.Report, error) {
	src := []byte(source)
	if source == "" {
		if s.args.Source != "" {
			return vmdebug.Report{}, errors.New(`online change: the program was launched from its text; give the new text ("source")`)
		}
		var err error
		if src, err = os.ReadFile(s.path); err != nil {
			return vmdebug.Report{}, err
		}
	}
	r, err := s.sess.Change(string(src))
	if errors.Is(err, vmdebug.ErrMidScan) {
		return r, errors.New("stopped inside a scan: run to its end (scan) to change the program online")
	}
	if err != nil {
		return r, err
	}
	s.output("console", "online change: "+summary(r))
	s.invalidate()
	return r, nil
}

// summary is r in a line.
func summary(r vmdebug.Report) string {
	parts := []string{fmt.Sprintf("%d variables kept", len(r.Kept))}
	for _, p := range []struct {
		what  string
		names []string
	}{{"reset", r.Reset}, {"added", r.Added}, {"removed", r.Removed}, {"unforced", r.Unforced}} {
		if len(p.names) > 0 {
			parts = append(parts, p.what+": "+strings.Join(p.names, ", "))
		}
	}
	return strings.Join(parts, "; ")
}

// disassemble answers the disassembly view: a.InstructionCount
// instructions from the one at a.MemoryReference, moved by
// a.InstructionOffset; past the code's ends, placeholders.
func (s *Server) disassemble(a disassembleArgs) (map[string]any, error) {
	ref, err := strconv.ParseInt(strings.TrimPrefix(strings.ToLower(a.MemoryReference), "0x"), 16, 64)
	if err != nil {
		return nil, fmt.Errorf("memory reference %q", a.MemoryReference)
	}
	code := s.sess.Disassembly()
	at := sort.Search(len(code), func(i int) bool { return code[i].Address >= int(ref) })
	src := &source{Name: filepath.Base(s.path), Path: s.path}
	out := make([]disassembledInstruction, 0, a.InstructionCount)
	lastPOU := ""
	for i := at + a.InstructionOffset; len(out) < a.InstructionCount; i++ {
		if i < 0 || i >= len(code) {
			// Beyond the code, one address per placeholder.
			addr := int(ref) + i - at
			switch {
			case len(code) > 0 && i < 0:
				addr = code[0].Address + i
			case len(code) > 0:
				addr = code[len(code)-1].Address + i - len(code) + 1
			}
			out = append(out, disassembledInstruction{Address: fmt.Sprintf("0x%x", max(addr, 0)), Instruction: "??"})
			continue
		}
		in := code[i]
		di := disassembledInstruction{Address: fmt.Sprintf("0x%x", in.Address), Instruction: in.Text}
		if in.POU != lastPOU {
			di.Symbol = in.POU
			lastPOU = in.POU
		}
		if in.Line > 0 {
			di.Location, di.Line = src, in.Line
		}
		out = append(out, di)
	}
	return map[string]any{"instructions": out}, nil
}
