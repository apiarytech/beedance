package evaluator

import (
	"beedance/object"
	"strings"
	"testing"
)

func TestLimitFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer tests
		{"LIMIT(10, 5, 20)", int64(10)},      // Input below min
		{"LIMIT(10, 15, 20)", int64(15)},     // Input within range
		{"LIMIT(10, 25, 20)", int64(20)},     // Input above max
		{"LIMIT(-20, -15, -10)", int64(-15)}, // Negative numbers
		{"LIMIT(10, 10, 20)", int64(10)},     // Input equals min
		{"LIMIT(10, 20, 20)", int64(20)},     // Input equals max

		// Real tests
		{"LIMIT(10.0, 5.0, 20.0)", 10.0},
		{"LIMIT(10.0, 15.5, 20.0)", 15.5},
		{"LIMIT(10.0, 25.0, 20.0)", 20.0},

		// Mixed type tests (promotion to REAL)
		{"LIMIT(10, 15.5, 20)", 15.5},
		{"LIMIT(10.0, 5, 20.0)", 10.0},
		{"LIMIT(10, 25, 20.0)", 20.0},

		// Error cases
		{"LIMIT(10, 5)", "wrong number of arguments for LIMIT. got=2, want=3"},
		{"LIMIT(10, 5, 20, 30)", "wrong number of arguments for LIMIT. got=4, want=3"},
		{"LIMIT(10, TRUE, 20)", "all arguments to `LIMIT` must be INTEGER or REAL, got BOOLEAN"},
		{`LIMIT(10, 5, "20")`, "all arguments to `LIMIT` must be INTEGER or REAL, got STRING"},
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
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Fatalf("object is not Error. got=%T (%+v)", evaluated, evaluated)
				}
				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestMinMaxFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// MIN function tests
		{"MIN(10, 20)", int64(10)},
		{"MIN(20, 10)", int64(10)},
		{"MIN(10, 20, 5, 30)", int64(5)},
		{"MIN(-10, -20)", int64(-20)},
		{"MIN(10.5, 10.6)", 10.5},
		{"MIN(10, 20.5)", 10.0}, // Type promotion to REAL
		{"MIN(1, 2.5, -3.0, 4)", -3.0},
		{"MIN(10)", int64(10)},
		{"MIN()", "wrong number of arguments for MIN. got=0, want>=1"},
		{"MIN(1, TRUE)", "all arguments to `MIN` must be INTEGER or REAL, got BOOLEAN"},

		// MAX function tests
		{"MAX(10, 20)", int64(20)},
		{"MAX(20, 10)", int64(20)},
		{"MAX(10, 20, 5, 30)", int64(30)},
		{"MAX(-10, -20)", int64(-10)},
		{"MAX(10.5, 10.6)", 10.6},
		{"MAX(10, 20.5)", 20.5}, // Type promotion to REAL
		{"MAX(1, 2.5, -3.0, 4)", 4.0},
		{"MAX(10)", int64(10)},
		{"MAX()", "wrong number of arguments for MAX. got=0, want>=1"},
		{"MAX(1, TRUE)", "all arguments to `MAX` must be INTEGER or REAL, got BOOLEAN"},
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
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Fatalf("object is not Error. got=%T (%+v)", evaluated, evaluated)
				}
				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestMuxFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer selection
		{"MUX(0, 100, 101, 102)", int64(100)},
		{"MUX(1, 100, 101, 102)", int64(101)},
		{"MUX(2, 100, 101, 102)", int64(102)},

		// Real selection
		{"MUX(1, 10.5, 20.5, 30.5)", 20.5},

		// String selection
		{`MUX(0, "a", "b", "c")`, "a"},
		{`MUX(2, "a", "b", "c")`, "c"},

		// Boolean selection
		{"MUX(1, FALSE, TRUE)", true},

		// Selection with expression as selector
		{"MUX(1+1, 10, 20, 30)", int64(30)},

		// Error cases
		{"MUX(3, 100, 101, 102)", "index 3 out of bounds for MUX with 3 inputs"},
		{"MUX(-1, 100, 101, 102)", "index -1 out of bounds for MUX"},
		{"MUX(0.5, 100, 101)", "argument 1 to `MUX` must be INTEGER, got REAL"},
		{`MUX(0, 100, "a")`, "all value arguments to `MUX` must be of the same type, got STRING but expected INTEGER"},
		{"MUX(0)", "wrong number of arguments for MUX. got=1, want>=2"},
		{"MUX()", "wrong number of arguments for MUX. got=0, want>=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				if err, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(err.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, err.Message)
					}
				} else {
					testStringObject(t, evaluated, expected)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestSelFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer selection
		{"SEL(FALSE, 10, 20)", int64(10)},
		{"SEL(TRUE, 10, 20)", int64(20)},

		// Real selection
		{"SEL(FALSE, 10.5, 20.5)", 10.5},
		{"SEL(TRUE, 10.5, 20.5)", 20.5},

		// Boolean selection
		{"SEL(FALSE, TRUE, FALSE)", true},
		{"SEL(TRUE, TRUE, FALSE)", false},

		// String selection
		{`SEL(FALSE, "hello", "world")`, "hello"},
		{`SEL(TRUE, "hello", "world")`, "world"},

		// Selection with expressions
		{"SEL(1 > 0, 5+5, 10+10)", int64(20)},
		{"SEL(1 < 0, 5+5, 10+10)", int64(10)},

		// Error cases
		{"SEL(1, 10, 20)", "argument 1 to `SEL` must be BOOLEAN, got INTEGER"},
		{`SEL(TRUE, 10, "world")`, "arguments 2 and 3 to `SEL` must be of the same type, got INTEGER and STRING"},
		{"SEL(TRUE, 10)", "wrong number of arguments for SEL. got=2, want=3"},
		{"SEL(TRUE, 10, 20, 30)", "wrong number of arguments for SEL. got=4, want=3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				if err, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(err.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, err.Message)
					}
				} else {
					testStringObject(t, evaluated, expected)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}
