/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package parser

import (
	"beedance/ast"
	"beedance/token"
	"strings"
)

// parseIlProgramBody parses the body of a POU written in Instruction List.
// It expects to be called when the parser is at the beginning of the IL body
// and will parse until it encounters the specified endToken.
func (p *Parser) parseIlProgramBody(endToken token.TokenType) *ast.BlockStatement {
	// The body of an IL program is a block of IL instructions.
	body := &ast.BlockStatement{Token: p.curToken}
	body.Statements = []ast.Statement{}

	// Loop until we hit the end of the block (e.g., END_FUNCTION_BLOCK) or EOF.
	for !p.curTokenIs(endToken) && !p.curTokenIs(token.EOF) {
		stmt := p.parseIlInstruction()
		if stmt != nil {
			body.Statements = append(body.Statements, stmt)
		}

		// In IL, each instruction is typically on a new line, ending with a semicolon
		// or implicitly ended by the newline. We advance to the next token to start
		// parsing the next instruction. If a semicolon is present, it will be consumed.
		// If not, we move to the next token on the new line.
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken()
	}

	return body
}

// parseIlInstruction parses a single instruction line in an IL program.
// An IL instruction has the general form: [label:] operator [operand] [(modifier)]
func (p *Parser) parseIlInstruction() ast.Statement {
	stmt := &ast.IlInstructionStatement{Token: p.curToken}

	// 1. Check for an optional label (e.g., "MyLabel:").
	// A label is an identifier followed by a colon.
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
		stmt.Label = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume the identifier
		p.nextToken() // consume the ':'
	}

	// 2. Parse the operator (e.g., LD, ST, ADD) and its modifiers.
	if !p.isIlMnemonic() {
		p.currentError("expected IL instruction mnemonic (e.g., LD, ST, ADD), got %q", p.curToken.Literal)
		return nil
	}
	operatorStr := p.curToken.Literal

	// The lexer provides the full identifier (e.g., "LDN", "JMPC"). We need to split
	// it into the base operator ("LD") and the modifier ("N", "C").
	baseOp, modifier := p.extractIlModifiers(operatorStr)
	stmt.Operator = baseOp
	stmt.Modifier = modifier

	// 3. Check for the '(' modifier, which defers the operation and takes precedence.
	if p.peekTokenIs(token.LPAREN) {
		// This is a deferred operation, e.g., `AND(`
		stmt.Modifier += "(" // Add '(' to the list of modifiers
		p.nextToken()        // Consume the operator (e.g., AND)
		p.nextToken()        // Consume the '('
		// The operand is the entire block of instructions inside the parentheses.
		stmt.Operand = p.parseIlProgramBody(token.RPAREN)
		// parseIlProgramBody will stop at RPAREN, so we don't need to consume it here.
		return stmt
	}

	// 4. Parse the optional operand for non-deferred operators.
	// The operand is an expression that follows the operator.
	// Heuristic: An operand is present if the next token is not an end-of-block
	// or another statement keyword.
	switch p.peekToken.Type {
	case token.END_PROGRAM, token.END_FUNCTION, token.END_FUNCTION_BLOCK, token.END_ACTION, token.END_STEP, token.END_TRANSITION, token.EOF, token.RPAREN, token.SEMICOLON:
		return stmt
	}
	if isStatementStartKeyword(p.peekToken.Type) {
		return stmt
	}

	// If we are here, an operand exists.
	// We check for EOF again just in case.
	if !p.peekTokenIs(token.EOF) {
		p.nextToken() // Consume the operator, move to the operand
		stmt.Operand = p.parseExpression(LOWEST)
	}

	return stmt
}

// extractIlModifiers splits an operator string like "JMPC" into its base ("JMP") and modifier ("C").
func (p *Parser) extractIlModifiers(op string) (baseOp string, modifier string) {
	opUpper := strings.ToUpper(op)
	mod := ""
	// Modifiers are checked from right to left. 'N' can appear before 'C'.
	if strings.HasSuffix(opUpper, "N") {
		mod = "N" + mod
		opUpper = opUpper[:len(opUpper)-1]
	}
	if strings.HasSuffix(opUpper, "C") {
		mod = "C" + mod
		opUpper = opUpper[:len(opUpper)-1]
	}

	// After stripping modifiers, what remains is the base operator. We need to find
	// the original casing of the base operator from the input string.
	baseOp = op[:len(opUpper)]

	return baseOp, mod
}

// isIlOperator checks if a token type is a common IL operator.
// This is used as a heuristic to decide whether to parse a POU body as IL or ST.
func isIlOperator(tok token.TokenType) bool {
	switch tok { // cspell:disable-line
	case token.LD, token.ST, token.S, token.R, token.CAL, token.JMP, token.RET,
		token.AND, token.OR, token.XOR, token.NOT:
		return true
	default:
		return false
	}
}

// To integrate this, you would modify the POU parsing functions in `parser.go`.
// For example, `parseFunctionBlockDeclaration` would need to detect if the body
// is ST or IL and call the appropriate body parser.
//
// Example modification in `parser.go`:
/*
func (p *Parser) parseFunctionBlockDeclaration() ast.Statement {
    ...
    // After parsing VAR blocks:
    // if language is IL {
    //     stmt.Body = p.parseIlProgramBody(token.END_FUNCTION_BLOCK)
    // } else {
    //     stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION_BLOCK)
    // }
    ...
}
*/
