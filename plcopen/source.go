/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

// This file finds the text of each POU's body in its source, so that the
// exported XML holds the code as written, with its layout and comments,
// rather than the AST's String form, which parenthesizes every expression.

import (
	"strings"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/token"
)

// varBlockStarts are the tokens that open a variable block.
var varBlockStarts = map[token.TokenType]bool{
	token.VAR: true, token.VAR_INPUT: true, token.VAR_OUTPUT: true, token.VAR_IN_OUT: true,
	token.VAR_EXTERNAL: true, token.VAR_GLOBAL: true, token.VAR_ACCESS: true, token.VAR_TEMP: true,
}

// pouEnds maps a POU keyword to the keyword that ends it.
var pouEnds = map[token.TokenType]token.TokenType{
	token.PROGRAM:        token.END_PROGRAM,
	token.FUNCTION_BLOCK: token.END_FUNCTION_BLOCK,
	token.FUNCTION:       token.END_FUNCTION,
}

// sourceBodies returns the body text of each POU in source, by the byte
// offset of the POU's keyword. A body is the text after the POU's header and
// variable blocks, up to its END keyword.
func sourceBodies(source string) map[int]string {
	tokens := []token.Token{}
	l := lexer.New(source)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		if tok.Type != token.COMMENT {
			tokens = append(tokens, tok)
		}
	}

	bodies := map[int]string{}
	for i, tok := range tokens {
		end, isPOU := pouEnds[tok.Type]
		if !isPOU {
			continue
		}
		start := headerEnd(tokens, i)
		stop := -1
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Type == end {
				stop = tokens[j].Pos
				break
			}
		}
		if start < 0 || stop < start {
			continue
		}
		bodies[tok.Pos] = dedent(source[start:stop])
	}
	return bodies
}

// headerEnd returns the byte offset where the body of the POU whose keyword
// is tokens[i] starts: after its name, return type, EXTENDS and IMPLEMENTS
// clauses, and variable blocks. It returns -1 if the header is incomplete.
func headerEnd(tokens []token.Token, i int) int {
	j := i + 1
	endOf := func(k int) int { return tokens[k].Pos + len(tokens[k].Literal) }
	// Modifiers, then the name.
	for j < len(tokens) && tokens[j].Type != token.IDENT {
		j++
	}
	if j >= len(tokens) {
		return -1
	}
	pos := endOf(j)
	j++

	// A function's return type.
	if tokens[i].Type == token.FUNCTION && j < len(tokens) && tokens[j].Type == token.COLON {
		j = skipType(tokens, j+1)
		pos = endOf(j - 1)
	}
	// A function block's EXTENDS and IMPLEMENTS clauses.
	for j < len(tokens) && (tokens[j].Type == token.EXTENDS || tokens[j].Type == token.IMPLEMENTS) {
		j++
		for j < len(tokens) {
			j = skipQualifiedName(tokens, j)
			if j < len(tokens) && tokens[j].Type == token.COMMA {
				j++
				continue
			}
			break
		}
		pos = endOf(j - 1)
	}
	// Variable blocks.
	for j < len(tokens) && varBlockStarts[tokens[j].Type] {
		for j < len(tokens) && tokens[j].Type != token.END_VAR {
			j++
		}
		if j >= len(tokens) {
			return -1
		}
		pos = endOf(j)
		j++
		if j < len(tokens) && tokens[j].Type == token.SEMICOLON {
			pos = endOf(j)
			j++
		}
	}
	return pos
}

// skipType returns the index after a data type starting at tokens[j], such
// as INT, STRING[20], Lib.T, ARRAY [1..3] OF INT or REF_TO INT.
func skipType(tokens []token.Token, j int) int {
	if j >= len(tokens) {
		return j
	}
	switch strings.ToUpper(tokens[j].Literal) {
	case "ARRAY":
		for j < len(tokens) && tokens[j].Type != token.OF {
			j++
		}
		return skipType(tokens, j+1)
	case "REF_TO", "REFERENCE", "POINTER":
		j++
		if j < len(tokens) && strings.EqualFold(tokens[j].Literal, "TO") {
			j++
		}
		return skipType(tokens, j)
	}
	j = skipQualifiedName(tokens, j)
	// A length or subrange, such as STRING[20] or INT(0..9).
	for _, pair := range [][2]token.TokenType{{token.LBRACKET, token.RBRACKET}, {token.LPAREN, token.RPAREN}} {
		if j < len(tokens) && tokens[j].Type == pair[0] {
			for j < len(tokens) && tokens[j].Type != pair[1] {
				j++
			}
			j++
		}
	}
	return j
}

// skipQualifiedName returns the index after a name such as Lib.Inner.T
// starting at tokens[j].
func skipQualifiedName(tokens []token.Token, j int) int {
	j++
	for j+1 < len(tokens) && tokens[j].Type == token.DOT {
		j += 2
	}
	return j
}

// dedent removes the blank lines around text and the indentation its lines
// share.
func dedent(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, line := range lines {
		if len(line) >= indent && indent > 0 {
			lines[i] = line[indent:]
		}
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}
