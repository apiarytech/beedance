package lexer

import (
	"testing"

	"beedance/token"
)

func TestNumericLiterals(t *testing.T) {
	runLexerTest(t, "Integers", []testToken{
		{"123", token.INT, "123"},
		{"0", token.INT, "0"},
		{"9876543210", token.INT, "9876543210"},
	})

	runLexerTest(t, "Reals", []testToken{
		{"123.456", token.REAL, "123.456"},
		{"0.0", token.REAL, "0.0"},
		{".123", token.ILLEGAL, "."},
	})

	runLexerTest(t, "Reals with exponents", []testToken{
		{"1.23E4", token.REAL, "1.23E4"},
		{"1.23e4", token.REAL, "1.23e4"},
		{"1.23e+4", token.REAL, "1.23e+4"},
		{"1.23E-4", token.REAL, "1.23E-4"},
		{"123E-4", token.REAL, "123E-4"},
	})
}

func TestBasedLiterals(t *testing.T) {
	runLexerTest(t, "Based literals", []testToken{
		{"2#1010_1100", token.INT, "2#1010_1100"},
		{"8#377", token.INT, "8#377"},
		{"16#FF", token.INT, "16#FF"},
		{"16#ff", token.INT, "16#ff"},
		{"16#FFe10", token.INT, "16#FFe10"}, // 'e' is a valid hex digit
		{"16#A.B", token.REAL, "16#A.B"},
		{"2#1011_0010", token.INT, "2#1011_0010"},
	})
}

func TestTypedLiteralsLexing(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTokens []token.Token
	}{
		{
			"INT", "INT#10", []token.Token{
				{Type: token.INT, Literal: "INT"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "10"},
			},
		},
		{
			"DINT with base", "DINT#16#FF", []token.Token{
				{Type: token.DINT, Literal: "DINT"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "16#FF"},
			},
		},
		{
			"REAL", "REAL#1.5", []token.Token{
				{Type: token.REAL, Literal: "REAL"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.REAL, Literal: "1.5"},
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

func TestBitStringLiteralLexing(t *testing.T) {
	tests := []struct {
		input          string
		expectedTokens []token.Token
	}{
		{
			"BYTE#16#A5", []token.Token{
				{Type: token.BYTE, Literal: "BYTE"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "16#A5"},
				{Type: token.EOF, Literal: ""},
			},
		},
		{
			"WORD#2#1111_0000", []token.Token{
				{Type: token.WORD, Literal: "WORD"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "2#1111_0000"},
				{Type: token.EOF, Literal: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
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

func TestInvalidLiterals(t *testing.T) {
	runLexerTest(t, "Invalid literals", []testToken{
		{"16#FFG", token.INT, "16#FF"},          // The lexer stops at 'G', which is not a valid hex digit.
		{"INVALID#123", token.IDENT, "INVALID"}, // Lexer sees IDENT, HASH, INT. Parser handles the error.
	})
}

func TestInvalidBasedLiterals(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTokens []token.Token
	}{
		{
			"Invalid base", "12#123", []token.Token{
				{Type: token.INT, Literal: "12"},
				{Type: token.HASH, Literal: "#"},
				{Type: token.INT, Literal: "123"},
				{Type: token.EOF, Literal: ""},
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

func TestBasedLiteralsWithExponentChar(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTokens []token.Token
	}{
		{
			"Decimal based literal with 'e'", "10#123e4", []token.Token{
				{Type: token.INT, Literal: "10#123"},
				{Type: token.IDENT, Literal: "e4"},
				{Type: token.EOF, Literal: ""},
			},
		},
		{
			"Hex based literal with 'e' (valid digit)", "16#123e4", []token.Token{
				{Type: token.INT, Literal: "16#123e4"},
				{Type: token.EOF, Literal: ""},
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

func TestDirectlyRepresentedVariables(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{"%IX0.0", token.DIRECT_VAR, "%IX0.0"},
		{"%MW100", token.DIRECT_VAR, "%MW100"},
		{"%QB7", token.DIRECT_VAR, "%QB7"},
		{"%ID42", token.DIRECT_VAR, "%ID42"},
		{"%I*", token.DIRECT_VAR, "%I*"},
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
	input := "123 myVar"
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

type testToken struct {
	input           string
	expectedType    token.TokenType
	expectedLiteral string
}

func runLexerTest(t *testing.T, name string, tests []testToken) {
	t.Run(name, func(t *testing.T) {
		for i, tt := range tests {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != tt.expectedType {
				t.Errorf("test %d (%q) - tokentype wrong. expected=%q, got=%q", i, tt.input, tt.expectedType, tok.Type)
			}

			if tok.Literal != tt.expectedLiteral {
				t.Errorf("test %d (%q) - literal wrong. expected=%q, got=%q", i, tt.input, tt.expectedLiteral, tok.Literal)
			}
		}
	})
}
