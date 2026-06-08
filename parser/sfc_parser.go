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
)

// isSFC is a heuristic to determine if the upcoming code is an SFC body.
func (p *Parser) isSFC() bool {
	// SFC bodies are composed of STEP, INITIAL_STEP, TRANSITION, and ACTION statements.
	// If the current token is one of these, it's a strong indicator of an SFC program.
	switch p.curToken.Type {
	case token.STEP, token.INITIAL_STEP, token.TRANSITION, token.ACTION:
		return true
	default:
		return false
	}
}

// parseSFCProgram parses an SFC body until a given end token.
func (p *Parser) parseSFCProgram(end token.TokenType) *ast.SFCProgram {
	defer untrace(trace("parseSFCProgram"))
	program := &ast.SFCProgram{Token: p.curToken}
	program.Elements = []ast.Statement{}

	for !p.curTokenIs(end) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Elements = append(program.Elements, stmt)
		}
		// After parsing a complete SFC element (like a STEP or TRANSITION block),
		// we must advance the token to begin parsing the next element.
		// The individual parsing functions consume up to their END_* token.
		p.nextToken()
	}
	return program
}

func (p *Parser) parseActionStatement() ast.Statement {
	defer untrace(trace("parseActionStatement"))
	stmt := &ast.ActionStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected action name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken() // Consume the action name identifier

	/* 	// Create a block statement and parse statements into it until END_ACTION.
	   	// This ensures the body is always a BlockStatement.
	   	body := &ast.BlockStatement{Token: p.curToken}
	   	body.Statements = []ast.Statement{}
	   	for !p.curTokenIs(token.END_ACTION) && !p.curTokenIs(token.EOF) {
	   		stmt := p.parseStatement()
	   		if stmt != nil {
	   			body.Statements = append(body.Statements, stmt)
	   		}
	   	}
	   	stmt.Body = body

	   	if !p.curTokenIs(token.END_ACTION) {
	   		p.peekError(token.END_ACTION)
	   	} */

	stmt.Body = p.parseBlockStatementUntil(token.END_ACTION)

	if !p.curTokenIs(token.END_ACTION) {
		p.peekError(token.END_ACTION)
	}

	return stmt
}

func (p *Parser) parseTransitionStatement() ast.Statement {
	defer untrace(trace("parseTransitionStatement"))
	stmt := &ast.TransitionStatement{Token: p.curToken}

	if !p.expectPeek(token.FROM) {
		return nil
	}

	p.nextToken() // consume FROM
	stmt.From = p.parseIdentifierList()

	if !p.expectPeek(token.TO) {
		return nil
	}

	p.nextToken() // consume TO
	stmt.To = p.parseIdentifierList()

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken() // consume :=
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	if !p.expectPeek(token.END_TRANSITION) { // This should consume the END_TRANSITION token
		return nil
	}

	return stmt
}

func (p *Parser) parseStepStatement() ast.Statement {
	defer untrace(trace("parseStepStatement"))
	return p.parseStep(false)
}

func (p *Parser) parseInitialStepStatement() ast.Statement {
	defer untrace(trace("parseInitialStepStatement"))
	return p.parseStep(true)
}

func (p *Parser) parseStep(isInitial bool) *ast.StepStatement {
	stmt := &ast.StepStatement{Token: p.curToken, IsInitial: isInitial}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}
	p.nextToken() // consume COLON

	stmt.Actions = []*ast.ActionBlockStatement{}
	// Loop while the next token is not the end of the step block.
	for !p.curTokenIs(token.END_STEP) && !p.curTokenIs(token.EOF) {
		assoc := p.parseActionBlockStatement()
		if assoc == nil {
			// If parsing an action fails, break to avoid an infinite loop.
			break
		}
		stmt.Actions = append(stmt.Actions, assoc)
		p.nextToken() // Advance to the next token for the next iteration or END_STEP
	}

	// The loop terminates with curToken on END_STEP.
	if !p.curTokenIs(token.END_STEP) {
		p.specificError("missing 'END_STEP' for step starting at row %d", stmt.Token.Row)
	}

	return stmt
}

func (p *Parser) parseActionBlockStatement() *ast.ActionBlockStatement {
	defer untrace(trace("parseActionBlockStatement"))
	stmt := &ast.ActionBlockStatement{
		Token:      p.curToken,
		ActionName: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	// Check if there are any arguments (qualifiers, duration)
	if !p.peekTokenIs(token.RPAREN) {
		p.nextToken() // consume '('

		// First argument is the qualifier
		stmt.Qualifier = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken()

		// Check for optional second argument (duration)
		if p.curTokenIs(token.COMMA) {
			p.nextToken() // consume ','
			stmt.Duration = p.parseExpression(LOWEST)
		}

	} else {
		p.nextToken() // consume '(' to move to ')'
	}

	if !p.curTokenIs(token.RPAREN) {
		p.peekError(token.RPAREN)
		return nil
	}

	// After the closing parenthesis, there must be a semicolon.
	if !p.expectPeek(token.SEMICOLON) {
		// Error already reported by expectPeek.
	}

	return stmt
}
