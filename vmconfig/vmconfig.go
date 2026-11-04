/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package vmconfig runs an IEC 61131-3 CONFIGURATION on the VM (Table 62
// and 63): its resources, their tasks, and the program instances the tasks
// run.
//
//	CONFIGURATION CELL
//	    VAR_GLOBAL g : INT := 9; cg : INT; END_VAR
//	    RESOURCE R1 ON PLC
//	        VAR_GLOBAL rg : INT := 7; END_VAR
//	        TASK FAST(INTERVAL := T#10ms, PRIORITY := 1);
//	        PROGRAM P1 WITH FAST : MAIN(k := g, out => cg);
//	    END_RESOURCE
//	    VAR_CONFIG R1.P1.x AT %IW10 : INT; END_VAR
//	END_CONFIGURATION
//
// Each program instance is its program compiled as a unit, with VMs of its
// own. The instances share the globals of the configuration and of its
// resources (one scope: two resources may not declare the same name) and
// one process image (the I/O of located variables). Tasks run one at a
// time, so an instance's globals are copied in before it runs and out
// after.
//
// A cycle runs, resource by resource, the tasks that are due in the order
// of their PRIORITY (0 first), each task's programs in declaration order:
//   - a task with INTERVAL when that much time has passed since it last ran;
//   - a task with SINGLE on each rising edge of its global;
//   - a task with neither, and the programs with no task (in a background
//     task after the others), every cycle.
//
// A connected input, MAIN(k := g), reads its global before each run of the
// instance; one connected to a constant, MAIN(k := 5), starts with it. A
// connected output, MAIN(out => cg), writes its global after each run.
// VAR_CONFIG gives an instance's variables (or members of its function
// block instances) their initial values, and its variables their
// addresses, %I* ones included.
package vmconfig

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

// Configuration is a loaded CONFIGURATION, ready to run.
type Configuration struct {
	Name      string
	Resources []*Resource
	// IO is the process image every instance shares, keyed by address
	// (%IX0.0). A host sets inputs in it before Cycle and reads outputs
	// after.
	IO map[string]object.Object
	// allowInputWrites: LoadOptions.AllowInputWrites (IEC-69).
	allowInputWrites bool
	globals          map[string]object.Object // by upper-case name
	names            map[string]string        // the declared name, by upper-case name
	now              time.Duration
}

// Resource is a RESOURCE of a configuration.
type Resource struct {
	Name      string
	Tasks     []*Task // in the order they run: by PRIORITY, then declaration
	Instances []*Instance
	// background are the instances that have no task.
	background []*Instance
}

// Task is a TASK of a resource.
type Task struct {
	Name      string
	Priority  int64
	Interval  time.Duration // 0 when the task has none
	Single    string        // the global whose rising edge triggers it, or ""
	Instances []*Instance
	ran       bool
	last      time.Duration
	prev      bool // the SINGLE global's value at the last cycle
}

// Instance is a program instance, PROGRAM P1 WITH T1 : MAIN.
type Instance struct {
	Name, Type string
	Task       *Task // nil for one with no task
	config     *Configuration
	resource   *Resource
	compiled   *compiler.CompiledProgram
	scan       *vm.VM
	globals    []object.Object
	slots      map[string]int    // a variable's slot by upper-case name
	addresses  map[string]string // a located variable's address by upper-case name
	shared     map[string]int    // a configuration global's slot by upper-case name
	inputs     []connection
	outputs    []connection
}

// connection is an input or output of an instance connected to a global.
type connection struct {
	slot   int
	global string // upper-case
}

// port is an input or output a configuration connects, by name.
type port struct{ name, global string }

// LoadOptions choose how LoadSourceWith compiles.
type LoadOptions struct {
	// AllowInputWrites lets a POU assign its own VAR_INPUT variables, as
	// CODESYS does (IEC-69); off, IEC 61131-3's rule refuses it.
	AllowInputWrites bool
	// Name is the CONFIGURATION to load, of a source that declares
	// several; empty is the first.
	Name string
}

// LoadSource parses source and loads its CONFIGURATION (the first, if
// there are several), compiling each program instance and running its
// initialization.
func LoadSource(source string) (*Configuration, error) {
	return LoadSourceWith(source, LoadOptions{})
}

// LoadSourceWith is LoadSource with options.
func LoadSourceWith(source string, opt LoadOptions) (*Configuration, error) {
	object.FinalizeBuiltins()
	base, err := parse(source)
	if err != nil {
		return nil, err
	}
	configIndex := -1
	var config *ast.ConfigurationDeclaration
	for i, s := range base.Statements {
		if cd, ok := s.(*ast.ConfigurationDeclaration); ok && (opt.Name == "" || cd.Name != nil && strings.EqualFold(cd.Name.Value, opt.Name)) {
			configIndex, config = i, cd
			break
		}
	}
	if config == nil && opt.Name != "" {
		return nil, fmt.Errorf("no CONFIGURATION %s in the source", opt.Name)
	}
	if config == nil {
		return nil, fmt.Errorf("no CONFIGURATION in the source")
	}
	entries, unmatched := config.ResolveConfigVars()
	if len(unmatched) > 0 {
		return nil, fmt.Errorf("VAR_CONFIG path '%s' does not name a variable of a program instance in configuration '%s'", unmatched[0].AccessPath.String(), config.Name.Value)
	}
	c := &Configuration{
		Name:             config.Name.Value,
		IO:               map[string]object.Object{},
		allowInputWrites: opt.AllowInputWrites,
		globals:          map[string]object.Object{},
		names:            map[string]string{},
	}
	for ri, res := range config.Resources {
		r := &Resource{Name: res.Name.Value}
		tasks := map[string]*Task{}
		for _, td := range res.Tasks {
			task, err := newTask(td)
			if err != nil {
				return nil, fmt.Errorf("resource %s: %w", r.Name, err)
			}
			tasks[strings.ToUpper(task.Name)] = task
			r.Tasks = append(r.Tasks, task)
		}
		sort.SliceStable(r.Tasks, func(a, b int) bool { return r.Tasks[a].Priority < r.Tasks[b].Priority })
		for pi, pc := range res.Programs {
			var mine []*ast.ConfigVarEntry
			for _, e := range entries {
				if e.Resource == res && e.Program == pc {
					mine = append(mine, e)
				}
			}
			inst, err := c.load(source, configIndex, ri, pi, mine)
			if err != nil {
				return nil, fmt.Errorf("program %s: %w", pc.InstanceName.Value, err)
			}
			inst.resource = r
			r.Instances = append(r.Instances, inst)
			if pc.TaskName == nil {
				r.background = append(r.background, inst)
				continue
			}
			task, ok := tasks[strings.ToUpper(pc.TaskName.Value)]
			if !ok {
				return nil, fmt.Errorf("program %s names task %s, which resource %s does not declare", inst.Name, pc.TaskName.Value, r.Name)
			}
			inst.Task = task
			task.Instances = append(task.Instances, inst)
		}
		c.Resources = append(c.Resources, r)
	}
	return c, nil
}

// parse parses source, failing on a syntax error.
func parse(source string) (*ast.Program, error) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("parse: %s", strings.Join(errs, "; "))
	}
	return program, nil
}

// newTask reads a TASK's settings, which are constants.
func newTask(td *ast.TaskDeclaration) (*Task, error) {
	task := &Task{Name: td.Name.Value}
	if td.Priority != nil {
		n, _, ok := object.GetIntegerObjectValue(constant(td.Priority))
		if !ok || n < 0 {
			return nil, fmt.Errorf("task %s: PRIORITY must be a non-negative constant, got %s", task.Name, td.Priority.String())
		}
		task.Priority = n
	}
	if td.Interval != nil {
		d, ok := constant(td.Interval).(*object.Time)
		if !ok || d.Value <= 0 {
			return nil, fmt.Errorf("task %s: INTERVAL must be a positive TIME constant, got %s", task.Name, td.Interval.String())
		}
		task.Interval = d.Value
	}
	if td.Single != nil {
		ident, ok := td.Single.(*ast.Identifier)
		if !ok {
			return nil, fmt.Errorf("task %s: SINGLE must name a global variable, got %s", task.Name, td.Single.String())
		}
		task.Single = strings.ToUpper(ident.Value)
	}
	return task, nil
}

// constant evaluates a constant expression of a task's settings.
func constant(e ast.Expression) object.Object {
	return evaluator.Eval(e, object.NewEnvironment())
}

// load compiles and initializes the program instance pi of resource ri of
// the configuration at configIndex, from a fresh parse of source, which it
// changes to give the instance its VAR_CONFIG entries and constant inputs.
func (c *Configuration) load(source string, configIndex, ri, pi int, entries []*ast.ConfigVarEntry) (*Instance, error) {
	fresh, err := parse(source)
	if err != nil {
		return nil, err
	}
	pc := fresh.Statements[configIndex].(*ast.ConfigurationDeclaration).Resources[ri].Programs[pi]
	var pd *ast.ProgramDeclaration
	for _, s := range fresh.Statements {
		if d, ok := s.(*ast.ProgramDeclaration); ok && d.Name != nil && strings.EqualFold(d.Name.Value, pc.TypeName.Value) {
			pd = d
		}
	}
	if pd == nil {
		return nil, fmt.Errorf("no PROGRAM %s", pc.TypeName.Value)
	}
	if err := configure(pd, entries); err != nil {
		return nil, err
	}
	var inputs, outputs []port
	for _, param := range pc.Parameters {
		switch p := param.(type) {
		case *ast.NamedArgument:
			decl := find(p.Name.Value, pd.VarInputs, pd.VarInOuts)
			if decl == nil {
				return nil, fmt.Errorf("%s has no input %s", pd.Name.Value, p.Name.Value)
			}
			if g, isGlobal := p.Value.(*ast.Identifier); isGlobal {
				inputs = append(inputs, port{decl.Name.Value, strings.ToUpper(g.Value)})
			} else {
				decl.Value = p.Value // a constant input starts with its value
			}
		case *ast.OutputArgument:
			if find(p.Source.Value, pd.VarOutputs) == nil {
				return nil, fmt.Errorf("%s has no output %s", pd.Name.Value, p.Source.Value)
			}
			g, isGlobal := p.Target.(*ast.Identifier)
			if !isGlobal {
				return nil, fmt.Errorf("the output %s must connect to a global variable, got %s", p.Source.Value, p.Target.String())
			}
			outputs = append(outputs, port{p.Source.Value, strings.ToUpper(g.Value)})
		}
	}

	comp := compiler.NewCompilerWithBuiltins(object.Builtins)
	comp.AllowInputWrites = c.allowInputWrites // IEC-69
	compiled, err := comp.CompileProgramUnit(fresh, pd.Name.Value)
	if err != nil {
		return nil, err
	}
	inst := &Instance{
		Name:      pc.InstanceName.Value,
		Type:      pd.Name.Value,
		config:    c,
		compiled:  compiled,
		globals:   make([]object.Object, vm.GlobalsSize),
		slots:     map[string]int{},
		addresses: map[string]string{},
		shared:    map[string]int{},
	}
	initVM := vm.NewWithGlobalsStore(compiled.InitBytecode, inst.globals)
	inst.scan = vm.NewWithGlobalsStore(compiled.CyclicBytecode, inst.globals)
	for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return c.now }) {
		initVM.SetBuiltin(idx, b)
		inst.scan.SetBuiltin(idx, b)
	}
	initVM.ShareIO(c.IO)
	if err := initVM.Run(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	inst.scan.ShareIO(c.IO)

	for _, v := range compiled.Variables {
		if v.Address != "" {
			inst.addresses[strings.ToUpper(v.Name)] = v.Address
		} else if v.Global >= 0 {
			inst.slots[strings.ToUpper(v.Name)] = v.Global
		}
	}
	for _, g := range compiled.ConfigGlobals {
		if g.Global < 0 {
			continue // a located global lives in the process image
		}
		name := strings.ToUpper(g.Name)
		inst.shared[name] = g.Global
		if _, ok := c.globals[name]; !ok {
			c.globals[name] = inst.globals[g.Global] // its initial value
			c.names[name] = g.Name
		}
	}
	connect := func(ports []port, kind string) ([]connection, error) {
		var out []connection
		for _, p := range ports {
			slot, ok := inst.slots[strings.ToUpper(p.name)]
			if !ok {
				return nil, fmt.Errorf("the %s %s has no slot", kind, p.name)
			}
			if _, isGlobal := inst.shared[p.global]; !isGlobal {
				return nil, fmt.Errorf("the %s %s: %s is not a global variable of the configuration", kind, p.name, p.global)
			}
			out = append(out, connection{slot, p.global})
		}
		return out, nil
	}
	if inst.inputs, err = connect(inputs, "input"); err != nil {
		return nil, err
	}
	if inst.outputs, err = connect(outputs, "output"); err != nil {
		return nil, err
	}
	return inst, nil
}

// configure gives the program's variables the initial values and
// addresses of VAR_CONFIG entries: a variable's (R1.P1.x), or a member's of
// one of its function block instances (R1.P1.t.PT).
func configure(pd *ast.ProgramDeclaration, entries []*ast.ConfigVarEntry) error {
	for _, e := range entries {
		decl := find(e.Path[0], pd.Vars, pd.VarInputs, pd.VarOutputs)
		if decl == nil {
			return fmt.Errorf("VAR_CONFIG %s: %s has no variable %s", e.Decl.AccessPath.String(), pd.Name.Value, e.Path[0])
		}
		switch len(e.Path) {
		case 1:
			if e.Decl.Location != nil {
				decl.Location = e.Decl.Location
			}
			if e.Decl.Value != nil {
				decl.Value = e.Decl.Value
			}
		case 2:
			if e.Decl.Location != nil {
				return fmt.Errorf("VAR_CONFIG %s: a member of an instance cannot be located", e.Decl.AccessPath.String())
			}
			if e.Decl.Value == nil {
				continue
			}
			init, _ := decl.Value.(*ast.StructLiteral)
			if init == nil {
				init = &ast.StructLiteral{Token: e.Decl.Token}
				decl.Value = init
			}
			name := &ast.Identifier{Token: e.Decl.Token, Value: e.Path[1]}
			init.Initializers = append(init.Initializers, &ast.NamedArgument{Token: e.Decl.Token, Name: name, Value: e.Decl.Value})
		default:
			return fmt.Errorf("VAR_CONFIG %s: a path deeper than an instance's member is not supported", e.Decl.AccessPath.String())
		}
	}
	return nil
}

// find returns the variable called name of blocks, or nil.
func find(name string, blocks ...[]*ast.VarDeclStatement) *ast.VarDeclStatement {
	for _, block := range blocks {
		for _, d := range block {
			if d.Name != nil && strings.EqualFold(d.Name.Value, name) {
				return d
			}
		}
	}
	return nil
}

// Cycle runs the tasks that are due at now, the time since the
// configuration started, which the standard timers also read.
func (c *Configuration) Cycle(now time.Duration) error {
	cy := c.Begin(now)
	for inst := cy.Next(); inst != nil; inst = cy.Next() {
		inst.In()
		if err := inst.VM().Run(); err != nil {
			return cy.Wrap(fmt.Errorf("program %s: %w", inst.Name, err))
		}
		inst.Out()
	}
	return nil
}

// A CycleRun is one cycle of the configuration that a host runs instance
// by instance, as a debugger does to stop inside one: Next gives each
// instance due in turn, in Cycle's order; the host calls the instance's
// In, runs its VM to the end, and calls its Out before the next.
type CycleRun struct {
	c        *Configuration
	ri, ti   int // the resource, and the task in it (len(Tasks): its background)
	ii       int // the next instance of that task
	resource *Resource
	task     *Task // the task of the last instance Next gave; nil in a background task
}

// Begin starts the cycle at now, the time since the configuration
// started, which the standard timers also read.
func (c *Configuration) Begin(now time.Duration) *CycleRun {
	c.now = now
	return &CycleRun{c: c, ti: -1}
}

// Next is the next instance due in the cycle, nil at its end. A task's
// INTERVAL or SINGLE trigger is read when the cycle reaches it, after the
// tasks before it have run, as Cycle reads it.
func (cy *CycleRun) Next() *Instance {
	for cy.ri < len(cy.c.Resources) {
		r := cy.c.Resources[cy.ri]
		cy.resource = r
		var list []*Instance
		switch {
		case cy.ti >= 0 && cy.ti < len(r.Tasks):
			list = r.Tasks[cy.ti].Instances
		case cy.ti == len(r.Tasks):
			list = r.background
		}
		if cy.ti >= 0 && cy.ii < len(list) {
			cy.ii++
			if cy.ti < len(r.Tasks) {
				cy.task = r.Tasks[cy.ti]
			} else {
				cy.task = nil
			}
			return list[cy.ii-1]
		}
		// The next task of the resource that is due, or its background.
		cy.ii = 0
		for cy.ti++; cy.ti < len(r.Tasks) && !cy.c.due(r.Tasks[cy.ti]); cy.ti++ {
		}
		if cy.ti > len(r.Tasks) {
			cy.ri, cy.ti = cy.ri+1, -1
		}
	}
	return nil
}

// Task is the task of the instance Next gave last; nil for one with no
// task.
func (cy *CycleRun) Task() *Task { return cy.task }

// Resource is the resource of the instance Next gave last.
func (cy *CycleRun) Resource() *Resource { return cy.resource }

// Wrap says where in the cycle err happened, as Cycle's errors do.
func (cy *CycleRun) Wrap(err error) error {
	if cy.resource == nil {
		return err
	}
	if cy.task == nil {
		return fmt.Errorf("resource %s: %w", cy.resource.Name, err)
	}
	return fmt.Errorf("resource %s, task %s: %w", cy.resource.Name, cy.task.Name, err)
}

// due reports whether a task runs in the cycle at c.now, and records it.
func (c *Configuration) due(task *Task) bool {
	switch {
	case task.Single != "":
		value := object.IsTruthy(c.globals[task.Single])
		edge := value && !task.prev
		task.prev = value
		return edge
	case task.Interval > 0:
		if task.ran && c.now-task.last < task.Interval {
			return false
		}
		task.ran, task.last = true, c.now
		return true
	}
	return true
}

// In starts a run of the instance: the shared globals and its connected
// inputs in, its VM at the start of its body.
func (i *Instance) In() {
	c := i.config
	for name, slot := range i.shared {
		i.globals[slot] = c.globals[name]
	}
	for _, in := range i.inputs {
		i.globals[in.slot] = object.CopyValue(c.globals[in.global])
	}
	i.scan.Reset()
}

// Out ends a run of the instance: the shared globals and its connected
// outputs out.
func (i *Instance) Out() {
	c := i.config
	for name, slot := range i.shared {
		c.globals[name] = i.globals[slot]
	}
	for _, out := range i.outputs {
		c.globals[out.global] = object.CopyValue(i.globals[out.slot])
	}
}

// VM is the VM that runs the instance's body; In resets it.
func (i *Instance) VM() *vm.VM { return i.scan }

// Compiled is the instance's program as compiled.
func (i *Instance) Compiled() *compiler.CompiledProgram { return i.compiled }

// Store is the instance's globals store, which its VM and Compiled's
// variables' slots index.
func (i *Instance) Store() []object.Object { return i.globals }

// Resource is the resource the instance is declared in.
func (i *Instance) Resource() *Resource { return i.resource }

// Path is the instance's name under its resource, "R1.P1".
func (i *Instance) Path() string { return i.resource.Name + "." + i.Name }

// SharedSlot is the slot of a configuration global in the instance's
// store, by name; false if the instance does not share it.
func (i *Instance) SharedSlot(name string) (int, bool) {
	slot, ok := i.shared[strings.ToUpper(name)]
	return slot, ok
}

// Run runs cycles every period on the wall clock until ctx ends, and
// returns the first error.
func (c *Configuration) Run(ctx context.Context, period time.Duration) error {
	start := time.Now()
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		if err := c.Cycle(time.Since(start)); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Global returns the value of a configuration's or resource's global.
func (c *Configuration) Global(name string) (object.Object, bool) {
	v, ok := c.globals[strings.ToUpper(name)]
	return v, ok
}

// SetGlobal sets a global, as a host (an HMI, a SINGLE trigger) does
// between cycles. It reports false if there is no such global.
func (c *Configuration) SetGlobal(name string, v object.Object) bool {
	key := strings.ToUpper(name)
	if _, ok := c.globals[key]; !ok {
		return false
	}
	c.globals[key] = v
	return true
}

// Instance returns the program instance a path names, "R1.P1", or nil.
func (c *Configuration) Instance(path string) *Instance {
	res, name, ok := strings.Cut(path, ".")
	if !ok {
		return nil
	}
	for _, r := range c.Resources {
		if !strings.EqualFold(r.Name, res) {
			continue
		}
		for _, inst := range r.Instances {
			if strings.EqualFold(inst.Name, name) {
				return inst
			}
		}
	}
	return nil
}

// Value returns the value of a variable of the instance, or of a global it
// shares, as of the end of its last run.
func (i *Instance) Value(name string) (object.Object, bool) {
	key := strings.ToUpper(name)
	if slot, ok := i.slots[key]; ok {
		return i.globals[slot], i.globals[slot] != nil
	}
	if addr, ok := i.addresses[key]; ok {
		v, found := i.scan.IO()[addr]
		return v, found
	}
	if slot, ok := i.shared[key]; ok {
		return i.globals[slot], i.globals[slot] != nil
	}
	return nil, false
}

// GlobalNames returns the names of the configuration's globals, as declared.
func (c *Configuration) GlobalNames() []string {
	names := make([]string, 0, len(c.names))
	for _, name := range c.names {
		names = append(names, name)
	}
	return names
}
