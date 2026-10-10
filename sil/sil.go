/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package sil runs Structured Text unit tests software-in-the-loop: the
// program under test runs scan by scan on a simulated clock, on the
// tree-walking evaluator, the bytecode VM or both, and the results come back
// to the caller to report, to fail a CI job, or to assert on in a Go test.
//
// A test is a PROGRAM whose name starts with TEST_ (case does not matter).
// It may use the function blocks, functions and types of the rest of the
// source. By convention, shared with beehive's engineering service, it has
//
//	VAR_OUTPUT failures : INT; END_VAR    (* checks that failed: 0 to pass *)
//	VAR_OUTPUT message : STRING; END_VAR  (* optional: what failed *)
//	VAR_OUTPUT done : BOOL; END_VAR       (* optional: run until TRUE *)
//
// A test without done runs one scan; with done it runs scan by scan,
// Interval of simulated time apart, until done is TRUE, so timers that take
// hours run in milliseconds. A test passes when, on every engine, it runs
// without error, is done and has failures = 0, and, when it runs on both
// engines, they agree on every elementary variable of the test program after
// every scan (REAL and LREAL within a relative tolerance).
package sil

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
)

// Engine names a beedance backend a test runs on.
type Engine string

const (
	// Evaluator is the tree-walking evaluator.
	Evaluator Engine = "eval"
	// VM is the bytecode compiler and virtual machine.
	VM Engine = "vm"
)

// ParseEngines parses a comma-separated list of engines, e.g. "eval,vm".
func ParseEngines(s string) ([]Engine, error) {
	var out []Engine
	seen := map[Engine]bool{}
	for _, f := range strings.Split(s, ",") {
		e := Engine(strings.ToLower(strings.TrimSpace(f)))
		switch e {
		case "":
			continue
		case Evaluator, VM:
		case "evaluator":
			e = Evaluator
		default:
			return nil, fmt.Errorf("unknown engine %q: want eval or vm", f)
		}
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no engines")
	}
	return out, nil
}

// Options control a run. The zero value runs every test on both engines
// with the defaults below.
type Options struct {
	// Interval is the simulated time between scans; default 10 ms.
	Interval time.Duration
	// MaxScans bounds a test with done: one not done by then fails;
	// default 50000.
	MaxScans int
	// Tolerance is the relative tolerance when comparing REAL and LREAL
	// values between engines; default 1e-3.
	Tolerance float64
	// Engines the tests run on; default both.
	Engines []Engine
	// Filter, when set, runs only tests whose name contains it (case does
	// not matter).
	Filter string
	// ScanBudget bounds the statements and loop iterations of one scan on
	// each engine, so a runaway loop fails its test instead of hanging the
	// run; default 10,000,000.
	ScanBudget int64
}

func (o Options) withDefaults() Options {
	if o.Interval <= 0 {
		o.Interval = 10 * time.Millisecond
	}
	if o.MaxScans <= 0 {
		o.MaxScans = 50000
	}
	if o.Tolerance <= 0 {
		o.Tolerance = 1e-3
	}
	if len(o.Engines) == 0 {
		o.Engines = []Engine{Evaluator, VM}
	}
	if o.ScanBudget <= 0 {
		o.ScanBudget = 10_000_000
	}
	return o
}

// Variable is an elementary variable of a test program, compared between
// engines and recorded after each scan.
type Variable struct {
	Name string
	Type string // upper case, e.g. "INT"
}

// Scan is the value of each Variable after one scan, by name, as the
// engine prints it.
type Scan map[string]string

// EngineResult is a test's run on one engine.
type EngineResult struct {
	Engine Engine
	Passed bool
	// Scans are the values after each scan run.
	Scans []Scan
	// Failures, Message and Done are the test's outputs after the last scan.
	Failures int
	Message  string
	Done     bool
	// Err is a runtime or compile error, or why the test could not run.
	Err string
}

// Problems lists why the engine's run did not pass; empty when it passed.
func (r EngineResult) Problems() []string {
	if r.Passed {
		return nil
	}
	var out []string
	if r.Err != "" {
		out = append(out, r.Err)
	}
	if r.Failures != 0 {
		out = append(out, fmt.Sprintf("%d failed checks, last: %s", r.Failures, r.Message))
	}
	if r.Err == "" && !r.Done {
		out = append(out, fmt.Sprintf("not done after %d scans", len(r.Scans)))
	}
	return out
}

// Result is one test program's outcome.
type Result struct {
	Name      string
	Variables []Variable
	// Engines holds a run per engine, in the order of Options.Engines.
	Engines []EngineResult
	// Mismatch is the first difference between the engines; empty when
	// they agree or only one engine ran.
	Mismatch string
	Passed   bool
}

// ParseError is returned when the source does not parse.
type ParseError struct{ Errors []string }

func (e *ParseError) Error() string {
	return "parser errors:\n\t" + strings.Join(e.Errors, "\n\t")
}

// ErrNoTests is returned when the source has no test programs (or none
// matching Options.Filter).
var ErrNoTests = errors.New("no test programs (PROGRAM TEST_...)")

var finalize sync.Once

// Run runs the tests in source. It returns a *ParseError if source does not
// parse and ErrNoTests if it has no tests; otherwise a Result per test, in
// source order. A cancelled ctx stops the run between scans and returns the
// results so far with ctx's error.
//
// The evaluator's clock is package-wide (evaluator.Session), so evaluator
// runs, here and elsewhere in the process, take turns.
func Run(ctx context.Context, source string, opts Options) ([]Result, error) {
	finalize.Do(object.FinalizeBuiltins)
	opts = opts.withDefaults()
	program, errs := parse(source)
	if len(errs) > 0 {
		return nil, &ParseError{Errors: errs}
	}
	filter := strings.ToUpper(opts.Filter)
	var tests []*ast.ProgramDeclaration
	for _, s := range program.Statements {
		pd, ok := s.(*ast.ProgramDeclaration)
		if !ok || pd.Name == nil {
			continue
		}
		name := strings.ToUpper(pd.Name.Value)
		if strings.HasPrefix(name, "TEST_") && strings.Contains(name, filter) {
			tests = append(tests, pd)
		}
	}
	if len(tests) == 0 {
		return nil, ErrNoTests
	}

	var results []Result
	for _, decl := range tests {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		results = append(results, runTest(ctx, source, program, decl, opts))
	}
	return results, ctx.Err()
}

// outputs are the declared names of a test's convention outputs; empty
// when not declared.
type outputs struct{ failures, message, done string }

func runTest(ctx context.Context, source string, program *ast.Program, decl *ast.ProgramDeclaration, opts Options) Result {
	res := Result{Name: decl.Name.Value, Variables: elementary(decl)}
	var out outputs
	for _, d := range decl.VarOutputs {
		if d.Name == nil {
			continue
		}
		switch strings.ToLower(d.Name.Value) {
		case "failures":
			out.failures = d.Name.Value
		case "message":
			out.message = d.Name.Value
		case "done":
			out.done = d.Name.Value
		}
	}
	limit := 1
	if out.done != "" {
		limit = opts.MaxScans
	}
	for _, e := range opts.Engines {
		var r EngineResult
		if out.failures == "" {
			r = EngineResult{Err: "a test program needs VAR_OUTPUT failures : INT"}
		} else if e == Evaluator {
			r = runEvaluator(ctx, source, decl, res.Variables, limit, opts)
		} else {
			r = runVM(ctx, program, decl, res.Variables, limit, opts)
		}
		r.Engine = e
		r.outcome(out)
		res.Engines = append(res.Engines, r)
	}
	res.Passed = true
	for _, r := range res.Engines {
		res.Passed = res.Passed && r.Passed
	}
	if len(res.Engines) == 2 {
		res.Mismatch = compare(res.Engines[0], res.Engines[1], res.Variables, opts.Tolerance)
		res.Passed = res.Passed && res.Mismatch == ""
	}
	return res
}

func parse(source string) (*ast.Program, []string) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	return program, p.Errors()
}

// elementary lists the test program's variables of elementary types.
func elementary(decl *ast.ProgramDeclaration) []Variable {
	var out []Variable
	for _, group := range [][]*ast.VarDeclStatement{decl.VarInputs, decl.VarOutputs, decl.VarInOuts, decl.Vars} {
		for _, d := range group {
			if d.Name == nil || d.DataType == nil {
				continue
			}
			t := strings.ToUpper(d.DataType.String())
			switch t {
			case "BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT",
				"REAL", "LREAL", "BYTE", "WORD", "DWORD", "LWORD", "STRING", "TIME":
				out = append(out, Variable{d.Name.Value, t})
			}
		}
	}
	return out
}

// outcome fills Failures, Message, Done and Passed from the last scan.
func (r *EngineResult) outcome(out outputs) {
	if r.Err != "" || len(r.Scans) == 0 {
		r.Done = false
		r.Passed = false
		return
	}
	last := r.Scans[len(r.Scans)-1]
	r.Failures, _ = strconv.Atoi(last[out.failures])
	if out.message != "" {
		r.Message = strings.Trim(last[out.message], `'"`)
	}
	r.Done = out.done == "" || strings.EqualFold(last[out.done], "true")
	r.Passed = r.Failures == 0 && r.Done
}

func isDone(values Scan) bool {
	for k, v := range values {
		if strings.EqualFold(k, "done") && strings.EqualFold(v, "true") {
			return true
		}
	}
	return false
}

// runEvaluator runs a test on the tree-walking evaluator in a session on a
// simulated clock.
func runEvaluator(ctx context.Context, source string, decl *ast.ProgramDeclaration, vars []Variable, limit int, opts Options) (r EngineResult) {
	defer func() {
		if p := recover(); p != nil {
			r.Err = fmt.Sprintf("evaluator panic: %v", p)
		}
	}()
	program, _ := parse(source) // a fresh AST: the evaluator keeps state in it
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var elapsed time.Duration
	end := evaluator.Session(func() time.Time { return start.Add(elapsed) })
	defer end()

	env := object.NewEnvironment()
	evaluator.SetBudget(opts.ScanBudget)
	if out := evaluator.Eval(program, env); isError(out) {
		r.Err = out.Inspect()
		return r
	}
	obj, ok := env.Get(decl.Name.Value)
	prog, isProg := obj.(*object.Program)
	if !ok || !isProg {
		r.Err = "program not declared"
		return r
	}
	call, _ := parse(decl.Name.Value + "();")
	for n := 0; n < limit; n++ {
		if err := ctx.Err(); err != nil {
			r.Err = err.Error()
			break
		}
		elapsed = time.Duration(n) * opts.Interval
		evaluator.SetBudget(opts.ScanBudget)
		if out := evaluator.Eval(call, env); isError(out) {
			r.Err = fmt.Sprintf("scan %d: %s", n+1, out.Inspect())
			break
		}
		values := Scan{}
		for _, v := range vars {
			if o, ok := prog.Env.Get(v.Name); ok && o != nil {
				values[v.Name] = o.Inspect()
			}
		}
		r.Scans = append(r.Scans, values)
		if isDone(values) {
			break
		}
	}
	return r
}

// runVM compiles a test to bytecode and runs it on the VM, with the
// standard timers on a simulated clock.
func runVM(ctx context.Context, program *ast.Program, decl *ast.ProgramDeclaration, vars []Variable, limit int, opts Options) (r EngineResult) {
	defer func() {
		if p := recover(); p != nil {
			r.Err = fmt.Sprintf("vm panic: %v", p)
		}
	}()
	compiled, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, decl.Name.Value)
	if err != nil {
		r.Err = "compile: " + err.Error()
		return r
	}
	globals := make([]object.Object, vm.GlobalsSize)
	initVM := vm.NewWithGlobalsStore(compiled.InitBytecode, globals)
	scanVM := vm.NewWithGlobalsStore(compiled.CyclicBytecode, globals)
	var elapsed time.Duration
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return elapsed }) {
		initVM.SetBuiltin(idx, b)
		scanVM.SetBuiltin(idx, b)
	}
	initVM.SetBudget(opts.ScanBudget)
	scanVM.SetBudget(opts.ScanBudget)
	if err := initVM.Run(); err != nil {
		r.Err = "init: " + err.Error()
		return r
	}
	for addr, v := range initVM.IO() {
		scanVM.IO()[addr] = v
	}
	slot := map[string]int{}
	for _, v := range compiled.Variables {
		slot[strings.ToUpper(v.Name)] = v.Global
	}
	for n := 0; n < limit; n++ {
		if err := ctx.Err(); err != nil {
			r.Err = err.Error()
			break
		}
		elapsed = time.Duration(n) * opts.Interval
		scanVM.Reset()
		if err := scanVM.Run(); err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
		values := Scan{}
		for _, v := range vars {
			if g, ok := slot[strings.ToUpper(v.Name)]; ok && g >= 0 && globals[g] != nil {
				values[v.Name] = globals[g].Inspect()
			}
		}
		r.Scans = append(r.Scans, values)
		if isDone(values) {
			break
		}
	}
	return r
}

func isError(o object.Object) bool { return o != nil && o.Type() == object.ERROR_OBJ }

// compare reports the first scan and variable where two engines disagree.
func compare(a, b EngineResult, vars []Variable, tol float64) string {
	if a.Err != "" || b.Err != "" {
		return ""
	}
	if len(a.Scans) != len(b.Scans) {
		return fmt.Sprintf("%s ran %d scans, %s %d", a.Engine, len(a.Scans), b.Engine, len(b.Scans))
	}
	for i := range a.Scans {
		for _, v := range vars {
			x, y := a.Scans[i][v.Name], b.Scans[i][v.Name]
			if !same(x, y, v.Type, tol) {
				return fmt.Sprintf("scan %d %s: %s %s, %s %s", i+1, v.Name, a.Engine, x, b.Engine, y)
			}
		}
	}
	return ""
}

func same(x, y, typ string, tol float64) bool {
	if x == y {
		return true
	}
	switch typ {
	case "REAL", "LREAL":
		fx, e1 := strconv.ParseFloat(x, 64)
		fy, e2 := strconv.ParseFloat(y, 64)
		if e1 != nil || e2 != nil {
			return false
		}
		return math.Abs(fx-fy) <= tol*math.Max(1, math.Max(math.Abs(fx), math.Abs(fy)))
	case "STRING":
		return strings.Trim(x, `'"`) == strings.Trim(y, `'"`)
	}
	return strings.EqualFold(x, y)
}
