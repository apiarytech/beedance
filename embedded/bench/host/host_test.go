/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package host runs the benchmark's workloads on the host: tests that the
// three engines agree, and Go benchmarks of each (go test -bench . ./host).
// Host timings say little about a microcontroller's, where TinyGo compiles
// differently; see the README.
package host

import (
	"fmt"
	"testing"
	"time"

	"github.com/apiarytech/beedance/embedded/bench/evalsrc"
	"github.com/apiarytech/beedance/embedded/bench/gocalc"
	"github.com/apiarytech/beedance/embedded/bench/goplc"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/stdlib"
	bvm "github.com/apiarytech/beedance/vm"
)

// calcAcc is what st/calc.st leaves in acc: the sum of (i*3) MOD 7 for i
// from 1 to 100.
const calcAcc = 303

// scanVM compiles src and returns a VM for its scans, initialized, and the
// compiled program.
func scanVM(t testing.TB, src string) (*bvm.VM, *compiler.CompiledProgram) {
	t.Helper()
	object.FinalizeBuiltins()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	cp, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(prog, "Main")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	clock := stdlib.ClockBuiltins(func() time.Duration { return time.Since(start) })
	globals := make([]object.Object, 64)
	opts := []bvm.Option{bvm.WithStackSize(256), bvm.WithMaxFrames(32)}
	init := bvm.NewWithGlobalsStore(cp.InitBytecode, globals, opts...)
	for i, b := range clock {
		init.SetBuiltin(i, b)
	}
	if err := init.Run(); err != nil {
		t.Fatal(err)
	}
	scan := bvm.NewWithGlobalsStore(cp.CyclicBytecode, globals, opts...)
	for i, b := range clock {
		scan.SetBuiltin(i, b)
	}
	for addr, v := range init.IO() {
		scan.IO()[addr] = v
	}
	return scan, cp
}

// evalProgram declares src's program in a new environment and returns the
// environment and a call of the program; end ends the evaluator's session.
func evalProgram(t testing.TB, src string) (env *object.Environment, call *ast.Program, end func()) {
	t.Helper()
	object.FinalizeBuiltins()
	end = evaluator.Session(time.Now)
	env = object.NewEnvironment()
	if out := evaluator.Eval(parser.New(lexer.New(src)).ParseProgram(), env); out != nil && out.Type() == object.ERROR_OBJ {
		end()
		t.Fatal(out.Inspect())
	}
	return env, parser.New(lexer.New("Main();")).ParseProgram(), end
}

func TestEnginesAgreeOnCalc(t *testing.T) {
	v, cp := scanVM(t, evalsrc.Calc)
	if err := v.Run(); err != nil {
		t.Fatal(err)
	}
	vmAcc := ""
	for _, variable := range cp.Variables {
		if variable.Name == "acc" {
			vmAcc = v.Globals()[variable.Global].Inspect()
		}
	}

	env, call, end := evalProgram(t, evalsrc.Calc)
	defer end()
	if out := evaluator.Eval(call, env); out != nil && out.Type() == object.ERROR_OBJ {
		t.Fatal(out.Inspect())
	}
	evalAcc := ""
	if o, ok := env.Get("Main"); ok {
		if a, ok := o.(*object.Program).Env.Get("acc"); ok {
			evalAcc = a.Inspect()
		}
	}

	g := gocalc.NewMain()
	g.Logic(time.Now())
	goAcc := ""
	for _, variable := range g.Variables() {
		if variable.Name == "acc" {
			goAcc = fmt.Sprint(variable.Get())
		}
	}

	want := fmt.Sprint(calcAcc)
	if vmAcc != want || evalAcc != want || goAcc != want {
		t.Errorf("acc: vm %q, eval %q, go %q; want %s", vmAcc, evalAcc, goAcc, want)
	}
}

// The PLC workload's output pulses once a second plus two scans on every engine.
func TestEnginesPulse(t *testing.T) {
	// Each engine reads the process clock. Pulses come about 1.02 s apart,
	// so 2.5 s of scans holds two, well clear of the third.
	const run = 2500 * time.Millisecond
	countPulses := func(scan func() bool) int {
		pulses, on := 0, false
		for end := time.Now().Add(run); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
			q := scan()
			if q && !on {
				pulses++
			}
			on = q
		}
		return pulses
	}
	v, _ := scanVM(t, evalsrc.Plc)
	vmPulses := countPulses(func() bool {
		v.Reset()
		if err := v.Run(); err != nil {
			t.Fatal(err)
		}
		q, _ := v.IO()["%QX0.0"].(*object.Boolean)
		return q != nil && q.Value
	})

	env, call, end := evalProgram(t, evalsrc.Plc)
	evalPulses := countPulses(func() bool {
		evaluator.Eval(call, env)
		q, _ := evaluator.IO()["%QX0.0"].(*object.Boolean)
		return q != nil && q.Value
	})
	end()

	g := goplc.NewMain()
	vars := g.Variables()
	goPulses := countPulses(func() bool {
		g.Logic(time.Now())
		for _, variable := range vars {
			if variable.Name == "pulse" {
				return fmt.Sprint(variable.Get()) == "true"
			}
		}
		return false
	})

	for engine, n := range map[string]int{"vm": vmPulses, "eval": evalPulses, "go": goPulses} {
		if n != 2 {
			t.Errorf("%s: %d pulses in %v, want 2", engine, n, run)
		}
	}
}

func benchVM(b *testing.B, src string) {
	v, _ := scanVM(b, src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Reset()
		if err := v.Run(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchEval(b *testing.B, src string) {
	env, call, end := evalProgram(b, src)
	defer end()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluator.Eval(call, env)
	}
}

func BenchmarkVMPLC(b *testing.B)    { benchVM(b, evalsrc.Plc) }
func BenchmarkVMCalc(b *testing.B)   { benchVM(b, evalsrc.Calc) }
func BenchmarkEvalPLC(b *testing.B)  { benchEval(b, evalsrc.Plc) }
func BenchmarkEvalCalc(b *testing.B) { benchEval(b, evalsrc.Calc) }

func BenchmarkGoPLC(b *testing.B) {
	p := goplc.NewMain()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Logic(time.Now())
	}
}

func BenchmarkGoCalc(b *testing.B) {
	c := gocalc.NewMain()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Logic(time.Time{})
	}
}
