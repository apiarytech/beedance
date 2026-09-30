/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"testing"

	"beedance/ast"
	"beedance/object"
)

// compileWithClock compiles input with the clock builtin the standard timers read.
func compileWithClock(t *testing.T, input string) (*Compiler, error) {
	t.Helper()
	c := NewCompilerWithBuiltins([]object.BuiltinEntry{{Name: "__CLOCK", Index: 0}})
	return c, c.Compile(parse(t, input))
}

func TestStandardFunctionBlocksCompile(t *testing.T) {
	if got := len(standardFBs()); got != 10 {
		t.Fatalf("%d standard function blocks, want 10", got)
	}
	// Only the standard FBs a program uses are compiled with it.
	c, err := compileWithClock(t, "VAR t : TON; r : R_TRIG; END_VAR t(IN := r.Q, PT := T#1s);")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"TON": true, "R_TRIG": true, "TOF": false, "CTU": false} {
		if _, got := c.typeInfo[name]; got != want {
			t.Errorf("%s compiled: %v, want %v", name, got, want)
		}
	}
	// A program that uses none compiles no standard FB.
	c, err = compileWithClock(t, "VAR x : INT; END_VAR x := 1;")
	if err != nil || len(c.typeInfo) != 0 {
		t.Fatalf("typeInfo %v, err %v", c.typeInfo, err)
	}
	// A program's own FB of a standard name replaces the standard one.
	c, err = compileWithClock(t, "FUNCTION_BLOCK CTU VAR_OUTPUT n : INT; END_VAR n := 1; END_FUNCTION_BLOCK VAR c : CTU; END_VAR c(); c.n;")
	if err != nil {
		t.Fatal(err)
	}
	if fb, ok := c.typeInfo["CTU"].(*ast.FunctionBlockDeclaration); !ok || len(fb.VarOutputs) != 1 || fb.VarOutputs[0].Name.Value != "n" {
		t.Errorf("the standard CTU replaced the program's own: %v", c.typeInfo["CTU"])
	}
	// Standard FB members have their declared types.
	for _, input := range []string{
		"VAR c : CTUD; b : BOOL; END_VAR c(CU := TRUE, PV := 3); b := c.QU AND NOT c.QD;",
		"VAR t : TP; d : TIME; END_VAR t(IN := TRUE, PT := T#1s); d := t.ET + T#1s;",
		"VAR s : SR; q : RS; END_VAR s(S1 := TRUE, R := FALSE); q(S := s.Q1, R1 := FALSE);",
		"VAR f : F_TRIG; c : CTD; END_VAR f(CLK := TRUE); c(CD := f.Q, LD := FALSE, PV := 2);",
	} {
		if _, err := compileWithClock(t, input); err != nil {
			t.Errorf("%s: %v", input, err)
		}
	}
	// The timers need the clock builtin.
	if err := NewCompilerWithBuiltins(nil).Compile(parse(t, "VAR t : TOF; END_VAR t();")); err == nil {
		t.Error("TOF compiled without the clock")
	}
}

func TestDeclarationOrderCompiles(t *testing.T) {
	for _, input := range []string{
		// A function calls a function declared after it.
		"FUNCTION A : INT A := B(); END_FUNCTION FUNCTION B : INT B := 1; END_FUNCTION A();",
		// A function uses a global declared after it.
		"FUNCTION A : INT A := g; END_FUNCTION VAR_GLOBAL g : INT := 1; END_VAR A();",
		// Constants and located and macro globals.
		"FUNCTION A : INT A := k; END_FUNCTION VAR_GLOBAL CONSTANT k : INT := 2; END_VAR VAR_GLOBAL io AT %IX0.0 : BOOL; END_VAR A();",
	} {
		if _, err := compileWithClock(t, input); err != nil {
			t.Errorf("%s: %v", input, err)
		}
	}
}

func TestElementaryTypeNames(t *testing.T) {
	for name, want := range map[object.ObjectType]object.ObjectType{
		"BOOL": object.BOOLEAN_OBJ, "bool": object.BOOLEAN_OBJ,
		"TOD": object.TIME_OF_DAY_OBJ, "LTOD": object.TIME_OF_DAY_OBJ, "LTIME_OF_DAY": object.TIME_OF_DAY_OBJ,
		"DT": object.DATE_AND_TIME_OBJ, "LDT": object.DATE_AND_TIME_OBJ, "LDATE_AND_TIME": object.DATE_AND_TIME_OBJ,
		"LTIME": object.TIME_OBJ, "LDATE": object.DATE_OBJ, "INT": object.INT_OBJ,
	} {
		if got := elementaryTypeName(name); got != want {
			t.Errorf("elementaryTypeName(%s) = %s, want %s", name, got, want)
		}
	}
	c := New()
	for _, tt := range []struct {
		op          string
		left, right object.ObjectType
		want        object.ObjectType
	}{
		{"AND", "BOOL", object.BOOLEAN_OBJ, object.BOOLEAN_OBJ},
		{"OR", object.BYTE_OBJ, object.WORD_OBJ, object.WORD_OBJ},
		{"XOR", object.DWORD_OBJ, object.BYTE_OBJ, object.DWORD_OBJ},
		{"<", object.BYTE_OBJ, object.LWORD_OBJ, object.BOOLEAN_OBJ},
		{"=", "BOOL", "BOOL", object.BOOLEAN_OBJ},
		{"<", "TOD", object.TIME_OF_DAY_OBJ, object.BOOLEAN_OBJ},
		{"-", "DT", object.TIME_OBJ, object.DATE_AND_TIME_OBJ},
	} {
		got, err := c.getResultingType(tt.op, tt.left, tt.right)
		if err != nil || got != tt.want {
			t.Errorf("%s %s %s = %s, %v; want %s", tt.left, tt.op, tt.right, got, err, tt.want)
		}
	}
}

func TestBitAccessCompiles(t *testing.T) {
	for _, input := range []string{
		"VAR x : BYTE; b : BOOL; END_VAR b := x.3; x.7 := b;",
		"FUNCTION Odd : BOOL VAR_INPUT n : INT; END_VAR Odd := n.0; END_FUNCTION Odd(3);",
		// An integer literal compares with a bit string, on either side.
		"VAR x : BYTE; b : BOOL; END_VAR b := (x = 10) OR (16#FF > x);",
	} {
		if _, err := compileWithClock(t, input); err != nil {
			t.Errorf("%s: %v", input, err)
		}
	}
	if _, err := compileWithClock(t, "VAR x : LWORD; END_VAR x.64;"); err == nil {
		t.Error("bit 64 accepted")
	}
}

func TestBoolLiteralInitialValues(t *testing.T) {
	for _, input := range []string{
		"VAR a : BOOL := 1; b : BOOL := 0; END_VAR",
		"VAR x : BOOL := TRUE; y : BOOL := x; END_VAR",
	} {
		if _, err := compileWithClock(t, input); err != nil {
			t.Errorf("%s: %v", input, err)
		}
	}
}
