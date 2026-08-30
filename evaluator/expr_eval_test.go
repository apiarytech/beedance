package evaluator

import (
	"testing"

	"beedance/object"
)

func TestEXPR(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`EXPR(5);`,
			`5`,
		},
		{
			`EXPR(5 + 8);`,
			`(5 + 8)`,
		},
		{
			`EXPR(foobar);`,
			`foobar`,
		},
		{
			`EXPR(foobar + barfoo);`,
			`(foobar + barfoo)`,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input) // This helper checks for parser errors
		quote, ok := evaluated.(*object.Quote)
		if !ok {
			t.Fatalf("expected *object.Quote. got=%T (%+v)",
				evaluated, evaluated)
		}

		if quote.Node == nil {
			t.Fatalf("quote.Node is nil")
		}

		if quote.Node.String() != tt.expected {
			t.Errorf("not equal. got=%q, want=%q",
				quote.Node.String(), tt.expected)
		}
	}
}

func TestEXPREVAL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`EXPR(EVAL(4));`, `4`},
		{`EXPR(EVAL(4 + 4));`, `8`},
		{`EXPR(8 + EVAL(4 + 4));`, `(8 + 8)`},
		{`EXPR(EVAL(4 + 4) + 8);`, `(8 + 8)`},
		{`VAR foobar : INT := 8; END_VAR EXPR(EVAL(foobar));`, `8`},
		{`EXPR(EVAL(TRUE));`, `TRUE`},
		{`EXPR(EVAL(TRUE = FALSE));`, `FALSE`},
		{`EXPR(EVAL(EXPR(4 + 4)));`, `(4 + 4)`},
		{`VAR quotedInfixExpression : MACRO := EXPR(4 + 4); END_VAR EXPR(EVAL(4 + 4) + EVAL(quotedInfixExpression));`, `(8 + (4 + 4))`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			quote, ok := evaluated.(*object.Quote)
			if !ok {
				t.Fatalf("expected *object.Quote. got=%T (%+v)",
					evaluated, evaluated)
			}

			if quote.Node == nil {
				t.Fatalf("quote.Node is nil")
			}

			if quote.Node.String() != tt.expected {
				t.Errorf("not equal. got=%q, want=%q",
					quote.Node.String(), tt.expected)
			}
		})
	}
}
