/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package watch

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
)

const source = `
TYPE Pt : STRUCT x : INT; y : REAL; END_STRUCT; END_TYPE
FUNCTION_BLOCK Base
VAR_OUTPUT bq : BOOL; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK Inner EXTENDS Base
VAR_INPUT i : INT; END_VAR
VAR_OUTPUT o : INT; last : Pt; END_VAR
VAR hidden : INT; tSample : TON; END_VAR
tSample(IN := TRUE, PT := T#1s);
o := i * 2; hidden := 7; bq := TRUE; last.x := o;
END_FUNCTION_BLOCK
PROGRAM P
VAR_OUTPUT done : BOOL; END_VAR
VAR b : Inner; t : TON; pt : Pt; list : ARRAY[1..2] OF INT; END_VAR
b(i := 3); t(IN := TRUE, PT := T#1s); pt.x := 4;
END_PROGRAM
`

func parse(t *testing.T) (*ast.Program, *ast.ProgramDeclaration) {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, s := range program.Statements {
		if pd, ok := s.(*ast.ProgramDeclaration); ok {
			return program, pd
		}
	}
	t.Fatal("no program")
	return nil, nil
}

func names(vars []Variable) string {
	var out []string
	for _, v := range vars {
		out = append(out, v.Name+":"+v.Type)
	}
	return strings.Join(out, " ")
}

func TestVariables(t *testing.T) {
	program, decl := parse(t)
	for _, tc := range []struct {
		name string
		o    Options
		want string
	}{
		{"outputs", Options{},
			"done:BOOL b.bq:BOOL b.o:INT b.last.x:INT b.last.y:REAL t.Q:BOOL t.ET:TIME pt.x:INT pt.y:REAL"},
		{"depth 1", Options{Depth: 1},
			"done:BOOL b.bq:BOOL b.o:INT t.Q:BOOL t.ET:TIME pt.x:INT pt.y:REAL"},
		{"all", Options{Members: All},
			"done:BOOL b.bq:BOOL b.i:INT b.o:INT b.last.x:INT b.last.y:REAL b.hidden:INT " +
				"b.tSample.IN:BOOL b.tSample.PT:TIME b.tSample.Q:BOOL b.tSample.ET:TIME " +
				"t.IN:BOOL t.PT:TIME t.Q:BOOL t.ET:TIME pt.x:INT pt.y:REAL"},
	} {
		if got := names(Variables(program, decl, tc.o)); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

// The same names read the same values on both engines.
func TestEvaluatorAndVM(t *testing.T) {
	object.FinalizeBuiltins()
	program, decl := parse(t)
	vars := Variables(program, decl, Options{Members: All})
	want := map[string]string{"b.bq": "true", "b.o": "6", "b.last.x": "6", "b.hidden": "7", "b.tSample.PT": "T#1s", "pt.x": "4", "t.IN": "true"}

	env := object.NewEnvironment()
	evaluator.Eval(program, env)
	evaluator.Eval(parser.New(lexer.New("P();")).ParseProgram(), env)
	p, _ := env.Get("P")
	got := map[string]string{}
	for _, v := range vars {
		if o, ok := Evaluator(p.(*object.Program).Env, v); ok {
			got[v.Name] = o.Inspect()
		}
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("evaluator %s = %q, want %q", k, got[k], w)
		}
	}

	program, decl = parse(t)
	compiled, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, decl.Name.Value)
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
	for _, v := range vars {
		o, ok := VM(globals, slots, v)
		if !ok {
			t.Errorf("vm %s: not found", v.Name)
			continue
		}
		if w, check := want[v.Name]; check && o.Inspect() != w {
			t.Errorf("vm %s = %q, want %q", v.Name, o.Inspect(), w)
		}
	}
}

func TestMemberMissing(t *testing.T) {
	inst := &object.FunctionBlockInstance{Env: object.NewEnclosedEnvironment(object.NewEnvironment())}
	inst.Env.Outer().Set("q", &object.Boolean{Value: true})
	if _, ok := Member(inst, []string{"q"}); ok {
		t.Error("a member found in an outer scope")
	}
	if _, ok := Member(&object.Boolean{}, []string{"x"}); ok {
		t.Error("a member of a BOOL")
	}
}
