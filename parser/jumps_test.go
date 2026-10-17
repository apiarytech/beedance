/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under:
 * - GPL v2.0
 * - Commercial
 *
 * You may choose to use this software under the terms of either license.
 * See the LICENSE files in the project root for full license text.
 */

package parser

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
)

func parseErrors(src string) []string {
	p := New(lexer.New(src))
	p.ParseProgram()
	return p.Errors()
}

// IEC-68: ST jumps parse as JMP label; and label:, also as a body's first
// statement, and print back as written; IL keeps its own jumps and labels.
func TestStJumpsParse(t *testing.T) {
	src := "PROGRAM P\nVAR i : INT; END_VAR\nSTART:\ni := i + 1;\nIF i < 3 THEN JMP START; END_IF;\nEND_PROGRAM\n"
	p := New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	body := prog.Statements[0].(*ast.ProgramDeclaration).Body.(*ast.BlockStatement)
	if l, ok := body.Statements[0].(*ast.LabelStatement); !ok || l.String() != "START:" {
		t.Fatalf("first statement %T %v", body.Statements[0], body.Statements[0])
	}
	if s := prog.String(); !strings.Contains(s, "START:") || !strings.Contains(s, "JMP START;") {
		t.Fatalf("printed:\n%s", s)
	}

	il := "PROGRAM Q\nVAR x : INT; END_VAR\nAGAIN: LD x\nADD 1\nST x\nLT 3\nJMPC AGAIN\nEND_PROGRAM\n"
	p = New(lexer.New(il))
	prog = p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	ilBody := prog.Statements[0].(*ast.ProgramDeclaration).Body.(*ast.BlockStatement)
	if _, ok := ilBody.Statements[0].(*ast.IlInstructionStatement); !ok {
		t.Fatalf("an IL body starting with a label read as %T", ilBody.Statements[0])
	}
}

// IEC-68: what no engine may run is refused when parsing, the same for all.
func TestStJumpsRefused(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"no such label": {"PROGRAM P\nJMP NOWHERE;\nEND_PROGRAM\n", "no label NOWHERE"},
		"label twice":   {"PROGRAM P\nVAR i : INT; END_VAR\nL: i := 1;\nL: i := 2;\nEND_PROGRAM\n", "label L is already defined"},
		"into an IF":    {"PROGRAM P\nVAR i : INT; END_VAR\nJMP INSIDE;\nIF i > 0 THEN\nINSIDE: i := 0;\nEND_IF;\nEND_PROGRAM\n", "may leave an IF, CASE or loop but not enter one"},
		"into a FOR":    {"PROGRAM P\nVAR i, k : INT; END_VAR\nJMP INSIDE;\nFOR k := 1 TO 2 DO\nINSIDE: i := k;\nEND_FOR;\nEND_PROGRAM\n", "not enter one"},
		"another POU":   {"FUNCTION F : INT\nTHERE: F := 1;\nEND_FUNCTION\nPROGRAM P\nJMP THERE;\nEND_PROGRAM\n", "no label THERE"},
	} {
		errs := parseErrors(tc.src)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), tc.want) {
			t.Errorf("%s: %v", name, errs)
		}
	}
	// Leaving blocks is allowed, and the label may come before or after.
	ok := "PROGRAM P\nVAR i, k : INT; END_VAR\nTOP:\nFOR k := 1 TO 2 DO\n  IF k = 2 THEN JMP OUT; END_IF;\n  WHILE i < 9 DO i := i + 1; IF i = 5 THEN JMP TOP; END_IF; END_WHILE;\nEND_FOR;\nOUT:\nEND_PROGRAM\n"
	if errs := parseErrors(ok); len(errs) != 0 {
		t.Fatal(errs)
	}
	// In a CASE branch, name: is the next branch, as before.
	caseSrc := "TYPE E : (A, B); END_TYPE\nPROGRAM P\nVAR e : E; i : INT; END_VAR\nCASE e OF\nA: i := 1;\nB: i := 2;\nEND_CASE;\nEND_PROGRAM\n"
	if errs := parseErrors(caseSrc); len(errs) != 0 {
		t.Fatal(errs)
	}
}
