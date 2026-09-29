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
	"beedance/code"
	"beedance/object"
	"testing"
	"time"
)

// TestFunctionResultAssignment checks IEC 61131-3 function results: assigning
// to the function's name sets the result but does not return, so the rest of
// the body still runs. The function returns its result at the end or at RETURN.
func TestFunctionResultAssignment(t *testing.T) {
	tests := []vmTestCase{
		{"FUNCTION f : INT f := 1; f := f + 1; END_FUNCTION f();", 2},
		{`FUNCTION f : INT VAR_INPUT n : INT; END_VAR
			IF n > 0 THEN f := 1; END_IF
			f := f + 10;
		  END_FUNCTION
		  f(1);`, 11},
		// RETURN ends the function with its current result.
		{`FUNCTION f : INT VAR_INPUT n : INT; END_VAR
			IF n > 0 THEN f := 1; RETURN; END_IF;
			f := 5;
		  END_FUNCTION
		  f(1) * 10 + f(0);`, 15},
		// `RETURN value` sets the result and returns.
		{"FUNCTION f : INT RETURN 7; END_FUNCTION f();", 7},
		// Methods and property getters follow the same rule.
		{`FUNCTION_BLOCK FB
			METHOD M : INT
				M := 1;
				M := M + 10;
			END_METHOD
		  END_FUNCTION_BLOCK
		  VAR x : FB; END_VAR
		  x.M();`, 11},
		{`FUNCTION_BLOCK FB
			PROPERTY P : INT
				GET
					P := 2;
					P := P * 3;
				END_GET
			END_PROPERTY
		  END_FUNCTION_BLOCK
		  VAR x : FB; END_VAR
		  x.P;`, 6},
	}
	runVmTests(t, tests)
}

func TestFunctionOutputsAreReturned(t *testing.T) {
	const scale = `
		FUNCTION Scale : INT
			VAR_INPUT n : INT; END_VAR
			VAR_OUTPUT doubled : INT; tripled : INT; END_VAR
			Scale := n;
			doubled := n * 2;
			tripled := n * 3;
		END_FUNCTION
		VAR d : INT; t : INT; END_VAR
	`
	tests := []vmTestCase{
		// The outputs are assigned to the `=>` targets.
		{scale + "Scale(4, doubled => d, tripled => t); d * 100 + t;", 812},
		// The call's own value is the function result.
		{scale + "Scale(4, doubled => d) + 1;", 5},
		// Output names are case-insensitive, like all identifiers.
		{scale + "Scale(5, DOUBLED => d); d;", 10},
		// Outputs can be ignored entirely.
		{scale + "Scale(3);", 3},
		// A recursive function with outputs still returns its own result.
		{`FUNCTION Sum : INT
			VAR_INPUT n : INT; END_VAR
			VAR_OUTPUT calls : INT; END_VAR
			IF n = 0 THEN Sum := 0; ELSE Sum := n + Sum(n - 1); END_IF
			calls := 1;
		  END_FUNCTION
		  Sum(4);`, 10},
	}
	runVmTests(t, tests)
}

// TestInferredOperandTypes covers operands whose type the compiler must work
// out: built-in calls, array elements and typed literals.
func TestInferredOperandTypes(t *testing.T) {
	tests := []vmTestCase{
		{"ABS(-3) + LEN('abcd');", 7},
		{"LEN('abc') > 2;", true},
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR a[2] * 10;", 30},
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR a[0] + a[1] = a[2];", true},
		{"INT#5 + INT#6;", 11},
		{"DINT#100 > INT#5;", true},
		{"LREAL#1.5 * 2.0;", 3.0},
	}
	runVmTests(t, tests)
}

func TestTimeAndDateOperations(t *testing.T) {
	object.FinalizeBuiltins()
	tests := []struct {
		input string
		want  string
	}{
		{"T#1s < T#2s;", "true"},
		{"T#1s = T#1000ms;", "true"},
		{"T#1s + T#500ms;", (1500 * time.Millisecond).String()},
		{"T#2s - T#500ms;", (1500 * time.Millisecond).String()},
		{"T#2s * 3;", (6 * time.Second).String()},
		{"T#3s / 2;", (1500 * time.Millisecond).String()},
		{"TOD#10:00:00 + T#1h;", "TOD#11:00:00"},
		{"TOD#10:30:00 - TOD#10:00:00;", (30 * time.Minute).String()},
		{"D#2024-01-02 - D#2024-01-01;", (24 * time.Hour).String()},
		{"D#2024-01-02 > D#2024-01-01;", "true"},
		{"DT#2024-01-01-12:00:00 > DT#2024-01-01-11:00:00;", "true"},
		{"BYTE#16#0F AND BYTE#16#3C;", "BYTE#16#C"},
		{"WORD#16#00FF = WORD#16#00FF;", "true"},
	}
	for _, tt := range tests {
		machine := New(compileForTest(t, tt.input))
		if err := machine.Run(); err != nil {
			t.Fatalf("%s: vm error: %s", tt.input, err)
		}
		got := machine.LastPoppedStackElem()
		text := got.Inspect()
		if d, ok := got.(*object.Time); ok {
			text = d.Value.String()
		}
		if text != tt.want {
			t.Fatalf("%s: expected %s, got %s", tt.input, tt.want, text)
		}
	}
}

func TestTimeOperationErrors(t *testing.T) {
	runVmErrorTests(t, []vmErrorTestCase{
		{"T#1s / 0;", "division by zero"},
	})

	// Operators with no time equivalent. The compiler rejects these, so they
	// are assembled directly.
	second := &object.Time{Value: time.Second}
	_, err := runBytecode([]object.Object{second, second}, nil,
		code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpExponent))
	if err == nil || err.Error() != "unsupported operator OpExponent for types TIME and TIME" {
		t.Fatalf("expected an unsupported operator error, got %v", err)
	}
	_, err = runBytecode([]object.Object{second, second}, nil,
		code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpMul))
	if err == nil || err.Error() != "unsupported operator '*' for types TIME and TIME" {
		t.Fatalf("expected TIME * TIME to be rejected, got %v", err)
	}
}
