package stdlib

import (
	"beedance/object"
	"testing"
	"time"
)

func TestBuiltinMinMaxAny(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// STRING tests
		{`MIN('apple', 'banana');`, "apple"},
		{`MAX('apple', 'banana');`, "banana"},
		{`MIN('z', 'a', 'm');`, "a"},
		{`MAX('z', 'a', 'm');`, "z"},

		// TIME tests
		{`MIN(T#5s, T#10s);`, 5 * time.Second},
		{`MAX(T#5s, T#10s);`, 10 * time.Second},
		{`MIN(T#1h, T#30m, T#90m);`, 30 * time.Minute},
		{`MAX(T#1h, T#30m, T#90m);`, 90 * time.Minute},

		// Error cases for mixed types
		{`MIN('apple', 1);`, "all arguments to `MIN` must be of the same type, got STRING and LINT"},
		{`MAX(T#5s, 'hello');`, "all arguments to `MAX` must be of the same type, got TIME and STRING"},
		{`MIN(1, T#1s);`, "BUILTIN ERROR: all arguments to `MIN` must be INTEGER or REAL, got TIME"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case string:
				testStringOrError(t, result, expected)
			case time.Duration:
				testTimeObject(t, result, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinMathFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// --- Trig Functions ---
		{"SIN(0);", 0.0},
		{"COS(0);", 1.0},
		{"TAN(0);", 0.0},
		{"ASIN(0);", 0.0},
		{"ACOS(1);", 0.0},
		{"ATAN(0);", 0.0},
		{"ATAN2(0, 1);", 0.0},
		{"SIN(1.570796);", 1.0},
		{"COS(3.141592);", -1.0},

		// --- Log/Exp Functions ---
		{"LN(1);", 0.0},
		{"LOG(10);", 1.0},
		{"EXP(0);", 1.0},
		{"EXP(1);", 2.718281828459045},

		// --- Numerical Functions ---
		{"SQRT(9);", 3.0},
		{"ABS(-10);", int64(10)},
		{"ABS(10);", int64(10)},
		{"ABS(-10.5);", 10.5},
		{"ROUND(3.5);", int64(4)},
		{"ROUND(3.4);", int64(3)},
		{"TRUNC(3.9);", int64(3)},
		{"TRUNC(-3.9);", int64(-3)},

		// --- Error Cases ---
		{"SQRT(-1);", "BUILTIN ERROR: argument to `SQRT` must be non-negative"},
		{"LN(0);", "BUILTIN ERROR: argument to `LN` must be positive"},
		{"LOG(-1);", "BUILTIN ERROR: argument to `LOG` must be positive"},
		{"ASIN(2);", "BUILTIN ERROR: argument to `ASIN` must be between -1 and 1"},
		{"ACOS(-2);", "BUILTIN ERROR: argument to `ACOS` must be between -1 and 1"},
		{"SIN(TRUE);", "BUILTIN ERROR: argument to `SIN` must be INTEGER or REAL"},
		{"ABS('a');", "BUILTIN ERROR: argument to `ABS` not supported"},
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

func TestBuiltinTimeDateFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// --- ADD ---
		{"ADD(T#5s, T#10s);", 15 * time.Second},
		{"ADD(TOD#10:00:00, T#1h30m);", time.Date(0, 1, 1, 11, 30, 0, 0, time.UTC)},
		{"ADD(DT#2026-05-21-10:00:00, T#2h);", time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)},
		{"ADD(T#1s, 10);", "BUILTIN ERROR: unsupported argument types for ADD: TIME + LINT"},

		// --- SUB ---
		{"SUB(T#1m, T#15s);", 45 * time.Second},
		{"SUB(D#2026-05-21, D#2026-05-20);", 24 * time.Hour},
		{"SUB(TOD#10:00:00, T#1h30m);", time.Date(0, 1, 1, 8, 30, 0, 0, time.UTC)},
		{"SUB(TOD#10:00:00, TOD#08:00:00);", 2 * time.Hour},
		{"SUB(DT#2026-05-21-10:00:00, T#30m);", time.Date(2026, 5, 21, 9, 30, 0, 0, time.UTC)},
		{"SUB(DT#2026-05-21-10:00:00, DT#2026-05-20-10:00:00);", 24 * time.Hour},
		{"SUB(D#2026-05-21, T#1s);", "BUILTIN ERROR: unsupported argument types for SUB: DATE - TIME"},

		// --- MUL ---
		{"MUL(T#10s, 5);", 50 * time.Second},
		{"MUL(5, T#10s);", 50 * time.Second},
		{"MUL(T#1m, 1.5);", 90 * time.Second},
		{"MUL(T#1s, T#2s);", "BUILTIN ERROR: unsupported argument types for MUL: TIME * TIME"},

		// --- DIV ---
		{"DIV(T#1m, 4);", 15 * time.Second},
		{"DIV(T#1m, 2.5);", 24 * time.Second},
		{"DIV(T#1m, T#2s);", "BUILTIN ERROR: unsupported argument types for DIV: TIME / TIME"},
		{"DIV(T#1m, 0);", "BUILTIN ERROR: division by zero"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case time.Duration:
				testTimeObject(t, evaluated, expected)
			case time.Time:
				switch result := evaluated.(type) {
				case *object.Date:
					if !result.Value.Equal(expected) {
						t.Errorf("wrong date value. want=%v, got=%v", expected, result.Value)
					}
				case *object.TimeOfDay:
					if result.Value.Format("15:04:05") != expected.Format("15:04:05") {
						t.Errorf("wrong tod value. want=%v, got=%v", expected.Format("15:04:05"), result.Value.Format("15:04:05"))
					}
				case *object.DateAndTime:
					if !result.Value.Equal(expected) {
						t.Errorf("wrong dt value. want=%v, got=%v", expected, result.Value)
					}
				default:
					t.Fatalf("unhandled time.Time result type: %T", result)
				}
			case string:
				testStringOrError(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}
