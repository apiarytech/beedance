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
					Internal: INT := 1;
				END_VAR

				Out := NOT In;
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				1, // The constant for Internal's initial value (index 0)
				// The compiled function for the main body of MyFB
				[]code.Instructions{ // index 1
					// Initialization for VAR
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 3), // Set 'Internal' (THIS=0, In=1, Out=2, Internal=3)
					// Body logic
					code.Make(code.OpGetLocal, 1), // Get 'In'
					code.Make(code.OpBang),
					code.Make(code.OpSetLocal, 2), // Set 'Out'
					code.Make(code.OpReturn),
				},
				"main", // The key for the main body in the FB hash
			},
			// The string "main" is added to constants *after* the mainFn.
			// So, if 1 is at index 0, mainFn is at index 1, then "main" is at index 2.
			expectedInstructions: []code.Instructions{
				// This now compiles to a hash containing the main body as a closure.
				code.Make(code.OpConstant, 2),   // "main" (now at index 2)
				code.Make(code.OpClosure, 1, 0), // The main body function (now at index 1)
				code.Make(code.OpHash, 2),       // Create the hash { "main": <closure> }
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFB' variable
			},
		},
	}
	runCompilerTests(t, tests)
}
