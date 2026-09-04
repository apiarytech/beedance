package stdlib

import (
	"beedance/object"
	"strings"
	"testing"
)

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
		{`DELETE('abcdef', 3, 2);`, "abef"},                     // Corrected: P=3, L=2
		{`DELETE("abcdef", 3, 2);`, wstringExpectation{"abef"}}, // Corrected: P=3, L=2
		{`DELETE('abc', 1, 1);`, "bc"},
		{`DELETE('abc', 1, 10);`, ""},   // L > len, truncates
		{`DELETE('abc', 0, 1);`, "abc"}, // l <= 0, returns original
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

func TestBuiltinArrayFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Array Bound Functions
		{`LOWER_BOUND([1,2,3], 1);`, int64(0)},
		{`UPPER_BOUND([1,2,3], 1);`, int64(2)},
		{`UPPER_BOUND([], 1);`, int64(-1)},
		{`LOWER_BOUND(1, 1);`, "BUILTIN ERROR: argument 1 to `LOWER_BOUND` must be of type ARRAY, got LINT"},
		{`UPPER_BOUND([1], 2);`, "BUILTIN ERROR: invalid dimension 2 for 1D array"},
		// INSERT
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
		{`DELETE([1, 2, 3, 4], 2, 2);`, []int{1, 4}}, // P=2, L=2
		{`DELETE([1, 2, 3], 1, 1);`, []int{2, 3}},    // P=1, L=1
		{`DELETE([1, 2, 3], 3, 1);`, []int{1, 2}},    // P=3, L=1
		{`DELETE([1, 2, 3], 5, 1);`, []int{1, 2, 3}}, // p > len, returns original
		{`DELETE([1, 2, 3], 2, 3);`, []int{1}},       // P=2, L=3 -> l > remaining, truncates
		{`DELETE([1, 2, 3], 1, 4);`, []int{}},        // P=1, L=4 -> l > len, truncates
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

		// FIRST, LAST, REST, PUSH
		{`FIRST([1, 2, 3]);`, 1},
		{`FIRST([]);`, nil},
		{`LAST([1, 2, 3]);`, 3},
		{`LAST([]);`, nil},
		{`REST([1, 2, 3]);`, []int{2, 3}},
		{`REST([]);`, nil},
		{`PUSH([], 1);`, []int{1}},
		{`PUSH([1, 2], 3);`, []int{1, 2, 3}},
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
			case int, int64:
				var exp int64
				if v, ok := expected.(int); ok {
					exp = int64(v)
				} else {
					exp = expected.(int64)
				}
				testIntegerObject(t, evaluated, exp)
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
			case nil:
				testNullObject(t, evaluated)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestLen(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`LEN("");`, 0},
		{`LEN("four");`, 4},
		{`LEN("hello world");`, 11},
		{`LEN([1, 2, 3]);`, 3},
		{`LEN([]);`, 0},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := evaluated.(*object.LInt)
		if !ok {
			t.Errorf("object is not LInt. got=%T (%+v)", evaluated, evaluated)
			continue
		}
		if integer.Value != tt.expected {
			t.Errorf("wrong value. want=%d, got=%d", tt.expected, integer.Value)
		}
	}
}

func TestLenWrongArgument(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`LEN(1);`,
			"argument to `LEN` not supported, got LINT",
		},
		{
			`LEN("one", "two");`,
			"wrong number of arguments for LEN. got=2, want=1",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Errorf("object is not Error. got=%T (%+v)", evaluated, evaluated)
			continue
		}

		if !strings.Contains(errObj.Message, tt.expected) {
			t.Errorf("wrong error message. expected=%q, got=%q", tt.expected, errObj.Message)
		}
	}
}
