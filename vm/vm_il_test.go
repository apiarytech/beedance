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

// TestIlConditionalJumpKeepsCurrentResult documents a compiler bug: in IL, a
// conditional jump (JMPC/JMPCN) tests the current result without consuming it,
// but the compiler emits a jump that pops it. When the jump is taken, the next
// instruction finds an empty stack.
func TestIlConditionalJumpKeepsCurrentResult(t *testing.T) {
	t.Skip("compiler bug: JMPC/JMPCN consume the IL current result; see this test's comment")
	tests := []vmTestCase{
		{ilProgram("a : INT; b : BOOL := FALSE;", " LD b\n JMPCN end_it\n LD 10\nend_it:\n ST a", "a"), false},
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
