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
// engines, they agree on every watched value of the test program after every
// scan (REAL and LREAL within a relative tolerance): its variables of
// elementary types and, under dotted names (b.Q), the outputs of its
// function block instances and the members of its structures (see package
// watch).
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
	"github.com/apiarytech/beedance/watch"
)

// Engine names a beedance backend a test runs on.
type Engine string

const (
	// Evaluator is the tree-walking evaluator.
	Evaluator Engine = "eval"
	// VM is the bytecode compiler and virtual machine.
	VM Engine = "vm"
	// Go is the transpiler: the source as Go on royaljelly, built with the
	// Go toolchain.
	Go Engine = "go"
)

// ParseEngines parses a comma-separated list of engines, e.g. "eval,vm";
// "all" is eval,vm,go.
func ParseEngines(s string) ([]Engine, error) {
	var out []Engine
	seen := map[Engine]bool{}
	var fields []string
	for _, f := range strings.Split(s, ",") {
		if strings.EqualFold(strings.TrimSpace(f), "all") {
			fields = append(fields, string(Evaluator), string(VM), string(Go))
		} else {
			fields = append(fields, f)
		}
	}
	for _, f := range fields {
		e := Engine(strings.ToLower(strings.TrimSpace(f)))
		switch e {
		case "":
			continue
		case Evaluator, VM, Go:
		case "evaluator":
			e = Evaluator
		case "transpiler":
			e = Go
		default:
			return nil, fmt.Errorf("unknown engine %q: want eval, vm, go or all", f)
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
	// Watch chooses the members of function block instances recorded and
	// compared; by default their outputs, two levels deep.
	Watch watch.Options
	// GoTimeout bounds one test on the go engine, which has no statement
	// budget; default one minute.
	GoTimeout time.Duration
	// GoReplace points modules the transpiled tests build with at local
	// directories, by module path, e.g. a royaljelly checkout.
	GoReplace map[string]string
	// IO, when set, connects the located variables (%I, %Q) of each run to
	// a plant model or to real I/O: hardware in the loop (see IO). The go
	// engine does not run with IO yet.
	IO IO
	// RealTime paces scans Interval apart on the wall clock, and timers
	// read it, instead of running them at once on a simulated clock. A run
	// against real I/O needs it; MaxScans × Interval then bounds a test.
	RealTime bool
	// Deterministic reports that IO gives every engine the same inputs for
	// the same outputs (a plant model in Go), so engines are compared as
	// they are without IO. With IO and without it, each engine's run is
	// judged on its own, as real inputs differ between runs.
	Deterministic bool
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
	if o.GoTimeout <= 0 {
		o.GoTimeout = defaultGoTimeout
	}
	return o
}

// Variable is a watched value of a test program, compared between engines
// and recorded after each scan: a variable of elementary type, or a member
// of an instance or structure under a dotted name, e.g. "b.Q".
type Variable = watch.Variable

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

	infos := make([]*testInfo, len(tests))
	for i, decl := range tests {
		infos[i] = newTestInfo(program, decl, opts)
	}
	var nat *native
	var natErr error
	for _, e := range opts.Engines {
		if e == Go && opts.IO == nil && nat == nil && natErr == nil {
			nat, natErr = buildNative(ctx, source, infos, opts)
			if nat != nil {
				defer nat.close()
			}
		}
	}

	var results []Result
	for _, t := range infos {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		results = append(results, runTest(ctx, source, program, t, nat, natErr, opts))
	}
	return results, ctx.Err()
}

// outputs are the declared names of a test's convention outputs; empty
// when not declared.
type outputs struct{ failures, message, done string }

// testInfo is what a run knows of a test before running it.
type testInfo struct {
	decl  *ast.ProgramDeclaration
	vars  []Variable
	out   outputs
	limit int // scans: 1, or MaxScans for a test with done
}

func newTestInfo(program *ast.Program, decl *ast.ProgramDeclaration, opts Options) *testInfo {
	t := &testInfo{decl: decl, vars: watch.Variables(program, decl, opts.Watch), limit: 1}
	for _, d := range decl.VarOutputs {
		if d.Name == nil {
			continue
		}
		switch strings.ToLower(d.Name.Value) {
		case "failures":
			t.out.failures = d.Name.Value
		case "message":
			t.out.message = d.Name.Value
		case "done":
			t.out.done = d.Name.Value
		}
	}
	if t.out.done != "" {
		t.limit = opts.MaxScans
	}
	return t
}

func runTest(ctx context.Context, source string, program *ast.Program, t *testInfo, nat *native, natErr error, opts Options) Result {
	res := Result{Name: t.decl.Name.Value, Variables: t.vars}
	for _, e := range opts.Engines {
		var r EngineResult
		switch {
		case t.out.failures == "":
			r = EngineResult{Err: "a test program needs VAR_OUTPUT failures : INT"}
		case e == Evaluator:
			r = runEvaluator(ctx, source, t.decl, t.vars, t.limit, opts)
		case e == VM:
			r = runVM(ctx, program, t.decl, t.vars, t.limit, opts)
		case opts.IO != nil:
			r = EngineResult{Err: "the go engine does not run with IO yet"}
		case natErr != nil:
			r = EngineResult{Err: natErr.Error()}
		default:
			r = nat.run(ctx, t, opts)
		}
		r.Engine = e
		r.outcome(t.out)
		res.Engines = append(res.Engines, r)
	}
	res.Passed = true
	for _, r := range res.Engines {
		res.Passed = res.Passed && r.Passed
	}
	// Each engine is compared with the first, unless real inputs make the
	// runs differ.
	compared := res.Engines[1:]
	if opts.IO != nil && !opts.Deterministic {
		compared = nil
	}
	for _, r := range compared {
		if res.Mismatch = compare(res.Engines[0], r, t.vars, opts.Tolerance); res.Mismatch != "" {
			break
		}
	}
	res.Passed = res.Passed && res.Mismatch == ""
	return res
}

func parse(source string) (*ast.Program, []string) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	return program, p.Errors()
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
	io, err := beginIO(ctx, opts.IO, decl.Name.Value, Evaluator, evaluator.IO(), evaluator.IOTypes())
	if err != nil {
		r.Err = err.Error()
		return r
	}
	defer io.end(ctx, &r)
	clk := newClock(opts)
	for n := 0; n < limit; n++ {
		if err := ctx.Err(); err != nil {
			r.Err = err.Error()
			break
		}
		t, err := clk.at(ctx, n)
		if err == nil {
			err = io.read(ctx, t)
		}
		if err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
		elapsed = t
		evaluator.SetBudget(opts.ScanBudget)
		if out := evaluator.Eval(call, env); isError(out) {
			r.Err = fmt.Sprintf("scan %d: %s", n+1, out.Inspect())
			break
		}
		values := Scan{}
		for _, v := range vars {
			o, ok := watch.Evaluator(prog.Env, v)
			// A located variable points into the I/O image.
			if p, isPtr := o.(*object.Pointer); ok && isPtr && p.Env == nil {
				o, ok = evaluator.IO()[p.Name]
			}
			if ok && o != nil {
				values[v.Name] = o.Inspect()
			}
		}
		io.record(values)
		r.Scans = append(r.Scans, values)
		if err := io.write(ctx, t); err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
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
	types := map[string]string{}
	located := map[string]string{} // address by upper-case name
	for _, v := range compiled.Variables {
		slot[strings.ToUpper(v.Name)] = v.Global
		if v.Address != "" {
			types[v.Address] = v.Type
			located[strings.ToUpper(v.Name)] = v.Address
		}
	}
	io, err := beginIO(ctx, opts.IO, decl.Name.Value, VM, scanVM.IO(), types)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	defer io.end(ctx, &r)
	clk := newClock(opts)
	for n := 0; n < limit; n++ {
		if err := ctx.Err(); err != nil {
			r.Err = err.Error()
			break
		}
		t, err := clk.at(ctx, n)
		if err == nil {
			err = io.read(ctx, t)
		}
		if err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
		elapsed = t
		scanVM.Reset()
		if err := scanVM.Run(); err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
		values := Scan{}
		for _, v := range vars {
			o, ok := watch.VM(globals, slot, v)
			if addr := located[strings.ToUpper(v.Name)]; !ok && addr != "" {
				o, ok = scanVM.IO()[addr] // a located variable has no slot
			}
			if ok && o != nil {
				values[v.Name] = o.Inspect()
			}
		}
		io.record(values)
		r.Scans = append(r.Scans, values)
		if err := io.write(ctx, t); err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
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
	case "BYTE", "WORD", "DWORD", "LWORD":
		bx, e1 := bits(x)
		by, e2 := bits(y)
		return e1 == nil && e2 == nil && bx == by
	}
	return strings.EqualFold(x, y)
}

// bits parses a bit string as an engine prints it: WORD#16#FF, 16#FF or
// 255.
func bits(s string) (uint64, error) {
	if i := strings.Index(s, "#"); i >= 0 && !strings.HasPrefix(s, "16#") {
		s = s[i+1:] // the type prefix
	}
	if strings.HasPrefix(s, "16#") {
		return strconv.ParseUint(strings.ReplaceAll(s[3:], "_", ""), 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}
