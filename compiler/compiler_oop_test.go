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

				THIS.Internalvar := THIS.MyMethod(THIS.Internalvar) + 1;
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				int64(2), // const[0] for the * 2 in the method
				[]code.Instructions{ // const[1] Method Body
					// THIS=0, MethodIn=1, MyMethod=2(ret)
					code.Make(code.OpNull),        // init return var
					code.Make(code.OpSetLocal, 2), // Set MyMethod (ret var)
					code.Make(code.OpGetLocal, 1), // Get MethodIn
					code.Make(code.OpConstant, 0), // Push 2
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
				},
				"MyMethod",    // const[2]
				"Internalvar", // const[3]
				int64(1),      // const[4] for the +1
				[]code.Instructions{ // const[5] Main Body
					// THIS=0
					// THIS.Internalvar := THIS.MyMethod(THIS.Internalvar) + 1;
					// RHS:
					code.Make(code.OpGetLocal, 0), // Get THIS for method call
					code.Make(code.OpConstant, 2), // "MyMethod"
					code.Make(code.OpIndex),       // Get method closure
					code.Make(code.OpGetLocal, 0), // Get THIS for argument
					code.Make(code.OpConstant, 3), // "Internalvar"
					code.Make(code.OpIndex),       // Get value of Internalvar
					code.Make(code.OpCall, 1),     // Call method
					code.Make(code.OpConstant, 4), // Push 1
					code.Make(code.OpAdd),         // Add
					// Assignment:
					code.Make(code.OpGetLocal, 0), // Get THIS for assignment
					code.Make(code.OpConstant, 3), // "Internalvar"
					code.Make(code.OpSetIndex),    // Set member
					code.Make(code.OpReturn),
				},
				"main", // const[6]
			},
			expectedInstructions: []code.Instructions{
				// The compiler sorts keys before creating the hash.
				// "main" comes before "MyMethod"
				code.Make(code.OpConstant, 6),   // "main"
				code.Make(code.OpClosure, 5, 0), // Main body closure
				code.Make(code.OpConstant, 2),   // "MyMethod"
				code.Make(code.OpClosure, 1, 0), // Method closure
				code.Make(code.OpHash, 4),       // Create hash with 2 key-value pairs
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFBWithMethod'
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionBlockProperty(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK MyFBWithProperty
				VAR
					internalvar : INT;
				END_VAR

				PROPERTY MyProp : INT
					GET
						MyProp := THIS.internalvar * 2;
					END_GET
					SET
						THIS.internalvar := value;
					END_SET
				END_PROPERTY
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				"internalvar", // const[0] (global)
				int64(2),      // const[1] (global, discovered during GET compilation)
				[]code.Instructions{ // const[2] GET body (global)
					// THIS=0, MyProp=1(ret)
					code.Make(code.OpNull),        // init return var
					code.Make(code.OpSetLocal, 1), //
					code.Make(code.OpGetLocal, 0), // Get THIS
					code.Make(code.OpConstant, 0), // "internalvar"
					code.Make(code.OpIndex),       // Get internal var
					code.Make(code.OpConstant, 1), // The constant '2'
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // const[3] SET body (global)
					// THIS=0, value=1
					code.Make(code.OpGetLocal, 1), // Get value
					code.Make(code.OpGetLocal, 0), // Get THIS
					code.Make(code.OpConstant, 0), // "internalvar"
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				[]code.Instructions{ // const[4] Main body (empty) (global)
					code.Make(code.OpReturn),
				},
				"main",       // const[5] (global)
				"get_MyProp", // const[6] (global)
				"set_MyProp", // const[7] (global)
			},
			expectedInstructions: []code.Instructions{
				// The compiler now processes hash elements in a deterministic order:
				// main, then sorted methods/properties (get_MyProp, set_MyProp).
				// The order of hash elements is now deterministic due to sorting keys.
				code.Make(code.OpConstant, 5),   // "main"
				code.Make(code.OpClosure, 4, 0), // Main body closure
				code.Make(code.OpConstant, 6),   // "get_MyProp"
				code.Make(code.OpClosure, 2, 0), // Getter closure
				code.Make(code.OpConstant, 7),   // "set_MyProp"
				code.Make(code.OpClosure, 3, 0), // Setter closure
				code.Make(code.OpHash, 6),       // Create hash with 3 key-value pairs
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}
	runCompilerTests(t, tests)
}
