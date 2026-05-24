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

func (p *Parser) parseFunctionBlockDeclaration() ast.Statement {
	defer untrace(trace("parseFunctionBlockDeclaration"))
	stmt := &ast.FunctionBlockDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected function block name
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
		stmt.Body = p.parseIlProgramBody(token.END_FUNCTION_BLOCK)
	} else if p.isSFC() {
		stmt.Body = p.parseSFCProgram(token.END_FUNCTION_BLOCK)
	} else {
		stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION_BLOCK, token.VAR)
	}

	if !p.curTokenIs(token.END_FUNCTION_BLOCK) {
		// Error Recovery: If we see a keyword that could start a new statement,
		// report the missing END_FUNCTION_BLOCK and return without advancing.
		if isStatementStartKeyword(p.curToken.Type) {
			p.currentError("expected next token to be %s, got %s instead", token.END_FUNCTION_BLOCK, p.curToken.Type)
		} else {
			p.peekError(token.END_FUNCTION_BLOCK)
		}
	}

	return stmt
}
