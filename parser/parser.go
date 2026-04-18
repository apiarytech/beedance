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
	"strconv"
	"strings"

	"beedance/ast"
	"beedance/lexer"
	"beedance/token"
)

const (
	_ int = iota
	LOWEST
	LOGICAL_OR  // OR, XOR, NOR
	LOGICAL_AND // AND, NAND
	EXPONENT    // **
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
)

var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,      // =
	token.NEQ:      EQUALS,      // <>
	token.LT:       LESSGREATER, // <
	token.GT:       LESSGREATER, // >
	token.LE:       LESSGREATER, // <=
	token.GE:       LESSGREATER, // >=
	token.PLUS:     SUM,         // +
	token.MINUS:    SUM,         // -
	token.SLASH:    PRODUCT,     // /
	token.ASTERISK: PRODUCT,     // *
	token.LPAREN:   CALL,        // (
	token.LBRACKET: INDEX,       // [
	token.AND:      LOGICAL_AND,
	token.NAND:     LOGICAL_AND,
	token.OR:       LOGICAL_OR,
	token.XOR:      LOGICAL_OR,
	token.NOR:      LOGICAL_OR,
	token.EXPONENT: EXPONENT,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken   token.Token
	peekToken  token.Token
	peekToken2 token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.STRING_LITERAL, p.parseStringLiteral)
	p.registerPrefix(token.WSTRING_LITERAL, p.parseWStringLiteral)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)
	p.registerPrefix(token.MACRO, p.parseMacroLiteral)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.TIME, p.parseTimeLiteral)
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

	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken() // curToken
	p.nextToken() // peekToken
	p.nextToken() // peekToken2

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
	p.peekToken2 = p.l.NextToken()

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
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.VAR:
		return p.parseVarDeclStatement()
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
	case token.EXIT:
		return &ast.ExitStatement{Token: p.curToken}
	default:
		return p.parseExpressionStatement()
	}
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

func (p *Parser) parseStructMember() *ast.VarDeclStatement {
	// This is similar to parseVarDeclStatement but for struct members,
	// which don't start with the VAR keyword.
	stmt := &ast.VarDeclStatement{Token: p.curToken}

	if !p.curTokenIs(token.IDENT) {
		p.errors = append(p.errors, "expected member name (identifier)")
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken() // Move to the data type token

	// Check if the current token is a valid data type
	if !p.isDataTypeToken(p.curToken) {
		p.errors = append(p.errors, fmt.Sprintf("expected a data type for member, got %s", p.curToken.Literal))
		return nil
	}

	stmt.DataType = &ast.TypeSpecifier{Token: p.curToken}

	// Check for optional initial value assignment (:=)
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // consume ':='
		p.nextToken() // consume expression start
		stmt.Value = p.parseExpression(LOWEST)
	}

	p.expectPeek(token.SEMICOLON)
	return stmt
}

func (p *Parser) parseGlobalVarDeclStatement() *ast.GlobalVarDeclaration {
	stmt := &ast.GlobalVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	// The parseVarDeclarations leaves us at the END_VAR token.

	return stmt
}

func (p *Parser) parseExternalVarDeclStatement() *ast.ExternalVarDeclaration {
	stmt := &ast.ExternalVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	// The parseVarDeclarations leaves us at the END_VAR token.

	return stmt
}

func (p *Parser) parseAccessVarDeclStatement() *ast.AccessVarDeclaration {
	stmt := &ast.AccessVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	// The parseVarDeclarations leaves us at the END_VAR token.

	return stmt
}

func (p *Parser) parseTempVarDeclStatement() *ast.TempVarDeclaration {
	stmt := &ast.TempVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	// The parseVarDeclarations leaves us at the END_VAR token.

	return stmt
}

func (p *Parser) parseConfigVarDeclStatement() *ast.ConfigVarDeclaration {
	stmt := &ast.ConfigVarDeclaration{Token: p.curToken}

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	// The parseVarDeclarations leaves us at the END_VAR token, so we expect it.
	p.expectPeek(token.END_VAR)

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
		p.nextToken()
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
	decl.DataType = p.parseExpression(LOWEST)
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
		p.nextToken()
	}

	p.expectPeek(token.END_STRUCT)
	return structDef
}

func (p *Parser) parseVarDeclarations(endToken token.TokenType) []*ast.VarDeclStatement {
	varDecls := []*ast.VarDeclStatement{}

	p.nextToken() // consume VAR, VAR_INPUT, etc.

	for !p.curTokenIs(endToken) && !p.curTokenIs(token.EOF) {
		stmt := p.parseVarDeclStatement()
		if stmt != nil {
			varDecls = append(varDecls, stmt)
		}
		p.nextToken()
		if p.curTokenIs(endToken) {
			break
		}
	}

	return varDecls
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

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

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

	literal := p.curToken.Literal
	base := 10

	if strings.Contains(literal, "#") {
		parts := strings.Split(literal, "#")
		base, _ = strconv.Atoi(parts[0])
		literal = parts[1]
	}

	value, err := strconv.ParseInt(literal, base, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseWStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseTimeLiteral() ast.Expression {
	return &ast.TimeLiteral{Token: p.curToken, Value: p.curToken.Literal}
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

	// <end_value>
	p.nextToken()
	stmt.EndValue = p.parseExpression(LOWEST)

	// [BY <step_value>]
	if p.peekTokenIs(token.BY) {
		p.nextToken() // consume BY
		p.nextToken() // consume expression start
		stmt.StepValue = p.parseExpression(LOWEST)
	}

	// DO
	if !p.expectPeek(token.DO) {
		return nil
	}

	p.nextToken() // Move to the start of the block
	stmt.Body = p.parseBlockStatementForLoop()
	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	p.nextToken() // consume WHILE
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.DO) {
		return nil
	}

	p.nextToken() // Move to the start of the block
	stmt.Body = p.parseBlockStatementWhileLoop()
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
	for !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_CASE) && !p.curTokenIs(token.EOF) {
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

		p.nextToken()
	}

	// Parse optional ELSE block
	if p.curTokenIs(token.ELSE) {
		p.nextToken() // consume ELSE
		stmt.Alternative = &ast.BlockStatement{Token: p.curToken}
		stmt.Alternative.Statements = []ast.Statement{}
		for !p.curTokenIs(token.END_CASE) && !p.curTokenIs(token.EOF) {
			statement := p.parseStatement()
			stmt.Alternative.Statements = append(stmt.Alternative.Statements, statement)
			p.nextToken()
		}
	}

	return stmt
}

func (p *Parser) parseBlockStatementForIf() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}
	p.nextToken()

	for !p.curTokenIs(token.ELSIF) && !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_IF) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
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

func (p *Parser) parseFunctionDeclaration() ast.Statement {
	// This will parse a named FUNCTION ... END_FUNCTION block
	// For now, we'll treat it like a function literal expression statement
	// for AST representation, but a more specific AST node would be better.
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected function name
	}
	// We don't have a name field in FunctionLiteral, so we'll just consume it.

	if !p.expectPeek(token.COLON) {
		return nil // Expected return type separator
	}

	p.nextToken() // Consume the return type

	// TODO: Parse VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR blocks

	lit.Body = p.parseBlockStatement() // Simplified body parsing

	return &ast.ExpressionStatement{Expression: lit}
}

func (p *Parser) parseFunctionBlockDeclaration() ast.Statement {
	stmt := &ast.FunctionBlockDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected function block name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	// Parse variable declaration blocks
	for p.curTokenIs(token.VAR_INPUT) || p.curTokenIs(token.VAR_OUTPUT) || p.curTokenIs(token.VAR_IN_OUT) || p.curTokenIs(token.VAR) {
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarDeclarations(token.END_VAR)...)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarDeclarations(token.END_VAR)...)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarDeclarations(token.END_VAR)...)
		case token.VAR:
			stmt.Vars = append(stmt.Vars, p.parseVarDeclarations(token.END_VAR)...)
		}
		p.nextToken()
	}

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

	// Parse variable declaration blocks
	for p.curTokenIs(token.VAR_INPUT) || p.curTokenIs(token.VAR_OUTPUT) || p.curTokenIs(token.VAR_IN_OUT) || p.curTokenIs(token.VAR) {
		switch p.curToken.Type {
		case token.VAR_INPUT:
			stmt.VarInputs = append(stmt.VarInputs, p.parseVarDeclarations(token.END_VAR)...)
		case token.VAR_OUTPUT:
			stmt.VarOutputs = append(stmt.VarOutputs, p.parseVarDeclarations(token.END_VAR)...)
		case token.VAR_IN_OUT:
			stmt.VarInOuts = append(stmt.VarInOuts, p.parseVarDeclarations(token.END_VAR)...)
		case token.VAR:
			stmt.Vars = append(stmt.Vars, p.parseVarDeclarations(token.END_VAR)...)
		}
		p.nextToken()
	}

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

func (p *Parser) parseActionStatement() ast.Statement {
	stmt := &ast.ActionStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected action name
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	stmt.Body = p.parseBlockStatementUntil(token.END_ACTION)

	if !p.curTokenIs(token.END_ACTION) {
		p.peekError(token.END_ACTION)
	}
	return stmt
}

func (p *Parser) parseTransitionStatement() ast.Statement {
	p.errors = append(p.errors, "parsing TRANSITION statements is not yet implemented")
	return nil
}

func (p *Parser) parseStepStatement() ast.Statement {
	p.errors = append(p.errors, "parsing STEP statements is not yet implemented")
	return nil
}

func (p *Parser) parseInitialStepStatement() ast.Statement {
	p.errors = append(p.errors, "parsing INITIAL_STEP statements is not yet implemented")
	return nil
}

func (p *Parser) parseBlockStatementUntil(end token.TokenType) *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}
	p.nextToken()

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
		p.nextToken()
	}
	if !p.curTokenIs(token.END_FOR) {
		p.peekError(token.END_FOR)
	}
	return block
}

func (p *Parser) parseBlockStatementWhileLoop() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.END_WHILE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	if !p.curTokenIs(token.END_WHILE) {
		p.peekError(token.END_WHILE)
	}
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
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
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
