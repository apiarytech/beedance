package stdlib

import (
	"beedance/ast"
	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"strings"
	"testing"
	"time"
)

// wstringExpectation is a helper struct for testing WString results.
type wstringExpectation struct {
	value string
}

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

	object.FinalizeBuiltins()

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			stmt := program.Statements[0].(*ast.ExpressionStatement)
			call := stmt.Expression.(*ast.CallExpression)
			funcName := call.Function.String()

			builtin, ok := object.GetBuiltinByName(funcName)
			if !ok {
				t.Fatalf("builtin not found: %s", funcName)
			}

			args := []object.Object{}
			for _, argNode := range call.Arguments {
				switch node := argNode.(type) {
				case *ast.IntegerLiteral:
					args = append(args, &object.LInt{Value: node.Value})
				case *ast.StringLiteral:
					args = append(args, &object.String{Value: node.Value})
				case *ast.TypedLiteral:
					// This handles literals like T#5s
					if strings.ToUpper(node.TypeName) == "T" || strings.ToUpper(node.TypeName) == "TIME" {
						valIdent, ok := node.Value.(*ast.Identifier)
						if !ok {
							t.Fatalf("time literal value is not an identifier: %T", node.Value)
						}
						d, err := time.ParseDuration(valIdent.Value)
						if err != nil {
							t.Fatalf("could not parse time literal '%s': %v", valIdent.Value, err)
						}
						args = append(args, &object.Time{Value: d})
					}
				default:
					t.Fatalf("unhandled argument type in test: %T", node)
				}
			}

			result := builtin.Fn(args...)

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

func TestBuiltinStringFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// LEFT
		{`LEFT('abcdef', 2);`, "ab"},
		{`LEFT("abcdef", 2);`, wstringExpectation{"ab"}},
		{`LEFT('abc', 5);`, "abc"},
		{`LEFT('abc', 0);`, ""},
		{`LEFT('abc', -1);`, ""},
		{`LEFT(123, 1);`, "BUILTIN ERROR: argument 1 to `LEFT` must be STRING or WSTRING, got LINT"},

		// RIGHT
		{`RIGHT('abcdef', 2);`, "ef"},
		{`RIGHT("abcdef", 2);`, wstringExpectation{"ef"}},
		{`RIGHT('abc', 5);`, "abc"},
		{`RIGHT('abc', 0);`, ""},
		{`RIGHT('abc', -1);`, ""},
		{`RIGHT(123, 1);`, "BUILTIN ERROR: argument 1 to `RIGHT` must be STRING or WSTRING, got LINT"},

		// MID
		{`MID('abcdef', 3, 2);`, "cd"},
		{`MID("abcdef", 1, 4);`, wstringExpectation{"abcd"}},
		{`MID('abcdef', 5, 4);`, "ef"}, // Length goes past end of string
		{`MID('abcdef', 0, 2);`, ""},   // Position <= 0
		{`MID('abcdef', 2, 0);`, ""},   // Length <= 0
		{`MID(123, 1, 1);`, "BUILTIN ERROR: argument 1 to `MID` must be STRING or WSTRING, got LINT"},

		// REPLACE
		{`REPLACE('abcdef', 'XX', 3, 2);`, "abXXef"}, // Replace 'cd' with 'XX'
		{`REPLACE('abc', 'XYZ', 2, 1);`, "aXYZc"},    // Replace 'b' with 'XYZ'
		{`REPLACE('abc', 'X', 1, 3);`, "X"},          // Replace entire string
		{`REPLACE('abc', 'X', 4, 1);`, "abcX"},       // Position > length, appends
		{`REPLACE('abc', 'X', 2, 5);`, "aX"},         // Length > remaining string
		{`REPLACE('abc', 'X', 2, 0);`, "aXbc"},       // Length is 0, acts as insert
		{`REPLACE(1, 'a', 1, 1);`, "BUILTIN ERROR: argument 1 to `REPLACE` must be STRING or WSTRING, got LINT"},

		// INSERT
		{`INSERT('ac', 'b', 2);`, "abc"},
		{`INSERT("ac", "b", 2);`, wstringExpectation{"abc"}},
		{`INSERT('abc', 'X', 1);`, "Xabc"},
		{`INSERT('abc', 'X', 4);`, "abcX"},
		{`INSERT('abc', 'X', 0);`, "Xabc"}, // Position < 1, prepends
		{`INSERT(1, 'a', 1);`, "BUILTIN ERROR: argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`INSERT('a', 1, 1);`, "BUILTIN ERROR: argument 2 to `INSERT` for strings must be STRING, got LINT"},

		// DELETE
		{`DELETE('abcdef', 2, 3);`, "abef"},
		{`DELETE("abcdef", 2, 3);`, wstringExpectation{"abef"}},
		{`DELETE('abc', 1, 1);`, "bc"},
		{`DELETE('abc', 10, 1);`, ""},   // L > len, truncates
		{`DELETE('abc', 1, 0);`, "abc"}, // l <= 0, returns original
		{`DELETE(1, 1, 1);`, "BUILTIN ERROR: argument 1 to `DELETE` must be ARRAY, STRING, or WSTRING, got LINT"},

		// CONCAT
		{`CONCAT('a', 'b', 'c');`, "abc"},
		{`CONCAT("a", "b", "c");`, wstringExpectation{"abc"}},
		{`CONCAT('a', 1);`, "BUILTIN ERROR: all arguments to `CONCAT` must be of the same type (STRING), got LINT"},

		// FIND
		{`FIND('abcdef', 'cd');`, int64(3)},
		{`FIND("abcdef", "cd");`, int64(3)},
		{`FIND('abc', 'd');`, int64(0)},
		{`FIND('abc', 1);`, "BUILTIN ERROR: argument 2 to `FIND` for strings must be STRING, got LINT"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case string:
				testStringOrError(t, evaluated, expected)
			case int64:
				testIntegerObject(t, evaluated, expected)
			case wstringExpectation:
				wstr, ok := evaluated.(*object.WString)
				if !ok {
					t.Fatalf("object is not WString. got=%T (%+v)", evaluated, evaluated)
				}
				if wstr.Value != expected.value {
					t.Errorf("wrong wstring value. want=%q, got=%q", expected.value, wstr.Value)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitShiftRotateFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// SHL
		{"SHL(BYTE#16#A5, 1);", uint64(0x4A)}, // 10100101 << 1 -> 01001010
		{"SHL(WORD#16#FF00, 8);", uint64(0x00)},
		{"SHL(DWORD#16#1, 31);", uint64(1 << 31)},
		{"SHL(BYTE#16#FF, 9);", uint64(0x00)}, // Shifted out

		// SHR
		{"SHR(BYTE#16#A5, 1);", uint64(0x52)}, // 10100101 >> 1 -> 01010010
		{"SHR(WORD#16#FF00, 8);", uint64(0x00FF)},
		{"SHR(DWORD#16#80000000, 31);", uint64(1)},

		// ROL
		{"ROL(BYTE#16#A5, 1);", uint64(0x4B)}, // 10100101 ROL 1 -> 01001011
		{"ROL(WORD#16#C0F0, 4);", uint64(0x0F0C)},
		{"ROL(DWORD#16#1, 32);", uint64(1)}, // Full rotation

		// ROR
		{"ROR(BYTE#16#A5, 1);", uint64(0xD2)}, // 10100101 ROR 1 -> 11010010
		{"ROR(WORD#16#0F0C, 4);", uint64(0xC0F0)},
		{"ROR(DWORD#16#1, 32);", uint64(1)}, // Full rotation

		// Error cases
		{"SHL(10, 1);", "BUILTIN ERROR: argument 1 to `SHL` must be a bitstring type, got LINT"},
		{"SHR(BYTE#16#FF, -1);", "BUILTIN ERROR: shift amount for `SHR` must be non-negative, got -1"},
		{"ROL(BYTE#16#FF, TRUE);", "BUILTIN ERROR: argument 2 to `ROL` must be INTEGER, got BOOLEAN"},
		{"ROR();", "BUILTIN ERROR: wrong number of arguments for ROR. got=0, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case uint64:
				bs, ok := evaluated.(*object.BitString)
				if !ok {
					t.Fatalf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
				}
				if bs.Value != expected {
					t.Errorf("wrong value. want=0x%X, got=0x%X", expected, bs.Value)
				}
			case string:
				testStringOrError(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

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

func TestBuiltinArrayFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// INSERT
		// Array Bound Functions
		{`LOWER_BOUND([1,2,3], 1);`, int64(0)},
		{`UPPER_BOUND([1,2,3], 1);`, int64(2)},
		{`UPPER_BOUND([], 1);`, int64(-1)},
		{`LOWER_BOUND(1, 1);`, "BUILTIN ERROR: argument 1 to `LOWER_BOUND` must be of type ARRAY, got LINT"},
		{`UPPER_BOUND([1], 2);`, "BUILTIN ERROR: invalid dimension 2 for 1D array"},
		{`INSERT([1, 2, 3], 99, 2);`, []int{1, 99, 2, 3}},
		{`INSERT([1, 2, 3], 99, 1);`, []int{99, 1, 2, 3}},
		{`INSERT([1, 2, 3], 99, 4);`, []int{1, 2, 3, 99}},
		{`INSERT([], 99, 1);`, []int{99}},
		{`INSERT([1], 99, 5);`, []int{1, 99}}, // Position > length, appends
		{`INSERT([1], 99, 0);`, []int{99, 1}}, // Position < 1, prepends
		{`INSERT(1, 2, 3);`, "BUILTIN ERROR: argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`INSERT([], 1, "a");`, "BUILTIN ERROR: argument 3 to `INSERT` must be INTEGER, got WSTRING"},
		{`INSERT([], 1);`, "BUILTIN ERROR: wrong number of arguments for INSERT. got=2, want=3"},

		// DELETE
		{`DELETE([1, 2, 3, 4], 2, 2);`, []int{1, 4}},
		{`DELETE([1, 2, 3], 1, 1);`, []int{2, 3}},    // DELETE(IN, L, P) -> L=1, P=1
		{`DELETE([1, 2, 3], 1, 3);`, []int{1, 2}},    // L=1, P=3
		{`DELETE([1, 2, 3], 1, 5);`, []int{1, 2, 3}}, // p > len, returns original
		{`DELETE([1, 2, 3], 3, 2);`, []int{1}},       // L=3, P=2 -> l > remaining, truncates
		{`DELETE([1, 2, 3], 4, 1);`, []int{}},        // L=4, P=1 -> l > len, truncates
		{`DELETE([1, 2, 3], 0, 1);`, []int{1, 2, 3}}, // l <= 0, returns original
		{`DELETE([1, 2, 3], 1, 0);`, []int{1, 2, 3}}, // p < 1, returns original
		{`DELETE([1, 2, 3], -1, 1);`, []int{1, 2, 3}},
		{`DELETE(1, 2, 3);`, "BUILTIN ERROR: argument 1 to `DELETE` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`DELETE([], "a", 1);`, "BUILTIN ERROR: argument 2 to `DELETE` must be INTEGER, got WSTRING"},
		{`DELETE([], 1, "a");`, "BUILTIN ERROR: argument 3 to `DELETE` must be INTEGER, got WSTRING"},
		{`DELETE([], 1);`, "BUILTIN ERROR: wrong number of arguments for DELETE. got=2, want=3"},

		// CONCAT
		{`CONCAT([1, 2], [3, 4]);`, []int{1, 2, 3, 4}},
		{`CONCAT([1], [2], [3], [4]);`, []int{1, 2, 3, 4}},
		{`CONCAT([1, 2]);`, []int{1, 2}},
		{`CONCAT([], [1]);`, []int{1}},
		{`CONCAT([1], []);`, []int{1}},
		{`CONCAT([], []);`, []int{}},
		{`CONCAT();`, "BUILTIN ERROR: wrong number of arguments for CONCAT. got=0, want>=1"},
		{`CONCAT([1], 2);`, "BUILTIN ERROR: all arguments to `CONCAT` must be of the same type (ARRAY), got LINT"},

		// FIND (for arrays)
		{`FIND([1, 2, 3], 2);`, int64(2)},
		{`FIND([1, 2, 3], 4);`, int64(0)},
		{`FIND(["a", "b", "c"], "b");`, int64(2)},
		{`FIND(["a", "b", "c"], "d");`, int64(0)},
		{`FIND([], 1);`, int64(0)},
		{`FIND([1, 2, 3], "a");`, int64(0)}, // Type mismatch, not found
		{`FIND(1, 1);`, "BUILTIN ERROR: argument 1 to `FIND` must be STRING, WSTRING, or ARRAY, got LINT"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case string:
				if !strings.HasPrefix(expected, "BUILTIN ERROR:") {
					t.Fatalf("expected value for string test should be an error message")
				}
				testStringOrError(t, evaluated, expected)
			case int64:
				testIntegerObject(t, evaluated, expected)
			case []int:
				arr, ok := evaluated.(*object.Array)
				if !ok {
					t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
				}
				if len(arr.Elements) != len(expected) {
					t.Fatalf("wrong number of elements. want=%d, got=%d", len(expected), len(arr.Elements))
				}
				for i, expectedElem := range expected {
					testIntegerObject(t, arr.Elements[i], int64(expectedElem))
				}
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

func TestBuiltinTypeConversions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// --- Valid Conversions ---
		{"INT_TO_REAL(123);", 123.0},
		{"REAL_TO_INT(123.6);", int64(124)}, // Rounding
		{"DINT_TO_SINT(127);", int64(127)},
		{"BOOL_TO_STRING(TRUE);", "TRUE"},
		{"BOOL_TO_STRING(FALSE);", "FALSE"},
		{"STRING_TO_INT(' -123 ');", int64(-123)},
		{"STRING_TO_REAL('1.23e2');", 123.0},
		{"INT_TO_BOOL(1);", true},
		{"INT_TO_BOOL(0);", false},
		{"LINT_TO_BOOL(-1);", true},
		{"REAL_TO_BOOL(0.0);", false},
		{"REAL_TO_BOOL(0.1);", true},
		{"INT_TO_BYTE(255);", uint64(255)},
		{"INT_TO_BCD(1234);", uint64(0x1234)},
		{"BCD_TO_INT(WORD#16#4321);", int64(4321)},

		// --- Out-of-Range Errors ---
		{"DINT_TO_SINT(128);", "BUILTIN ERROR: value 128 is out of range for type SINT (-128 to 127)"},
		{"DINT_TO_SINT(-129);", "BUILTIN ERROR: value -129 is out of range for type SINT (-128 to 127)"},
		{"INT_TO_USINT(-1);", "BUILTIN ERROR: value -1 is out of range for type USINT (0 to 255)"},
		{"INT_TO_USINT(256);", "BUILTIN ERROR: value 256 is out of range for type USINT (0 to 255)"},
		{"INT_TO_BYTE(256);", "BUILTIN ERROR: value 256 is out of range for type BYTE (0 to 255)"},
		{"INT_TO_BCD(10000);", "BUILTIN ERROR: value 10000 out of range for 4-digit BCD conversion (0-9999)"},

		// --- Invalid Format/Type Errors ---
		{"STRING_TO_INT('abc');", "BUILTIN ERROR: could not parse string to integer: abc"},
		{"STRING_TO_REAL('xyz');", "BUILTIN ERROR: could not parse string to real: xyz"},
		{"TIME_TO_INT(T#5s);", "BUILTIN ERROR: conversion from TIME to INT is not supported"},
		{"BOOL_TO_INT(TRUE);", "BUILTIN ERROR: conversion from BOOLEAN to INT is not supported"},
		{"BCD_TO_INT(BYTE#16#12);", "BUILTIN ERROR: argument for BCD_TO_INT must be a WORD (16-bit BitString), got BITSTRING"},
		{"BCD_TO_INT(WORD#16#1A2B);", "BUILTIN ERROR: invalid BCD format: nibble 2 has value 10 > 9"},
		{"INT_TO_REAL();", "BUILTIN ERROR: wrong number of arguments for INT_TO_REAL. got=0, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case uint64:
				bs, ok := evaluated.(*object.BitString)
				if !ok {
					t.Fatalf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
				}
				if bs.Value != expected {
					t.Errorf("wrong value. want=0x%X, got=0x%X", expected, bs.Value)
				}
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				// Can be a valid string result or an error message
				if strings.HasPrefix(expected, "BUILTIN ERROR:") {
					testStringOrError(t, evaluated, expected)
				} else {
					str, ok := evaluated.(*object.String)
					if !ok {
						t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
					}
					if str.Value != expected {
						t.Errorf("wrong string value. want=%q, got=%q", expected, str.Value)
					}
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

// testEval is a helper that parses and evaluates a given input string.
func testEval(t *testing.T, input string) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input %q: %v", input, p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement is not ExpressionStatement, got %T", program.Statements[0])
	}

	call, ok := stmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expression is not CallExpression, got %T", stmt.Expression)
	}

	funcName := call.Function.String()
	builtin, ok := object.GetBuiltinByName(strings.ToUpper(funcName))
	if !ok {
		t.Fatalf("builtin not found: %s", funcName)
	}

	// This is a simplified test evaluator for arguments.
	// It creates a dummy environment to evaluate the arguments.
	env := object.NewEnvironment()
	args := []object.Object{}
	for _, argNode := range call.Arguments {
		args = append(args, evaluator.Eval(argNode, env))
	}

	return builtin.Fn(args...)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) {
	t.Helper()
	val, _, ok := object.GetIntegerObjectValue(obj)
	if !ok {
		t.Fatalf("object is not an integer type. got=%T (%+v)", obj, obj)
	}
	if val != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", val, expected)
	}
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) {
	t.Helper()
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Fatalf("object is not Boolean. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
	}
}

func testStringObject(t *testing.T, obj object.Object, expected string) {
	t.Helper()
	result, ok := obj.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%q, want=%q", result.Value, expected)
	}
}

func testRealObject(t *testing.T, obj object.Object, expected float64) {
	t.Helper()
	var val float64
	switch o := obj.(type) {
	case *object.Real:
		val = o.Value
	case *object.LReal:
		val = o.Value
	default:
		t.Fatalf("object is not a REAL or LREAL. got=%T (%+v)", obj, obj)
	}

	if diff := val - expected; diff < -0.000001 || diff > 0.000001 {
		t.Errorf("wrong real value. want=%f, got=%f", expected, val)
	}
}

func testTimeObject(t *testing.T, obj object.Object, expected time.Duration) {
	t.Helper()
	result, ok := obj.(*object.Time)
	if !ok {
		t.Fatalf("object is not Time. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%v, want=%v", result.Value, expected)
	}
}

func testStringOrError(t *testing.T, obj object.Object, expected string) {
	t.Helper()
	errObj, isErr := obj.(*object.Error)
	if isErr {
		if !strings.Contains(errObj.Message, expected) {
			t.Errorf("error message %q does not contain %q", errObj.Message, expected)
		}
		return
	}
	testStringObject(t, obj, expected)
}
