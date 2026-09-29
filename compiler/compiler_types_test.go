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
	"beedance/ast"
	"beedance/object"
	"testing"
)

func TestResultingTypes(t *testing.T) {
	const (
		lint  = object.LINT_OBJ
		lreal = object.LREAL_OBJ
		boolT = object.BOOLEAN_OBJ
		str   = object.STRING_OBJ
		wstr  = object.WSTRING_OBJ
		tm    = object.TIME_OBJ
		date  = object.DATE_OBJ
		tod   = object.TIME_OF_DAY_OBJ
		dt    = object.DATE_AND_TIME_OBJ
		word  = object.WORD_OBJ
	)
	c := New()
	tests := []struct {
		op          string
		left, right object.ObjectType
		want        object.ObjectType
	}{
		// Arithmetic
		{"+", lint, lint, lint},
		{"*", lint, lreal, lreal},
		{"MOD", lint, lint, lint},
		{"+", str, str, str},
		{"+", str, wstr, wstr},
		// Time and date arithmetic
		{"+", tm, tm, tm},
		{"-", tm, tm, tm},
		{"*", tm, lint, tm},
		{"/", tm, lreal, tm},
		{"*", lint, tm, tm},
		{"-", date, date, tm},
		{"+", tod, tm, tod},
		{"-", tod, tod, tm},
		{"-", dt, tm, dt},
		{"-", dt, dt, tm},
		// Comparisons
		{"<", lint, lreal, boolT},
		{"=", str, wstr, boolT},
		{"=", boolT, boolT, boolT},
		{"<", tm, tm, boolT},
		{">=", date, date, boolT},
		{"<>", word, word, boolT},
		{"=", "COLOR", "COLOR", boolT}, // equality of same-typed values, e.g. enums
		// Logic
		{"AND", boolT, boolT, boolT},
		{"XOR", word, word, word},
		// An operand known only at runtime is not rejected.
		{"+", anyType, lint, lint},
		{"*", lreal, anyType, lreal},
		{"<", anyType, anyType, boolT},
		{"AND", anyType, boolT, boolT},
	}
	for _, tt := range tests {
		got, err := c.getResultingType(tt.op, tt.left, tt.right)
		if err != nil || got != tt.want {
			t.Errorf("%s %s %s: expected %s, got %s (err %v)", tt.left, tt.op, tt.right, tt.want, got, err)
		}
	}

	failures := []struct {
		op          string
		left, right object.ObjectType
		want        string
	}{
		{"+", boolT, lint, "operator '+' not defined for types BOOLEAN and LINT"},
		{"MOD", lint, lreal, "operator 'MOD' not defined for types LINT and LREAL"},
		{"*", tm, tm, "operator '*' not defined for types TIME and TIME"},
		{"<", str, lint, "comparison operator '<' not defined for types STRING and LINT"},
		{"<", "COLOR", "COLOR", "comparison operator '<' not defined for types COLOR and COLOR"},
		{"AND", lreal, lreal, "logical operator 'AND' not defined for types LREAL and LREAL"},
		{"**?", lint, lint, "unknown operator '**?'"},
	}
	for _, tt := range failures {
		_, err := c.getResultingType(tt.op, tt.left, tt.right)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s %s %s: expected error %q, got %v", tt.left, tt.op, tt.right, tt.want, err)
		}
	}
}

func TestTypeHelpers(t *testing.T) {
	for name, want := range map[string]object.ObjectType{
		"T": object.TIME_OBJ, "time": object.TIME_OBJ, "D": object.DATE_OBJ, "TOD": object.TIME_OF_DAY_OBJ,
		"DT": object.DATE_AND_TIME_OBJ, "int": object.INT_OBJ, "Color": "COLOR",
	} {
		if got := typedLiteralType(name); got != want {
			t.Errorf("typedLiteralType(%q): expected %s, got %s", name, want, got)
		}
	}
	for width, want := range map[int]object.ObjectType{8: object.BYTE_OBJ, 16: object.WORD_OBJ, 32: object.DWORD_OBJ, 64: object.LWORD_OBJ} {
		if got := bitStringTypeForWidth(width); got != want {
			t.Errorf("bitStringTypeForWidth(%d): expected %s, got %s", width, want, got)
		}
	}
	for typ, want := range map[object.ObjectType]bool{object.TIME_OBJ: true, object.DATE_AND_TIME_OBJ: true, object.LINT_OBJ: false} {
		if got := isTemporalType(typ); got != want {
			t.Errorf("isTemporalType(%s): expected %v, got %v", typ, want, got)
		}
	}

	// An untyped integer literal in a logical operation takes the other
	// operand's bit-string type, or LWORD.
	literal := &ast.IntegerLiteral{Value: 1}
	if got := literalAsBitString(literal, object.LINT_OBJ, object.WORD_OBJ); got != object.WORD_OBJ {
		t.Errorf("literal with a WORD: expected WORD, got %s", got)
	}
	if got := literalAsBitString(literal, object.LINT_OBJ, object.LINT_OBJ); got != object.LWORD_OBJ {
		t.Errorf("literal with an integer: expected LWORD, got %s", got)
	}
	if got := literalAsBitString(&ast.Identifier{Value: "x"}, object.INT_OBJ, object.WORD_OBJ); got != object.INT_OBJ {
		t.Errorf("a variable keeps its type: expected INT, got %s", got)
	}
}

// expressionType compiles the declarations in setup, then returns the type
// the compiler infers for the expression statement expr.
func expressionType(t *testing.T, setup, expr string) (object.ObjectType, error) {
	t.Helper()
	c := New()
	if setup != "" {
		if err := c.Compile(parse(t, setup)); err != nil {
			t.Fatalf("setup %q: compiler error: %s", setup, err)
		}
	}
	program := parse(t, expr)
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("%q is not an expression statement", expr)
	}
	return c.getExpressionType(stmt.Expression)
}

func TestInferredExpressionTypes(t *testing.T) {
	const decls = `
		VAR a : ARRAY[0..2] OF INT; n : INT; x : REAL; w : WSTRING; END_VAR
		FUNCTION Twice : DINT VAR_INPUT v : DINT; END_VAR Twice := v * 2; END_FUNCTION
		FUNCTION NoResult : INT END_FUNCTION`
	tests := []struct {
		expr string
		want object.ObjectType
	}{
		{"1.5;", object.LREAL_OBJ},
		{"LREAL#1.5;", object.LREAL_OBJ},
		{"UINT#7;", object.UINT_OBJ},
		{`"wide";`, object.WSTRING_OBJ},
		{"BYTE#16#0F;", object.BYTE_OBJ},
		{"WORD#16#0F;", object.WORD_OBJ},
		{"T#1s;", object.TIME_OBJ},
		{"D#2024-01-01;", object.DATE_OBJ},
		{"TOD#10:00:00;", object.TIME_OF_DAY_OBJ},
		{"DT#2024-01-01-10:00:00;", object.DATE_AND_TIME_OBJ},
		{"Color#Red;", "COLOR"},
		{"INT#5;", object.INT_OBJ},
		{"a[1];", object.INT_OBJ},
		{"[1, 2][0];", anyType},
		{"ABS(-1);", anyType},
		{"Twice(2);", object.DINT_OBJ},
		{"-x;", object.REAL_OBJ},
		{"NOT (1 < 2);", object.BOOLEAN_OBJ},
		{"n + 1.5;", object.LREAL_OBJ},
		{"10 AND 12;", object.LWORD_OBJ},
	}
	for _, tt := range tests {
		got, err := expressionType(t, decls, tt.expr)
		if err != nil || got != tt.want {
			t.Errorf("%s: expected %s, got %s (err %v)", tt.expr, tt.want, got, err)
		}
	}

	errorsTests := []struct {
		expr string
		want string
	}{
		{"missing;", "undefined identifier: missing"},
		{"THIS;", "cannot use THIS outside of a function block context"},
		{"missing + 1;", "undefined identifier: missing"},
		{"1 + missing;", "undefined identifier: missing"},
		{"[1, 2];", "cannot determine type of expression: *ast.ArrayLiteral"},
	}
	for _, tt := range errorsTests {
		_, err := expressionType(t, decls, tt.expr)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s: expected error %q, got %v", tt.expr, tt.want, err)
		}
	}
}

func TestLiteralConstants(t *testing.T) {
	tests := []struct {
		input    string
		wantType object.ObjectType
		want     string
	}{
		{"UINT#7;", object.ULINT_OBJ, "7"},
		{"USINT#16#FF;", object.ULINT_OBJ, "255"},
		{"LREAL#2.5;", object.LREAL_OBJ, "2.500000"},
		{"BYTE#16#0F;", object.BITSTRING_OBJ, "BYTE#16#F"},
		{"DWORD#16#FF;", object.BITSTRING_OBJ, "DWORD#16#FF"},
		{"LWORD#16#FF;", object.BITSTRING_OBJ, "LWORD#16#FF"},
		{"Color#Red;", object.ENUMERATED_VALUE_OBJ, "Color#Red"},
		{"TOD#10:30:00.5;", object.TIME_OF_DAY_OBJ, "TOD#10:30:00.5"},
		{"DT#2024-01-01-10:30:00.5;", object.DATE_AND_TIME_OBJ, "DT#2024-01-01-10:30:00.5"},
	}
	for _, tt := range tests {
		c := New()
		if err := c.Compile(parse(t, tt.input)); err != nil {
			t.Fatalf("%s: compiler error: %s", tt.input, err)
		}
		constants := c.Bytecode().Constants
		if len(constants) != 1 {
			t.Fatalf("%s: expected one constant, got %d", tt.input, len(constants))
		}
		if constants[0].Type() != tt.wantType || constants[0].Inspect() != tt.want {
			t.Errorf("%s: expected %s %s, got %s %s", tt.input, tt.wantType, tt.want, constants[0].Type(), constants[0].Inspect())
		}
	}
}

// TestUnsignedIntegerLiteral builds the node directly: the parser only
// produces it for unsigned token types.
func TestUnsignedIntegerLiteral(t *testing.T) {
	c := New()
	lit := &ast.UnsignedIntegerLiteral{Value: 1 << 63}
	if got, err := c.getExpressionType(lit); err != nil || got != object.ULINT_OBJ {
		t.Fatalf("expected ULINT, got %s (err %v)", got, err)
	}
	if err := c.Compile(lit); err != nil {
		t.Fatalf("compiler error: %s", err)
	}
	constant, ok := c.Bytecode().Constants[0].(*object.ULInt)
	if !ok || constant.Value != 1<<63 {
		t.Fatalf("expected ULINT constant %d, got %v", uint64(1<<63), c.Bytecode().Constants[0])
	}
}
