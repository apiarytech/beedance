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

// ilProgram wraps IL instructions in a PROGRAM with the given variables, runs
// it, and reads the program's variable result afterwards, so the test sees
// the value the program stored.
func ilProgram(vars, body, result string) string {
	return "PROGRAM P\n VAR " + vars + " END_VAR\n" + body + "\nEND_PROGRAM\nP();\nP." + result + ";"
}

func TestIlArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{ilProgram("x : INT; y : INT := 4;", " LD 5\n ADD 3\n MUL y\n SUB 2\n ST x", "x"), 30},
		{ilProgram("x : INT;", " LD 17\n DIV 5\n ST x", "x"), 3},
		{ilProgram("x : INT;", " LD 17\n MOD 5\n ST x", "x"), 2},
	}
	runVmTests(t, tests)
}

func TestIlLogicAndComparison(t *testing.T) {
	const flags = "a : BOOL := TRUE; b : BOOL := FALSE; q : BOOL;"
	tests := []vmTestCase{
		{ilProgram(flags, " LD a\n AND b\n ST q", "q"), false},
		{ilProgram(flags, " LD b\n OR a\n ST q", "q"), true},
		{ilProgram(flags, " LD b\n OR a\n NOT\n ST q", "q"), false},
		{ilProgram(flags, " LDN a\n ST q", "q"), false},
		{ilProgram(flags, " LD a\n STN q", "q"), false},
		{ilProgram("q : BOOL;", " LD 5\n GT 3\n ST q", "q"), true},
		{ilProgram("q : BOOL;", " LD 5\n EQ 5\n ST q", "q"), true},
	}
	runVmTests(t, tests)
}

func TestIlSetAndJump(t *testing.T) {
	tests := []vmTestCase{
		// S sets the operand to TRUE when the current result is TRUE.
		{ilProgram("a : BOOL := TRUE; q : BOOL := FALSE;", " LD a\n S q", "q"), true},
		// An unconditional jump skips the instructions before its label.
		{ilProgram("x : INT := 0;", " LD 1\n ST x\n JMP done\n LD 2\n ST x\ndone: LD x\n ST x", "x"), 1},
	}
	runVmTests(t, tests)
}

func TestIlFunction(t *testing.T) {
	tests := []vmTestCase{
		{"FUNCTION Twice : INT VAR_INPUT n : INT; END_VAR\n LD n\n MUL 2\n ST Twice\nEND_FUNCTION\nTwice(21);", 42},
	}
	runVmTests(t, tests)
}

// In IL, ST, S, R, conditional jumps and calls read the current result
// without consuming it.
func TestIlCurrentResultIsKept(t *testing.T) {
	tests := []vmTestCase{
		// A taken JMPCN keeps the current result for the instruction at the label.
		{ilProgram("q : BOOL := TRUE; b : BOOL := FALSE;", " LD b\n JMPCN end_it\n LD TRUE\nend_it:\n ST q", "q"), false},
		{ilProgram("x : INT; b : BOOL := TRUE;", " LD b\n JMPC end_it\n LD 10\n ST x\nend_it:\n LD 1\n ST x", "x"), 1},
		// ST leaves the current result for the next instruction.
		{ilProgram("x : INT; y : INT;", " LD 3\n ST x\n ADD 1\n ST y\n MUL x", "y"), 4},
		{ilProgram("n : INT; q : BOOL;", " LD n\n ADD 1\n ST n\n GT 0\n S q\n ST n", "q"), true},
		// A loop runs with a balanced stack.
		{ilProgram("i : INT;", "again:\n LD i\n ADD 1\n ST i\n LT 1000\n JMPC again", "i"), 1000},
		// CAL leaves the current result as it was.
		{"FUNCTION_BLOCK Fb VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; x : INT; END_VAR LD 5 CAL f() ADD 1 ST x END_PROGRAM P(); P.x;", 6},
		{"FUNCTION_BLOCK Fb VAR_OUTPUT n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR LD FALSE CALC f() LD TRUE CALC f() CALCN f() END_PROGRAM P(); P.f.n;", 1},
		// CAL of a function makes its result the current result.
		{"FUNCTION Seven : INT Seven := 7; END_FUNCTION PROGRAM P VAR x : INT; END_VAR LD 1 CAL Seven() ADD 1 ST x END_PROGRAM P(); P.x;", 8},
		// RET returns from a function with its result.
		{"FUNCTION F : INT VAR_INPUT b : BOOL; END_VAR LD 1 ST F LD b RETC LD 2 ST F END_FUNCTION F(TRUE) * 10 + F(FALSE);", 12},
	}
	runVmTests(t, tests)
}

// TestIlCallFunctionBlock is the IL form of an FB call.
func TestIlCallFunctionBlock(t *testing.T) {
	tests := []vmTestCase{
		{`FUNCTION_BLOCK Acc VAR_INPUT n : INT; END_VAR VAR total : INT := 0; END_VAR total := total + n; END_FUNCTION_BLOCK
		  PROGRAM P VAR f : Acc; END_VAR
		   CAL f(n := 5)
		   CAL f(n := 2)
		  END_PROGRAM
		  P();
		  P.f.total;`, 7},
	}
	runVmTests(t, tests)
}
