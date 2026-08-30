package compiler

import (
	"beedance/code"
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
				1, // The constant for Internal's initial value
				// The compiled function for the body of MyFB
				[]code.Instructions{
					// Initialization for VAR
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 2), // Set 'Internal' (In=0, Out=1, Internal=2)
					// Body logic
					code.Make(code.OpGetLocal, 0), // Get 'In'
					code.Make(code.OpBang),
					code.Make(code.OpSetLocal, 1), // Set 'Out'
					code.Make(code.OpReturn),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0), // Closure points to constant 1 (the compiled func)
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}
	runCompilerTests(t, tests)
}
