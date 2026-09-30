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

// Arrays are indexed from their declared lower bound, as in the VM.
func TestArrayLowerBounds(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR a : ARRAY[1..3] OF INT := [10, 20, 30]; END_VAR a[1];", "10"},
		{"VAR a : ARRAY[1..3] OF INT := [10, 20, 30]; END_VAR a[3] := 5; a;", "[10, 20, 5]"},
		// An array assigned to the variable takes its bounds.
		{"VAR a : ARRAY[1..3] OF INT; END_VAR a := [7, 8, 9]; a[1];", "7"},
		{"VAR a : ARRAY[-1..1] OF INT := [1, 2, 3]; END_VAR a[-1] * 100 + a[0] * 10 + a[1];", "123"},
		{"VAR m : ARRAY[1..2, 1..3] OF INT; END_VAR m[2][3] := 9; m[2][3] * 10 + m[1][1];", "90"},
		{"VAR a : ARRAY[1..3] OF INT; i : INT; s1 : INT; END_VAR FOR i := 1 TO 3 DO a[i] := i * i; END_FOR FOR i := 1 TO 3 DO s1 := s1 + a[i]; END_FOR s1;", "14"},
		{"VAR a : ARRAY[1..3] OF INT := [1, 2, 3]; i : INT := 4; END_VAR a[i];", "null"},
		{"VAR a : ARRAY[1..3] OF INT := [1, 2, 3]; i : INT := 0; END_VAR a[i] := 1;", "ERROR: index out of bounds: 0"},
		// A parameter indexes its argument as declared.
		{"FUNCTION Second : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR Second := v[2]; END_FUNCTION Second([4, 5, 6]);", "5"},
		{"FUNCTION_BLOCK Fb METHOD M : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR M := v[3]; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.M([4, 5, 6]);", "6"},
		// Named array types, in structures and function blocks.
		{"TYPE Row : ARRAY[1..3] OF INT; END_TYPE VAR x : Row; END_VAR x[1] := 11; x[1];", "11"},
		{"TYPE Row : ARRAY[1..3] OF INT := [4]; END_TYPE VAR x : Row; END_VAR x;", "[4, 0, 0]"},
		{"TYPE Row : ARRAY[1..3] OF INT; END_TYPE TYPE Rec : STRUCT row1 : Row; END_STRUCT; END_TYPE VAR x : Rec; END_VAR x.row1[1] := 11; x.row1 := [1, 2, 3]; x.row1[3] * 100 + x.row1[1];", "301"},
		{"FUNCTION_BLOCK Fb VAR buf : ARRAY[1..2] OF INT; END_VAR VAR_OUTPUT first : INT; END_VAR buf := [4, 5]; first := buf[1]; END_FUNCTION_BLOCK VAR f : Fb; res : INT; END_VAR f(first => res); res;", "4"},
		{"VAR i : INT; END_VAR i[0] := 1;", "ERROR: index operator not supported for assignment"},
	})
}
