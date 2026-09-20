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
	case token.ABSTRACT:
		return p.parseFunctionBlockDeclaration()
	case token.FUNCTION:
		return p.parseFunctionDeclaration()
	case token.FUNCTION_BLOCK:
		return p.parseFunctionBlockDeclaration()
	case token.PUBLIC:
		return p.parseFunctionBlockDeclaration()
	case token.INTERNAL:
		return p.parseFunctionBlockDeclaration()
	case token.INTERFACE:
		return p.parseInterfaceDeclaration()
	case token.PROGRAM:
		return p.parseProgramDeclaration()
	}
	// Should not be reached if called from parseStatement correctly.
	p.currentError("unexpected token for POU declaration: %s", p.curToken.Type)
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
	returnType := p.parseTypeSpecifier()
	if returnType == nil {
		// Error already logged by parseTypeSpecifier
		return nil
	}
	ts, ok := returnType.(*ast.TypeSpecifier)
	if !ok {
		p.currentError("function return type cannot be a complex type like ARRAY or STRUCT, got %T", returnType)
		return nil
	}
	if ts.Token.Type == token.VOID {
		p.currentError("functions must return a value; use a METHOD with a VOID return type within a FUNCTION_BLOCK for procedures")
		return nil
	}
	stmt.ReturnType = ts

	p.nextToken() // Consume return type

	// Consume any comments between the header and the variable blocks.
	p.consumeLeadingComments()

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
			// In a valid program, the loop should break before this, but this handles recovery.
			if p.curTokenIs(token.END_FUNCTION) {
				break
			}
			p.currentError("%s declarations are not allowed in a FUNCTION", p.curToken.Type)
			// To prevent an infinite loop and to recover, we parse the invalid block and discard it.
			// This switch handles parsing different invalid block types for recovery.
			switch p.curToken.Type {
			case token.VAR_EXTERNAL:
				p.parseExternalVarDeclStatement()
			case token.VAR_GLOBAL:
				p.parseGlobalVarDeclStatement()
			case token.VAR_ACCESS:
				p.parseAccessVarDeclStatement() // cspell:disable-line
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
var_loop:
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
			stmt.VarTemp = append(stmt.VarTemp, p.parseTempVarDeclStatement())

		default:
			// No more VAR blocks, break the loop to parse the body
			break var_loop
		}
	}
	// After var blocks, we have the body. Check if it's IL or ST.
	// A simple heuristic: if it starts with an IL operator, parse as IL.
	if p.isIlInstruction() || (p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON)) {
		stmt.Body = p.parseIlProgramBody(token.END_PROGRAM)
	} else if p.isSFC() {
		stmt.Body = p.parseSFCProgram(token.END_PROGRAM)
	} else {
		stmt.Body = p.parseBlockStatementUntil(token.END_PROGRAM)
	}

	if !p.curTokenIs(token.END_PROGRAM) {
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

	stmt.Vars = p.parseVarDeclarations(token.END_VAR, blockType)

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

	stmt.Vars = p.parseVarDeclarations(token.END_VAR, blockType)

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
// func (p *Parser) parseProgramConfiguration() *ast.ProgramConfiguration {
// 	defer untrace(trace("parseProgramConfiguration"))
// 	stmt := &ast.ProgramConfiguration{Token: p.curToken}

// 	if !p.expectPeek(token.IDENT) {
// 		return nil // Expected program instance name
// 	}
// 	stmt.InstanceName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

// 	// Check for optional WITH clause
// 	if p.peekTokenIs(token.WITH) {
// 		p.nextToken() // consume instance name, move to WITH
// 		if !p.expectPeek(token.IDENT) {
// 			return nil // Expected task name
// 		}
// 		stmt.TaskName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
// 		p.nextToken() // Consume the task name (e.g., T1)
// 	}

// 	// If a type name is explicitly provided with a colon
// 	if p.peekTokenIs(token.COLON) {
// 		p.nextToken() // consume instance/task name
// 		p.nextToken() // consume COLON
// 		if !p.curTokenIs(token.IDENT) {
// 			p.currentError("expected program type name after :")
// 			return nil
// 		}
// 		stmt.TypeName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
// 	} else {
// 		// Otherwise, the type name is the same as the instance name.
// 		stmt.TypeName = stmt.InstanceName
// 	}
// 	// TODO: Parse optional parenthesized connection list `(...)`
// 	// Optional: ( <parameter_assignments> )
// 	if p.peekTokenIs(token.LPAREN) {
// 		p.nextToken() // consume type name, move to LPAREN
// 		stmt.Parameters = p.parseExpressionList(token.RPAREN)
// 	}

// 	// Program configuration must end with a semicolon
// 	p.expectPeek(token.SEMICOLON) // Consume semicolon

// 	return stmt
// }

// parseMethodImplementation parses a METHOD ... END_METHOD block with a body, within a FUNCTION_BLOCK.
// func (p *Parser) parseMethodImplementation() *ast.MethodImplementation {
// 	defer untrace(trace("parseMethodImplementation"))

// 	methodToken := p.curToken // curToken is METHOD
// 	if !p.curTokenIs(token.METHOD) {
// 		p.currentError("expected METHOD, got %s", p.curToken.Type)
// 		return nil
// 	}

// 	isAbstract := false
// 	if p.curTokenIs(token.ABSTRACT) {
// 		isAbstract = true
// 		p.nextToken() // consume ABSTRACT
// 	}

// 	isFinal := false
// 	if p.curTokenIs(token.FINAL) {
// 		isFinal = true
// 		p.nextToken() // consume FINAL
// 	}

// 	var accessSpecifier string
// 	if p.curTokenIs(token.PUBLIC) || p.curTokenIs(token.PRIVATE) || p.curTokenIs(token.PROTECTED) {
// 		accessSpecifier = p.curToken.Literal
// 		p.nextToken() // consume access specifier
// 	}

// 	stmt := &ast.MethodImplementation{Token: methodToken, IsAbstract: isAbstract, IsFinal: isFinal, AccessSpecifier: accessSpecifier}

// 	if !p.curTokenIs(token.IDENT) {
// 		p.currentError("expected method name, got %s", p.curToken.Type)
// 		return nil // Expected method name
// 	}
// 	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

// 	// Optional return type
// 	if p.peekTokenIs(token.COLON) {
// 		p.nextToken() // consume name
// 		p.nextToken() // consume ':'
// 		returnType := p.parseTypeSpecifier()
// 		if returnType == nil {
// 			return nil // Error already logged
// 		}
// 		ts, ok := returnType.(*ast.TypeSpecifier)
// 		if !ok {
// 			p.currentError("method return type cannot be a complex type like ARRAY or STRUCT, got %T", returnType)
// 			return nil
// 		}
// 		stmt.ReturnType = ts
// 	}

// 	p.nextToken() // Consume name or return type

// 	// Loop to parse VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR blocks
// var_loop:
// 	for !p.curTokenIs(token.END_METHOD) && !p.curTokenIs(token.EOF) {
// 		switch p.curToken.Type {
// 		case token.VAR_INPUT:
// 			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
// 		case token.VAR_OUTPUT:
// 			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
// 		case token.VAR_IN_OUT:
// 			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
// 		case token.VAR:
// 			if isAbstract {
// 				p.currentError("abstract method cannot have VAR declarations")
// 			}
// 			stmt.Vars = append(stmt.Vars, p.parseVarBlock(token.VAR)...)
// 		case token.COMMENT:
// 			p.nextToken()
// 			continue
// 		default:
// 			// No more VAR blocks, break the loop to parse the body
// 			break var_loop
// 		}
// 	}

// 	// After var blocks, we have the body
// 	if !isAbstract {
// 		stmt.Body = p.parseBlockStatementUntil(token.END_METHOD)
// 	} else {
// 		if !p.curTokenIs(token.END_METHOD) && !p.peekTokenIs(token.END_METHOD) {
// 			p.currentError("abstract method cannot have a body")
// 			p.synchronize(token.END_METHOD)
// 		}
// 	}

// 	if !p.curTokenIs(token.END_METHOD) {
// 		p.currentError("expected END_METHOD, got %s", p.curToken.Type)
// 	} else {
// 		p.nextToken() // Consume END_METHOD
// 	}

// 	return stmt
// }

// parseInterfaceDeclaration parses an INTERFACE ... END_INTERFACE block.
func (p *Parser) parseInterfaceDeclaration() ast.Statement {
	stmt := &ast.InterfaceDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()

	for !p.curTokenIs(token.END_INTERFACE) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.METHOD:
			method := p.parseMethodDeclaration(true) // true for prototype
			if method != nil {
				stmt.Methods = append(stmt.Methods, method)
			}
			p.nextToken() // consume semicolon
		case token.PROPERTY:
			prop := p.parsePropertyDeclaration(true) // true for prototype
			if prop != nil {
				stmt.Properties = append(stmt.Properties, prop)
			}
			p.nextToken() // consume semicolon
		case token.COMMENT:
			p.nextToken()
		default:
			p.currentError("unexpected token in INTERFACE block: %s", p.curToken.Type)
			p.nextToken()
		}
	}

	if !p.curTokenIs(token.END_INTERFACE) {
		p.currentError("expected END_INTERFACE, got %s", p.curToken.Type)
	} else {
		p.nextToken()
	}
	return stmt
}

// parseMethodDeclaration parses a method prototype inside an INTERFACE.
func (p *Parser) parseMethodDeclaration(isPrototype bool) *ast.MethodDeclaration {
	stmt := &ast.MethodDeclaration{Token: p.curToken}
	p.nextToken() // consume METHOD

	if !p.curTokenIs(token.IDENT) {
		p.currentError("expected method name, got %s", p.curToken.Type)
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()

	if p.curTokenIs(token.COLON) {
		p.nextToken()
		stmt.ReturnType = p.parseTypeSpecifier().(*ast.TypeSpecifier)
		p.nextToken()
	}

	// In a prototype, we only expect a semicolon.
	if isPrototype {
		// A prototype can have VAR blocks or just end.
		for p.curTokenIs(token.VAR_INPUT) || p.curTokenIs(token.VAR_OUTPUT) || p.curTokenIs(token.VAR_IN_OUT) {
			switch p.curToken.Type {
			case token.VAR_INPUT:
				stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
			case token.VAR_OUTPUT:
				stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
			case token.VAR_IN_OUT:
				stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
			}
		}

		// After VAR blocks, we can have either END_METHOD (for block form) or a semicolon (for simple form).
		// The caller is responsible for consuming the end token.
		if !p.curTokenIs(token.END_METHOD) && !p.curTokenIs(token.SEMICOLON) {
			p.currentError("expected ; at end of method prototype, got %s", p.curToken.Type)
		}
	}
	return stmt
}

// // parseMethodImplementation parses a full METHOD ... END_METHOD block inside a FUNCTION_BLOCK.
func (p *Parser) parseMethodImplementation() *ast.MethodImplementation {
	stmt := &ast.MethodImplementation{Token: p.curToken}

	if !p.curTokenIs(token.METHOD) {
		p.currentError("expected METHOD, got %s", p.curToken.Type)
		return nil
	}
	p.nextToken() // consume METHOD

	if p.curTokenIs(token.ABSTRACT) {
		stmt.IsAbstract = true
		p.nextToken()
	}

	if !p.curTokenIs(token.IDENT) {
		p.currentError("expected method name, got %s", p.curToken.Type)
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()

	if p.curTokenIs(token.COLON) {
		p.nextToken()
		returnType := p.parseTypeSpecifier()
		if returnType == nil {
			// Error already logged by parseTypeSpecifier
			return nil
		}
		ts, ok := returnType.(*ast.TypeSpecifier)
		if !ok {
			p.currentError("method return type cannot be a complex type like ARRAY or STRUCT, got %T", returnType)
			return nil
		}
		stmt.ReturnType = ts
		p.nextToken()
	}

	// Parse VAR blocks
	for !isStatementStartKeyword(p.curToken.Type) && !p.curTokenIs(token.END_METHOD) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = p.parseVarBlock(token.VAR_INPUT)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = p.parseVarBlock(token.VAR_OUTPUT)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = p.parseVarBlock(token.VAR_IN_OUT)
		case token.VAR:
			if stmt.IsAbstract {
				p.currentError("abstract method cannot have VAR declarations")
			}
			stmt.Vars = p.parseVarBlock(token.VAR)
		case token.COMMENT:
			p.nextToken()
		default:
			goto method_body_loop
		}
	}
method_body_loop:

	// If abstract, there's no body, just END_METHOD.
	if stmt.IsAbstract {
		if !p.curTokenIs(token.END_METHOD) {
			p.currentError("abstract method cannot have a body")
			p.synchronize(token.END_METHOD)
		}
		if p.curTokenIs(token.END_METHOD) {
			p.nextToken()
		}
		return stmt
	}

	stmt.Body = p.parseBlockStatementUntil(token.END_METHOD)

	if !p.curTokenIs(token.END_METHOD) {
		p.currentError("expected END_METHOD, got %s", p.curToken.Type)
	} else {
		p.nextToken()
	}
	return stmt
}

// parseMethodPrototype parses a method signature within an INTERFACE.
// e.g., `METHOD Start : BOOL;`
func (p *Parser) parseMethodPrototype() *ast.MethodDeclaration {
	defer untrace(trace("parseMethodPrototype"))
	stmt := &ast.MethodDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected method name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Optional return type
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // consume name
		p.nextToken() // consume ':'
		returnType := p.parseTypeSpecifier()
		if returnType == nil {
			return nil // Error already logged
		}
		ts, ok := returnType.(*ast.TypeSpecifier)
		if !ok {
			p.currentError("method return type cannot be a complex type like ARRAY or STRUCT, got %T", returnType)
			return nil
		}
		stmt.ReturnType = ts
	}

	p.nextToken() // Consume name or return type

	// Loop to parse VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT blocks
var_loop:
	for !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.END_METHOD) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarBlock(token.VAR_INPUT)...)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarBlock(token.VAR_OUTPUT)...)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarBlock(token.VAR_IN_OUT)...)
		case token.COMMENT:
			p.nextToken()
			continue
		default:
			break var_loop
		}
	}

	// A prototype ends with a semicolon (for simple form) or END_METHOD (for block form).
	if !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.END_METHOD) {
		p.currentError("expected ; or END_METHOD at end of method prototype, got %s", p.curToken.Type)
		return nil
	}

	return stmt
}

// parsePropertyDeclaration parses a PROPERTY declaration, handling both prototypes and full implementations.
func (p *Parser) parsePropertyDeclaration(isPrototype bool) *ast.PropertyDeclaration {
	stmt := &ast.PropertyDeclaration{Token: p.curToken}

	if !p.curTokenIs(token.PROPERTY) {
		p.currentError("expected PROPERTY, got %s", p.curToken.Type)
		return nil
	}
	p.nextToken() // consume PROPERTY

	if p.curTokenIs(token.ABSTRACT) {
		stmt.IsAbstract = true
		p.nextToken()
	}

	if !p.curTokenIs(token.IDENT) {
		p.currentError("expected property name, got %s", p.curToken.Type)
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()

	if !p.curTokenIs(token.COLON) {
		p.currentError("expected : after property name, got %s", p.curToken.Type)
		return nil
	}
	p.nextToken()
	stmt.DataType = p.parseTypeSpecifier().(*ast.TypeSpecifier)
	p.nextToken()

	if isPrototype {
		// Prototypes can have GET, SET, or both, followed by a semicolon.
		if p.curTokenIs(token.GET) {
			stmt.Getter = &ast.PropertyGetter{Token: p.curToken}
			p.nextToken()
		}
		if p.curTokenIs(token.SET) {
			stmt.Setter = &ast.PropertySetter{Token: p.curToken}
			p.nextToken()
		}
		if !p.curTokenIs(token.SEMICOLON) {
			p.currentError("expected ; at end of property prototype, got %s", p.curToken.Type)
		}
	} else {
		// Handle abstract property with body error
		if stmt.IsAbstract {
			if !p.curTokenIs(token.END_PROPERTY) {
				p.currentError("abstract property cannot have GET or SET implementations")
				p.synchronize(token.END_PROPERTY)
			}
			if p.curTokenIs(token.END_PROPERTY) {
				p.nextToken()
			}
			return stmt
		}
		// Full implementation with bodies
		for !p.curTokenIs(token.END_PROPERTY) && !p.curTokenIs(token.EOF) {
			var accessorAccessSpecifier string
			if p.curTokenIs(token.PUBLIC) || p.curTokenIs(token.PRIVATE) || p.curTokenIs(token.PROTECTED) {
				accessorAccessSpecifier = p.curToken.Literal
				p.nextToken() // consume access specifier
			}

			switch p.curToken.Type {
			case token.GET:
				stmt.Getter = p.parsePropertyGetter(false, accessorAccessSpecifier)
			case token.SET:
				stmt.Setter = p.parsePropertySetter(false, accessorAccessSpecifier)
			case token.COMMENT:
				p.nextToken()
			default:
				p.currentError("unexpected token in PROPERTY block: %s", p.curToken.Type)
				p.nextToken()
			}
		}
		if !p.curTokenIs(token.END_PROPERTY) {
			p.currentError("expected END_PROPERTY, got %s", p.curToken.Type)
		} else {
			p.nextToken()
		}
	}
	return stmt
}

// parsePropertyDeclaration parses a PROPERTY ... END_PROPERTY block.
// func (p *Parser) parsePropertyDeclaration() *ast.PropertyDeclaration {
// 	defer untrace(trace("parsePropertyDeclaration"))

// 	propToken := p.curToken // curToken is PROPERTY
// 	if !p.curTokenIs(token.PROPERTY) {
// 		p.currentError("expected PROPERTY, got %s", p.curToken.Type)
// 		return nil
// 	}
// 	p.nextToken() // consume PROPERTY

// 	isAbstract := false
// 	if p.curTokenIs(token.ABSTRACT) {
// 		isAbstract = true
// 		p.nextToken() // consume ABSTRACT
// 	}

// 	var accessSpecifier string
// 	if p.curTokenIs(token.PUBLIC) || p.curTokenIs(token.PRIVATE) || p.curTokenIs(token.PROTECTED) {
// 		accessSpecifier = p.curToken.Literal
// 		p.nextToken() // consume access specifier
// 	}

// 	stmt := &ast.PropertyDeclaration{Token: propToken, IsAbstract: isAbstract, AccessSpecifier: accessSpecifier}

// 	if !p.curTokenIs(token.IDENT) {
// 		p.currentError("expected property name, got %s", p.curToken.Type)
// 		return nil // Expected property name
// 	}
// 	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

// 	if !p.expectPeek(token.COLON) {
// 		return nil // Expected ':' after property name
// 	}

// 	p.nextToken() // Consume ':', move to data type
// 	returnType := p.parseTypeSpecifier()
// 	if returnType == nil {
// 		return nil // Error already logged
// 	}
// 	ts, ok := returnType.(*ast.TypeSpecifier)
// 	if !ok {
// 		p.currentError("property return type cannot be a complex type like ARRAY or STRUCT, got %T", returnType)
// 		return nil
// 	}
// 	stmt.DataType = ts

// 	p.nextToken() // Consume data type

// 	// Loop to parse GET and SET blocks
// 	for !p.curTokenIs(token.END_PROPERTY) && !p.curTokenIs(token.EOF) {
// 		var accessorAccessSpecifier string
// 		if p.curTokenIs(token.PUBLIC) || p.curTokenIs(token.PRIVATE) || p.curTokenIs(token.PROTECTED) {
// 			accessorAccessSpecifier = p.curToken.Literal
// 			p.nextToken() // consume access specifier
// 		}

// 		switch p.curToken.Type {
// 		case token.GET:
// 			if stmt.Getter != nil {
// 				p.currentError("property can only have one GET block")
// 				// Continue parsing to attempt recovery, but mark as error.
// 			}
// 			stmt.Getter = p.parsePropertyGetter(accessorAccessSpecifier)
// 		case token.SET:
// 			if stmt.Setter != nil {
// 				p.currentError("property can only have one SET block")
// 				// Continue parsing to attempt recovery, but mark as error.
// 			}
// 			stmt.Setter = p.parsePropertySetter(accessorAccessSpecifier)
// 		case token.COMMENT:
// 			p.nextToken()
// 			continue
// 		default:
// 			p.currentError("unexpected token in PROPERTY block: %s", p.curToken.Type)
// 			p.synchronize(token.END_PROPERTY)
// 		}
// 	}

// 	if isAbstract && (stmt.Getter != nil || stmt.Setter != nil) {
// 		p.currentError("abstract property cannot have GET or SET implementations")
// 	}

// 	if !p.curTokenIs(token.END_PROPERTY) {
// 		p.currentError("expected END_PROPERTY, got %s", p.curToken.Type)
// 	} else {
// 		p.nextToken() // Consume END_PROPERTY
// 	}

// 	return stmt
// }

// parsePropertyPrototype parses a property signature within an INTERFACE.
// e.g., `PROPERTY Speed : INT GET;`
func (p *Parser) parsePropertyPrototype() *ast.PropertyDeclaration {
	defer untrace(trace("parsePropertyPrototype"))
	stmt := &ast.PropertyDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken() // consume ':'
	dataType := p.parseTypeSpecifier()
	if dataType == nil {
		return nil
	}
	ts, ok := dataType.(*ast.TypeSpecifier)
	if !ok {
		p.currentError("property data type cannot be a complex type, got %T", dataType)
		return nil
	}
	stmt.DataType = ts

	// After the type, we can have optional GET and/or SET keywords.
	if p.peekTokenIs(token.GET) {
		p.nextToken()
		stmt.Getter = &ast.PropertyGetter{Token: p.curToken} // Create a getter with no body
	}
	if p.peekTokenIs(token.SET) {
		p.nextToken()
		stmt.Setter = &ast.PropertySetter{Token: p.curToken} // Create a setter with no body
	}

	// A prototype ends with a semicolon.
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	// expectPeek consumed the semicolon.

	return stmt
}

// parsePropertyGetter parses a GET accessor block.
func (p *Parser) parsePropertyGetter(isPrototype bool, accessSpecifier string) *ast.PropertyGetter {
	getter := &ast.PropertyGetter{Token: p.curToken, AccessSpecifier: accessSpecifier}
	p.nextToken() // consume GET
	if !isPrototype {
		getter.Body = p.parseBlockStatementUntil(token.END_GET, token.END_PROPERTY, token.SET)
		if !p.curTokenIs(token.END_GET) {
			// If we hit END_PROPERTY or SET, it might be a shorthand implementation without an explicit END_GET.
			// In this case, we don't report an error and let the parent loop handle the token.
			if !p.curTokenIs(token.END_PROPERTY) && !p.curTokenIs(token.SET) {
				p.currentError("expected END_GET, got %s", p.curToken.Type)
			}
		} else {
			p.nextToken()
		}
	}
	return getter
}

// parsePropertySetter parses a SET accessor block.
func (p *Parser) parsePropertySetter(isPrototype bool, accessSpecifier string) *ast.PropertySetter {
	setter := &ast.PropertySetter{Token: p.curToken, AccessSpecifier: accessSpecifier}
	p.nextToken() // consume SET
	if !isPrototype {
		setter.Body = p.parseBlockStatementUntil(token.END_SET, token.END_PROPERTY, token.GET)
		if !p.curTokenIs(token.END_SET) {
			// If we hit END_PROPERTY or GET, it might be a shorthand implementation without an explicit END_SET.
			// In this case, we don't report an error and let the parent loop handle the token.
			if !p.curTokenIs(token.END_PROPERTY) && !p.curTokenIs(token.GET) {
				p.currentError("expected END_SET, got %s", p.curToken.Type)
			}
		} else {
			p.nextToken()
		}
	}
	return setter
}
