/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package native lets a host give programs function blocks implemented in
// Go: blocks that cannot be Structured Text, such as TwinCAT's TCP/IP
// blocks (FB_SocketConnect, ...) that OSCAT NETWORK calls, which beebread
// ports onto Go's net package.
//
// A host allows a program the blocks it chooses: it makes a Registry of
// them, compiles the program with the registry's Source (an ST wrapper for
// each block) and gives the program the registry's built-ins, on the VM
// with SetBuiltin and in the evaluator with Bind. A wrapper is an ordinary
// function block whose body passes its inputs to the Go block, runs it and
// reads its outputs back, so the compiler, the VM and the evaluator need
// nothing special. A program can call only the blocks its registry holds;
// without a registry the built-ins refuse every call.
//
// beedance does not import the blocks: the host, which links them, does.
package native

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/stdlib"
)

// Instance is a function block implemented in Go: a pointer to a struct
// whose exported fields are its inputs and outputs, named as in ST (case
// does not matter), with royaljelly's IEC types or Go's basic types.
type Instance interface {
	Execute(now time.Time)
}

// Field is an input or output of a block: its ST name and type.
type Field struct {
	Name, Type string
}

// Block declares a native function block.
type Block struct {
	// Name is the block's ST name, e.g. FB_SocketConnect.
	Name string
	// Inputs and Outputs are its VAR_INPUT and VAR_OUTPUT, in order.
	Inputs, Outputs []Field
	// New returns a new instance.
	New func() Instance
}

// Registry is the native blocks one program may call, and their instances.
// It is safe for concurrent use.
type Registry struct {
	blocks map[string]Block // by upper case name
	types  string           // ST TYPE declarations the blocks' fields use
	clock  func() time.Time

	mu        sync.Mutex
	instances []Instance // handle n is instances[n-1]
}

// NewRegistry returns a registry of blocks for one program. types holds the
// ST TYPE declarations their fields use (such as TwinCAT's T_HSOCKET), and
// clock is the time a block's Execute receives (time.Now if nil).
func NewRegistry(clock func() time.Time, types string, blocks ...Block) (*Registry, error) {
	if clock == nil {
		clock = time.Now
	}
	r := &Registry{blocks: map[string]Block{}, types: types, clock: clock}
	for _, b := range blocks {
		if b.Name == "" || b.New == nil {
			return nil, fmt.Errorf("native: a block needs a name and New")
		}
		up := strings.ToUpper(b.Name)
		if _, dup := r.blocks[up]; dup {
			return nil, fmt.Errorf("native: block %s twice", b.Name)
		}
		r.blocks[up] = b
	}
	return r, nil
}

// Source returns the ST a program is compiled with to call the blocks: the
// TYPE declarations, then a wrapper FUNCTION_BLOCK for each block.
func (r *Registry) Source() string {
	var b strings.Builder
	if r.types != "" {
		b.WriteString(r.types)
		b.WriteString("\n")
	}
	names := make([]string, 0, len(r.blocks))
	for n := range r.blocks {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		blk := r.blocks[n]
		fmt.Fprintf(&b, "FUNCTION_BLOCK %s\n", blk.Name)
		writeVars(&b, "VAR_INPUT", blk.Inputs)
		writeVars(&b, "VAR_OUTPUT", blk.Outputs)
		b.WriteString("VAR __handle : DINT; END_VAR\n")
		fmt.Fprintf(&b, "__handle := __NATIVE_NEW(__handle, '%s');\n", blk.Name)
		for _, f := range blk.Inputs {
			fmt.Fprintf(&b, "__NATIVE_SET(__handle, '%s', %s);\n", f.Name, f.Name)
		}
		b.WriteString("__NATIVE_RUN(__handle);\n")
		for _, f := range blk.Outputs {
			fmt.Fprintf(&b, "%s := __NATIVE_GET(__handle, '%s');\n", f.Name, f.Name)
		}
		b.WriteString("END_FUNCTION_BLOCK\n\n")
	}
	return b.String()
}

func writeVars(b *strings.Builder, block string, fields []Field) {
	if len(fields) == 0 {
		return
	}
	b.WriteString(block)
	for _, f := range fields {
		fmt.Fprintf(b, " %s : %s;", f.Name, f.Type)
	}
	b.WriteString(" END_VAR\n")
}

// Builtins returns the registry's built-ins by index, to give a VM with
// SetBuiltin.
func (r *Registry) Builtins() map[int]*object.Builtin {
	return map[int]*object.Builtin{
		stdlib.BuiltinNativeNew: {Fn: r.newFn},
		stdlib.BuiltinNativeSet: {Fn: r.setFn},
		stdlib.BuiltinNativeRun: {Fn: r.runFn},
		stdlib.BuiltinNativeGet: {Fn: r.getFn},
	}
}

// Bind gives an evaluator environment the registry's built-ins.
func (r *Registry) Bind(env *object.Environment) {
	names := map[int]string{
		stdlib.BuiltinNativeNew: "__NATIVE_NEW", stdlib.BuiltinNativeSet: "__NATIVE_SET",
		stdlib.BuiltinNativeRun: "__NATIVE_RUN", stdlib.BuiltinNativeGet: "__NATIVE_GET",
	}
	for idx, b := range r.Builtins() {
		env.Set(names[idx], b)
	}
}

// __NATIVE_NEW(handle, name) returns handle, or the handle of a new
// instance of block name if handle is 0.
func (r *Registry) newFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("__NATIVE_NEW: want 2 arguments, got %d", len(args))
	}
	h, _, ok := object.GetIntegerObjectValue(args[0])
	name, isStr := args[1].(*object.String)
	if !ok || !isStr {
		return object.NewBuiltinError("__NATIVE_NEW: want a handle and a block name")
	}
	if h != 0 {
		return &object.DInt{Value: int32(h)}
	}
	blk, ok := r.blocks[strings.ToUpper(name.Value)]
	if !ok {
		return object.NewBuiltinError("native block %s is not allowed on this host", name.Value)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instances = append(r.instances, blk.New())
	return &object.DInt{Value: int32(len(r.instances))}
}

func (r *Registry) instance(h object.Object) (Instance, error) {
	n, _, ok := object.GetIntegerObjectValue(h)
	r.mu.Lock()
	defer r.mu.Unlock()
	if !ok || n < 1 || int(n) > len(r.instances) {
		return nil, fmt.Errorf("no native block instance %s", h.Inspect())
	}
	return r.instances[n-1], nil
}

// field returns the Go field named name (ignoring case) of an instance.
func field(inst Instance, name string) (reflect.Value, error) {
	v := reflect.ValueOf(inst).Elem()
	f := v.FieldByNameFunc(func(n string) bool { return strings.EqualFold(n, name) })
	if !f.IsValid() || !f.CanSet() {
		return reflect.Value{}, fmt.Errorf("%s has no field %s", v.Type().Name(), name)
	}
	return f, nil
}

// __NATIVE_SET(handle, field, value) sets an input of an instance.
func (r *Registry) setFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("__NATIVE_SET: want 3 arguments, got %d", len(args))
	}
	inst, err := r.instance(args[0])
	if err != nil {
		return object.NewBuiltinError("__NATIVE_SET: %v", err)
	}
	name, ok := args[1].(*object.String)
	if !ok {
		return object.NewBuiltinError("__NATIVE_SET: want a field name")
	}
	f, err := field(inst, name.Value)
	if err != nil {
		return object.NewBuiltinError("__NATIVE_SET: %v", err)
	}
	if err := toGo(args[2], f); err != nil {
		return object.NewBuiltinError("__NATIVE_SET %s: %v", name.Value, err)
	}
	return object.NULL
}

// __NATIVE_RUN(handle) runs an instance at the registry's clock.
func (r *Registry) runFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("__NATIVE_RUN: want 1 argument, got %d", len(args))
	}
	inst, err := r.instance(args[0])
	if err != nil {
		return object.NewBuiltinError("__NATIVE_RUN: %v", err)
	}
	inst.Execute(r.clock())
	return object.NULL
}

// __NATIVE_GET(handle, field) reads an output of an instance.
func (r *Registry) getFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("__NATIVE_GET: want 2 arguments, got %d", len(args))
	}
	inst, err := r.instance(args[0])
	if err != nil {
		return object.NewBuiltinError("__NATIVE_GET: %v", err)
	}
	name, ok := args[1].(*object.String)
	if !ok {
		return object.NewBuiltinError("__NATIVE_GET: want a field name")
	}
	f, err := field(inst, name.Value)
	if err != nil {
		return object.NewBuiltinError("__NATIVE_GET: %v", err)
	}
	return fromGo(f)
}
