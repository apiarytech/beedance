/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"testing"
)

// The session's tests, on each engine: the VM runs them as the package's
// own tests; these run the same on the others. What an engine has not got
// is left out (the VM's disassembly).

func TestEvaluatorEngine(t *testing.T) { runOn(t, EngineEvaluator) }

// runOn runs the session's tests on engine e.
func runOn(t *testing.T, e Engine) {
	defer func(prev Engine) { testEngine = prev }(testEngine)
	testEngine = e
	for _, tc := range []struct {
		name string
		f    func(*testing.T)
	}{
		{"BreakpointStopsBeforeTheStatement", TestBreakpointStopsBeforeTheStatement},
		{"BreakpointMovesToTheNextStatement", TestBreakpointMovesToTheNextStatement},
		{"ConditionalBreakpoint", TestConditionalBreakpoint},
		{"BreakpointInAFunctionBlock", TestBreakpointInAFunctionBlock},
		{"Stepping", TestStepping},
		{"StepScan", TestStepScan},
		{"PausingChangesNothing", TestPausingChangesNothing},
		{"Pause", TestPause},
		{"YieldEvery", TestYieldEvery},
		{"Force", TestForce},
		{"Set", TestSet},
		{"Exception", TestException},
		{"ChangeKeepsTheValues", TestChangeKeepsTheValues},
		{"ChangeResetsAMemberWhoseTypeChanged", TestChangeResetsAMemberWhoseTypeChanged},
		{"ChangeWhileRunning", TestChangeWhileRunning},
	} {
		t.Run(tc.name, tc.f)
	}
}
