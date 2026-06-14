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

// Prepend pushes a string back to the front of the input stream.
// This is useful when the parser needs to split a token.
func (l *Lexer) Prepend(s string) {
	l.input = s + l.input[l.position:]
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()
	startLine := l.line
	startCol := l.col
	startPos := l.position // Capture the absolute start position of the token
	switch l.ch {
	case '=':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.ARROW, Literal: literal, Row: startLine, Column: startCol, Pos: startPos} // Correctly identify '=>'
		} else {
			tok = newToken(token.EQ, l.ch, startLine, startCol, startPos)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NEQ, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else {
			tok = newToken(token.NOT, l.ch, startLine, startCol, startPos)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch, startLine, startCol, startPos)
	case '-':
		tok = newToken(token.MINUS, l.ch, startLine, startCol, startPos)
	case '/':
		if l.peekChar() == '/' {
			// This is a single-line comment, skip to the end of the line
			l.skipSingleLineComment()
			return l.NextToken()
		}
		tok = newToken(token.SLASH, l.ch, startLine, startCol, startPos)
	case '*':
		if l.peekChar() == '*' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EXPONENT, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else {
			tok = newToken(token.ASTERISK, l.ch, startLine, startCol, startPos)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.LE, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NEQ, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else {
			tok = newToken(token.LT, l.ch, startLine, startCol, startPos)
		}
	case '&':
		tok = newToken(token.AMPERSAND, l.ch, startLine, startCol, startPos)
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.GE, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else {
			tok = newToken(token.GT, l.ch, startLine, startCol, startPos)
		}
	case ';':
		tok = newToken(token.SEMICOLON, l.ch, startLine, startCol, startPos)
	case ':':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.ASSIGN, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else {
			tok = newToken(token.COLON, l.ch, startLine, startCol, startPos)
		}
	case ',':
		tok = newToken(token.COMMA, l.ch, startLine, startCol, startPos)
	case '{':
		tok = newToken(token.LBRACE, l.ch, startLine, startCol, startPos)
	case '}':
		tok = newToken(token.RBRACE, l.ch, startLine, startCol, startPos)
	case '(':
		if l.peekChar() == '*' {
			// This is the start of a comment, skip it and get the next token
			terminated, nested := l.skipComment()
			if !terminated {
				return token.Token{Type: token.UNTERMINATED_COMMENT, Literal: "(*", Row: startLine, Column: startCol, Pos: startPos}
			}
			if nested {
				return token.Token{Type: token.ILLEGAL, Literal: "nested comment", Row: startLine, Column: startCol, Pos: startPos}
			}
			return l.NextToken() // Get the token after the comment
		}
		tok = newToken(token.LPAREN, l.ch, startLine, startCol, startPos)
	case ')':
		tok = newToken(token.RPAREN, l.ch, startLine, startCol, startPos)
	case '"':
		tok.Type = token.WSTRING_LITERAL
		tok.Literal, tok.Type = l.readString('"')
		tok.Row = startLine
		tok.Column = startCol
		tok.Pos = startPos
	case '\'':
		tok.Type = token.STRING_LITERAL
		tok.Literal, tok.Type = l.readString('\'')
		tok.Row = startLine
		tok.Column = startCol
		tok.Pos = startPos
	case '[':
		tok = newToken(token.LBRACKET, l.ch, startLine, startCol, startPos)
	case ']':
		tok = newToken(token.RBRACKET, l.ch, startLine, startCol, startPos)
	case 0:
		tok.Type = token.EOF
		tok.Literal = ""
		tok.Row = startLine
		tok.Column = startCol
		tok.Pos = startPos
	case '.':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.RANGE, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else if isDigit(l.peekChar()) {
			tok = newToken(token.ILLEGAL, l.ch, startLine, startCol, startPos)
		} else {
			// A single dot is not a valid token on its own in IEC 61131-3,
			// except as a structure member accessor, which is handled by the parser.
			// We'll tokenize it as DOT and let the parser decide its validity.
			tok = newToken(token.DOT, l.ch, startLine, startCol, startPos)
		}
	case '#':
		tok = newToken(token.HASH, l.ch, startLine, startCol, startPos)
	case '%':
		if isLetter(l.peekChar()) {
			// This is the start of a directly represented variable (e.g., %IX1.0)
			tok.Type = token.DIRECT_VAR
			tok.Literal = l.readDirectVariable()
			return tok // readDirectVariable advances the lexer, so we return early
		}
		tok = newToken(token.ILLEGAL, l.ch, startLine, startCol, startPos)
	default:
		if isLetter(l.ch) {
			ident := l.readIdentifier()
			tok.Literal = ident
			tok.Row = startLine
			tok.Column = startCol
			tok.Pos = startPos

			// Check if the identifier is a potential time/date keyword.
			// This logic is now simplified. The parser will handle `TYPE#value`.
			// The lexer just needs to tokenize `DATE`, `#`, and the value separately.
			tok.Type = token.LookupIdent(ident)

			// Special handling for short-form date/time keywords (D, T, TOD, DT).
			// They should only be treated as keywords if followed by a '#'.
			// Otherwise, they are just regular identifiers.
			if isShortTimeDateKeyword(ident) {
				tempPos := l.position
				tempReadPos := l.readPos
				tempCh := l.ch
				l.skipWhitespace()
				isFollowedByHash := l.ch == '#'
				l.position = tempPos
				l.readPos = tempReadPos
				l.ch = tempCh

				if !isFollowedByHash {
					// It's not followed by '#', so treat it as a regular identifier.
					tok.Type = token.IDENT
				}
			}

			return tok
		} else if isDigit(l.ch) {
			tok.Literal, tok.Type = l.readNumber()
			tok.Row = startLine
			tok.Column = startCol
			tok.Pos = startPos
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch, startLine, startCol, startPos)
		}
	}
	l.readChar()
	return tok
}

// skipComment scans through the input until it finds the comment termination characters '*)'.
func (l *Lexer) skipComment() (terminated bool, nested bool) {
	isNested := false
	l.readChar() // consume '('
	l.readChar() // consume '*'

	for l.ch != 0 {
		// Check for nested comment start
		if l.ch == '(' && l.peekChar() == '*' {
			isNested = true
			// We've found a nested comment, but we continue scanning for the end
			// to allow the lexer to find its place. The caller will report the error.
		}

		if l.ch == '*' && l.peekChar() == ')' {
			l.readChar()
			l.readChar()
			return true, isNested // Terminated successfully. Report if nesting was found.
		}
		l.readChar()
	}

	// If we reach here, it means l.ch is 0 (EOF) but we haven't found '*)'
	return false, isNested
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
	// Per IEC 61131-3 §2.1.2, an identifier is a string of letters, digits, and underscores.
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() (string, token.TokenType) {
	position := l.position // 0-based index for slicing
	tokType := token.TokenType(token.INT)

	// Read the integer part, allowing for underscores
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
		for (digitCheckFn(l.ch) || l.ch == '_') && (l.ch != 'e' && l.ch != 'E') {
			l.readChar()
		}

		// Check for a fractional part in a based literal (e.g., 16#A.B)
		if l.ch == '.' {
			// This is a non-standard extension for based real literals.
			tokType = token.REAL
			l.readChar() // consume '.'
			// Continue reading the fractional part based on the same base.
			for digitCheckFn(l.ch) || l.ch == '_' {
				l.readChar()
			}
			return l.input[position:l.position], tokType
		}

		// After parsing a based literal, we should not look for exponents. Return immediately.
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

	// Check for an exponent part (also making it a REAL)
	if l.ch == 'e' || l.ch == 'E' {
		tokType = token.REAL // cspell:disable-line
		l.readChar()         // consume 'e' or 'E'
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	// After a number, if we see a letter, it might be a time unit (e.g., 5s, 10ms).
	// We consume the rest of what looks like a duration string.
	if isLetter(l.ch) || l.ch == ':' || (l.ch == '-' && isDigit(l.peekChar())) {
		tokType = token.IDENT // It's no longer just a number, but part of a time/date/duration identifier
		for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '.' || l.ch == '-' || l.ch == ':' {
			l.readChar()
		}
	}

	return l.input[position:l.position], tokType
}

// readTimeLiteralValue consumes the value part of a time/date literal.
// This is a special case because these literals can contain '-' and ':'
// which would normally be treated as separate tokens.
func (l *Lexer) readTimeLiteralValue() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '.' || l.ch == '-' || l.ch == ':' {
		l.readChar()
	}
	return l.input[position:l.position]
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

func newToken(tokenType token.TokenType, ch byte, row int, col int, pos int) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch), Row: row, Column: col, Pos: pos}
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

// isTimeDateKeyword checks if an identifier is a time/date keyword or abbreviation.
// It returns the corresponding token type and a boolean indicating if it's a match.
func isTimeDateKeyword(ident string) (token.TokenType, bool) {
	upper := strings.ToUpper(ident)
	switch upper {
	case "TIME", "T":
		return token.TIME, true
	case "DATE", "D":
		return token.DATE, true
	case "TIME_OF_DAY", "TOD":
		return token.TIME_OF_DAY, true
	case "DATE_AND_TIME", "DT":
		return token.DATE_AND_TIME, true
	}
	return token.ILLEGAL, false
}

// isShortTimeDateKeyword checks if an identifier is one of the short-form
// keywords that require special lookahead handling.
func isShortTimeDateKeyword(ident string) bool {
	upper := strings.ToUpper(ident)
	switch upper {
	case "T", "D", "TOD", "DT":
		return true
	default:
		return false
	}
}
