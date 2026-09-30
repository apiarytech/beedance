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

// Declaring a PROGRAM defines it; calling it runs its body. Its variables
// keep their values between calls and can be read as members. The VM's
// TestProgramCalls checks the same programs.
func TestProgramCalls(t *testing.T) {
	const twice = "PROGRAM Pg VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := i * 2; END_PROGRAM "
	integers := []struct {
		input    string
		expected int64
	}{
		{twice + "VAR res : INT; END_VAR Pg(i := 4, o => res); res;", 8},
		{twice + "Pg(i := 5); Pg.o;", 10},
		{"PROGRAM Pc VAR n : INT; END_VAR n := n + 1; END_PROGRAM Pc.n;", 0},
		{"PROGRAM Pc VAR n : INT; END_VAR n := n + 1; END_PROGRAM Pc(); Pc(); Pc.n;", 2},
		{"PROGRAM Pt VAR_TEMP t1 : INT; END_VAR VAR_OUTPUT o : INT; END_VAR t1 := t1 + 1; o := t1; END_PROGRAM Pt(); Pt(); Pt.o;", 1},
		{"PROGRAM Pio VAR_IN_OUT io : INT; END_VAR io := io * 3; END_PROGRAM VAR v : INT := 2; END_VAR Pio(io := v); Pio(io := v); v;", 18},
		{"PROGRAM Pgl VAR_GLOBAL g : INT := 1; END_VAR g := g + 1; END_PROGRAM Pgl(); g;", 2},
		{"PROGRAM A1 VAR n : INT := 1; END_VAR n := n + 1; END_PROGRAM PROGRAM B1 VAR n : INT := 10; END_VAR n := n + 1; END_PROGRAM A1(); B1(); A1.n * 100 + B1.n;", 211},
		{"FUNCTION_BLOCK Cnt VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK PROGRAM Pb VAR c : Cnt; END_VAR c(); END_PROGRAM Pb(); Pb(); Pb.c.n;", 2},
		// EN = FALSE skips the body.
		{twice + "Pg(i := 1); Pg(EN := FALSE, i := 7); Pg.o;", 2},
	}
	for _, tt := range integers {
		testIntegerObject(t, testEval(t, tt.input), tt.input, tt.expected)
	}

	booleans := []struct {
		input    string
		expected bool
	}{
		{twice + "VAR ok : BOOL := TRUE; END_VAR Pg(EN := FALSE, i := 1, ENO => ok); ok;", false},
		{twice + "VAR ok : BOOL; END_VAR Pg(EN := FALSE); Pg(i := 1, ENO => ok); ok;", true},
		{"PROGRAM Pi VAR n : INT; q : BOOL; END_VAR LD n ADD 1 ST n LD n GT 1 S q END_PROGRAM Pi(); Pi(); Pi.q;", true},
	}
	for _, tt := range booleans {
		testBooleanObject(t, testEval(t, tt.input), tt.input, tt.expected)
	}

	testErrorObjectContains(t, testEval(t, twice+"Pg.nope;"), "program 'Pg' has no variable 'nope'")
	testErrorObjectContains(t, testEval(t, "PROGRAM Pe VAR_GLOBAL g : INT := missing; END_VAR END_PROGRAM"), "missing")
}

// Arrays and structures are assigned and passed by value: changing a copy
// leaves the original alone. A VAR_IN_OUT still changes its argument. The
// VM's TestArrayAndStructureCopies checks the same programs.
func TestArrayAndStructureCopies(t *testing.T) {
	const pt = "TYPE Pt : STRUCT x : INT; y : INT; END_STRUCT; END_TYPE "
	tests := []struct {
		input    string
		expected int64
	}{
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; b : ARRAY[0..2] OF INT; END_VAR b := a; b[0] := 9; a[0] * 10 + b[0];", 19},
		{pt + "VAR p : Pt; q : Pt; END_VAR p.x := 1; q := p; q.x := 5; p.x * 10 + q.x;", 15},
		{"TYPE Rec : STRUCT row1 : ARRAY[1..2] OF INT; END_STRUCT; END_TYPE VAR p : Rec; q : Rec; END_VAR q := p; q.row1[1] := 6; p.row1[1] * 10 + q.row1[1];", 6},
		{"VAR m : ARRAY[0..1, 0..1] OF INT; row : ARRAY[0..1] OF INT; END_VAR m[0] := row; row[0] := 4; m[0][0];", 0},
		{pt + "VAR ps : ARRAY[0..1] OF Pt; q : Pt; END_VAR ps[0] := q; q.x := 3; ps[0].x;", 0},
		{"FUNCTION F : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR F := v[1]; END_FUNCTION VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR F(a) * 10 + a[0];", 11},
		{"FUNCTION F : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR F := v[1]; END_FUNCTION VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR F(v := a) * 10 + a[0];", 11},
		{pt + "FUNCTION_BLOCK Fb VAR_INPUT p : Pt; END_VAR END_FUNCTION_BLOCK VAR f : Fb; q : Pt; END_VAR f(p := q); q.x := 7; f.p.x;", 0},
		{"FUNCTION_BLOCK Fb VAR_INPUT v : ARRAY[0..1] OF INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; a : ARRAY[0..1] OF INT; END_VAR f(v := a); a[0] := 5; f.v[0];", 0},
		{"FUNCTION_BLOCK Fb VAR_OUTPUT v : ARRAY[0..1] OF INT; END_VAR v[0] := v[0] + 1; END_FUNCTION_BLOCK VAR f : Fb; a : ARRAY[0..1] OF INT; END_VAR f(v => a); a[0] := 50; f(); f.v[0];", 2},
		{"TYPE Buf : ARRAY[0..1] OF INT; END_TYPE FUNCTION_BLOCK Fb VAR buf : Buf; END_VAR METHOD Snapshot : Buf Snapshot := buf; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; a : Buf; END_VAR a := f.Snapshot(); a[0] := 8; f.buf[0];", 0},
		{"FUNCTION F : INT VAR_IN_OUT v : ARRAY[0..2] OF INT; END_VAR v[0] := 100; F := 0; END_FUNCTION VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR F(a); a[0];", 100},
		{"FUNCTION_BLOCK Acc VAR_IN_OUT v : ARRAY[0..1] OF INT; END_VAR v[1] := 5; END_FUNCTION_BLOCK VAR f : Acc; a : ARRAY[0..1] OF INT; END_VAR f(v := a); a[1];", 5},
		// A positional argument is evaluated once.
		{"FUNCTION Id : INT VAR_INPUT i : INT; END_VAR Id := i; END_FUNCTION FUNCTION_BLOCK Cnt VAR n : INT; END_VAR METHOD Next : INT n := n + 1; Next := n; END_METHOD END_FUNCTION_BLOCK VAR c : Cnt; END_VAR Id(c.Next()); c.n;", 1},
	}
	for _, tt := range tests {
		testIntegerObject(t, testEval(t, tt.input), tt.input, tt.expected)
	}
}

// In IL, ST, S, R, conditional jumps and calls of function blocks read the
// current result without changing it. The VM's TestIlCurrentResultIsKept
// checks the same programs.
func TestIlCurrentResultIsKept(t *testing.T) {
	il := func(vars, body, result string) string {
		return "PROGRAM P\n VAR " + vars + " END_VAR\n" + body + "\nEND_PROGRAM\nP();\nP." + result + ";"
	}
	const cnt = "FUNCTION_BLOCK Fb VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK "
	integers := []struct {
		input    string
		expected int64
	}{
		{il("x : INT; b : BOOL := TRUE;", " LD b\n JMPC end_it\n LD 10\n ST x\nend_it:\n LD 1\n ST x", "x"), 1},
		{il("x : INT; y : INT;", " LD 3\n ST x\n ADD 1\n ST y\n MUL x", "y"), 4},
		{il("i : INT;", "again:\n LD i\n ADD 1\n ST i\n LT 1000\n JMPC again", "i"), 1000},
		{cnt + "PROGRAM P VAR f : Fb; x : INT; END_VAR LD 5 CAL f() ADD 1 ST x END_PROGRAM P(); P.x;", 6},
		{cnt + "PROGRAM P VAR f : Fb; END_VAR LD FALSE CALC f() LD TRUE CALC f() CALCN f() END_PROGRAM P(); P.f.n;", 1},
		{"FUNCTION Seven : INT Seven := 7; END_FUNCTION PROGRAM P VAR x : INT; END_VAR LD 1 CAL Seven() ADD 1 ST x END_PROGRAM P(); P.x;", 8},
		{"FUNCTION F : INT VAR_INPUT b : BOOL; END_VAR LD 1 ST F LD b RETC LD 2 ST F END_FUNCTION F(TRUE) * 10 + F(FALSE);", 12},
	}
	for _, tt := range integers {
		testIntegerObject(t, testEval(t, tt.input), tt.input, tt.expected)
	}
	testBooleanObject(t, testEval(t, il("q : BOOL := TRUE; b : BOOL := FALSE;", " LD b\n JMPCN end_it\n LD TRUE\nend_it:\n ST q", "q")), "JMPCN", false)
	testBooleanObject(t, testEval(t, il("n : INT; q : BOOL;", " LD n\n ADD 1\n ST n\n GT 0\n S q\n ST n", "q")), "S", true)
}

// A type declared in a namespace is named with its namespace, in any case.
func TestNamespacedTypes(t *testing.T) {
	const lib = "NAMESPACE Lib TYPE Mode : (Idle, Busy); END_TYPE END_NAMESPACE "
	testBooleanObject(t, testEval(t, lib+"VAR m : Lib.Mode; END_VAR m := Lib.Mode#Busy; m = Lib.Mode#Busy;"), "qualified", true)
	testBooleanObject(t, testEval(t, lib+"VAR m : Lib.Mode; END_VAR m = LIB.MODE#Idle;"), "case", true)
	testErrorObjectContains(t, testEval(t, lib+"VAR m : Lib.Mode; END_VAR m := Lib.Mode#Gone;"), "enumerated value 'Gone' not found in type 'LIB.MODE'")
	testErrorObjectContains(t, testEval(t, "VAR m : INT; END_VAR m := Nope.Mode#Busy;"), "unknown type: NOPE.MODE")
}
