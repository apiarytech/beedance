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
	"strings"
	"testing"

	"beedance/ast"
	"beedance/lexer"
)

// IL modifiers are split off only operators that take them; SIN, LN, TAN and
// ATAN keep their names.
func TestIlMnemonicModifiers(t *testing.T) {
	tests := []struct{ mnemonic, operator, modifier string }{
		{"LDN", "LD", "N"},
		{"JMPCN", "JMP", "CN"},
		{"RETC", "RET", "C"},
		{"ANDN", "AND", "N"},
		{"SIN", "SIN", ""},
		{"LN", "LN", ""},
		{"TAN", "TAN", ""},
		{"ATAN", "ATAN", ""},
	}
	p := New(lexer.New(""))
	for _, tt := range tests {
		op, mod := p.extractIlModifiers(tt.mnemonic)
		if op != tt.operator || mod != tt.modifier {
			t.Errorf("%s: got operator %q modifier %q, want %q %q", tt.mnemonic, op, mod, tt.operator, tt.modifier)
		}
	}

	p = New(lexer.New("PROGRAM Pg LD 1 SIN END_PROGRAM"))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	body := program.Statements[0].(*ast.ProgramDeclaration).Body.(*ast.BlockStatement)
	if il := body.Statements[1].(*ast.IlInstructionStatement); il.Operator != "SIN" {
		t.Errorf("expected operator SIN, got %q", il.Operator)
	}
}

// A time or date literal without a value is a parse error, not a crash.
func TestTypedLiteralWithoutValue(t *testing.T) {
	for _, input := range []string{"T#;", "D#;", "TOD#;", "DT#;"} {
		p := New(lexer.New(input))
		p.ParseProgram()
		if len(p.Errors()) == 0 || !strings.Contains(p.Errors()[0], "expected a value for typed literal") {
			t.Errorf("%s: expected a parse error, got %v", input, p.Errors())
		}
	}
}

// A typed literal may name a type declared in a namespace.
func TestQualifiedTypedLiteral(t *testing.T) {
	p := New(lexer.New("x := Lib.Inner.Mode#Busy;"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	lit, ok := program.Statements[0].(*ast.AssignmentStatement).Value.(*ast.TypedLiteral)
	if !ok || lit.TypeName != "Lib.Inner.Mode" {
		t.Fatalf("expected a typed literal of Lib.Inner.Mode, got %#v", program.Statements[0])
	}
	// A left side that is not a name is still an error.
	p = New(lexer.New("x := a[1]#Busy;"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected an error for a[1]#Busy")
	}
}
