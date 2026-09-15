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

// parseFunctionBlockDeclaration parses a FUNCTION_BLOCK ... END_FUNCTION_BLOCK declaration.
// It handles the FB's name, its various variable declaration blocks (VAR_INPUT, VAR_OUTPUT, etc.),
// and its body, which can be written in ST, IL, or SFC.
func (p *Parser) parseFunctionBlockDeclaration() ast.Statement {
	defer untrace(trace("parseFunctionBlockDeclaration"))

	isAbstract := false
	var fbToken token.Token

	if p.curTokenIs(token.ABSTRACT) {
		isAbstract = true
		if !p.expectPeek(token.FUNCTION_BLOCK) {
			return nil // Expected FUNCTION_BLOCK after ABSTRACT
		}
		fbToken = p.curToken // This is now FUNCTION_BLOCK
	} else {
		fbToken = p.curToken // This is FUNCTION_BLOCK
	}

	stmt := &ast.FunctionBlockDeclaration{Token: fbToken, IsAbstract: isAbstract, LeadingComments: p.leadingComments}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected function block name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Check for optional EXTENDS clause for inheritance
	if p.peekTokenIs(token.EXTENDS) {
		p.nextToken() // consume name, move to EXTENDS
		if !p.expectPeek(token.IDENT) {
			return nil // Expected base function block name
		}
		stmt.Extends = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// Check for optional IMPLEMENTS clause for interfaces
	if p.peekTokenIs(token.IMPLEMENTS) {
		p.nextToken() // consume name or extends, move to IMPLEMENTS
		p.nextToken() // consume IMPLEMENTS
		stmt.Implements = p.parseIdentifierList()
	}

	p.nextToken()

	// Loop to parse all variable declaration blocks
var_loop:
	for {
		// Consume any comments before the next var block.
		p.consumeLeadingComments()

		// This switch handles the various types of variable blocks that can appear
		// at the start of a function block declaration.
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
		case token.VAR_TEMP:
			stmt.VarTemp = append(stmt.VarTemp, p.parseTempVarDeclStatement())
		case token.VAR_EXTERNAL:
			stmt.VarExternal = append(stmt.VarExternal, p.parseExternalVarDeclStatement())
		case token.VAR:
			stmt.Vars = append(stmt.Vars, p.parseVarBlock(token.VAR)...)
		case token.VAR_ACCESS:
			p.currentError("%s declarations are not allowed in a FUNCTION_BLOCK", p.curToken.Type)
			p.parseAccessVarDeclStatement() // Parse to recover
		case token.VAR_GLOBAL:
			p.currentError("%s declarations are not allowed in a FUNCTION_BLOCK", p.curToken.Type)
			p.parseGlobalVarDeclStatement() // Parse to recover
		default:
			// No more VAR blocks, break the loop to parse the body
			break var_loop
		}
	}

	// After var blocks, we have the body.
	// The body can be a list of methods, or a list of ST/IL/SFC statements.
	body := &ast.BlockStatement{Token: p.curToken}
	body.Statements = []ast.Statement{}

	// Heuristic: If the first thing after VAR blocks is METHOD, PROPERTY, or ABSTRACT, we assume an OO-style body.
	if p.curTokenIs(token.METHOD) || p.curTokenIs(token.PROPERTY) || p.curTokenIs(token.ABSTRACT) {
		for !p.curTokenIs(token.END_FUNCTION_BLOCK) && !p.curTokenIs(token.EOF) {
			if p.curTokenIs(token.ABSTRACT) && !stmt.IsAbstract {
				p.currentError("abstract members are not allowed in a non-abstract function block")
				p.synchronize(token.END_METHOD, token.END_PROPERTY, token.END_FUNCTION_BLOCK) // Skip to end of member or FB
				continue
			}

			if p.curTokenIs(token.METHOD) || (p.curTokenIs(token.ABSTRACT) && p.peekTokenIs(token.METHOD)) {
				method := p.parseMethodImplementation()
				if method != nil {
					body.Statements = append(body.Statements, method)
				}
			} else if p.curTokenIs(token.COMMENT) {
				p.nextToken()
				continue
			} else if p.curTokenIs(token.PROPERTY) || (p.curTokenIs(token.ABSTRACT) && p.peekTokenIs(token.PROPERTY)) {
				prop := p.parsePropertyDeclaration()
				if prop != nil {
					stmt.Properties = append(stmt.Properties, prop)
				}
			} else {
				p.currentError("unexpected token in FUNCTION_BLOCK body: %s", p.curToken.Type)
				p.nextToken() // Skip to recover
			}
		}
		stmt.Body = body
	} else if p.isIlInstruction() || (p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON)) {
		stmt.Body = p.parseIlProgramBody(token.END_FUNCTION_BLOCK)
	} else if p.isSFC() {
		stmt.Body = p.parseSFCProgram(token.END_FUNCTION_BLOCK)
	} else {
		// Otherwise, it's a regular ST body.
		stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION_BLOCK)
	}

	if !p.curTokenIs(token.END_FUNCTION_BLOCK) {
		p.currentError("expected next token to be %s, got %s instead", token.END_FUNCTION_BLOCK, p.curToken.Type)
	} else {
		p.nextToken() // Consume END_FUNCTION_BLOCK
	}

	return stmt
}
