/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

import "testing"

// REF_TO and POINTER TO are the same typed reference: REF and ADR refer to a
// variable, an array element or a member, ^ reads and writes it, NULL refers
// to nothing, and references compare by what they refer to.
func TestReferences(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR x : INT := 5; r : REF_TO INT; END_VAR r := REF(x); r^ := 7; x;", "7"},
		{"VAR x : INT := 5; p : POINTER TO INT; END_VAR p := ADR(x); p^ + 1;", "6"},
		{"VAR x : INT := 5; r : REF_TO INT := REF(x); END_VAR r^;", "5"},
		{"VAR ra : REF_TO INT; END_VAR ra = NULL;", "true"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra <> NULL;", "true"},
		{"VAR x, y : INT; ra, rb : REF_TO INT; END_VAR ra := REF(x); rb := ADR(x); ra = rb;", "true"},
		{"VAR x, y : INT; ra, rb : REF_TO INT; END_VAR ra := REF(x); rb := REF(y); ra <> rb;", "true"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra := NULL; ra = NULL;", "true"},
		{"VAR x : INT; ra : REF_TO INT := REF(x); rb : POINTER TO INT; END_VAR rb := ra; rb^ := 3; x;", "3"},
		{"VAR a : ARRAY[1..3] OF INT; ra : REF_TO INT; END_VAR ra := REF(a[2]); ra^ := 5; a[2];", "5"},
		{"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR s : Pt; ra : REF_TO INT; END_VAR ra := REF(s.px); ra^ := 9; s.px;", "9"},
		{"VAR x : INT; ra : REF_TO INT; rr : REF_TO REF_TO INT; END_VAR ra := REF(x); rr := REF(ra); rr^^ := 4; x;", "4"},
		// Through POINTER TO ARRAY[0..n], index 0 is the first element of the
		// array referred to, whatever its own bounds.
		{"VAR a : ARRAY[1..3] OF INT := [1, 2, 3]; p : POINTER TO ARRAY[0..32000] OF INT; END_VAR p := ADR(a); p^[0] := 9; a[1];", "9"},
		{`FUNCTION AVG3 : REAL VAR_INPUT pt : POINTER TO ARRAY[0..32000] OF REAL; END_VAR
			AVG3 := (pt^[0] + pt^[1] + pt^[2]) / 3.0; END_FUNCTION
		  VAR a : ARRAY[1..3] OF REAL := [1.0, 2.0, 6.0]; END_VAR AVG3(ADR(a));`, "3.000000"},
		{`FUNCTION SORT2 : BOOL VAR_INPUT pt : POINTER TO ARRAY[0..1] OF INT; END_VAR VAR h : INT; END_VAR
			IF pt^[0] > pt^[1] THEN h := pt^[0]; pt^[0] := pt^[1]; pt^[1] := h; END_IF; SORT2 := TRUE; END_FUNCTION
		  VAR a : ARRAY[1..2] OF INT := [9, 4]; END_VAR SORT2(ADR(a)); a[1];`, "4"},
		{"FUNCTION SETX : BOOL VAR_INPUT pt : POINTER TO INT; END_VAR pt^ := 42; SETX := TRUE; END_FUNCTION VAR x : INT; END_VAR SETX(ADR(x)); x;", "42"},
		// A reference to a VAR_IN_OUT refers to the variable the caller passed.
		{"FUNCTION SETY : BOOL VAR_IN_OUT y : INT; END_VAR VAR ry : REF_TO INT; END_VAR ry := REF(y); ry^ := 8; SETY := TRUE; END_FUNCTION VAR x : INT; END_VAR SETY(x); x;", "8"},
		// A program may still name a variable NULL.
		{"VAR NULL : INT := 3; END_VAR NULL + 1;", "4"},
	})
}

func TestReferenceErrors(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR ra : REF_TO INT; END_VAR ra^;", "ERROR: dereferencing a NULL reference"},
		{"VAR ra : REF_TO INT; END_VAR ra^ := 1;", "ERROR: dereferencing a NULL reference"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := 5;", "ERROR: it takes REF(), ADR() or NULL"},
		{"VAR ra : REF_TO INT; END_VAR ra := REF(missing);", "ERROR: identifier not found: missing"},
		{"VAR ra : REF_TO INT; END_VAR ra := REF(1 + 2);", "ERROR: REF needs a variable"},
		{"VAR a : ARRAY[1..3] OF INT; ra : REF_TO INT; END_VAR ra := REF(a[7]);", "ERROR: out of bounds"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra < ra;", "ERROR: no arithmetic"},
		{"VAR x : INT; ra : REF_TO INT; END_VAR ra := REF(x); ra = 1;", "ERROR: compared only with a reference or NULL"},
		{"VAR x : INT; END_VAR REF(x, x);", "ERROR: REF takes one argument"},
	})
}

// References to the variables of function blocks, functions and the I/O
// image, and a function that happens to be called REF.
func TestReferenceTargets(t *testing.T) {
	checkEval(t, []evalCase{
		{"FUNCTION_BLOCK Fb VAR v : INT; r : REF_TO INT; END_VAR r := REF(v); r^ := 11; END_FUNCTION_BLOCK VAR f : Fb; END_VAR f(); f.v;", "11"},
		{"FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; ra : REF_TO INT; END_VAR ra := REF(f.v); ra^ := 5; f.v;", "5"},
		{"VAR q AT %QW1 : INT; ra : REF_TO INT; END_VAR ra := REF(q); ra^ := 6; q;", "6"},
		{"FUNCTION F : INT VAR v : INT; ra : REF_TO INT; END_VAR ra := REF(v); ra^ := 2; F := v; END_FUNCTION F();", "2"},
		{"VAR x : INT; ra, rb : REF_TO INT; END_VAR ra := REF(x); rb := REF(ra^); rb^ := 1; x;", "1"},
		{"FUNCTION REF : INT VAR_INPUT v : INT; END_VAR REF := v * 2; END_FUNCTION REF(4);", "8"},
		{"VAR a : ARRAY[0..1] OF INT; ra, rb : REF_TO INT; END_VAR ra := REF(a[1]); rb := REF(a[1]); ra = rb;", "true"},
		{"VAR str : STRING(20); ra : REF_TO STRING; END_VAR ra := REF(str); ra^ := 'hi'; str;", "hi"},
		{"VAR a : ARRAY[0..1] OF INT; p : POINTER TO ARRAY[1..5] OF INT; END_VAR p := ADR(a); p^[1] := 3; a[0];", "3"},
		{"VAR x : INT; END_VAR x^;", "ERROR: not applicable to type"},
	})
}

// The evaluator checks references given to inputs, and to variables, by the
// declared types of the variables referred to, as the compiler does.
func TestReferenceArgumentErrors(t *testing.T) {
	checkEval(t, []evalCase{
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := pt^; END_FUNCTION VAR x : REAL; END_VAR G(ADR(x));", "ERROR: refers to a REAL, but pt is declared POINTER TO INT"},
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := pt^; END_FUNCTION VAR x : REAL; END_VAR G(pt := ADR(x));", "ERROR: refers to a REAL, but pt is declared POINTER TO INT"},
		{"FUNCTION F : BOOL VAR_INPUT pt : POINTER TO BYTE; END_VAR F := TRUE; END_FUNCTION VAR str : STRING; END_VAR F(ADR(str));", "ERROR: refers to a STRING, but pt is declared POINTER TO BYTE"},
		{"FUNCTION_BLOCK Fb VAR_INPUT pt : POINTER TO INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; x : REAL; END_VAR f(pt := ADR(x));", "ERROR: refers to a REAL, but pt is declared POINTER TO INT"},
		{"VAR x : REAL; ra : REF_TO INT; END_VAR ra := REF(x);", "ERROR: refers to a REAL, but ra is declared REF_TO INT"},
		{"VAR a : ARRAY[0..1] OF REAL; ra : REF_TO INT; END_VAR ra := REF(a[1]);", "ERROR: refers to a REAL, but ra is declared REF_TO INT"},
		{"VAR x : REAL; ra : REF_TO INT := REF(x); END_VAR ra;", "ERROR: refers to a REAL, but ra is declared REF_TO INT"},
		{"FUNCTION G : INT VAR_INPUT pt : POINTER TO INT; END_VAR G := pt^; END_FUNCTION VAR x : INT := 4; END_VAR G(pt := ADR(x));", "4"},
		{"FUNCTION_BLOCK Fb VAR_INPUT pt : POINTER TO INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := pt^; END_FUNCTION_BLOCK VAR f : Fb; x : INT := 6; END_VAR f(pt := ADR(x)); f.o;", "6"},
	})
}

// A reference to a function's local variable ends with the call, as in the VM.
func TestReferenceToReturnedLocal(t *testing.T) {
	const decls = `VAR g : REF_TO INT; END_VAR
FUNCTION Leak : BOOL VAR v : INT := 7; END_VAR g := REF(v); Leak := g^ = 7; END_FUNCTION
FUNCTION Other : INT VAR w : INT := 99; END_VAR Other := w; END_FUNCTION `
	checkEval(t, []evalCase{
		{decls + "Leak();", "true"},
		{decls + "Leak(); Other(); g^;", "ERROR: local variable of a call that has returned"},
		{decls + "Leak(); g^ := 1;", "ERROR: local variable of a call that has returned"},
	})
}
