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

func (p *Parser) parsePoulDeclaration() ast.Statement {
	defer untrace(trace("parsePoulDeclaration"))
	switch p.curToken.Type {
	case token.FUNCTION:
		return p.parseFunctionDeclaration()
	case token.FUNCTION_BLOCK:
		return p.parseFunctionBlockDeclaration()
	case token.PROGRAM:
		return p.parseProgramDeclaration()
	}
	return nil // Should not be reached
}

func (p *Parser) parseFunctionDeclaration() ast.Statement {
	defer untrace(trace("parseFunctionDeclaration"))
	stmt := &ast.FunctionDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected function name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil // Expected return type separator
	}

	p.nextToken() // Consume ':', move to return type
	stmt.ReturnType = p.parseTypeSpecifier().(*ast.TypeSpecifier)

	p.nextToken() // Consume return type

	// Loop to parse all variable declaration blocks
	for !p.curTokenIs(token.END_FUNCTION) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.VAR_INPUT) {
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
		} else if p.curTokenIs(token.VAR_OUTPUT) {
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
		} else if p.curTokenIs(token.VAR_IN_OUT) {
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
		} else if p.curTokenIs(token.VAR) {
			stmt.Vars = append(stmt.Vars, p.parseVarBlock(token.VAR)...)
		} else {
			// No more VAR blocks, break the loop to parse the body
			break
		}
	}

	// After var blocks, we have the body
	stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION)

	return stmt
}

func (p *Parser) parseProgramDeclaration() ast.Statement {
	defer untrace(trace("parseProgramDeclaration"))
	stmt := &ast.ProgramDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected program name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	// Loop to parse all variable declaration blocks
	for {
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
		case token.VAR:
			stmt.Vars = append(stmt.Vars, p.parseVarBlock(token.VAR)...)
		default:
			// No more VAR blocks, break the loop to parse the body
			goto end_var_parsing
		}
	}
end_var_parsing:

	// After var blocks, we have the body. Check if it's IL or ST.
	// A simple heuristic: if it starts with an IL operator, parse as IL.
	if isIlOperator(p.curToken.Type) {
		stmt.Body = p.parseIlProgramBody(token.END_PROGRAM)
	} else if p.isSFC() {
		stmt.Body = p.parseSFCProgram(token.END_PROGRAM)
	} else {
		stmt.Body = p.parseBlockStatementUntil(token.END_PROGRAM)
	}

	if !p.curTokenIs(token.END_PROGRAM) {
		p.currentError("expected next token to be %s, got %s instead", token.END_PROGRAM, p.curToken.Type)
	}

	return stmt
}

func (p *Parser) parseProgramConfiguration() *ast.ProgramConfiguration {
	defer untrace(trace("parseProgramConfiguration"))
	stmt := &ast.ProgramConfiguration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected program instance name
	}
	stmt.InstanceName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Check for optional WITH clause
	if p.peekTokenIs(token.WITH) {
		p.nextToken() // consume instance name, move to WITH
		if !p.expectPeek(token.IDENT) {
			return nil // Expected task name
		}
		stmt.TaskName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected program type name
	}
	stmt.TypeName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// TODO: Parse optional parenthesized connection list `(...)`
	// Optional: ( <parameter_assignments> )
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // consume type name, move to LPAREN
		stmt.Parameters = p.parseExpressionList(token.RPAREN)
	}

	// Program configuration must end with a semicolon
	p.expectPeek(token.SEMICOLON) // Consume semicolon

	return stmt
}
