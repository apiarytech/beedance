package lexer

import (
	"testing"

	"beedance/token"
)

func TestPrepend(t *testing.T) {
	l := New("world")
	l.Prepend("hello ")

	expected := "hello world"
	if l.input != expected {
		t.Fatalf("input wrong. expected=%q, got=%q", expected, l.input)
	}

	tok := l.NextToken()
	if tok.Type != token.IDENT || tok.Literal != "hello" {
		t.Fatalf("first token wrong. expected=IDENT 'hello', got=%s %q", tok.Type, tok.Literal)
	}
}

func TestMiscellaneousAndEdgeCases(t *testing.T) {
	input := `=> != ! .. . fn 1..10`
	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.ARROW, "=>"},
		{token.NE, "!="},
		{token.NOT, "!"},
		{token.RANGE, ".."},
		{token.DOT, "."},
		{token.IDENT, "fn"}, // Special case for 'fn' keyword
		{token.INT, "1"},
		{token.RANGE, ".."},
		{token.INT, "10"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Errorf("tests[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Errorf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestIsTypedLiteralPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Valid Time/Date prefixes
		{"TIME", true},
		{"time", true},
		{"T", true},
		{"DATE", true},
		{"D", true},
		{"TIME_OF_DAY", true},
		{"TOD", true},
		{"DATE_AND_TIME", true},
		{"DT", true},

		// Valid Integer prefixes
		{"SINT", true},
		{"INT", true},
		{"DINT", true},
		{"LINT", true},
		{"USINT", true},
		{"UINT", true},
		{"UDINT", true},
		{"ULINT", true},

		// Valid Real prefixes
		{"REAL", true},
		{"LREAL", true},

		// Valid Bit-string prefixes
		{"BYTE", true},
		{"WORD", true},
		{"DWORD", true},
		{"LWORD", true},

		// Invalid prefixes (other keywords or identifiers)
		{"VAR", false},
		{"PROGRAM", false},
		{"MyIdentifier", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isTypedLiteralPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("isTypedLiteralPrefix(%q) wrong. want=%t, got=%t", tt.input, tt.expected, result)
			}
		})
	}
}

func TestGetDigitCheckFn(t *testing.T) {
	tests := []struct {
		name     string
		base     int
		char     byte
		expected bool
	}{
		// Case 10: Decimal
		{"Base 10 - Lower bound", 10, '0', true},
		{"Base 10 - Mid range", 10, '5', true},
		{"Base 10 - Upper bound", 10, '9', true},
		{"Base 10 - Invalid char 'a'", 10, 'a', false},

		// Default case: Invalid base
		{"Default case (invalid base 12) - Digit", 12, '1', false},
		{"Default case (invalid base 0) - Zero", 0, '0', false},
		{"Default case (invalid base -1) - One", -1, '1', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkFn := getDigitCheckFn(tt.base)
			result := checkFn(tt.char)
			if result != tt.expected {
				t.Errorf("getDigitCheckFn(%d) for char %q wrong. want=%t, got=%t", tt.base, tt.char, tt.expected, result)
			}
		})
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid identifier", "My_Var", true},
		{"Valid with leading underscore", "_MyVar", true},
		{"Valid with digits", "Var123", true},
		{"Invalid with trailing underscore", "MyVar_", false},
		{"Invalid with double leading underscore", "__MyVar", false},
		{"Invalid with internal double underscore", "My__Var", false},
		{"Invalid with only an underscore", "_", false},
		{"Invalid with only double underscore", "__", false},
		{"Empty string", "", true}, // The function currently allows this
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("isValidIdentifier(%q) wrong. want=%t, got=%t", tt.input, tt.expected, result)
			}
		})
	}
}
