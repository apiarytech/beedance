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

// The configuration engine: a CONFIGURATION's program instances on their
// tasks (package vmconfig), each instance's body on the VM with the
// session's hook in it. A scan of the session is one cycle of the
// configuration: the instances due run one after another, and a
// breakpoint on a line stops whichever instance runs the statement. The
// variables are the configuration's globals, then each resource with its
// instances as members ("R1.P1.x", as VAR_CONFIG names them); the
// program's body frame lists its instance's variables as its locals.

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/vmconfig"
	"github.com/apiarytech/beedance/watch"
)

// configEngine is a CONFIGURATION on the VM.
type configEngine struct {
	name    string
	cfg     *vmconfig.Configuration
	globals []compiler.Variable // the configuration's and its resources' globals, as declared
	insts   []*configInstance   // in declaration order, resource by resource
	byInst  map[*vmconfig.Instance]*configInstance
	clock   func() time.Duration

	run *vmconfig.CycleRun // the cycle in progress; nil before the first scan
	cur *configInstance    // the instance running, or stopped in

	rows     map[int]bool
	firstRow int
}

// configInstance is a program instance of the configuration.
type configInstance struct {
	inst *vmconfig.Instance
	eng  *vmEngine // its body on its VM
}

// newConfigEngine loads img's CONFIGURATION, compiling its program
// instances and running their initialization; clock is the session's.
func newConfigEngine(img *image, o Options, clock func() time.Duration) (*configEngine, error) {
	cfg, err := vmconfig.LoadSourceWith(img.source, vmconfig.LoadOptions{AllowInputWrites: o.AllowInputWrites, Name: img.name})
	if err != nil {
		return nil, err
	}
	e := &configEngine{name: img.name, cfg: cfg, byInst: map[*vmconfig.Instance]*configInstance{}, clock: clock, rows: map[int]bool{}}
	for _, r := range cfg.Resources {
		for _, inst := range r.Instances {
			inst.VM().SetBudget(o.Budget)
			eng := attachVMEngine(&image{name: inst.Path(), userPOU: img.userPOU}, inst.Compiled(), inst.Store(), inst.VM())
			ci := &configInstance{inst: inst, eng: eng}
			e.insts = append(e.insts, ci)
			e.byInst[inst] = ci
			for row := range eng.Rows() {
				e.rows[row] = true
			}
			if e.firstRow == 0 {
				e.firstRow = eng.FirstRow()
			}
		}
	}
	if len(e.insts) == 0 {
		return nil, fmt.Errorf("CONFIGURATION %s runs no program", img.name)
	}
	e.globals = e.insts[0].inst.Compiled().ConfigGlobals
	return e, nil
}

func (e *configEngine) Engine() Engine { return EngineVM }

// Variables are the globals, then a variable for each resource, whose
// members are its instances.
func (e *configEngine) Variables() []Variable {
	out := make([]Variable, 0, len(e.globals)+len(e.cfg.Resources))
	for _, g := range e.globals {
		out = append(out, Variable{Name: g.Name, Type: g.Type, Address: g.Address, Constant: g.Constant})
	}
	for _, r := range e.cfg.Resources {
		out = append(out, Variable{Name: r.Name, Type: "RESOURCE", group: true})
	}
	return out
}

// global is the declared global called name.
func (e *configEngine) global(name string) (compiler.Variable, bool) {
	for _, g := range e.globals {
		if strings.EqualFold(g.Name, name) {
			return g, true
		}
	}
	return compiler.Variable{}, false
}

// resource is the resource called name.
func (e *configEngine) resource(name string) *vmconfig.Resource {
	for _, r := range e.cfg.Resources {
		if strings.EqualFold(r.Name, name) {
			return r
		}
	}
	return nil
}

// instance is the instance called name of r.
func (e *configEngine) instance(r *vmconfig.Resource, name string) *configInstance {
	for _, inst := range r.Instances {
		if strings.EqualFold(inst.Name, name) {
			return e.byInst[inst]
		}
	}
	return nil
}

// readGlobal is a global's value: the running instance's copy while one
// runs, which is newer than the configuration's.
func (e *configEngine) readGlobal(g compiler.Variable) (object.Object, bool) {
	if g.Address != "" {
		v, ok := e.cfg.IO[g.Address]
		return v, ok && v != nil
	}
	if e.cur != nil {
		if slot, ok := e.cur.inst.SharedSlot(g.Name); ok {
			v := e.cur.inst.Store()[slot]
			return v, v != nil
		}
	}
	return e.cfg.Global(g.Name)
}

func (e *configEngine) Read(path []string) (object.Object, bool) {
	if len(path) == 0 {
		return nil, false
	}
	if g, ok := e.global(path[0]); ok {
		top, found := e.readGlobal(g)
		if !found {
			return nil, false
		}
		return readMember(top, path[1:])
	}
	r := e.resource(path[0])
	if r == nil {
		return nil, false
	}
	if len(path) == 1 {
		h := newHash()
		for _, inst := range r.Instances {
			putHash(h, inst.Name, e.instanceValue(e.byInst[inst]))
		}
		return h, true
	}
	ci := e.instance(r, path[1])
	if ci == nil {
		return nil, false
	}
	if len(path) == 2 {
		return e.instanceValue(ci), true
	}
	return ci.eng.Read(path[2:])
}

// instanceValue is an instance's variables as a structure, to show under
// its resource.
func (e *configEngine) instanceValue(ci *configInstance) object.Object {
	h := newHash()
	for _, v := range ci.eng.Variables() {
		if o, ok := ci.eng.Read([]string{v.Name}); ok {
			putHash(h, v.Name, o)
		}
	}
	return h
}

func (e *configEngine) Write(path []string, o object.Object) error {
	if g, ok := e.global(path[0]); ok {
		if len(path) > 1 {
			parent, found := e.Read(path[:len(path)-1])
			if !found {
				return fmt.Errorf("no %s", strings.Join(path[:len(path)-1], "."))
			}
			return setMember(parent, path[len(path)-1], o)
		}
		if g.Address != "" {
			e.cfg.IO[g.Address] = o
			return nil
		}
		e.cfg.SetGlobal(g.Name, o)
		if e.cur != nil {
			if slot, ok := e.cur.inst.SharedSlot(g.Name); ok {
				e.cur.inst.Store()[slot] = o
			}
		}
		return nil
	}
	r := e.resource(path[0])
	if r == nil {
		return fmt.Errorf("%s has no variable %s", e.name, path[0])
	}
	if len(path) < 3 {
		return fmt.Errorf("%s is a %s: write its variables", strings.Join(path, "."), map[bool]string{true: "resource", false: "program instance"}[len(path) == 1])
	}
	ci := e.instance(r, path[1])
	if ci == nil {
		return fmt.Errorf("resource %s has no program instance %s", r.Name, path[1])
	}
	if err := ci.eng.Write(path[2:], o); err != nil {
		return err
	}
	// A global the instance shares (VAR_EXTERNAL, of the same name) is
	// the configuration's until the instance runs.
	if len(path) == 3 && ci != e.cur {
		if slot, ok := ci.inst.SharedSlot(path[2]); ok {
			e.cfg.SetGlobal(path[2], ci.inst.Store()[slot])
		}
	}
	return nil
}

func (e *configEngine) IO() map[string]object.Object { return e.cfg.IO }
func (e *configEngine) Rows() map[int]bool           { return e.rows }
func (e *configEngine) FirstRow() int                { return e.firstRow }
func (e *configEngine) Close()                       {}

// Reset starts the next cycle, at the session's clock.
func (e *configEngine) Reset() {
	e.run, e.cur = e.cfg.Begin(e.clock()), nil
}

// Run runs the instances due in the cycle, one after another, each to the
// end of its body unless hook stops it there.
func (e *configEngine) Run(hook func(site) bool, instructions bool) error {
	for {
		if e.cur == nil {
			if e.run == nil {
				return nil
			}
			inst := e.run.Next()
			if inst == nil {
				e.run = nil
				return nil
			}
			inst.In()
			e.cur = e.byInst[inst]
		}
		err := e.cur.eng.Run(hook, instructions)
		if errors.Is(err, errPaused) {
			return err
		}
		if err != nil {
			// e.cur stays, for the frames of the exception, until Reset.
			run := e.run
			e.run = nil
			return run.Wrap(fmt.Errorf("program %s: %w", e.cur.inst.Name, err))
		}
		e.cur.inst.Out()
		e.cur = nil
	}
}

// Frames are the running instance's calls; its body's frame, named for
// the instance ("R1.P1"), has the instance's variables as its locals.
func (e *configEngine) Frames() []frame {
	if e.cur == nil {
		return nil
	}
	ci := e.cur
	fs := ci.eng.Frames()
	for i := range fs {
		if fs[i].depth != 1 {
			continue
		}
		fs[i].locals = func() []Value {
			var out []Value
			for _, v := range ci.eng.Variables() {
				if strings.HasPrefix(v.Name, "__") {
					continue
				}
				val := Value{Name: v.Name, Type: v.Type, Address: v.Address}
				if o, ok := ci.eng.Read([]string{v.Name}); ok {
					val.Object = o
				}
				out = append(out, val)
			}
			return out
		}
		fs[i].setLocal = func(name, value string) (object.Object, error) {
			path := []string{ci.inst.Resource().Name, ci.inst.Name, name}
			cur, _ := ci.eng.Read([]string{name})
			typ := valueType(cur)
			for _, v := range ci.eng.Variables() {
				if strings.EqualFold(v.Name, name) && isElementary(v.Type) {
					typ = v.Type
				}
			}
			o, err := parseValue(typ, value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return o, e.Write(path, o)
		}
	}
	return fs
}

// carry carries the variables of old's instances into e's, for online
// change: an instance of the same name and program keeps the values of
// its variables that kept their types. It adds to r under the
// variables' paths, "R1.P1.x".
func (e *configEngine) carry(old *configEngine, c carrier, r *Report) {
	for _, ci := range e.insts {
		path := ci.inst.Path()
		var was *configInstance
		if res := old.resource(ci.inst.Resource().Name); res != nil {
			was = old.instance(res, ci.inst.Name)
		}
		if was == nil || !strings.EqualFold(was.inst.Type, ci.inst.Type) {
			r.Added = append(r.Added, path)
			continue
		}
		oldVars := map[string]Variable{}
		for _, v := range was.eng.Variables() {
			oldVars[strings.ToUpper(v.Name)] = v
		}
		for _, nv := range ci.eng.Variables() {
			key := strings.ToUpper(nv.Name)
			name := path + "." + nv.Name
			ov, existed := oldVars[key]
			delete(oldVars, key)
			if _, shared := ci.inst.SharedSlot(nv.Name); shared {
				continue // a global: carried as the configuration's
			}
			switch {
			case !existed:
				r.Added = append(r.Added, name)
			case nv.Constant || !strings.EqualFold(nv.Type, ov.Type):
				r.Reset = append(r.Reset, name)
			case nv.Address != "":
				r.Kept = append(r.Kept, name)
			default:
				oldV, _ := was.eng.Read([]string{nv.Name})
				freshV, _ := ci.eng.Read([]string{nv.Name})
				if v, ok := c.carry(oldV, freshV, ov.Type, nv.Type); ok && ci.eng.Write([]string{nv.Name}, v) == nil {
					r.Kept = append(r.Kept, name)
				} else {
					r.Reset = append(r.Reset, name)
				}
			}
		}
		for _, v := range was.eng.Variables() {
			if _, gone := oldVars[strings.ToUpper(v.Name)]; gone {
				r.Removed = append(r.Removed, path+"."+v.Name)
			}
		}
	}
	for _, was := range old.insts {
		if res := e.resource(was.inst.Resource().Name); res == nil || e.instance(res, was.inst.Name) == nil {
			r.Removed = append(r.Removed, was.inst.Path())
		}
	}
}

// configVariables are the watched values of a configuration: its
// elementary globals, then each instance's, under "R1.P1.".
func configVariables(program *ast.Program, cd *ast.ConfigurationDeclaration, o watch.Options) []watch.Variable {
	var out []watch.Variable
	elementary := func(t string) bool { return isElementary(t) }
	for _, gs := range append(append([]*ast.GlobalVarDeclaration{}, cd.GlobalVars...), resourceGlobals(cd)...) {
		for _, d := range gs.Vars {
			if d.Name == nil || d.DataType == nil || !elementary(d.DataType.String()) {
				continue
			}
			out = append(out, watch.Variable{Name: d.Name.Value, Path: []string{d.Name.Value}, Type: strings.ToUpper(d.DataType.String())})
		}
	}
	types := map[string]*ast.ProgramDeclaration{}
	for _, st := range program.Statements {
		if pd, ok := st.(*ast.ProgramDeclaration); ok && pd.Name != nil {
			types[strings.ToUpper(pd.Name.Value)] = pd
		}
	}
	for _, r := range cd.Resources {
		for _, pc := range r.Programs {
			pd := types[strings.ToUpper(pc.TypeName.Value)]
			if pd == nil || pc.InstanceName == nil {
				continue
			}
			prefix := []string{r.Name.Value, pc.InstanceName.Value}
			for _, v := range watch.Variables(program, pd, o) {
				out = append(out, watch.Variable{Name: strings.Join(prefix, ".") + "." + v.Name,
					Path: append(append([]string{}, prefix...), v.Path...), Type: v.Type})
			}
		}
	}
	return out
}

// resourceGlobals are the VAR_GLOBAL blocks of cd's resources.
func resourceGlobals(cd *ast.ConfigurationDeclaration) []*ast.GlobalVarDeclaration {
	var out []*ast.GlobalVarDeclaration
	for _, r := range cd.Resources {
		out = append(out, r.GlobalVars...)
	}
	return out
}

// isElementary reports whether t names an elementary type.
func isElementary(t string) bool {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT",
		"REAL", "LREAL", "BYTE", "WORD", "DWORD", "LWORD", "STRING", "WSTRING",
		"TIME", "LTIME", "DATE", "TOD", "TIME_OF_DAY", "DT", "DATE_AND_TIME", "CHAR", "WCHAR":
		return true
	}
	return false
}

// newHash is an empty structure value.
func newHash() *object.Hash { return &object.Hash{Pairs: map[object.HashKey]object.HashPair{}} }

// putHash sets a member of a structure value.
func putHash(h *object.Hash, name string, v object.Object) {
	key := &object.String{Value: name}
	h.Pairs[key.HashKey()] = object.HashPair{Key: key, Value: v}
}
