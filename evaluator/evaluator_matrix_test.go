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
	"testing"
)

func TestEvalCommaSeparatedIndexExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			`
			VAR
				Matrix : ARRAY [0..1, 0..2] OF INT := [[1, 2, 3], [4, 5, 6]];
			END_VAR
			Matrix[1, 2];
			`,
			6,
		},
		{
			`
			VAR
				Matrix : ARRAY [0..1, 0..2] OF INT := [[1, 2, 3], [4, 5, 6]];
			END_VAR
			Matrix[0, 1] := 99;
			Matrix[0, 1];
			`,
			99,
		},
		{
			`
			VAR
				Matrix : ARRAY [0..1, 0..2] OF INT := [[1, 2, 3], [4, 5, 6]];
				r : INT := 1;
				c : INT := 0;
			END_VAR
			Matrix[r, c];
			`,
			4,
		},
		{
			`
			VAR
				Tensor : ARRAY [0..1, 0..1, 0..1] OF INT := [[[1, 2], [3, 4]], [[5, 6], [7, 8]]];
			END_VAR
			Tensor[1, 1, 0];
			`,
			7,
		},
		{
			`
			VAR
				Matrix : ARRAY [0..1, 0..2] OF INT;
			END_VAR
			Matrix[0, 4];
			`,
			nil, // Out of bounds access should result in NULL
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, tt.input, int64(expected))
		case nil:
			testNullObject(t, evaluated)
		}
	}
}
