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
	"os"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// TestOscatLibraryRuns evaluates the converted OSCAT BASIC library with a
// call to some of its functions. It is skipped without the library.
func TestOscatLibraryRuns(t *testing.T) {
	library, err := os.ReadFile("../reference/beedance_oscat_basic.st")
	if err != nil {
		t.Skipf("OSCAT library not present: %v", err)
	}
	p := parser.New(lexer.New(string(library)))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("the library does not parse: %v", p.Errors()[0])
	}
	env := object.NewEnvironment()
	if result := Eval(program, env); isError(result) {
		t.Fatalf("the library does not evaluate: %s", result.Inspect())
	}
	for call, want := range map[string]interface{}{
		"GCD(12, 18)":                         6,
		"BIT_COUNT(DWORD#16#F0F0)":            8,
		"WORK_WEEK(D#2024-05-06)":             19,
		"DST(DT#2024-07-01-12:00:00)":         true,
		"DST(DT#2024-01-15-12:00:00)":         false,
		"BCDC_TO_INT(BYTE#16#42)":             42,
		"FIB(10)":                             55,
		"YEAR_OF_DATE(D#2024-05-06)":          2024,
		"DAY_OF_WEEK(D#2024-05-06)":           1,
		"LEAP_YEAR(2024)":                     true,
		"SET_DATE(2024, 3, 1) = D#2024-03-01": true,
		"HOUR(TOD#13:45:00)":                  13,
		"EXPN(2.0, 10) = 1024.0":              true,
		"DEG_TO_DIR(90, 2, 0) = 'E'":          true,
	} {
		p := parser.New(lexer.New(call + ";"))
		got := Eval(p.ParseProgram(), env)
		switch want := want.(type) {
		case int:
			if n, _, ok := object.GetIntegerObjectValue(got); !ok || n != int64(want) {
				t.Errorf("%s = %s, want %d", call, got.Inspect(), want)
			}
		case bool:
			if b, ok := got.(*object.Boolean); !ok || b.Value != want {
				t.Errorf("%s = %s, want %v", call, got.Inspect(), want)
			}
		}
	}
}

// TestCaseInsensitiveNames checks that identifiers ignore case, as in
// IEC 61131-3.
func TestCaseInsensitiveNames(t *testing.T) {
	for input, want := range map[string]int64{
		"VAR abc : INT := 5; END_VAR ABC;":                                                                       5,
		"VAR t : INT := 5; END_VAR T := t + 1; t;":                                                               6,
		"FUNCTION F : INT VAR x : INT; END_VAR X := 3; f := x; END_FUNCTION f();":                                3,
		"FUNCTION_BLOCK Fb VAR_OUTPUT q : INT; END_VAR Q := 7; END_FUNCTION_BLOCK VAR i : FB; END_VAR I(); i.Q;": 7,
	} {
		testIntegerObject(t, testEval(t, input), input, want)
	}
}

// TestMultiDimensionalInitialValues checks that a multi-dimensional array's
// initial value, written as one list, fills the array in row order.
func TestMultiDimensionalInitialValues(t *testing.T) {
	for input, want := range map[string]int64{
		"VAR a : ARRAY[1..2,0..1] OF INT := [1,2,3,4]; END_VAR a[2,1];":          4,
		"VAR a : ARRAY[1..2,0..1] OF INT := [1,2,3,4]; END_VAR a[1,1];":          2,
		"VAR a : ARRAY[1..2,0..1] OF INT := [2(7), 9]; END_VAR a[2,0] + a[2,1];": 9,
		"VAR a : ARRAY[1..2,0..1] OF INT := [1]; END_VAR a[2,1];":                0,
		"VAR a : ARRAY[1..2,0..1] OF INT := [[1,2],[3,4]]; END_VAR a[2,0];":      3,
	} {
		testIntegerObject(t, testEval(t, input), input, want)
	}
	if got := testEval(t, "VAR a : ARRAY[1..2,0..1] OF INT := [1,2,3,4,5]; END_VAR a;"); !isError(got) {
		t.Errorf("five elements for four: %s", got.Inspect())
	}
}
