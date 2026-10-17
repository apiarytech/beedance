package evaluator

import "testing"

// A VAR_GLOBAL CONSTANT read inside a POU is its value, in comparisons and
// CASE labels as in arithmetic.
func TestGlobalConstantInPOU(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"VAR_GLOBAL CONSTANT K : INT := 1; END_VAR FUNCTION F : BOOL VAR_INPUT s : INT; END_VAR F := s = K; END_FUNCTION F(s := 1);", true},
		{"VAR_GLOBAL CONSTANT K : INT := 1; END_VAR FUNCTION F : BOOL VAR_INPUT s : INT; END_VAR F := s <> K; END_FUNCTION F(s := 1);", false},
		{"VAR_GLOBAL CONSTANT K : INT := 2; END_VAR FUNCTION F : BOOL VAR_INPUT s : INT; END_VAR CASE s OF K: F := TRUE; END_CASE; END_FUNCTION F(s := 2);", true},
	}
	for _, tt := range tests {
		testBooleanObject(t, testEval(t, tt.input), tt.input, tt.expected)
	}
}
