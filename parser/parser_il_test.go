package parser

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"testing"
)

func TestProgramWithIlBody(t *testing.T) {
	input := `
		PROGRAM MyIlProgram
			VAR
				myVar : INT;
			END_VAR

			LD    10
			ADD   5
			ST    myVar
		END_PROGRAM
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramWithIlBody", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	progDecl, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	body, ok := progDecl.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Program body is not ast.BlockStatement for IL. got=%T", progDecl.Body)
	}

	if len(body.Statements) != 3 {
		t.Fatalf("IL body does not have 3 statements. got=%d", len(body.Statements))
	}

	testIlInstruction(t, body.Statements[0], "LD", "10")
	testIlInstruction(t, body.Statements[1], "ADD", "5")
	testIlInstruction(t, body.Statements[2], "ST", "myVar")
}

func TestIlProgramParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, body *ast.BlockStatement)
	}{
		{
			"Basic IL Instructions",
			`
				FUNCTION_BLOCK MyIlProgram
					VAR
						Start : BOOL;
						Counter : INT;
						Result : INT;
					END_VAR

					LD    Start
					ADD   Counter
					ST    Result
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 3 {
					t.Fatalf("Function block body does not have 3 statements. got=%d", len(body.Statements))
				}

				// Test instruction 1: LD Start
				inst1 := testIlInstruction(t, body.Statements[0], "LD", "Start")
				// Test instruction 2: ADD Counter
				inst2 := testIlInstruction(t, body.Statements[1], "ADD", "Counter")
				// Test instruction 3: ST Result
				inst3 := testIlInstruction(t, body.Statements[2], "ST", "Result")

				if inst1 == nil || inst2 == nil || inst3 == nil {
					t.Fatalf("One or more IL instructions failed to parse correctly.")
				}
			},
		},
		{
			"Simple LD/ST",
			`
				FUNCTION_BLOCK TestFB
					VAR A, B : INT; END_VAR
					LD    A
					ST    B
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(body.Statements))
				}
				testIlInstruction(t, body.Statements[0], "LD", "A")
				testIlInstruction(t, body.Statements[1], "ST", "B")
			},
		},
		{
			"Simple JMP Instruction",
			`
				FUNCTION_BLOCK MyJmpFB
					JMP Label
				Label:
					RET
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(body.Statements))
				}
				testIlInstruction(t, body.Statements[0], "JMP", "Label")
				testIlInstruction(t, body.Statements[1], "RET", "")
			},
		},
		{
			"Simple CAL Instruction",
			`
				FUNCTION_BLOCK MyCalFB
					CAL MyOtherFB(In := TRUE)
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(body.Statements))
				}
				inst, ok := body.Statements[0].(*ast.IlInstructionStatement)
				if !ok {
					t.Fatalf("Statement is not IlInstructionStatement. got=%T", body.Statements[0])
				}
				if inst.Operator != "CAL" {
					t.Errorf("Operator is not CAL. got=%s", inst.Operator)
				}
				if _, ok := inst.Operand.(*ast.CallExpression); !ok {
					t.Fatalf("Operand is not CallExpression. got=%T", inst.Operand)
				}
			},
		},
		{
			"Simple SUB Instruction",
			`
				FUNCTION_BLOCK MySubFB
					LD 10
					SUB 3
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(body.Statements))
				}
				// This test ensures that mnemonics like SUB, which are not ambiguous
				// with ST operators, fall through to the default case and are
				// correctly parsed by parseIlInstruction.
				testIlInstruction(t, body.Statements[0], "LD", "10")
				testIlInstruction(t, body.Statements[1], "SUB", "3")
			},
		},
		{
			"Labels and Modifiers",
			`
				FUNCTION_BLOCK TestFB
					VAR C : BOOL; END_VAR
					Loop: LDN   C
					JMPC  Loop
					RET
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 3 {
					t.Fatalf("Expected 3 statements, got %d", len(body.Statements))
				}
				inst1 := testIlInstruction(t, body.Statements[0], "LD", "C", "N")
				if inst1.Label == nil || inst1.Label.Value != "Loop" {
					t.Errorf("Instruction 1 has incorrect label. want='Loop', got=%v", inst1.Label)
				}
				testIlInstruction(t, body.Statements[1], "JMP", "Loop", "C")
				testIlInstruction(t, body.Statements[2], "RET", "")
			},
		},
		{
			"Parenthesized Expression",
			`
				FUNCTION_BLOCK TestFB
					VAR A, B, C : INT; END_VAR
					LD(
						LD A
						ADD B )
					ST C
				END_FUNCTION_BLOCK
			`,
			func(t *testing.T, body *ast.BlockStatement) {
				if len(body.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(body.Statements))
				}
				// Manually check the LD( instruction because testIlInstruction is too simple for it.
				ldInst, ok := body.Statements[0].(*ast.IlInstructionStatement)
				if !ok {
					t.Fatalf("Statement 0 is not IlInstructionStatement. got=%T", body.Statements[0])
				}
				if ldInst.Operator != "LD" {
					t.Errorf("Operator is not LD. got=%s", ldInst.Operator)
				}
				if ldInst.Modifier != "(" {
					t.Errorf("Modifier is not '('. got=%s", ldInst.Modifier)
				}
				parenBlock, ok := ldInst.Operand.(*ast.BlockStatement)
				if !ok {
					t.Fatalf("Operand of LD( is not a BlockStatement. got=%T", ldInst.Operand)
				}
				if len(parenBlock.Statements) != 2 {
					t.Fatalf("Parenthesized block should have 2 statements. got=%d", len(parenBlock.Statements))
				}
				testIlInstruction(t, parenBlock.Statements[0], "LD", "A")
				testIlInstruction(t, parenBlock.Statements[1], "ADD", "B")

				testIlInstruction(t, body.Statements[1], "ST", "C")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p, tt.name, tt.input)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
			}

			fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
			}

			body, ok := fb.Body.(*ast.BlockStatement)
			if !ok {
				t.Fatalf("Function block body is not a BlockStatement. got=%T", fb.Body)
			}

			tt.check(t, body)
		})
	}
}

func testIlInstruction(t *testing.T, s ast.Statement, operator, operand string, modifiers ...string) *ast.IlInstructionStatement {
	t.Helper()
	inst, ok := s.(*ast.IlInstructionStatement)
	if !ok {
		t.Fatalf("Statement is not IlInstructionStatement. got=%T", s)
		return nil
	}

	if inst.Operator != operator {
		t.Errorf("Instruction operator is incorrect. want=%q, got=%q", operator, inst.Operator)
	}

	if operand != "" {
		if inst.Operand == nil {
			t.Errorf("Instruction operand is nil, want=%q", operand)
		} else if inst.Operand.String() != operand {
			t.Errorf("Instruction operand is incorrect. want=%q, got=%q", operand, inst.Operand.String())
		}
	} else {
		if inst.Operand != nil {
			t.Errorf("Instruction operand is not nil, want=nil, got=%q", inst.Operand.String())
		}
	}

	expectedModifier := ""
	if len(modifiers) > 0 {
		expectedModifier = modifiers[0]
	}

	if inst.Modifier != expectedModifier {
		t.Errorf("Instruction modifier is incorrect. want=%q, got=%q", expectedModifier, inst.Modifier)
	}

	return inst
}
