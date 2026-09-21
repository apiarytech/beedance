/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"beedance/code"
	_ "beedance/stdlib"
	"testing"
)

func TestFunctionBlockWithMethod(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK MyFBWithMethod
				VAR
					Internalvar: INT := 1;
				END_VAR

				METHOD MyMethod : INT
					VAR_INPUT
						MethodIn : INT;
					END_VAR
					MyMethod := MethodIn * 2;
				END_METHOD
				
				Internalvar := THIS^.MyMethod(Internalvar) + 1;
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				int64(2), // const[0] for the * 2 in the method
				[]code.Instructions{ // const[1] Method Body
					// THIS=0, MyMethod=1(ret), MethodIn=2
					code.Make(code.OpGetLocal, 2), // Get MethodIn
					code.Make(code.OpConstant, 0), // Push 2
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
				},
				int64(1),   // const[2] for initial value of Internal and the +1
				"MyMethod", // const[3] for method name string
				[]code.Instructions{ // const[4] Main Body
					// THIS=0, Internal=1
					code.Make(code.OpConstant, 2), // Push 1 (initial value)
					code.Make(code.OpSetLocal, 1), // Set Internal
					// Internal := MyMethod(Internal) + 1;
					code.Make(code.OpGetLocal, 0), // Get THIS
					code.Make(code.OpConstant, 3), // "MyMethod"
					code.Make(code.OpIndex),       // Get method closure
					code.Make(code.OpGetLocal, 1), // Get Internal (arg)
					code.Make(code.OpCall, 1),     // Call method
					code.Make(code.OpConstant, 2), // Push 1
					code.Make(code.OpAdd),
					code.Make(code.OpSetLocal, 1), // Set Internal
					code.Make(code.OpReturn),
				},
				"main", // const[5]
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 5),   // "main"
				code.Make(code.OpClosure, 4, 0), // Main body closure
				code.Make(code.OpConstant, 3),   // "MyMethod"
				code.Make(code.OpClosure, 1, 0), // Method closure
				code.Make(code.OpHash, 4),       // Create hash with 2 key-value pairs
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFBWithMethod'
			},
		},
	}
	runCompilerTests(t, tests)
}
