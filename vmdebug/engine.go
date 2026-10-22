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

// The engine: what runs the program's scans for a Session. The session
// keeps the scan loop, the breakpoints, the stepping, the forces, the
// writes, the live values, the trace, the plant models and online change;
// the engine runs one scan with the session's hook in it, and gives the
// program's variables and the frames it is stopped in. The VM
// (engine_vm.go) is the first engine; the evaluator (engine_eval.go) and
// a transpiled program (engine_go.go) are the others.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/watch"
)

// Engine names what runs the program: Options.Engine.
type Engine string

// The engines.
const (
	// EngineVM runs the program compiled to bytecode on the VM: the
	// default. It alone has a disassembly and instruction steps.
	EngineVM Engine = "vm"
	// EngineEvaluator runs the program on the tree-walking evaluator, the
	// toolchain's reference engine, as it is in a browser.
	EngineEvaluator Engine = "eval"
	// EngineGo transpiles the program to Go, builds it with the Go
	// toolchain (royaljelly, as the firmware is built) and runs it in a
	// process of its own, the debugger speaking to it on its pipes.
	EngineGo Engine = "go"
)

// ParseEngine reads an engine's name; empty is the VM.
func ParseEngine(name string) (Engine, error) {
	switch e := Engine(strings.ToLower(strings.TrimSpace(name))); e {
	case "", EngineVM:
		return EngineVM, nil
	case EngineEvaluator, "evaluator":
		return EngineEvaluator, nil
	case EngineGo, "transpiler":
		return EngineGo, nil
	default:
		return "", fmt.Errorf("no engine %q: vm, eval or go", name)
	}
}

// Variable is a program variable as declared.
type Variable struct {
	Name     string
	Type     string
	Address  string // a located variable's
	Constant bool
}

// errPaused is returned by an engine's Run when the hook stopped the scan:
// the next Run goes on where it stopped.
var errPaused = errors.New("vmdebug: paused")

// site is where an engine asks the session's hook whether to stop.
type site struct {
	row, column int
	depth       int  // the number of calls, 1 in the program's body
	statement   bool // at the start of a statement; false for an instruction inside one (instruction steps)
}

// frame is a call an engine is stopped in.
type frame struct {
	name        string
	row, column int
	depth       int
	address     int // the instruction's address, where the engine has a disassembly
	// locals gives the frame's local variables; nil when it has none (the
	// program's body: its variables are the globals).
	locals func() []Value
	// setLocal writes a local or, in a function block's body, a member of
	// its instance; it returns the value written.
	setLocal func(name, value string) (object.Object, error)
}

// engine runs a PROGRAM's scans for a Session.
type engine interface {
	// Engine is the engine's name.
	Engine() Engine
	// Variables are the program's variables as declared, in order.
	Variables() []Variable
	// Read is the value at path, a variable's dotted name split; false if
	// there is none.
	Read(path []string) (object.Object, bool)
	// Write puts o at path.
	Write(path []string, o object.Object) error
	// IO is the I/O image, by address; the plant models share it.
	IO() map[string]object.Object
	// Rows are the lines a statement the program runs starts on: the
	// body's and the source's own POUs'.
	Rows() map[int]bool
	// FirstRow is the body's first statement's line.
	FirstRow() int
	// Reset starts the body over, for the next scan.
	Reset()
	// Run runs the scan until it ends (nil), until hook stops it
	// (errPaused: the next Run goes on there), or until it fails. The hook
	// is asked before each statement of the source; with instructions,
	// before each instruction of it, where the engine has them.
	Run(hook func(site) bool, instructions bool) error
	// Frames are the calls the run is stopped in, innermost first; none
	// between scans.
	Frames() []frame
	// Close frees what the engine holds.
	Close()
}

// declVariables are decl's variables as declared, and the addresses of
// the located ones by upper-case name, for an engine that reads them from
// the syntax tree.
func declVariables(decl *ast.ProgramDeclaration) (vars []Variable, located map[string]string) {
	located = map[string]string{}
	for _, group := range [][]*ast.VarDeclStatement{decl.VarInputs, decl.VarOutputs, decl.VarInOuts, decl.Vars} {
		for _, d := range group {
			if d == nil || d.Name == nil {
				continue
			}
			v := Variable{Name: d.Name.Value, Constant: d.IsConstant}
			if d.DataType != nil {
				v.Type = d.DataType.String()
			}
			if d.Location != nil && d.Location.Location != nil {
				v.Address = d.Location.Location.String()
				located[strings.ToUpper(v.Name)] = v.Address
			}
			vars = append(vars, v)
		}
	}
	return vars, located
}

// readMember follows path through the members of o.
func readMember(o object.Object, path []string) (object.Object, bool) {
	return watch.Member(o, path)
}

// setMember writes o into the member name of parent, a structure, a
// function block instance or an array (name "[2]").
func setMember(parent object.Object, name string, o object.Object) error {
	switch c := parent.(type) {
	case *object.Hash:
		for k, p := range c.Pairs {
			if strings.EqualFold(p.Key.Inspect(), name) {
				c.Pairs[k] = object.HashPair{Key: p.Key, Value: o}
				return nil
			}
		}
	case *object.FunctionBlockInstance:
		if _, ok := c.Env.GetRaw(name); ok {
			c.Env.Set(name, o)
			return nil
		}
	case *object.Array:
		i, ok := arrayIndex(c, name)
		if ok {
			c.Elements[i] = o
			return nil
		}
		return fmt.Errorf("no element %s", name)
	default:
		return fmt.Errorf("%s has no members", Format(parent))
	}
	return fmt.Errorf("no member %s", name)
}
