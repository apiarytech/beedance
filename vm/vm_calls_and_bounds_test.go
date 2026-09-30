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

import "testing"

// Calling a function block instance runs its body with the given inputs,
// then assigns the requested outputs. Its variables keep their values
// between calls.
func TestFunctionBlockCalls(t *testing.T) {
	const twice = "FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := i * 2; END_FUNCTION_BLOCK "
	const acc = "FUNCTION_BLOCK Acc VAR_INPUT amount : INT; END_VAR VAR_IN_OUT total : INT; END_VAR total := total + amount; END_FUNCTION_BLOCK "
	runVmTests(t, []vmTestCase{
		{twice + "VAR f : Fb; res : INT; END_VAR f(i := 3, o => res); res;", 6},
		// State persists between calls.
		{"FUNCTION_BLOCK Cnt VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK VAR c : Cnt; END_VAR c(); c(); c(); c.n;", 3},
		// An output can be read from the instance after the call.
		{twice + "VAR f : Fb; END_VAR f(i := 21); f.o;", 42},
		// EN = FALSE skips the body; ENO follows EN.
		{twice + "VAR f : Fb; res : INT; ok : BOOL; END_VAR f(EN := FALSE, i := 3, o => res, ENO => ok); res;", 0},
		{twice + "VAR f : Fb; res : INT; ok : BOOL := TRUE; END_VAR f(EN := FALSE, i := 3, o => res, ENO => ok); ok;", false},
		{twice + "VAR f : Fb; ok : BOOL; END_VAR f(i := 3, ENO => ok); ok;", true},
		// A VAR_IN_OUT is copied in and back, by name or position, to any variable.
		{acc + "VAR a : Acc; sm : INT := 1; END_VAR a(amount := 5, total := sm); a(amount := 2, total := sm); sm;", 8},
		{acc + "VAR a : Acc; sm : INT := 1; END_VAR a(4, sm); sm;", 5},
		{acc + "VAR a : Acc; arr : ARRAY[0..1] OF INT; END_VAR a(amount := 3, total := arr[1]); arr;", []int{0, 3}},
		// A derived function block runs its parent's body first.
		{"FUNCTION_BLOCK Base VAR_INPUT i : INT; END_VAR VAR_OUTPUT seen : INT; END_VAR seen := i; END_FUNCTION_BLOCK " +
			"FUNCTION_BLOCK Dv EXTENDS Base VAR_OUTPUT twice : INT; END_VAR twice := seen * 2; END_FUNCTION_BLOCK " +
			"VAR d : Dv; res : INT; END_VAR d(i := 7, twice => res); res;", 14},
		// A function block calls the instances it contains.
		{"FUNCTION_BLOCK Inner VAR_OUTPUT q : INT; END_VAR q := q + 10; END_FUNCTION_BLOCK " +
			"FUNCTION_BLOCK Outer VAR sub : Inner; END_VAR VAR_OUTPUT total : INT; END_VAR sub(q => total); END_FUNCTION_BLOCK " +
			"VAR o : Outer; res : INT; END_VAR o(); o(total => res); res;", 20},
		{"FUNCTION_BLOCK Inner VAR_OUTPUT q : INT; END_VAR q := q + 10; END_FUNCTION_BLOCK " +
			"FUNCTION_BLOCK Outer VAR sub : Inner; END_VAR END_FUNCTION_BLOCK VAR o : Outer; res : INT; END_VAR o.sub(); o.sub(q => res); res;", 20},
		// VAR_TEMP starts afresh on every call.
		{"FUNCTION_BLOCK Fb VAR_TEMP t1 : INT := 5; END_VAR VAR_OUTPUT o : INT; END_VAR t1 := t1 + 1; o := t1; END_FUNCTION_BLOCK VAR f : Fb; res : INT; END_VAR f(); f(o => res); res;", 6},
		// Function blocks call functions, whichever is declared first.
		{"FUNCTION_BLOCK Fb METHOD M : INT M := F(); END_METHOD END_FUNCTION_BLOCK FUNCTION F : INT F := 4; END_FUNCTION VAR f : Fb; END_VAR f.M();", 4},
		{"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 4; F := 1; END_FUNCTION FUNCTION_BLOCK Fb VAR_OUTPUT total : INT; END_VAR F(o => total); END_FUNCTION_BLOCK VAR f : Fb; res : INT; END_VAR f(total => res); res;", 4},
	})
}

// Declaring a PROGRAM defines it; calling it runs its body. Its variables
// keep their values between calls and can be read as members.
func TestProgramCalls(t *testing.T) {
	const twice = "PROGRAM Pg VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := i * 2; END_PROGRAM "
	runVmTests(t, []vmTestCase{
		{twice + "VAR res : INT; END_VAR Pg(i := 4, o => res); res;", 8},
		{twice + "Pg(i := 5); Pg.o;", 10},
		// Declaring a program does not run it.
		{"PROGRAM Pc VAR n : INT; END_VAR n := n + 1; END_PROGRAM Pc.n;", 0},
		{"PROGRAM Pc VAR n : INT; END_VAR n := n + 1; END_PROGRAM Pc(); Pc(); Pc.n;", 2},
		// VAR_TEMP starts afresh on every call.
		{"PROGRAM Pt VAR_TEMP t1 : INT; END_VAR VAR_OUTPUT o : INT; END_VAR t1 := t1 + 1; o := t1; END_PROGRAM Pt(); Pt(); Pt.o;", 1},
		// A VAR_IN_OUT is copied in and back.
		{"PROGRAM Pio VAR_IN_OUT io : INT; END_VAR io := io * 3; END_PROGRAM VAR v : INT := 2; END_VAR Pio(io := v); Pio(io := v); v;", 18},
		// Globals declared in a program stay global.
		{"PROGRAM Pgl VAR_GLOBAL g : INT := 1; END_VAR g := g + 1; END_PROGRAM Pgl(); g;", 2},
		// Two programs keep their own variables of the same name.
		{"PROGRAM A1 VAR n : INT := 1; END_VAR n := n + 1; END_PROGRAM PROGRAM B1 VAR n : INT := 10; END_VAR n := n + 1; END_PROGRAM A1(); B1(); A1.n * 100 + B1.n;", 211},
		// Programs call functions and function blocks, and EN/ENO work.
		{"FUNCTION F : INT F := 7; END_FUNCTION PROGRAM Pf VAR_OUTPUT o : INT; END_VAR o := F(); END_PROGRAM Pf(); Pf.o;", 7},
		{"FUNCTION_BLOCK Cnt VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK PROGRAM Pb VAR c : Cnt; END_VAR c(); END_PROGRAM Pb(); Pb(); Pb.c.n;", 2},
		{twice + "VAR ok : BOOL := TRUE; END_VAR Pg(EN := FALSE, i := 1, ENO => ok); ok;", false},
		// A program's variable hides a class or global of the same name.
		{"FUNCTION_BLOCK Counter VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK PROGRAM Pk VAR counter : Counter; END_VAR counter(); END_PROGRAM Pk(); Pk.counter.n;", 1},
		// IL programs.
		{"PROGRAM Pi VAR n : INT; q : BOOL; END_VAR LD n ADD 1 ST n LD n GT 1 S q END_PROGRAM Pi(); Pi(); Pi.q;", true},
	})
}

// A function's VAR_IN_OUT is copied in and its final value copied back.
func TestFunctionInOuts(t *testing.T) {
	const inc = "FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR io := io + 1; Inc := io * 10; END_FUNCTION "
	runVmTests(t, []vmTestCase{
		{inc + "VAR v : INT := 4; res : INT; END_VAR res := Inc(v); res + v;", 55},
		{inc + "VAR v : INT := 4; END_VAR Inc(io := v); Inc(io := v); v;", 6},
		{"FUNCTION Swap : BOOL VAR_IN_OUT a : INT; b : INT; END_VAR VAR t1 : INT; END_VAR t1 := a; a := b; b := t1; Swap := TRUE; END_FUNCTION VAR x : INT := 1; y : INT := 2; END_VAR Swap(x, y); x * 10 + y;", 21},
		// With an input and an output alongside.
		{"FUNCTION AddTo : INT VAR_INPUT n : INT; END_VAR VAR_IN_OUT acc : INT; END_VAR VAR_OUTPUT old : INT; END_VAR old := acc; acc := acc + n; AddTo := acc; END_FUNCTION VAR s1 : INT := 10; o1 : INT; END_VAR AddTo(n := 5, acc := s1, old => o1); o1 * 100 + s1;", 1015},
		{inc + "VAR a : ARRAY[0..1] OF INT; END_VAR Inc(a[1]); a;", []int{0, 1}},
		// An in-out passed on to another function.
		{"FUNCTION G : INT VAR_IN_OUT io : INT; END_VAR io := io * 2; G := io; END_FUNCTION FUNCTION F : INT VAR_IN_OUT io : INT; END_VAR G(io); F := io; END_FUNCTION VAR v : INT := 4; END_VAR F(v); v;", 8},
		// A function block's variable as the argument.
		{inc + "FUNCTION_BLOCK Fb VAR_OUTPUT n : INT; END_VAR Inc(n); END_FUNCTION_BLOCK VAR f : Fb; res : INT; END_VAR f(); f(n => res); res;", 2},
	})
}

// Arrays are indexed from their declared lower bound.
func TestArrayLowerBounds(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"VAR a : ARRAY[1..3] OF INT := [10, 20, 30]; END_VAR a[1];", 10},
		{"VAR a : ARRAY[1..3] OF INT := [10, 20, 30]; END_VAR a[3] := 5; a;", []int{10, 20, 5}},
		// An array assigned to the variable takes its bounds.
		{"VAR a : ARRAY[1..3] OF INT; END_VAR a := [7, 8, 9]; a[1];", 7},
		{"VAR a : ARRAY[-1..1] OF INT := [1, 2, 3]; END_VAR a[-1] * 100 + a[0] * 10 + a[1];", 123},
		{"VAR m : ARRAY[1..2, 1..3] OF INT; END_VAR m[2][3] := 9; m[2][3] * 10 + m[1][1];", 90},
		{"VAR a : ARRAY[1..3] OF INT; i : INT; s1 : INT; END_VAR FOR i := 1 TO 3 DO a[i] := i * i; END_FOR FOR i := 1 TO 3 DO s1 := s1 + a[i]; END_FOR s1;", 14},
		{"VAR a : ARRAY[1..3] OF INT := [1, 2, 3]; i : INT := 4; END_VAR a[i];", Null},
		// A parameter indexes its argument as declared.
		{"FUNCTION Second : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR Second := v[2]; END_FUNCTION Second([4, 5, 6]);", 5},
		// Named array types, in structures and function blocks.
		{"TYPE Row : ARRAY[1..3] OF INT; END_TYPE VAR x : Row; END_VAR x[1] := 11; x[1];", 11},
		{"TYPE Row : ARRAY[1..3] OF INT; END_TYPE TYPE Rec : STRUCT row1 : Row; END_STRUCT; END_TYPE VAR x : Rec; END_VAR x.row1[1] := 11; x.row1 := [1, 2, 3]; x.row1[3] * 100 + x.row1[1];", 301},
		{"FUNCTION_BLOCK Fb VAR buf : ARRAY[1..2] OF INT; END_VAR VAR_OUTPUT first : INT; END_VAR buf := [4, 5]; first := buf[1]; END_FUNCTION_BLOCK VAR f : Fb; res : INT; END_VAR f(first => res); res;", 4},
	})
	runVmErrorTests(t, []vmErrorTestCase{
		// Errors report the declared index.
		{"VAR a : ARRAY[1..3] OF INT := [1, 2, 3]; i : INT := 0; END_VAR a[i] := 1;", "array index out of bounds: 0"},
	})
}

// Arrays and structures are assigned and passed by value: changing a copy
// leaves the original alone. A VAR_IN_OUT still changes its argument.
func TestArrayAndStructureCopies(t *testing.T) {
	const pt = "TYPE Pt : STRUCT x : INT; y : INT; END_STRUCT; END_TYPE "
	runVmTests(t, []vmTestCase{
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; b : ARRAY[0..2] OF INT; END_VAR b := a; b[0] := 9; a[0] * 10 + b[0];", 19},
		{pt + "VAR p : Pt; q : Pt; END_VAR p.x := 1; q := p; q.x := 5; p.x * 10 + q.x;", 15},
		// Nested arrays and structures are copied too, and bounds are kept.
		{"TYPE Rec : STRUCT row1 : ARRAY[1..2] OF INT; END_STRUCT; END_TYPE VAR p : Rec; q : Rec; END_VAR q := p; q.row1[1] := 6; p.row1[1] * 10 + q.row1[1];", 6},
		{"VAR m : ARRAY[0..1, 0..1] OF INT; row : ARRAY[0..1] OF INT; END_VAR m[0] := row; row[0] := 4; m[0][0];", 0},
		{pt + "VAR ps : ARRAY[0..1] OF Pt; q : Pt; END_VAR ps[0] := q; q.x := 3; ps[0].x;", 0},
		// A function's array input is its own copy, so taking the declared
		// bounds leaves the caller's array alone.
		{"FUNCTION F : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR F := v[1]; END_FUNCTION VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR F(a) * 10 + a[0];", 11},
		{"FUNCTION F : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR F := v[1]; END_FUNCTION VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR F(v := a) * 10 + a[0];", 11},
		// A function block keeps copies of its inputs and hands out copies of its outputs.
		{pt + "FUNCTION_BLOCK Fb VAR_INPUT p : Pt; END_VAR END_FUNCTION_BLOCK VAR f : Fb; q : Pt; END_VAR f(p := q); q.x := 7; f.p.x;", 0},
		{"FUNCTION_BLOCK Fb VAR_INPUT v : ARRAY[0..1] OF INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; a : ARRAY[0..1] OF INT; END_VAR f(v := a); a[0] := 5; f.v[0];", 0},
		{"FUNCTION_BLOCK Fb VAR_OUTPUT v : ARRAY[0..1] OF INT; END_VAR v[0] := v[0] + 1; END_FUNCTION_BLOCK VAR f : Fb; a : ARRAY[0..1] OF INT; END_VAR f(v => a); a[0] := 50; f(); f.v[0];", 2},
		{"TYPE Buf : ARRAY[0..1] OF INT; END_TYPE FUNCTION_BLOCK Fb VAR buf : Buf; END_VAR METHOD Snapshot : Buf Snapshot := buf; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; a : Buf; END_VAR a := f.Snapshot(); a[0] := 8; f.buf[0];", 0},
		// A VAR_IN_OUT changes its argument.
		{"FUNCTION F : INT VAR_IN_OUT v : ARRAY[0..2] OF INT; END_VAR v[0] := 100; F := 0; END_FUNCTION VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR F(a); a[0];", 100},
	})
}

// A type declared in a namespace is named with its namespace.
func TestNamespacedEnumLiterals(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"NAMESPACE Lib TYPE Mode : (Idle, Busy); END_TYPE END_NAMESPACE VAR m : Lib.Mode; END_VAR m := Lib.Mode#Busy; m = Lib.Mode#Busy;", true},
	})
}

// An IF leaves the stack as it found it, whichever branch runs, at the top
// level and in a loop.
func TestIfKeepsTheStackBalanced(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"VAR b : BOOL := TRUE; c : INT; END_VAR IF b THEN c := 5; END_IF c;", 5},
		{"VAR b : BOOL; c : INT := 1; END_VAR IF b THEN c := 5; ELSIF NOT b THEN c := 7; END_IF; c;", 7},
		// Many IFs that do not run, and many that do, in a loop.
		{"FUNCTION F : INT VAR i : INT; n : INT; END_VAR FOR i := 1 TO 5000 DO IF i > 9000 THEN n := n + 1; END_IF; IF i > 0 THEN n := n + 1; END_IF; END_FOR F := n; END_FUNCTION F();", 5000},
	})
}

// A bit of an integer or bit string is read and written as a BOOL; the
// variable keeps its type.
func TestBitAccess(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{"VAR x : BYTE := 16#0A; END_VAR x.3;", true},
		{"VAR x : BYTE := 16#0A; END_VAR x.0;", false},
		{"VAR x : INT := -1; END_VAR x.15 := FALSE; x;", 32767},
		{"VAR x : INT := 5; END_VAR x.15 := TRUE; x;", -32763},
		{"VAR a : ARRAY[0..1] OF INT; END_VAR a[1].2 := TRUE; a[1];", 4},
		{"FUNCTION_BLOCK Fb VAR_OUTPUT f : INT; END_VAR f.1 := TRUE; END_FUNCTION_BLOCK VAR fb : Fb; END_VAR fb(); fb.f;", 2},
		{"VAR x : BYTE := 16#0A; b : BOOL; END_VAR IF x.1 AND NOT x.0 THEN b := TRUE; END_IF b;", true},
	})
	runVmErrorTests(t, []vmErrorTestCase{
		{"VAR x : INT; END_VAR x.16;", "bit 16 is outside the 16 bits of INT"},
		{"VAR x : INT; END_VAR x.20 := TRUE;", "bit 20 is outside the 16 bits of INT"},
		{"VAR x : REAL; END_VAR x.1;", "bit access needs an integer or bit string, got LREAL"},
	})
}
