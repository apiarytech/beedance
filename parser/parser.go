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
	token.HASH:      MEMBER, // Typed literals have similar precedence to member access.
}

var ilMnemonics = map[string]bool{
	"LD": true, "LDN": true, "ST": true, "STN": true, "S": true, "R": true,
	"ADD": true, "SUB": true, "MUL": true, "DIV": true, "GT": true, "GE": true,
	"EQ": true, "NE": true, "LE": true, "LT": true, "JMP": true, "JMPC": true,
	"JMPCN": true, "CAL": true, "CALC": true, "CALCN": true, "RET": true,
	"RETC": true, "RETCN": true, "AND": true, "ANDN": true, "OR": true,
	"ORN": true, "XOR": true, "XORN": true, "NOT": true, "ABS": true,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken        token.Token
	peekToken       token.Token
	peek2Token      token.Token // Second lookahead token
	leadingComments []string

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
	p.registerPrefix(token.INT, p.parseIntOrType)
	p.registerPrefix(token.REAL, p.parseRealOrType)
	p.registerPrefix(token.SINT, p.parseIntOrType)
	p.registerPrefix(token.DINT, p.parseIntOrType)
	p.registerPrefix(token.LINT, p.parseIntOrType)
	p.registerPrefix(token.USINT, p.parseIntOrType)
	p.registerPrefix(token.UINT, p.parseIntOrType)
	p.registerPrefix(token.UDINT, p.parseIntOrType)
	p.registerPrefix(token.ULINT, p.parseIntOrType)
	p.registerPrefix(token.LREAL, p.parseRealOrType)
	p.registerPrefix(token.BOOL, p.parseIdentifier)
	// Time and Date keywords can start a typed literal expression (e.g., T#5s).
	// We treat them like identifiers at this stage.
	p.registerPrefix(token.TIME, p.parseDataTypeKeyword)
	p.registerPrefix(token.DATE, p.parseDataTypeKeyword)
	// Register keywords that are also infix operators but can be used as
	// function calls (e.g., "MOD(a, b)"). When they appear in a prefix position,
	// they should be parsed as identifiers to enable function call parsing.
	p.registerPrefix(token.MOD, p.parseIdentifier)
	p.registerPrefix(token.AND, p.parseIdentifier)
	p.registerPrefix(token.OR, p.parseIdentifier)
	p.registerPrefix(token.XOR, p.parseIdentifier)
	p.registerPrefix(token.NAND, p.parseIdentifier)
	p.registerPrefix(token.NOR, p.parseIdentifier)
	p.registerPrefix(token.GT, p.parseIdentifier)
	p.registerPrefix(token.GE, p.parseIdentifier)
	p.registerPrefix(token.EQ, p.parseIdentifier)
	p.registerPrefix(token.LE, p.parseIdentifier)
	p.registerPrefix(token.LT, p.parseIdentifier)
	p.registerPrefix(token.NEQ, p.parseIdentifier)
	p.registerPrefix(token.MIN, p.parseIdentifier)
	p.registerPrefix(token.MAX, p.parseIdentifier)
	p.registerPrefix(token.MOVE, p.parseIdentifier)

	p.registerPrefix(token.S, p.parseIdentifier)
	p.registerPrefix(token.R, p.parseIdentifier)
	p.registerPrefix(token.TIME_OF_DAY, p.parseDataTypeKeyword)
	p.registerPrefix(token.DATE_AND_TIME, p.parseDataTypeKeyword)

	// Prefix Operators
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)

	// Grouped Expressions
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	// The non-standard anonymous function syntax from Monkey is now triggered by CPT.
	// This avoids ambiguity with the standard FUNCTION keyword.
	// p.registerPrefix(token.CPT, p.parseFunctionLiteral)
	p.registerPrefix(token.MACRO, p.parseMacroLiteral)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.LBRACE, p.parseHashLiteral)
	p.registerPrefix(token.STRUCT, p.parseStructDefinition)
	// Bit-string literals
	p.registerPrefix(token.BYTE, p.parseIdentifier)
	p.registerPrefix(token.WORD, p.parseIdentifier)
	p.registerPrefix(token.DWORD, p.parseIdentifier)
	p.registerPrefix(token.LWORD, p.parseIdentifier)
	p.registerPrefix(token.SR, p.parseIdentifier)
	p.registerPrefix(token.RS, p.parseIdentifier)

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
	p.registerInfix(token.HASH, p.parseTypedLiteralExpression)
	// // Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.peek2Token
	p.peek2Token = p.l.NextToken()

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

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) peek2TokenIs(t token.TokenType) bool {
	return p.peek2Token.Type == t
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

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	p.specificError("expected next token to be %s, got %s instead", t, p.peekToken.Type)
}

func (p *Parser) isIlInstruction() bool {
	if !p.isIlMnemonic() {
		return false
	}

	// The token is a known IL mnemonic. Now, we must resolve the ambiguity between
	// an ST function call like `ADD(5)` and a deferred IL operation like `ADD(LD A)`.
	if p.peekTokenIs(token.LPAREN) {
		// Heuristic: Look at the token *inside* the parenthesis (peek2Token).
		// If it's an unambiguous IL keyword, it's a deferred IL operation.
		// Otherwise, we assume it's a standard ST function call.
		nextTokenAfterParen := p.peek2Token
		switch nextTokenAfterParen.Type {
		case token.LD, token.ST, token.S, token.R, token.JMP, token.CAL, token.RET:
			// e.g., ADD(LD A) -> This is IL
			return true
		default:
			// e.g., ADD(5) or ADD(MyVar). Assume ST function call.
			return false
		}
	}
	// It's a mnemonic not followed by '(', so it's a standard IL instruction (e.g., LD var).
	return true
}

func (p *Parser) isIlMnemonic() bool {
	// Check if the literal (e.g., "LD", "ADD") is a known mnemonic.
	_, isMnemonic := ilMnemonics[strings.ToUpper(p.curToken.Literal)]
	return isMnemonic
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

// consumeLeadingComments consumes a sequence of comment tokens and stores their
// literals in the parser's leadingComments slice. This is called before parsing
// a statement to associate comments with the subsequent AST node.
func (p *Parser) consumeLeadingComments() {
	p.leadingComments = nil // Reset before consuming
	for p.curTokenIs(token.COMMENT) {
		p.leadingComments = append(p.leadingComments, p.curToken.Literal)
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
			// Block statements manage their own token consumption, so we only
			// advance if it's not a block statement.
			if !isBlockStatement(stmt) {
				p.nextToken()
			}
		}()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	defer untrace(trace("parseStatement"))

	// Consume any comments before the statement starts. They will be stored in p.leadingComments
	// and attached to the AST node by the specific parsing function.
	p.consumeLeadingComments()
	// If the current token looks like an IL instruction, decide whether to parse it as IL or ST.
	if p.isIlInstruction() {
		switch p.curToken.Type {
		// These mnemonics are also ST operators. In an ST context, they should be parsed as expressions.
		case token.AND, token.OR, token.XOR, token.NOT, token.MOD:
			return p.parseExpressionStatement()
		// These mnemonics are unambiguous or are handled as function calls by the expression parser.
		// If isIlInstruction is true, it means they are not function calls, so we treat them as IL.
		default:
			return p.parseIlInstruction()
		}
	}

	// If it's not an IL instruction, parse as a standard ST statement.
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
		// isIlInstruction() was already checked and returned false, so this IDENT is not an IL mnemonic.
		if p.peekTokenIs(token.ASSIGN) {
			return p.parseAssignmentStatement()
		}
		return p.parseExpressionStatement()
	default:
		// Any other token that can start an expression.
		// isIlInstruction() was already checked and returned false.
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
	stmt := &ast.VarBlockDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}

	p.nextToken() // Consume VAR

	stmt.Declarations = p.parseVarDeclarations(token.END_VAR)

	// After parsing the declarations, we must be at the END_VAR token.
	if !p.curTokenIs(token.END_VAR) {
		// The error is already reported by parseVarDeclarations, so we don't need to report it again.
		// Just ensure we don't advance past the token that should start the next statement.
		return stmt
	}
	p.nextToken() // Consume END_VAR

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
	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExternalVarDeclStatement() *ast.ExternalVarDeclaration {
	defer untrace(trace("parseExternalVarDeclStatement"))
	stmt := &ast.ExternalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_EXTERNAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseAccessVarDeclStatement() *ast.AccessVarDeclaration {
	defer untrace(trace("parseAccessVarDeclStatement"))
	stmt := &ast.AccessVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_ACCESS
	stmt.Vars = p.parseAccessDeclarations()
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}
	return stmt
}

// parseAccessDeclarations is a specialized version of parseVarDeclarations for VAR_ACCESS blocks.
// It handles the READ_ONLY and READ_WRITE qualifiers per declaration.
func (p *Parser) parseAccessDeclarations() []*ast.VarDeclStatement {
	defer untrace(trace("parseAccessDeclarations"))
	varDecls := []*ast.VarDeclStatement{}
	for !p.curTokenIs(token.END_VAR) && !p.curTokenIs(token.EOF) {
		// Consume any comments before the next declaration line.
		p.consumeLeadingComments()

		if isStatementStartKeyword(p.curToken.Type) {
			p.peekError(token.END_VAR)
			return varDecls
		}

		// VAR_ACCESS syntax is: LocalName : AccessPath [READ_ONLY | READ_WRITE];
		if !p.curTokenIs(token.IDENT) {
			p.currentError("expected identifier for local access variable name, got %s", p.curToken.Type)
			p.synchronizeParser()
			continue
		}
		localName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken() // Consume ':', move to access path
		accessPath := p.parseExpression(LOWEST)

		// Check for optional READ_ONLY or READ_WRITE
		var accessType string
		if p.peekTokenIs(token.READ_ONLY) || p.peekTokenIs(token.READ_WRITE) {
			p.nextToken()
			accessType = p.curToken.Literal
		}

		// DataType is implicit and resolved by the compiler/linker.
		decl := &ast.VarDeclStatement{Token: localName.Token, Name: localName, AccessPath: accessPath, DataType: nil, AccessType: accessType, LeadingComments: p.leadingComments}
		varDecls = append(varDecls, decl)

		p.expectPeek(token.SEMICOLON)
		p.nextToken() // Consume semicolon to move to the next declaration or END_VAR
	}
	return varDecls
}

func (p *Parser) parseTempVarDeclStatement() *ast.TempVarDeclaration {
	defer untrace(trace("parseTempVarDeclStatement"))
	stmt := &ast.TempVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_TEMP

	stmt.Vars = p.parseVarDeclarations(token.END_VAR)
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}
	return stmt
}

func (p *Parser) parseConfigVarDeclStatement() *ast.ConfigVarDeclaration {
	defer untrace(trace("parseConfigVarDeclStatement"))
	stmt := &ast.ConfigVarDeclaration{Token: p.curToken}
	p.nextToken() // consume VAR_CONFIG

	if !p.curTokenIs(token.IDENT) {
		p.currentError("expected program instance name after VAR_CONFIG, got %s", p.curToken.Type)
		// Attempt to recover by skipping to the end of the block.
		for !p.curTokenIs(token.END_VAR) && !p.curTokenIs(token.EOF) {
			p.nextToken()
		}
		return nil
	}
	stmt.ProgramInstanceName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken() // consume instance name

	stmt.Declarations = p.parseVarDeclarations(token.END_VAR)
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}
	return stmt
}

func (p *Parser) parseTypeBlockDeclaration() *ast.TypeBlockDeclaration {
	defer untrace(trace("parseTypeBlockDeclaration"))
	block := &ast.TypeBlockDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}
	block.Declarations = []*ast.TypeDeclaration{}

	p.nextToken() // consume TYPE

	for !p.curTokenIs(token.END_TYPE) && !p.curTokenIs(token.EOF) {
		// Consume any comments before the next declaration.
		p.consumeLeadingComments()

		decl := p.parseTypeDeclaration()
		if decl != nil {
			block.Declarations = append(block.Declarations, decl)
		}
		// p.parseTypeDeclaration consumes the semicolon, so we just advance.
		p.nextToken()
	}

	if p.curTokenIs(token.END_TYPE) {
		p.nextToken() // Consume END_TYPE
	}

	return block
}

func (p *Parser) parseTypeDeclaration() *ast.TypeDeclaration {
	defer untrace(trace("parseTypeDeclaration"))
	decl := &ast.TypeDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}

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
		// Skip any comments that might be between declarations.
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}

		// Consume any comments before the next declaration line.
		p.consumeLeadingComments()

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
				Token:           name.Token,
				Name:            name,
				Location:        atDecl,
				DataType:        dataType,
				Value:           initialValue,
				IsConstant:      isConstant, // Apply the block-level qualifier
				IsRetain:        isRetain,
				IsNonRetain:     isNonRetain,
				LeadingComments: p.leadingComments,
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
			cfgVar := p.parseConfigVarDeclStatement()
			if cfgVar != nil {
				stmt.VarConfigs = append(stmt.VarConfigs, cfgVar)
			}
		default:
			// If we encounter a token we don't recognize at this level, we advance past it to avoid an infinite loop.
			p.nextToken()
		}
	}
	if !p.curTokenIs(token.END_CONFIGURATION) {
		p.specificError("missing 'END_CONFIGURATION' for configuration starting at row %d", stmt.Token.Row)
		return nil
	} else {
		p.nextToken() // Consume END_CONFIGURATION
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
	switch tok.Type {
	case token.BOOL, token.SINT, token.INT, token.DINT, token.LINT,
		token.USINT, token.UINT, token.UDINT, token.ULINT, token.SR, token.RS, token.MACRO,
		token.REAL, token.LREAL, token.STRING, token.WSTRING,
		token.TIME, token.DATE, token.TIME_OF_DAY, token.DATE_AND_TIME,
		token.BYTE, token.WORD, token.DWORD, token.LWORD,
		token.ARRAY, token.STRUCT,
		token.IDENT: // User-defined types are identifiers
		return true
	default:
		return false
	}
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
	// An identifier is just an identifier. The Pratt parser's infix logic
	// will handle what comes next (like a '#' for a typed literal).
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseDataTypeKeyword() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))
	lit := &ast.IntegerLiteral{Token: p.curToken}
	literal := strings.ReplaceAll(p.curToken.Literal, "_", "")
	base := 10
	valueStr := literal

	if strings.Contains(literal, "#") {
		parts := strings.SplitN(literal, "#", 2)
		if len(parts) == 2 {
			parsedBase, err := strconv.Atoi(parts[0])
			if err == nil && (parsedBase == 2 || parsedBase == 8 || parsedBase == 10 || parsedBase == 16) {
				base = parsedBase
				valueStr = parts[1]
			}
		}
	}

	// Distinguish between signed and unsigned parsing
	switch p.curToken.Type {
	case token.USINT, token.UINT, token.UDINT, token.ULINT:
		// Parse as unsigned integer
		uValue, err := strconv.ParseUint(valueStr, base, 64)
		if err != nil {
			msg := fmt.Sprintf("could not parse %q as unsigned integer", p.curToken.Literal)
			p.errors = append(p.errors, msg)
			return nil
		}
		return &ast.UnsignedIntegerLiteral{Token: p.curToken, Value: uValue}
	default:
		// Parse as signed integer
		value, err := strconv.ParseInt(valueStr, base, 64)
		if err != nil {
			msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
			p.errors = append(p.errors, msg)
			return nil
		}
		lit.Value = value
		return lit
	}
}

func (p *Parser) parseRealLiteral() ast.Expression {
	defer untrace(trace("parseRealLiteral"))
	lit := &ast.RealLiteral{Token: p.curToken}
	bitSize := 64
	lit.Precision = bitSize
	literal := strings.ReplaceAll(p.curToken.Literal, "_", "")
	// The lexer does not produce based real literals, so we can parse directly.
	value, err := strconv.ParseFloat(literal, bitSize)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as real", literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

// func (p *Parser) parseBitStringLiteral() ast.Expression {
// 	defer untrace(trace("parseBitStringLiteral"))
// 	lit := &ast.BitStringLiteral{Token: p.curToken}

// 	var width int
// 	switch p.curToken.Type {
// 	case token.BYTE:
// 		width = 8
// 	case token.WORD:
// 		width = 16
// 	case token.DWORD:
// 		width = 32
// 	case token.LWORD:
// 		width = 64
// 	default:
// 		// This case should ideally not be reached if prefix functions are registered correctly.
// 		p.errors = append(p.errors, fmt.Sprintf("unknown bitstring type: %s", p.curToken.Type))
// 		return nil
// 	}
// 	lit.Width = width

// 	literal := p.curToken.Literal // e.g., "BYTE#16#FF" or "WORD#FF"

// 	parts := strings.SplitN(literal, "#", 2) // cspell:disable-line
// 	if len(parts) < 2 {
// 		p.errors = append(p.errors, fmt.Sprintf("invalid bitstring literal format: %q", literal))
// 		return nil
// 	}
// 	valuePart := parts[1] // e.g., "16#FF" or "FF"

// 	var base int = 16 // Default base for bit strings if not specified, as per IEC 61131-3
// 	var valueStr string = valuePart

// 	// Check if the value part itself contains a base (e.g., "16#FF")
// 	if strings.Contains(valuePart, "#") {
// 		valueParts := strings.SplitN(valuePart, "#", 2)
// 		if len(valueParts) == 2 {
// 			parsedBase, err := strconv.Atoi(valueParts[0])
// 			if err != nil {
// 				p.errors = append(p.errors, fmt.Sprintf("invalid base in bitstring literal: %q", valueParts[0]))
// 				return nil
// 			}
// 			base = parsedBase
// 			valueStr = valueParts[1]
// 		}
// 	}

// 	// Remove underscores from the value string before parsing
// 	valueStr = strings.ReplaceAll(valueStr, "_", "")

// 	val, err := strconv.ParseUint(valueStr, base, width)
// 	if err != nil {
// 		// Check if the error is due to the value being out of range for the specified width.
// 		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
// 			p.errors = append(p.errors, fmt.Sprintf("value %q is out of range for type %s (width %d)", valueStr, p.curToken.Type, width))
// 		} else {
// 			// Handle other parsing errors.
// 			p.errors = append(p.errors, fmt.Sprintf("could not parse %q as %s (base %d): %s", valueStr, p.curToken.Type, base, err.Error()))
// 		}
// 		return nil
// 	}

// 	lit.Value = val
// 	return lit
// }

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
	ifStmt := &ast.IfStatement{Token: p.curToken, LeadingComments: p.leadingComments}

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
	} else {
		p.nextToken() // Consume END_IF
	}

	return ifStmt
}

func (p *Parser) parseForStatement() ast.Statement {
	defer untrace(trace("parseForStatement"))
	stmt := &ast.ForLoopStatement{Token: p.curToken, LeadingComments: p.leadingComments}

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
	if p.peekTokenIs(token.DO) {
		p.nextToken() // consume EndValue
		p.nextToken() // consume DO
	} else {
		p.peekError(token.DO)
		// Recovery: assume DO was missing and the next token starts the body.
		p.nextToken() // consume EndValue to get to the start of the body
	}
	stmt.Body = p.parseBlockStatementUntil(token.END_FOR)
	if p.curTokenIs(token.END_FOR) {
		p.nextToken() // Consume END_FOR
	}

	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	defer untrace(trace("parseWhileStatement"))
	stmt := &ast.WhileStatement{Token: p.curToken, LeadingComments: p.leadingComments}

	p.nextToken() // Consume WHILE
	stmt.Condition = p.parseExpression(LOWEST)

	// Improved Error Recovery: Report missing DO but continue parsing the block.
	if p.peekTokenIs(token.DO) {
		p.nextToken() // consume condition
		p.nextToken() // consume DO
	} else {
		p.peekError(token.DO)
		// Recovery: assume DO was missing and the next token starts the body.
		p.nextToken() // consume condition to get to the start of the body
	}
	stmt.Body = p.parseBlockStatementUntil(token.END_WHILE)

	// parseBlockStatementWhileLoop leaves us on END_WHILE, so we just need to consume it.
	if !p.curTokenIs(token.END_WHILE) {
		p.specificError("missing 'END_WHILE' for WHILE statement starting at row %d", stmt.Token.Row)
		// Return the partially parsed statement for better recovery
		return stmt
	} else {
		p.nextToken() // Consume END_WHILE
	}
	return stmt
}

func (p *Parser) parseStepList() []*ast.Identifier {
	defer untrace(trace("parseStepList"))
	// A step list can be a single identifier, a comma-separated list,
	// or a parenthesized, comma-separated list.
	if p.curTokenIs(token.LPAREN) {
		p.nextToken() // consume '('
		list := p.parseIdentifierList()
		if !p.expectPeek(token.RPAREN) {
			return nil // Error reported by expectPeek
		}
		return list
	}
	// If not parenthesized, it's a regular identifier list.
	return p.parseIdentifierList()
}

func (p *Parser) parseRepeatStatement() ast.Statement {
	defer untrace(trace("parseRepeatStatement"))
	stmt := &ast.RepeatStatement{Token: p.curToken, LeadingComments: p.leadingComments}

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
		return stmt // Error reported, return for recovery
	}
	// expectPeek leaves us on END_REPEAT. Consume it to advance to the next token.
	p.nextToken()

	return stmt
}

func (p *Parser) parseCaseStatement() ast.Statement {
	defer untrace(trace("parseCaseStatement"))
	stmt := &ast.CaseStatement{Token: p.curToken, LeadingComments: p.leadingComments}

	p.nextToken() // Consume CASE
	stmt.Expression = p.parseExpression(LOWEST)

	if !p.expectPeek(token.OF) {
		return nil
	}

	p.nextToken() // Consurem OF

	for !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_CASE) && !p.curTokenIs(token.EOF) && !p.peekTokenIs(token.END_CASE) {
		branch := &ast.CaseBranch{Token: p.curToken, Values: []ast.Expression{}}
		for {
			branch.Values = append(branch.Values, p.parseExpression(LOWEST))
			if !p.peekTokenIs(token.COMMA) {
				break
			}
			p.nextToken() // Consume expression
			p.nextToken() // Consume comma
		}

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken() // consume COLON, move to start of consequence

		branch.Consequence = p.parseBlockStatementUntil(token.ELSE, token.END_CASE)
		stmt.Cases = append(stmt.Cases, branch)
	}

	// Parse optional ELSE block
	if p.curTokenIs(token.ELSE) {
		p.nextToken() // consume ELSE
		stmt.Alternative = p.parseBlockStatementUntil(token.END_CASE)
	}

	if !p.curTokenIs(token.END_CASE) {
		p.peekError(token.END_CASE)
	} else {
		p.nextToken() // Consume END_CASE
	}

	return stmt
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
	// This function is called for standalone assignments, so it should attach the leading comments.
	stmt := &ast.AssignmentStatement{
		Token: p.curToken, LeadingComments: p.leadingComments,
		Left: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
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

func isTimeDateKeyword(name string) bool {
	upper := strings.ToUpper(name)
	switch upper {
	case "TIME", "T", "DATE", "D", "TIME_OF_DAY", "TOD", "DATE_AND_TIME", "DT":
		return true
	default:
		return false
	}
}

func (p *Parser) parseTypedLiteralExpression(left ast.Expression) ast.Expression {
	// 'left' is the type name (e.g., the identifier 'INT' or 'COLOR').
	// The current token is '#'.
	if left == nil {
		// The prefix parser already logged an error and returned nil.
		// We cannot form a TypedLiteral without a type, so we propagate the nil
		// to prevent a panic on `left.Token()`.
		// cspell:disable-next-line
		return nil
	}

	// The type name must be an identifier.
	typeIdent, ok := left.(*ast.Identifier)
	if !ok {
		p.currentError("left side of # for typed literal must be a type identifier, got %T", left) // cspell:disable-line
		return nil
	}
	lit := &ast.TypedLiteral{Token: typeIdent.Token}
	lit.TypeName = typeIdent.Value

	p.nextToken() // Consume '#'

	// The value part is now parsed as a single identifier containing the whole value string.
	lit.Value = p.parseIecLiteralValue(lit.TypeName)

	return lit
}

// parseIecLiteralValue consumes tokens to build a single identifier that represents the value
// part of a typed literal (e.g., the "5s" in "T#5s" or "16#FF" in "BYTE#16#FF").
// This makes the parser smarter by grouping tokens based on the context of a typed literal,
// simplifying the evaluator which can then parse the resulting string.
func (p *Parser) parseIecLiteralValue(typeName string) ast.Expression {
	defer untrace(trace("parseIecLiteralValue"))
	startToken := p.curToken
	var builder strings.Builder

	isTimeType := isTimeDateKeyword(typeName)

	// Handle optional sign for time durations or negative numbers
	if p.curTokenIs(token.MINUS) {
		builder.WriteString(p.curToken.Literal)
		p.nextToken()
	}

	if isTimeType {
		// This loop stitches together what the lexer has broken apart for time literals.
		for {
			isAllowed := false
			switch p.curToken.Type {
			case token.INT, token.REAL, token.IDENT, token.DOT, token.MINUS, token.S, token.DATE, token.COLON:
				isAllowed = true
			}

			if !isAllowed {
				break
			}

			builder.WriteString(p.curToken.Literal)

			// Peek ahead to see if the next token could also be part of the literal.
			peekIsAllowed := false
			switch p.peekToken.Type {
			case token.INT, token.REAL, token.IDENT, token.DOT, token.MINUS, token.S, token.DATE, token.COLON:
				peekIsAllowed = true
			}

			if peekIsAllowed {
				p.nextToken() // It's part of the literal, consume and continue
			} else {
				goto end_loop
			}
		}
	} else {
		// For numeric, bit-string, or enum types, the value is a single token.
		// The lexer already handles based literals (e.g., 16#FF) as one token.
		switch p.curToken.Type {
		case token.INT, token.REAL, token.IDENT, token.TRUE, token.FALSE:
			builder.WriteString(p.curToken.Literal)
		default:
			// This case might be hit if a value is missing, e.g., `INT#;`
			// The check for an empty combinedLiteral below will handle this.
		}
	}
end_loop:

	combinedLiteral := builder.String()
	if combinedLiteral == "" {
		p.currentError("expected a value for typed literal")
		return nil
	}

	// We return an Identifier node containing the full literal string.
	return &ast.Identifier{
		Token: token.Token{
			Type:    token.IDENT,
			Literal: combinedLiteral,
			Row:     startToken.Row,
			Column:  startToken.Column,
			Pos:     startToken.Pos,
		},
		Value: combinedLiteral,
	}
}

func (p *Parser) parseIntOrType() ast.Expression {
	// If the literal can be parsed as an integer, it's an integer literal.
	if _, err := strconv.ParseInt(p.curToken.Literal, 10, 64); err == nil {
		return p.parseIntegerLiteral()
	}
	// Otherwise, it's a type keyword like 'INT', so treat it as an identifier.
	return p.parseIdentifier()
}

func (p *Parser) parseRealOrType() ast.Expression {
	if _, err := strconv.ParseFloat(p.curToken.Literal, 64); err == nil {
		return p.parseRealLiteral()
	}
	return p.parseIdentifier()
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	defer untrace(trace("parseExpressionStatement"))
	stmt := &ast.ExpressionStatement{Token: p.curToken, LeadingComments: p.leadingComments}

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

// parseVarTempBlock is a helper to parse a VAR_TEMP...END_VAR block and return the declarations.
func (p *Parser) parseVarTempBlock(blockType token.TokenType) *ast.TempVarDeclaration {
	defer untrace(trace(fmt.Sprintf("parseVarTempBlock (%s)", blockType)))
	if !p.curTokenIs(blockType) {
		return nil
	}
	// The current token is VAR_TEMP.
	stmt := &ast.TempVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume the block type token (e.g., VAR_TEMP)

	// We are at the start of a VAR block, parseVarDeclarations expects to be after the block token
	stmt.Vars = p.parseVarDeclarations(token.END_VAR)

	// After parsing declarations, we should be on the END_VAR token.
	// We consume it here so the caller doesn't have to.
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}

	return stmt
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
		// Skip any comments that might be inside the block.
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}

		// Heuristic for CASE statements: A new case label can start with any expression
		// (e.g., an INT like `3`, an IDENT like `MyState`). If we see a token that
		// could start an expression and is followed by a comma or colon, it's very
		// likely the start of the next case label list, so we should stop parsing statements for the current branch.
		if p.isStartOfCaseLabel() {
			break
		}

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		if !isBlockStatement(stmt) {
			p.nextToken()
		}
	}
	return block
}

// isStartOfCaseLabel is a heuristic to detect the beginning of a new case label list.
func (p *Parser) isStartOfCaseLabel() bool {
	// Check if the current token can start an expression.
	if _, ok := p.prefixParseFns[p.curToken.Type]; !ok {
		return false
	}

	// Look ahead to see if it's followed by a comma or colon, which are strong
	// indicators of a case label list (e.g., `3, 4:` or `MyState:`).
	if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.COLON) || p.peekTokenIs(token.RANGE) || p.peekTokenIs(token.HASH) {
		return true
	}
	return false
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
		if !isBlockStatement(stmt) {
			p.nextToken()
		}
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
	if stmt == nil {
		return false
	}
	switch stmt.(type) {
	case *ast.IfStatement, *ast.ForLoopStatement, *ast.WhileStatement,
		*ast.RepeatStatement, *ast.CaseStatement, *ast.ConfigurationDeclaration, *ast.StepStatement, *ast.TransitionStatement,
		*ast.FunctionDeclaration, *ast.FunctionBlockDeclaration, *ast.ProgramDeclaration, *ast.GlobalVarDeclaration,
		*ast.TypeBlockDeclaration, *ast.VarBlockDeclaration, *ast.ActionStatement, *ast.AccessVarDeclaration,
		*ast.ConfigVarDeclaration, *ast.ExternalVarDeclaration, *ast.TempVarDeclaration:
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

	// Handle empty list case: `()`
	if p.peekTokenIs(end) {
		p.nextToken() // consume ')'
		return list
	}

	p.nextToken() // Consume the opening token (e.g., '(' or '[')

	// Parse a comma-separated list of arguments
	if !p.curTokenIs(end) {
		list = append(list, p.parseCallArgument())
		for {
			if p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken() // consume comma
				for p.curTokenIs(token.COMMENT) {
					p.nextToken()
				}
				list = append(list, p.parseCallArgument())
			} else if p.peekTokenIs(token.COMMENT) {
				p.nextToken() // Skip comment and re-evaluate
			} else {
				break
			}
		}
	}
	p.expectPeek(end) // Consume the closing token
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
	for {
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken() // consume comma
			for p.curTokenIs(token.COMMENT) {
				p.nextToken()
			}
			list = append(list, p.parseExpression(LOWEST))
		} else if p.peekTokenIs(token.COMMENT) {
			p.nextToken() // Skip comment and re-evaluate
		} else {
			break
		}
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseCallArgument() ast.Expression {
	defer untrace(trace("parseCallArgument"))
	// Check for named arguments (IDENT := or IDENT =>)
	if p.peekTokenIs(token.ASSIGN) {
		// Input argument: In1 := 10
		name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume IDENT, curToken is now ASSIGN
		arg := &ast.NamedArgument{Token: p.curToken, Name: name}
		p.nextToken() // consume ASSIGN
		arg.Value = p.parseExpression(LOWEST)
		return arg
	} else if p.peekTokenIs(token.ARROW) {
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
