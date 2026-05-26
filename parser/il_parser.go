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

	// 2. Parse the operator (e.g., LD, ST, ADD).
	// The operator is expected to be an identifier.
	// We check for IDENT or a known IL operator keyword.
	if !p.curTokenIs(token.IDENT) && !isIlOperator(p.curToken.Type) {
		p.currentError("expected IL operator (e.g., LD, ST), got %s", p.curToken.Type)
		return nil
	}
	operatorStr := p.curToken.Literal

	// 2a. Parse modifiers from the operator string (e.g., 'LDN', 'JMPC').
	// The lexer provides the full identifier (e.g., "LDN"). We need to split it
	// into the base operator ("LD") and the modifier ("N").
	baseOp, modifier := p.extractIlModifiers(operatorStr)
	stmt.Operator = baseOp
	stmt.Modifier = modifier

	// 3. Check for the '(' modifier, which defers the operation.
	if p.peekTokenIs(token.LPAREN) {
		stmt.Modifier += "("
		p.nextToken() // consume the operator IDENT
		// The operand for a deferred operator is the result of the parenthesized expression block.
		// We need a new function to parse this special block.
		stmt.Operand = p.parseIlParenthesizedExpression()
		// The ')' is consumed by parseIlParenthesizedExpression, so we can fall through.
	} else if !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.RPAREN) && !p.peekTokenIs(token.EOF) {

		// 4. Parse the optional operand for non-deferred operators.
		// The operand is an expression that follows the operator.
		// Not all operators have operands (e.g., RET).
		// We can check if the next token could start an expression.
		if !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.RPAREN) && !p.peekTokenIs(token.EOF) {
			p.nextToken()
			stmt.Operand = p.parseExpression(LOWEST)
		}
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

	// After stripping modifiers, what remains is the base operator.
	// We need to find the original casing of the base operator.
	if len(mod) > 0 {
		return op[:len(op)-len(mod)], mod
	}

	return op, ""
}

// parseIlParenthesizedExpression handles deferred operations like `AND( ... )`.
func (p *Parser) parseIlParenthesizedExpression() ast.Expression {
	// The parenthesized expression is a block of IL instructions that acts as a single operand.
	// We can represent this with a BlockStatement.
	p.nextToken() // consume '('
	block := p.parseIlProgramBody(token.RPAREN)

	if !p.curTokenIs(token.RPAREN) {
		p.currentError("expected ')' to close deferred IL expression, got %s", p.curToken.Type)
		return nil
	}

	return block
}

// isIlOperator checks if a token type is a common IL operator.
// This is used as a heuristic to decide whether to parse a POU body as IL or ST.
func isIlOperator(tok token.TokenType) bool {
	switch tok {
	case token.LD, token.ST, token.S, token.R,
		token.AND, token.OR, token.XOR, token.NOT: // Also common IL operators
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
