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

// EN/ENO lets a caller disable a function or function block call: with
// EN := FALSE the body does not run and ENO is FALSE. The compiler does not
// generate EN/ENO handling yet (EN is passed as an ordinary argument, so the
// call fails with "wrong number of arguments"). These tests describe the
// expected behavior and are skipped until that support exists.

const enEnoSkipReason = "not supported yet: EN/ENO on function and function block calls"

const enEnoFunction = `
	FUNCTION Inc : INT
		VAR_INPUT n : INT; END_VAR
		Inc := n + 1;
	END_FUNCTION
	VAR ok : BOOL; r : INT := 0; END_VAR
`

func TestFunctionCallWithEnableTrue(t *testing.T) {
	t.Skip(enEnoSkipReason)
	runVmTests(t, []vmTestCase{
		{enEnoFunction + "r := Inc(EN := TRUE, n := 5, ENO => ok); r;", 6},
		{enEnoFunction + "r := Inc(EN := TRUE, n := 5, ENO => ok); ok;", true},
	})
}

func TestFunctionCallWithEnableFalse(t *testing.T) {
	t.Skip(enEnoSkipReason)
	runVmTests(t, []vmTestCase{
		// The body does not run, so the result keeps its default value.
		{enEnoFunction + "r := Inc(EN := FALSE, n := 5, ENO => ok); r;", 0},
		{enEnoFunction + "r := Inc(EN := FALSE, n := 5, ENO => ok); ok;", false},
	})
}

func TestFunctionBlockCallWithEnableFalse(t *testing.T) {
	t.Skip(enEnoSkipReason)
	runVmTests(t, []vmTestCase{
		{`FUNCTION_BLOCK Acc
			VAR_INPUT n : INT; END_VAR
			VAR total : INT := 0; END_VAR
			total := total + n;
		  END_FUNCTION_BLOCK
		  VAR a : Acc; END_VAR
		  a(EN := TRUE, n := 5);
		  a(EN := FALSE, n := 100);
		  a.total;`, 5},
	})
}
