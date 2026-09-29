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

const loopVars = "VAR i : INT; j : INT; delta : INT := -2; total : INT := 0; END_VAR "

func TestForLoops(t *testing.T) {
	tests := []vmTestCase{
		{loopVars + "FOR i := 1 TO 5 DO total := total + i; END_FOR; total;", 15},
		{loopVars + "FOR i := 1 TO 10 BY 3 DO total := total + i; END_FOR; total;", 22}, // 1+4+7+10
		// A negative step counts down.
		{loopVars + "FOR i := 10 TO 1 BY -3 DO total := total + i; END_FOR; total;", 22}, // 10+7+4+1
		// A step held in a variable has its direction checked at runtime.
		{loopVars + "FOR i := 6 TO 1 BY delta DO total := total + i; END_FOR; total;", 12}, // 6+4+2
		// The body does not run when the range is empty.
		{loopVars + "FOR i := 5 TO 1 DO total := total + 1; END_FOR; total;", 0},
		// The control variable keeps its value after the loop.
		{loopVars + "FOR i := 1 TO 3 DO END_FOR; i;", 4},
		{loopVars + "FOR i := 1 TO 3 DO FOR j := 1 TO 4 DO total := total + 1; END_FOR; END_FOR; total;", 12},
	}
	runVmTests(t, tests)
}

func TestWhileAndRepeatLoops(t *testing.T) {
	tests := []vmTestCase{
		{"VAR i : INT := 0; END_VAR WHILE i < 7 DO i := i + 2; END_WHILE; i;", 8},
		{"VAR i : INT := 9; END_VAR WHILE i < 7 DO i := i + 2; END_WHILE; i;", 9},
		{"VAR i : INT := 0; END_VAR REPEAT i := i + 3; UNTIL i > 10 END_REPEAT; i;", 12},
		// REPEAT runs its body at least once.
		{"VAR i : INT := 50; END_VAR REPEAT i := i + 1; UNTIL TRUE END_REPEAT; i;", 51},
	}
	runVmTests(t, tests)
}

func TestExitLeavesInnermostLoop(t *testing.T) {
	tests := []vmTestCase{
		{loopVars + "FOR i := 1 TO 10 DO IF i > 3 THEN EXIT; END_IF total := total + i; END_FOR; total;", 6},
		{"VAR i : INT := 0; END_VAR WHILE TRUE DO i := i + 1; IF i = 4 THEN EXIT; END_IF END_WHILE; i;", 4},
		{"VAR i : INT := 0; END_VAR REPEAT i := i + 1; IF i = 2 THEN EXIT; END_IF UNTIL FALSE END_REPEAT; i;", 2},
		// EXIT in the inner loop only ends the inner loop.
		{loopVars + `FOR i := 1 TO 3 DO
			FOR j := 1 TO 3 DO
				IF j = 2 THEN EXIT; END_IF
				total := total + 1;
			END_FOR;
		END_FOR;
		total;`, 3},
		// EXIT in the outer loop after an inner loop ends the outer loop.
		{loopVars + `FOR i := 1 TO 5 DO
			FOR j := 1 TO 2 DO total := total + 1; END_FOR;
			IF i = 2 THEN EXIT; END_IF
		END_FOR;
		total * 10 + i;`, 42},
	}
	runVmTests(t, tests)
}

func TestCaseStatements(t *testing.T) {
	caseOn := func(selector string) string {
		return "VAR x : INT := " + selector + "; res : INT := 0; END_VAR " +
			"CASE x OF 1: res := 10; 2, 3: res := 20; 5..9: res := 50; 12, 20..22: res := 70; ELSE res := 99; END_CASE; res;"
	}
	tests := []vmTestCase{
		{caseOn("1"), 10},
		{caseOn("2"), 20},
		{caseOn("3"), 20}, // second value of a list
		{caseOn("5"), 50}, // range bounds are inclusive
		{caseOn("9"), 50},
		{caseOn("12"), 70}, // a list mixing a value and a range
		{caseOn("21"), 70},
		{caseOn("4"), 99},
		{caseOn("42"), 99},
		// Without ELSE, an unmatched selector changes nothing.
		{"VAR x : INT := 8; res : INT := 5; END_VAR CASE x OF 1: res := 10; END_CASE; res;", 5},
		// Enumerated values as labels.
		{`TYPE Color : (Red, Green, Blue); END_TYPE
		  VAR c : Color := Color#Green; res : INT := 0; END_VAR
		  CASE c OF Color#Red: res := 1; Color#Green, Color#Blue: res := 2; END_CASE; res;`, 2},
	}
	runVmTests(t, tests)
}
