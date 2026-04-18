package lexer

import (
	"testing"

	"beedance/token"
)

func TestNumericLiterals(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.TokenType
		expectedLiteral string
	}{
		// Integers
		{"123", token.INT, "123"},
		{"0", token.INT, "0"},
		{"9876543210", token.INT, "9876543210"},

		// Reals
		{"123.456", token.REAL, "123.456"},
		{"0.0", token.REAL, "0.0"},
		{".123", token.ILLEGAL, "."}, // Assuming numbers must start with a digit

		// Reals with exponents
		{"1.23E4", token.REAL, "1.23E4"},
		{"1.23e4", token.REAL, "1.23e4"},
		{"1.23e+4", token.REAL, "1.23e+4"},
		{"1.23E-4", token.REAL, "1.23E-4"},
		{"123E-4", token.REAL, "123E-4"},

		// Based literals
		{"2#1010_1100", token.INT, "2#1010_1100"},
		{"8#377", token.INT, "8#377"},
		{"16#FF", token.INT, "16#FF"},
		{"16#ff", token.INT, "16#ff"},

		// Typed Literals
		{"INT#10", token.INT, "INT#10"},
		{"DINT#123", token.DINT, "DINT#123"},
		{"REAL#1.5", token.REAL, "REAL#1.5"},
		{"TIME#5s", token.TIME, "TIME#5s"},
		{"T#5s", token.TIME, "T#5s"},               // Short form for TIME
		{"T#5m_10s", token.TIME, "T#5m_10s"},       // With underscore
		{"TIME#1h_30m", token.TIME, "TIME#1h_30m"}, // Long form with underscore
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

func TestNumberWithFollowingIdentifier(t *testing.T) {
	input := "123myVar"
	l := New(input)

	tok := l.NextToken()
	if tok.Type != token.INT || tok.Literal != "123" {
		t.Fatalf("Expected INT 123, got %s %s", tok.Type, tok.Literal)
	}

	tok = l.NextToken()
	if tok.Type != token.IDENT || tok.Literal != "myVar" {
		t.Fatalf("Expected IDENT myVar, got %s %s", tok.Type, tok.Literal)
	}
}
