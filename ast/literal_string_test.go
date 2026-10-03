/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast_test

import (
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/token"
)

// Time and date literals print as written, with their type prefix, so that
// printed code parses back to the same literal.
func TestTimeDateLiteralStrings(t *testing.T) {
	for _, src := range []string{"T#5s", "t#5s", "TIME#1h2m", "D#2026-01-01", "DATE#2026-01-01", "TOD#12:30:00", "TIME_OF_DAY#12:30:00", "DT#2026-01-01-12:30:00"} {
		p := parser.New(lexer.New("x := " + src + ";"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%s: %v", src, p.Errors())
		}
		value := program.Statements[0].(*ast.AssignmentStatement).Value
		if got := value.String(); got != src {
			t.Errorf("%s prints as %s", src, got)
		}
		// Printed code parses back to the same literal.
		p2 := parser.New(lexer.New("x := " + value.String() + ";"))
		again := p2.ParseProgram()
		if len(p2.Errors()) != 0 || again.Statements[0].(*ast.AssignmentStatement).Value.String() != src {
			t.Errorf("%s does not parse back: %v", src, p2.Errors())
		}
	}
	// A literal built without its type token takes the short prefix; one
	// whose value holds its prefix is printed as it is.
	for _, tt := range []struct {
		node ast.Expression
		want string
	}{
		{&ast.TimeLiteral{Value: "5s"}, "T#5s"},
		{&ast.DateLiteral{Value: "2026-01-01"}, "D#2026-01-01"},
		{&ast.TimeOfDayLiteral{Value: "12:00:00"}, "TOD#12:00:00"},
		{&ast.DateAndTimeLiteral{Value: "2026-01-01-12:00:00"}, "DT#2026-01-01-12:00:00"},
		{&ast.TimeLiteral{Token: token.Token{Literal: "T#5s"}, Value: "T#5s"}, "T#5s"},
		{&ast.TimeLiteral{Token: token.Token{Literal: "TIME#5s"}, Value: "5s"}, "TIME#5s"},
	} {
		if got := tt.node.String(); got != tt.want {
			t.Errorf("%T: got %s, want %s", tt.node, got, tt.want)
		}
	}
}
