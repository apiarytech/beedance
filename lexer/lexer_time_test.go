package lexer

import (
	"testing"

	"github.com/apiarytech/beedance/token"
)

func TestTimeDateLiterals(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTokens []token.Token
	}{
		{
			"TIME short form", "T#5s", []token.Token{
				{Type: token.TIME, Literal: "T"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "5"},
				{Type: token.S, Literal: "s"},
			},
		},
		{
			"TIME long form", "TIME#1h_30m", []token.Token{
				{Type: token.TIME, Literal: "TIME"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "1"},
				{Type: token.IDENT, Literal: "h_30m"},
			},
		},
		{
			"DATE short form", "D#2026-05-21", []token.Token{
				{Type: token.DATE, Literal: "D"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "2026"},
				{Type: token.MINUS, Literal: "-"},
				{Type: token.INT, Literal: "05"},
				{Type: token.MINUS, Literal: "-"},
				{Type: token.INT, Literal: "21"},
			},
		},
		{
			"TOD with fraction", "TOD#14:30:00.123", []token.Token{
				{Type: token.TIME_OF_DAY, Literal: "TOD"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "14"},
				{Type: token.COLON, Literal: ":"},
				{Type: token.INT, Literal: "30"},
				{Type: token.COLON, Literal: ":"},
				{Type: token.REAL, Literal: "00.123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, expectedToken := range tt.expectedTokens {
				tok := l.NextToken()
				if tok.Type != expectedToken.Type {
					t.Errorf("token %d type wrong. want=%q, got=%q", i, expectedToken.Type, tok.Type)
				}
				if tok.Literal != expectedToken.Literal {
					t.Errorf("token %d literal wrong. want=%q, got=%q", i, expectedToken.Literal, tok.Literal)
				}
			}
		})
	}
}
