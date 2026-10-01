package stdlib

import (
	"beedance/object"
	"strings"
	"testing"
)

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
		{"TIME_TO_INT(T#40s);", "BUILTIN ERROR: value 40000 is out of range for type INT (-32768 to 32767)"},
		{"BOOL_TO_DATE(TRUE);", "BUILTIN ERROR: conversion from BOOLEAN to DATE is not supported"},
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
