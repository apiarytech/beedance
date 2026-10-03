package lexer

import (
	"testing"

	"github.com/apiarytech/beedance/token"
)

func TestStringLiterals(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.TokenType
		expectedLiteral string
	}{
		// Single-quoted strings
		{"d", token.DATE, "d"},
		{"t", token.TIME, "t"},
		{"tod", token.TIME_OF_DAY, "tod"},
		{"dt", token.DATE_AND_TIME, "dt"},
		{"'hello world'", token.STRING_LITERAL, "hello world"},
		{"''", token.STRING_LITERAL, ""},
		{"'a'", token.STRING_LITERAL, "a"},
		{"'123'", token.STRING_LITERAL, "123"},
		{"'$$1.00'", token.STRING_LITERAL, "$$1.00"},
		// A $ escape is kept as written; an escaped quote does not end the string.
		{"'it$'s'", token.STRING_LITERAL, "it$'s"},
		{"\"say $\"hi$\"\"", token.WSTRING_LITERAL, "say $\"hi$\""},
		{"'unterminated $", token.UNTERMINATED_STRING, "unterminated $"},
		{"'\"'", token.STRING_LITERAL, "\""}, // single-quoted string containing a double quote

		// Double-quoted strings
		{"\"hello world\"", token.WSTRING_LITERAL, "hello world"},
		{"\"\"", token.WSTRING_LITERAL, ""},
		{"\"a\"", token.WSTRING_LITERAL, "a"},
		{"\"123\"", token.WSTRING_LITERAL, "123"},
		{"\"'\"", token.WSTRING_LITERAL, "'"}, // double-quoted string containing a single quote

		// Double-quoted strings with runes
		{"\"你好, 世界\"", token.WSTRING_LITERAL, "你好, 世界"},           // Chinese
		{"\"Привет, мир\"", token.WSTRING_LITERAL, "Привет, мир"}, // Russian
		{"\"こんにちは世界\"", token.WSTRING_LITERAL, "こんにちは世界"},         // Japanese
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Errorf("test for %q - tokentype wrong. expected=%q, got=%q",
				tt.input, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Errorf("test for %q - literal wrong. expected=%q, got=%q",
				tt.input, tt.expectedLiteral, tok.Literal)
		}
	}
}
