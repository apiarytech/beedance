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
	"strings"
	"testing"

	"github.com/apiarytech/beedance/compiler"
)

// The VM runs references as the evaluator does; see
// evaluator/references_test.go.
func TestReferences(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"VAR x : INT := 5; r : REF_TO INT; END_VAR r := REF(x); r^ := 7; x;", 7},
		{"VAR x : INT := 5; p : POINTER TO INT; END_VAR p := ADR(x); p^ + 1;", 6},
		{"VAR x : INT := 5; r : REF_TO INT := REF(x); END_VAR r^;", 5},
		{"VAR ra : REF_TO INT; END_VAR ra = NULL;", true},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra <> NULL;", true},
		{"VAR x, y : INT; ra, rb : REF_TO INT; END_VAR ra := REF(x); rb := ADR(x); ra = rb;", true},
		{"VAR x, y : INT; ra, rb : REF_TO INT; END_VAR ra := REF(x); rb := REF(y); ra <> rb;", true},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra := NULL; ra = NULL;", true},
		{"VAR x : INT; ra : REF_TO INT := REF(x); rb : POINTER TO INT; END_VAR rb := ra; rb^ := 3; x;", 3},
		{"VAR a : ARRAY[1..3] OF INT; ra : REF_TO INT; END_VAR ra := REF(a[2]); ra^ := 5; a[2];", 5},
		{"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR s : Pt; ra : REF_TO INT; END_VAR ra := REF(s.px); ra^ := 9; s.px;", 9},
		{"VAR x : INT; ra : REF_TO INT; rr : REF_TO REF_TO INT; END_VAR ra := REF(x); rr := REF(ra); rr^^ := 4; x;", 4},
		{"VAR a : ARRAY[1..3] OF INT := [1, 2, 3]; p : POINTER TO ARRAY[0..32000] OF INT; END_VAR p := ADR(a); p^[0] := 9; a[1];", 9},
		{`FUNCTION AVG3 : REAL VAR_INPUT pt : POINTER TO ARRAY[0..32000] OF REAL; END_VAR
			AVG3 := (pt^[0] + pt^[1] + pt^[2]) / 3.0; END_FUNCTION
		  VAR a : ARRAY[1..3] OF REAL := [1.0, 2.0, 6.0]; END_VAR AVG3(ADR(a));`, 3.0},
		{`FUNCTION SORT2 : BOOL VAR_INPUT pt : POINTER TO ARRAY[0..1] OF INT; END_VAR VAR h : INT; END_VAR
			IF pt^[0] > pt^[1] THEN h := pt^[0]; pt^[0] := pt^[1]; pt^[1] := h; END_IF; SORT2 := TRUE; END_FUNCTION
		  VAR a : ARRAY[1..2] OF INT := [9, 4]; END_VAR SORT2(ADR(a)); a[1];`, 4},
		{"FUNCTION SETX : BOOL VAR_INPUT pt : POINTER TO INT; END_VAR pt^ := 42; SETX := TRUE; END_FUNCTION VAR x : INT; END_VAR SETX(ADR(x)); x;", 42},
		{"VAR NULL : INT := 3; END_VAR NULL + 1;", 4},
	})
}

// A reference takes only REF(), ADR() or NULL of the type it refers to: a
// POINTER TO BYTE cannot refer to a STRING, which is how OSCAT reads strings
// byte by byte.
func TestReferenceCompileErrors(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := 5;", "it takes REF(), ADR() or NULL, not 5"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := x + 1;", "it takes REF(), ADR() or NULL"},
		{"VAR s : STRING; pt : POINTER TO BYTE; END_VAR pt := ADR(s);", "ADR(s) refers to a STRING, but pt is declared POINTER TO BYTE"},
		{"VAR x : REAL; ra : REF_TO INT := REF(x); END_VAR ra;", "REF(x) refers to a REAL, but ra is declared REF_TO INT"},
		{"VAR x : INT; ra : REF_TO INT; rb : REF_TO REAL; END_VAR rb := ra;", "rb is declared REF_TO REAL, but ra is REF_TO INT"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x + 1);", "REF needs a variable"},
		{"VAR x : INT; END_VAR REF(x, x);", "REF takes one argument"},
		{"VAR a : ARRAY[1..3] OF INT; ra : REF_TO INT; END_VAR ra := REF(a[7]);", "array index out of bounds"},
	} {
		err := compiler.New().Compile(parse(t, tt.input))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v, want an error containing %q", tt.input, err, tt.want)
		}
	}
}

func TestReferenceRuntimeErrors(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"VAR ra : REF_TO INT; END_VAR ra^;", "dereferencing a NULL reference"},
		{"VAR ra : REF_TO INT; END_VAR ra^ := 1;", "dereferencing a NULL reference"},
		{"VAR a : ARRAY[1..3] OF INT; i : INT := 7; ra : REF_TO INT; END_VAR ra := REF(a[i]);", "out of bounds"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra = 1;", "compared only with a reference or NULL"},
	} {
		err := New(compileForTest(t, tt.input)).Run()
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v, want an error containing %q", tt.input, err, tt.want)
		}
	}
}

// References to the variables of function blocks, functions and the I/O
// image, and a function that happens to be called REF.
func TestReferenceTargets(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"FUNCTION_BLOCK Fb VAR v : INT; r : REF_TO INT; END_VAR r := REF(v); r^ := 11; END_FUNCTION_BLOCK VAR f : Fb; END_VAR f(); f.v;", 11},
		{"FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; ra : REF_TO INT; END_VAR ra := REF(f.v); ra^ := 5; f.v;", 5},
		{"VAR q AT %QW1 : INT; ra : REF_TO INT; END_VAR ra := REF(q); ra^ := 6; q;", 6},
		{"FUNCTION F : INT VAR v : INT; ra : REF_TO INT; END_VAR ra := REF(v); ra^ := 2; F := v; END_FUNCTION F();", 2},
		{"VAR x : INT; ra, rb : REF_TO INT; END_VAR ra := REF(x); rb := REF(ra^); rb^ := 1; x;", 1},
		{"FUNCTION REF : INT VAR_INPUT v : INT; END_VAR REF := v * 2; END_FUNCTION REF(4);", 8},
		{"VAR a : ARRAY[0..1] OF INT; ra, rb : REF_TO INT; END_VAR ra := REF(a[1]); rb := REF(a[1]); ra = rb;", true},
		{"VAR str : STRING(20); ra : REF_TO STRING; END_VAR ra := REF(str); ra^ := 'hi'; str;", "hi"},
		{"VAR a : ARRAY[0..1] OF INT; p : POINTER TO ARRAY[1..5] OF INT; END_VAR p := ADR(a); p^[1] := 3; a[0];", 3},
	})
	if err := New(compileForTest(t, "VAR x : INT; END_VAR x^;")).Run(); err == nil || !strings.Contains(err.Error(), "not applicable to type") {
		t.Errorf("x^ of an INT: %v", err)
	}
	if err := compiler.New().Compile(parse(t, "VAR a : ARRAY[0..1] OF REAL; p : POINTER TO ARRAY[0..5] OF INT; END_VAR p := ADR(a);")); err == nil || !strings.Contains(err.Error(), "refers to a ARRAY OF REAL") {
		t.Errorf("a pointer to INT arrays given a REAL array: %v", err)
	}
}

// A reference given to an input is checked as an assignment is: a function's
// or a function block's input declared POINTER TO INT takes no ADR of a REAL.
func TestReferenceArgumentErrors(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := pt^; END_FUNCTION VAR x : REAL; END_VAR G(ADR(x));",
			"ADR(x) refers to a REAL, but input pt of function G is declared POINTER TO INT"},
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := pt^; END_FUNCTION VAR x : REAL; END_VAR G(pt := ADR(x));",
			"input pt of function G is declared POINTER TO INT"},
		{"FUNCTION F : BOOL VAR_INPUT pt : POINTER TO BYTE; END_VAR F := TRUE; END_FUNCTION VAR str : STRING; END_VAR F(ADR(str));",
			"ADR(str) refers to a STRING, but input pt of function F is declared POINTER TO BYTE"},
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := 0; END_FUNCTION G(5);",
			"it takes REF(), ADR() or NULL, not 5"},
		{"FUNCTION_BLOCK Fb VAR_INPUT pt : POINTER TO INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; x : REAL; END_VAR f(pt := ADR(x));",
			"input pt of function block 'Fb' is declared POINTER TO INT"},
	} {
		err := compiler.New().Compile(parse(t, tt.input))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v, want an error containing %q", tt.input, err, tt.want)
		}
	}
	runVmTests(t, []vmTestCase{
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := pt^; END_FUNCTION VAR x : INT := 4; END_VAR G(pt := ADR(x));", 4},
		{"FUNCTION_BLOCK Fb VAR_INPUT pt : POINTER TO INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := pt^; END_FUNCTION_BLOCK VAR f : Fb; x : INT := 6; END_VAR f(pt := ADR(x)); f.o;", 6},
	})
}

// A reference to a function's local variable is used while the call runs;
// after it returns, the variable is gone and its stack slot belongs to
// other calls, so dereferencing the reference is an error, not a read of
// another call's variable.
func TestReferenceToReturnedLocal(t *testing.T) {
	input := `VAR g : REF_TO INT; END_VAR
FUNCTION Leak : BOOL VAR v : INT := 7; END_VAR g := REF(v); Leak := g^ = 7; END_FUNCTION
FUNCTION Other : INT VAR w : INT := 99; END_VAR Other := w; END_FUNCTION
Leak(); Other(); g^;`
	err := New(compileForTest(t, input)).Run()
	if err == nil || !strings.Contains(err.Error(), "local variable of a call that has returned") {
		t.Errorf("got %v", err)
	}
	machine := New(compileForTest(t, "VAR g : REF_TO INT; END_VAR FUNCTION Leak : BOOL VAR v : INT; END_VAR g := REF(v); Leak := TRUE; END_FUNCTION Leak(); g^ := 1;"))
	if err := machine.Run(); err == nil || !strings.Contains(err.Error(), "has returned") {
		t.Errorf("writing through it: %v", err)
	}
	runVmTests(t, []vmTestCase{
		{"VAR g : REF_TO INT; END_VAR FUNCTION Leak : BOOL VAR v : INT := 7; END_VAR g := REF(v); Leak := g^ = 7; END_FUNCTION Leak();", true},
	})
}
