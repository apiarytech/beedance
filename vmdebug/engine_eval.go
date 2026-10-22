/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

// The evaluator engine: the program run by the tree-walking evaluator, in
// a session of its own (evaluator.Session: the clock and the I/O image),
// watched through evaluator.Debug. The evaluator does not stop: a scan
// runs in a goroutine of its own, which parks in the statement hook while
// the program is stopped, and Run returns errPaused meanwhile.
//
// The evaluator's session is the package's: one at a time. The Session
// holds it from its first evaluator engine until Close, so an online
// change builds the new program in the same session, on the same I/O
// image.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// evalEpoch is the evaluator's clock at the session's start.
var evalEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// ErrEvaluatorBusy is returned by Load on the evaluator engine while
// another session holds the evaluator (another debug session on it, or a
// test run): the evaluator's session is the package's, one at a time.
var ErrEvaluatorBusy = errors.New("vmdebug: the evaluator is in use by another session; end it first")

// evalSession makes sure the session holds the evaluator's session; the
// evaluator's clock is the session's. Rather than wait for another
// session to end, which a single-threaded host could not survive, it
// fails with ErrEvaluatorBusy.
func (s *Session) evalSession() error {
	if s.evalEnd == nil {
		if evaluator.Busy() {
			return ErrEvaluatorBusy
		}
		s.evalEnd = evaluator.Session(func() time.Time { return evalEpoch.Add(s.elapsed) })
	}
	return nil
}

// evalEngine is a PROGRAM on the evaluator.
type evalEngine struct {
	name    string
	userPOU map[string]bool
	decls   []Variable
	located map[string]string // address by upper-case name
	env     *object.Environment
	progEnv *object.Environment
	call    *ast.Program // "Name();"
	rows    map[int]bool
	first   int
	budget  int64

	hook    func(site) bool
	running bool // a scan's goroutine is alive: running or parked in the hook
	paused  chan struct{}
	resume  chan struct{}
	done    chan error
	stack   []evalFrame
	failed  []evalFrame // the stack where the scan failed
}

// evalFrame is a POU the evaluator is in.
type evalFrame struct {
	name        string
	env         *object.Environment
	levels      int
	row, column int
	user        bool // one of the source's own POUs
}

// newEvalEngine evaluates img's source in the evaluator's session (which
// acquire makes sure of) and finds its PROGRAM.
func newEvalEngine(img *image, o Options, acquire func() error) (*evalEngine, error) {
	if err := acquire(); err != nil {
		return nil, err
	}
	// A fresh syntax tree: the evaluator rewrites the one it is given,
	// which the session reads the source's places from.
	program, errs := parseSource(img.source)
	if len(errs) > 0 {
		return nil, fmt.Errorf("parse: %s", strings.Join(errs, "; "))
	}
	evaluator.SetAllowInputWrites(o.AllowInputWrites)
	evaluator.SetBudget(o.Budget)
	env := object.NewEnvironment()
	if out := evaluator.Eval(program, env); isEvalError(out) {
		return nil, fmt.Errorf("init: %s", out.Inspect())
	}
	obj, ok := env.Get(img.name)
	prog, isProg := obj.(*object.Program)
	if !ok || !isProg {
		return nil, fmt.Errorf("init: PROGRAM %s was not declared", img.name)
	}
	call, errs := parseSource(img.name + "();")
	if len(errs) > 0 {
		return nil, fmt.Errorf("init: %s", strings.Join(errs, "; "))
	}
	e := &evalEngine{name: img.name, userPOU: img.userPOU, env: env, progEnv: prog.Env, call: call, budget: o.Budget,
		located: map[string]string{}, paused: make(chan struct{}), resume: make(chan struct{}), done: make(chan error, 1)}
	e.decls, e.located = declVariables(img.decl)
	e.rows, e.first = sourceRows(img)
	return e, nil
}

// parseSource parses source.
func parseSource(source string) (*ast.Program, []string) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	return program, p.Errors()
}

func isEvalError(o object.Object) bool { return o != nil && o.Type() == object.ERROR_OBJ }

func (e *evalEngine) Engine() Engine               { return EngineEvaluator }
func (e *evalEngine) Variables() []Variable        { return e.decls }
func (e *evalEngine) IO() map[string]object.Object { return evaluator.IO() }
func (e *evalEngine) Rows() map[int]bool           { return e.rows }
func (e *evalEngine) FirstRow() int                { return e.first }
func (e *evalEngine) Reset()                       {}

func (e *evalEngine) Read(path []string) (object.Object, bool) {
	if len(path) == 0 {
		return nil, false
	}
	top, ok := e.progEnv.GetRaw(path[0])
	if !ok {
		return nil, false
	}
	top = e.deref(top)
	if top == nil {
		return nil, false
	}
	o, ok := readMember(top, path[1:])
	return unwrap(o), ok && o != nil
}

// deref is o as a variable's value: a constant's value, a located
// variable's from the I/O image.
func (e *evalEngine) deref(o object.Object) object.Object {
	o = unwrap(o)
	if p, ok := o.(*object.Pointer); ok && p.Env == nil {
		return evaluator.IO()[p.Name]
	}
	return o
}

func (e *evalEngine) Write(path []string, o object.Object) error {
	cur, ok := e.progEnv.GetRaw(path[0])
	if !ok {
		return fmt.Errorf("%s has no variable %s", e.name, path[0])
	}
	if len(path) == 1 {
		if _, constant := cur.(*object.Constant); constant {
			return fmt.Errorf("%s is a constant", path[0])
		}
		if p, located := cur.(*object.Pointer); located && p.Env == nil {
			evaluator.IO()[p.Name] = o
			return nil
		}
		e.progEnv.Set(path[0], o)
		return nil
	}
	parent, ok := e.Read(path[:len(path)-1])
	if !ok {
		return fmt.Errorf("no %s", strings.Join(path[:len(path)-1], "."))
	}
	return setMember(parent, path[len(path)-1], o)
}

// Run runs the scan in a goroutine of its own, or lets the one parked in
// the hook go on, and waits for the scan to end or to stop again.
func (e *evalEngine) Run(hook func(site) bool, _ bool) error {
	e.hook = hook
	evaluator.Debug(e)
	if !e.running {
		e.running = true
		e.stack, e.failed = e.stack[:0], nil
		evaluator.SetBudget(e.budget)
		go func() {
			out := evaluator.Eval(e.call, e.env)
			var err error
			if isEvalError(out) {
				err = errors.New(strings.TrimPrefix(out.Inspect(), "ERROR: "))
			}
			e.done <- err
		}()
	} else {
		e.resume <- struct{}{}
	}
	select {
	case <-e.paused:
		return errPaused
	case err := <-e.done:
		e.running = false
		return err
	}
}

// Close lets a scan parked in the hook run to its end, with nothing to
// stop it, so no goroutine is left behind.
func (e *evalEngine) Close() {
	if e.running {
		e.hook = nil
		e.resume <- struct{}{}
		<-e.done
		e.running = false
	}
}

// Enter is evaluator.Debugger's: a POU's body starts.
func (e *evalEngine) Enter(pou string, env *object.Environment, levels int) {
	name, _, _ := strings.Cut(pou, ".")
	e.stack = append(e.stack, evalFrame{name: pou, env: env, levels: levels, user: e.userPOU[strings.ToUpper(name)]})
}

// Leave is evaluator.Debugger's: the body is over.
func (e *evalEngine) Leave() {
	if len(e.stack) > 0 {
		e.stack = e.stack[:len(e.stack)-1]
	}
}

// Statement is evaluator.Debugger's: before each statement, it asks the
// session's hook, and parks the scan while the program is stopped.
func (e *evalEngine) Statement(stmt ast.Statement, _ *object.Environment) {
	if len(e.stack) == 0 {
		return
	}
	f := &e.stack[len(e.stack)-1]
	f.row, f.column = safePos(stmt)
	if !f.user || e.hook == nil || f.row <= 0 {
		return
	}
	if e.hook(site{row: f.row, column: f.column, depth: len(e.stack), statement: true}) {
		e.paused <- struct{}{}
		<-e.resume
	}
}

// Failed is evaluator.Debugger's: it keeps the stack where the scan
// failed, for the exception's frames.
func (e *evalEngine) Failed(ast.Node) {
	if e.failed == nil {
		e.failed = append([]evalFrame(nil), e.stack...)
	}
}

func (e *evalEngine) Frames() []frame {
	stack := e.stack
	if !e.running && e.failed != nil {
		stack = e.failed
	}
	out := make([]frame, 0, len(stack))
	for i := len(stack) - 1; i >= 0; i-- {
		f := stack[i]
		if !f.user {
			continue
		}
		fr := frame{name: f.name, row: f.row, column: f.column, depth: i + 1}
		if i > 0 {
			env, levels := f.env, f.levels
			fr.locals = func() []Value { return envValues(env, levels) }
			fr.setLocal = func(name, value string) (object.Object, error) { return envSet(env, levels, name, value) }
		}
		out = append(out, fr)
	}
	return out
}

// envValues are the variables of env and the levels-1 environments
// outside it, innermost first, each name once.
func envValues(env *object.Environment, levels int) []Value {
	seen := map[string]bool{}
	var out []Value
	for ; levels > 0 && env != nil; levels-- {
		names := env.Names()
		sort.Slice(names, func(i, j int) bool { return strings.ToUpper(names[i]) < strings.ToUpper(names[j]) })
		for _, name := range names {
			key := strings.ToUpper(name)
			o, _ := env.GetRaw(name)
			if seen[key] || hidden(name, o) {
				continue
			}
			seen[key] = true
			o = unwrap(o)
			if p, ok := o.(*object.Pointer); ok && p.Env != nil {
				// A VAR_IN_OUT: the caller's variable.
				if v, ok := p.Env.Get(p.Name); ok {
					o = unwrap(v)
				}
			}
			out = append(out, Value{Name: name, Type: typeName(o), Object: o})
		}
		env = env.Outer()
	}
	return out
}

// envSet writes value to the variable name of env or the levels-1
// environments outside it.
func envSet(env *object.Environment, levels int, name, value string) (object.Object, error) {
	for ; levels > 0 && env != nil; levels-- {
		cur, ok := env.GetRaw(name)
		if !ok {
			env = env.Outer()
			continue
		}
		if _, constant := cur.(*object.Constant); constant {
			return nil, fmt.Errorf("%s is a constant", name)
		}
		o, err := parseValue(valueType(unwrap(cur)), value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		env.Set(name, o)
		return o, nil
	}
	return nil, fmt.Errorf("no local %s", name)
}
