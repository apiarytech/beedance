/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"github.com/apiarytech/beedance/object"
	"testing"
)

// runInspectTests runs each input and compares the type and printed form of
// its result, for values the typed helpers do not cover.
func runInspectTests(t *testing.T, tests []struct{ input, wantType, want string }) {
	t.Helper()
	for _, tt := range tests {
		machine := New(compileForTest(t, tt.input))
		if err := machine.Run(); err != nil {
			t.Fatalf("%s: vm error: %s", tt.input, err)
		}
		got := machine.LastPoppedStackElem()
		if string(got.Type()) != tt.wantType || got.Inspect() != tt.want {
			t.Errorf("%s:\n  expected %s %s\n  got      %s %s", tt.input, tt.wantType, tt.want, got.Type(), got.Inspect())
		}
	}
}

// TestDefaultValues checks that a variable declared without an initial value
// starts at the IEC 61131-3 default of its type.
func TestDefaultValues(t *testing.T) {
	runInspectTests(t, []struct{ input, wantType, want string }{
		{"VAR x : INT; END_VAR x;", "LINT", "0"},
		{"VAR x : UDINT; END_VAR x;", "LINT", "0"},
		{"VAR x : BOOL; END_VAR x;", "BOOLEAN", "false"},
		{"VAR x : REAL; END_VAR x;", "LREAL", "0.000000"},
		{"VAR x : STRING; END_VAR x;", "STRING", ""},
		{"VAR x : WSTRING; END_VAR x;", "WSTRING", ""},
		{"VAR x : TIME; END_VAR x;", "TIME", "T#0s"},
		{"VAR x : BYTE; END_VAR x;", "BITSTRING", "BYTE#16#0"},
		{"VAR x : DATE; END_VAR x;", "DATE", "D#0001-01-01"},
		{"VAR x : TOD; END_VAR x;", "TIME_OF_DAY", "TOD#00:00:00"},
		{"VAR x : DT; END_VAR x;", "DATE_AND_TIME", "DT#0001-01-01-00:00:00"},
		// Defaults make arithmetic on undeclared-value variables work.
		{"VAR x : INT; END_VAR x + 1;", "LINT", "1"},
	})
}

func TestInitialValueConversion(t *testing.T) {
	runInspectTests(t, []struct{ input, wantType, want string }{
		// An integer constant takes the declared REAL or bit-string type.
		{"VAR x : REAL := 1; END_VAR x;", "LREAL", "1.000000"},
		{"VAR x : LREAL := -2; END_VAR x;", "LREAL", "-2.000000"},
		{"VAR x : BYTE := 16#FF; END_VAR x;", "BITSTRING", "BYTE#16#FF"},
		{"VAR x : INT := 2 * 3 + 1; END_VAR x;", "LINT", "7"},
		{"VAR a : INT := 2; b : INT := a * 10; END_VAR b;", "LINT", "20"},
	})
}

func TestArrayInitialValues(t *testing.T) {
	runInspectTests(t, []struct{ input, wantType, want string }{
		{"VAR a : ARRAY[0..2] OF INT; END_VAR a;", "ARRAY", "[0, 0, 0]"},
		// Missing elements take the element type's default.
		{"VAR a : ARRAY[1..3] OF INT := [1, 2]; END_VAR a;", "ARRAY", "[1, 2, 0]"},
		{"VAR a : ARRAY[0..3] OF INT := [2(5), 1]; END_VAR a;", "ARRAY", "[5, 5, 1, 0]"},
		{"VAR a : ARRAY[0..1, 0..2] OF BOOL; END_VAR a;", "ARRAY", "[[false, false, false], [false, false, false]]"},
		{"VAR a : ARRAY[0..1] OF REAL := [1]; END_VAR a;", "ARRAY", "[1.000000, 0.000000]"},
	})
}

func TestStructureInitialValues(t *testing.T) {
	const pt = "TYPE Pt : STRUCT px : INT := 4; py : BOOL; tag : STRING; END_STRUCT; END_TYPE "
	runVmTests(t, []vmTestCase{
		// Members take their declared initial values, or their type's defaults.
		{pt + "VAR p : Pt; END_VAR p.px;", 4},
		{pt + "VAR p : Pt; END_VAR p.py;", false},
		{pt + "VAR p : Pt; END_VAR p.tag;", ""},
		// An initializer overrides the named members only.
		{pt + "VAR p : Pt := (px := 9); END_VAR p.px + 1;", 10},
		{pt + "VAR p : Pt := (py := TRUE); END_VAR p.py;", true},
		{pt + "VAR p : Pt := (py := TRUE); END_VAR p.px;", 4},
		// Nested structures and structures inside function blocks.
		{pt + "TYPE Line : STRUCT a : Pt; b : Pt := (px := 7); END_STRUCT; END_TYPE VAR seg : Line; END_VAR seg.b.px * 10 + seg.a.px;", 74},
		{pt + "FUNCTION_BLOCK Holder VAR p : Pt; END_VAR METHOD Read : INT Read := p.px; END_METHOD END_FUNCTION_BLOCK VAR h : Holder; END_VAR h.Read();", 4},
	})
}

func TestEnumerationAndSubrangeInitialValues(t *testing.T) {
	const types = `TYPE Color : (Red, Green, Blue); END_TYPE
		TYPE Shade : (Light, Dark) := Dark; END_TYPE
		TYPE Rng : INT(1..10); END_TYPE
		TYPE MyInt : INT := 42; END_TYPE `
	runInspectTests(t, []struct{ input, wantType, want string }{
		{types + "VAR c : Color; END_VAR c;", "ENUMERATED_VALUE", "Color#Red"},  // first value
		{types + "VAR c : Shade; END_VAR c;", "ENUMERATED_VALUE", "Shade#Dark"}, // the type's initial value
		{types + "VAR c : Color := Blue; END_VAR c;", "ENUMERATED_VALUE", "Color#Blue"},
		{types + "VAR c : Color := Color#Green; END_VAR c;", "ENUMERATED_VALUE", "Color#Green"},
		{types + "VAR rv : Rng; END_VAR rv;", "LINT", "1"}, // a subrange's lower limit
		{types + "VAR rv : Rng := 7; END_VAR rv;", "LINT", "7"},
		{types + "VAR m : MyInt; END_VAR m;", "LINT", "42"},     // an alias's initial value
		{types + "VAR m : MyInt := 5; END_VAR m;", "LINT", "5"}, // the variable's own value wins
	})
}

func TestConstantInitialValues(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"VAR CONSTANT k : INT := 3; END_VAR k * 2;", 6},
		{"VAR CONSTANT k : INT := 4; END_VAR VAR x : INT := k * 2; END_VAR x;", 8},
		{"VAR CONSTANT k : INT; END_VAR k;", 0},
	})
}

func TestFunctionInitialValues(t *testing.T) {
	const inc = "FUNCTION Inc : INT VAR_INPUT a : INT := 5; b : INT; END_VAR Inc := a * 10 + b; END_FUNCTION "
	runVmTests(t, []vmTestCase{
		// A formal call may omit inputs; they take their defaults.
		{inc + "Inc();", 50},
		{inc + "Inc(b := 2);", 52},
		{inc + "Inc(a := 1, b := 2);", 12},
		// A non-formal (positional) call supplies every input.
		{inc + "Inc(3, 4);", 34},
		// Results start at the return type's default.
		{"FUNCTION F : INT END_FUNCTION F();", 0},
		{"FUNCTION F : BOOL END_FUNCTION F();", false},
		// VAR_OUTPUTs start at their initial value.
		{"FUNCTION Out : INT VAR_OUTPUT o : INT := 7; END_VAR Out := 1; END_FUNCTION VAR res : INT; END_VAR Out(o => res); res;", 7},
		// Locals are initialized on every call.
		{"FUNCTION Cnt : INT VAR c : INT := 10; END_VAR c := c + 1; Cnt := c; END_FUNCTION Cnt() * 100 + Cnt();", 1111},
	})
}

func TestFunctionBlockInitialValues(t *testing.T) {
	const fb = `FUNCTION_BLOCK Fb
		VAR_INPUT i : INT := 4; END_VAR
		VAR v : INT := 3; w : REAL; END_VAR
		METHOD Sum : INT Sum := v + i; END_METHOD
		METHOD Wv : REAL Wv := w; END_METHOD
	END_FUNCTION_BLOCK `
	runVmTests(t, []vmTestCase{
		{fb + "VAR f : Fb; END_VAR f.Sum();", 7},
		// An initializer sets named variables, including inputs.
		{fb + "VAR f : Fb := (v := 9, i := 1); END_VAR f.Sum();", 10},
		{fb + "VAR f : Fb := (i := 6); END_VAR f.Sum();", 9},
		// A method result starts at its type's default.
		{"FUNCTION_BLOCK G METHOD M : INT END_METHOD END_FUNCTION_BLOCK VAR g : G; END_VAR g.M();", 0},
		// Method locals are initialized on every call.
		{"FUNCTION_BLOCK G METHOD M : INT VAR c : INT := 10; END_VAR c := c + 1; M := c; END_METHOD END_FUNCTION_BLOCK VAR g : G; END_VAR g.M() * 100 + g.M();", 1111},
	})

	// A REAL field defaults to REAL 0.0 even after an INT 0 constant exists.
	machine := New(compileForTest(t, fb+"VAR f : Fb; END_VAR f.Wv();"))
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if got, ok := machine.LastPoppedStackElem().(*object.LReal); !ok || got.Value != 0 {
		t.Fatalf("expected LREAL 0, got %T %v", machine.LastPoppedStackElem(), machine.LastPoppedStackElem())
	}
}
