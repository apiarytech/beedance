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
		// The main parse loop advances the token, so we don't need to here
		// unless a statement parser doesn't consume its final token.
		// Since parseStatement handles this, we just need to advance to the next token.
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

	stmt.Body = p.parseBlockStatementUntil(token.END_ACTION, token.VAR)

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

	if !p.expectPeek(token.END_TRANSITION) {
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
	for !p.curTokenIs(token.END_STEP) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.LPAREN) {
			assoc := p.parseActionBlockStatement()
			if assoc != nil {
				stmt.Actions = append(stmt.Actions, assoc)
			}
		}
		if !p.expectPeek(token.SEMICOLON) {
			break // Or handle error
		}
		p.nextToken()
	}

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

	return stmt
}
