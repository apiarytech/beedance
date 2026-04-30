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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"beedance/ast"
	"beedance/lexer"
	"beedance/token"
)

const (
	_ int = iota
	LOWEST
	ASSIGN      // :=
	LOGICAL_OR  // OR
	LOGICAL_XOR // XOR
	LOGICAL_AND // AND
	EQUALS      // =, <>
	LESSGREATER // <, >, <=, >=
	SUM         // +, -
	PRODUCT     // *, /, MOD
	EXPONENT    // **
	PREFIX      // -X or NOT X
	CALL        // myFunction(X)
	INDEX       // array[index]
	MEMBER      // struct.member
)

var precedences = map[token.TokenType]int{
	token.ASSIGN:    ASSIGN,
	token.OR:        LOGICAL_OR,
	token.NOR:       LOGICAL_OR,
	token.XOR:       LOGICAL_XOR,
	token.AND:       LOGICAL_AND,
	token.AMPERSAND: LOGICAL_AND,
	token.NAND:      LOGICAL_AND,
	token.EQ:        EQUALS,
	token.NEQ:       EQUALS,
	token.LT:        LESSGREATER,
	token.GT:        LESSGREATER,
	token.LE:        LESSGREATER,
	token.GE:        LESSGREATER,
	token.RANGE:     LESSGREATER,
	token.MINUS:     SUM,
	token.PLUS:      SUM,
	token.MOD:       PRODUCT,
	token.SLASH:     PRODUCT,
	token.ASTERISK:  PRODUCT,
	token.EXPONENT:  EXPONENT,
	token.NOT:       PREFIX,
	token.LPAREN:    CALL,
	token.LBRACKET:  INDEX,
	token.DOT:       MEMBER,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// B.3.1 Expressions - prefix functions
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)

	// Literals (B.1.2)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.REAL, p.parseRealLiteral)
	p.registerPrefix(token.LREAL, p.parseRealLiteral)
	p.registerPrefix(token.STRING_LITERAL, p.parseStringLiteral)
	p.registerPrefix(token.WSTRING_LITERAL, p.parseStringLiteral) // Treat WSTRING as STRING for now
	p.registerPrefix(token.TIME, p.parseTimeDateLiteral)
	p.registerPrefix(token.DATE, p.parseTimeDateLiteral)
	p.registerPrefix(token.TIME_OF_DAY, p.parseTimeDateLiteral)
	// Specific integer types
	p.registerPrefix(token.SINT, p.parseIntegerLiteral)
	p.registerPrefix(token.DINT, p.parseIntegerLiteral)
	p.registerPrefix(token.LINT, p.parseIntegerLiteral)
	p.registerPrefix(token.USINT, p.parseIntegerLiteral)
	p.registerPrefix(token.UINT, p.parseIntegerLiteral)
	p.registerPrefix(token.UDINT, p.parseIntegerLiteral)
	p.registerPrefix(token.ULINT, p.parseIntegerLiteral)
	p.registerPrefix(token.BYTE, p.parseBitStringLiteral)
	p.registerPrefix(token.WORD, p.parseBitStringLiteral)
	p.registerPrefix(token.DWORD, p.parseBitStringLiteral)
	p.registerPrefix(token.LWORD, p.parseBitStringLiteral)
	p.registerPrefix(token.DATE_AND_TIME, p.parseTimeDateLiteral)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)

	// Variables (B.1.4)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.DIRECT_VAR, p.parseDirectVariable)

	// Prefix Operators
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)

	// Grouped Expressions
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)
	p.registerPrefix(token.MACRO, p.parseMacroLiteral)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.LBRACE, p.parseHashLiteral)
	p.registerPrefix(token.STRUCT, p.parseStructDefinition)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EXPONENT, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NEQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LE, p.parseInfixExpression)
	p.registerInfix(token.GE, p.parseInfixExpression)

	// Logical operators
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.XOR, p.parseInfixExpression)
	p.registerInfix(token.NAND, p.parseInfixExpression)
	p.registerInfix(token.NOR, p.parseInfixExpression)
	// Non-standard logical operators are now grouped with standard ones
	// for clarity, but the precedence map dictates their behavior.
	p.registerInfix(token.AMPERSAND, p.parseInfixExpression) // Standard alias for AND

	p.registerInfix(token.RANGE, p.parseInfixExpression)
	p.registerInfix(token.MOD, p.parseInfixExpression)

	p.registerInfix(token.ASSIGN, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseMemberAccessExpression)

	// // Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()

	// Check for lexer errors and create parser errors for them.
	switch p.curToken.Type {
	case token.ILLEGAL:
		msg := fmt.Sprintf("illegal character %q at row %d, column %d", p.curToken.Literal, p.curToken.Row, p.curToken.Column)
		p.errors = append(p.errors, msg)
	case token.UNTERMINATED_STRING:
		msg := fmt.Sprintf("unterminated string at row %d, column %d", p.curToken.Row, p.curToken.Column)
		p.errors = append(p.errors, msg)
	case token.UNTERMINATED_COMMENT:
		msg := fmt.Sprintf("unterminated comment starting at row %d, column %d", p.curToken.Row, p.curToken.Column)
		p.errors = append(p.errors, msg)
	}
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead at row %d, column %d",
		t, p.peekToken.Type, p.peekToken.Row, p.peekToken.Column)
	p.errors = append(p.errors, msg)
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		if !p.curTokenIs(token.EOF) {
			p.nextToken()
		}
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.VAR:
		return p.parseVarBlockStatement()
	case token.VAR_GLOBAL:
		return p.parseGlobalVarDeclStatement()
	case token.VAR_EXTERNAL:
		return p.parseExternalVarDeclStatement()
	case token.VAR_ACCESS:
		return p.parseAccessVarDeclStatement()
	case token.VAR_TEMP:
		return p.parseTempVarDeclStatement()
	case token.VAR_CONFIG:
		return p.parseConfigVarDeclStatement()
	case token.TYPE:
		return p.parseTypeBlockDeclaration()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.IF:
		return p.parseIfStatement()
	case token.FOR:
		return p.parseForStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.REPEAT:
		return p.parseRepeatStatement()
	case token.CASE:
		return p.parseCaseStatement()
	case token.PROGRAM, token.FUNCTION, token.FUNCTION_BLOCK:
		return p.parsePoulDeclaration()
	case token.ACTION:
		return p.parseActionStatement()
	case token.TRANSITION:
		return p.parseTransitionStatement()
	case token.STEP:
		return p.parseStepStatement()
	case token.INITIAL_STEP:
		return p.parseInitialStepStatement()
	case token.CONFIGURATION:
		return p.parseConfigurationDeclaration() // No semicolon expected after this block
	case token.EXIT:
		return p.parseExitStatement()
	case token.IDENT:
		if p.peekTokenIs(token.ASSIGN) {
			return p.parseAssignmentStatement()
		}
		return p.parseExpressionStatement()
	default:
		return p.parseExpressionStatement()
	}
}
func (p *Parser) parseExitStatement() *ast.ExitStatement {
	stmt := &ast.ExitStatement{Token: p.curToken}

	// An EXIT statement must be followed by a semicolon.
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	return stmt
}
func (p *Parser) parseSingleVarDecl() *ast.VarDeclStatement {
	stmt := &ast.VarDeclStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken() // Move to the data type token

	// Check for CONSTANT qualifier
	if p.curTokenIs(token.CONSTANT) {
		stmt.IsConstant = true
		p.nextToken() // consume CONSTANT, move to data type
	}

	// Check for RETAIN/NON_RETAIN qualifiers
	if p.curTokenIs(token.RETAIN) {
		stmt.IsRetain = true
		p.nextToken() // consume RETAIN
	} else if p.curTokenIs(token.NON_RETAIN) {
		stmt.IsNonRetain = true
		p.nextToken() // consume NON_RETAIN
	}

	// Check if the current token is a valid data type
	if !p.isDataTypeToken(p.curToken) {
		p.errors = append(p.errors, fmt.Sprintf("expected a data type, got %s", p.curToken.Literal))
		return nil
	}

	stmt.DataType = &ast.TypeSpecifier{Token: p.curToken}

	// Check for optional initial value assignment (:=)
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // consume ':='
		p.nextToken() // consume expression start
		stmt.Value = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseVarDeclStatement() *ast.VarDeclStatement {
	stmt := &ast.VarDeclStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken() // Move to the data type token

	// Check for RETAIN/NON_RETAIN qualifiers
	if p.curTokenIs(token.RETAIN) {
		stmt.IsRetain = true
		p.nextToken() // consume RETAIN
	} else if p.curTokenIs(token.NON_RETAIN) {
		stmt.IsNonRetain = true
		p.nextToken() // consume NON_RETAIN
	}

	stmt.DataType = p.parseTypeSpecifier() // This now correctly assigns ast.Expression
	if stmt.DataType == nil {
		return nil
	}
	p.nextToken() // consume data type

	// Check for optional initial value assignment (:=)
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // consume ':='
		p.nextToken() // consume expression start
		stmt.Value = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	return stmt
}

func (p *Parser) parseVarBlockStatement() *ast.VarBlockDeclaration {
	stmt := &ast.VarBlockDeclaration{Token: p.curToken}

	stmt.Declarations = p.parseVarDeclarations(token.END_VAR)
	p.nextToken()
	return stmt
}

func (p *Parser) parseStructMember() *ast.VarDeclStatement {
	// Struct members are like variable declarations but without the VAR keyword
	// and, according to the IEC 61131-3 standard, without initial values.
	stmt := &ast.VarDeclStatement{Token: p.curToken}

	if !p.curTokenIs(token.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("expected member name (identifier), got %s", p.curToken.Type))
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}
	p.nextToken() // Move to the data type token

	stmt.DataType = p.parseTypeSpecifier()
	if stmt.DataType == nil {
		return nil
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.errors = append(p.errors, fmt.Sprintf("initialization is not allowed for struct members at row %d, column %d", p.peekToken.Row, p.peekToken.Column))
		// We can try to recover by skipping the initialization part to continue parsing.
		for !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.END_STRUCT) && !p.peekTokenIs(token.EOF) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	return stmt
}

func (p *Parser) parseGlobalVarDeclStatement() *ast.GlobalVarDeclaration {
	stmt := &ast.GlobalVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseExternalVarDeclStatement() *ast.ExternalVarDeclaration {
	stmt := &ast.ExternalVarDeclaration{Token: p.curToken}
	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseAccessVarDeclStatement() *ast.AccessVarDeclaration {
	stmt := &ast.AccessVarDeclaration{Token: p.curToken}
	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseTempVarDeclStatement() *ast.TempVarDeclaration {
	stmt := &ast.TempVarDeclaration{Token: p.curToken}
	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseConfigVarDeclStatement() *ast.ConfigVarDeclaration {
	stmt := &ast.ConfigVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	p.nextToken()
	return stmt
}

func (p *Parser) parseTypeBlockDeclaration() *ast.TypeBlockDeclaration {
	block := &ast.TypeBlockDeclaration{Token: p.curToken}
	block.Declarations = []*ast.TypeDeclaration{}

	p.nextToken() // consume TYPE

	for !p.curTokenIs(token.END_TYPE) && !p.curTokenIs(token.EOF) {
		decl := p.parseTypeDeclaration()
		if decl != nil {
			block.Declarations = append(block.Declarations, decl)
		}
		p.nextToken() // Move to the start of the next declaration or END_TYPE
	}

	return block
}

func (p *Parser) parseTypeDeclaration() *ast.TypeDeclaration {
	decl := &ast.TypeDeclaration{Token: p.curToken}

	if !p.curTokenIs(token.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier, got %s", p.curToken.Type))
		return nil
	}
	decl.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}
	p.nextToken() // consume COLON, move to type definition

	if p.curTokenIs(token.STRUCT) {
		decl.DataType = p.parseStructDefinition()
	} else {
		decl.DataType = p.parseTypeSpecifier() // Can be ARRAY or simple type
	}

	// After the base type, check for optional subrange or initialization.
	if p.peekTokenIs(token.LPAREN) {
		// This is a subrange declaration, e.g., INT (0..100)
		p.nextToken() // consume type, curToken is now '('
		decl.Subrange = p.parseGroupedExpression()
	}

	if p.peekTokenIs(token.ASSIGN) {
		// This is an initialization, e.g., INT := 10
		p.nextToken() // consume type or subrange, curToken is now ':='
		p.nextToken() // consume ':=', move to expression start
		decl.InitialValue = p.parseExpression(LOWEST)
	}

	// A type declaration must end with a semicolon.
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	return decl
}

func (p *Parser) parseStructDefinition() ast.Expression {
	structDef := &ast.StructDefinition{Token: p.curToken}
	structDef.Members = []*ast.VarDeclStatement{}

	p.nextToken() // consume STRUCT

	for !p.curTokenIs(token.END_STRUCT) && !p.curTokenIs(token.EOF) {
		member := p.parseStructMember()
		if member != nil {
			structDef.Members = append(structDef.Members, member)
		}
		p.nextToken() // Consume semicolon to advance to the next member or END_STRUCT
	}

	if !p.curTokenIs(token.END_STRUCT) {
		p.peekError(token.END_STRUCT)
		return nil
	}

	return structDef
}

func (p *Parser) parseVarDeclarations(endToken token.TokenType) []*ast.VarDeclStatement {
	varDecls := []*ast.VarDeclStatement{}

	isConstant := false
	isRetain := false
	isNonRetain := false

	p.nextToken() // Consume VAR, VAR_INPUT, etc.

	if p.curTokenIs(token.CONSTANT) {
		isConstant = true
		p.nextToken() // consume CONSTANT
	} else if p.curTokenIs(token.RETAIN) {
		isRetain = true
		p.nextToken() // consume RETAIN
	} else if p.curTokenIs(token.NON_RETAIN) {
		isNonRetain = true
		p.nextToken() // consume NON_RETAIN
	}

	for !p.curTokenIs(endToken) && !p.curTokenIs(token.EOF) && !p.peekTokenIs(token.EOF) {
		// Each iteration parses one or more variables of the same type.
		// e.g., Var1, Var2 : INT;
		names := p.parseIdentifierList()

		var atDecl *ast.AtDeclaration
		if p.peekTokenIs(token.AT) {
			p.nextToken() // consume name, move to AT
			atDecl = p.parseAtDeclaration()
		}

		if !p.expectPeek(token.COLON) { // After this, curToken is ':'
			return nil
		}

		p.nextToken() // Consume ':', move to data type
		dataType := p.parseTypeSpecifier()
		if dataType == nil {
			return nil
		}
		// After parsing the type, we should be on the type token.
		// Now we advance to check for initialization or semicolon.

		var initialValue ast.Expression
		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // to ASSIGN
			p.nextToken() // to expression start
			initialValue = p.parseExpression(LOWEST)
		}

		for _, name := range names {
			decl := &ast.VarDeclStatement{
				Token:       name.Token,
				Name:        name,
				Location:    atDecl,
				DataType:    dataType,
				Value:       initialValue,
				IsConstant:  isConstant,
				IsRetain:    isRetain,
				IsNonRetain: isNonRetain,
			}
			varDecls = append(varDecls, decl)
		}

		// After parsing the declaration, we should be at the semicolon.
		if !p.expectPeek(token.SEMICOLON) {
			return nil // Expect and consume the semicolon
		}
		p.nextToken() // Move to the start of the next declaration or end token
	}
	return varDecls
}

func (p *Parser) parseAtDeclaration() *ast.AtDeclaration {
	if !p.curTokenIs(token.AT) {
		return nil
	}
	decl := &ast.AtDeclaration{Token: p.curToken}

	if !p.expectPeek(token.DIRECT_VAR) {
		return nil
	}

	// p.curToken is now the DIRECT_VAR token.
	// We don't call parseDirectVariable here because that's a prefix function.
	// We just build the AST node directly.
	loc := &ast.DirectVariable{Token: p.curToken, Address: strings.TrimPrefix(p.curToken.Literal, "%")}
	if loc == nil {
		return nil
	}
	decl.Location = loc
	return decl
}

func (p *Parser) parseDirectVariable() ast.Expression {
	dv := &ast.DirectVariable{Token: p.curToken}
	// The lexer now provides the full direct variable literal (e.g., "%IX1.0")
	// We just need to strip the leading '%' for the AST node's value.
	dv.Address = strings.TrimPrefix(p.curToken.Literal, "%")
	return dv
}

func (p *Parser) parseConfigurationDeclaration() ast.Statement {
	stmt := &ast.ConfigurationDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	for !p.curTokenIs(token.END_CONFIGURATION) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.RESOURCE:
			res := p.parseResourceDeclaration()
			if res != nil {
				stmt.Resources = append(stmt.Resources, res)
			}
		case token.VAR_GLOBAL:
			// Simplified, assumes one global block
			stmt.GlobalVars = append(stmt.GlobalVars, p.parseGlobalVarDeclStatement())
		}
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseResourceDeclaration() *ast.ResourceDeclaration {
	stmt := &ast.ResourceDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.ON) {
		return nil
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.ResourceType = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	for !p.curTokenIs(token.END_RESOURCE) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.TASK:
			task := p.parseTaskDeclaration()
			if task != nil {
				stmt.Tasks = append(stmt.Tasks, task)
			}
		case token.PROGRAM:
			progConfig := p.parseProgramConfiguration()
			if progConfig != nil {
				stmt.Programs = append(stmt.Programs, progConfig)
			}
		}
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseTaskDeclaration() *ast.TaskDeclaration {
	stmt := &ast.TaskDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	// Parse the task configuration (SINGLE, INTERVAL, PRIORITY)
	for !p.curTokenIs(token.RPAREN) && !p.curTokenIs(token.EOF) {
		p.nextToken() // move to the keyword
		switch p.curToken.Type {
		case token.SINGLE:
			if !p.expectPeek(token.ASSIGN) {
				return nil
			}
			p.nextToken()
			stmt.Single = p.parseExpression(LOWEST)
		case token.INTERVAL:
			if !p.expectPeek(token.ASSIGN) {
				return nil
			}
			p.nextToken()
			stmt.Interval = p.parseExpression(LOWEST)
		case token.PRIORITY:
			if !p.expectPeek(token.ASSIGN) {
				return nil
			}
			p.nextToken()
			stmt.Priority = p.parseExpression(LOWEST)
		}
		if p.curTokenIs(token.COMMA) {
			p.nextToken()
		}
	}

	if !p.curTokenIs(token.RPAREN) {
		return nil
	}

	return stmt
}

func (p *Parser) parseProgramConfiguration() *ast.ProgramConfiguration {
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

	// Program configuration must end with a semicolon
	p.expectPeek(token.SEMICOLON)

	return stmt
}

func (p *Parser) parseIdentifierList() []*ast.Identifier {
	list := []*ast.Identifier{}
	list = append(list, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	return list
}

func (p *Parser) parseTypeSpecifier() ast.Expression {
	if p.curTokenIs(token.ARRAY) {
		return p.parseArrayDefinition()
	}
	if p.isDataTypeToken(p.curToken) {
		return &ast.TypeSpecifier{Token: p.curToken}
	}

	p.errors = append(p.errors, fmt.Sprintf("expected a data type, got %s", p.curToken.Type))
	return nil
}

func (p *Parser) parseArrayDefinition() *ast.ArrayDefinition {
	def := &ast.ArrayDefinition{Token: p.curToken}

	if !p.expectPeek(token.LBRACKET) {
		return nil
	}
	p.nextToken() // consume '['

	def.Ranges = []ast.Expression{}
	for !p.curTokenIs(token.RBRACKET) {
		rangeExp := p.parseExpression(LOWEST)
		def.Ranges = append(def.Ranges, rangeExp)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume expression
			p.nextToken() // consume ','
		} else {
			p.nextToken() // consume expression
		}
	}

	if !p.curTokenIs(token.RBRACKET) {
		p.peekError(token.RBRACKET)
		return nil
	}

	if !p.expectPeek(token.OF) {
		return nil
	}
	p.nextToken() // consume 'OF', move to data type
	def.DataType = &ast.TypeSpecifier{Token: p.curToken}
	return def
}

func (p *Parser) isDataTypeToken(tok token.Token) bool {
	isBuiltIn := (tok.Type >= token.BOOL && tok.Type <= token.STRUCT) || tok.Type == token.STRING || tok.Type == token.WSTRING
	return isBuiltIn || tok.Type == token.IDENT
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	literal := p.curToken.Literal // e.g., "INT#10", "16#FF", "10"
	base := 10
	bitSize := 64 // Default to LINT/ULINT size

	// Handle typed literals (e.g., SINT#10, INT#20, DINT#30)
	// The lexer provides the full literal string, e.g., "SINT#10"
	if strings.Contains(literal, "#") {
		parts := strings.SplitN(literal, "#", 2)
		if len(parts) == 2 {
			typePart := parts[0]
			valuePart := parts[1]

			// Determine bitSize based on the typePart
			switch token.LookupIdent(strings.ToUpper(typePart)) {
			case token.SINT, token.USINT:
				bitSize = 8
			case token.INT, token.UINT:
				bitSize = 16
			case token.DINT, token.UDINT:
				bitSize = 32
			case token.LINT, token.ULINT:
				bitSize = 64
			}

			// Check for based literal within the value part (e.g., DINT#16#FF)
			if strings.Contains(valuePart, "#") {
				baseParts := strings.SplitN(valuePart, "#", 2)
				if len(baseParts) == 2 {
					parsedBase, err := strconv.Atoi(baseParts[0])
					if err == nil {
						base = parsedBase
					}
					valuePart = baseParts[1]
				}
			}
			literal = valuePart
		}
	} else if strings.Contains(literal, "#") {
		// Handle non-typed based literals like 16#FF, 8#77, 2#1011
		parts := strings.SplitN(literal, "#", 2)
		if len(parts) == 2 {
			parsedBase, err := strconv.Atoi(parts[0])
			if err == nil {
				base = parsedBase
			}
			literal = parts[1]
		}
	}
	literal = strings.ReplaceAll(literal, "_", "") // Remove underscores
	value, err := strconv.ParseInt(literal, base, bitSize)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

func (p *Parser) parseRealLiteral() ast.Expression {
	lit := &ast.RealLiteral{Token: p.curToken}

	literal := p.curToken.Literal
	// Handle typed literals like REAL#1.23 or LREAL#1.23E-4
	if strings.Contains(literal, "#") {
		// The lexer should have already captured the full literal including exponent.
		// We just need to extract the value part.
		parts := strings.SplitN(literal, "#", 2)
		literal = parts[1]
	}

	// Default to REAL (32-bit) unless LREAL is specified.
	bitSize := 32
	if p.curToken.Type == token.LREAL {
		bitSize = 64
	}
	lit.Precision = bitSize
	literal = strings.ReplaceAll(literal, "_", "") // Remove underscores

	value, err := strconv.ParseFloat(literal, bitSize)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as real", literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseBitStringLiteral() ast.Expression {
	lit := &ast.BitStringLiteral{Token: p.curToken}

	var width int
	switch p.curToken.Type {
	case token.BYTE:
		width = 8
	case token.WORD:
		width = 16
	case token.DWORD:
		width = 32
	case token.LWORD:
		width = 64
	default:
		// This case should ideally not be reached if prefix functions are registered correctly.
		p.errors = append(p.errors, fmt.Sprintf("unknown bitstring type: %s", p.curToken.Type))
		return nil
	}
	lit.Width = width

	literal := p.curToken.Literal // e.g., "BYTE#16#FF" or "WORD#FF"

	// Extract the value part after the first '#'
	parts := strings.SplitN(literal, "#", 2)
	if len(parts) < 2 {
		p.errors = append(p.errors, fmt.Sprintf("invalid bitstring literal format: %q", literal))
		return nil
	}
	valuePart := parts[1] // e.g., "16#FF" or "FF"

	var base int = 16 // Default base for bit strings if not specified, as per IEC 61131-3
	var valueStr string = valuePart

	// Check if the value part itself contains a base (e.g., "16#FF")
	if strings.Contains(valuePart, "#") {
		valueParts := strings.SplitN(valuePart, "#", 2)
		if len(valueParts) == 2 {
			parsedBase, err := strconv.Atoi(valueParts[0])
			if err != nil {
				p.errors = append(p.errors, fmt.Sprintf("invalid base in bitstring literal: %q", valueParts[0]))
				return nil
			}
			base = parsedBase
			valueStr = valueParts[1]
		}
	}

	// Remove underscores from the value string before parsing
	valueStr = strings.ReplaceAll(valueStr, "_", "")

	val, err := strconv.ParseUint(valueStr, base, width)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as %s (base %d, width %d): %s", valueStr, p.curToken.Type, base, width, err.Error())
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = val
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseTimeDateLiteral() ast.Expression {
	literal := p.curToken.Literal
	parts := strings.SplitN(literal, "#", 2)
	if len(parts) != 2 {
		p.errors = append(p.errors, fmt.Sprintf("invalid time/date literal format: %q", literal))
		return nil
	}
	valuePart := parts[1]

	switch p.curToken.Type {
	case token.TIME:
		// Regex to validate IEC 61131-3 duration format.
		// It allows for optional days, hours, minutes, seconds, and milliseconds.
		// It also handles underscores between units.
		// Example valid: 2d_5h_30m_10s_500ms, 10.5s, 500ms
		validTimeRegex := regexp.MustCompile(`^(((\d+d_?)?(\d+h_?)?(\d+m_?)?(\d+(\.\d+)?s_?)?(\d+ms)?)|(\d+(\.\d+)?s))$`)
		if !validTimeRegex.MatchString(strings.ToLower(valuePart)) {
			p.errors = append(p.errors, fmt.Sprintf("invalid TIME literal format: %q", valuePart))
			return nil
		}
		return &ast.TimeLiteral{Token: p.curToken, Value: literal}

	case token.DATE:
		// Use Go's time parsing with a strict layout.
		_, err := time.Parse("2006-01-02", valuePart)
		if err != nil {
			p.errors = append(p.errors, fmt.Sprintf("invalid DATE literal format: %q, expected YYYY-MM-DD", valuePart))
			return nil
		}
		return &ast.DateLiteral{Token: p.curToken, Value: literal}

	case token.TIME_OF_DAY:
		// Use Go's time parsing with a strict layout.
		_, err := time.Parse("15:04:05", valuePart)
		if err != nil {
			// Allow for fractional seconds
			_, err2 := time.Parse("15:04:05.999999999", valuePart)
			if err2 != nil {
				p.errors = append(p.errors, fmt.Sprintf("invalid TIME_OF_DAY literal format: %q, expected HH:MM:SS", valuePart))
				return nil
			}
		}
		return &ast.TimeOfDayLiteral{Token: p.curToken, Value: literal}

	case token.DATE_AND_TIME:
		// Use Go's time parsing with a strict layout.
		_, err := time.Parse("2006-01-02-15:04:05", valuePart)
		if err != nil {
			// Allow for fractional seconds
			_, err2 := time.Parse("2006-01-02-15:04:05.999999999", valuePart)
			if err2 != nil {
				p.errors = append(p.errors, fmt.Sprintf("invalid DATE_AND_TIME literal format: %q, expected YYYY-MM-DD-HH:MM:SS", valuePart))
				return nil
			}
		}
		return &ast.DateAndTimeLiteral{Token: p.curToken, Value: literal}
	}

	// This should not be reached if the prefix functions are registered correctly.
	p.errors = append(p.errors, fmt.Sprintf("no parsing function for time/date literal type %s", p.curToken.Type))
	return nil
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseMemberAccessExpression(left ast.Expression) ast.Expression {
	exp := &ast.MemberAccessExpression{
		Token:  p.curToken,
		Struct: left,
	}

	// We expect an identifier after the dot.
	if !p.expectPeek(token.IDENT) {
		return nil
	}

	exp.Member = &ast.Identifier{
		Token: p.curToken,
		Value: p.curToken.Literal,
	}

	return exp
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	ifStmt := &ast.IfStatement{Token: p.curToken}

	p.nextToken()
	ifStmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.THEN) {
		return nil
	}

	p.nextToken() // consume THEN
	ifStmt.Consequence = p.parseBlockStatementForIf()

	// Keep track of the current statement for chaining ELSIF
	current := ifStmt

	// Loop to handle all ELSIF clauses
	for p.curTokenIs(token.ELSIF) {
		newIf := &ast.IfStatement{Token: p.curToken}
		p.nextToken()
		newIf.Condition = p.parseExpression(LOWEST)

		if !p.expectPeek(token.THEN) {
			return nil
		}

		p.nextToken() // consume THEN
		newIf.Consequence = p.parseBlockStatementForIf()

		current.Alternative = newIf
		current = newIf
	}

	// Handle the final ELSE clause
	if p.curTokenIs(token.ELSE) {
		p.nextToken() // Consume ELSE
		current.Alternative = p.parseBlockStatementForIf()
	}

	// The last token should be END_IF
	if !p.curTokenIs(token.END_IF) {
		p.peekError(token.END_IF)
		p.nextToken() // Consume the END_IF to avoid infinite loop if error recovery is attempted
		return nil
	}

	return ifStmt
}

func (p *Parser) parseForStatement() ast.Statement {
	stmt := &ast.ForLoopStatement{Token: p.curToken}

	// FOR <Identifier>
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Identifier = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// :=
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	// <start_value>
	p.nextToken()
	stmt.StartValue = p.parseExpression(LOWEST)

	// TO
	if !p.expectPeek(token.TO) {
		return nil
	}

	p.nextToken() // Move to the start of the EndValue expression
	stmt.EndValue = p.parseExpression(LOWEST)

	// [BY <step_value>]
	if p.peekTokenIs(token.BY) {
		p.nextToken() // Consume the expression before BY
		p.nextToken() // Consume BY, move to expression start
		stmt.StepValue = p.parseExpression(LOWEST)
	}

	// DO
	if !p.expectPeek(token.DO) {
		return nil
	}

	p.nextToken() // Consume DO, move to start of body

	stmt.Body = p.parseBlockStatementUntil(token.END_FOR)

	// After parsing the body, p.curToken should be END_FOR. We need to consume it.
	// parseBlockStatementUntil leaves us on END_FOR, so we just need to consume it.
	p.nextToken()

	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	p.nextToken() // Consume WHILE
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.DO) {
		return nil
	}
	p.nextToken() // Consume DO, move to the start of the block.
	stmt.Body = p.parseBlockStatementUntil(token.END_WHILE)

	// parseBlockStatementWhileLoop leaves us on END_WHILE, so we just need to consume it.
	if !p.curTokenIs(token.END_WHILE) {
		p.peekError(token.END_WHILE)
		return nil
	}

	return stmt
}

func (p *Parser) parseRepeatStatement() ast.Statement {
	stmt := &ast.RepeatStatement{Token: p.curToken}

	p.nextToken() // consume REPEAT

	stmt.Body = p.parseBlockStatementRepeatLoop()

	if !p.curTokenIs(token.UNTIL) {
		return nil // error already reported by parseBlockStatementRepeatLoop
	}

	p.nextToken() // consume UNTIL
	stmt.Condition = p.parseExpression(LOWEST)

	p.expectPeek(token.END_REPEAT)
	return stmt
}

func (p *Parser) parseCaseStatement() ast.Statement {
	stmt := &ast.CaseStatement{Token: p.curToken}

	p.nextToken()
	stmt.Expression = p.parseExpression(LOWEST)

	if !p.expectPeek(token.OF) {
		return nil
	}

	p.nextToken()

	// Parse case branches
	for !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_CASE) && !p.curTokenIs(token.EOF) && !p.peekTokenIs(token.END_CASE) {
		branch := &ast.CaseBranch{Token: p.curToken}
		branch.Values = []ast.Expression{}

		// Parse comma-separated values
		for {
			branch.Values = append(branch.Values, p.parseExpression(LOWEST))
			if !p.peekTokenIs(token.COMMA) {
				break
			}
			p.nextToken() // consume value
			p.nextToken() // consume comma
		}

		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken() // Move to the start of the statement

		branch.Consequence = p.parseStatement()
		stmt.Cases = append(stmt.Cases, branch)

		// After parsing a statement, we might be on a semicolon.
		// The next token should be the start of the next case label, ELSE, or END_CASE.
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken() // Move to the next token to check the loop condition
	}

	// Parse optional ELSE block
	if p.curTokenIs(token.ELSE) {
		p.nextToken() // consume ELSE
		stmt.Alternative = p.parseBlockStatementUntil(token.END_CASE)
	}

	if !p.curTokenIs(token.END_CASE) {
		p.peekError(token.END_CASE)
	}

	return stmt
}

func (p *Parser) parseBlockStatementForIf() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.ELSIF) && !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_IF) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		// parseStatement should leave us on the last token of the statement (e.g., ';')
		// so we advance to the next one to check the loop condition.
		p.nextToken()
	}

	return block
}

func (p *Parser) parsePoulDeclaration() ast.Statement {
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

func (p *Parser) parseAssignmentStatement() *ast.AssignmentStatement {
	stmt := &ast.AssignmentStatement{
		Token: p.curToken,
		Left:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	// curToken is now ASSIGN

	p.nextToken() // Move to the start of the value expression
	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseFunctionDeclaration() ast.Statement {
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

func (p *Parser) parseFunctionBlockDeclaration() ast.Statement {
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

	// After var blocks, we have the body
	body := &ast.BlockStatement{Token: p.curToken}
	body.Statements = []ast.Statement{}
	for !p.curTokenIs(token.END_FUNCTION_BLOCK) && !p.curTokenIs(token.EOF) {
		s := p.parseStatement()
		if s != nil {
			body.Statements = append(body.Statements, s)
		}
		p.nextToken()
	}
	stmt.Body = body

	return stmt
}

func (p *Parser) parseProgramDeclaration() ast.Statement {
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

	// After var blocks, we have the body
	body := &ast.BlockStatement{Token: p.curToken}
	body.Statements = []ast.Statement{}
	for !p.curTokenIs(token.END_PROGRAM) && !p.curTokenIs(token.EOF) {
		s := p.parseStatement()
		body.Statements = append(body.Statements, s)
		p.nextToken()
	}
	stmt.Body = body

	return stmt
}

// parseVarBlock is a helper to parse a VAR...END_VAR block and return the declarations.
func (p *Parser) parseVarBlock(blockType token.TokenType) []*ast.VarDeclStatement {
	if !p.curTokenIs(blockType) {
		return nil
	}
	// We are at the start of a VAR block, parseVarDeclarations expects to be after the block token
	decls := p.parseVarDeclarations(token.END_VAR)
	p.nextToken() // Consume END_VAR
	return decls
}

func (p *Parser) parseActionStatement() ast.Statement {
	stmt := &ast.ActionStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected action name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken() // Consume the action name identifier

	stmt.Body = p.parseBlockStatementUntil(token.END_ACTION)

	if !p.curTokenIs(token.END_ACTION) {
		p.peekError(token.END_ACTION)
	}
	return stmt
}

func (p *Parser) parseTransitionStatement() ast.Statement {
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
	return p.parseStep(false)
}

func (p *Parser) parseInitialStepStatement() ast.Statement {
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

	stmt.Actions = []*ast.ActionAssociation{}
	for !p.curTokenIs(token.END_STEP) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.LPAREN) {
			assoc := p.parseActionAssociation()
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
		p.peekError(token.END_STEP)
		return nil
	}

	return stmt
}

func (p *Parser) parseActionAssociation() *ast.ActionAssociation {
	assoc := &ast.ActionAssociation{
		Token:      p.curToken,
		ActionName: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken() // consume (

	// For now, we'll assume a simple qualifier, like (N)
	assoc.Qualifier = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return assoc
}

func (p *Parser) parseBlockStatementUntil(end token.TokenType) *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}
	//p.nextToken()

	for !p.curTokenIs(end) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parseBlockStatementForLoop() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.END_FOR) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken() // <-- This is the problem
	}
	// END_FOR is consumed by parseForStatement
	return block
}

func (p *Parser) parseBlockStatementRepeatLoop() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.UNTIL) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	if !p.curTokenIs(token.UNTIL) {
		p.peekError(token.UNTIL)
	}
	return block
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()

	// Check for named arguments (IDENT := or IDENT =>)
	if p.curTokenIs(token.IDENT) && (p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.ARROW)) {
		list = append(list, p.parseNamedArgument())
	} else {
		list = append(list, p.parseExpression(LOWEST))
	}

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		if p.curTokenIs(token.IDENT) && (p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.ARROW)) {
			list = append(list, p.parseNamedArgument())
		} else {
			list = append(list, p.parseExpression(LOWEST))
		}
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseNamedArgument() ast.Expression {
	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken() // move to := or =>

	if p.curTokenIs(token.ASSIGN) {
		arg := &ast.NamedArgument{Token: name.Token, Name: name}
		p.nextToken() // move to expression
		arg.Value = p.parseExpression(LOWEST)
		return arg
	}

	if p.curTokenIs(token.ARROW) {
		arg := &ast.OutputArgument{Token: p.curToken, Source: name}
		p.nextToken() // move to expression
		arg.Target = p.parseExpression(LOWEST)
		return arg
	}

	// Should not happen due to checks in parseExpressionList
	return nil
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.curToken}

	array.Elements = p.parseExpressionList(token.RBRACKET)

	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseHashLiteral() ast.Expression {
	hash := &ast.HashLiteral{Token: p.curToken}
	hash.Pairs = make(map[ast.Expression]ast.Expression)

	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		key := p.parseExpression(LOWEST)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		hash.Pairs[key] = value

		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return hash
}

func (p *Parser) parseMacroLiteral() ast.Expression {
	lit := &ast.MacroLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
