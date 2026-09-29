package compiler

import (
	"testing"
)

func TestArrayBoundsCheckWithConstantExpressions(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Valid index with addition",
			input: `
				PROGRAM TestValidIndex
					VAR
						MyArray : ARRAY[0..10] OF INT;
						x : INT;
					END_VAR
					x := MyArray[2 + 3];
				END_PROGRAM
			`,
			expectedError: "", // No error expected
		},
		{
			name: "Out of bounds index with subtraction",
			input: `
				PROGRAM TestOOB
					VAR
						MyArray : ARRAY[0..5] OF INT;
						x : INT;
					END_VAR
					x := MyArray[10 - 2];
				END_PROGRAM
			`,
			expectedError: "array index out of bounds: index is 8 but bounds are 0..5",
		},
		{
			name: "Valid array bounds with arithmetic",
			input: `
				PROGRAM TestValidBounds
					VAR
						MyArray : ARRAY[1*2..5+5] OF INT;
						x : INT;
					END_VAR
					x := MyArray[5];
				END_PROGRAM
			`,
			expectedError: "", // No error expected
		},
		{
			name: "Out of bounds with complex expression",
			input: `
				PROGRAM TestOOBComplex
					VAR
						MyArray : ARRAY[-5..5] OF INT;
						x : INT;
					END_VAR
					x := MyArray[2 * 3];
				END_PROGRAM
			`,
			expectedError: "array index out of bounds: index is 6 but bounds are -5..5",
		},
		{
			name: "Division by zero in index",
			input: `
				PROGRAM TestDivZero
					VAR
						MyArray : ARRAY[0..10] OF INT;
						x : INT;
					END_VAR
					x := MyArray[5 / 0];
				END_PROGRAM
			`,
			expectedError: "division by zero in constant expression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			program := parse(t, tt.input)
			err := compiler.Compile(program)

			if tt.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %s", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error %q, but got none", tt.expectedError)
				}
				if err.Error() != tt.expectedError {
					t.Fatalf("wrong error message. want=%q, got=%q", tt.expectedError, err.Error())
				}
			}
		})
	}
}
