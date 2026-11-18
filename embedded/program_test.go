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

// StackSize and MaxFrames reach the generated program's VMs, which run the
// program in them.
func TestGenerateEmbeddedProgramStackSize(t *testing.T) {
	cp := compileUnit(t, deviceProgram, "Main")
	var out bytes.Buffer
	if err := GenerateEmbeddedProgram(&out, cp, ProgramOptions{Interval: time.Millisecond, Scans: 5, StackSize: 128, MaxFrames: 16}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "vm.WithStackSize(128), vm.WithMaxFrames(16)") {
		t.Errorf("generated VM options missing")
	}
	if got := goRun(t, out.Bytes()); !strings.Contains(got, "program done") {
		t.Fatalf("output %q", got)
	}
}

// A program with a RETAIN counter, a counter that starts over at each
// start, and an input that, when set, makes the scan fail.
const faultProgram = `
PROGRAM Main
VAR RETAIN
	count : DINT;
END_VAR
VAR
	fresh : DINT;
	idx : INT := 1;
	arr : ARRAY[1..2] OF INT;
	bad AT %IX0.0 : BOOL;
	ok AT %QX0.0 : BOOL;
END_VAR
count := count + 1;
fresh := fresh + 1;
IF bad THEN
	arr[idx + 5] := 1;
END_IF;
ok := TRUE;
END_PROGRAM
`

// firmware replaces the generated main with main.
func firmware(t *testing.T, src, main string, o ProgramOptions) string {
	t.Helper()
	o.Package = "main"
	o.Interval = time.Millisecond
	var out bytes.Buffer
	if err := GenerateEmbeddedProgram(&out, compileUnit(t, src, "Main"), o); err != nil {
		t.Fatal(err)
	}
	gen := strings.Replace(out.String(), "\nfunc main() {", "\nfunc unusedMain() {", 1)
	gen = strings.Replace(gen, "import (\n", "import (\n\t\"fmt\"\n", 1)
	return goRun(t, []byte(gen+main))
}

// A flipped instruction, a flipped TRUE and a failing scan each call
// SafeState and restart the program, which goes on from its RETAIN
// variables; a second start, as after a reset, does too.
func TestGenerateEmbeddedProgramRecovers(t *testing.T) {
	got := firmware(t, faultProgram, `
func main() {
	RetainStore = &harden.MemStore{}
	var safe []string
	SafeState = func(err error) { safe = append(safe, err.Error()) }
	dog := 0
	Watchdog = func() { dog++ }
	n := 0
	var count, fresh object.Object
	err := Run(12, func(m *vm.VM) {
		n++
		switch n {
		case 2:
			scanInstructions[0] ^= 0xff
		case 5:
			vm.True.Value = false
		case 8:
			m.IO()["%IX0.0"] = vm.True
		}
		count = m.Globals()[retainSlots[0]]
	})
	fmt.Printf("err=%v faults=%d safe=%d dog=%d count=%s\n", err, Faults, len(safe), dog, count.Inspect())
	fmt.Printf("faults: %q\n", safe)
	Faults = 0
	err = Run(3, func(m *vm.VM) {
		count = m.Globals()[retainSlots[0]]
		fresh = m.Globals()[retainSlots[0]+1]
	})
	fmt.Printf("restart err=%v count=%s fresh=%s retainErr=%v\n", err, count.Inspect(), fresh.Inspect(), RetainErr)
}
`, ProgramOptions{Restarts: 3, ScrubBytes: 1 << 20})
	for _, want := range []string{
		"err=<nil> faults=3 safe=3 dog=12 count=12\n",
		"restart err=<nil> count=15 fresh=3 retainErr=<nil>\n",
		"TRUE or FALSE corrupt",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}

// Faults that go on past Restarts in a row stop the program, SafeState
// called for each; with Restarts 0, the first does.
func TestGenerateEmbeddedProgramGivesUp(t *testing.T) {
	main := `
func main() {
	safe := 0
	SafeState = func(error) { safe++ }
	n := 0
	err := Run(0, func(m *vm.VM) {
		if n++; n == 2 {
			vm.False.Value = true
		}
	})
	fmt.Printf("err=%v faults=%d safe=%d\n", err != nil, Faults, safe)
}
`
	// A FALSE that reads true fails the scan (bad is FALSE) or the check
	// after it: either is a fault.
	if got := firmware(t, faultProgram, main, ProgramOptions{}); !strings.Contains(got, "err=true faults=0 safe=1\n") {
		t.Errorf("Restarts 0: %s", got)
	}
	// A RETAIN count past 2 makes every start fail.
	src := strings.Replace(faultProgram, "IF bad THEN", "IF bad OR count > 2 THEN", 1)
	main = `
func main() {
	RetainStore = &harden.MemStore{}
	safe := 0
	SafeState = func(error) { safe++ }
	err := Run(0, nil)
	fmt.Printf("err=%v faults=%d safe=%d\n", err != nil, Faults, safe)
}
`
	if got := firmware(t, src, main, ProgramOptions{Restarts: 2}); !strings.Contains(got, "err=true faults=2 safe=3\n") {
		t.Errorf("Restarts 2: %s", got)
	}
}

// The checksum the generator writes is the one the firmware's image has.
func TestGenerateEmbeddedProgramChecksum(t *testing.T) {
	got := firmware(t, deviceProgram, `
func main() {
	fmt.Println("match", harden.Checksum(images()...) == imageChecksum, checkImage() == nil)
}
`, ProgramOptions{})
	if !strings.Contains(got, "match true true") {
		t.Errorf("%s", got)
	}
}
