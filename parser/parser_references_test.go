/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package parser

import (
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
)

// REF_TO <type> and POINTER TO <type> declare references, wherever a type
// is written; POINTER is still an ordinary name when TO does not follow.
func TestRefToTypes(t *testing.T) {
	input := `TYPE PInt : POINTER TO INT; END_TYPE
FUNCTION F : BOOL
VAR_INPUT pt : POINTER TO ARRAY[0..32000] OF REAL; END_VAR
VAR r : REF_TO INT; rr : REF_TO REF_TO INT; s : POINTER TO STRING(20); m : REF_TO MyType; POINTER : INT; END_VAR
r^ := 1; rr^^ := 2; POINTER := 3; F := pt^[0] > 0.0;
END_FUNCTION`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestRefToTypes", input)

	want := map[string]string{
		"pt": "POINTER TO ARRAY [0 .. 32000] OF REAL",
		"r":  "REF_TO INT",
		"rr": "REF_TO REF_TO INT",
		"m":  "REF_TO MyType",
	}
	var fn *ast.FunctionDeclaration
	for _, s := range program.Statements {
		switch d := s.(type) {
		case *ast.TypeDeclaration:
			if rt, ok := d.DataType.(*ast.RefToType); !ok || !rt.Pointer || rt.String() != "POINTER TO INT" {
				t.Errorf("PInt: got %T %v", d.DataType, d.DataType)
			}
		case *ast.FunctionDeclaration:
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("no function parsed")
	}
	for _, d := range append(fn.VarInputs, fn.Vars...) {
		if w, ok := want[d.Name.Value]; ok {
			if _, isRef := d.DataType.(*ast.RefToType); !isRef || d.DataType.String() != w {
				t.Errorf("%s: got %T %q, want %q", d.Name.Value, d.DataType, d.DataType.String(), w)
			}
		}
		if d.Name.Value == "s" {
			if rt, ok := d.DataType.(*ast.RefToType); !ok || !rt.Pointer {
				t.Errorf("s: got %T", d.DataType)
			}
		}
		if d.Name.Value == "POINTER" && d.DataType.String() != "INT" {
			t.Errorf("POINTER: got %q", d.DataType.String())
		}
	}
}

// R and S are Instruction List keywords too; followed by ^ they name a
// reference variable.
func TestDereferenceStatementsNamedLikeILKeywords(t *testing.T) {
	input := "VAR x : INT; r, s : REF_TO INT; END_VAR r := REF(x); r^ := 1; s^ := r^;"
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestDereferenceStatementsNamedLikeILKeywords", input)
	if len(program.Statements) != 4 {
		t.Fatalf("got %d statements: %s", len(program.Statements), program.String())
	}
	for _, s := range program.Statements[2:] {
		assign, ok := s.(*ast.AssignmentStatement)
		if !ok {
			t.Fatalf("%s: got %T", s.String(), s)
		}
		if _, ok := assign.Left.(*ast.DereferenceExpression); !ok {
			t.Errorf("%s: target is %T", s.String(), assign.Left)
		}
	}
}
