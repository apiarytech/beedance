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

import (
	"sort"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// Report is what an online change did with the program's variables.
type Report struct {
	// Kept are the variables whose values were carried over: those of the
	// same name and type. A function block instance or structure is kept
	// member by member, so members added start at their initial values and
	// members removed are dropped.
	Kept []string
	// Reset start at their initial values: their type changed, they are
	// constants, or they hold references, which would point into the
	// replaced program.
	Reset []string
	// Added are new variables; Removed, variables the new program no longer
	// has.
	Added, Removed []string
	// Unforced are the forces dropped because their variable or address is
	// gone, or no longer takes their value.
	Unforced []string
}

// Change replaces the program with the same PROGRAM of source while it
// runs, between scans, as a PLC's online change does: the new code is
// compiled and initialized, the variables are carried over (see Report),
// as are the I/O image, the clock, the forces and the breakpoints (by
// line, set again on the new source). On an error nothing changes.
//
// Stopped between scans, the change is made at once; stopped in a scan,
// it is ErrMidScan. While a run is in progress, it is made at the next
// scan boundary, and Change waits for it: at most a scan, unless the run
// stops in a scan first (ErrMidScan).
func (s *Session) Change(source string) (Report, error) {
	if s.mu.TryLock() {
		defer s.mu.Unlock()
		return s.change(source)
	}
	req := &changeRequest{source: source, done: make(chan changeResult, 1)}
	s.cmu.Lock()
	s.changes = append(s.changes, req)
	s.cmu.Unlock()
	for {
		select {
		case res := <-req.done:
			return res.report, res.err
		case <-time.After(10 * time.Millisecond):
			// The run may have stopped before it saw the request.
			if s.mu.TryLock() {
				s.serveChanges()
				s.mu.Unlock()
			}
		}
	}
}

// changeRequest is a Change waiting for a scan boundary.
type changeRequest struct {
	source string
	done   chan changeResult
}

type changeResult struct {
	report Report
	err    error
}

// serveChanges makes the changes waiting, or refuses them in a scan; mu is
// held.
func (s *Session) serveChanges() {
	s.cmu.Lock()
	todo := s.changes
	s.changes = nil
	s.cmu.Unlock()
	for _, req := range todo {
		r, err := s.change(req.source)
		req.done <- changeResult{r, err}
	}
}

// change is Change, with mu held.
func (s *Session) change(source string) (Report, error) {
	if s.host != nil {
		return Report{}, ErrAttached
	}
	if s.midScan {
		return Report{}, ErrMidScan
	}
	old := s.img
	img, err := s.build(source, old.name)
	if err != nil {
		return Report{}, err
	}

	var r Report
	c := carrier{old: old.members(), fresh: img.members()}
	oldVars := map[string]bool{}
	for _, v := range old.eng.Variables() {
		oldVars[strings.ToUpper(v.Name)] = true
	}
	for _, nv := range img.eng.Variables() {
		key := strings.ToUpper(nv.Name)
		ov, existed := old.decls[key]
		delete(oldVars, key)
		switch {
		case !existed:
			r.Added = append(r.Added, nv.Name)
		case nv.Constant || !strings.EqualFold(nv.Type, ov.Type):
			r.Reset = append(r.Reset, nv.Name)
		case nv.Address != "":
			r.Kept = append(r.Kept, nv.Name) // located: its value is in the I/O image
		default:
			oldV, _ := old.eng.Read([]string{nv.Name})
			freshV, _ := img.eng.Read([]string{nv.Name})
			v, ok := c.carry(oldV, freshV, ov.Type, nv.Type)
			if _, inPlace := v.(*remoteValue); ok && inPlace {
				r.Kept = append(r.Kept, nv.Name) // carried member by member, in the other process
			} else if ok && img.eng.Write([]string{nv.Name}, v) == nil {
				r.Kept = append(r.Kept, nv.Name)
			} else {
				r.Reset = append(r.Reset, nv.Name)
			}
		}
	}
	for _, v := range old.eng.Variables() {
		if oldVars[strings.ToUpper(v.Name)] {
			r.Removed = append(r.Removed, v.Name)
		}
	}
	carryPlants(old, img, c)
	// The I/O image: the plant's inputs, the outputs as last written.
	if oldIO, newIO := old.eng.IO(), img.eng.IO(); oldIO != nil && newIO != nil {
		for addr, v := range oldIO {
			newIO[addr] = v
		}
	}
	old.eng.Close()

	s.cmu.Lock()
	s.img = img
	s.retrace()
	for key, f := range s.forces {
		t, err := s.resolve(f.name)
		if err == nil {
			_, err = parseValue(t.typ, f.value)
		}
		if err != nil {
			delete(s.forces, key)
			r.Unforced = append(r.Unforced, f.name)
			continue
		}
		f.t = t
	}
	var specs []BreakpointSpec
	for _, b := range s.bps {
		specs = append(specs, b.BreakpointSpec)
	}
	s.setBreakpoints(specs)
	s.cmu.Unlock()
	s.applyForces()
	sort.Strings(r.Unforced)
	return r, nil
}

// carrier carries values from one program to its changed version, with
// the declared types of each version's function blocks' members.
type carrier struct {
	old, fresh map[string]map[string]string
}

// carry is the new value of a variable that was old and starts as fresh in
// the changed program: old's value where the two agree, fresh's where they
// do not. oldType and freshType are the declared types, where known. It
// reports false if nothing of old could be kept.
func (c carrier) carry(old, fresh object.Object, oldType, freshType string) (object.Object, bool) {
	if oldType != "" && freshType != "" && !strings.EqualFold(oldType, freshType) {
		return fresh, false
	}
	if old == nil {
		return fresh, false
	}
	if fresh == nil {
		return old, isPlain(old)
	}
	switch f := fresh.(type) {
	case *remoteValue:
		// A value in another process (the go engine): member by member,
		// written over the pipe.
		if !f.structured {
			return fresh, false
		}
		oldMembers := map[string]Value{}
		for _, m := range Members(old) {
			oldMembers[strings.ToUpper(m.Name)] = m
		}
		oldTypes, freshTypes := c.old[strings.ToUpper(typeName(old))], c.fresh[strings.ToUpper(f.typ)]
		for _, fm := range f.members() {
			key := strings.ToUpper(fm.Name)
			om, ok := oldMembers[key]
			if !ok || om.Object == nil {
				continue
			}
			if inner, structured := fm.Object.(*remoteValue); structured && inner.structured {
				c.carry(om.Object, inner, oldTypes[key], freshTypes[key])
				continue
			}
			if v, ok := c.carry(om.Object, fm.Object, oldTypes[key], freshTypes[key]); ok {
				f.set(fm.Name, Format(v))
			}
		}
		return f, true
	case *object.FunctionBlockInstance:
		// An instance on the evaluator: member by member, in its environment.
		o, ok := old.(*object.FunctionBlockInstance)
		if !ok || typeName(o) != typeName(f) {
			return fresh, false
		}
		oldTypes, freshTypes := c.old[strings.ToUpper(typeName(o))], c.fresh[strings.ToUpper(typeName(f))]
		for _, name := range f.Env.Names() {
			// Its variables, and a standard block's own state (__startTime),
			// which a timer keeps its elapsed time in; not its code.
			fv, _ := f.Env.GetRaw(name)
			if isCode(fv) || strings.EqualFold(name, "THIS") {
				continue
			}
			ov, ok := o.Env.GetRaw(name)
			if !ok || isCode(ov) {
				continue
			}
			key := strings.ToUpper(name)
			if v, ok := c.carry(unwrap(ov), unwrap(fv), oldTypes[key], freshTypes[key]); ok {
				if _, constant := fv.(*object.Constant); !constant {
					f.Env.Set(name, v)
				}
			}
		}
		// A standard block's state the fresh instance has not made yet.
		for _, name := range o.Env.Names() {
			ov, _ := o.Env.GetRaw(name)
			if _, fresh := f.Env.GetRaw(name); !fresh && strings.HasPrefix(name, "__") && ov != nil && isPlain(ov) {
				f.Env.Set(name, ov)
			}
		}
		return f, true
	case *object.Hash:
		o, ok := old.(*object.Hash)
		if !ok || typeName(o) != typeName(f) {
			return fresh, false
		}
		oldTypes, freshTypes := c.old[strings.ToUpper(typeName(o))], c.fresh[strings.ToUpper(typeName(f))]
		byName := map[string]object.Object{}
		for _, p := range o.Pairs {
			byName[strings.ToUpper(p.Key.Inspect())] = p.Value
		}
		for k, p := range f.Pairs {
			name := p.Key.Inspect()
			if strings.HasPrefix(name, "__") {
				continue // the class: the new code's
			}
			key := strings.ToUpper(name)
			if ov, ok := byName[key]; ok {
				if v, ok := c.carry(ov, p.Value, oldTypes[key], freshTypes[key]); ok {
					f.Pairs[k] = object.HashPair{Key: p.Key, Value: v}
				}
			}
		}
		return f, true
	case *object.Array:
		o, ok := old.(*object.Array)
		if !ok || len(o.Elements) != len(f.Elements) || o.LowerBound != f.LowerBound {
			return fresh, false
		}
		for i := range f.Elements {
			if v, ok := c.carry(o.Elements[i], f.Elements[i], "", ""); ok {
				f.Elements[i] = v
			}
		}
		return f, true
	}
	if !isPlain(old) || oldType == "" && family(old) != family(fresh) {
		return fresh, false
	}
	return old, true
}

// family is the kind of value o is, whatever its width: the VM may hold
// an INT as an LInt, and a value written by a debugger as an Int.
func family(o object.Object) string {
	switch o.(type) {
	case *object.SInt, *object.Int, *object.DInt, *object.LInt,
		*object.USInt, *object.UInt, *object.UDInt, *object.ULInt:
		return "integer"
	case *object.Byte, *object.Word, *object.DWord, *object.LWord:
		return "bits"
	case *object.Real, *object.LReal:
		return "real"
	}
	return string(o.Type())
}

// members are the declared types of the members of img's function blocks,
// by upper-case block and member name.
func (img *image) members() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, st := range img.program.Statements {
		fb, ok := st.(*ast.FunctionBlockDeclaration)
		if !ok || fb.Name == nil {
			continue
		}
		m := map[string]string{}
		for _, group := range [][]*ast.VarDeclStatement{fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars} {
			for _, d := range group {
				if d.Name != nil && d.DataType != nil {
					m[strings.ToUpper(d.Name.Value)] = strings.ToUpper(d.DataType.String())
				}
			}
		}
		out[strings.ToUpper(fb.Name.Value)] = m
	}
	return out
}

// isCode reports whether o is code or a definition rather than a value.
func isCode(o object.Object) bool {
	switch o.(type) {
	case *object.Builtin, *object.Function, *object.Program, *object.FunctionBlock, *object.BuiltinFunctionBlock,
		*object.Method, *object.Closure, *object.CompiledFunction:
		return true
	}
	return false
}

// isPlain reports whether o is a value that can move to another program:
// not code, nor a reference into its variables.
func isPlain(o object.Object) bool {
	switch o.(type) {
	case *object.Closure, *object.CompiledFunction, *object.Builtin,
		*object.Reference, *object.Pointer, *object.Hash, *object.Array,
		*object.FunctionBlockInstance, *object.Constant, *remoteValue:
		return false
	}
	return true
}
