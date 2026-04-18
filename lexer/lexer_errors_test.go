package lexer

import (
	"testing"

	"beedance/token"
)

func TestErrorLexing(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.TokenType
		expectedLiteral string
		expectedRow     int
		expectedColumn  int
	}{
		{"'unterminated", token.UNTERMINATED_STRING, "unterminated", 1, 1},
		{"\"another one", token.UNTERMINATED_STRING, "another one", 1, 1},
		{"(* comment that never ends", token.UNTERMINATED_COMMENT, "(*", 1, 1},
		{"VAR myVar : INT := ?;", token.ILLEGAL, "?", 1, 20},
		{"!", token.ILLEGAL, "!", 1, 1},
		{"@", token.ILLEGAL, "@", 1, 1},
		{"$", token.ILLEGAL, "$", 1, 1},
		{"%", token.ILLEGAL, "%", 1, 1},
		{"^", token.ILLEGAL, "^", 1, 1},
		{"&", token.ILLEGAL, "&", 1, 1},
		{"|", token.ILLEGAL, "|", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			foundError := false
			for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
				if tok.Type == tt.expectedType {
					foundError = true
					if tok.Literal != tt.expectedLiteral {
						t.Errorf("literal wrong. expected=%q, got=%q", tt.expectedLiteral, tok.Literal)
					}
					if tok.Row != tt.expectedRow {
						t.Errorf("row wrong. expected=%d, got=%d", tt.expectedRow, tok.Row)
					}
					if tok.Column != tt.expectedColumn {
						t.Errorf("column wrong. expected=%d, got=%d", tt.expectedColumn, tok.Column)
					}
					break // Found the token we were looking for
				}
			}

			if !foundError {
				t.Errorf("did not find expected error token %q", tt.expectedType)
			}
		})
	}
}
