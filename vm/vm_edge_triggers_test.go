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

// Edge detection needs two things the VM does not support yet: calling a
// function block instance (`trig(CLK := x)`) and the standard function blocks
// (R_TRIG, F_TRIG). These tests describe the expected behavior and are skipped
// until that support exists. Each program calls the block several times in one
// run, standing in for successive PLC scans.

const edgeTriggerSkipReason = "not supported yet: calling function block instances and the standard R_TRIG/F_TRIG blocks"

func TestRisingEdgeTrigger(t *testing.T) {
	t.Skip(edgeTriggerSkipReason)
	const program = `
		VAR trig : R_TRIG; q1 : BOOL; q2 : BOOL; q3 : BOOL; q4 : BOOL; END_VAR
		trig(CLK := FALSE); q1 := trig.Q;
		trig(CLK := TRUE);  q2 := trig.Q;
		trig(CLK := TRUE);  q3 := trig.Q;
		trig(CLK := FALSE); q4 := trig.Q;
	`
	runVmTests(t, []vmTestCase{
		{program + "q1;", false}, // no edge yet
		{program + "q2;", true},  // FALSE -> TRUE
		{program + "q3;", false}, // still TRUE: no new edge
		{program + "q4;", false}, // TRUE -> FALSE is not a rising edge
	})
}

func TestFallingEdgeTrigger(t *testing.T) {
	t.Skip(edgeTriggerSkipReason)
	const program = `
		VAR trig : F_TRIG; q1 : BOOL; q2 : BOOL; q3 : BOOL; END_VAR
		trig(CLK := TRUE);  q1 := trig.Q;
		trig(CLK := FALSE); q2 := trig.Q;
		trig(CLK := FALSE); q3 := trig.Q;
	`
	runVmTests(t, []vmTestCase{
		{program + "q1;", false},
		{program + "q2;", true}, // TRUE -> FALSE
		{program + "q3;", false},
	})
}

// TestEdgeInputDeclaration covers `R_EDGE` inputs, which make a function block
// see its input as TRUE only on the call where it rises.
func TestEdgeInputDeclaration(t *testing.T) {
	t.Skip(edgeTriggerSkipReason)
	runVmTests(t, []vmTestCase{
		{`FUNCTION_BLOCK Counter
			VAR_INPUT pulse : BOOL R_EDGE; END_VAR
			VAR_OUTPUT hits : INT := 0; END_VAR
			IF pulse THEN hits := hits + 1; END_IF
		  END_FUNCTION_BLOCK
		  VAR c : Counter; END_VAR
		  c(pulse := TRUE); c(pulse := TRUE); c(pulse := FALSE); c(pulse := TRUE);
		  c.hits;`, 2},
	})
}
