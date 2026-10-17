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

package inputwrites_test

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/inputwrites"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// The check refuses exactly what the VM's compiler refuses (IEC-69), so the
// evaluator and the transpiler, which use it, agree with the VM.
func TestCheckAgreesWithTheVM(t *testing.T) {
	fns := "FUNCTION F1 : INT\nVAR_INPUT a : INT; END_VAR\nF1 := a;\nEND_FUNCTION\n"
	fb2 := "FUNCTION_BLOCK FB2\nEND_FUNCTION_BLOCK\n"
	use := "\nPROGRAM P\nVAR f : FB; d : FB2; x : INT; END_VAR\nf(); d(); x := F1(1);\nEND_PROGRAM\n"
	for name, tc := range map[string]struct {
		src    string
		refuse string // the variable named, "" when allowed
	}{
		"fb assign":                 {"FUNCTION_BLOCK FB\nVAR_INPUT a : BOOL; END_VAR\na := FALSE;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, "a"},
		"fb assign in an IF":        {"FUNCTION_BLOCK FB\nVAR_INPUT a : BOOL; END_VAR\nIF TRUE THEN a := TRUE; END_IF;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, "a"},
		"fb assign in a CASE":       {"FUNCTION_BLOCK FB\nVAR_INPUT a : INT; END_VAR\nCASE a OF 1: a := 2; END_CASE;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, "a"},
		"function assign":           {"FUNCTION_BLOCK FB\nEND_FUNCTION_BLOCK\n" + fb2 + "FUNCTION F1 : INT\nVAR_INPUT a : INT; END_VAR\na := 2; F1 := a;\nEND_FUNCTION\n", "a"},
		"method writes fb input":    {"FUNCTION_BLOCK FB\nVAR_INPUT a : BOOL; END_VAR\nMETHOD M\na := TRUE;\nEND_METHOD\nEND_FUNCTION_BLOCK\n" + fb2 + fns, "a"},
		"method own input":          {"FUNCTION_BLOCK FB\nMETHOD M\nVAR_INPUT k : INT; END_VAR\nk := 1;\nEND_METHOD\nEND_FUNCTION_BLOCK\n" + fb2 + fns, "k"},
		"derived writes base input": {"FUNCTION_BLOCK FB\nVAR_INPUT a : BOOL; END_VAR\nEND_FUNCTION_BLOCK\nFUNCTION_BLOCK FB2 EXTENDS FB\na := TRUE;\nEND_FUNCTION_BLOCK\n" + fns, "a"},
		"output":                    {"FUNCTION_BLOCK FB\nVAR_OUTPUT a : BOOL; END_VAR\na := TRUE;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, ""},
		"element of an input":       {"FUNCTION_BLOCK FB\nVAR_INPUT a : ARRAY[1..2] OF INT; END_VAR\na[1] := 3;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, ""},
		"member of an input":        {"TYPE ST_S : STRUCT b : BOOL; END_STRUCT END_TYPE\nFUNCTION_BLOCK FB\nVAR_INPUT a : ST_S; END_VAR\na.b := TRUE;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, ""},
		"FOR with an input":         {"FUNCTION_BLOCK FB\nVAR_INPUT a : INT; END_VAR\nFOR a := 1 TO 2 DO END_FOR;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, ""},
		"IL ST":                     {"FUNCTION_BLOCK FB\nVAR_INPUT a : BOOL; END_VAR\nLD TRUE\nST a\nEND_FUNCTION_BLOCK\n" + fb2 + fns, ""},
		"edge input":                {"FUNCTION_BLOCK FB\nVAR_INPUT a : BOOL R_EDGE; END_VAR\nVAR_OUTPUT q : BOOL; END_VAR\nq := a;\nEND_FUNCTION_BLOCK\n" + fb2 + fns, ""},
	} {
		src := tc.src + use
		p := parser.New(lexer.New(src))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("%s: %v", name, errs)
		}
		checkErr := inputwrites.Check(prog)

		p = parser.New(lexer.New(src))
		_, vmErr := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(p.ParseProgram(), "P")

		want := ""
		if tc.refuse != "" {
			want = "cannot assign to read-only variable '" + tc.refuse + "'"
		}
		for engine, err := range map[string]error{"check": checkErr, "vm": vmErr} {
			switch {
			case want == "" && err != nil:
				t.Errorf("%s: %s refused it: %v", name, engine, err)
			case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
				t.Errorf("%s: %s: %v, want %q", name, engine, err, want)
			}
		}
	}
}
