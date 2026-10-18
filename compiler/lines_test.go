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
	"slices"
	"testing"

	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

func TestLinesMapTheCodeToTheSource(t *testing.T) {
	src := `FUNCTION_BLOCK Acc
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT sum : INT; END_VAR
VAR_TEMP d : INT; END_VAR
d := x * 2;
sum := sum + d;
END_FUNCTION_BLOCK

PROGRAM Main
VAR a : Acc; n : INT; END_VAR
n := n + 1;
IF n > 2 THEN
    a(x := n);
END_IF;
END_PROGRAM
`
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	compiled, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, "Main")
	if err != nil {
		t.Fatal(err)
	}
	rows := func(lines []object.SourceLine, ins code.Instructions) []int {
		var out []int
		for i, l := range lines {
			if i > 0 && l.Offset <= lines[i-1].Offset || l.Offset >= len(ins) {
				t.Errorf("line %+v out of order or past the code (%d bytes)", l, len(ins))
			}
			out = append(out, l.Row)
		}
		return out
	}
	body := compiled.CyclicBytecode
	if got := rows(body.Lines, body.Instructions); !slices.Equal(got, []int{11, 12, 13}) {
		t.Errorf("the body's lines %v, want [11 12 13]", got)
	}
	var acc *object.CompiledFunction
	for _, c := range body.Constants {
		if fn, ok := c.(*object.CompiledFunction); ok && fn.Name == "Acc" {
			acc = fn
		}
	}
	if acc == nil {
		t.Fatal("no code named Acc")
	}
	if got := rows(acc.Lines, acc.Instructions); !slices.Equal(got, []int{5, 6}) {
		t.Errorf("Acc's lines %v, want [5 6]", got)
	}
	if !slices.Contains(acc.LocalNames, "d") || acc.LocalNames[0] != "THIS" {
		t.Errorf("Acc's locals %q", acc.LocalNames)
	}
	if l, ok := object.LineAt(acc.Lines, acc.Lines[1].Offset+1); !ok || l.Row != 6 {
		t.Errorf("LineAt inside line 6: %+v %v", l, ok)
	}
}
