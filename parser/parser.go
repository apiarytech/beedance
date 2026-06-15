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
	"regexp"
)

const (
	_ int = iota
	LOWEST
	ASSIGN      // :=
	LOGICAL_OR  // OR
	LOGICAL_XOR // XOR
	LOGICAL_AND // AND
	EQUALS      // =, <>
	LESSGREATER // >, <, <=, >=
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

// parseError is a custom error type used for panicking during parsing errors.
// This allows us to unwind the stack to a recovery point (e.g., ParseProgram)
// without cluttering every parsing function with error checks.
type parseError struct {
	msg string
}

func (e *parseError) Error() string { return e.msg }

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// B.3.1 Expressions - prefix functions
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)

	// Literals (B.1.2)
	// Generic literals are handled by the lexer and parsed as identifiers or numbers.
	// Typed literals (e.g., INT#10) are handled by the HASH infix parser.
	p.registerPrefix(token.STRING_LITERAL, p.parseStringLiteral)
	p.registerPrefix(token.WSTRING_LITERAL, p.parseStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)

	// Variables (B.1.4)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.DIRECT_VAR, p.parseDirectVariable)

	// Numeric literals (which can also be part of a typed literal)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.REAL, p.parseRealLiteral)
	p.registerPrefix(token.LREAL, p.parseRealLiteral)
	// Time and Date keywords can start a typed literal expression (e.g., T#5s).
	// We treat them like identifiers at this stage.
	p.registerPrefix(token.TIME, p.parseDataTypeKeyword)
	p.registerPrefix(token.DATE, p.parseDataTypeKeyword)
	p.registerPrefix(token.TIME_OF_DAY, p.parseDataTypeKeyword)
	p.registerPrefix(token.DATE_AND_TIME, p.parseDataTypeKeyword)

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
	p.registerInfix(token.HASH, p.parseTypedLiteral)

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
		// Error Recovery: Skip tokens until we find a potential statement boundary.
		for !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.EOF) && !isStatementEndToken(p.curToken.Type) {
			p.curToken = p.peekToken
			p.peekToken = p.l.NextToken()
		}

	}
}

func (p *Parser) isIdentFollowedByColon() bool {
	// This helper checks if the current token is an identifier
	// and is immediately followed by a colon, like a label.
	return p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON)
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
		// If we expected a semicolon, we can often recover by just pretending it was there
		// and continuing. For other tokens, this might not be safe.
		if t == token.SEMICOLON {
			// We don't advance the token, we just allow parsing to continue from the current position
			// as if the semicolon was optional.
			return true // "Recovered"
		}
		// For other errors, we might want to skip until the next semicolon or block end.
		// for !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.EOF) {
		// 	p.nextToken()
		// }
		return false
	}
}

// parseDateTimeIdentifier consumes tokens to build a single identifier for date/time literals.
// It handles constructs like `5s`, `5m_10s`, `2026-05-21`, and `14:30:00.5`.
func (p *Parser) parseDateTimeIdentifier() ast.Expression {
	defer untrace(trace("parseDateTimeIdentifier"))
	startToken := p.curToken
	var builder strings.Builder

	// Consume tokens that can be part of a date/time literal value.
	// This loop continues as long as the tokens are numbers, identifiers (for units like 's', 'ms'),
	// or separators like '-', ':', and '.'.
	for {
		if p.curTokenIs(token.INT) || p.curTokenIs(token.REAL) || p.curTokenIs(token.IDENT) ||
			p.curTokenIs(token.MINUS) || p.curTokenIs(token.COLON) || p.curTokenIs(token.DOT) {
			builder.WriteString(p.curToken.Literal)

			// Peek ahead to see if the next token is also part of the literal.
			if !(p.peekTokenIs(token.INT) || p.peekTokenIs(token.REAL) || p.peekTokenIs(token.IDENT) ||
				p.peekTokenIs(token.MINUS) || p.peekTokenIs(token.COLON) || p.peekTokenIs(token.DOT)) {
				break
			}
			p.nextToken()
		} else {
			break // Not a valid date/time token, so we stop.
		}
	}

	combinedLiteral := builder.String()
	combinedToken := token.Token{
		Type:    token.IDENT,
		Literal: combinedLiteral,
		Row:     startToken.Row,
		Column:  startToken.Column,
		Pos:     startToken.Pos,
	}
	return &ast.Identifier{Token: combinedToken, Value: combinedLiteral}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	p.specificError("expected next token to be %s, got %s instead", t, p.peekToken.Type)
}

// synchronizeParser advances the parser's tokens until it finds a likely start of a new statement.
// This is used after a parsing panic to get the parser back to a stable state.
func (p *Parser) synchronizeParser() {
	for !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.EOF) && !isStatementStartKeyword(p.curToken.Type) {
		p.nextToken()
	}
	// If we stopped on a semicolon, consume it to move to the next statement.
	if p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

// specificError creates a formatted error message with line and column numbers.
func (p *Parser) specificError(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	p.errors = append(p.errors, fmt.Sprintf("%s at row %d, column %d",
		msg, p.peekToken.Row, p.peekToken.Column))
}

// currentError creates a formatted error message using the current token's position.
func (p *Parser) currentError(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	p.errors = append(p.errors, fmt.Sprintf("%s at row %d, column %d",
		msg, p.curToken.Row, p.curToken.Column))
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) ParseProgram() *ast.Program {
	defer untrace(trace("ParseProgram"))
	program := &ast.Program{}
	program.Statements = []ast.Statement{}
	lastPosition := -1 // Track the last token position to detect infinite loops

	for !p.curTokenIs(token.EOF) {
		tracePrint(fmt.Sprintf("Current Token: %s (%s) at %d:%d", p.curToken.Literal, p.curToken.Type, p.curToken.Row, p.curToken.Column))

		// Infinite loop detection: if the token position hasn't changed since the last iteration, we're stuck.
		if p.curToken.Pos == lastPosition {
			p.errors = append(p.errors, fmt.Sprintf("Infinite loop detected at row %d, column %d. Parser is not advancing past token %s (%s).", p.curToken.Row, p.curToken.Column, p.curToken.Type, p.curToken.Literal))
			break // Break out of the loop to prevent the program from hanging.
		}
		lastPosition = p.curToken.Pos

		// Use defer-recover to catch parsing panics for a single statement.
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Check if it's our custom parseError or another runtime panic.
					if _, ok := r.(*parseError); !ok {
						// Re-panic if it's not our expected parseError
						panic(r)
					}
					// An error has already been logged by the panic source (e.g., p.peekError or p.currentError).
					// We just need to synchronize the parser to a safe point.
					p.synchronizeParser()
				}
			}()

			stmt := p.parseStatement()
			if stmt != nil {
				program.Statements = append(program.Statements, stmt)
			}
			// After a successful parse, advance to the next token.
			p.nextToken()
		}()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	defer untrace(trace("parseStatement"))
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
	case token.TRANSITION:
		return p.parseTransitionStatement()
	case token.STEP:
		return p.parseStepStatement()
	case token.INITIAL_STEP:
		return p.parseInitialStepStatement()
	case token.CONFIGURATION:
		return p.parseConfigurationDeclaration() // No semicolon expected after this block
	case token.ACTION:
		return p.parseActionStatement()
	case token.PROGRAM, token.FUNCTION, token.FUNCTION_BLOCK:
		return p.parsePoulDeclaration()
	case token.EXIT:
		return p.parseExitStatement()
	case token.IDENT:
		if p.peekTokenIs(token.ASSIGN) {
			return p.parseAssignmentStatement()
		} else {
			return p.parseExpressionStatement()
		}
	default:
		return p.parseExpressionStatement()
	}
}
func (p *Parser) parseExitStatement() *ast.ExitStatement {
	stmt := &ast.ExitStatement{Token: p.curToken}

	// An EXIT statement must be followed by a semicolon.
	if !p.expectPeek(token.SEMICOLON) {
		// Don't return nil, allow recovery. The main loop will advance.
	}
	// The expectPeek call already consumed the semicolon.
	// The main loop will call nextToken() to move to the next statement.
	return stmt
}
func (p *Parser) parseSingleVarDecl() *ast.VarDeclStatement {
	defer untrace(trace("parseSingleVarDecl"))
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
	defer untrace(trace("parseVarDeclStatement"))
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

	p.expectPeek(token.SEMICOLON) // Consume the semicolon
	return stmt
}

func (p *Parser) parseVarBlockStatement() *ast.VarBlockDeclaration {
	defer untrace(trace("parseVarBlockStatement"))
	stmt := &ast.VarBlockDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR

	stmt.Declarations = p.parseVarDeclarations(token.END_VAR)

	// After parsing the declarations, we must be at the END_VAR token.
	if !p.curTokenIs(token.END_VAR) {
		// The error is already reported by parseVarDeclarations, so we don't need to report it again.
		// Just ensure we don't advance past the token that should start the next statement.
		return stmt
	}

	return stmt
}

func (p *Parser) parseStructMember() *ast.VarDeclStatement {
	defer untrace(trace("parseStructMember"))
	// cspell:disable-next-line
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
		p.nextToken() // consume data type, curToken is now ':='
		p.nextToken() // consume ':=', move to expression start
		stmt.Value = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(token.SEMICOLON) { // Consume semicolon for a valid declaration
		// Allow recovery even if semicolon is missing
	}
	return stmt
}

func (p *Parser) parseGlobalVarDeclStatement() *ast.GlobalVarDeclaration {
	defer untrace(trace("parseGlobalVarDeclStatement"))
	stmt := &ast.GlobalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_GLOBAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseExternalVarDeclStatement() *ast.ExternalVarDeclaration {
	defer untrace(trace("parseExternalVarDeclStatement"))
	stmt := &ast.ExternalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_EXTERNAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseAccessVarDeclStatement() *ast.AccessVarDeclaration {
	defer untrace(trace("parseAccessVarDeclStatement"))
	stmt := &ast.AccessVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_ACCESS
	stmt.Vars = p.parseAccessDeclarations()
	return stmt
}

// parseAccessDeclarations is a specialized version of parseVarDeclarations for VAR_ACCESS blocks.
// It handles the READ_ONLY and READ_WRITE qualifiers per declaration.
func (p *Parser) parseAccessDeclarations() []*ast.VarDeclStatement {
	defer untrace(trace("parseAccessDeclarations"))
	varDecls := []*ast.VarDeclStatement{}

	for !p.curTokenIs(token.END_VAR) && !p.curTokenIs(token.EOF) {
		if isStatementStartKeyword(p.curToken.Type) {
			p.peekError(token.END_VAR)
			return varDecls
		}

		names := p.parseIdentifierList()

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken() // Consume ':', move to data type
		dataType := p.parseTypeSpecifier()
		if dataType == nil {
			return nil
		}
		p.nextToken() // Consume data type

		// Check for optional READ_ONLY or READ_WRITE
		var accessType string
		if p.curTokenIs(token.READ_ONLY) || p.curTokenIs(token.READ_WRITE) {
			accessType = p.curToken.Literal
			p.nextToken() // Consume the access type keyword
		}

		for _, name := range names {
			decl := &ast.VarDeclStatement{Token: name.Token, Name: name, DataType: dataType, AccessType: accessType}
			varDecls = append(varDecls, decl)
		}

		if !p.curTokenIs(token.SEMICOLON) {
			p.errors = append(p.errors, fmt.Sprintf("expected semicolon, got %s instead at row %d, column %d", p.curToken.Type, p.curToken.Row, p.curToken.Column))
			return nil
		}
		p.nextToken()
	}
	return varDecls
}

func (p *Parser) parseTempVarDeclStatement() *ast.TempVarDeclaration {
	defer untrace(trace("parseTempVarDeclStatement"))
	stmt := &ast.TempVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_TEMP

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseConfigVarDeclStatement() *ast.ConfigVarDeclaration {
	defer untrace(trace("parseConfigVarDeclStatement"))
	stmt := &ast.ConfigVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_CONFIG

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	return stmt
}

func (p *Parser) parseTypeBlockDeclaration() *ast.TypeBlockDeclaration {
	defer untrace(trace("parseTypeBlockDeclaration"))
	block := &ast.TypeBlockDeclaration{Token: p.curToken}
	block.Declarations = []*ast.TypeDeclaration{}

	p.nextToken() // consume TYPE

	for !p.curTokenIs(token.END_TYPE) && !p.curTokenIs(token.EOF) {
		decl := p.parseTypeDeclaration()
		if decl != nil {
			block.Declarations = append(block.Declarations, decl)
		}
		// p.parseTypeDeclaration consumes the semicolon, so we just advance.
		p.nextToken()
	}

	return block
}

func (p *Parser) parseTypeDeclaration() *ast.TypeDeclaration {
	defer untrace(trace("parseTypeDeclaration"))
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
	} else if p.curTokenIs(token.LPAREN) {
		decl.DataType = p.parseEnumDefinition()
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
	p.expectPeek(token.SEMICOLON)
	return decl
}

func (p *Parser) parseStructDefinition() ast.Expression {
	defer untrace(trace("parseStructDefinition"))
	structDef := &ast.StructDefinition{Token: p.curToken}
	structDef.Members = []*ast.VarDeclStatement{}

	p.nextToken() // consume STRUCT

	for !p.curTokenIs(token.END_STRUCT) && !p.curTokenIs(token.EOF) {
		member := p.parseStructMember()
		if member != nil {
			structDef.Members = append(structDef.Members, member)
		}
		p.nextToken() // parseStructMember consumed the ';', so advance to next token
	}

	if !p.curTokenIs(token.END_STRUCT) {
		p.specificError("missing 'END_STRUCT' for struct definition starting at row %d", structDef.Token.Row)
		return nil
	}

	return structDef
}

func (p *Parser) parseEnumDefinition() ast.Expression {
	defer untrace(trace("parseEnumDefinition"))
	enumDef := &ast.EnumDefinition{Token: p.curToken}
	enumDef.Values = []*ast.Identifier{}

	// Current token is '('. We expect a list of identifiers.
	p.nextToken() // consume '('

	// Parse the first identifier
	if p.curTokenIs(token.IDENT) {
		enumDef.Values = append(enumDef.Values, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
		p.nextToken()
	}

	// Parse subsequent identifiers separated by commas
	for p.curTokenIs(token.COMMA) {
		p.nextToken() // consume ','
		if p.curTokenIs(token.IDENT) {
			enumDef.Values = append(enumDef.Values, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
			p.nextToken()
		}
	}

	// After the loop, the current token should be the closing parenthesis.
	if !p.curTokenIs(token.RPAREN) {
		p.currentError("expected ')' to close enumeration, got %s instead", p.curToken.Type)
		return nil
	}
	return enumDef
}

func (p *Parser) parseVarDeclarations(endToken token.TokenType) []*ast.VarDeclStatement {
	defer untrace(trace(fmt.Sprintf("parseVarDeclarations (until %s)", endToken)))
	varDecls := []*ast.VarDeclStatement{}

	var isRetain bool
	var isNonRetain bool
	isConstant := false

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
	// Check for the optional CONSTANT keyword after VAR
	if p.curTokenIs(token.CONSTANT) {
		isConstant = true
		p.nextToken() // consume CONSTANT
	}

	for !p.curTokenIs(endToken) && !p.curTokenIs(token.EOF) && !p.peekTokenIs(token.EOF) {
		// Error Recovery: If we encounter a token that looks like the start of a new statement block,
		// assume END_VAR was missing and stop parsing this var block.
		if isStatementStartKeyword(p.curToken.Type) {
			// Report the missing end token, but do not advance the parser.
			// This allows the main loop to process the current token as the start of a new statement.
			p.currentError("expected next token to be %s, got %s instead", endToken, p.curToken.Type)
			return varDecls // Return what we have, leaving the parser on the new statement's keyword.
		}
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
				IsConstant:  isConstant, // Apply the block-level qualifier
				IsRetain:    isRetain,
				IsNonRetain: isNonRetain,
			}
			varDecls = append(varDecls, decl)
		}
		p.expectPeek(token.SEMICOLON)
		p.nextToken()
		// // After parsing the declaration, we should be at the semicolon.
		// if !p.peekTokenIs(token.IDENT) || isStatementStartKeyword(p.curToken.Type) {
		// 	// Report the missing end token, but do not advance the parser.
		// 	// This allows the main loop to process the current token as the start of a new statement.
		// 	p.currentError("expected next token to be %s, got %s instead", endToken, p.curToken.Type)
		// 	return varDecls // Return what we have, leaving the parser on the new statement's keyword.
		// } else {
		// 	p.nextToken() // Move to the start of the next declaration or end token
		// }
	}

	//p.nextToken() // Consume the endToken (e.g., END_VAR) to advance the parser
	return varDecls
}

func (p *Parser) parseAtDeclaration() *ast.AtDeclaration {
	defer untrace(trace("parseAtDeclaration"))
	// This function is called when p.curToken is AT.
	decl := &ast.AtDeclaration{Token: p.curToken}

	if !p.expectPeek(token.DIRECT_VAR) {
		return nil
	}

	dvExp := p.parseDirectVariable()
	dv, ok := dvExp.(*ast.DirectVariable)
	if !ok {
		p.currentError("expected a direct variable for AT declaration, but got %T", dvExp)
		return nil
	}

	decl.Location = dv
	return decl
}

func (p *Parser) parseDirectVariable() ast.Expression {
	defer untrace(trace("parseDirectVariable"))
	dv := &ast.DirectVariable{Token: p.curToken}
	// The lexer now provides the full direct variable literal (e.g., "%IX1.0")
	// We just need to strip the leading '%' for the AST node's value.
	dv.Address = strings.TrimPrefix(p.curToken.Literal, "%")
	return dv
}

func (p *Parser) parseConfigurationDeclaration() ast.Statement {
	defer untrace(trace("parseConfigurationDeclaration"))
	stmt := &ast.ConfigurationDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()

	for !p.curTokenIs(token.END_CONFIGURATION) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.VAR_GLOBAL:
			// A configuration can have global variable blocks.
			stmt.GlobalVars = append(stmt.GlobalVars, p.parseGlobalVarDeclStatement())
		case token.RESOURCE:
			res := p.parseResourceDeclaration()
			if res != nil {
				stmt.Resources = append(stmt.Resources, res)
			}
		case token.VAR_ACCESS:
			// Access variable declarations for communication.
			stmt.AccessVars = append(stmt.AccessVars, p.parseAccessVarDeclStatement())
		case token.VAR_CONFIG:
			// Instance-specific initializations.
			stmt.ConfigVars = append(stmt.ConfigVars, p.parseConfigVarDeclStatement())
		default:
			// If we encounter a token we don't recognize at this level, we advance past it to avoid an infinite loop.
			p.nextToken()
		}
	}
	if !p.curTokenIs(token.END_CONFIGURATION) {
		p.specificError("missing 'END_CONFIGURATION' for configuration starting at row %d", stmt.Token.Row)
		return nil
	}
	return stmt
}

func (p *Parser) parseResourceDeclaration() *ast.ResourceDeclaration {
	defer untrace(trace("parseResourceDeclaration"))
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
	defer untrace(trace("parseTaskDeclaration"))
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

	p.nextToken() // Consume ')'

	return stmt
}

func (p *Parser) parseIdentifierList() []*ast.Identifier {
	defer untrace(trace("parseIdentifierList"))
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
	defer untrace(trace("parseTypeSpecifier"))
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
	defer untrace(trace("parseArrayDefinition"))
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
		p.specificError("missing ']' in array definition")
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
	defer untrace(trace("parseReturnStatement"))
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	p.expectPeek(token.SEMICOLON) // Consume semicolon
	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	defer untrace(trace(fmt.Sprintf("parseExpression (precedence %d)", precedence)))
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
	defer untrace(trace("parseIdentifier"))
	// Check if this identifier is the start of a user-defined typed literal (e.g., "COLOR#RED").
	// This is a more direct way of parsing this construct than relying on the generic infix parser,
	// which helps avoid conflicts with keyword-based typed literals (e.g., T#5s).
	if p.peekTokenIs(token.HASH) {
		typeIdent := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		p.nextToken() // Consume the type identifier, curToken is now '#'
		p.nextToken() // Consume '#', curToken is now the value identifier (e.g., 'RED')

		valueIdent := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		return &ast.TypedLiteral{
			Token:    typeIdent.Token,
			TypeName: typeIdent.Value,
			Value:    valueIdent,
		}
	}
	// If not followed by '#', it's a simple identifier.
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseDataTypeKeyword() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))
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

func (p *Parser) parseTypedIntegerLiteral() ast.Expression {
	defer untrace(trace("parseTypedIntegerLiteral"))

	tok := p.curToken
	literal := tok.Literal
	base := 10
	var bitSize int
	var isUnsigned bool

	// Determine properties from token type
	switch tok.Type {
	case token.SINT:
		bitSize = 8
	case token.INT:
		bitSize = 16
	case token.DINT:
		bitSize = 32
	case token.LINT:
		bitSize = 64
	case token.USINT:
		bitSize, isUnsigned = 8, true
	case token.UINT:
		bitSize, isUnsigned = 16, true
	case token.UDINT:
		bitSize, isUnsigned = 32, true
	case token.ULINT:
		bitSize, isUnsigned = 64, true
	default:
		// Fallback to generic integer if called with a non-specific type
		return p.parseIntegerLiteral()
	}

	// The lexer provides the full literal, e.g., "SINT#10" or "UINT#16#FF"
	parts := strings.SplitN(literal, "#", 2)
	if len(parts) != 2 {
		p.currentError("invalid typed integer literal format: %q", literal)
		return nil
	}
	valuePart := parts[1]

	// Check for an explicit base in the value part
	if strings.Contains(valuePart, "#") {
		baseParts := strings.SplitN(valuePart, "#", 2)
		parsedBase, err := strconv.Atoi(baseParts[0])
		if err != nil {
			p.currentError("invalid base in typed integer literal: %q", baseParts[0])
			return nil
		}
		base = parsedBase
		valuePart = baseParts[1]
	}

	valuePart = strings.ReplaceAll(valuePart, "_", "")

	if isUnsigned {
		value, err := strconv.ParseUint(valuePart, base, bitSize)
		if err != nil {
			p.currentError("could not parse %q as unsigned integer (base %d, size %d): %v", valuePart, base, bitSize, err)
			return nil
		}
		return &ast.UnsignedIntegerLiteral{Token: tok, Value: value, Type: tok.Type}
	} else {
		value, err := strconv.ParseInt(valuePart, base, bitSize)
		if err != nil {
			p.currentError("could not parse %q as signed integer (base %d, size %d): %v", valuePart, base, bitSize, err)
			return nil
		}
		return &ast.IntegerLiteral{Token: tok, Value: value, Type: tok.Type}
	}
}

func (p *Parser) parseRealLiteral() ast.Expression {
	defer untrace(trace("parseRealLiteral"))
	lit := &ast.RealLiteral{Token: p.curToken}

	literal := p.curToken.Literal

	bitSize := 64 // Always parse to float64 to maintain precision

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
	defer untrace(trace("parseBitStringLiteral"))
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
	defer untrace(trace("parseStringLiteral")) // cspell:disable-line
	if p.curToken.Type == token.WSTRING_LITERAL {
		return &ast.WStringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	} else {
		return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	defer untrace(trace("parsePrefixExpression"))
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseInfixExpression"))
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()

	// For right-associative operators like exponentiation, we need to use a slightly lower precedence
	// to allow the right-hand side to be grouped first. e.g., a ** b ** c -> a ** (b ** c)
	if expression.Operator == "**" {
		expression.Right = p.parseExpression(precedence - 1)
	} else {
		expression.Right = p.parseExpression(precedence)
	}

	return expression
}

func (p *Parser) parseMemberAccessExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseMemberAccessExpression"))
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
	defer untrace(trace("parseBoolean"))
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	defer untrace(trace("parseGroupedExpression"))
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	defer untrace(trace("parseIfStatement"))
	ifStmt := &ast.IfStatement{Token: p.curToken}

	p.nextToken()
	ifStmt.Condition = p.parseExpression(LOWEST)

	// After parsing the condition, the next token should be THEN.
	if !p.expectPeek(token.THEN) {
		// Error recovery: if THEN is missing, an error has been logged by expectPeek.
		// We return the partially parsed statement and let the main ParseProgram loop's
		// recovery mechanism handle synchronization to the next statement.
		//return ifStmt
	}
	p.nextToken() // Consume THEN

	ifStmt.Consequence = p.parseBlockStatementUntil(token.ELSIF, token.ELSE, token.END_IF)

	// Keep track of the current statement for chaining ELSIF
	current := ifStmt

	// Loop to handle all ELSIF clauses
	for p.curTokenIs(token.ELSIF) {
		newIf := &ast.IfStatement{Token: p.curToken}
		p.nextToken()
		newIf.Condition = p.parseExpression(LOWEST)

		if !p.expectPeek(token.THEN) {
			p.specificError("missing 'THEN' in ELSIF statement, got %s instead", p.peekToken.Type)
		}
		p.nextToken() // consume THEN
		newIf.Consequence = p.parseBlockStatementUntil(token.ELSIF, token.ELSE, token.END_IF)

		current.Alternative = newIf
		current = newIf
	}

	// Handle the final ELSE clause
	if p.curTokenIs(token.ELSE) {
		p.nextToken() // Consume ELSE
		current.Alternative = p.parseBlockStatementUntil(token.END_IF)
	}

	// The last token should be END_IF
	if !p.curTokenIs(token.END_IF) {
		p.currentError("missing 'END_IF' for IF statement starting at row %d", ifStmt.Token.Row)
		// Do not return nil. Return the partially parsed statement to allow recovery.
	}

	return ifStmt
}

func (p *Parser) parseForStatement() ast.Statement {
	defer untrace(trace("parseForStatement"))
	stmt := &ast.ForLoopStatement{Token: p.curToken}

	// The initialization part is an assignment statement, but without the trailing semicolon.
	// We parse it manually here.
	if !p.expectPeek(token.IDENT) { // Expect and consume FOR, move to IDENT
		return nil
	}

	controlVarIdent := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	assignToken := p.curToken

	p.nextToken() // Move to the start of the start value expression
	startValue := p.parseExpression(LOWEST)

	stmt.ControlVar = &ast.AssignmentStatement{Token: assignToken, Left: controlVarIdent, Value: startValue}

	// After parsing the start value, the next token should be TO.
	if !p.expectPeek(token.TO) {
		return nil
	}

	p.nextToken() // Move to the start of the EndValue expression
	stmt.EndValue = p.parseExpression(LOWEST)

	// [BY <step_value>]
	if p.peekTokenIs(token.BY) {
		p.nextToken() // Consume EndValue, move to BY
		p.nextToken() // Consume BY, move to expression start
		stmt.StepValue = p.parseExpression(LOWEST)
	}

	// DO
	if !p.expectPeek(token.DO) {
		//return stmt // allow recovery
	}
	p.nextToken() // Consume DO
	stmt.Body = p.parseBlockStatementUntil(token.END_FOR)

	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	defer untrace(trace("parseWhileStatement"))
	stmt := &ast.WhileStatement{Token: p.curToken}

	p.nextToken() // Consume WHILE
	stmt.Condition = p.parseExpression(LOWEST)

	// Improved Error Recovery: Report missing DO but continue parsing the block.
	if !p.expectPeek(token.DO) {
		// Do not return, allow parsing of the body to continue.
		// expectPeek has already logged the error.
	}

	p.nextToken() // Consume DO
	stmt.Body = p.parseBlockStatementUntil(token.END_WHILE)

	// parseBlockStatementWhileLoop leaves us on END_WHILE, so we just need to consume it.
	if !p.curTokenIs(token.END_WHILE) {
		p.specificError("missing 'END_WHILE' for WHILE statement starting at row %d", stmt.Token.Row)
		// Return the partially parsed statement for better recovery
		return stmt
	}
	return stmt
}

func (p *Parser) parseRepeatStatement() ast.Statement {
	defer untrace(trace("parseRepeatStatement"))
	stmt := &ast.RepeatStatement{Token: p.curToken}

	p.nextToken() // consume REPEAT
	stmt.Body = p.parseBlockStatementRepeatLoop()

	if !p.curTokenIs(token.UNTIL) {
		return nil // error already reported by parseBlockStatementRepeatLoop
	}

	p.nextToken() // consume UNTIL
	stmt.Condition = p.parseExpression(LOWEST)

	// After parsing the expression, the current token is the last token of the expression.
	// We need to advance to the END_REPEAT token.
	if !p.expectPeek(token.END_REPEAT) {
		return nil // Error already reported
	}
	// Consume the END_REPEAT token to finish parsing the statement.
	p.nextToken()

	return stmt
}

func (p *Parser) parseCaseStatement() ast.Statement {
	defer untrace(trace("parseCaseStatement"))
	stmt := &ast.CaseStatement{Token: p.curToken}

	p.nextToken() // Consume CASE
	stmt.Expression = p.parseExpression(LOWEST)

	if !p.expectPeek(token.OF) {
		return nil
	}

	p.nextToken() // Consurem OF

	// Parse case branches
	for !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_CASE) && !p.curTokenIs(token.EOF) && !p.peekTokenIs(token.END_CASE) {
		branch := &ast.CaseBranch{Token: p.curToken}
		branch.Values = []ast.Expression{}

		// Parse comma-separated values
		for {
			branch.Values = append(branch.Values, p.parseCaseValue())

			if !p.peekTokenIs(token.COMMA) { // If the next token is NOT a comma, we're done with this list of values
				break
			}
			// If we are here, p.peekToken IS a COMMA.
			p.nextToken() // Consume the last token of the current value (e.g., 'RED')
			p.nextToken() // Consume the comma ',' to position for the next value
		}

		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken() // Move to the start of the statement

		branch.Consequence = p.parseStatement()
		if es, ok := branch.Consequence.(*ast.ExpressionStatement); ok && es.Expression == nil {
			branch.Consequence = p.parseBlockStatementUntil(token.ELSE, token.END_CASE)
		}
		p.nextToken() // After parsing a statement, curToken is ';'. Advance to the next token.
		stmt.Cases = append(stmt.Cases, branch)
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

func (p *Parser) parseCaseValue() ast.Expression {
	defer untrace(trace("parseCaseValue"))

	// A case label can be any valid expression, including literals (5),
	// ranges (1..10), or typed literals (COLOR#RED). The general
	// expression parser is designed to handle all of these cases correctly.
	return p.parseExpression(LOWEST)
}

func (p *Parser) parseBlockStatementForIf() *ast.BlockStatement {
	defer untrace(trace("parseBlockStatementForIf"))
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.ELSIF) && !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_IF) && !p.curTokenIs(token.EOF) {
		// The main loop in ParseProgram now handles the nextToken call,
		// so we don't need special logic here.
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseAssignmentStatement() *ast.AssignmentStatement {
	defer untrace(trace("parseAssignmentStatement"))
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

	p.expectPeek(token.SEMICOLON) // Consume semicolon
	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	defer untrace(trace("parseExpressionStatement"))
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	return stmt
}

// parseVarBlock is a helper to parse a VAR...END_VAR block and return the declarations.
func (p *Parser) parseVarBlock(blockType token.TokenType) []*ast.VarDeclStatement {
	defer untrace(trace(fmt.Sprintf("parseVarBlock (%s)", blockType)))
	if !p.curTokenIs(blockType) {
		return nil
	}

	p.nextToken() // Consume the block type token (e.g., VAR_INPUT)

	// We are at the start of a VAR block, parseVarDeclarations expects to be after the block token
	decls := p.parseVarDeclarations(token.END_VAR)

	// After parsing declarations, we should be on the END_VAR token.
	// We consume it here so the caller doesn't have to.
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}

	return decls
}

func (p *Parser) parseBlockStatementUntil(end ...token.TokenType) *ast.BlockStatement {
	defer untrace(trace(fmt.Sprintf("parseBlockStatementUntil (until %v)", end)))
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	isEndToken := func(t token.TokenType) bool {
		for _, et := range end {
			if t == et {
				return true
			}
		}
		return false
	}

	for !isEndToken(p.curToken.Type) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		if isStatementStartKeyword(p.curToken.Type) {
			break
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parseBlockStatementRepeatLoop() *ast.BlockStatement {
	defer untrace(trace("parseBlockStatementRepeatLoop"))
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

func isStatementEndToken(tok token.TokenType) bool {
	switch tok {
	case token.END_VAR,
		token.END_IF,
		token.END_FOR,
		token.END_WHILE,
		token.END_REPEAT,
		token.END_CASE:
		return true
	default:
		return false
	}
}

// isBlockStatement checks if a statement is a block-level statement
// that manages its own token consumption until its end token.
func isBlockStatement(stmt ast.Statement) bool {
	switch stmt.(type) {
	case *ast.IfStatement, *ast.ForLoopStatement, *ast.WhileStatement,
		*ast.RepeatStatement, *ast.CaseStatement, *ast.ConfigurationDeclaration,
		*ast.FunctionDeclaration, *ast.FunctionBlockDeclaration, *ast.ProgramDeclaration,
		*ast.TypeBlockDeclaration, *ast.VarBlockDeclaration, *ast.ActionStatement,
		*ast.ConfigVarDeclaration, *ast.ExternalVarDeclaration:
		return true
	default:
		return false
	}
}

// isStatementStartKeyword checks if a token type is a keyword that typically starts a new statement or block.
// This is useful for error recovery.
func isStatementStartKeyword(tok token.TokenType) bool {
	switch tok {
	case token.VAR, // A new var block indicates the previous one wasn't closed.
		token.IF,
		token.ELSE,
		token.ELSIF,
		token.FOR,
		token.WHILE,
		token.REPEAT,
		token.CASE,
		token.PROGRAM,
		token.FUNCTION,
		token.FUNCTION_BLOCK,
		token.ACTION,
		token.TRANSITION,
		token.STEP:
		return true
	default:
		return false
	}
}

// isValidIecDuration checks if a string conforms to the IEC 61131-3 time duration format.
// This is a simplified check using regex. A full validation would be more complex.
func isValidIecDuration(s string) bool {
	// This regex checks for an optional negative sign, followed by one or more segments
	// of (number)(unit), separated by underscores.
	// Units can be d, h, m, s, ms.
	// Example matches: 5s, 1h_30m, -10s_500ms
	// Example non-matches: 5z, 1h30m (missing underscore)
	re := regexp.MustCompile(`^-?(\d+(\.\d+)?(d|h|m|s|ms))(_\d+(\.\d+)?(d|h|m|s|ms))*$`)
	return re.MatchString(strings.ToLower(s))
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	defer untrace(trace("parseBlockStatement"))
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
	defer untrace(trace("parseFunctionLiteral"))
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
	defer untrace(trace("parseFunctionParameters"))
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
	defer untrace(trace("parseCallExpression"))
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	defer untrace(trace(fmt.Sprintf("parseExpressionList (until %s)", end)))
	list := []ast.Expression{}
	namedArgumentFound := false

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()                // Consume the opening parenthesis or bracket
	arg := p.parseCallArgument() // Parse the first argument
	if _, ok := arg.(*ast.NamedArgument); ok {
		namedArgumentFound = true
	} else if _, ok := arg.(*ast.OutputArgument); ok {
		namedArgumentFound = true
	}
	list = append(list, arg)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		arg = p.parseCallArgument() // Parse the next argument
		var isNamed bool
		if _, ok := arg.(*ast.NamedArgument); ok {
			isNamed = true
		} else if _, ok := arg.(*ast.OutputArgument); ok {
			isNamed = true
		}

		if namedArgumentFound && !isNamed {
			p.specificError("positional argument follows named argument in function call")
		} else if isNamed {
			namedArgumentFound = true
		}
		list = append(list, arg)
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

// parseGenericExpressionList is a more general version of parseExpressionList.
// It parses a comma-separated list of simple expressions until a given end token.
// This is suitable for array literals and repetition factors.
func (p *Parser) parseGenericExpressionList(end token.TokenType) []ast.Expression {
	defer untrace(trace(fmt.Sprintf("parseGenericExpressionList (until %s)", end)))
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken() // Consume the end token (e.g., ']')
		return list
	}

	p.nextToken() // Move to the first element
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume the expression
		p.nextToken() // Consume the comma
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseCallArgument() ast.Expression {
	defer untrace(trace("parseCallArgument"))
	// Check for named arguments (IDENT := or IDENT =>)
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
		// Input argument: In1 := 10
		name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume IDENT, curToken is now ASSIGN
		arg := &ast.NamedArgument{Token: p.curToken, Name: name}
		p.nextToken() // consume ASSIGN
		arg.Value = p.parseExpression(LOWEST)
		return arg
	} else if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ARROW) {
		// Output argument: Out1 => Res1
		name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume IDENT, curToken is now ARROW
		arg := &ast.OutputArgument{Token: p.curToken, Source: name}
		p.nextToken() // consume ARROW
		arg.Target = p.parseExpression(LOWEST)
		return arg
	}

	// Otherwise, it's a positional argument (an expression)
	return p.parseExpression(LOWEST)
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	defer untrace(trace("parseArrayLiteral"))
	array := &ast.ArrayLiteral{Token: p.curToken}

	array.Elements = p.parseArrayElementsList(token.RBRACKET)

	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseIndexExpression"))
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseTypedLiteral(left ast.Expression) ast.Expression {
	defer untrace(trace("parseTypedLiteral"))

	// The 'left' expression should be the type identifier (e.g., INT, REAL, DATE).
	typeIdent, ok := left.(*ast.Identifier)
	if !ok {
		p.currentError("expected a type name before '#', got %T", left)
		return nil
	}

	// The current token is '#'. We need to parse the literal value that follows.
	p.nextToken() // Consume '#'

	var valueExp ast.Expression
	typeNameUpper := strings.ToUpper(typeIdent.Value)

	// For date and time literals, we manually consume tokens to form a single identifier string.
	// This simplifies the AST, leaving the string parsing to the evaluator.
	if isDateTimeKeyword(typeNameUpper) {
		// For time/date types, we consume tokens to build a single string identifier.
		// This is simpler than parsing a complex expression for values like '2023-10-26-10:00:00'.
		valueExp = p.parseDateTimeIdentifier()

		// After assembling the string, perform validation for TIME literals.
		if typeNameUpper == "TIME" || typeNameUpper == "T" {
			if ident, ok := valueExp.(*ast.Identifier); ok {
				if !isValidIecDuration(ident.Value) {
					p.specificError("invalid time duration format: '%s'", ident.Value)
					return nil
				}
			}
		}
	} else {
		// For all other typed literals (INT#10, DATE#..., COLOR#RED), parse the value as a normal expression.
		valueExp = p.parseExpression(PREFIX)
	}

	if valueExp == nil {
		return nil // An error occurred during value parsing.
	}

	return &ast.TypedLiteral{
		Token:    typeIdent.Token, // The token of the type name (e.g., INT)
		TypeName: typeIdent.Value,
		Value:    valueExp,
	}
}

// isDateTimeKeyword checks if an identifier is a time/date keyword or abbreviation.
func isDateTimeKeyword(ident string) bool {
	// This logic is central to parsing and belongs in the parser, not the lexer.
	upper := strings.ToUpper(ident)
	switch upper {
	case "TIME", "T",
		"DATE", "D",
		"TIME_OF_DAY", "TOD",
		"DATE_AND_TIME", "DT":
		return true
	default:
		return false
	}
}

// parseDateTimeLiteral consumes tokens to build a single identifier for date/time literals.
// It handles constructs like `5s`, `5m_10s`, `2026-05-21`, and `14:30:00.5`.
func (p *Parser) parseDateTimeLiteral(typeName string) ast.Expression {
	defer untrace(trace(fmt.Sprintf("parseDateTimeLiteral (type: %s)", typeName)))
	startToken := p.curToken
	var builder strings.Builder

	// Consume tokens that can be part of a date/time literal value.
	// This loop continues as long as the tokens are numbers, identifiers (for units like 's', 'ms'),
	// or separators like '-', ':', and '.'.
	for {
		// The token must be a number, an identifier, or a separator.
		if p.curTokenIs(token.INT) || p.curTokenIs(token.REAL) || p.curTokenIs(token.IDENT) ||
			p.curTokenIs(token.MINUS) || p.curTokenIs(token.COLON) {
			builder.WriteString(p.curToken.Literal)

			// Peek ahead to see if the next token is also part of the literal.
			// We stop if the next token is a semicolon, parenthesis, or another operator
			// that would not be part of a date/time string.
			if !(p.peekTokenIs(token.INT) || p.peekTokenIs(token.REAL) || p.peekTokenIs(token.IDENT) ||
				p.peekTokenIs(token.MINUS) || p.peekTokenIs(token.COLON) || p.peekTokenIs(token.DOT)) {
				break
			}
			p.nextToken()
		} else {
			// The first token was not a valid start for a date/time value.
			p.currentError("invalid value for date/time literal, got %s", p.curToken.Type)
			return nil
		}
	}

	// Create a new identifier token that represents the entire literal value.
	combinedLiteral := builder.String()
	combinedToken := token.Token{
		Type:    token.IDENT,
		Literal: combinedLiteral,
		Row:     startToken.Row,
		Column:  startToken.Column,
		Pos:     startToken.Pos,
	}
	return &ast.Identifier{Token: combinedToken, Value: combinedLiteral}
}

func (p *Parser) parseHashLiteral() ast.Expression {
	defer untrace(trace("parseHashLiteral"))
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

// parseArrayElementsList parses a comma-separated list of expressions or repetition factors
// for an array literal, until the specified end token is encountered.
func (p *Parser) parseArrayElementsList(end token.TokenType) []ast.Expression {
	defer untrace(trace(fmt.Sprintf("parseArrayElementsList (until %s)", end)))
	elements := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken() // Consume the ']'
		return elements
	}

	p.nextToken() // Consume the '['

	// Parse the first element or repetition factor
	elements = append(elements, p.parseArrayElementOrRepetition())

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume the current element/repetition
		p.nextToken() // Consume the comma
		elements = append(elements, p.parseArrayElementOrRepetition())
	}

	if !p.expectPeek(end) {
		return nil
	}

	return elements
}

// parseArrayElementOrRepetition parses a single element within an array literal,
// which can either be a regular expression or a repetition factor (e.g., 3(0)).
func (p *Parser) parseArrayElementOrRepetition() ast.Expression {
	defer untrace(trace("parseArrayElementOrRepetition"))

	// Check for repetition factor: e.g., 3(0) or 2(1,2,3)
	if p.curTokenIs(token.INT) && p.peekTokenIs(token.LPAREN) {
		return p.parseArrayRepetition()
	}
	// Otherwise, it's a regular expression
	return p.parseExpression(LOWEST)
}

func (p *Parser) parseMacroLiteral() ast.Expression {
	defer untrace(trace("parseMacroLiteral"))
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

// parseArrayRepetition parses a repetition factor like `3(0)` or `2(1,2,3)`.
func (p *Parser) parseArrayRepetition() ast.Expression {
	defer untrace(trace("parseArrayRepetition"))

	factorToken := p.curToken
	factorExp := p.parseIntegerLiteral() // Parse the repetition factor (e.g., '3')

	if !p.expectPeek(token.LPAREN) { // Consume '('
		return nil
	}
	// The current token is now '('. We need to parse the elements inside.
	// We can reuse parseExpressionList for the elements inside the parentheses.
	// We use parseGenericExpressionList here as repetition elements are simple expressions.
	repeatedElements := p.parseGenericExpressionList(token.RPAREN)

	return &ast.ArrayRepetition{
		Token:    factorToken,
		Factor:   factorExp,
		Elements: repeatedElements,
	}
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
