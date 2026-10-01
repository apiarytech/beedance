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

import "testing"

// TestInvalidInitialValues checks that an initial value must fit the declared
// type: its kind, its range, and for arrays, structures and function blocks,
// its shape.
func TestInvalidInitialValues(t *testing.T) {
	const pt = "TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE "
	const color = "TYPE Color : (Red, Green); END_TYPE "
	const rng = "TYPE Rng : INT(1..10); END_TYPE "
	const fb = "FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK "
	runCompileErrorTests(t, []compileErrorCase{
		// Elementary types
		{"SINT out of range", "VAR x : SINT := 300; END_VAR", "initial value of 'x': value 300 is out of range for type SINT (-128 to 127)"},
		{"USINT negative", "VAR x : USINT := -1; END_VAR", "initial value of 'x': value -1 is out of range for type USINT (0 to 255)"},
		{"INT from STRING", "VAR x : INT := 'hi'; END_VAR", "cannot initialize 'x' of type INT with a value of type STRING"},
		{"BOOL from integer", "VAR x : BOOL := 5; END_VAR", "cannot initialize 'x' of type BOOL with a value of type LINT"},
		{"REAL from INT variable", "VAR n : INT := 2; x : REAL := n; END_VAR", "cannot initialize 'x' of type REAL with a value of type INT"},
		{"STRING from WSTRING", `VAR x : STRING := "wide"; END_VAR`, "cannot initialize 'x' of type STRING with a value of type WSTRING"},
		{"WSTRING from STRING", "VAR x : WSTRING := 'narrow'; END_VAR", "cannot initialize 'x' of type WSTRING with a value of type STRING"},
		{"TIME from integer", "VAR x : TIME := 5; END_VAR", "cannot initialize 'x' of type TIME with a value of type LINT"},
		{"TIME from DATE", "VAR x : TIME := D#2024-01-01; END_VAR", "cannot initialize 'x' of type TIME with a value of type DATE"},
		{"BYTE too wide", "VAR x : BYTE := 16#1FF; END_VAR", "initial value of 'x': 511 does not fit in BYTE"},
		{"BYTE negative", "VAR x : BYTE := -1; END_VAR", "initial value of 'x': -1 does not fit in BYTE"},
		{"BYTE from BOOL", "VAR x : BYTE := TRUE; END_VAR", "cannot initialize 'x' of type BYTE with a value of type BOOLEAN"},
		// Arrays
		{"too many elements", "VAR a : ARRAY[0..1] OF INT := [1, 2, 3]; END_VAR", "array 'a' has 2 elements but its initial value lists 3"},
		{"too many repeated elements", "VAR a : ARRAY[0..1] OF INT := [3(7)]; END_VAR", "array 'a' has 2 elements but its initial value lists 3"},
		{"wrong element type", "VAR a : ARRAY[0..1] OF INT := [1, 'x']; END_VAR", "cannot initialize 'a' of type INT with a value of type STRING"},
		{"non-constant bounds", "VAR n : INT := 3; a : ARRAY[0..n] OF INT; END_VAR", "array 'a' must have constant bounds, got (0 .. n)"},
		// Structures and function blocks
		{"unknown structure member", pt + "VAR p : Pt := (nope := 1); END_VAR", "structure 'Pt' has no member 'nope'"},
		{"wrong member type", pt + "VAR p : Pt := (px := 'x'); END_VAR", "cannot initialize 'px' of type INT with a value of type STRING"},
		{"structure from a number", pt + "VAR p : Pt := (1); END_VAR", "cannot initialize 'p' of type Pt with a value of type LINT"},
		{"structure that contains itself", "TYPE Loop : STRUCT inner : Loop; END_STRUCT; END_TYPE VAR l : Loop; END_VAR", "type 'Loop' contains itself"},
		{"unknown FB variable", fb + "VAR f : Fb := (nope := 1); END_VAR", "function block 'Fb' has no variable 'nope'"},
		{"initializer on an elementary type", "VAR x : INT := (a := 1); END_VAR",
			"the initial value of 'x' is a structure initializer, which requires a structure or function block type, but 'INT' is neither"},
		{"initializer on an unsupported type", "VAR t : Missing := (PT := T#1s); END_VAR",
			"the initial value of 't' is a structure initializer, but type 'MISSING' is not supported by the compiler"},
		{"initializer outside a declaration", "x := (a := 1);",
			"a structure initializer (a := 1) is only allowed as the initial value of a structure or function block variable"},
		// Enumerations and subranges
		{"unknown enumeration value", color + "VAR c : Color := Purple; END_VAR", "initial value of 'c': 'Purple' is not a value of enumeration 'Color'"},
		{"unknown typed enumeration value", color + "VAR c : Color := Color#Purple; END_VAR", "initial value of 'c': 'Purple' is not a value of enumeration 'Color'"},
		{"other enumeration", color + "TYPE Other : (A1, B1); END_TYPE VAR c : Color := Other#A1; END_VAR", "cannot initialize 'c' of type Color with a value of type Other"},
		{"above subrange", rng + "VAR rv : Rng := 50; END_VAR", "initial value of 'rv': 50 is out of range for subrange type 'Rng' (1..10)"},
		{"below subrange", rng + "VAR rv : Rng := 0; END_VAR", "initial value of 'rv': 0 is out of range for subrange type 'Rng' (1..10)"},
		// Constants and parameters
		{"assigning a constant", "VAR CONSTANT k : INT := 1; END_VAR k := 2;", "cannot assign to a constant variable 'k'"},
		{"unknown input", "FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION F(z := 1);", "'F' has no input 'z'"},
		{"bad output initial value", "FUNCTION F : INT VAR_OUTPUT o : BOOL := 3; END_VAR F := 1; END_FUNCTION", "cannot initialize 'o' of type BOOL with a value of type LINT"},
		{"bad input default", "FUNCTION F : INT VAR_INPUT a : INT := 'x'; END_VAR F := a; END_FUNCTION F();", "cannot initialize 'a' of type INT with a value of type STRING"},
	})
}
