package ast

import (
	"testing"

	"beedance/token"
)

func TestString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&VarDeclStatement{
				Token: token.Token{Type: token.VAR, Literal: "VAR"},
				Name: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "myVar"},
					Value: "myVar",
				},
				DataType: &TypeSpecifier{Token: token.Token{Type: token.IDENT, Literal: "IDENT"}},
				Value: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "anotherVar"},
					Value: "anotherVar",
				},
			},
		},
	}

	if program.String() != "VAR myVar : IDENT := anotherVar;" {
		t.Errorf("program.String() wrong. got=%q", program.String())
	}
}
