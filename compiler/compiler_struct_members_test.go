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

package compiler

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

func compileUnit(t *testing.T, src, name string) error {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	_, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(prog, name)
	return err
}

// IEC-72: a structured type is global to the project, so it may be declared
// after the POU that uses it, also where a member is typed in an expression.
func TestStructTypeDeclaredAfterUse(t *testing.T) {
	use := "PROGRAM P\nVAR s : ST_X; n : DINT; END_VAR\nIF s.b THEN n := 0; END_IF;\nIF (s.t - T#1s) >= T#30s THEN n := n + 1; END_IF;\ns.t := s.t + T#1ms;\nEND_PROGRAM\n"
	decl := "TYPE ST_X : STRUCT b : BOOL; t : TIME; END_STRUCT END_TYPE\n"
	for name, src := range map[string]string{"before": decl + use, "after": use + decl, "after, other case": strings.Replace(use, "ST_X", "st_x", 1) + decl} {
		if err := compileUnit(t, src, "P"); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// IEC-73: a write to a member the structure does not have is refused, as a
// read of it is; the members it has, in any case, are written.
func TestStructUnknownMemberWrite(t *testing.T) {
	decl := "TYPE ST_X : STRUCT b : BOOL; t : TIME; END_STRUCT END_TYPE\n"
	if err := compileUnit(t, decl+"PROGRAM P\nVAR s : ST_X; END_VAR\ns.nope := TRUE;\nEND_PROGRAM\n", "P"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown member written: %v", err)
	}
	if err := compileUnit(t, decl+"PROGRAM P\nVAR s : ST_X; x : BOOL; END_VAR\nx := s.nope;\nEND_PROGRAM\n", "P"); err == nil {
		t.Fatal("unknown member read")
	}
	if err := compileUnit(t, decl+"PROGRAM P\nVAR s : ST_X; a : ARRAY[1..2] OF ST_X; END_VAR\ns.B := TRUE;\ns.t := T#1s;\na[1].b := s.b;\nEND_PROGRAM\n", "P"); err != nil {
		t.Fatalf("known members: %v", err)
	}
}
