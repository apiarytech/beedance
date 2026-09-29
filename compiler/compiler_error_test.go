package compiler

import (
	"testing"
)

func TestCompilerErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "Undefined variable",
			input:         `unknownVar;`,
			expectedError: "undefined variable unknownVar",
		},
		{
			name: "Assign to constant",
			input: `
				PROGRAM TestAssignToConst
					VAR CONSTANT
						MyConst : INT := 5;
					END_VAR
					MyConst := 10;
				END_PROGRAM
			`,
			expectedError: "cannot assign to a constant variable 'MyConst'",
		},
		{
			name: "Type mismatch in expression",
			input: `
				PROGRAM TestTypeMismatch
					VAR
						a : INT := 5;
						b : STRING := 'hello';
					END_VAR
					a := a + b;
				END_PROGRAM
			`,
			expectedError: "type error in expression '(a + b)': operator '+' not defined for types INT and STRING",
		},
		{
			name: "EXIT outside loop",
			input: `
				PROGRAM TestExitOutsideLoop
					EXIT;
				END_PROGRAM
			`,
			expectedError: "EXIT statement not within a loop",
		},
		{
			name: "Undefined IL jump label",
			input: `
			PROGRAM JmpTest
				VAR a: INT; END_VAR
				JMP undefined_label
			end_it:
				ST a
			END_PROGRAM
			`,
			expectedError: "undefined jump label: undefined_label",
		},
		{
			name: "Invalid assignment target",
			input: `
				PROGRAM InvalidAssign
					5 := 10;
				END_PROGRAM
			`,
			expectedError: "unsupported assignment target: *ast.IntegerLiteral",
		},
		{
			name: "Array index out of bounds",
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
			name: "Array index out of bounds complex",
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
			name: "Array index division by zero",
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
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}
