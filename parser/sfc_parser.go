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

// isSFC provides a heuristic check to determine if the current token indicates the
// start of a Sequential Function Chart (SFC) body. It looks for keywords that are
// unique to SFC, such as STEP, TRANSITION, or ACTION.
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

// parseSFCProgram parses the body of a Program Organization Unit (POU) written in
// Sequential Function Chart (SFC). It consumes SFC elements like steps, actions,
// and transitions until it reaches a specified end token (e.g., `END_PROGRAM`).
func (p *Parser) parseSFCProgram(end token.TokenType) *ast.SFCProgram {
	defer untrace(trace("parseSFCProgram"))
	program := &ast.SFCProgram{Token: p.curToken}
	program.Elements = []ast.Statement{}

	for !p.curTokenIs(end) && !p.curTokenIs(token.EOF) {
		// Skip any comments that might be between SFC elements.
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			program.Elements = append(program.Elements, stmt)
		}
		// SFC elements are block statements that consume their own end tokens.
		// The parser is now positioned at the start of the next element, so we
		// do not advance the token here. This aligns with the main ParseProgram loop.
		if !isBlockStatement(stmt) {
			p.nextToken()
		}
	}
	return program
}

// parseActionStatement parses an `ACTION...END_ACTION` block, which defines a named
// set of instructions that can be associated with an SFC step.
func (p *Parser) parseActionStatement() ast.Statement {
	defer untrace(trace("parseActionStatement"))
	stmt := &ast.ActionStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected action name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// An action declaration is followed by a colon.
	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken() // consume COLON

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
	} else {
		p.nextToken() // Consume END_ACTION
	}

	return stmt
}

// parseTransitionStatement parses a `TRANSITION...END_TRANSITION` block. This defines
// the condition that must be met for the SFC to move from one or more source steps
// to one or more destination steps.
func (p *Parser) parseTransitionStatement() ast.Statement {
	defer untrace(trace("parseTransitionStatement"))
	stmt := &ast.TransitionStatement{Token: p.curToken}

	if !p.expectPeek(token.FROM) {
		// Allow recovery even if FROM is missing
	} else {
		p.nextToken() // consume FROM
		stmt.From = p.parseStepList()
	}

	if !p.expectPeek(token.TO) {
		// Allow recovery even if TO is missing
	} else {
		p.nextToken() // consume TO
		stmt.To = p.parseStepList()
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken() // consume ASSIGN
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.SEMICOLON) {
		return stmt // Allow recovery
	}

	if !p.expectPeek(token.END_TRANSITION) {
		return stmt // Allow recovery
	}
	p.nextToken() // Consume END_TRANSITION
	return stmt
}

// parseStepStatement is a convenience function that calls `parseStep` to parse a
// standard `STEP` block.
func (p *Parser) parseStepStatement() ast.Statement {
	defer untrace(trace("parseStepStatement"))
	return p.parseStep(false)
}

// parseInitialStepStatement is a convenience function that calls `parseStep` to parse
// an `INITIAL_STEP` block, marking it as the starting point of the SFC.
func (p *Parser) parseInitialStepStatement() ast.Statement {
	defer untrace(trace("parseInitialStepStatement"))
	return p.parseStep(true)
}

// parseStep parses a `STEP` or `INITIAL_STEP` block. It captures the step's name
// and its body, which can either be a list of action associations or a block of ST code.
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

	// Heuristic to decide between action associations and ST body.
	// If we see `IDENT (`, it's an action association list.
	// Otherwise, it's an ST body.
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.LPAREN) {
		stmt.Actions = []*ast.ActionBlockStatement{}
		// Loop while the next token is not the end of the step block.
		for !p.curTokenIs(token.END_STEP) && !p.curTokenIs(token.EOF) {
			// Skip any comments that might be inside the step body.
			if p.curTokenIs(token.COMMENT) {
				p.nextToken()
				continue
			}

			assoc := p.parseActionBlockStatement()
			if assoc == nil {
				// If parsing an action fails, break to avoid an infinite loop.
				break
			}
			stmt.Actions = append(stmt.Actions, assoc)
			p.nextToken() // Advance to the next token for the next iteration or END_STEP
		}
	} else {
		// It's not an action association list, so parse it as a block of ST statements.
		stmt.Body = p.parseBlockStatementUntil(token.END_STEP)
	}

	// The loop terminates with curToken on END_STEP.
	if !p.curTokenIs(token.END_STEP) {
		p.specificError("missing 'END_STEP' for step starting at row %d", stmt.Token.Row)
	} else {
		p.nextToken() // Consume END_STEP
	}

	return stmt
}

// parseActionBlockStatement parses an action association within a `STEP` body,
// such as `MyAction(N);`. This links a defined `ACTION` to the step and specifies
// its execution behavior with an optional qualifier (e.g., N, S, R, P, L, D).
func (p *Parser) parseActionBlockStatement() *ast.ActionBlockStatement {
	defer untrace(trace("parseActionBlockStatement"))
	stmt := &ast.ActionBlockStatement{
		Token:      p.curToken,
		ActionName: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.expectPeek(token.LPAREN) { // After this, curToken is '('
		return nil
	}

	// Check if there are any arguments (qualifiers, duration)
	if !p.peekTokenIs(token.RPAREN) {
		p.nextToken() // consume '(', move to first arg (qualifier)

		// First argument is the qualifier
		stmt.Qualifier = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		// Check for optional second argument (duration)
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume qualifier
			p.nextToken() // consume ','
			stmt.Duration = p.parseExpression(LOWEST)
		}
	}

	// After parsing arguments, we must find the closing parenthesis.
	if !p.expectPeek(token.RPAREN) {
		p.peekError(token.RPAREN)
		return nil
	}

	// After the closing parenthesis, there must be a semicolon.
	if !p.expectPeek(token.SEMICOLON) {
		// Error already reported by expectPeek.
	}

	return stmt
}
