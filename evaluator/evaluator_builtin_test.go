package evaluator

import (
	"github.com/apiarytech/beedance/object"
	"math"
	"strings"
	"testing"
	"time"
)

func TestBuiltinFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// LEN
		{`LEN('');`, int64(0)},
		{`LEN('four');`, int64(4)},
		{`LEN('hello world');`, int64(11)},
		{`LEN(1);`, "BUILTIN ERROR: argument to `LEN` not supported, got LINT"},
		{`LEN('one', 'two');`, "BUILTIN ERROR: wrong number of arguments for LEN. got=2, want=1"},
		{`LEN([1, 2, 3]);`, int64(3)},
		{`LEN([]);`, int64(0)},

		// FIRST
		{`FIRST([1, 2, 3]);`, int64(1)},
		{`FIRST([]);`, nil},
		{`FIRST(1);`, "BUILTIN ERROR: argument to `FIRST` must be ARRAY, got LINT"},

		// LAST
		{`LAST([1, 2, 3]);`, int64(3)},
		{`LAST([]);`, nil},
		{`LAST(1);`, "BUILTIN ERROR: argument to `LAST` must be ARRAY, got LINT"},

		// REST
		{`REST([1, 2, 3]);`, []int{2, 3}},
		{`REST([]);`, nil},

		// PUSH
		{`PUSH([], 1);`, []int{1}},
		{`PUSH(1, 1);`, "BUILTIN ERROR: argument to `PUSH` must be ARRAY, got LINT"},

		// --- IEC 61131-3 Standard Built-ins ---
		// String Functions
		{`CONCAT('a', 'b');`, "ab"},
		{`CONCAT('a', 'b', 'c');`, "abc"},
		{`CONCAT('a');`, "BUILTIN ERROR: wrong number of arguments for CONCAT. got=1, want>=2"},
		{`LEFT('abcde', 2);`, "ab"},
		{`RIGHT('abcde', 2);`, "de"},
		{`MID('abcde', 3, 2);`, "bcd"},
		{`FIND('abcabc', 'b');`, int64(2)},

		// Selection Functions
		{`LIMIT(10, 5, 20);`, int64(10)},
		{`LIMIT(10, 15, 20);`, int64(15)},
		{`LIMIT(10, 25, 20);`, int64(20)},
		{`LIMIT(10.0, 5.5, 20.0);`, 10.0},
		{`MUX(0, 100, 101, 102);`, int64(100)},
		{`MUX(2, 'a', 'b', 'c');`, "c"},
		{`SEL(FALSE, 10, 20);`, int64(10)},
		{`SEL(TRUE, 'a', 'b');`, "b"},
		{`MOVE(123);`, int64(123)},
		{`MOVE('hello');`, "hello"},

		// Math Functions
		{`SQRT(9);`, 3.0},
		{`ABS(-10);`, int64(10)},
		{`ABS(-10.5);`, 10.5},
		{`ROUND(3.5);`, int64(4)},
		{`TRUNC(-3.9);`, int64(-3)},
		{"SIN(0);", 0.0},
		{"COS(0);", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int:
				testIntegerObject(t, evaluated, tt.input, int64(expected))
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case bool:
				testBooleanObject(t, evaluated, tt.input, expected)
			case nil:
				testNullObject(t, evaluated)
			case string:
				// Can be a string result or an error message
				if errObj, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(errObj.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
					}
				} else {
					testStringObject(t, evaluated, tt.input, expected)
				}
			case []int:
				array, ok := evaluated.(*object.Array)
				if !ok {
					t.Errorf("object not Array: %T (%+v)", evaluated, evaluated)
					return
				}
				if len(array.Elements) != len(expected) {
					t.Errorf("wrong num of elements. want=%d, got=%d", len(expected), len(array.Elements))
					return
				}
				for i, expectedElem := range expected {
					testIntegerObject(t, array.Elements[i], "elem", int64(expectedElem))
				}
			}
		})
	}
}

func TestBuiltinAddSub(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be int64, float64, time.Duration, time.Time, or string for error
	}{
		// ADD operations
		{"ADD(T#1s, T#2s);", 3 * time.Second},
		{"ADD(TIME#1m, TIME#30s);", (1 * time.Minute) + (30 * time.Second)},
		{"ADD(TOD#10:00:00, T#1h);", time.Date(0, 1, 1, 11, 0, 0, 0, time.UTC)},               // TOD + TIME
		{"ADD(TOD#23:00:00, T#2h);", time.Date(0, 1, 2, 1, 0, 0, 0, time.UTC)},                // TOD + TIME with wrap-around
		{"ADD(DT#2026-04-30-10:00:00, T#1h);", time.Date(2026, 4, 30, 11, 0, 0, 0, time.UTC)}, // DT + TIME
		{"ADD(10, 20);", int64(30)},                                                           // cspell:disable-line
		{"ADD(1.5, 2.5);", 4.0},                                                               // cspell:disable-line
		{"ADD(10, 2.5);", 12.5},                                                               // cspell:disable-line
		{"ADD(1.5, 20);", 21.5},                                                               // cspell:disable-line

		// SUB operations
		{"SUB(T#5s, T#2s);", 3 * time.Second},
		{"SUB(TIME#2m, TIME#30s);", (1 * time.Minute) + (30 * time.Second)},
		{"SUB(D#2026-04-30, D#2026-04-29);", 24 * time.Hour},                                 // DATE - DATE -> TIME
		{"SUB(TOD#10:00:00, T#1h);", time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC)},               // TOD - TIME
		{"SUB(TOD#01:00:00, T#2h);", time.Date(0, 1, 0, 23, 0, 0, 0, time.UTC)},              // TOD - TIME with wrap-around // cspell:disable-line
		{"SUB(TOD#10:00:00, TOD#09:00:00);", 1 * time.Hour},                                  // TOD - TOD -> TIME
		{"SUB(DT#2026-04-30-10:00:00, T#1h);", time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)}, // DT - TIME
		{"SUB(DT#2026-04-30-10:00:00, DT#2026-04-30-09:00:00);", 1 * time.Hour},              // DT - DT -> TIME
		{"SUB(20, 10);", int64(10)},                                                          // cspell:disable-line
		{"SUB(4.0, 1.5);", 2.5},                                                              // cspell:disable-line
		{"SUB(10, 2.5);", 7.5},                                                               // cspell:disable-line
		{"SUB(4.0, 2);", 2.0},                                                                // cspell:disable-line

		// Error cases
		{"ADD(T#1s, D#2026-04-30);", "BUILTIN ERROR: unsupported argument types for ADD: TIME + DATE"},
		{"SUB(T#1s, D#2026-04-30);", "BUILTIN ERROR: unsupported argument types for SUB: TIME - DATE"},
		{"ADD(10, TRUE);", "BUILTIN ERROR: unsupported argument types for ADD: LINT + BOOLEAN"},
		{"SUB(10, TRUE);", "BUILTIN ERROR: unsupported argument types for SUB: LINT - BOOLEAN"},
		{"ADD(T#1s);", "BUILTIN ERROR: wrong number of arguments for ADD. got=1, want=2"},
		{"SUB(T#1s);", "BUILTIN ERROR: wrong number of arguments for SUB. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case time.Duration:
				timeObj, ok := evaluated.(*object.Time)
				if !ok {
					t.Errorf("input: %q, object is not Time. got=%T (%+v)", tt.input, evaluated, evaluated)
					return
				}
				if timeObj.Value != expected {
					t.Errorf("input: %q, wrong time duration value. want=%v, got=%v", tt.input, expected, timeObj.Value)
				}
			case time.Time:
				switch evaluated := evaluated.(type) {
				case *object.Date:
					if !evaluated.Value.Equal(expected) {
						t.Errorf("input: %q, wrong date value. want=%v, got=%v", tt.input, expected, evaluated.Value)
					}
				case *object.TimeOfDay:
					// For TOD, we only compare the time part, not the date part.
					// The expected time.Time will have a default date (Jan 1, year 0000 or Jan 2, year 0000 for wrap-around).
					// We need to compare only the time components.
					if evaluated.Value.Hour() != expected.Hour() ||
						evaluated.Value.Minute() != expected.Minute() ||
						evaluated.Value.Second() != expected.Second() ||
						evaluated.Value.Nanosecond() != expected.Nanosecond() {
						t.Errorf("input: %q, wrong time of day value. want=%v, got=%v", tt.input, expected.Format("15:04:05.999999999"), evaluated.Value.Format("15:04:05.999999999"))
					}
				case *object.DateAndTime:
					if !evaluated.Value.Equal(expected) {
						t.Errorf("input: %q, wrong date and time value. want=%v, got=%v", tt.input, expected, evaluated.Value)
					}
				default:
					t.Errorf("input: %q, object is not a known time.Time-based type. got=%T (%+v)", tt.input, evaluated, evaluated)
				}
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case string: // For error messages
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("input: %q, object is not Error. got=%T (%+v)", tt.input, evaluated, evaluated)
					return
				}
				if errObj.Message != expected {
					t.Errorf("input: %q, wrong error message. expected=%q, got=%q", tt.input, expected, errObj.Message)
				}
			default:
				t.Errorf("input: %q, unhandled expected type: %T", tt.input, tt.expected)
			}
		})
	}
}

func TestBuiltinMulDiv(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be int64, float64, time.Duration, or string for error
	}{
		// MUL operations
		{"MUL(10, 20);", int64(200)},
		{"MUL(1.5, 2.0);", 3.0},
		{"MUL(10, 2.5);", 25.0}, // INT * REAL promotion
		{"MUL(1.5, 10);", 15.0}, // REAL * INT promotion
		{"MUL(T#10s, 2);", 20 * time.Second},
		{"MUL(2, T#10s);", 20 * time.Second},
		{"MUL(T#1m, 1.5);", 90 * time.Second},

		// DIV operations
		{"DIV(20, 10);", int64(2)},
		{"DIV(5.0, 2.0);", 2.5},
		{"DIV(10, 2.5);", 4.0},
		{"DIV(5.0, 2);", 2.5},
		{"DIV(T#20s, 2);", 10 * time.Second},
		{"DIV(T#1m, 2.5);", 24 * time.Second},

		// Error cases
		{"MUL(10, TRUE);", "BUILTIN ERROR: unsupported argument types for MUL: LINT * BOOLEAN"},
		{"DIV(10, TRUE);", "BUILTIN ERROR: unsupported argument types for DIV: LINT / BOOLEAN"},
		{"MUL(T#1s, T#2s);", "BUILTIN ERROR: unsupported argument types for MUL: TIME * TIME"},
		{"DIV(10, 0);", "BUILTIN ERROR: division by zero"},
		{"DIV(10.0, 0);", "division by zero"},
		{"DIV(T#10s, 0);", "division by zero"},
		{"MUL(T#1s);", "BUILTIN ERROR: wrong number of arguments for MUL. got=1, want=2"},
		{"DIV(T#1s);", "BUILTIN ERROR: wrong number of arguments for DIV. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case int:
				testErrorObjectContains(t, evaluated, "SINT overflow")
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case time.Duration:
				timeObj, ok := evaluated.(*object.Time)
				if !ok {
					t.Errorf("object is not Time. got=%T (%+v)", evaluated, evaluated)
					return
				}
				if timeObj.Value != expected {
					t.Errorf("wrong time duration value. want=%v, got=%v", expected, timeObj.Value)
				}
			case string: // Error messages
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("object is not Error. got=%T (%+v)", evaluated, evaluated)
					return
				}
				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			}
		})
	}
}

func TestBuiltinModExpt(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be int64, float64, or string for error
	}{
		// MOD operations
		{"MOD(10, 3);", int64(1)},
		{"MOD(10, 2);", int64(0)},
		{"MOD(-10, 3);", int64(-1)},
		{"MOD(10, -3);", int64(1)},
		{"MOD(10, 0);", "BUILTIN ERROR: division by zero in MOD"},
		{"MOD(10.5, 2);", "BUILTIN ERROR: arguments to `MOD` must be INTEGER, got LREAL and LINT"},
		{"MOD(10, 2.5);", "BUILTIN ERROR: arguments to `MOD` must be INTEGER, got LINT and LREAL"},
		{"MOD(10);", "BUILTIN ERROR: wrong number of arguments for MOD. got=1, want=2"},

		// EXPT operations
		{"EXPT(2, 3);", 8.0},
		{"EXPT(2.0, 3.0);", 8.0},
		{"EXPT(4, 0.5);", 2.0},
		{"EXPT(10, -1);", 0.1},
		{"EXPT(-2, 3);", -8.0},
		{"EXPT(9, 0.5);", 3.0},
		{"EXPT(2, 3.5);", math.Pow(2, 3.5)},
		{"EXPT(TRUE, 2);", "BUILTIN ERROR: argument 1 to `EXPT` must be numeric, got BOOLEAN"},
		{"EXPT(2, TRUE);", "BUILTIN ERROR: argument 2 to `EXPT` must be numeric, got BOOLEAN"},
		{"EXPT(2);", "BUILTIN ERROR: wrong number of arguments for EXPT. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case string: // Error messages
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("object is not Error. got=%T (%+v)", evaluated, evaluated)
					return
				}
				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			}
		})
	}
}

func TestBuiltinComparisonFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// GT (Greater Than)
		{"GT(20, 10);", true},
		{"GT(10, 20);", false},
		{"GT(10, 10);", false},
		{"GT(20.5, 10);", true},
		{"GT('b', 'a');", true},

		// GE (Greater Than or Equal)
		{"GE(20, 10);", true},
		{"GE(10, 20);", false},
		{"GE(10, 10);", true},
		{"GE(10.0, 10);", true},
		{"GE('b', 'a');", true},
		{"GE('a', 'a');", true},

		// EQ (Equal)
		{"EQ(10, 10);", true},
		{"EQ(10, 20);", false},
		{"EQ(10.0, 10);", true},
		{"EQ('a', 'a');", true},
		{"EQ('a', 'b');", false},
		{"EQ(TRUE, TRUE);", true},
		{"EQ(FALSE, FALSE);", true},
		{"EQ(TRUE, FALSE);", false},
		{"EQ(T#1s, T#1s);", true},
		{"EQ(T#1s, T#2s);", false},
		{"EQ(D#2026-01-01, D#2026-01-01);", true},
		{"EQ(D#2026-01-01, D#2026-01-02);", false},
		{"EQ(TOD#10:00:00, TOD#10:00:00);", true},
		{"EQ(TOD#10:00:00, TOD#10:00:01);", false},
		{"EQ(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:00);", true},
		{"EQ(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", false},

		// LE (Less Than or Equal)
		{"LE(10, 20);", true},
		{"LE(20, 10);", false},
		{"LE(10, 10);", true},
		{"LE(10, 10.0);", true},
		{"LE(10.0, 10.0);", true},
		{"LE('a', 'b');", true},
		{"LE('a', 'a');", true},
		{"LE(T#1s, T#1s);", true},
		{"LE(T#1s, T#2s);", true},
		{"LE(D#2026-01-01, D#2026-01-01);", true},
		{"LE(D#2026-01-01, D#2026-01-02);", true},
		{"LE(TOD#10:00:00, TOD#10:00:00);", true},
		{"LE(TOD#10:00:00, TOD#10:00:01);", true},
		{"LE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:00);", true},
		{"LE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", true},

		// LT (Less Than)
		{"LT(10, 20);", true},
		{"LT(20, 10);", false},
		{"LT(10, 10);", false},
		{"LT(10, 20.5);", true},
		{"LT(10.0, 20.5);", true},
		{"LT('a', 'b');", true},
		{"LT(T#1s, T#2s);", true},
		{"LT(T#2s, T#1s);", false},
		{"LT(D#2026-01-01, D#2026-01-02);", true},
		{"LT(D#2026-01-02, D#2026-01-01);", false},
		{"LT(TOD#10:00:00, TOD#10:00:01);", true},
		{"LT(TOD#10:00:01, TOD#10:00:00);", false},
		{"LT(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", true},
		{"LT(DT#2026-01-01-10:00:01, DT#2026-01-01-10:00:00);", false},

		// NE (Not Equal)
		{"NE(10, 20);", true},
		{"NE(10, 10);", false},
		{"NE(10.0, 10);", false},
		{"NE('a', 'b');", true},
		{"NE('a', 'a');", false},
		{"NE(TRUE, FALSE);", true},
		{"NE(TRUE, TRUE);", false},
		{"NE(T#1s, T#2s);", true},
		{"NE(T#1s, T#1s);", false},
		{"NE(D#2026-01-01, D#2026-01-01);", false},
		{"NE(D#2026-01-01, D#2026-01-02);", true},
		{"NE(TOD#10:00:00, TOD#10:00:00);", false},
		{"NE(TOD#10:00:00, TOD#10:00:01);", true},
		{"NE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:00);", false},
		{"NE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", true},

		// Error cases
		{"GT(10, 'a');", "type mismatch for comparison: LINT > STRING"},
		{"LT(TRUE, 1);", "type mismatch for comparison: BOOLEAN < LINT"},
		{"GT(T#1s, 1);", "type mismatch for comparison: TIME > LINT"},
		{"GT(10);", "BUILTIN ERROR: wrong number of arguments for GT. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case bool:
				testBooleanObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinSQRTAndROUND(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // int64, float64, or string for error
	}{
		// SQRT
		{"SQRT(9);", 3.0},
		{"SQRT(2.25);", 1.5},
		{"SQRT(0);", 0.0},
		{"SQRT(-1);", "BUILTIN ERROR: argument to `SQRT` must be non-negative, got -1.000000"},
		{"SQRT(-2.25);", "BUILTIN ERROR: argument to `SQRT` must be non-negative, got -2.250000"},
		{"SQRT(TRUE);", "BUILTIN ERROR: argument to `SQRT` not supported, got BOOLEAN"},
		{"SQRT();", "BUILTIN ERROR: wrong number of arguments for SQRT. got=0, want=1"},
		{"SQRT(1, 2);", "BUILTIN ERROR: wrong number of arguments for SQRT. got=2, want=1"},

		// ROUND
		{"ROUND(2.4);", int64(2)},
		{"ROUND(2.5);", int64(3)}, // round half to even
		{"ROUND(2.6);", int64(3)},
		{"ROUND(3.5);", int64(4)}, // round half to even
		{"ROUND(-2.4);", int64(-2)},
		{"ROUND(-2.5);", int64(-3)}, // round half to even
		{"ROUND(-2.6);", int64(-3)},
		{"ROUND(0.0);", int64(0)},
		{"ROUND(5);", "BUILTIN ERROR: argument to `ROUND` must be REAL, got LINT"},
		{"ROUND(TRUE);", "BUILTIN ERROR: argument to `ROUND` must be REAL, got BOOLEAN"},
		{"ROUND();", "BUILTIN ERROR: wrong number of arguments for ROUND. got=0, want=1"},
		{"ROUND(1, 2);", "BUILTIN ERROR: wrong number of arguments for ROUND. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEvalWithPi(t, tt.input) // Use a helper that defines PI
			switch expected := tt.expected.(type) {
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			}
		})
	}
}

func TestConvertToIntegerFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // int64, float64, or string for error
	}{
		// REAL to Integer (rounding)
		{"REAL_TO_INT(5.4);", int64(5)},
		{"REAL_TO_INT(5.5);", int64(6)}, // round half to even
		{"REAL_TO_INT(5.6);", int64(6)},
		{"REAL_TO_INT(6.5);", int64(7)},
		{"LREAL_TO_DINT(-3.5);", int64(-4)},

		// STRING to Integer
		{"STRING_TO_INT('42');", int64(42)},
		{"STRING_TO_INT('-123');", int64(-123)},
		{"STRING_TO_INT('abc');", "BUILTIN ERROR: could not parse string to integer: abc"},

		// Integer to Integer (widening/narrowing)
		{"INT_TO_DINT(123);", int64(123)},
		{"DINT_TO_INT(32767);", int64(32767)},
		{"DINT_TO_INT(32768);", "BUILTIN ERROR: value 32768 is out of range for type INT (-32768 to 32767)"},
		// SINT (-128 to 127)
		{"INT_TO_SINT(127);", int64(127)},
		{"INT_TO_SINT(128);", "BUILTIN ERROR: value 128 is out of range for type SINT (-128 to 127)"},
		{"INT_TO_SINT(-128);", int64(-128)},
		{"INT_TO_SINT(-129);", "BUILTIN ERROR: value -129 is out of range for type SINT (-128 to 127)"},
		// USINT (0 to 255)
		{"INT_TO_USINT(255);", int64(255)},
		{"INT_TO_USINT(256);", "BUILTIN ERROR: value 256 is out of range for type USINT (0 to 255)"},
		{"INT_TO_USINT(-1);", "BUILTIN ERROR: value -1 is out of range for type USINT (0 to 255)"},
		// UINT (0 to 65535)
		{"DINT_TO_UINT(65535);", int64(65535)},
		{"DINT_TO_UINT(65536);", "BUILTIN ERROR: value 65536 is out of range for type UINT (0 to 65535)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestConvertToRealFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"INT_TO_REAL(10);", 10.0},
		{"DINT_TO_LREAL(-500);", -500.0},
		{"STRING_TO_REAL('123.45');", 123.45},
		{"STRING_TO_REAL('-1.23e-4');", -0.000123},
		{"STRING_TO_REAL('xyz');", "BUILTIN ERROR: could not parse string to real: xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestConvertToStringFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"INT_TO_STRING(123);", "123"},
		{"REAL_TO_STRING(1.23);", "1.230000"},
		{"BOOL_TO_STRING(TRUE);", "TRUE"},
		{"TIME_TO_STRING(T#1m30s);", "1m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testStringObject(t, evaluated, tt.input, tt.expected)
		})
	}
}

func TestConvertToBitStringFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"INT_TO_BYTE(255);", uint64(255)},
		{"INT_TO_BYTE(256);", "BUILTIN ERROR: value 256 is out of range for type BYTE (0 to 255)"},
		{"INT_TO_BYTE(-1);", "BUILTIN ERROR: value -1 is out of range for type BYTE (0 to 255)"},
		{"INT_TO_WORD(65535);", uint64(65535)},
		{"INT_TO_WORD(65536);", "BUILTIN ERROR: value 65536 is out of range for type WORD (0 to 65535)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case uint64:
				testBitStringObject(t, evaluated, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBCDConversionFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"INT_TO_BCD(1234);", uint64(0x1234)},
		{"INT_TO_BCD(9999);", uint64(0x9999)},
		{"INT_TO_BCD(10000);", "BUILTIN ERROR: value 10000 out of range for 4-digit BCD conversion (0-9999)"},
		{"BCD_TO_INT(WORD#16#1234);", int64(1234)},
		{"BCD_TO_INT(WORD#16#9999);", int64(9999)},
		{"BCD_TO_INT(WORD#16#1A2B);", "BUILTIN ERROR: invalid BCD format: nibble 2 has value 10 > 9"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case uint64:
				testBitStringObject(t, evaluated, expected)
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestTypeConversionErrors(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{"TIME_TO_DATE(T#1s);", "BUILTIN ERROR: conversion from TIME to DATE is not supported"},
		{"INT_TO_REAL();", "BUILTIN ERROR: wrong number of arguments for INT_TO_REAL. got=0, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testErrorObjectContains(t, evaluated, tt.expectedMessage)
		})
	}
}

func TestConvertToBooleanFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"INT_TO_BOOL(1);", true},
		{"INT_TO_BOOL(0);", false},
		{"LINT_TO_BOOL(-1);", true},
		{"REAL_TO_BOOL(0.0);", false},
		{"REAL_TO_BOOL(0.1);", true},
		{"REAL_TO_BOOL(-0.1);", true},
		{"REAL_TO_BOOL(1.0);", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testBooleanObject(t, evaluated, tt.input, tt.expected)
		})
	}
}

func TestBuiltinAbsFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer tests
		{"ABS(10);", int64(10)},
		{"ABS(-10);", int64(10)},
		{"ABS(0);", int64(0)},

		// Real tests
		{"ABS(10.5);", 10.5},
		{"ABS(-10.5);", 10.5},

		// Error cases
		{"ABS(TRUE);", "BUILTIN ERROR: argument to `ABS` not supported, got BOOLEAN"},
		{"ABS();", "BUILTIN ERROR: wrong number of arguments for ABS. got=0, want=1"},
		{"ABS(1, 2);", "BUILTIN ERROR: wrong number of arguments for ABS. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64: // cspell:disable-line
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64: // cspell:disable-line
				testRealObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			}
		})
	}
}

func TestBuiltinTruncFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Positive and negative values
		{"TRUNC(5.7);", int64(5)},
		{"TRUNC(-5.7);", int64(-5)},
		{"TRUNC(5.2);", int64(5)},
		{"TRUNC(-5.2);", int64(-5)},
		{"TRUNC(0.0);", int64(0)},
		{"TRUNC(5.0);", int64(5)},

		// Error cases
		{"TRUNC(5);", "BUILTIN ERROR: argument to `TRUNC` must be REAL, got LINT"},
		{"TRUNC(TRUE);", "BUILTIN ERROR: argument to `TRUNC` must be REAL, got BOOLEAN"},
		{"TRUNC();", "BUILTIN ERROR: wrong number of arguments for TRUNC. got=0, want=1"},
		{"TRUNC(1.0, 2.0);", "BUILTIN ERROR: wrong number of arguments for TRUNC. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}
