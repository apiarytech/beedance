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
			input: `
			PROGRAM TestInheritance
				VAR
					c_def : C;
					b_def : B;
					a_def : A;
					instance : A;
				END_VAR

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

				instance := A();
				instance.GetValue();
			`,
			expected: 60,
		},
	}
	runVmTests(t, tests)
}

// --- Helper functions copied from vm_test.go ---
