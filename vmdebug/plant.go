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

// Plant models: the source's PROGRAMs named PLANT_..., which simulate what
// the program controls (a tank's level, a conveyor's load) in Structured
// Text. Each runs after every scan of the program, on the same simulated
// clock and the same I/O image: it reads the outputs (%Q) and writes the
// inputs (%I) through its located variables. It is not debugged: no
// breakpoints stop it. Online change carries its variables, as the
// program's, so a running plant keeps its state while the program is
// tuned.

import (
	"fmt"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
)

// IsPlant reports whether a PROGRAM's name makes it a plant model.
func IsPlant(name string) bool { return strings.HasPrefix(strings.ToUpper(name), "PLANT_") }

// plant is a plant model built for an image.
type plant struct {
	name     string
	compiled *compiler.CompiledProgram
	globals  []object.Object
	vm       *vm.VM
	decls    map[string]compiler.Variable // by upper-case name
}

// buildPlants compiles and initializes the source's plant models on img's
// I/O image. The plant's initial values of its inputs (a tank half full)
// are the image's.
func (s *Session) buildPlants(img *image) error {
	for _, st := range img.program.Statements {
		pd, ok := st.(*ast.ProgramDeclaration)
		if !ok || pd.Name == nil || pd == img.decl || !IsPlant(pd.Name.Value) {
			continue
		}
		comp := compiler.NewCompilerWithBuiltins(object.Builtins)
		comp.AllowInputWrites = s.opts.AllowInputWrites
		compiled, err := comp.CompileProgramUnit(img.program, pd.Name.Value)
		if err != nil {
			return fmt.Errorf("compile %s: %w", pd.Name.Value, err)
		}
		p := &plant{name: pd.Name.Value, compiled: compiled, globals: make([]object.Object, vm.GlobalsSize),
			decls: map[string]compiler.Variable{}}
		initVM := vm.NewWithGlobalsStore(compiled.InitBytecode, p.globals)
		p.vm = vm.NewWithGlobalsStore(compiled.CyclicBytecode, p.globals)
		for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return s.elapsed }) {
			initVM.SetBuiltin(idx, b)
			p.vm.SetBuiltin(idx, b)
		}
		initVM.SetBudget(s.opts.Budget)
		if err := initVM.Run(); err != nil {
			return fmt.Errorf("init %s: %w", p.name, err)
		}
		for addr, v := range initVM.IO() {
			if ast.IsInputAddress(addr) || img.vm.IO()[addr] == nil {
				img.vm.IO()[addr] = v
			}
		}
		p.vm.ShareIO(img.vm.IO())
		p.vm.SetBudget(s.opts.Budget)
		for _, v := range compiled.Variables {
			p.decls[strings.ToUpper(v.Name)] = v
		}
		img.plants = append(img.plants, p)
	}
	return nil
}

// runPlants runs one scan of every plant model, after a scan of the
// program.
func (s *Session) runPlants() error {
	for _, p := range s.img.plants {
		p.vm.Reset()
		if err := p.vm.Run(); err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
	}
	return nil
}

// carryPlants carries the variables of old's plant models to img's of the
// same name, as Change carries the program's.
func carryPlants(old, img *image, c carrier) {
	byName := map[string]*plant{}
	for _, p := range old.plants {
		byName[strings.ToUpper(p.name)] = p
	}
	for _, np := range img.plants {
		op := byName[strings.ToUpper(np.name)]
		if op == nil {
			continue
		}
		for _, nv := range np.compiled.Variables {
			ov, ok := op.decls[strings.ToUpper(nv.Name)]
			if !ok || nv.Constant || nv.Address != "" || nv.Global < 0 || ov.Global < 0 || !strings.EqualFold(nv.Type, ov.Type) {
				continue
			}
			if v, ok := c.carry(op.globals[ov.Global], np.globals[nv.Global], ov.Type, nv.Type); ok {
				np.globals[nv.Global] = v
			}
		}
	}
}

// Plants are the names of the plant models the session runs.
func (s *Session) Plants() []string {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	var out []string
	for _, p := range s.img.plants {
		out = append(out, p.name)
	}
	return out
}

// PlantGlobals are the variables of the plant model i (an index of
// Plants), in declaration order.
func (s *Session) PlantGlobals(i int) []Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.img.plants) {
		return nil
	}
	p := s.img.plants[i]
	var out []Value
	for _, v := range p.compiled.Variables {
		val := Value{Name: v.Name, Type: v.Type, Address: v.Address}
		if v.Address != "" {
			val.Object = s.img.vm.IO()[v.Address]
		} else if v.Global >= 0 && v.Global < len(p.globals) {
			val.Object = p.globals[v.Global]
		}
		out = append(out, val)
	}
	return out
}
