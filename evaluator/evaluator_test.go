package evaluator

import (
	"github.com/apiarytech/beedance/object"
	_ "github.com/apiarytech/beedance/stdlib"
	"math"
	"strings"
	"testing"
	"time"
)

type wstringExpectation struct {
	value string
}

func TestBuiltinMove(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Test with various literal types
		{"MOVE(5);", int64(5)},
		{"MOVE(10.5);", 10.5},
		{"MOVE(TRUE);", true},
		{`MOVE('hello');`, "hello"},
		{`MOVE("hello");`, wstringExpectation{"hello"}},
		{"MOVE(T#5s);", 5 * time.Second},
		{"MOVE(BYTE#16#FF);", uint64(0xFF)},

		// Test with an expression
		{"MOVE(5 + 5);", int64(10)},

		// Error cases for wrong number of arguments
		{"MOVE();", "BUILTIN ERROR: wrong number of arguments for MOVE. got=0, want=1"},
		{"MOVE(1, 2);", "BUILTIN ERROR: wrong number of arguments for MOVE. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64: // cspell:disable-line
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64: // cspell:disable-line
				testRealObject(t, evaluated, tt.input, expected)
			case bool:
				testBooleanObject(t, evaluated, tt.input, expected)
			case string:
				// Can be a string result or an error message
				if _, ok := evaluated.(*object.Error); ok {
					testErrorObjectContains(t, evaluated, expected)
				} else {
					testStringObject(t, evaluated, tt.input, expected)
				}
			case wstringExpectation:
				wstr, ok := evaluated.(*object.WString)
				if !ok {
					t.Fatalf("object is not WString. got=%T (%+v)", evaluated, evaluated)
				}
				if wstr.Value != expected.value {
					t.Errorf("wrong wstring value. want=%q, got=%q", expected.value, wstr.Value)
				}
			case time.Duration:
				testTimeObject(t, evaluated, tt.input, expected)
			case uint64:
				if bs, ok := evaluated.(*object.BitString); !ok || bs.Value != expected {
					t.Errorf("object is not correct BitString. want=%d, got=%v", expected, evaluated)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitwiseFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// AND function
		{"AND(BYTE#16#F0, BYTE#16#A5);", uint64(0xA0)},
		{"AND(WORD#16#1234, WORD#16#00FF, WORD#16#FFFF);", uint64(0x0034)},
		{"AND(BYTE#16#FF, BYTE#16#FF);", uint64(0xFF)},

		// OR function
		{"OR(BYTE#16#A0, BYTE#16#05);", uint64(0xA5)},
		{"OR(WORD#16#1200, WORD#16#0034);", uint64(0x1234)},
		{"OR(BYTE#16#F0, BYTE#16#0F, BYTE#16#AA);", uint64(0xFF)},

		// XOR function
		{"XOR(BYTE#16#A5, BYTE#16#F0);", uint64(0x55)},
		{"XOR(WORD#16#FFFF, WORD#16#1234);", uint64(0xEDCB)},
		{"XOR(BYTE#16#FF, BYTE#16#FF, BYTE#16#FF);", uint64(0xFF)}, // A ^ A ^ A = A

		// Error cases
		{"AND(BYTE#16#FF);", "BUILTIN ERROR: wrong number of arguments for AND. got=1, want>=2"},
		{"OR(BYTE#16#FF, WORD#16#FF);", "BUILTIN ERROR: all arguments to `OR` must have the same width, got 8 and 16"},
		{"XOR(BYTE#16#FF, 10);", "BUILTIN ERROR: all arguments to `XOR` must be bit-string types, got LINT"},

		// NAND function
		{"NAND(BYTE#16#F0, BYTE#16#A5);", uint64(0x5F)},             // NOT(A0) -> 5F
		{"NAND(WORD#16#1234, WORD#16#00FF);", uint64(0xFFCB)},       // NOT(0034) -> FFCB
		{"NAND(BYTE#16#FF, BYTE#16#FF, BYTE#16#FF);", uint64(0x00)}, // NOT(FF) -> 00

		// NOR function
		{"NOR(BYTE#16#A0, BYTE#16#05);", uint64(0x5A)},       // NOT(A5) -> 5A
		{"NOR(WORD#16#1200, WORD#16#0034);", uint64(0xEDCB)}, // NOT(1234) -> EDCB
		{"NOR(BYTE#16#00, BYTE#16#00);", uint64(0xFF)},       // NOT(00) -> FF
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
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value, bs.Value)
				}
			case string:
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("object is not Error. got=%T (%+v)",
						evaluated, evaluated)
					return
				}

				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q",
						expected, errObj.Message)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinTrigFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// SIN
		{"SIN(0);", 0.0},
		{"SIN(PI / 2);", 1.0},                  // Assuming PI is a variable in the environment
		{"SIN(PI);", math.Sin(math.Pi)},        // cspell:disable-line
		{"SIN(1.570796);", math.Sin(1.570796)}, // cspell:disable-line
		{"SIN(TRUE);", "BUILTIN ERROR: argument to `SIN` must be INTEGER or REAL, got BOOLEAN"},
		{"SIN();", "BUILTIN ERROR: wrong number of arguments for SIN. got=0, want=1"},

		// COS
		{"COS(0);", 1.0},
		{"COS(PI / 2);", math.Cos(math.Pi / 2)},
		{"COS(PI);", -1.0},
		{"COS(TRUE);", "BUILTIN ERROR: argument to `COS` must be INTEGER or REAL, got BOOLEAN"},

		// TAN
		{"TAN(0);", 0.0},
		{"TAN(PI / 4);", 1.0},
		{"TAN(TRUE);", "BUILTIN ERROR: argument to `TAN` must be INTEGER or REAL, got BOOLEAN"},
	}

	for _, tt := range tests {
		evaluated := testEvalWithPi(t, tt.input) // Use a helper that defines PI
		switch expected := tt.expected.(type) {
		case float64:
			testRealObject(t, evaluated, tt.input, expected)
		case string:
			testErrorObjectContains(t, evaluated, expected)
		}
	}
}

func TestBuiltinInverseTrigFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// ASIN
		{"ASIN(0);", 0.0},
		{"ASIN(1);", math.Pi / 2},
		{"ASIN(-1);", -math.Pi / 2},
		{"ASIN(0.5);", math.Asin(0.5)}, // cspell:disable-line
		{"ASIN(2);", "BUILTIN ERROR: argument to `ASIN` must be between -1 and 1, got 2.000000"},
		{"ASIN(-2.0);", "BUILTIN ERROR: argument to `ASIN` must be between -1 and 1, got -2.000000"},
		{"ASIN(TRUE);", "BUILTIN ERROR: argument to `ASIN` must be INTEGER or REAL, got BOOLEAN"},
		{"ASIN();", "BUILTIN ERROR: wrong number of arguments for ASIN. got=0, want=1"},

		// ACOS
		{"ACOS(1);", 0.0},
		{"ACOS(-1);", math.Pi},
		{"ACOS(0);", math.Pi / 2},
		{"ACOS(0.5);", math.Acos(0.5)}, // cspell:disable-line
		{"ACOS(2);", "BUILTIN ERROR: argument to `ACOS` must be between -1 and 1, got 2.000000"},
		{"ACOS(TRUE);", "BUILTIN ERROR: argument to `ACOS` must be INTEGER or REAL, got BOOLEAN"},

		// ATAN
		{"ATAN(0);", 0.0},
		{"ATAN(1);", math.Pi / 4},
		{"ATAN(-1);", -math.Pi / 4},
		{"ATAN(100);", math.Atan(100)},
		{"ATAN(TRUE);", "BUILTIN ERROR: argument to `ATAN` must be INTEGER or REAL, got BOOLEAN"},

		// ATAN2
		{"ATAN2(1, 1);", math.Pi / 4},        // Quadrant 1
		{"ATAN2(1, -1);", 3 * math.Pi / 4},   // Quadrant 2
		{"ATAN2(-1, -1);", -3 * math.Pi / 4}, // Quadrant 3
		{"ATAN2(-1, 1);", -math.Pi / 4},      // Quadrant 4
		{"ATAN2(1.0, 0.0);", math.Pi / 2},
		{"ATAN2(1, 0);", math.Pi / 2},
		{"ATAN2(TRUE, 1);", "BUILTIN ERROR: argument 1 to `ATAN2` must be INTEGER or REAL, got BOOLEAN"},
		{"ATAN2(1, TRUE);", "BUILTIN ERROR: argument 2 to `ATAN2` must be INTEGER or REAL, got BOOLEAN"},
		{"ATAN(TRUE);", "BUILTIN ERROR: argument to `ATAN` must be INTEGER or REAL, got BOOLEAN"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case float64:
			testRealObject(t, evaluated, tt.input, expected)
		case string:
			testErrorObjectContains(t, evaluated, expected)
		}
	}
}

func TestBuiltinLogFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// LN (Natural Log)
		{"LN(E);", 1.0}, // Assuming E is a variable in the environment
		{"LN(1);", 0.0},
		{"LN(10);", math.Log(10)},
		{"LN(0);", "BUILTIN ERROR: argument to `LN` must be positive, got 0.000000"},
		{"LN(-1);", "BUILTIN ERROR: argument to `LN` must be positive, got -1.000000"},
		{"LN(TRUE);", "BUILTIN ERROR: argument to `LN` must be INTEGER or REAL, got BOOLEAN"},
		{"LN();", "BUILTIN ERROR: wrong number of arguments for LN. got=0, want=1"},

		// LOG (Base-10 Log)
		{"LOG(10);", 1.0},
		{"LOG(100);", 2.0},
		{"LOG(1);", 0.0},
		{"LOG(0.1);", -1.0},
		{"LOG(0);", "BUILTIN ERROR: argument to `LOG` must be positive, got 0.000000"},
		{"LOG(-10);", "BUILTIN ERROR: argument to `LOG` must be positive, got -10.000000"},
		{"LOG(TRUE);", "BUILTIN ERROR: argument to `LOG` must be INTEGER or REAL, got BOOLEAN"},
		{"LOG();", "BUILTIN ERROR: wrong number of arguments for LOG. got=0, want=1"},
	}

	for _, tt := range tests {
		evaluated := testEvalWithBuiltinVars(t, tt.input) // Use a helper that defines E
		switch expected := tt.expected.(type) {
		case float64: // cspell:disable-line
			testRealObject(t, evaluated, tt.input, expected)
		case string:
			testErrorObjectContains(t, evaluated, expected)
		}
	}
}

func TestBuiltinExpFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// EXP
		{"EXP(1);", math.E},
		{"EXP(0);", 1.0},
		{"EXP(2);", math.Exp(2)},
		{"EXP(-1);", math.Exp(-1)},
		{"EXP(2.5);", math.Exp(2.5)}, // cspell:disable-line
		{"EXP(TRUE);", "BUILTIN ERROR: argument to `EXP` must be INTEGER or REAL, got BOOLEAN"},
		{"EXP();", "BUILTIN ERROR: wrong number of arguments for EXP. got=0, want=1"},
		{"EXP(1, 2);", "BUILTIN ERROR: wrong number of arguments for EXP. got=2, want=1"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case float64: // cspell:disable-line
			testRealObject(t, evaluated, tt.input, expected)
		case string:
			testErrorObjectContains(t, evaluated, expected)
		}
	}
}

func TestBuiltinStringFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // string or error message
	}{
		// LEN
		{`LEN('hello');`, int64(5)},
		{`LEN("hello");`, int64(5)},
		{`LEN(1);`, "BUILTIN ERROR: argument to `LEN` not supported, got LINT"},

		// LEFT
		{`LEFT('abcdef', 2);`, "ab"},                     // cspell:disable-line
		{`LEFT("abcdef", 2);`, wstringExpectation{"ab"}}, // cspell:disable-line
		{`LEFT('abc', 5);`, "abc"},                       // cspell:disable-line
		{`LEFT("abc", 5);`, wstringExpectation{"abc"}},   // cspell:disable-line
		{`LEFT('abc', 0);`, ""},                          // cspell:disable-line
		{`LEFT("abc", -1);`, wstringExpectation{""}},     // cspell:disable-line
		{`LEFT(123, 1);`, "BUILTIN ERROR: argument 1 to `LEFT` must be STRING or WSTRING, got LINT"},
		{`LEFT('abc', 'a');`, "BUILTIN ERROR: argument 2 to `LEFT` must be INTEGER, got STRING"},
		{`LEFT("abc");`, "BUILTIN ERROR: wrong number of arguments for LEFT. got=1, want=2"},

		// RIGHT
		{`RIGHT('abcdef', 2);`, "ef"},                     // cspell:disable-line
		{`RIGHT("abcdef", 2);`, wstringExpectation{"ef"}}, // cspell:disable-line
		{`RIGHT('abc', 5);`, "abc"},                       // cspell:disable-line
		{`RIGHT("abc", 5);`, wstringExpectation{"abc"}},
		{`RIGHT('abc', 0);`, ""},
		{`RIGHT("abc", -1);`, wstringExpectation{""}},
		{`RIGHT(123, 1);`, "BUILTIN ERROR: argument 1 to `RIGHT` must be STRING or WSTRING, got LINT"},
		{`RIGHT('abc', 'a');`, "BUILTIN ERROR: argument 2 to `RIGHT` must be INTEGER, got STRING"},
		{`RIGHT("abc");`, "BUILTIN ERROR: wrong number of arguments for RIGHT. got=1, want=2"},

		// MID (P=2, L=3) -> 'bcd'
		{`MID('abcdef', 3, 2);`, "bcd"},
		{`MID("abcdef", 3, 2);`, wstringExpectation{"bcd"}},
		{`MID('abcdef', 10, 1);`, "abcdef"},
		{`MID(123, 1, 1);`, "BUILTIN ERROR: argument 1 to `MID` must be STRING or WSTRING, got LINT"},
		{`MID('abc', 'a', 1);`, "BUILTIN ERROR: argument 2 to `MID` must be INTEGER, got STRING"},
		{`MID("abc", 1, "a");`, "BUILTIN ERROR: argument 3 to `MID` must be INTEGER, got WSTRING"},

		// FIND
		{`FIND('abcdef', 'cd');`, int64(3)},  // cspell:disable-line
		{`FIND("abcdef", "cd");`, int64(3)},  // cspell:disable-line
		{`FIND('abcdef', 'xyz');`, int64(0)}, // cspell:disable-line
		{`FIND([1, 2, 3], 2);`, int64(2)},    // cspell:disable-line
		{`FIND(1, 1);`, "BUILTIN ERROR: argument 1 to `FIND` must be STRING, WSTRING, or ARRAY, got LINT"},

		// REPLACE
		{`REPLACE('abcdef', 'XX', 3, 2);`, "aXXef"},
		{`REPLACE("abcdef", "XX", 3, 2);`, wstringExpectation{"aXXef"}},
		{`REPLACE('abc', 'XX', 2, 4);`, "abcXX"},
		{`REPLACE(1, 'a', 1, 1);`, "BUILTIN ERROR: argument 1 to `REPLACE` must be STRING or WSTRING, got LINT"},
		{`REPLACE('a', 1, 1, 1);`, "BUILTIN ERROR: argument 2 to `REPLACE` must be STRING, got LINT"},

		// CONCAT (for strings)
		{`CONCAT('a', 'b', 'c');`, "abc"},
		{`CONCAT("a", "b", "c");`, wstringExpectation{"abc"}},
		{`CONCAT('a', 1);`, "BUILTIN ERROR: all arguments to `CONCAT` must be of the same type (STRING), got LINT"},

		// DELETE (for strings)
		{`DELETE('abcdef', 2, 3);`, "abef"},
		{`DELETE("abcdef", 2, 3);`, wstringExpectation{"abef"}},
		{`DELETE('abc', 10, 1);`, ""}, // L > len, truncates
		{`DELETE(123, 1, 1);`, "BUILTIN ERROR: argument 1 to `DELETE` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`DELETE('abc', 'a', 1);`, "BUILTIN ERROR: argument 2 to `DELETE` must be INTEGER, got STRING"},

		// INSERT (for strings)
		{`INSERT('abcdef', 'XX', 2);`, "abXXcdef"},
		{`INSERT("abcdef", "XX", 2);`, wstringExpectation{"abXXcdef"}},
		{`INSERT(123, 'a', 1);`, "BUILTIN ERROR: argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`INSERT('abc', 1, 1);`, "BUILTIN ERROR: argument 2 to `INSERT` for strings must be STRING, got LINT"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case string:
				if _, ok := evaluated.(*object.Error); ok {
					testErrorObjectContains(t, evaluated, expected)
				} else {
					testStringObject(t, evaluated, tt.input, expected)
				}
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
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

func TestBuiltinMinMax(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // int64, float64, or string for error
	}{
		// MIN function
		{"MIN(10, 20);", int64(10)},
		{"MIN(20, 10);", int64(10)},
		{"MIN(10, 20, 5, 30);", int64(5)},
		{"MIN(-10, -20);", int64(-20)},
		{"MIN(10);", int64(10)},
		{"MIN(10.5, 10.6);", 10.5},
		{"MIN(10, 20.5);", 10.0},
		{"MIN(10.5, 20);", 10.5},
		{"MIN(1, 2.5, -3.0, 4);", -3.0},
		{"MIN();", "BUILTIN ERROR: wrong number of arguments for MIN. got=0, want>=1"},
		{"MIN(1, TRUE);", "BUILTIN ERROR: all arguments to `MIN` must be INTEGER or REAL, got BOOLEAN"},

		// MAX function
		{"MAX(10, 20);", int64(20)},
		{"MAX(20, 10);", int64(20)},
		{"MAX(10, 20, 5, 30);", int64(30)},
		{"MAX(-10, -20);", int64(-10)},
		{"MAX(10);", int64(10)},
		{"MAX(10.5, 10.6);", 10.6},
		{"MAX(10, 20.5);", 20.5},
		{"MAX(10.5, 20);", 20.0},
		{"MAX(1, 2.5, -3.0, 4);", 4.0},
		{"MAX();", "BUILTIN ERROR: wrong number of arguments for MAX. got=0, want>=1"},
		{"MAX(1, TRUE);", "BUILTIN ERROR: all arguments to `MAX` must be INTEGER or REAL, got BOOLEAN"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64: // cspell:disable-line
				testRealObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitShiftFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be uint64 or string for error
	}{
		// SHL (Shift Left)
		{"SHL(BYTE#2#1010_0101, 1);", uint64(0x4A)}, // 0xA5 << 1 = 0x14A, masked to 8 bits is 0x4A
		{"SHL(BYTE#16#A5, 1);", uint64(0x4A)},       // Same as above
		{"SHL(WORD#16#FF00, 8);", uint64(0x0000)},
		{"SHL(WORD#16#00FF, 8);", uint64(0xFF00)},
		{"SHL(DWORD#16#1, 31);", uint64(1 << 31)},
		{"SHL(DWORD#16#1, 32);", uint64(0)}, // Shifted out
		{"SHL(10, 2);", "argument 1 to `SHL` must be a bitstring type, got LINT"},
		{"SHL(BYTE#16#10, -1);", "shift amount for `SHL` must be non-negative, got -1"},
		{"SHL(BYTE#16#10);", "wrong number of arguments for SHL. got=1, want=2"},

		// SHR (Shift Right)
		{"SHR(BYTE#2#1010_0101, 1);", uint64(0x52)}, // 0xA5 >> 1 = 0x52 (82)
		{"SHR(BYTE#16#A5, 1);", uint64(0x52)},       // Same as above
		{"SHR(WORD#16#FF00, 8);", uint64(0x00FF)},
		{"SHR(WORD#16#00FF, 8);", uint64(0x0000)},
		{"SHR(DWORD#16#80000000, 31);", uint64(1)},
		{"SHR(DWORD#16#FFFFFFFF, 32);", uint64(0)}, // Shifted out
		{"SHR(BYTE#16#10, 2.5);", "argument 2 to `SHR` must be INTEGER, got LREAL"},
		{"SHR(BYTE#16#10, -1);", "shift amount for `SHR` must be non-negative, got -1"},
		{"SHR(BYTE#16#10);", "wrong number of arguments for SHR. got=1, want=2"},
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
				// Mask the result to the width of the bitstring to handle overflow cases in tests correctly
				mask := uint64(math.MaxUint64)
				if bs.Width < 64 {
					mask = (1 << bs.Width) - 1
				}
				if (bs.Value & mask) != expected {
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value, bs.Value)
				}
			case string:
				if !testErrorObjectContains(t, evaluated, expected) {
					t.Errorf("error message did not contain expected text")
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitRotateFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be uint64 or string for error
	}{
		// ROL (Rotate Left)
		{"ROL(BYTE#2#1010_0101, 1);", uint64(0x4B)}, // ROL(0xA5, 1) -> 0x4B
		{"ROL(BYTE#16#A5, 1);", uint64(0x4B)},
		{"ROL(WORD#16#FF00, 8);", uint64(0x00FF)},
		{"ROL(WORD#16#C0F0, 4);", uint64(0x0F0C)},
		{"ROL(DWORD#16#1, 31);", uint64(1 << 31)},
		{"ROL(DWORD#16#1, 32);", uint64(1)}, // Rotated full circle
		{"ROL(10, 2);", "argument 1 to `ROL` must be a bitstring type, got LINT"},
		{"ROL(BYTE#16#10, -1);", "rotate amount for `ROL` must be non-negative, got -1"},
		{"ROL(BYTE#16#10);", "wrong number of arguments for ROL. got=1, want=2"},

		// ROR (Rotate Right)
		{"ROR(BYTE#2#1010_0101, 1);", uint64(0xD2)}, // ROR(0xA5, 1) -> 0xD2
		{"ROR(BYTE#16#A5, 1);", uint64(0xD2)},
		{"ROR(WORD#16#FF00, 8);", uint64(0x00FF)},
		{"ROR(WORD#16#0F0C, 4);", uint64(0xC0F0)},
		{"ROR(DWORD#16#80000000, 31);", uint64(1)},
		{"ROR(DWORD#16#FFFFFFFF, 32);", uint64(0xFFFFFFFF)}, // Rotated full circle
		{"ROR(BYTE#16#10, 2.5);", "argument 2 to `ROR` must be INTEGER, got LREAL"},
		{"ROR(BYTE#16#10, -1);", "rotate amount for `ROR` must be non-negative, got -1"},
		{"ROR();", "wrong number of arguments for ROR. got=0, want=2"},
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
				// Mask the result to the width of the bitstring to handle overflow cases in tests correctly
				mask := uint64(math.MaxUint64)
				if bs.Width < 64 {
					mask = (1 << bs.Width) - 1
				}
				if (bs.Value & mask) != (expected & mask) {
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value&mask, bs.Value&mask)
				}
			case string:
				if !testErrorObjectContains(t, evaluated, expected) {
					t.Errorf("error message did not contain expected text")
				}
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
		{`INSERT([1, 2, 3], 99, 2);`, []int{1, 99, 2, 3}},
		{`INSERT([1, 2, 3], 99, 1);`, []int{99, 1, 2, 3}},
		{`INSERT([1, 2, 3], 99, 4);`, []int{1, 2, 3, 99}},
		{`INSERT([], 99, 1);`, []int{99}},
		{`INSERT([1], 99, 5);`, []int{1, 99}}, // Position > length, appends
		{`INSERT([1], 99, 0);`, []int{99, 1}}, // Position < 1, prepends
		{`INSERT(1, 2, 3);`, "BUILTIN ERROR: argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`INSERT([], 1, "a");`, "BUILTIN ERROR: argument 3 to `INSERT` must be INTEGER, got WSTRING"},
		{`INSERT([], 1);`, "wrong number of arguments for INSERT. got=2, want=3"},

		// DELETE
		{`DELETE([1, 2, 3, 4], 2, 2);`, []int{1, 4}},
		{`DELETE([1, 2, 3], 1, 1);`, []int{2, 3}},    // P=1, L=1
		{`DELETE([1, 2, 3], 1, 3);`, []int{1, 2}},    // P=3, L=1
		{`DELETE([1, 2, 3], 1, 5);`, []int{1, 2, 3}}, // p > len, returns original
		{`DELETE([1, 2, 3], 3, 2);`, []int{1}},       // P=2, L=3 -> l > remaining, truncates
		{`DELETE([1, 2, 3], 4, 1);`, []int{}},        // P=1, L=4 -> l > len, truncates
		{`DELETE([1, 2, 3], 1, 0);`, []int{1, 2, 3}}, // p < 1, returns original
		{`DELETE([1, 2, 3], 0, 1);`, []int{1, 2, 3}}, // l <= 0, returns original
		{`DELETE([1, 2, 3], 1, -1);`, []int{1, 2, 3}},
		{`DELETE(1, 3, 2);`, "BUILTIN ERROR: argument 1 to `DELETE` must be ARRAY, STRING, or WSTRING, got LINT"},
		{`DELETE([], "a", 1);`, "BUILTIN ERROR: argument 2 to `DELETE` must be INTEGER, got WSTRING"},
		{`DELETE([], 1, "a");`, "BUILTIN ERROR: argument 3 to `DELETE` must be INTEGER, got WSTRING"},
		{`DELETE([], 1);`, "wrong number of arguments for DELETE. got=2, want=3"},

		// CONCAT
		{`CONCAT([1, 2], [3, 4]);`, []int{1, 2, 3, 4}},
		{`CONCAT([1], [2], [3], [4]);`, []int{1, 2, 3, 4}},
		{`CONCAT([1, 2]);`, "BUILTIN ERROR: wrong number of arguments for CONCAT. got=1, want>=2"},
		{`CONCAT([], [1]);`, []int{1}},
		{`CONCAT([1], []);`, []int{1}},
		{`CONCAT([], []);`, []int{}},
		{`CONCAT();`, "BUILTIN ERROR: wrong number of arguments for CONCAT. got=0, want>=2"},
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
				testErrorObjectContains(t, evaluated, expected)
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case []int:
				arr, ok := evaluated.(*object.Array)
				if !ok {
					t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
				}
				if len(arr.Elements) != len(expected) {
					t.Fatalf("wrong number of elements. want=%d, got=%d", len(expected), len(arr.Elements))
				}
				for i, expectedElem := range expected {
					testIntegerObject(t, arr.Elements[i], "elem", int64(expectedElem))
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestMuxFunction(t *testing.T) {
	type wstringExpectation struct {
		value string
	}

	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer selection
		{"MUX(0, 100, 101, 102);", int64(100)},
		{"MUX(1, 100, 101, 102);", int64(101)},
		{"MUX(2, 100, 101, 102);", int64(102)},

		// Real selection
		{"MUX(1, 10.5, 20.5, 30.5);", 20.5},

		// String selection
		{`MUX(0, "a", "b", "c");`, wstringExpectation{"a"}},
		{`MUX(2, "a", "b", "c");`, wstringExpectation{"c"}},

		// Boolean selection
		{"MUX(1, FALSE, TRUE);", true},

		// Selection with expression as selector
		{"MUX(1+1, 10, 20, 30);", int64(30)},

		// Error cases
		{"MUX(3, 100, 101, 102);", "BUILTIN ERROR: index 3 out of bounds for MUX with 3 inputs"},
		{"MUX(-1, 100, 101, 102);", "BUILTIN ERROR: index -1 out of bounds for MUX"},
		{"MUX(0.5, 100, 101);", "BUILTIN ERROR: argument 1 to `MUX` must be INTEGER, got LREAL"},
		{`MUX(0, 100, "a");`, "BUILTIN ERROR: all value arguments to `MUX` must be of the same type, got WSTRING but expected LINT"},
		{"MUX(0);", "BUILTIN ERROR: wrong number of arguments for MUX. got=1, want>=2"},
		{"MUX();", "BUILTIN ERROR: wrong number of arguments for MUX. got=0, want>=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64: // cspell:disable-line
				testRealObject(t, evaluated, tt.input, expected)
			case bool:
				testBooleanObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
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

func TestSelFunction(t *testing.T) {
	type wstringExpectation struct {
		value string
	}

	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer selection
		{"SEL(FALSE, 10, 20);", int64(10)},
		{"SEL(TRUE, 10, 20);", int64(20)},

		// Real selection
		{"SEL(FALSE, 10.5, 20.5);", 10.5},
		{"SEL(TRUE, 10.5, 20.5);", 20.5},

		// Boolean selection
		{"SEL(FALSE, TRUE, FALSE);", true},
		{"SEL(TRUE, TRUE, FALSE);", false},

		// String selection
		{`SEL(FALSE, "hello", "world");`, wstringExpectation{"hello"}},
		{`SEL(TRUE, "hello", "world");`, wstringExpectation{"world"}},

		// Selection with expressions
		{"SEL(1 > 0, 5+5, 10+10);", int64(20)},
		{"SEL(1 < 0, 5+5, 10+10);", int64(10)},

		// Error cases
		{"SEL(1, 10, 20);", "BUILTIN ERROR: argument 1 to `SEL` must be BOOLEAN, got LINT"},
		{`SEL(TRUE, 10, "world");`, "BUILTIN ERROR: arguments 2 and 3 to `SEL` must be of the same type, got LINT and WSTRING"},
		{"SEL(TRUE, 10);", "BUILTIN ERROR: wrong number of arguments for SEL. got=2, want=3"},
		{"SEL(TRUE, 10, 20, 30);", "BUILTIN ERROR: wrong number of arguments for SEL. got=4, want=3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64: // cspell:disable-line
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64:
				testRealObject(t, evaluated, tt.input, expected)
			case bool:
				testBooleanObject(t, evaluated, tt.input, expected)
			case string:
				testErrorObjectContains(t, evaluated, expected)
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
