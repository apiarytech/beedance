package lexer

import (
	"testing"

	"beedance/token"
)

func TestTimeDateLiterals(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedType    token.TokenType
		expectedLiteral string
	}{
		// TIME literals
		{"TIME short form", "T#5s", token.TIME, "T#5s"},
		{"TIME long form", "TIME#1h_30m", token.TIME, "TIME#1h_30m"},
		{"TIME negative", "T#-10s_500ms", token.TIME, "T#-10s_500ms"},
		{"TIME with fraction", "T#1.5s", token.TIME, "T#1.5s"},

		// DATE literals
		{"DATE short form", "D#2026-05-21", token.DATE, "D#2026-05-21"},
		{"DATE long form", "DATE#2026-05-21", token.DATE, "DATE#2026-05-21"},

		// TIME_OF_DAY literals
		{"TOD short form", "TOD#14:30:00", token.TIME_OF_DAY, "TOD#14:30:00"},
		{"TOD long form with fraction", "TIME_OF_DAY#14:30:00.123", token.TIME_OF_DAY, "TIME_OF_DAY#14:30:00.123"},

		// DATE_AND_TIME literals
		{"DT short form", "DT#2026-05-21-14:30:00", token.DATE_AND_TIME, "DT#2026-05-21-14:30:00"},
		{"DT long form with fraction", "DATE_AND_TIME#2026-05-21-14:30:00.5", token.DATE_AND_TIME, "DATE_AND_TIME#2026-05-21-14:30:00.5"},

		// Separated by other tokens
		{"TIME followed by semicolon", "T#10s;", token.TIME, "T#10s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != tt.expectedType {
				t.Errorf("token type wrong. want=%q, got=%q", tt.expectedType, tok.Type)
			}
			if tok.Literal != tt.expectedLiteral {
				t.Errorf("token literal wrong. want=%q, got=%q", tt.expectedLiteral, tok.Literal)
			}

			// If the input has a semicolon, check that it's the next token
			if tt.input[len(tt.input)-1] == ';' {
				tok = l.NextToken()
				if tok.Type != token.SEMICOLON {
					t.Errorf("Expected semicolon after literal, got %q", tok.Type)
				}
			}
		})
	}
}
