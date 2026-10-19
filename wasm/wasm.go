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
//	{"op":"exportXML","source":"...","name":"...","sim":"{...}","from":"<xml>"}
//	                                                 PLCopen XML (IEC 61131-10)
//	{"op":"importXML","xml":"..."}                   its source, name and sim data
//	{"op":"lint","source":"..."}                     the analyzer's findings
//	{"op":"hover","source":"...","line":3,"column":5} also complete, definition,
//	                                                 references, rename ("newName")
//	{"op":"diagrams","source":"..."}                 LD, FBD and SFC bodies as SVG
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
	// exportXML: the project's name, the simulator's settings (JSON) and
	// the XML it was imported from; importXML: the XML.
	Name string `json:"name,omitempty"`
	Sim  string `json:"sim,omitempty"`
	From string `json:"from,omitempty"`
	XML  string `json:"xml,omitempty"`
	// hover, complete, definition, references, rename: a place in the
	// source, 1-based; rename's new name.
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	NewName string `json:"newName,omitempty"`
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
		var pe *ParseError
		if errs, ok := parseErrors(err); ok {
			e.Error, e.Errors = "parse errors", errs
		} else if errors.As(err, &pe) {
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
	if r, ok, err := ideOp(req); ok {
		return r, err
	}
	switch strings.ToLower(req.Op) {
	case "version":
		return versionReply{GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Ops: ops}, nil
	case "check":
		return check(req.Source), nil
	case "test":
		return testOp(req)
	case "transpile":
		return transpileOp(req)
	case "exportxml":
		return exportXML(req)
	case "importxml":
		return importXML(req)
	case "load":
		return h.load(req)
	case "scan", "set", "read", "unload":
		return h.withSession(req)
	case "":
		return nil, errors.New("no op")
	}
	return nil, fmt.Errorf("unknown op %q: want one of %s", req.Op, strings.Join(ops, ", "))
}

// ops are the ops this build answers: these, and the full build's
// (full.go; a build with -tags lite leaves them out).
var ops = append([]string{"version", "check", "exportXML", "importXML", "lint", "hover", "complete",
	"definition", "references", "rename", "outline", "diagrams", "load", "scan", "set", "read", "unload"}, buildOps...)

type versionReply struct {
	GoVersion string   `json:"goVersion"`
	GOOS      string   `json:"goos"`
	GOARCH    string   `json:"goarch"`
	Ops       []string `json:"ops"`
}

type checkReply struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
	// Programs are the PROGRAMs declared, tests among them.
	Programs []string `json:"programs"`
	Tests    []string `json:"tests"`
	// POUs are the source's declarations, in order: an editor's outline
	// and the scopes of a search.
	POUs []pouInfo `json:"pous"`
	// Located are the PROGRAMs' variables with an address (AT %IX0.0).
	Located []locatedInfo `json:"located"`
}

// pouInfo is a declaration of the source: Kind is PROGRAM,
// FUNCTION_BLOCK, FUNCTION, INTERFACE, TYPE, VAR_GLOBAL or
// CONFIGURATION; Line is where it starts.
type pouInfo struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Line int    `json:"line"`
}

// locatedInfo is a PROGRAM's variable at an address.
type locatedInfo struct {
	Program string `json:"program"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Type    string `json:"type"`
	Line    int    `json:"line"`
}

func check(source string) checkReply {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	r := checkReply{OK: len(p.Errors()) == 0, Errors: p.Errors(), Programs: []string{}, Tests: []string{},
		POUs: outline(program), Located: []locatedInfo{}}
	for _, pd := range programs(program) {
		name := pd.Name.Value
		r.Programs = append(r.Programs, name)
		if isTest(name) {
			r.Tests = append(r.Tests, name)
		}
		for _, list := range [][]*ast.VarDeclStatement{pd.VarInputs, pd.VarOutputs, pd.VarInOuts, pd.Vars} {
			for _, v := range list {
				if v == nil || v.Name == nil || v.Location == nil || v.Location.Location == nil {
					continue
				}
				addr := strings.ToUpper(v.Location.Location.Address)
				if !strings.HasPrefix(addr, "%") {
					addr = "%" + addr
				}
				li := locatedInfo{Program: name, Name: v.Name.Value, Address: addr}
				if v.DataType != nil {
					li.Type = v.DataType.String()
				}
				li.Line, _ = v.Location.Pos()
				r.Located = append(r.Located, li)
			}
		}
	}
	return r
}

// outline lists the source's top-level declarations.
func outline(program *ast.Program) []pouInfo {
	out := []pouInfo{}
	add := func(kind string, name *ast.Identifier, line int) {
		if line <= 0 {
			return
		}
		n := ""
		if name != nil {
			n = name.Value
		}
		out = append(out, pouInfo{Kind: kind, Name: n, Line: line})
	}
	for _, st := range program.Statements {
		row, _ := st.Pos()
		switch d := st.(type) {
		case *ast.ProgramDeclaration:
			add("PROGRAM", d.Name, row)
		case *ast.FunctionBlockDeclaration:
			add("FUNCTION_BLOCK", d.Name, row)
		case *ast.FunctionDeclaration:
			add("FUNCTION", d.Name, row)
		case *ast.InterfaceDeclaration:
			add("INTERFACE", d.Name, row)
		case *ast.TypeBlockDeclaration:
			var name *ast.Identifier
			if len(d.Declarations) > 0 {
				name = d.Declarations[0].Name
			}
			add("TYPE", name, row)
		case *ast.TypeDeclaration:
			add("TYPE", d.Name, row)
		case *ast.GlobalVarDeclaration:
			add("VAR_GLOBAL", nil, row)
		case *ast.ConfigurationDeclaration:
			add("CONFIGURATION", d.Name, row)
		}
	}
	return out
}

// ParseError is the parse errors of a source, as sil.ParseError (which
// the lite build has not got).
type ParseError struct{ Errors []string }

func (e *ParseError) Error() string {
	return "parser errors:\n\t" + strings.Join(e.Errors, "\n\t")
}

func parse(source string) (*ast.Program, error) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, &ParseError{Errors: errs}
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

type variable struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Block   string `json:"block,omitempty"`
	Address string `json:"address,omitempty"`
}

// fromObject is an elementary value as Go holds it (as sil.FromObject,
// which the lite build has not got).
func fromObject(o object.Object) (any, error) {
	switch x := o.(type) {
	case *object.Boolean:
		return x.Value, nil
	case *object.Real:
		return x.Value, nil
	case *object.LReal:
		return x.Value, nil
	case *object.String:
		return x.Value, nil
	case *object.WString:
		return x.Value, nil
	case *object.Time:
		return x.Value, nil
	case *object.Byte:
		return uint64(x.Value), nil
	case *object.Word:
		return uint64(x.Value), nil
	case *object.DWord:
		return uint64(x.Value), nil
	case *object.LWord:
		return x.Value, nil
	case *object.BitString:
		return x.Value, nil
	case *object.ULInt:
		return x.Value, nil
	}
	if n, unsigned, ok := object.GetIntegerObjectValue(o); ok {
		if unsigned {
			return uint64(n), nil
		}
		return n, nil
	}
	if o == nil {
		return nil, fmt.Errorf("no value")
	}
	return nil, fmt.Errorf("%s (%T) is not an elementary value", o.Inspect(), o)
}

// jsonValue is o as JSON gives it to the host: a bool, number or string,
// TIME as written in Structured Text (T#1s500ms), and anything else as the
// engine prints it.
func jsonValue(o object.Object) any {
	v, err := fromObject(o)
	if err != nil {
		return o.Inspect()
	}
	if _, isTime := v.(time.Duration); isTime {
		return o.Inspect()
	}
	return v
}
