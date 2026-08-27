package lexer

import (
	"testing"

	"beedance/token"
)

func TestComplexToken(t *testing.T) {
	input := `
VAR
	myVar : INT := 5; (* A variable declaration *)
	anotherVar : BOOL := TRUE;
END_VAR

IF myVar >= 5.1_000 AND myVar <= 10 THEN
    anotherVar := FALSE;
ELSE
    myVar := (myVar + 1) * 2 / 2 - 1 ** 2;
END_IF

myVar <> 10;
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
		expectedRow     int
		expectedColumn  int
	}{
		{token.VAR, "VAR", 2, 1},
		{token.IDENT, "myVar", 3, 2},
		{token.COLON, ":", 3, 8},
		{token.INT, "INT", 3, 10},
		{token.ASSIGN, ":=", 3, 14},
		{token.INT, "5", 3, 17},
		{token.SEMICOLON, ";", 3, 18},
		{token.COMMENT, " A variable declaration ", 3, 20},
		{token.IDENT, "anotherVar", 4, 2},
		{token.COLON, ":", 4, 13},
		{token.BOOL, "BOOL", 4, 15},
		{token.ASSIGN, ":=", 4, 20},
		{token.TRUE, "TRUE", 4, 23},
		{token.SEMICOLON, ";", 4, 27},
		{token.END_VAR, "END_VAR", 5, 1},
		{token.IF, "IF", 7, 1},
		{token.IDENT, "myVar", 7, 4},
		{token.GE, ">=", 7, 10},        // >=
		{token.REAL, "5.1_000", 7, 13}, // REAL
		{token.AND, "AND", 7, 21},
		{token.IDENT, "myVar", 7, 25},
		{token.LE, "<=", 7, 31}, // <=
		{token.INT, "10", 7, 34},
		{token.THEN, "THEN", 7, 37},
		{token.IDENT, "anotherVar", 8, 5},
		{token.ASSIGN, ":=", 8, 16},
		{token.FALSE, "FALSE", 8, 19},
		{token.SEMICOLON, ";", 8, 24},
		{token.ELSE, "ELSE", 9, 1},
		{token.IDENT, "myVar", 10, 5},  // myVar
		{token.ASSIGN, ":=", 10, 11},   // :=
		{token.LPAREN, "(", 10, 14},    // (
		{token.IDENT, "myVar", 10, 15}, // myVar
		{token.PLUS, "+", 10, 21},      // +
		{token.INT, "1", 10, 23},       // 1
		{token.RPAREN, ")", 10, 24},    // )
		{token.ASTERISK, "*", 10, 26},  // *
		{token.INT, "2", 10, 28},       // 2
		{token.SLASH, "/", 10, 30},     // /
		{token.INT, "2", 10, 32},       // 2
		{token.MINUS, "-", 10, 34},     // -
		{token.INT, "1", 10, 36},       // 1
		{token.EXPONENT, "**", 10, 38}, // **
		{token.INT, "2", 10, 41},       // 2
		{token.SEMICOLON, ";", 10, 42}, // ;
		{token.END_IF, "END_IF", 11, 1},
		{token.IDENT, "myVar", 13, 1},
		{token.NEQ, "<>", 13, 7},
		{token.INT, "10", 13, 10},
		{token.SEMICOLON, ";", 13, 12},
		{token.EOF, "", 14, 1},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q (row %d, column %d)",
				i, tt.expectedType, tok.Type, tok.Row, tok.Column)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q (row %d, column %d)",
				i, tt.expectedLiteral, tok.Literal, tok.Row, tok.Column)
		}

		if tok.Row != tt.expectedRow {
			t.Fatalf("tests[%d] - row wrong. expected=%d, got=%d for %q",
				i, tt.expectedRow, tok.Row, tok.Literal)
		}

		if tok.Column != tt.expectedColumn {
			t.Fatalf("tests[%d] - column wrong. expected=%d, got=%d for %q",
				i, tt.expectedColumn, tok.Column, tok.Literal)
		}
	}
}

func TestSingleLineComments(t *testing.T) {
	input := `
		VAR // This is a variable block
			myVar : INT; // This is a variable declaration
		END_VAR
		// This is a full line comment
		myVar := 5; // Trailing comment
	`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.VAR, "VAR"},
		{token.COMMENT, " This is a variable block"},
		{token.IDENT, "myVar"},
		{token.COLON, ":"},
		{token.INT, "INT"},
		{token.SEMICOLON, ";"},
		{token.COMMENT, " This is a variable declaration"},
		{token.END_VAR, "END_VAR"},
		{token.COMMENT, " This is a full line comment"},
		{token.IDENT, "myVar"},
		{token.ASSIGN, ":="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.COMMENT, " Trailing comment"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestEnumeratedValueLexing(t *testing.T) {
	input := `COLOR#RED`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "COLOR"},
		{token.HASH, "#"},
		{token.IDENT, "RED"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken(t *testing.T) {
	input := `=+(){},;[]<,>:**`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.EQ, "="},
		{token.PLUS, "+"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RBRACE, "}"},
		{token.COMMA, ","},
		{token.SEMICOLON, ";"},
		{token.LBRACKET, "["},
		{token.RBRACKET, "]"},
		{token.LT, "<"},
		{token.COMMA, ","},
		{token.GT, ">"},
		{token.COLON, ":"},
		{token.EXPONENT, "**"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
