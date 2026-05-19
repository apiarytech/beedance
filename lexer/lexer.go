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
	"strconv"
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
			tok = token.Token{Type: token.ARROW, Literal: literal, Row: startLine, Column: startCol} // Correctly identify '=>'
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
			terminated, nested := l.skipComment()
			if !terminated {
				return token.Token{Type: token.UNTERMINATED_COMMENT, Literal: "(*", Row: startLine, Column: startCol}
			}
			if nested {
				tok = token.Token{Type: token.ILLEGAL, Literal: "nested comment", Row: startLine, Column: startCol}
				l.readChar() // Consume the illegal token to allow parser to continue
				return tok
			}
			return l.NextToken() // Get the token after the comment
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
			tok.Literal = ident
			tok.Row = startLine
			tok.Column = startCol

			// Check for typed literals (e.g., "INT#10", "D#2026-01-01").
			// This is the only context where single-letter identifiers like D, T, DT, TOD
			// should be treated as keywords.
			if l.ch == '#' {
				// It's a typed literal like D#..., INT#..., etc.
				tok = l.readTypedLiteral(ident, startLine, startCol) // This sets the correct token type (e.g., DATE, INT)
			} else {
				// Not followed by '#'. Check if it's a date/time abbreviation that should be an IDENT.
				if isTimeDateAbbreviation(ident) {
					tok.Type = token.IDENT
				} else {
					// It's a regular keyword (VAR, IF) or an identifier.
					tok.Type = token.LookupIdent(ident)
				}
			}
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

func (l *Lexer) skipComment() (terminated bool, nested bool) {
	nestingLevel := 1
	isNested := false
	l.readChar() // consume '('
	l.readChar() // consume '*'

	for nestingLevel > 0 && l.ch != 0 {
		if l.ch == '(' && l.peekChar() == '*' {
			l.readChar()
			l.readChar()
			isNested = true
			nestingLevel++
			continue
		}

		if l.ch == '*' && l.peekChar() == ')' {
			l.readChar()
			l.readChar()
			nestingLevel--
			continue
		}
		l.readChar()
	}

	if nestingLevel > 0 {
		return false, isNested
	}

	return true, isNested
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

	// Check for a based literal (e.g., 16#FF, 8#77, 2#1010)
	if l.ch == '#' {
		baseStr := l.input[position:l.position]
		base, err := strconv.Atoi(baseStr)
		if err != nil || (base != 2 && base != 8 && base != 10 && base != 16) {
			// If the part before # is not a valid base, it's not a valid based literal.
			// We'll let the parser handle the error. For now, we just return the number part.
			return baseStr, tokType
		}

		l.readChar() // consume '#'

		// Read the value part based on the detected base
		digitCheckFn := getDigitCheckFn(base)
		for digitCheckFn(l.ch) || l.ch == '_' {
			l.readChar()
		}

		// Check for a fractional part in a based literal (e.g., 16#A.B)
		if l.ch == '.' {
			// According to IEC 61131-3, based literals are only for integers (bit strings).
			// However, some extensions might support this. We'll flag it as illegal for now.
			// To support it, we would change tokType to REAL and continue parsing.
			// For now, we stop here and let the parser report an error on the '.'
			return l.input[position:l.position], token.ILLEGAL
		}

		// Check for an exponent part, which is not allowed for based literals
		if l.ch == 'e' || l.ch == 'E' {
			l.readChar()
			for isDigit(l.ch) || l.ch == '+' || l.ch == '-' || l.ch == '_' {
				l.readChar()
			}
			return l.input[position:l.position], token.ILLEGAL
		}
		return l.input[position:l.position], token.INT
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
	startPos := l.position - len(typePart) // Mark the start of the entire literal
	l.readChar()                           // consume '#'

	// Delegate to a specific reader based on the type part.
	// This makes the lexer more robust and compliant with IEC 61131-3 literal formats.
	typeKeyword := token.LookupIdent(strings.ToUpper(typePart))
	switch typeKeyword {
	case token.BYTE, token.WORD, token.DWORD, token.LWORD:
		l.readBasedIntegerPart()
	case token.SINT, token.INT, token.DINT, token.LINT, token.USINT, token.UINT, token.UDINT, token.ULINT:
		l.readIntegerPart()
	case token.REAL, token.LREAL:
		l.readRealPart()
	case token.TIME, token.DATE, token.TIME_OF_DAY, token.DATE_AND_TIME:
		l.readTimeDatePart(typeKeyword) // Pass typeKeyword for specific validation
	default:
		// If the type is not a known keyword for typed literals, it's an error.
		return token.Token{Type: token.ILLEGAL, Literal: typePart, Row: startLine, Column: startCol}
	}

	literal := l.input[startPos:l.position]
	return token.Token{Type: typeKeyword, Literal: literal, Row: startLine, Column: startCol}
}

// readBasedIntegerPart reads the value part of a bit-string literal (e.g., 16#FF_AB).
func (l *Lexer) readBasedIntegerPart() {
	// Optional base (e.g., 2, 8, 16)
	if isDigit(l.ch) {
		for isDigit(l.ch) {
			l.readChar()
		}
		if l.ch == '#' {
			l.readChar() // consume '#'
		}
	}
	// Value part (hex digits for bit-strings)
	for isHexDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
}

// readIntegerPart reads a standard integer value part.
func (l *Lexer) readIntegerPart() {
	if l.ch == '+' || l.ch == '-' {
		l.readChar()
	}
	for isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
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

// readRealPart reads the value part of a REAL or LREAL literal.
func (l *Lexer) readRealPart() {
	// This logic is similar to readNumber but simplified for the value part of a typed literal.
	if l.ch == '+' || l.ch == '-' {
		l.readChar()
	}
	for isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	if l.ch == '.' {
		l.readChar() // consume '.'
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}
	// Check for an exponent part (e.g., E+4, e-2)
	if l.ch == 'e' || l.ch == 'E' {
		l.readChar() // consume 'e' or 'E'
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}
}

// readTimeDatePart reads the value part of time and date related literals.
func (l *Lexer) readTimeDatePart(tokType token.TokenType) {
	// This function should be more specific based on tokType for strict compliance.
	// For now, it's a general reader for time/date components.
	// Full IEC 61131-3 validation of the *format* (e.g., T#5s, DATE#1990-01-01)
	// is complex and might be better handled in the parser or a dedicated validator.
	// Here, we ensure we capture all characters that *could* be part of a valid time/date literal.
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '-' || l.ch == '_' || l.ch == '.' || l.ch == ':' || l.ch == '#' {
		l.readChar()
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

// isOctalDigit checks if a character is an octal digit (0-7).
func isOctalDigit(ch byte) bool {
	return '0' <= ch && ch <= '7'
}

// isValidIdentifier checks for invalid underscore usage according to IEC 61131-3 §2.1.2
func isValidIdentifier(ident string) bool {
	// An identifier cannot contain consecutive underscores or end with an underscore.
	return !strings.Contains(ident, "__") && !strings.HasSuffix(ident, "_") && !strings.HasPrefix(ident, "__")
}

func newToken(tokenType token.TokenType, ch byte, position int, col int) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch), Row: position, Column: col}
}

// getDigitCheckFn returns a function to validate digits for a given base.
func getDigitCheckFn(base int) func(byte) bool {
	switch base {
	case 2:
		return func(ch byte) bool { return ch == '0' || ch == '1' }
	case 8:
		return isOctalDigit
	case 10:
		return isDigit
	case 16:
		return isHexDigit
	default:
		return func(ch byte) bool { return false } // Should not happen with pre-validation
	}
}

// isTypedLiteralPrefix checks if an identifier is a keyword that can prefix a typed literal.
func isTypedLiteralPrefix(ident string) bool {
	// Check against both the full keyword and its abbreviation
	switch token.LookupIdent(strings.ToUpper(ident)) {
	case token.TIME, token.DATE, token.TIME_OF_DAY, token.DATE_AND_TIME,
		token.SINT, token.INT, token.DINT, token.LINT,
		token.USINT, token.UINT, token.UDINT, token.ULINT,
		token.REAL, token.LREAL,
		token.BYTE, token.WORD, token.DWORD, token.LWORD:
		return true
	default:
		return false
	}
}

// isTimeDateAbbreviation checks if an identifier is one of the special
// single- or two-letter abbreviations for date/time types.
func isTimeDateAbbreviation(ident string) bool {
	upper := strings.ToUpper(ident)
	switch upper {
	case "D", "T", "DT", "TOD":
		return true
	}
	return false
}
