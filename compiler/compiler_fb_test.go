package compiler

import (
	"beedance/code"
	_ "beedance/stdlib"
	"testing"
)

func TestFunctionBlockDeclaration(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK MyFB
				VAR_INPUT
					In: BOOL;
				END_VAR
				VAR_OUTPUT
					Out: BOOL;
				END_VAR
				VAR
					Internalvar: INT := 1;
				END_VAR

				Out := NOT In;
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				"Out", // index 0
				"In",  // index 1
				// The compiled function for the main body of MyFB. Its only
				// parameter is THIS (local 0); the inputs and outputs are
				// fields of the instance, so they keep their values between calls.
				[]code.Instructions{ // index 2
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 0), // "Out"
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 1), // "In"
					code.Make(code.OpIndex),       // THIS.In
					code.Make(code.OpBang),
					code.Make(code.OpSetIndex), // THIS.Out := NOT THIS.In
					code.Make(code.OpReturn),
				},
				"main", // The key for the main body in the FB hash (index 3)
			},
			expectedInstructions: []code.Instructions{
				// A hash containing the main body as a closure.
				code.Make(code.OpConstant, 3),   // "main"
				code.Make(code.OpClosure, 2, 0), // The main body function
				code.Make(code.OpHash, 2),       // Create the hash { "main": <closure> }
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFB' variable
			},
		},
	}
	runCompilerTests(t, tests)
}
