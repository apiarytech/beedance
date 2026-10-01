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

	// The standard allows PUBLIC and INTERNAL.
	var accessSpecifier string
	if p.curTokenIs(token.PUBLIC) || p.curTokenIs(token.INTERNAL) {
		accessSpecifier = p.curToken.Literal
		p.nextToken() // consume PUBLIC or INTERNAL
	}

	isAbstract := false
	if p.curTokenIs(token.ABSTRACT) {
		isAbstract = true
		p.nextToken() // consume ABSTRACT
	}

	if !p.curTokenIs(token.FUNCTION_BLOCK) {
		p.currentError("expected FUNCTION_BLOCK, got %s", p.curToken.Type)
		return nil
	}
	fbToken := p.curToken // This is FUNCTION_BLOCK
	p.nextToken()         // consume FUNCTION_BLOCK

	if !isAbstract && p.curTokenIs(token.ABSTRACT) {
		isAbstract = true
		p.nextToken() // consume ABSTRACT
	}

	isFinal := false
	if p.curTokenIs(token.FINAL) {
		isFinal = true
		p.nextToken() // consume FINAL
	}

	stmt := &ast.FunctionBlockDeclaration{Token: fbToken, IsAbstract: isAbstract, IsFinal: isFinal, AccessSpecifier: accessSpecifier, LeadingComments: p.leadingComments}

	// A contextual keyword, such as SR, can name a function block.
	if contextualKeywords[p.curToken.Type] {
		p.curToken.Type = token.IDENT
	}
	if !p.curTokenIs(token.IDENT) {
		p.currentError("expected function block name, got %s", p.curToken.Type)
		return nil // Expected function block name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Check for optional EXTENDS clause for inheritance
	if p.peekTokenIs(token.EXTENDS) {
		p.nextToken() // consume name, move to EXTENDS
		p.nextToken() // consume EXTENDS
		stmt.Extends = p.parseExpression(MEMBER)
	}

	// Check for optional IMPLEMENTS clause for interfaces
	if p.peekTokenIs(token.IMPLEMENTS) {
		p.nextToken() // consume name or extends, move to IMPLEMENTS
		p.nextToken() // consume IMPLEMENTS
		stmt.Implements = p.parseTypeNameList()
	}

	p.nextToken()
	if p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	// Loop to parse all variable declaration blocks
var_loop:
	for {
		// Consume any comments before the next var block.
		p.consumeLeadingComments()
		if p.curTokenIs(token.SEMICOLON) {
			p.nextToken()
			continue
		}

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
	// The body can contain methods, properties, and then a main logic block.
	body := &ast.BlockStatement{Token: p.curToken}
	body.Statements = []ast.Statement{}

	// Loop for methods and properties
	for {
		p.consumeLeadingComments()

		// An access specifier may also be written before the keyword,
		// e.g. `PRIVATE METHOD X` as well as `METHOD PRIVATE X`.
		prefixAccess := ""
		if (p.curTokenIs(token.PUBLIC) || p.curTokenIs(token.PRIVATE) || p.curTokenIs(token.PROTECTED) || p.curTokenIs(token.INTERNAL)) &&
			(p.peekTokenIs(token.METHOD) || p.peekTokenIs(token.PROPERTY)) {
			prefixAccess = p.curToken.Literal
			p.nextToken()
		}

		isMethod := p.curTokenIs(token.METHOD)
		isProperty := p.curTokenIs(token.PROPERTY)

		if isMethod {
			method := p.parseMethodImplementation()
			if method != nil && prefixAccess != "" && method.AccessSpecifier == "" {
				method.AccessSpecifier = prefixAccess
			}
			if method != nil {
				if method.IsAbstract && !stmt.IsAbstract {
					p.currentError("abstract members are not allowed in a non-abstract function block")
				} else {
					body.Statements = append(body.Statements, method)
				}
			}
		} else if isProperty {
			prop := p.parsePropertyDeclaration(false)
			if prop != nil && prefixAccess != "" && prop.AccessSpecifier == "" {
				prop.AccessSpecifier = prefixAccess
			}
			if prop != nil {
				if prop.IsAbstract && !stmt.IsAbstract {
					p.currentError("abstract members are not allowed in a non-abstract function block")
				} else {
					stmt.Properties = append(stmt.Properties, prop)
				}
			}
		} else {
			// No more methods or properties, break to parse the main body
			break
		}
	}

	// Now, parse the main body of ST/IL/SFC statements if any exist.
	if !p.curTokenIs(token.END_FUNCTION_BLOCK) {
		// If methods were parsed, the main body can only be ST statements.
		// If no methods were parsed, we detect the language of the body.
		if len(body.Statements) == 0 && p.isSFC() {
			stmt.Body = p.parseSFCProgram(token.END_FUNCTION_BLOCK)
		} else if len(body.Statements) == 0 && (p.isIlInstruction() || (p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON))) {
			stmt.Body = p.parseIlProgramBody(token.END_FUNCTION_BLOCK)
		} else {
			// It's an ST body, or an ST body following methods.
			mainBody := p.parseBlockStatementUntil(token.END_FUNCTION_BLOCK)
			if mainBody != nil {
				body.Statements = append(body.Statements, mainBody.Statements...)
			}
			stmt.Body = body
		}
	} else {
		stmt.Body = body
	}

	if !p.curTokenIs(token.END_FUNCTION_BLOCK) {
		p.currentError("expected next token to be %s, got %s instead", token.END_FUNCTION_BLOCK, p.curToken.Type)
	} else {
		p.nextToken() // Consume END_FUNCTION_BLOCK
	}
	return stmt
}
