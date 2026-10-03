package transpiler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// transpileSource parses and transpiles input, returning the Go code with
// whitespace collapsed, as in transpileAndCheck.
func transpileSource(t *testing.T, input string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors for %q: %v", input, p.Errors())
	}
	var buf bytes.Buffer
	err := New(&buf).Transpile(program)
	return strings.Join(strings.Fields(buf.String()), " "), err
}

// checkContains checks that the transpiled input contains each of want.
func checkContains(t *testing.T, input string, want ...string) {
	t.Helper()
	got, err := transpileSource(t, input)
	if err != nil {
		t.Fatalf("transpile %q: %v", input, err)
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("output of %q does not contain %q:\n%s", input, w, got)
		}
	}
}

const initTypes = `TYPE
	Pt : STRUCT px : INT := 4; py : BOOL; END_STRUCT;
	Line : STRUCT a : Pt; b : Pt := (px := 7); END_STRUCT;
	Color : (Red, Green, Blue);
	Shade : (Light, Dark) := Dark;
	Rng : INT(1..10);
	MyInt : INT := 42;
END_TYPE
`

func TestProgramInitialValues(t *testing.T) {
	program := func(vars string) string {
		return initTypes + "PROGRAM P VAR " + vars + " END_VAR END_PROGRAM"
	}
	tests := []struct {
		vars string
		want []string
	}{
		// Initial values may refer to variables declared before them.
		{"a : INT := 2; b : INT := a * 10;", []string{"instance.a = 2", "instance.b = (instance.a * 10)"}},
		// Structure members take their initial values; an initializer overrides some.
		{"p : Pt;", []string{"instance.p = Pt{px: 4}"}},
		{"p : Pt := (py := TRUE);", []string{"instance.p = Pt{px: 4, py: true}"}},
		{"seg : Line;", []string{"instance.seg = Line{a: Pt{px: 4}, b: Pt{px: 7}}"}},
		// Enumerations start at the type's initial value, else the first value.
		{"sh : Shade; c : Color := Blue;", []string{"instance.sh = Shade_Dark", "instance.c = Color_Blue"}},
		// A subrange starts at its lower limit; an alias at its initial value.
		{"rv : Rng; m : MyInt;", []string{"instance.rv = 1", "instance.m = 42"}},
		{"m : MyInt := 5;", []string{"instance.m = 5"}},
		// Arrays have their declared length; missing elements take the default.
		{"arr : ARRAY[1..3] OF INT;", []string{"instance.arr = [3]iec.INT{}"}},
		{"arr : ARRAY[1..3] OF INT := [1, 2];", []string{"instance.arr = [3]iec.INT{1, 2}"}},
		{"arr : ARRAY[0..3] OF INT := [2(5), 1, 0];", []string{"instance.arr = [4]iec.INT{5, 5, 1, 0}"}},
		{"grid : ARRAY[0..1, 0..2] OF BOOL;", []string{"instance.grid = [2][3]iec.BOOL{}"}},
		{"pts : ARRAY[0..1] OF Pt;", []string{"__a[__i] = Pt{px: 4}"}},
		{"g : ARRAY[0..1, 0..1] OF INT := [[1, 2], [3]];", []string{"instance.g = [2][2]iec.INT{[2]iec.INT{1, 2}, [2]iec.INT{3}}"}},
		{"g : ARRAY[0..1, 0..1] OF INT := [1, 2, 3, 4];", []string{"instance.g = [2][2]iec.INT{[2]iec.INT{1, 2}, [2]iec.INT{3, 4}}"}},
	}
	for _, tt := range tests {
		checkContains(t, program(tt.vars), tt.want...)
	}
}

func TestFunctionBlockInit(t *testing.T) {
	input := initTypes + `
FUNCTION_BLOCK Inner
	VAR_INPUT k : INT := 3; END_VAR
	VAR s : Shade; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK Plain
	VAR z : INT; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK Outer EXTENDS Inner
	VAR v : INT := 3; inn : Inner := (k := 8); pl : Plain; END_VAR
END_FUNCTION_BLOCK
PROGRAM P
	VAR o : Outer := (v := 9); pl : Plain; END_VAR
END_PROGRAM`
	checkContains(t, input,
		"func (i *Inner) Init() { i.k = 3 i.s = Shade_Dark }",
		// The parent's Init runs first; nested instances are initialized too.
		"func (o *Outer) Init() { o.Inner.Init() o.v = 3 o.inn.Init() o.inn.k = 8 }",
		// The program's factory initializes instances, then applies initializers.
		"instance.o.Init() instance.o.v = 9 instance.o.EN = true",
		"instance.pl.EN = true",
	)
	got, _ := transpileSource(t, input)
	if strings.Contains(got, "Plain) Init()") || strings.Contains(got, "instance.pl.Init()") {
		t.Errorf("a function block with nothing to set should have no Init:\n%s", got)
	}
}

func TestFunctionResultsAndCalls(t *testing.T) {
	input := initTypes + `
FUNCTION Inc : INT
	VAR_INPUT a : INT := 5; b : INT; END_VAR
	Inc := a * 10 + b;
END_FUNCTION
FUNCTION Split : Pt
	VAR_INPUT v : INT; END_VAR
	VAR_OUTPUT hi : INT := 7; lo : BOOL; END_VAR
	VAR c : INT := 10; END_VAR
	hi := v + c;
	IF v > 100 THEN
		RETURN;
	END_IF
	lo := TRUE;
END_FUNCTION
PROGRAM P
	VAR r : INT; h : INT; l : BOOL; q : Pt; END_VAR
	r := Inc(b := 2);
	r := Inc();
	r := Inc(3, 4);
	q := Split(v := r, hi => h, lo => l);
	Split(v := 1, lo => l);
END_PROGRAM`
	checkContains(t, input,
		// The result is a named Go result; assigning it does not return.
		"func Inc(a iec.INT, b iec.INT) (Inc iec.INT) { Inc = ((a * 10) + b) return }",
		// Results and VAR_OUTPUTs start at their defaults and initial values.
		"func Split(v iec.INT) (Split Pt, hi iec.INT, lo iec.BOOL) { Split = Pt{px: 4} hi = 7 var c iec.INT = 10",
		"if (v > 100) { return }",
		// Formal calls fill omitted inputs with their defaults.
		"p.r = Inc(5, 2)",
		"p.r = Inc(5, 0)",
		"p.r = Inc(3, 4)",
		// Bound outputs are assigned after the call.
		"p.q = func() Pt { __r, __o0, __o1 := Split(p.r); p.h = __o0; p.l = __o1; return __r }()",
		"func() Pt { __r, _, __o1 := Split(1); p.l = __o1; return __r }()",
	)
}

func TestMethodInitialValues(t *testing.T) {
	input := `
FUNCTION_BLOCK Fb
	VAR v : INT := 3; END_VAR
	METHOD Sum : INT
		VAR_INPUT extra : INT := 1; END_VAR
		VAR loc : INT := 2; END_VAR
		Sum := v + extra + loc;
	END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
	VAR f : Fb; r : INT; END_VAR
	r := f.Sum(extra := 5);
	r := f.Sum();
END_PROGRAM`
	checkContains(t, input,
		// Parameters and locals are written without the receiver.
		"func (f *Fb) Sum(extra iec.INT) (Sum iec.INT) { var loc iec.INT = 2 Sum = ((f.v + extra) + loc) return }",
		"p.r = p.f.Sum(5)",
		"p.r = p.f.Sum(1)",
	)
}

func TestStructureAssignment(t *testing.T) {
	checkContains(t, initTypes+"PROGRAM P VAR p : Pt; END_VAR p := (py := TRUE); END_PROGRAM",
		"p.p = Pt{px: 4, py: true}")
}

func TestGlobalAndTempInitialValues(t *testing.T) {
	checkContains(t, initTypes+"VAR_GLOBAL gp : Pt; ga : ARRAY[0..1] OF INT := [9]; END_VAR",
		"var gp Pt = Pt{px: 4}",
		"var ga [2]iec.INT = [2]iec.INT{9}")
	checkContains(t, "PROGRAM P VAR a : INT := 2; END_VAR VAR_TEMP tmp : INT := 4; END_VAR a := tmp; END_PROGRAM",
		"var tmp iec.INT = 4")
}

func TestInvalidInitialValuesTranspilation(t *testing.T) {
	tests := []struct{ input, want string }{
		{initTypes + "PROGRAM P VAR p : Pt := (nope := 1); END_VAR END_PROGRAM", "structure 'Pt' has no member 'nope'"},
		{"PROGRAM P VAR x : INT := (a := 1); END_VAR END_PROGRAM", "needs a structure or function block type"},
		{"PROGRAM P VAR a : ARRAY[0..1] OF INT := [1, 2, 3]; END_VAR END_PROGRAM", "array initial value has 3 elements, but the array holds 2"},
		{"FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb := (w := 1); END_VAR END_PROGRAM", "function block 'Fb' has no variable 'w'"},
		{"FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION PROGRAM P VAR r : INT; END_VAR r := F(b := 1); END_PROGRAM", "'F' has no input 'b'"},
		{"FUNCTION F : INT VAR_INPUT a : INT; b : INT; END_VAR F := a; END_FUNCTION PROGRAM P VAR r : INT; END_VAR r := F(1); END_PROGRAM", "must supply all 2 inputs, got 1"},
		{"FUNCTION F : INT VAR_IN_OUT io : INT; END_VAR F := io; END_FUNCTION PROGRAM P VAR r : INT; END_VAR r := F(); END_PROGRAM", "must supply its VAR_IN_OUT 'io'"},
		{"PROGRAM P VAR x : INT; END_VAR x := (a := 1); END_PROGRAM", "but 'INT' is neither"},
	}
	for _, tt := range tests {
		_, err := transpileSource(t, tt.input)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected error containing %q, got %v", tt.input, tt.want, err)
		}
	}
}
