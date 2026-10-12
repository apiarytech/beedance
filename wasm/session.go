/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/sil"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
	"github.com/apiarytech/beedance/watch"
)

// session is a PROGRAM compiled to bytecode and running on the VM, scan by
// scan, on a simulated clock: elapsed advances by interval before each scan.
type session struct {
	name     string
	globals  []object.Object
	scanVM   *vm.VM
	interval time.Duration
	elapsed  time.Duration
	scans    int
	vars     []watch.Variable
	slot     map[string]int               // global slot by upper-case name
	decls    map[string]compiler.Variable // by upper-case name
	located  map[string]compiler.Variable // by upper-case address
}

type loadReply struct {
	Session   int            `json:"session"`
	Program   string         `json:"program"`
	Variables []variable     `json:"variables"`
	Values    map[string]any `json:"values"`
	IO        map[string]any `json:"io"`
}

type scanReply struct {
	Session int            `json:"session"`
	Scans   int            `json:"scans"`
	TimeMs  float64        `json:"timeMs"`
	Values  map[string]any `json:"values"`
	IO      map[string]any `json:"io"`
}

// load compiles req.Program, or the source's first PROGRAM that is not a
// test, and runs its initialization.
func (h *Host) load(req Request) (loadReply, error) {
	object.FinalizeBuiltins()
	program, err := parse(req.Source)
	if err != nil {
		return loadReply{}, err
	}
	decl, err := pick(program, req.Program)
	if err != nil {
		return loadReply{}, err
	}
	compiled, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, decl.Name.Value)
	if err != nil {
		return loadReply{}, fmt.Errorf("compile: %w", err)
	}
	s := &session{
		name:     decl.Name.Value,
		globals:  make([]object.Object, vm.GlobalsSize),
		interval: interval(req.IntervalMs),
		vars:     watch.Variables(program, decl, watch.Options{}),
		slot:     map[string]int{},
		decls:    map[string]compiler.Variable{},
		located:  map[string]compiler.Variable{},
	}
	initVM := vm.NewWithGlobalsStore(compiled.InitBytecode, s.globals)
	s.scanVM = vm.NewWithGlobalsStore(compiled.CyclicBytecode, s.globals)
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return s.elapsed }) {
		initVM.SetBuiltin(idx, b)
		s.scanVM.SetBuiltin(idx, b)
	}
	if err := initVM.Run(); err != nil {
		return loadReply{}, fmt.Errorf("init: %w", err)
	}
	for addr, v := range initVM.IO() {
		s.scanVM.IO()[addr] = v
	}
	reply := loadReply{Program: s.name, Variables: []variable{}}
	for _, v := range compiled.Variables {
		s.decls[strings.ToUpper(v.Name)] = v
		if v.Address != "" {
			s.located[strings.ToUpper(v.Address)] = v
		} else {
			s.slot[strings.ToUpper(v.Name)] = v.Global
		}
		reply.Variables = append(reply.Variables, variable{Name: v.Name, Type: v.Type, Block: v.Block, Address: v.Address})
	}
	reply.Values, reply.IO = s.values(), s.io()

	h.mu.Lock()
	h.next++
	reply.Session = h.next
	h.sessions[h.next] = s
	h.mu.Unlock()
	return reply, nil
}

// pick finds the PROGRAM named name, or, when name is empty, the first that
// is not a test, else the first.
func pick(program *ast.Program, name string) (*ast.ProgramDeclaration, error) {
	all := programs(program)
	if len(all) == 0 {
		return nil, errors.New("the source declares no PROGRAM")
	}
	if name == "" {
		for _, pd := range all {
			if !isTest(pd.Name.Value) {
				return pd, nil
			}
		}
		return all[0], nil
	}
	for _, pd := range all {
		if strings.EqualFold(pd.Name.Value, name) {
			return pd, nil
		}
	}
	return nil, fmt.Errorf("no PROGRAM %s", name)
}

func (h *Host) withSession(req Request) (any, error) {
	h.mu.Lock()
	s, ok := h.sessions[req.Session]
	if ok && strings.EqualFold(req.Op, "unload") {
		delete(h.sessions, req.Session)
	}
	h.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no session %d", req.Session)
	}
	switch strings.ToLower(req.Op) {
	case "scan":
		if err := s.set(req.Values); err != nil {
			return nil, err
		}
		count := req.Count
		if count <= 0 {
			count = 1
		}
		for range count {
			if err := s.scan(); err != nil {
				return nil, err
			}
		}
	case "set":
		if err := s.set(req.Values); err != nil {
			return nil, err
		}
	}
	return scanReply{Session: req.Session, Scans: s.scans, TimeMs: float64(s.elapsed) / float64(time.Millisecond), Values: s.values(), IO: s.io()}, nil
}

// scan runs one scan, interval after the last.
func (s *session) scan() error {
	s.elapsed += s.interval
	s.scans++
	s.scanVM.Reset()
	if err := s.scanVM.Run(); err != nil {
		return fmt.Errorf("scan %d: %w", s.scans, err)
	}
	return nil
}

// set assigns values, by variable name or by address (%IX0.0), converting
// each to the variable's declared type. Every name is checked before any
// value is set.
func (s *session) set(values map[string]any) error {
	type assignment struct {
		addr string
		slot int
		obj  object.Object
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	var todo []assignment
	for _, name := range names {
		key := strings.ToUpper(strings.TrimSpace(name))
		var a assignment
		var typ string
		if strings.HasPrefix(key, "%") {
			a.addr = key
			if d, ok := s.located[key]; ok {
				a.addr, typ = d.Address, d.Type
			} else if t, ok := ast.DirectType(key); ok {
				typ = t
			} else {
				return fmt.Errorf("%s is not an address", name)
			}
		} else {
			d, ok := s.decls[key]
			if !ok {
				return fmt.Errorf("%s has no variable %s", s.name, name)
			}
			if d.Constant {
				return fmt.Errorf("%s is a constant", name)
			}
			typ, a.addr, a.slot = d.Type, d.Address, d.Global
		}
		obj, err := sil.ToObject(typ, values[name])
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		a.obj = obj
		todo = append(todo, a)
	}
	for _, a := range todo {
		if a.addr != "" {
			s.scanVM.IO()[a.addr] = a.obj
		} else {
			s.globals[a.slot] = a.obj
		}
	}
	return nil
}

// values reads the watched variables, as package sil records them.
func (s *session) values() map[string]any {
	out := map[string]any{}
	for _, v := range s.vars {
		o, ok := watch.VM(s.globals, s.slot, v)
		if d, isDecl := s.decls[strings.ToUpper(v.Name)]; !ok && isDecl && d.Address != "" {
			o, ok = s.scanVM.IO()[d.Address] // a located variable has no slot
		}
		if ok && o != nil {
			out[v.Name] = jsonValue(o)
		}
	}
	return out
}

// io reads the I/O image: every located variable and direct address the
// program has used, by address.
func (s *session) io() map[string]any {
	out := map[string]any{}
	for addr, o := range s.scanVM.IO() {
		if o != nil {
			out[addr] = jsonValue(o)
		}
	}
	return out
}
