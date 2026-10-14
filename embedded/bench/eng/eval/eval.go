/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package eval runs the workloads on the tree-walking evaluator, which
// parses them on the device. It needs a goroutine stack of 16 KB or more
// (-stack-size=16KB): more than a Pico's default 8 KB.
package eval

import (
	"fmt"
	"time"

	"github.com/apiarytech/beedance/embedded/bench/evalsrc"
	"github.com/apiarytech/beedance/embedded/bench/report"

	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib" // the built-in functions and standard blocks
)

// Bench runs and reports both workloads.
func Bench() {
	object.FinalizeBuiltins()
	run("plc", evalsrc.Plc)
	run("calc", evalsrc.Calc)
}

// run parses src, declares its program and calls it once per scan.
func run(workload, src string) {
	end := evaluator.Session(time.Now)
	defer end()
	began := time.Now()
	prog := parser.New(lexer.New(src)).ParseProgram()
	call := parser.New(lexer.New("Main();")).ParseProgram()
	env := object.NewEnvironment()
	if out := evaluator.Eval(prog, env); isError(out) {
		fmt.Println("bench: eval error:", out.Inspect())
		return
	}
	setup := time.Since(began)
	t := report.Start()
	pulses, on := 0, false
	for n := 0; n < report.Scans; n++ {
		b := time.Now()
		out := evaluator.Eval(call, env)
		t.Add(time.Since(b))
		if isError(out) {
			fmt.Println("bench: eval error:", out.Inspect())
			return
		}
		q, _ := evaluator.IO()["%QX0.0"].(*object.Boolean)
		if q != nil && q.Value && !on {
			pulses++
		}
		on = q != nil && q.Value
	}
	res := fmt.Sprintf("pulses:%d", pulses)
	if workload == "calc" {
		res = "acc:" + member(env, "acc")
	}
	t.Print("eval", workload, fmt.Sprintf("%s,setup_ms:%d", res, setup.Milliseconds()))
}

func isError(o object.Object) bool { return o != nil && o.Type() == object.ERROR_OBJ }

// member is the value of a variable of the program Main, or "".
func member(env *object.Environment, name string) string {
	o, _ := env.Get("Main")
	if prog, ok := o.(*object.Program); ok {
		if v, ok := prog.Env.Get(name); ok {
			return v.Inspect()
		}
	}
	return ""
}
