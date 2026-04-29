/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package lexer

import (
	"strings"

	"beedance/token"
)

type Lexer struct {
	input    string
	position int  // current position number for col
	readPos  int  // current position number for readPos
	ch       byte // current char under examination
	line     int  // current reading col in input (after current char)
	col      int  // current col in input (points to current char)
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1}
	l.readChar()
	return l
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()
	startLine := l.line
	startCol := l.col

	switch l.ch {
	case '=':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.ARROW, Literal: literal, Row: startLine, Column: startCol}
		} else {
			tok = newToken(token.EQ, l.ch, startLine, startCol)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch, startLine, startCol)
	case '-':
		tok = newToken(token.MINUS, l.ch, startLine, startCol)
	case '/':
		if l.peekChar() == '/' {
			// This is a single-line comment, skip to the end of the line
			l.skipSingleLineComment()
			return l.NextToken()
		}
		tok = newToken(token.SLASH, l.ch, startLine, startCol)
	case '*':
		if l.peekChar() == '*' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EXPONENT, Literal: literal, Row: startLine, Column: startCol}
		} else {
			tok = newToken(token.ASTERISK, l.ch, startLine, startCol)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.LE, Literal: literal, Row: startLine, Column: startCol}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NEQ, Literal: literal, Row: startLine, Column: startCol}
		} else {
			tok = newToken(token.LT, l.ch, startLine, startCol)
		}
	case '&':
		tok = newToken(token.AMPERSAND, l.ch, startLine, startCol)
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.GE, Literal: literal, Row: startLine, Column: startCol}
		} else {
			tok = newToken(token.GT, l.ch, startLine, startCol)
		}
	case ';':
		tok = newToken(token.SEMICOLON, l.ch, startLine, startCol)
	case ':':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.ASSIGN, Literal: literal, Row: startLine, Column: startCol}
		} else {
			tok = newToken(token.COLON, l.ch, startLine, startCol)
		}
	case ',':
		tok = newToken(token.COMMA, l.ch, startLine, startCol)
	case '{':
		tok = newToken(token.LBRACE, l.ch, startLine, startCol)
	case '}':
		tok = newToken(token.RBRACE, l.ch, startLine, startCol)
	case '(':
		if l.peekChar() == '*' {
			// This is the start of a comment, skip it and get the next token
			return l.skipComment()
		}
		tok = newToken(token.LPAREN, l.ch, startLine, startCol)
	case ')':
		tok = newToken(token.RPAREN, l.ch, startLine, startCol)
	case '"':
		tok.Type = token.WSTRING_LITERAL
		tok.Literal, tok.Type = l.readString('"')
		tok.Row = startLine
		tok.Column = startCol
	case '\'':
		tok.Type = token.STRING_LITERAL
		tok.Literal, tok.Type = l.readString('\'')
		tok.Row = startLine
		tok.Column = startCol
	case '[':
		tok = newToken(token.LBRACKET, l.ch, startLine, startCol)
	case ']':
		tok = newToken(token.RBRACKET, l.ch, startLine, startCol)
	case 0:
		tok.Type = token.EOF
		tok.Literal = ""
		tok.Row = startLine
		tok.Column = startCol
	case '.':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.RANGE, Literal: literal, Row: startLine, Column: startCol}
		} else if isDigit(l.peekChar()) {
			tok = newToken(token.ILLEGAL, l.ch, startLine, startCol)
		} else {
			// A single dot is not a valid token on its own in IEC 61131-3,
			// except as a structure member accessor, which is handled by the parser.
			// We'll tokenize it as DOT and let the parser decide its validity.
			tok = newToken(token.DOT, l.ch, startLine, startCol)
		}
	case '%':
		if isLetter(l.peekChar()) {
			// This is the start of a directly represented variable (e.g., %IX1.0)
			tok.Type = token.DIRECT_VAR
			tok.Literal = l.readDirectVariable()
			return tok // readDirectVariable advances the lexer, so we return early
		}
		tok = newToken(token.ILLEGAL, l.ch, startLine, startCol)
	default:
		if isLetter(l.ch) {
			ident := l.readIdentifier()
			if l.ch == '#' {
				// This is a typed literal, like DINT#10 or T#5s
				tok = l.readTypedLiteral(ident, startLine, startCol)
			} else {
				tok.Type = token.LookupIdent(strings.ToUpper(ident))
				// After lookup, if it's still an IDENT, validate it.
				if tok.Type == token.IDENT && !isValidIdentifier(ident) {
					tok.Type = token.ILLEGAL
				}
				tok.Literal = ident
			}
			tok.Row = startLine
			tok.Column = startCol
			return tok
		} else if isDigit(l.ch) {
			literal, tokType := l.readNumber()
			tok.Literal = literal
			tok.Type = tokType
			tok.Row = startLine
			tok.Column = startCol
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch, startLine, startCol)

		}
	}
	l.readChar()
	return tok
}

func (l *Lexer) skipComment() token.Token {
	startLine := l.line
	startCol := l.col
	for !(l.ch == '*' && l.peekChar() == ')') {
		if l.ch == 0 { // Check for unterminated comment
			return token.Token{Type: token.UNTERMINATED_COMMENT, Literal: "(*", Row: startLine, Column: startCol}
		}
		l.readChar()
	}
	l.readChar() // consume '*'
	l.readChar() // consume ')'
	return l.NextToken()
}

func (l *Lexer) skipSingleLineComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0 // NUL character for EOF
	} else {
		l.ch = l.input[l.readPos]
	}
	l.position = l.readPos
	l.readPos++

	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) readIdentifier() string { // Changed to return the identifier string
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() (string, token.TokenType) {
	position := l.position // 0-based index for slicing
	tokType := token.TokenType(token.INT)

	// Read the integer part
	for isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}

	// Check for a based literal (e.g., 16#FF)
	if l.ch == '#' {
		l.readChar() // consume '#'
		// Read the integer part of the based literal's value
		for isHexDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
		// Check for a fractional part in a based literal (e.g., 16#A.B)
		if l.ch == '.' {
			tokType = token.REAL // It's a real now
			l.readChar()         // consume '.'
			for isHexDigit(l.ch) || l.ch == '_' {
				l.readChar()
			}
		}
		// Based literals do not have exponents in IEC 61131-3
		return l.input[position:l.position], tokType
	}

	// If not a based literal, check for fractional part (making it a REAL)
	// Make sure it's not the start of a '..' range token
	if l.ch == '.' && l.peekChar() != '.' {
		tokType = token.REAL
		l.readChar() // consume '.'
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	// Check for a fractional part (making it a REAL)
	// Make sure it's not the start of a '..' range token
	if l.ch == '.' && l.peekChar() != '.' {
		tokType = token.REAL
		l.readChar() // consume '.'
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	// Check for an exponent part (also making it a REAL)
	if l.ch == 'e' || l.ch == 'E' {
		tokType = token.REAL
		l.readChar() // consume 'e' or 'E'
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	return l.input[position:l.position], tokType
}

func (l *Lexer) readDirectVariable() string {
	position := l.position
	l.readChar() // consume '%'
	// Read location (I, Q, M), size (X, B, W, D, L), and address parts
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '.' || l.ch == '*' {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readTypedLiteral(typePart string, startLine int, startCol int) token.Token {
	l.readChar() // consume '#'
	valuePartStart := l.position

	// Read the rest of the literal. This can be complex for time values.
	// For now, we read until a character that cannot be part of the value.
	for isDigit(l.ch) || isLetter(l.ch) || l.ch == '.' || l.ch == '_' || l.ch == '+' || l.ch == '-' || l.ch == ':' {
		l.readChar()
	}
	valuePart := l.input[valuePartStart:l.position]

	// The whole literal is the token's literal
	literal := typePart + "#" + valuePart
	return token.Token{Type: token.LookupIdent(strings.ToUpper(typePart)), Literal: literal, Row: l.position, Column: l.col}
}

func (l *Lexer) readString(quote byte) (string, token.TokenType) {
	position := l.position + 1 // Start after the opening quote
	for {
		l.readChar()
		if l.ch == quote || l.ch == 0 {
			break
		}
	}
	if l.ch == 0 {
		return l.input[position:l.position], token.UNTERMINATED_STRING
	}
	// The literal is the content between the quotes
	if quote == '"' {
		return l.input[position:l.position], token.WSTRING_LITERAL
	} else {
		return l.input[position:l.position], token.STRING_LITERAL
	}
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// isHexDigit checks if a character is a hexadecimal digit (0-9, a-f, A-F).
func isHexDigit(ch byte) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

// isValidIdentifier checks for invalid underscore usage according to IEC 61131-3 §2.1.2
func isValidIdentifier(ident string) bool {
	if strings.HasPrefix(ident, "__") { // Multiple leading underscores
		return false
	}
	if strings.Contains(ident, "__") { // Multiple embedded underscores
		return false
	}
	if strings.HasSuffix(ident, "_") { // Trailing underscore
		return false
	}
	return true
}

func newToken(tokenType token.TokenType, ch byte, position int, col int) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch), Row: position, Column: col}
}
