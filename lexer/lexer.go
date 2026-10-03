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

	"github.com/apiarytech/beedance/token"
)

// Lexer holds the state of the lexical analysis process, including the input string,
// current position, and line/column tracking for error reporting.
type Lexer struct {
	input    string
	position int  // current position number for col
	readPos  int  // current position number for readPos
	ch       byte // current char under examination
	line     int  // current reading col in input (after current char)
	col      int  // current col in input (points to current char)
}

// New creates and initializes a new Lexer with the given input string.
func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1}
	l.readChar()
	return l
}

// Prepend pushes a string back to the front of the input stream by modifying the
// underlying input string and adjusting the current position. This is a utility
// that could be used by the parser if it needs to re-process a token differently.
func (l *Lexer) Prepend(s string) {
	l.input = s + l.input[l.position:]
}

// NextToken reads the input string and returns the next token it finds. It is the
// central function of the lexer, responsible for tokenizing the source code.
func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()
	startLine := l.line
	startCol := l.col
	startPos := l.position // Capture the absolute start position of the token
	// This switch statement is the core of the lexer, dispatching to different
	// handlers based on the current character being examined.
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
			tok = token.Token{Type: token.NE, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
		} else {
			tok = newToken(token.NOT, l.ch, startLine, startCol, startPos)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch, startLine, startCol, startPos)
	case '-':
		tok = newToken(token.MINUS, l.ch, startLine, startCol, startPos)
	case '/':
		if l.peekChar() == '/' {
			tok.Type = token.COMMENT
			tok.Literal = l.readSingleLineComment()
			tok.Row = startLine
			tok.Column = startCol
			tok.Pos = startPos
			return tok
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
			tok = token.Token{Type: token.NE, Literal: literal, Row: startLine, Column: startCol, Pos: startPos}
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
			comment, terminated := l.readBlockComment()
			if !terminated {
				return token.Token{Type: token.UNTERMINATED_COMMENT, Literal: "(*", Row: startLine, Column: startCol, Pos: startPos}
			}
			tok.Type = token.COMMENT
			tok.Literal = comment
			tok.Row = startLine
			tok.Column = startCol
			tok.Pos = startPos
			return tok
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
	case '^':
		tok = newToken(token.CARET, l.ch, startLine, startCol, startPos)
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
		} else if isDigit(l.peekChar()) && !l.followsOperand() {
			// A real literal cannot start with a dot, e.g. .5. After a name,
			// ] or ), the dot is a bit access such as flags.3.
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
			// HACK: Special case for the 'fn' keyword from Monkey language tests.
			// We must ensure it is tokenized as an IDENT so the parser can treat
			// it as the start of a FunctionLiteral expression.
			if strings.ToLower(ident) == "fn" {
				tok.Type = token.IDENT
			} else {
				tok.Type = token.LookupIdent(ident)
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

// readBlockComment consumes a block comment, which starts with `(*` and ends
// with the matching `*)`. Comments nest, as IEC 61131-3 allows and CODESYS
// does: `(* outer (* inner *) still outer *)` is one comment. It returns the
// comment's content and whether it was terminated.
func (l *Lexer) readBlockComment() (string, bool) {
	l.readChar() // consume '('
	l.readChar() // consume '*'
	position := l.position
	depth := 1

	for l.ch != 0 {
		switch {
		case l.ch == '(' && l.peekChar() == '*':
			depth++
			l.readChar()
		case l.ch == '*' && l.peekChar() == ')':
			depth--
			if depth == 0 {
				comment := l.input[position:l.position]
				l.readChar()
				l.readChar()
				return comment, true
			}
			l.readChar()
		}
		l.readChar()
	}

	// The input ended before the comment did.
	return l.input[position:l.position], false
}

// readSingleLineComment consumes a single-line comment, which starts with `//` and ends at the newline.
func (l *Lexer) readSingleLineComment() string {
	l.readChar() // consume first /
	l.readChar() // consume second /
	position := l.position
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	return l.input[position:l.position]
}

// skipWhitespace consumes a sequence of whitespace characters (space, tab, newline, carriage return).
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// readChar advances the lexer's position in the input string by one character,
// updating the current character `l.ch` and the line/column counters.
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

// peekChar returns the next character in the input string without consuming it.
func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// readIdentifier consumes a sequence of letters, digits, and underscores to form an identifier.
func (l *Lexer) readIdentifier() string { // Changed to return the identifier string
	position := l.position
	// Per IEC 61131-3 §2.1.2, an identifier is a string of letters, digits, and underscores.
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber consumes a numeric literal, which can be an integer or a real number,
// and can include based notation (e.g., 16#FF) or an exponent.
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
		for digitCheckFn(l.ch) || l.ch == '_' {
			if base != 16 && (l.ch == 'e' || l.ch == 'E') {
				break
			}
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

	// After parsing a number, any following letter is part of the next token.
	// This correctly handles cases like `5s` (number, identifier) and `5z`.
	return l.input[position:l.position], tokType
}

// readDirectVariable consumes a directly represented variable, like `%IX0.0` or `%MW100`.
func (l *Lexer) readDirectVariable() string {
	position := l.position
	l.readChar() // consume '%'
	// Read location (I, Q, M), size (X, B, W, D, L), and address parts
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '.' || l.ch == '*' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readString consumes a string literal enclosed in either single or double quotes.
// It returns the content of the string and the appropriate token type.
func (l *Lexer) readString(quote byte) (string, token.TokenType) {
	position := l.position + 1 // Start after the opening quote
	for {
		l.readChar()
		// A $ escape, such as $' or $$, is kept as written; its second
		// character does not end the string.
		if l.ch == '$' {
			l.readChar()
			if l.ch == 0 {
				break
			}
			continue
		}
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

// isLetter checks if a character is a letter (a-z, A-Z) or an underscore.
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// isDigit checks if a character is a decimal digit (0-9).
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// isHexDigit checks if a character is a valid hexadecimal digit (0-9, a-f, A-F).
func isHexDigit(ch byte) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

// isOctalDigit checks if a character is a valid octal digit (0-7).
func isOctalDigit(ch byte) bool {
	return '0' <= ch && ch <= '7'
}

// isValidIdentifier checks for invalid underscore usage in an identifier according
// to the IEC 61131-3 standard (§2.1.2).
func isValidIdentifier(ident string) bool {
	// An identifier cannot contain consecutive underscores or end with an underscore.
	return !strings.Contains(ident, "__") && !strings.HasSuffix(ident, "_") && !strings.HasPrefix(ident, "__")
}

// newToken is a helper function to create a new token with the given type,
// literal, and position information.
func newToken(tokenType token.TokenType, ch byte, row int, col int, pos int) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch), Row: row, Column: col, Pos: pos}
}

// getDigitCheckFn returns a validation function appropriate for the given numeric
// base (2, 8, 10, or 16). This is used when parsing based numeric literals.
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

// isTypedLiteralPrefix checks if an identifier is a keyword that can legally
// prefix a typed literal (e.g., `INT`, `TIME`, `BYTE`).
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

// followsOperand reports whether the character before the current one ends
// an operand: a letter, digit or underscore of a name, or ] or ).
func (l *Lexer) followsOperand() bool {
	if l.position == 0 {
		return false
	}
	prev := l.input[l.position-1]
	return isLetter(prev) || isDigit(prev) || prev == ']' || prev == ')'
}
