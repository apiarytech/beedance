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
			expectedConstants: []interface{}{[]code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpReturn),
			}, []code.Instructions{
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpReturnValue),
			}, "initial_step", "S1", "actions", "MyAction", "transitions", "condition", "from", "to", "S2", "steps", "name", "qualifier", "N"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpConstant, 8),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpArray, 1),
				code.Make(code.OpConstant, 9),
				code.Make(code.OpConstant, 10),
				code.Make(code.OpArray, 1),
				code.Make(code.OpHash, 6),
				code.Make(code.OpArray, 1),
				code.Make(code.OpConstant, 11),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpConstant, 12),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpConstant, 13),
				code.Make(code.OpConstant, 14),
				code.Make(code.OpHash, 4),
				code.Make(code.OpArray, 1),
				code.Make(code.OpHash, 2),
				code.Make(code.OpConstant, 10),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpArray, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpHash, 4),
				code.Make(code.OpHash, 8),
				code.Make(code.OpPop),
			},
		},
	}
	runCompilerTests(t, tests)
}
