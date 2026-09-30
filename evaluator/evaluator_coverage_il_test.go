package evaluator

import (
	"testing"

	"beedance/ast"
	"beedance/object"
	"beedance/token"
)

// ilProgram wraps IL instructions in a program with some variables, and
// calls it.
func ilProgram(body string) string {
	return `PROGRAM P
	VAR a : INT := 6; b : INT := 3; x : BOOL; y : BOOL := TRUE; str : STRING := 'txt'; res : INT; END_VAR
` + body + `
END_PROGRAM
P();`
}

func TestIlArithmeticAndComparison(t *testing.T) {
	checkEval(t, []evalCase{
		{ilProgram("LD a\nSUB b"), "3"},
		{ilProgram("LD a\nMUL b"), "18"},
		{ilProgram("LD a\nDIV b"), "2"},
		{ilProgram("LD a\nGT b"), "true"},
		{ilProgram("LD a\nLT b"), "false"},
		{ilProgram("LD a\nEQ 6"), "true"},
		{ilProgram("LD a\nNE 6"), "false"},
		{ilProgram("LD a\nGE 7"), "false"},
		{ilProgram("LD a\nLE 6"), "true"},
		{ilProgram("LD y\nOR x"), "true"},
		{ilProgram("LD y\nXOR y"), "false"},
		// The N modifier negates the operand.
		{ilProgram("LDN x"), "true"},
		{ilProgram("LD y\nANDN x"), "true"},
		// A parenthesized operand is evaluated as a nested IL program.
		{ilProgram("LD a\nADD( LD b\nMUL 2\n)"), "12"},
	})
}

func TestIlStoreSetAndReset(t *testing.T) {
	checkEval(t, []evalCase{
		// STN stores the negated result.
		{ilProgram("LD y\nSTN x\nLD x"), "false"},
		// S and R change a BOOL only when the result is TRUE, and keep the result.
		{ilProgram("LD TRUE\nS x\nLD x"), "true"},
		{ilProgram("LD FALSE\nS x\nLD x"), "false"},
		{ilProgram("LD TRUE\nR y\nLD y"), "false"},
		{ilProgram("LD a\nS x"), "6"},
	})
}

func TestIlFlowControl(t *testing.T) {
	checkEval(t, []evalCase{
		// RET stops the program; the result is the current result.
		{ilProgram("LD 1\nRET\nLD 2"), "1"},
		{ilProgram("LD TRUE\nRETC\nLD 2"), "true"},
		{ilProgram("LD FALSE\nRETC\nLD 2"), "2"},
		{ilProgram("LD FALSE\nRETCN\nLD 2"), "false"},
		// A program that never loads the current result returns NULL.
		{ilProgram("RET"), "null"},
		{ilProgram("LD FALSE\nJMPCN skip\nLD 1\nskip: LD 2"), "2"},
		{ilProgram("LD TRUE\nJMPCN skip\nLD 1\nRET\nskip: LD 2"), "1"},
	})
}

func TestIlCalls(t *testing.T) {
	const fn = "FUNCTION Seven : INT Seven := 7; END_FUNCTION\n"
	checkEval(t, []evalCase{
		{fn + ilProgram("CAL Seven()"), "7"},
		{fn + ilProgram("LD TRUE\nCALC Seven()"), "7"},
		{fn + ilProgram("LD FALSE\nCALC Seven()"), "false"},
		{fn + ilProgram("LD FALSE\nCALCN Seven()"), "7"},
		{fn + ilProgram("LD TRUE\nCALCN Seven()"), "true"},
		{ilProgram("CAL nope()"), "ERROR: nope"},
	})
}

func TestIlErrors(t *testing.T) {
	checkEval(t, []evalCase{
		{ilProgram("l1: LD 1\nl1: LD 2"), "ERROR: duplicate label defined: l1"},
		{ilProgram("JMP nowhere"), "ERROR: jump target label not found: nowhere"},
		{ilProgram("JMP 5"), "ERROR: operand for JMP must be a label identifier"},
		{ilProgram("LD TRUE\nJMPC 5"), "ERROR: operand for JMPCC must be a label identifier"},
		{ilProgram("LD str\nSTN x"), "ERROR: unknown operator: NOT"},
		{ilProgram("LD 1\nST 5"), "ERROR: operand for ST must be a variable identifier"},
		{ilProgram("LD TRUE\nS 5"), "ERROR: operand for S/R must be a boolean variable"},
		{ilProgram("LD missing"), "ERROR: identifier not found: missing"},
		{ilProgram("LD 1\nADD missing"), "ERROR: identifier not found: missing"},
		{ilProgram("LDN str"), "ERROR: unknown operator: NOT"},
		{ilProgram("LD a\nADD str"), "ERROR: unsupported operator '+' for types INT and STRING"},
		{ilProgram("LD 1\nNOT"), "ERROR: unknown IL operator: NOT"},
		{ilProgram("LD 1\nADD( )"), "ERROR: parenthesized IL expression did not produce a result"},
	})
}

// The parser splits modifiers from mnemonics, but the evaluator also accepts
// a combined mnemonic such as JMPCN, and reports a missing operand.
func TestIlInstructionNodes(t *testing.T) {
	tests := []struct {
		operator string
		operand  ast.Expression
		cr       object.Object
		want     string
	}{
		{"JMPCN", &ast.Identifier{Value: "L1"}, FALSE, "JUMP to L1"},
		{"JMPC", &ast.Identifier{Value: "L1"}, TRUE, "JUMP to L1"},
		{"LDN", &ast.Boolean{Value: true}, TRUE, "false"},
		{"LD", nil, TRUE, "ERROR: instruction requires an operand"},
		{"ST", &ast.Identifier{Value: "a"}, nil, "ERROR: ST instruction executed but Current Result is not set"},
		{"ADD", &ast.IntegerLiteral{Value: 1}, nil, "ERROR: ADD instruction executed but Current Result is not set"},
	}
	for _, tt := range tests {
		env := object.NewEnvironment()
		if tt.cr != nil {
			env.Set(currentResultVar, tt.cr)
		}
		node := &ast.IlInstructionStatement{Token: token.Token{Literal: tt.operator}, Operator: tt.operator, Operand: tt.operand}
		checkEvalResult(t, tt.operator, evalIlInstructionStatement(node, env), tt.want)
	}
}

func TestIlFunctionBlock(t *testing.T) {
	checkEval(t, []evalCase{
		// An IL function block body runs when the instance is called.
		{`FUNCTION_BLOCK Twice
			VAR_INPUT i : INT; END_VAR
			VAR_OUTPUT o : INT; END_VAR
			LD i
			MUL 2
			ST o
		END_FUNCTION_BLOCK
		VAR f : Twice; out : INT; END_VAR
		f(i := 4, o => out);
		out;`, "8"},
	})
}

// An IL program that never sets the current result returns NULL.
func TestIlProgramWithoutResult(t *testing.T) {
	ret := &ast.IlInstructionStatement{Operator: "RET"}
	checkEvalResult(t, "RET", evalIlProgram([]ast.Statement{ret}, object.NewEnvironment()), "null")
}
