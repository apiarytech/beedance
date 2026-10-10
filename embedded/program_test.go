/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package embedded

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	beeparser "github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib"
)

// A program with what a real one has: function blocks calling function
// blocks, a standard timer, a function, an array, REAL constants that %f
// would round, and a located output.
const deviceProgram = `
FUNCTION_BLOCK Inner
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT; END_VAR
VAR t : TON; END_VAR
t(IN := x > 0, PT := T#1s);
y := x * 2;
END_FUNCTION_BLOCK

FUNCTION_BLOCK Middle
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT; END_VAR
VAR i : Inner; END_VAR
i(x := x + 1);
y := i.y;
END_FUNCTION_BLOCK

FUNCTION Square : INT
VAR_INPUT v : INT; END_VAR
Square := v * v;
END_FUNCTION

PROGRAM Main
VAR
	m : Middle;
	k : INT;
	arr : ARRAY[1..3] OF INT := [10, 20, 30];
	gain : LREAL := 0.000000001;
	scans : INT;
	ok AT %QX0.0 : BOOL;
END_VAR
scans := scans + 1;
FOR k := 1 TO 3 DO
	m(x := k);
	arr[k] := Square(m.y);
END_FOR;
ok := arr[3] = 64 AND gain > 0.0;
END_PROGRAM
`

func compileUnit(t *testing.T, src, name string) *compiler.CompiledProgram {
	t.Helper()
	object.FinalizeBuiltins()
	p := beeparser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %s", strings.Join(errs, "; "))
	}
	cp, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, name)
	if err != nil {
		t.Fatal(err)
	}
	return cp
}

// goRun builds and runs a generated main package from inside this module,
// so it imports this beedance.
func goRun(t *testing.T, src []byte) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain")
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("testdata", "gen-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goTool, "run", "./"+filepath.ToSlash(dir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\n--- generated:\n%s", err, out, src)
	}
	return string(out)
}

func TestGenerateEmbeddedProgramRuns(t *testing.T) {
	cp := compileUnit(t, deviceProgram, "Main")
	var out bytes.Buffer
	if err := GenerateEmbeddedProgram(&out, cp, ProgramOptions{Interval: time.Millisecond, Scans: 5}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1e-09") {
		t.Errorf("LREAL 0.000000001 not kept exactly")
	}
	if got := goRun(t, out.Bytes()); !strings.Contains(got, "program done") {
		t.Fatalf("output %q", got)
	}
}

// Run's onScan sees the I/O image after each scan.
func TestGenerateEmbeddedProgramPackage(t *testing.T) {
	cp := compileUnit(t, deviceProgram, "Main")
	var out bytes.Buffer
	if err := GenerateEmbeddedProgram(&out, cp, ProgramOptions{Package: "main", Interval: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	// A firmware's own main, around Run.
	src := strings.Replace(out.String(), "\nfunc main() {", "\nfunc unusedMain() {", 1) + `
func main() {
	n := 0
	err := Run(3, func(machine *vm.VM) {
		if q, ok := machine.IO()["%QX0.0"].(*object.Boolean); ok && q.Value {
			n++
		}
	})
	println("err", err == nil, "ok scans", n)
}
`
	if got := goRun(t, []byte(src)); !strings.Contains(got, "err true ok scans 3") {
		t.Fatalf("output %q", got)
	}
}

func TestGenerateEmbeddedProgramRefuses(t *testing.T) {
	var out bytes.Buffer
	if err := GenerateEmbeddedProgram(&out, nil, ProgramOptions{}); err == nil {
		t.Error("nil program accepted")
	}
}

// Constants of every elementary kind become literals that read back exactly.
func TestObjectToGoLiteral(t *testing.T) {
	for _, c := range []struct {
		obj  object.Object
		want string
	}{
		{&object.LReal{Value: 1e-9}, "&object.LReal{Value: 1e-09}"},
		{&object.Real{Value: 2}, "&object.Real{Value: 2.0}"},
		{&object.Int{Value: -3}, "&object.Int{Value: -3}"},
		{&object.Word{Value: 65535}, "&object.Word{Value: 65535}"},
		{&object.Array{LowerBound: 1, Elements: []object.Object{&object.LInt{Value: 1}}},
			"&object.Array{LowerBound: 1, Elements: []object.Object{\n\t\t&object.LInt{Value: 1},\n\t}}"},
		{object.NULL, "vm.Null"},
	} {
		got, err := objectToGoLiteral(c.obj)
		if err != nil || got != c.want {
			t.Errorf("%T: %q, %v; want %q", c.obj, got, err, c.want)
		}
	}
	if _, err := objectToGoLiteral(&object.LReal{Value: posInf()}); err == nil {
		t.Error("an infinite constant accepted")
	}
	if f, _ := floatLiteral(0.1); f != strconv.FormatFloat(0.1, 'g', -1, 64) {
		t.Errorf("0.1 as %s", f)
	}
}

func posInf() float64 {
	f, _ := strconv.ParseFloat("+Inf", 64)
	return f
}
