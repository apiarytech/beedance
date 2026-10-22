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

// The VM engine: the program compiled to bytecode, run on the VM with its
// hook (vm.SetHook) and the line table the compiler gives the code
// (Bytecode.Lines, CompiledFunction.Lines).

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
)

// vmEngine is a PROGRAM on the VM.
type vmEngine struct {
	name     string
	compiled *compiler.CompiledProgram
	globals  []object.Object
	vm       *vm.VM
	userPOU  map[string]bool // the source's own POUs, upper case

	decls    map[string]compiler.Variable                           // by upper-case name
	marks    map[*object.CompiledFunction]map[int]object.SourceLine // statement starts; nil: not the source's
	rows     map[int]bool
	firstRow int

	blocks    []block // the code at its addresses (code.go)
	blockBase map[*object.CompiledFunction]int

	hook         func(site) bool
	instructions bool
}

// newVMEngine compiles img's PROGRAM and runs its initialization on a
// store of its own; clock is the session's.
func newVMEngine(img *image, o Options, clock func() time.Duration) (*vmEngine, error) {
	object.FinalizeBuiltins()
	comp := compiler.NewCompilerWithBuiltins(object.Builtins)
	comp.AllowInputWrites = o.AllowInputWrites
	compiled, err := comp.CompileProgramUnit(img.program, img.name)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	globals := make([]object.Object, vm.GlobalsSize)
	initVM := vm.NewWithGlobalsStore(compiled.InitBytecode, globals)
	cyclic := vm.NewWithGlobalsStore(compiled.CyclicBytecode, globals)
	for idx, b := range stdlib.ClockBuiltins(clock) {
		initVM.SetBuiltin(idx, b)
		cyclic.SetBuiltin(idx, b)
	}
	initVM.SetBudget(o.Budget)
	if err := initVM.Run(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	for addr, v := range initVM.IO() {
		cyclic.IO()[addr] = v
	}
	cyclic.SetBudget(o.Budget)
	return attachVMEngine(img, compiled, globals, cyclic), nil
}

// attachVMEngine is the engine on a program compiled and initialized
// already: a host's (attach.go).
func attachVMEngine(img *image, compiled *compiler.CompiledProgram, globals []object.Object, cyclic *vm.VM) *vmEngine {
	e := &vmEngine{name: img.name, compiled: compiled, globals: globals, vm: cyclic, userPOU: img.userPOU,
		decls: map[string]compiler.Variable{}, marks: map[*object.CompiledFunction]map[int]object.SourceLine{}, rows: map[int]bool{}}
	for _, v := range compiled.Variables {
		e.decls[strings.ToUpper(v.Name)] = v
	}
	for _, l := range compiled.CyclicBytecode.Lines {
		e.rows[l.Row] = true
		if e.firstRow == 0 {
			e.firstRow = l.Row
		}
	}
	for _, c := range compiled.CyclicBytecode.Constants {
		if fn, ok := c.(*object.CompiledFunction); ok && e.isUser(fn) {
			for _, l := range fn.Lines {
				e.rows[l.Row] = true
			}
		}
	}
	return e
}

func (e *vmEngine) Engine() Engine { return EngineVM }

func (e *vmEngine) Variables() []Variable {
	out := make([]Variable, 0, len(e.compiled.Variables))
	for _, v := range e.compiled.Variables {
		out = append(out, Variable{Name: v.Name, Type: v.Type, Address: v.Address, Constant: v.Constant})
	}
	return out
}

func (e *vmEngine) Read(path []string) (object.Object, bool) {
	if len(path) == 0 {
		return nil, false
	}
	d, ok := e.decls[strings.ToUpper(path[0])]
	if !ok {
		return nil, false
	}
	var top object.Object
	if d.Address != "" {
		top = e.vm.IO()[d.Address]
	} else if d.Global >= 0 && d.Global < len(e.globals) {
		top = e.globals[d.Global]
	}
	if top == nil {
		return nil, false
	}
	return readMember(top, path[1:])
}

func (e *vmEngine) Write(path []string, o object.Object) error {
	d, ok := e.decls[strings.ToUpper(path[0])]
	if !ok {
		return fmt.Errorf("%s has no variable %s", e.name, path[0])
	}
	if len(path) == 1 {
		if d.Address != "" {
			e.vm.IO()[d.Address] = o
			return nil
		}
		if d.Global < 0 || d.Global >= len(e.globals) {
			return fmt.Errorf("%s has no slot", path[0])
		}
		e.globals[d.Global] = o
		return nil
	}
	parent, ok := e.Read(path[:len(path)-1])
	if !ok {
		return fmt.Errorf("no %s", strings.Join(path[:len(path)-1], "."))
	}
	return setMember(parent, path[len(path)-1], o)
}

func (e *vmEngine) IO() map[string]object.Object { return e.vm.IO() }
func (e *vmEngine) Rows() map[int]bool           { return e.rows }
func (e *vmEngine) FirstRow() int                { return e.firstRow }
func (e *vmEngine) Reset()                       { e.vm.Reset() }
func (e *vmEngine) Close()                       {}

func (e *vmEngine) Run(hook func(site) bool, instructions bool) error {
	e.hook, e.instructions = hook, instructions
	if hook == nil {
		e.vm.SetHook(nil)
	} else {
		e.vm.SetHook(e.vmHook)
	}
	err := e.vm.Run()
	if errors.Is(err, vm.ErrPaused) {
		return errPaused
	}
	return err
}

// unhook takes the debugger's hook out of the VM, for a host that runs on
// without it (attach.go).
func (e *vmEngine) unhook() { e.vm.SetHook(nil) }

// vmHook is the VM's hook: it asks the session's before each statement of
// the source, or each instruction of it in instruction mode.
func (e *vmEngine) vmHook(fn *object.CompiledFunction, ip, depth int) bool {
	if e.instructions {
		if !e.isUser(fn) {
			return false
		}
		l, _ := object.LineAt(fn.Lines, ip)
		_, start := e.markAt(fn, ip)
		return e.hook(site{row: l.Row, column: l.Column, depth: depth, statement: start})
	}
	line, ok := e.markAt(fn, ip)
	if !ok {
		return false
	}
	return e.hook(site{row: line.Row, column: line.Column, depth: depth, statement: true})
}

// isUser reports whether fn is the code of one of the source's own POUs
// (or of the program's body), whose lines are the source's; a standard
// block's are not.
func (e *vmEngine) isUser(fn *object.CompiledFunction) bool {
	if fn.Name == "" {
		return true
	}
	pou, _, _ := strings.Cut(fn.Name, ".")
	return e.userPOU[strings.ToUpper(pou)]
}

// markAt is the statement starting at ip in fn, if fn is the source's.
func (e *vmEngine) markAt(fn *object.CompiledFunction, ip int) (object.SourceLine, bool) {
	m, seen := e.marks[fn]
	if !seen {
		if e.isUser(fn) && len(fn.Lines) > 0 {
			m = make(map[int]object.SourceLine, len(fn.Lines))
			for _, l := range fn.Lines {
				m[l.Offset] = l
			}
		}
		e.marks[fn] = m
	}
	l, ok := m[ip]
	return l, ok
}

func (e *vmEngine) Frames() []frame {
	vfs := e.vm.Frames()
	out := make([]frame, 0, len(vfs))
	for i := len(vfs) - 1; i >= 0; i-- {
		f := vfs[i]
		if !e.isUser(f.Fn) {
			continue
		}
		fr := frame{name: f.Fn.Name, depth: i + 1}
		if fr.name == "" {
			fr.name = e.name
		}
		if l, ok := object.LineAt(f.Fn.Lines, f.IP); ok {
			fr.row, fr.column = l.Row, l.Column
		}
		fr.address = e.address(f.Fn, f.IP)
		if i > 0 {
			fr.locals = func() []Value { return vmLocals(f) }
			fr.setLocal = func(name, value string) (object.Object, error) { return vmSetLocal(f, name, value) }
		}
		out = append(out, fr)
	}
	return out
}

// vmLocals are the local variables of f: in a function block's body or
// method, its instance's members first.
func vmLocals(f vm.FrameInfo) []Value {
	var out []Value
	for i, o := range f.Locals {
		name := ""
		if i < len(f.Fn.LocalNames) {
			name = f.Fn.LocalNames[i]
		}
		if name == "" || strings.HasPrefix(name, "__") {
			continue
		}
		if strings.EqualFold(name, "THIS") {
			if h, ok := o.(*object.Hash); ok {
				out = append(Members(h), out...)
			}
			continue
		}
		out = append(out, Value{Name: name, Type: typeName(o), Object: o})
	}
	return out
}

// vmSetLocal writes value to the local name of f, or to the member of the
// function block instance whose body it is.
func vmSetLocal(f vm.FrameInfo, name, value string) (object.Object, error) {
	for i, o := range f.Locals {
		local := ""
		if i < len(f.Fn.LocalNames) {
			local = f.Fn.LocalNames[i]
		}
		if strings.EqualFold(local, "THIS") {
			if h, ok := o.(*object.Hash); ok {
				if v, err := setIn(h, name, value); err == nil {
					return v, nil
				}
			}
			continue
		}
		if strings.EqualFold(local, name) {
			v, err := parseValue(valueType(o), value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			f.Locals[i] = v
			return v, nil
		}
	}
	return nil, fmt.Errorf("no local %s", name)
}
