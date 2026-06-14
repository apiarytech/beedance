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
		{"16#A.B", token.REAL, "16#A.B"},
		{"2#1011_0010", token.INT, "2#1011_0010"},
	})
}

func TestTypedLiteralsLexing(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedTokens   []token.Token
		expectedLiterals []string
	}{
		{"INT", "INT#10", []token.Token{{Type: token.INT}, {Type: token.HASH}, {Type: token.INT}}, []string{"INT", "#", "10"}},
		{"DINT", "DINT#123", []token.Token{{Type: token.DINT}, {Type: token.HASH}, {Type: token.INT}}, []string{"DINT", "#", "123"}},
		{"REAL", "REAL#1.5", []token.Token{{Type: token.REAL}, {Type: token.HASH}, {Type: token.REAL}}, []string{"REAL", "#", "1.5"}},
		// TIME literals
		{"TIME short form", "T#5s", []token.Token{{Type: token.TIME}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"T", "#", "5s"}},
		{"TIME long form", "TIME#5m_10s", []token.Token{{Type: token.TIME}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"TIME", "#", "5m_10s"}},
		{"TIME with milliseconds", "T#100ms", []token.Token{{Type: token.TIME}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"T", "#", "100ms"}},
		// DATE literals
		{"DATE short form", "D#2026-05-21", []token.Token{{Type: token.DATE}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"D", "#", "2026-05-21"}},
		{"DATE long form", "DATE#1999-12-31", []token.Token{{Type: token.DATE}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"DATE", "#", "1999-12-31"}},
		// TIME_OF_DAY literals
		{"TOD short form", "TOD#14:21:00.123", []token.Token{{Type: token.TIME_OF_DAY}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"TOD", "#", "14:21:00.123"}},
		{"TOD long form", "TIME_OF_DAY#23:59:59", []token.Token{{Type: token.TIME_OF_DAY}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"TIME_OF_DAY", "#", "23:59:59"}},
		// DATE_AND_TIME literals
		{"DT short form", "DT#2026-05-21-14:21:00", []token.Token{{Type: token.DATE_AND_TIME}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"DT", "#", "2026-05-21-14:21:00"}},
		{"DT long form", "DATE_AND_TIME#1984-06-25-15:36:55.36", []token.Token{{Type: token.DATE_AND_TIME}, {Type: token.HASH}, {Type: token.IDENT}}, []string{"DATE_AND_TIME", "#", "1984-06-25-15:36:55.36"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, expectedToken := range tt.expectedTokens {
				tok := l.NextToken()
				if tok.Type != expectedToken.Type {
					t.Errorf("token %d type wrong. want=%q, got=%q", i, expectedToken.Type, tok.Type)
				}
				if tok.Literal != tt.expectedLiterals[i] {
					t.Errorf("token %d literal wrong. want=%q, got=%q", i, tt.expectedLiterals[i], tok.Literal)
				}
			}
		})
	}
}

func TestInvalidLiterals(t *testing.T) {
	runLexerTest(t, "Invalid literals", []testToken{
		{"16#FFe10", token.INT, "16#FF"},        // The lexer stops at 'e', which is not a valid hex digit.
		{"INVALID#123", token.IDENT, "INVALID"}, // Lexer sees IDENT, HASH, INT. Parser handles the error.
	})
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
