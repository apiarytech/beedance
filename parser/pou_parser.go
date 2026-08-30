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

// parsePoulDeclaration dispatches to the correct parsing function based on the POU type
// (Program Organization Unit), which can be a FUNCTION, FUNCTION_BLOCK, or PROGRAM.
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

// parseFunctionDeclaration parses a FUNCTION ... END_FUNCTION declaration.
// It handles the function's name, return type, its variable declaration blocks
// (VAR_INPUT, VAR_OUTPUT, VAR), and its body. It explicitly disallows
// VAR_GLOBAL, VAR_EXTERNAL, etc., as per the IEC 61131-3 standard.
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

	// Loop to parse all variable declaration blocks allowed within a FUNCTION.
	// Loop to parse all variable declaration blocks
	for !p.curTokenIs(token.END_FUNCTION) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.VAR_INPUT) {
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
		} else if p.curTokenIs(token.VAR_OUTPUT) {
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
		} else if p.curTokenIs(token.VAR_IN_OUT) {
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
		} else if p.curTokenIs(token.VAR_EXTERNAL) || p.curTokenIs(token.VAR_GLOBAL) || p.curTokenIs(token.VAR_ACCESS) || p.curTokenIs(token.VAR_TEMP) {
			p.currentError("%s declarations are not allowed in a FUNCTION", p.curToken.Type)
			// To prevent an infinite loop and to recover, we parse the invalid block and discard it.
			// This switch handles parsing different invalid block types for recovery.
			switch p.curToken.Type {
			case token.VAR_EXTERNAL:
				p.parseExternalVarDeclStatement()
			case token.VAR_GLOBAL:
				p.parseGlobalVarDeclStatement()
			case token.VAR_ACCESS:
				p.parseAccessVarDeclStatement()
			case token.VAR_TEMP:
				p.parseTempVarDeclStatement()
			}
		} else if p.curTokenIs(token.VAR) {
			stmt.Vars = append(stmt.Vars, p.parseVarBlock(token.VAR)...)
		} else {
			// No more VAR blocks, break the loop to parse the body
			break
		}
	}

	// After var blocks, we have the body
	stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION)

	if p.curTokenIs(token.END_FUNCTION) {
		p.nextToken() // Consume END_FUNCTION
	}

	return stmt
}

// parseProgramDeclaration parses a PROGRAM ... END_PROGRAM declaration.
// It handles the program's name, its various variable declaration blocks
// (VAR_INPUT, VAR_OUTPUT, VAR, VAR_GLOBAL, etc.), and its body, which can be
// written in ST, IL, or SFC.
func (p *Parser) parseProgramDeclaration() ast.Statement {
	defer untrace(trace("parseProgramDeclaration"))
	stmt := &ast.ProgramDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected program name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	// Loop to parse all variable declaration blocks.
	// Loop to parse all variable declaration blocks
	for {
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}
		// This switch handles the various types of variable blocks that can appear at the start of a program declaration.
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
		case token.VAR:
			stmt.Vars = append(stmt.Vars, p.parseVarBlock(token.VAR)...)
		case token.VAR_GLOBAL:
			globalBlock := p.parseVarGlobalBlock(token.VAR_GLOBAL)
			if globalBlock != nil {
				stmt.VarGlobal = append(stmt.VarGlobal, globalBlock)
			}
		case token.VAR_EXTERNAL:
			externalBlock := p.parseVarExternalBlock(token.VAR_EXTERNAL)
			if externalBlock != nil {
				stmt.VarExternal = append(stmt.VarExternal, externalBlock)
			}
		case token.VAR_ACCESS:
			accessBlock := p.parseVarAccessBlock(token.VAR_ACCESS)
			if accessBlock != nil {
				stmt.VarAccess = append(stmt.VarAccess, accessBlock)
			}
		case token.VAR_TEMP:
			stmt.VarTemp = append(stmt.VarTemp, p.parseVarTempBlock(token.VAR_TEMP))

		default:
			// No more VAR blocks, break the loop to parse the body
			goto end_var_parsing
		}
	}
end_var_parsing:

	// After var blocks, we have the body. Check if it's IL or ST.
	// A simple heuristic: if it starts with an IL operator, parse as IL.
	if p.isIlInstruction() || (p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON)) {
		stmt.Body = p.parseIlProgramBody(token.END_PROGRAM)
	} else if p.isSFC() {
		stmt.Body = p.parseSFCProgram(token.END_PROGRAM)
	} else {
		stmt.Body = p.parseBlockStatementUntil(token.END_PROGRAM)
	}

	if !p.curTokenIs(token.END_PROGRAM) || (p.peekTokenIs(token.EOF) && p.curTokenIs(token.EOF)) {
		p.currentError("expected next token to be %s, got %s instead", token.END_PROGRAM, p.curToken.Type)
	} else {
		p.nextToken() // Consume END_PROGRAM
	}

	return stmt
}

// parseVarGlobalBlock parses a VAR_GLOBAL ... END_VAR block and returns it as a
// GlobalVarDeclaration node.
func (p *Parser) parseVarGlobalBlock(blockType token.TokenType) *ast.GlobalVarDeclaration {
	defer untrace(trace("parseVarGlobalBlock"))
	if !p.curTokenIs(blockType) {
		return nil
	}
	stmt := &ast.GlobalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_GLOBAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}
	return stmt
}

// parseVarExternalBlock parses a VAR_EXTERNAL ... END_VAR block and returns it as an
// ExternalVarDeclaration node.
func (p *Parser) parseVarExternalBlock(blockType token.TokenType) *ast.ExternalVarDeclaration {
	defer untrace(trace("parseVarExternalBlock"))
	if !p.curTokenIs(blockType) {
		return nil
	}
	stmt := &ast.ExternalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_EXTERNAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}
	return stmt
}

// parseVarAccessBlock parses a VAR_ACCESS ... END_VAR block and returns it as an
// AccessVarDeclaration node.
func (p *Parser) parseVarAccessBlock(blockType token.TokenType) *ast.AccessVarDeclaration {
	defer untrace(trace("parseVarAccessBlock"))
	if !p.curTokenIs(blockType) {
		return nil
	}
	stmt := &ast.AccessVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_ACCESS

	stmt.Vars = p.parseAccessDeclarations()

	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}
	return stmt
}

// parseProgramConfiguration parses a program instance declaration within a RESOURCE block.
// This includes the instance name, the program type, an optional task assignment
// (WITH clause), and optional parameter assignments.
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
