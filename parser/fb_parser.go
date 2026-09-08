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
	stmt := &ast.FunctionBlockDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected function block name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	// Loop to parse all variable declaration blocks
var_loop:
	for {
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
			stmt.VarTemp = append(stmt.VarTemp, p.parseVarTempBlock(token.VAR_TEMP))
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
	// After var blocks, we have the body. Check if it's IL or ST.
	// A simple heuristic: if it starts with an IL operator, parse as IL.
	if p.isIlInstruction() || (p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON)) {
		stmt.Body = p.parseIlProgramBody(token.END_FUNCTION_BLOCK)
	} else if p.isSFC() {
		stmt.Body = p.parseSFCProgram(token.END_FUNCTION_BLOCK)
	} else {
		stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION_BLOCK)
	}

	if !p.curTokenIs(token.END_FUNCTION_BLOCK) || (p.peekTokenIs(token.EOF) && p.curTokenIs(token.EOF)) {
		p.currentError("expected next token to be %s, got %s instead", token.END_FUNCTION_BLOCK, p.curToken.Type)
	} else {
		p.nextToken() // Consume END_FUNCTION_BLOCK
	}

	return stmt
}
