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

func TestArrayBoundFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Success cases
		{"LOWER_BOUND([1, 2, 3], 1);", int64(0)},
		{"UPPER_BOUND([1, 2, 3], 1);", int64(2)},
		{"UPPER_BOUND(['a', 'b', 'c', 'd'], 1);", int64(3)},
		{"LOWER_BOUND([], 1);", int64(0)},
		{"UPPER_BOUND([], 1);", int64(-1)},

		// Error cases
		{"LOWER_BOUND();", "wrong number of arguments for LOWER_BOUND. got=0, want=2"},
		{"UPPER_BOUND([1]);", "wrong number of arguments for UPPER_BOUND. got=1, want=2"},
		{"LOWER_BOUND(1, 1);", "argument 1 to `LOWER_BOUND` must be of type ARRAY, got LINT"},
		{"UPPER_BOUND('hello', 1);", "argument 1 to `UPPER_BOUND` must be of type ARRAY, got STRING"},
		{"LOWER_BOUND([], 1.0);", "argument 2 to `LOWER_BOUND` must be of type INT, got REAL"},
		{"UPPER_BOUND([], TRUE);", "argument 2 to `UPPER_BOUND` must be of type INT, got BOOLEAN"},
		{"LOWER_BOUND([], 2);", "invalid dimension 2 for 1D array"},
		{"UPPER_BOUND([], 0);", "invalid dimension 0 for 1D array"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			}
		})
	}
}
