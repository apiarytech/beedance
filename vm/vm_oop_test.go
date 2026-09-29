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
	_ "beedance/stdlib"
	"testing"
)

func TestFunctionBlockThreeLevelInheritanceVM(t *testing.T) {
	tests := []vmTestCase{
		{
			// Function blocks are declared at top level; IEC 61131-3 does not
			// allow POUs to be nested inside a PROGRAM. Declaring `instance : A`
			// creates the instance.
			input: `
			FUNCTION_BLOCK C
				METHOD GetValue : INT
					GetValue := 10;
				END_METHOD
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK B EXTENDS C
				METHOD GetValue : INT
					GetValue := SUPER^.GetValue() + 20;
				END_METHOD
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK A EXTENDS B
				METHOD GetValue : INT
					GetValue := SUPER^.GetValue() + 30;
				END_METHOD
			END_FUNCTION_BLOCK

			PROGRAM TestInheritance
				VAR
					instance : A;
				END_VAR
				instance.GetValue();
			END_PROGRAM
			`,
			expected: 60,
		},
	}
	runVmTests(t, tests)
}

// --- Helper functions copied from vm_test.go ---

func TestFunctionBlockInstanceStateVM(t *testing.T) {
	tests := []vmTestCase{
		{
			// The PROGRAM is written before the FB it instantiates, and methods
			// read and update the instance's own variable across calls.
			input: `
			PROGRAM TestCounter
				VAR
					counter : Counter;
				END_VAR
				counter.Increment();
				counter.Increment();
				counter.Current();
			END_PROGRAM

			FUNCTION_BLOCK Counter
				VAR
					count : INT := 5;
				END_VAR
				METHOD Increment : INT
					count := count + 1;
					Increment := count;
				END_METHOD
				METHOD Current : INT
					Current := THIS.count;
				END_METHOD
			END_FUNCTION_BLOCK
			`,
			expected: 7,
		},
		{
			// Two instances of the same FB keep separate state.
			input: `
			FUNCTION_BLOCK Counter
				VAR
					count : INT := 0;
				END_VAR
				METHOD Add : INT
					VAR_INPUT
						n : INT;
					END_VAR
					count := count + n;
					Add := count;
				END_METHOD
			END_FUNCTION_BLOCK

			PROGRAM TestTwoCounters
				VAR
					a : Counter;
					b : Counter;
				END_VAR
				a.Add(10);
				b.Add(1);
				a.Add(5) * 100 + b.Add(2);
			END_PROGRAM
			`,
			expected: 1503,
		},
	}
	runVmTests(t, tests)
}
