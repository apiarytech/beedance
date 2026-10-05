package parser

import (
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
)

// Comments may stand anywhere white space may in IL: on their own lines,
// after a label, at the end of an instruction and before the first one.
func TestIlComments(t *testing.T) {
	input := `
FUNCTION_BLOCK CounterIL
	VAR_INPUT Reset : BOOL; END_VAR
	VAR Cnt : INT; END_VAR
	VAR_OUTPUT Out : INT; END_VAR
(* count, or reset *)
LD Reset
JMPC ResetCnt (* to the reset *)

(* increment counter *)
LD Cnt // a line comment
ADD (* inline *) 1
NOT (* no operand *)
NOT
JMP QuitFb

ResetCnt:
(* reset counter *)
LD 0

QuitFb: (* save results *)
ST Cnt
ST Out
END_FUNCTION_BLOCK
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIlComments", input)
	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("not a function block: %T", program.Statements[0])
	}
	body, ok := fb.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("body is %T, want an IL block", fb.Body)
	}
	want := []string{"LD", "JMP", "LD", "ADD", "NOT", "NOT", "JMP", "LD", "ST", "ST"}
	if len(body.Statements) != len(want) {
		t.Fatalf("got %d instructions, want %d: %s", len(body.Statements), len(want), body.String())
	}
	for i, s := range body.Statements {
		il, ok := s.(*ast.IlInstructionStatement)
		if !ok || il.Operator != want[i] {
			t.Errorf("instruction %d = %s, want %s", i, s.String(), want[i])
		}
	}
	if l := body.Statements[7].(*ast.IlInstructionStatement).Label; l == nil || l.Value != "ResetCnt" {
		t.Errorf("label of instruction 7 = %v, want ResetCnt", l)
	}
	if l := body.Statements[8].(*ast.IlInstructionStatement).Label; l == nil || l.Value != "QuitFb" {
		t.Errorf("label of instruction 8 = %v, want QuitFb", l)
	}
}

// Comments inside ST expressions and argument lists.
func TestExpressionComments(t *testing.T) {
	input := `PROGRAM P
VAR x, y : INT; END_VAR
x := (* first *) y (* then *) + 1;
y := MAX(x, (* a bound *) 10) // the larger
  ;
IF (* only *) x > 0 THEN x := 0; END_IF;
END_PROGRAM`
	p := New(lexer.New(input))
	p.ParseProgram()
	checkParserErrors(t, p, "TestExpressionComments", input)
}
