/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package native_test

import (
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/native"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/vm"
)

// Socket is a stand-in for a TwinCAT-style handle structure.
type Socket struct {
	HANDLE uint32
	PEER   string
}

// adder is a Go function block: it adds A and B, counts its runs, and
// hands out a handle structure, as FB_SocketConnect hands out hSocket.
type adder struct {
	A, B  int16 // INT
	SUM   int16
	CALLS uint32 // UDINT
	SOCK  Socket
	TS    time.Duration // the time of the last run since start, a TIME
}

var start = time.Unix(1000, 0)

func (a *adder) Execute(now time.Time) {
	a.SUM = a.A + a.B
	a.CALLS++
	a.SOCK = Socket{HANDLE: 7, PEER: "plc"}
	a.TS = now.Sub(start)
}

func registry(t *testing.T, now *time.Time) *native.Registry {
	t.Helper()
	r, err := native.NewRegistry(func() time.Time { return *now },
		"TYPE Socket : STRUCT HANDLE : UDINT; PEER : STRING; END_STRUCT; END_TYPE",
		native.Block{
			Name:    "Adder",
			Inputs:  []native.Field{{Name: "A", Type: "INT"}, {Name: "B", Type: "INT"}},
			Outputs: []native.Field{{Name: "SUM", Type: "INT"}, {Name: "CALLS", Type: "UDINT"}, {Name: "SOCK", Type: "Socket"}, {Name: "TS", Type: "TIME"}},
			New:     func() native.Instance { return &adder{} },
		})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const body = `VAR a : Adder; s : INT; n : UDINT; h : UDINT; when : TIME; END_VAR
a(A := 2, B := 3);
s := a.SUM; n := a.CALLS; h := a.SOCK.HANDLE; when := a.TS;
`

func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v\n%s", errs, src)
	}
	return prog
}

// TestNativeBlockOnVM runs a program calling a native block for two scans,
// as a host such as beehive runs one: the instance keeps its state.
func TestNativeBlockOnVM(t *testing.T) {
	object.FinalizeBuiltins()
	now := start.Add(1500 * time.Millisecond)
	r := registry(t, &now)

	src := r.Source() + "PROGRAM Main\n" + body + "END_PROGRAM\n"
	prog, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(parse(t, src), "Main")
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, src)
	}
	globals := make([]object.Object, vm.GlobalsSize)
	init := vm.NewWithGlobalsStore(prog.InitBytecode, globals)
	scan := vm.NewWithGlobalsStore(prog.CyclicBytecode, globals)
	for idx, b := range r.Builtins() {
		init.SetBuiltin(idx, b)
		scan.SetBuiltin(idx, b)
	}
	if err := init.Run(); err != nil {
		t.Fatalf("init: %v", err)
	}
	for i := 0; i < 2; i++ {
		scan.Reset()
		if err := scan.Run(); err != nil {
			t.Fatalf("scan %d: %v", i+1, err)
		}
	}
	got := map[string]string{}
	for _, v := range prog.Variables {
		if v.Global >= 0 && globals[v.Global] != nil {
			got[v.Name] = globals[v.Global].Inspect()
		}
	}
	for name, want := range map[string]string{"s": "5", "n": "2", "h": "7", "when": "1.5s"} {
		if !strings.Contains(got[name], want) {
			t.Errorf("%s = %q, want %s", name, got[name], want)
		}
	}
}

func TestNativeBlockInEvaluator(t *testing.T) {
	now := start.Add(1500 * time.Millisecond)
	r := registry(t, &now)
	env := object.NewEnvironment()
	r.Bind(env)
	if res := evaluator.Eval(parse(t, r.Source()+body), env); res != nil && res.Type() == object.ERROR_OBJ {
		t.Fatalf("eval: %s", res.Inspect())
	}
	for name, want := range map[string]string{"s": "5", "n": "1", "h": "7", "when": "1.5s"} {
		v, _ := env.Get(name)
		if v == nil || !strings.Contains(v.Inspect(), want) {
			t.Errorf("%s = %v, want %s", name, v, want)
		}
	}
}

// TestNativeBlocksAreAllowedOnly checks that a program reaches only the
// blocks its host allows.
func TestNativeBlocksAreAllowedOnly(t *testing.T) {
	now := start
	r := registry(t, &now)

	// A block the registry does not hold, called through a hand-written
	// wrapper, is refused.
	env := object.NewEnvironment()
	r.Bind(env)
	res := evaluator.Eval(parse(t, `FUNCTION_BLOCK Evil VAR h : DINT; END_VAR h := __NATIVE_NEW(h, 'Evil'); END_FUNCTION_BLOCK
VAR e : Evil; END_VAR e();`), env)
	if res == nil || !strings.Contains(res.Inspect(), "not allowed") {
		t.Errorf("a block outside the registry ran: %v", res)
	}

	// Without a registry, the built-ins refuse everything.
	res = evaluator.Eval(parse(t, r.Source()+body), object.NewEnvironment())
	if res == nil || !strings.Contains(res.Inspect(), "allows no native function blocks") {
		t.Errorf("a native block ran without a registry: %v", res)
	}

	if _, err := native.NewRegistry(nil, "", native.Block{Name: "X"}); err == nil {
		t.Error("a block without New was accepted")
	}
}
