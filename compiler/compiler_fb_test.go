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
				// The compiled function for the main body of MyFB
				[]code.Instructions{ // index 0
					// Body logic (THIS=0, In=1, Out=2)
					code.Make(code.OpGetLocal, 1), // Get 'In'
					code.Make(code.OpBang),
					code.Make(code.OpSetLocal, 2), // Set 'Out'
					code.Make(code.OpReturn),
				},
				"main", // The key for the main body in the FB hash (index 1)
			},
			expectedInstructions: []code.Instructions{
				// This now compiles to a hash containing the main body as a closure.
				// "main" is constant 1, and the main body function is constant 0.
				code.Make(code.OpConstant, 1),   // "main"
				code.Make(code.OpClosure, 0, 0), // The main body function
				code.Make(code.OpHash, 2),       // Create the hash { "main": <closure> }
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFB' variable
			},
		},
	}
	runCompilerTests(t, tests)
}
