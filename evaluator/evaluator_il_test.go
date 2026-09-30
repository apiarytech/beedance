package evaluator

import (
	"beedance/ast"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"testing"
)

func TestIlProgramEvaluation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "Simple Add and Store",
			input: `
			PROGRAM TestIL
				VAR
					myVar : INT;
				END_VAR

				LD 10
				ADD 5
				ST myVar
			END_PROGRAM
		`,
			expected: 15,
		},
		{
			name: "LD and ST with initialized and uninitialized vars",
			input: `
			PROGRAM TestLDST
				VAR
					varA : INT := 10;
					varB : INT;
				END_VAR

				LD varA
				ADD 5
				ST varB
				LD varB
			END_PROGRAM
		`,
			expected: 15,
		},
		{
			name: "Add two variables",
			input: `
			PROGRAM TestIL2
				VAR
					a : INT := 10;
					b : INT := 20;
					c : INT;
				END_VAR

				LD a
				ADD b
				ST c
				LD c
			END_PROGRAM
		`,
			expected: 30,
		},
		{
			name: "JMPC when TRUE",
			input: `
			PROGRAM TestIL3
				VAR
					a : BOOL;
					b : INT;
				END_VAR

				LD TRUE
				ST a
				LD a
				JMPC set_b
				LD 0
				ST b
				JMP end_jmp
			set_b:
				LD 1
				ST b
			end_jmp:
				LD b
			END_PROGRAM
		`,
			expected: 1,
		},
		{
			name: "JMPC with non-zero integer",
			input: `
			PROGRAM TestJmpcInt
				VAR
					b : INT;
				END_VAR

				LD 5
				JMPC set_b_to_1
				LD 0
				ST b
				JMP end_jmp
			set_b_to_1:
				LD 1
				ST b
			end_jmp:
				LD b
			END_PROGRAM
		`,
			expected: 1,
		},
		{
			name: "JMPC with zero integer",
			input: `
			PROGRAM TestJmpcIntZero
				VAR
					b : INT;
				END_VAR

				LD 0
				JMPC set_b_to_1
				LD 0
				ST b
				JMP end_jmp
			set_b_to_1:
				LD 1
				ST b
			end_jmp:
				LD b
			END_PROGRAM
		`,
			expected: 0,
		},
		{
			name: "JMPCN with zero integer",
			input: `
			PROGRAM TestJmpcnIntZero
				VAR
					b : INT;
				END_VAR

				LD 0
				JMPCN set_b_to_1
				LD 0
				ST b
				JMP end_jmp
			set_b_to_1:
				LD 1
				ST b
			end_jmp:
				LD b
			END_PROGRAM
		`,
			expected: 1,
		},
		{
			name: "JMPCN with non-zero integer",
			input: `
			PROGRAM TestJmpcnInt
				VAR
					b : INT;
				END_VAR

				LD -5
				JMPCN set_b_to_1
				LD 0
				ST b
				JMP end_jmp
			set_b_to_1:
				LD 1
				ST b
			end_jmp:
				LD b
			END_PROGRAM
		`,
			expected: 0,
		},
		{
			name: "JMPC when FALSE",
			input: `
			PROGRAM TestIL4
				VAR
					a : BOOL;
					b : INT;
				END_VAR
				LD FALSE
				ST a
				LD a
				JMPC set_b
				LD 0
				ST b
				JMP end_jmp
			set_b:
				LD 1
				ST b
			end_jmp:
				LD b
			END_PROGRAM
		`,
			expected: 0,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { // cspell:disable-line
			// Parse to get program name
			l := lexer.New(tt.input)
			p := parser.New(l)
			programAST := p.ParseProgram()
			checkParserErrors(t, p, "TestIlProgramEvaluation", tt.input)
			var progName string
			for _, stmt := range programAST.Statements {
				if progDecl, ok := stmt.(*ast.ProgramDeclaration); ok {
					progName = progDecl.Name.Value
					break
				}
			}
			if progName == "" {
				t.Fatalf("No PROGRAM declaration found in test input for %q", tt.name)
			}

			// Setup env and define the program
			env := object.NewEnvironment()
			testEvalWithEnv(t, tt.input, env)

			// Call the program, then read the current result it ends with.
			result := testEvalWithEnv(t, progName+"();", env)
			if prog, ok := env.Get(progName); ok && !isError(result) {
				result, _ = prog.(*object.Program).Env.GetRaw(currentResultVar)
			}

			switch expected := tt.expected.(type) {
			case int:
				if !testIntegerObject(t, result, "result", int64(expected)) {
					t.Logf("Test %d/%d", i+1, len(tests))
				}
			case int64:
				if !testIntegerObject(t, result, "result", expected) {
					t.Logf("Test %d/%d", i+1, len(tests))
				}
			case bool:
				if !testBooleanObject(t, result, "result", expected) {
					t.Logf("Test %d/%d", i+1, len(tests))
				}
			default:
				t.Fatalf("unsupported expected type: %T", tt.expected)
			}
		})
	}
}
