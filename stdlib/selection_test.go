package stdlib

import "testing"

func TestBuiltinSelectionFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// LIMIT
		{"LIMIT(10, 5, 20);", int64(10)},  // Input below min
		{"LIMIT(10, 15, 20);", int64(15)}, // Input within range
		{"LIMIT(10, 25, 20);", int64(20)}, // Input above max
		{"LIMIT(10.0, 5.5, 20.0);", 10.0}, // Real numbers
		{"LIMIT(10, 15.5, 20);", 15.5},    // Mixed numbers
		{"LIMIT(10, 5, TRUE);", "BUILTIN ERROR: all arguments to `LIMIT` must be INTEGER or REAL, got BOOLEAN"},

		// MUX
		{"MUX(0, 100, 101, 102);", int64(100)},
		{"MUX(1, 100, 101, 102);", int64(101)},
		{"MUX(2, 'a', 'b', 'c');", "c"},
		{"MUX(3, 1, 2, 3);", "BUILTIN ERROR: index 3 out of bounds for MUX with 3 inputs"},
		{"MUX(-1, 1, 2, 3);", "BUILTIN ERROR: index -1 out of bounds for MUX"},
		{"MUX('a', 1, 2);", "BUILTIN ERROR: argument 1 to `MUX` must be INTEGER, got STRING"},

		// SEL
		{"SEL(FALSE, 10, 20);", int64(10)},
		{"SEL(TRUE, 10, 20);", int64(20)},
		{"SEL(FALSE, 'a', 'b');", "a"},
		{"SEL(TRUE, 'a', 'b');", "b"},
		{"SEL(1, 10, 20);", "BUILTIN ERROR: argument 1 to `SEL` must be BOOLEAN, got LINT"},
		{"SEL(TRUE, 10, 'a');", "BUILTIN ERROR: arguments 2 and 3 to `SEL` must be of the same type, got LINT and STRING"},

		// MOVE
		{"MOVE(123);", int64(123)},
		{`MOVE('hello');`, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case string:
				testStringOrError(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}
