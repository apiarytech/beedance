package compiler

import (
	"beedance/code"
	_ "beedance/stdlib"
	"testing"
)

func TestActionDeclaration(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			ACTION MyAction:
				VAR a : INT := 1; END_VAR
				a := a + 1;
			END_ACTION
			`,
			expectedConstants: []interface{}{
				1, // for 'a := 1' and 'a + 1'
				[]code.Instructions{ // The compiled function for the action body
					code.Make(code.OpConstant, 0), // Push 1 for initial value
					code.Make(code.OpSetLocal, 0), // Set local 'a'
					code.Make(code.OpGetLocal, 0), // Get 'a'
					code.Make(code.OpConstant, 0), // Push 1 for addition
					code.Make(code.OpAdd),         // Add
					code.Make(code.OpSetLocal, 0), // Set 'a' again
					code.Make(code.OpReturn),      // Implicit return
				},
			},
			expectedInstructions: []code.Instructions{
				// The action is compiled into a closure and stored in a global variable.
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestSFCCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			PROGRAM MySFC
				VAR cond: BOOL; END_VAR

				ACTION MyAction:
					cond := TRUE;
				END_ACTION

				INITIAL_STEP S1: MyAction(); END_STEP
				TRANSITION FROM S1 TO S2 := cond; END_TRANSITION
				STEP S2: END_STEP
			END_PROGRAM
			`,
			// We expect the ACTION to be compiled into a closure,
			// and the entire SFC to be compiled into a data structure (a hash).
			expectedConstants: []interface{}{
				// Constant 0: Compiled function for MyAction
				[]code.Instructions{
					code.Make(code.OpTrue),
					code.Make(code.OpSetGlobal, 0),
					code.Make(code.OpReturn),
				},
				// Constant 1: Compiled function for the transition condition
				[]code.Instructions{
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpReturnValue),
				},
				// Constants for building the SFC hash
				"initial_step",
				"S1",
				"actions",
				"MyAction",
				"transitions",
				"condition",
				"from",
				"to",
				"S2",
				"steps",
				"name",
				"qualifier",
				"N",
			},
			expectedInstructions: []code.Instructions{
				// Init for VAR cond: BOOL;
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 0),
				// --- Start building the SFC Hash ---
				code.Make(code.OpConstant, 2), // "initial_step"
				code.Make(code.OpConstant, 3), // "S1"
				code.Make(code.OpConstant, 4), // "actions"
				code.Make(code.OpConstant, 5), // "MyAction"
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpConstant, 6), // "transitions"
				code.Make(code.OpConstant, 7), // "condition"
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpConstant, 8), // "from"
				code.Make(code.OpConstant, 3), // "S1"
				code.Make(code.OpArray, 1),
				code.Make(code.OpConstant, 9),  // "to"
				code.Make(code.OpConstant, 10), // "S2"
				code.Make(code.OpArray, 1),
				code.Make(code.OpHash, 6),
				code.Make(code.OpArray, 1),
				code.Make(code.OpConstant, 11), // "steps"
				code.Make(code.OpConstant, 3),  // "S1" (key)
				code.Make(code.OpConstant, 4),  // "actions" (key)
				code.Make(code.OpConstant, 12), // "name"
				code.Make(code.OpConstant, 5),  // "MyAction"
				code.Make(code.OpConstant, 13), // "qualifier"
				code.Make(code.OpConstant, 14), // "N"
				code.Make(code.OpHash, 4),
				code.Make(code.OpArray, 1),
				code.Make(code.OpHash, 2),      // S1 props hash
				code.Make(code.OpConstant, 10), // "S2" (key)
				code.Make(code.OpConstant, 4),  // "actions" (key)
				code.Make(code.OpArray, 0),
				code.Make(code.OpHash, 2), // S2 props hash
				code.Make(code.OpHash, 4), // steps hash
				code.Make(code.OpHash, 8), // final SFC hash
				code.Make(code.OpPop),
			},
		},
	}
	runCompilerTests(t, tests)
}
