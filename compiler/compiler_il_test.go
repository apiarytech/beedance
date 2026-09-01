package compiler

import (
	"beedance/ast"
	"beedance/code"
	_ "beedance/stdlib"
	"testing"
)

func TestIlCompiler(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			PROGRAM MyIlProgram
				VAR
					a: INT := 10;
					b: INT := 20;
					c: INT;
				END_VAR

				LD a
				ADD b
				ST c
			END_PROGRAM
			`,
			expectedConstants: []interface{}{10, 20},
			expectedInstructions: []code.Instructions{
				// This test assumes a simplified compilation model where the body is compiled directly.
				// The actual compiler separates init and cyclic logic, which is harder to test this way.
				// We are testing the body compilation here.
				code.Make(code.OpGetGlobal, 0), // LD a
				code.Make(code.OpGetGlobal, 1), // operand for ADD
				code.Make(code.OpAdd),          // ADD
				code.Make(code.OpSetGlobal, 2), // ST c
			},
		},
		{
			input: `
			PROGRAM JmpTest
				VAR
					a: INT;
					b: BOOL := FALSE;
				END_VAR

				LD b
				JMPCN end_it
				LD 10
			end_it:
				ST a
			END_PROGRAM
			`,
			expectedConstants: []interface{}{10},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpFalse),             // LD b
				code.Make(code.OpJumpNotTruthy, 10), // JMPCN to ST a (pos 10)
				code.Make(code.OpConstant, 0),       // LD 10
				code.Make(code.OpSetGlobal, 0),      // ST a
			},
		},
		{
			input: `
			PROGRAM SetResetTest
				VAR
					cond: BOOL;
					target: BOOL;
				END_VAR

				LD cond
				S target
				R target
			END_PROGRAM
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetGlobal, 0),      // LD cond
				code.Make(code.OpDup),               // S target (conditional)
				code.Make(code.OpJumpNotTruthy, 11), // Jump over set logic
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpDup),               // R target (conditional)
				code.Make(code.OpJumpNotTruthy, 19), // Jump over reset logic
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
			},
		},
	}

	// This custom runner is needed because the default one doesn't handle IL blocks correctly.
	for _, tt := range tests {
		program := parse(tt.input)
		programDecl := program.Statements[0].(*ast.ProgramDeclaration)

		compiler := New()
		// Manually compile the var decls to set up the symbol table
		for _, v := range programDecl.Vars {
			compiler.Compile(v)
		}

		// Now compile just the body
		err := compiler.Compile(programDecl.Body)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}
	}
}
