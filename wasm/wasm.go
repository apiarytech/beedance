/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package wasm is beedance's toolchain behind one JSON call, for hosts that
// load it as a WebAssembly module: a web page (GOOS=js) or a WASI runtime
// (GOOS=wasip1) running software-in-the-loop tests. Nothing in it depends
// on the platform, so it is tested, and can be used, natively too.
//
// A request is a JSON object naming an op; the reply is a JSON object, with
// "error" set when the op failed:
//
//	{"op":"version"}
//	{"op":"check","source":"..."}                    parse; list programs and tests
//	{"op":"test","source":"...","engines":["eval","vm"],"filter":"",
//	 "intervalMs":10,"maxScans":50000,"tolerance":1e-3,"scans":false}
//	{"op":"transpile","source":"..."}                Go source on royaljelly
//	{"op":"load","source":"...","program":"Main","intervalMs":10}
//	{"op":"scan","session":1,"count":1,"values":{"start":true,"%IX0.0":true}}
//	{"op":"set","session":1,"values":{...}}
//	{"op":"read","session":1}
//	{"op":"unload","session":1}
//
// test runs the TEST_ programs of the source (see package sil) on the
// evaluator and the VM and compares them. load compiles a PROGRAM to
// bytecode and starts it on the VM, on a simulated clock that each scan
// advances by the interval: a session the host drives scan by scan, setting
// inputs by variable name or address and reading the watched values back.
package wasm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/sil"
	"github.com/apiarytech/beedance/transpiler"
)

// Request is one call. Only the fields its op uses are read.
type Request struct {
	Op      string `json:"op"`
	Source  string `json:"source,omitempty"`
	Program string `json:"program,omitempty"`
	Session int    `json:"session,omitempty"`
	// test
	Engines   []string `json:"engines,omitempty"`
	Filter    string   `json:"filter,omitempty"`
	MaxScans  int      `json:"maxScans,omitempty"`
	Tolerance float64  `json:"tolerance,omitempty"`
	// Scans includes each engine's values after every scan in a test's
	// result.
	Scans bool `json:"scans,omitempty"`
	// IntervalMs is the simulated time between scans; default 10.
	IntervalMs float64 `json:"intervalMs,omitempty"`
	// scan: how many scans to run (default 1); values, set before them.
	Count  int            `json:"count,omitempty"`
	Values map[string]any `json:"values,omitempty"`
}

// Host answers requests. It holds the sessions load starts; it is safe for
// concurrent use.
type Host struct {
	mu       sync.Mutex
	sessions map[int]*session
	next     int
}

// New returns a Host with no sessions.
func New() *Host {
	return &Host{sessions: map[int]*session{}}
}

// errorReply is the reply of a failed op. Errors lists parse errors.
type errorReply struct {
	Error  string   `json:"error"`
	Errors []string `json:"errors,omitempty"`
}

// Handle answers a JSON request with a JSON reply. It never fails: a bad
// request gets a reply with "error" set.
func (h *Host) Handle(request []byte) []byte {
	var req Request
	var reply any
	if err := json.Unmarshal(request, &req); err != nil {
		reply = errorReply{Error: "bad request: " + err.Error()}
	} else if r, err := h.Do(req); err != nil {
		e := errorReply{Error: err.Error()}
		var pe *sil.ParseError
		if errors.As(err, &pe) {
			e.Error, e.Errors = "parse errors", pe.Errors
		}
		reply = e
	} else {
		reply = r
	}
	out, err := json.Marshal(reply)
	if err != nil {
		out, _ = json.Marshal(errorReply{Error: "reply: " + err.Error()})
	}
	return out
}

// Do answers a request with a reply to encode as JSON.
func (h *Host) Do(req Request) (any, error) {
	switch strings.ToLower(req.Op) {
	case "version":
		return versionReply{GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Ops: ops}, nil
	case "check":
		return check(req.Source), nil
	case "test":
		return test(req)
	case "transpile":
		program, err := parse(req.Source)
		if err != nil {
			return nil, err
		}
		src, err := transpiler.GoFile(program)
		if err != nil {
			return nil, err
		}
		return transpileReply{Go: string(src)}, nil
	case "load":
		return h.load(req)
	case "scan", "set", "read", "unload":
		return h.withSession(req)
	case "":
		return nil, errors.New("no op")
	}
	return nil, fmt.Errorf("unknown op %q: want one of %s", req.Op, strings.Join(ops, ", "))
}

var ops = []string{"version", "check", "test", "transpile", "load", "scan", "set", "read", "unload"}

type versionReply struct {
	GoVersion string   `json:"goVersion"`
	GOOS      string   `json:"goos"`
	GOARCH    string   `json:"goarch"`
	Ops       []string `json:"ops"`
}

type transpileReply struct {
	Go string `json:"go"`
}

type checkReply struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
	// Programs are the PROGRAMs declared, tests among them.
	Programs []string `json:"programs"`
	Tests    []string `json:"tests"`
}

func check(source string) checkReply {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	r := checkReply{OK: len(p.Errors()) == 0, Errors: p.Errors(), Programs: []string{}, Tests: []string{}}
	for _, pd := range programs(program) {
		name := pd.Name.Value
		r.Programs = append(r.Programs, name)
		if isTest(name) {
			r.Tests = append(r.Tests, name)
		}
	}
	return r
}

func parse(source string) (*ast.Program, error) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, &sil.ParseError{Errors: errs}
	}
	return program, nil
}

func programs(program *ast.Program) []*ast.ProgramDeclaration {
	var out []*ast.ProgramDeclaration
	for _, s := range program.Statements {
		if pd, ok := s.(*ast.ProgramDeclaration); ok && pd.Name != nil {
			out = append(out, pd)
		}
	}
	return out
}

func isTest(name string) bool { return strings.HasPrefix(strings.ToUpper(name), "TEST_") }

func interval(ms float64) time.Duration {
	if ms <= 0 {
		return 10 * time.Millisecond
	}
	return time.Duration(ms * float64(time.Millisecond))
}

type testReply struct {
	Passed  bool         `json:"passed"`
	Failed  int          `json:"failed"`
	Results []testResult `json:"results"`
}

type testResult struct {
	Name      string         `json:"name"`
	Passed    bool           `json:"passed"`
	Mismatch  string         `json:"mismatch,omitempty"`
	Variables []variable     `json:"variables"`
	Engines   []engineResult `json:"engines"`
}

type variable struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Block   string `json:"block,omitempty"`
	Address string `json:"address,omitempty"`
}

type engineResult struct {
	Engine    string     `json:"engine"`
	Passed    bool       `json:"passed"`
	Done      bool       `json:"done"`
	Failures  int        `json:"failures"`
	Message   string     `json:"message,omitempty"`
	Error     string     `json:"error,omitempty"`
	Problems  []string   `json:"problems,omitempty"`
	ScanCount int        `json:"scanCount"`
	Scans     []sil.Scan `json:"scans,omitempty"`
}

// test runs the TEST_ programs of the source with package sil. The go and
// wasm engines build the tests with the Go toolchain, which a WebAssembly
// module has not got.
func test(req Request) (testReply, error) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if len(req.Engines) > 0 {
		var err error
		if engines, err = sil.ParseEngines(strings.Join(req.Engines, ",")); err != nil {
			return testReply{}, err
		}
	}
	for _, e := range engines {
		if (e == sil.Go || e == sil.Wasm) && runtime.GOARCH == "wasm" {
			return testReply{}, fmt.Errorf("the %s engine needs the Go toolchain, which WebAssembly has not got: use eval and vm", e)
		}
	}
	results, err := sil.Run(context.Background(), req.Source, sil.Options{
		Interval:  interval(req.IntervalMs),
		MaxScans:  req.MaxScans,
		Tolerance: req.Tolerance,
		Engines:   engines,
		Filter:    req.Filter,
	})
	if err != nil {
		return testReply{}, err
	}
	reply := testReply{Passed: true, Results: []testResult{}}
	for _, res := range results {
		tr := testResult{Name: res.Name, Passed: res.Passed, Mismatch: res.Mismatch, Variables: []variable{}}
		for _, v := range res.Variables {
			tr.Variables = append(tr.Variables, variable{Name: v.Name, Type: v.Type})
		}
		for _, r := range res.Engines {
			er := engineResult{Engine: string(r.Engine), Passed: r.Passed, Done: r.Done, Failures: r.Failures,
				Message: r.Message, Error: r.Err, Problems: r.Problems(), ScanCount: len(r.Scans)}
			if req.Scans {
				er.Scans = r.Scans
			}
			tr.Engines = append(tr.Engines, er)
		}
		if !res.Passed {
			reply.Passed = false
			reply.Failed++
		}
		reply.Results = append(reply.Results, tr)
	}
	return reply, nil
}

// jsonValue is o as JSON gives it to the host: a bool, number or string,
// TIME as written in Structured Text (T#1s500ms), and anything else as the
// engine prints it.
func jsonValue(o object.Object) any {
	v, err := sil.FromObject(o)
	if err != nil {
		return o.Inspect()
	}
	if _, isTime := v.(time.Duration); isTime {
		return o.Inspect()
	}
	return v
}
