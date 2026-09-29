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

import (
	"beedance/object"
	"strings"
	"testing"
)

// checkEvaluated evaluates each input and compares the printed form of its
// result. Integer results are compared by value, since the evaluator keeps
// declared integer types (INT, LINT, ...).
func checkEvaluated(t *testing.T, tests []struct{ input, want string }) {
	t.Helper()
	for _, tt := range tests {
		result := testEvalWithEnv(t, tt.input, object.NewEnvironment())
		if result == nil {
			t.Errorf("%s: got no result", tt.input)
			continue
		}
		if got := result.Inspect(); got != tt.want {
			t.Errorf("%s:\n  expected %s\n  got      %s", tt.input, tt.want, got)
		}
	}
}

func TestStructureVariables(t *testing.T) {
	const pt = "TYPE Pt : STRUCT px : INT := 4; py : BOOL; tag : STRING; END_STRUCT; END_TYPE "
	checkEvaluated(t, []struct{ input, want string }{
		// Members take their declared initial values, or their type's defaults.
		{pt + "VAR p : Pt; END_VAR p.px;", "4"},
		{pt + "VAR p : Pt; END_VAR p.py;", "false"},
		// An initializer overrides the named members only.
		{pt + "VAR p : Pt := (px := 9); END_VAR p.px + 1;", "10"},
		{pt + "VAR p : Pt := (py := TRUE); END_VAR p.px;", "4"},
		// Members can be assigned.
		{pt + "VAR p : Pt; END_VAR p.px := 6; p.px;", "6"},
		// Nested structures.
		{pt + "TYPE Line : STRUCT a : Pt; b : Pt := (px := 7); END_STRUCT; END_TYPE VAR seg : Line; END_VAR seg.b.px * 10 + seg.a.px;", "74"},
	})
}

func TestEnumerationVariables(t *testing.T) {
	const types = "TYPE Color : (Red, Green, Blue); END_TYPE TYPE Shade : (Light, Dark) := Dark; END_TYPE "
	checkEvaluated(t, []struct{ input, want string }{
		{types + "VAR c : Color; END_VAR c;", "COLOR#Red"},  // first value
		{types + "VAR c : Shade; END_VAR c;", "SHADE#Dark"}, // the type's initial value
		{types + "VAR c : Color := Blue; END_VAR c;", "COLOR#Blue"},
		{types + "VAR c : Color := Color#Green; END_VAR c;", "COLOR#Green"},
		{types + "VAR c : Color := Color#Green; END_VAR c = Color#Green;", "true"},
		{types + "VAR c : Color := Color#Green; END_VAR c <> Color#Red;", "true"},
		{types + "VAR c : Color := Blue; res : INT := 0; END_VAR CASE c OF Color#Red: res := 1; Color#Blue: res := 3; END_CASE; res;", "3"},
	})
}

func TestStructureInitializerErrors(t *testing.T) {
	const pt = "TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE "
	const fb = "FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK "
	tests := []struct{ input, want string }{
		{pt + "VAR p : Pt := (nope := 1); END_VAR p;", "structure 'Pt' has no member 'nope'"},
		{fb + "VAR f : Fb := (nope := 1); END_VAR f;", "function block 'Fb' has no variable 'nope'"},
		{"TYPE Color : (Red, Green); END_TYPE VAR c : Color := Purple; END_VAR c;", "initial value of 'c': 'Purple' is not a value of enumeration 'Color'"},
		{"VAR x : INT := (a := 1); END_VAR x;", "the initial value of 'x' is a structure initializer, which requires a structure or function block type, but 'INT' is neither"},
	}
	for _, tt := range tests {
		result := testEvalWithEnv(t, tt.input, object.NewEnvironment())
		errObj, ok := result.(*object.Error)
		if !ok {
			t.Errorf("%s: expected an error, got %v", tt.input, result)
			continue
		}
		if !strings.HasSuffix(errObj.Message, tt.want) {
			t.Errorf("%s:\n  expected an error ending with %q\n  got %q", tt.input, tt.want, errObj.Message)
		}
	}
}

func TestFunctionBlockInitializers(t *testing.T) {
	const fb = `FUNCTION_BLOCK Fb
		VAR_INPUT i : INT := 4; END_VAR
		VAR v : INT := 3; w : REAL; END_VAR
		METHOD Sum : INT Sum := v + i; END_METHOD
		METHOD Wv : REAL Wv := w; END_METHOD
	END_FUNCTION_BLOCK `
	checkEvaluated(t, []struct{ input, want string }{
		{fb + "VAR f : Fb; END_VAR f.Sum();", "7"},
		{fb + "VAR f : Fb := (v := 9, i := 1); END_VAR f.Sum();", "10"},
		{fb + "VAR f : Fb; END_VAR f.Wv();", "0.000000"},
		// A standard function block with an initializer.
		{"VAR t : TON := (PT := T#1s); END_VAR t.PT;", "T#1s"},
	})
}

func TestCallInitialValues(t *testing.T) {
	const inc = "FUNCTION Inc : INT VAR_INPUT a : INT := 5; b : INT; END_VAR Inc := a * 10 + b; END_FUNCTION "
	checkEvaluated(t, []struct{ input, want string }{
		// A formal call may omit inputs; they take their defaults.
		{inc + "Inc();", "50"},
		{inc + "Inc(b := 2);", "52"},
		{inc + "Inc(3, 4);", "34"},
		// Results start at the return type's default.
		{"FUNCTION F : INT END_FUNCTION F();", "0"},
		{"FUNCTION F : BOOL END_FUNCTION F();", "false"},
		// VAR_OUTPUTs start at their initial value.
		{"FUNCTION Out : INT VAR_OUTPUT o : INT := 7; END_VAR Out := 1; END_FUNCTION VAR res : INT; END_VAR Out(o => res); res;", "7"},
		// Methods: result default, input default, and locals initialized per call.
		{"FUNCTION_BLOCK G METHOD M : INT END_METHOD END_FUNCTION_BLOCK VAR g : G; END_VAR g.M();", "0"},
		{"FUNCTION_BLOCK G METHOD M : INT VAR_INPUT a : INT := 3; END_VAR M := a; END_METHOD END_FUNCTION_BLOCK VAR g : G; END_VAR g.M();", "3"},
		{"FUNCTION_BLOCK G METHOD M : INT VAR c : INT := 10; END_VAR c := c + 1; M := c; END_METHOD END_FUNCTION_BLOCK VAR g : G; END_VAR g.M() * 100 + g.M();", "1111"},
	})
}
