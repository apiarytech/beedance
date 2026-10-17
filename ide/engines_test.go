/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ide

import (
	"maps"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/vm"
)

// A diagram's watches are names the engines have: the evaluator (the
// simulator) and the VM (the target) show the same diagram alike after a
// scan, every watched name read on both.
func TestLiveOnEngines(t *testing.T) {
	src := strings.Replace(ldSrc, "VAR Start, Stop, Run, Done, Fault : BOOL; Mode : INT; END_VAR",
		"VAR Start : BOOL := TRUE; Stop, Run, Done, Fault : BOOL; Mode : INT := 2; END_VAR", 1)
	d := Analyze(src).Diagrams()[0]
	object.FinalizeBuiltins()

	// The evaluator.
	program := parser.New(lexer.New(src)).ParseProgram()
	env := object.NewEnvironment()
	evaluator.Eval(program, env)
	evaluator.Eval(parser.New(lexer.New("Lad();")).ParseProgram(), env)
	p, _ := env.Get("Lad")
	onEval, err := d.Live(FromEvaluator(p.(*object.Program).Env), "")
	if err != nil {
		t.Fatal(err)
	}

	// The VM.
	program = parser.New(lexer.New(src)).ParseProgram()
	compiled, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, "Lad")
	if err != nil {
		t.Fatal(err)
	}
	globals := make([]object.Object, vm.GlobalsSize)
	if err := vm.NewWithGlobalsStore(compiled.InitBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	if err := vm.NewWithGlobalsStore(compiled.CyclicBytecode, globals).Run(); err != nil {
		t.Fatal(err)
	}
	slots := map[string]int{}
	for _, v := range compiled.Variables {
		slots[strings.ToUpper(v.Name)] = v.Global
	}
	onVM, err := d.Live(FromVM(globals, slots), "")
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range d.Watches() {
		if _, ok := onVM.Values[n]; !ok {
			t.Errorf("%s is watched but the VM has no such name", n)
		}
		if _, ok := onEval.Values[n]; !ok {
			t.Errorf("%s is watched but the evaluator has no such name", n)
		}
	}
	if onVM.Values["Run"] != "TRUE" || onVM.Values["Fault"] != "TRUE" {
		t.Errorf("after a scan: Run = %s, Fault = %s; want both TRUE (sealed in; Mode = 2 sets Fault)", onVM.Values["Run"], onVM.Values["Fault"])
	}
	if !maps.Equal(onVM.Values, onEval.Values) || !maps.Equal(onVM.Exprs, onEval.Exprs) {
		t.Errorf("the engines disagree:\nVM        %v %v\nevaluator %v %v", onVM.Values, onVM.Exprs, onEval.Values, onEval.Exprs)
	}
}
