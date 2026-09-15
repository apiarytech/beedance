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

// Constants for operator precedence levels, used by the Pratt parser.
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

// precedences maps token types to their precedence level.
var precedences = map[token.TokenType]int{
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
	token.CARET:     CALL,   // Give it the same high precedence
	token.WITH:      MEMBER, // Give WITH similar precedence for program config
	token.HASH:      MEMBER, // Typed literals have similar precedence to member access.
}

// ilMnemonics is a set of all valid Instruction List mnemonics.
var ilMnemonics = map[string]bool{
	"LD": true, "LDN": true, "ST": true, "STN": true, "S": true, "R": true,
	"ADD": true, "SUB": true, "MUL": true, "DIV": true, "GT": true, "GE": true,
	"EQ": true, "NE": true, "LE": true, "LT": true, "JMP": true, "JMPC": true,
	"JMPCN": true, "CAL": true, "CALC": true, "CALCN": true, "RET": true,
	"RETC": true, "RETCN": true, "AND": true, "ANDN": true, "OR": true,
	"ORN": true, "XOR": true, "XORN": true, "NOT": true, "ABS": true,
}

// prefixParseFn is a function type for parsing prefix expressions (e.g., -5, NOT flag).
type (
	prefixParseFn func() ast.Expression
	// infixParseFn is a function type for parsing infix expressions (e.g., 5 + 5).
	infixParseFn func(ast.Expression) ast.Expression
)

// Parser holds the state of the parsing process, including the lexer, tokens,
// errors, and registered parsing functions for the Pratt parser.
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

// Error returns the error message for a parseError.
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

	// Register other IL mnemonics that are keywords to allow them as identifiers in ST
	p.registerPrefix(token.LD, p.parseIdentifier)
	p.registerPrefix(token.ST, p.parseIdentifier)
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
	p.registerPrefix(token.THIS, p.parseThisExpression)
	p.registerPrefix(token.SUPER, p.parseSuperExpression)
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

	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseMemberAccessExpression)
	p.registerInfix(token.HASH, p.parseTypedLiteralExpression)
	p.registerInfix(token.CARET, p.parseDereferenceExpression)
	p.registerInfix(token.WITH, p.parseInfixExpression)
	// // Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()
	p.nextToken()

	return p
}

// nextToken advances the parser's tokens (curToken, peekToken, peek2Token)
// and checks for lexer errors, converting them into parser errors.
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

// curTokenIs checks if the current token has the given type.
func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// peekTokenIs checks if the next token has the given type.
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// peek2TokenIs checks if the token after the next one has the given type.
func (p *Parser) peek2TokenIs(t token.TokenType) bool {
	return p.peek2Token.Type == t
}

// expectPeek checks if the next token is of the expected type. If it is, it advances the parser and returns true. If not, it logs an error and returns false.
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t) // Log the error
		// Always advance the token to prevent getting stuck, even if the expected token was not found.
		// This is crucial for robust error recovery.
		p.nextToken()
		return false
	}
}

// Errors returns the list of errors encountered during parsing.
func (p *Parser) Errors() []string {
	return p.errors
}

// peekError logs an error indicating that the next token was not of the expected type.
func (p *Parser) peekError(t token.TokenType) {
	p.specificError("expected next token to be %s, got %s instead", t, p.peekToken.Type)
}

// isIlInstruction provides a heuristic check to determine if the current token sequence represents an Instruction List (IL) instruction rather than a Structured Text (ST) expression.
func (p *Parser) isIlInstruction() bool {
	if !p.isIlMnemonic() {
		return false
	}

	// If a mnemonic is followed by ':=', it's an ST assignment, not an IL instruction.
	// This is the primary way to resolve the `ST := ...` ambiguity.
	if p.peekTokenIs(token.ASSIGN) {
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

// isIlMnemonic checks if the current token's literal is a known IL mnemonic.
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

// synchronize advances the parser until it finds one of the given end tokens or EOF.
// This is useful for recovering from an error within a block by skipping to the end of it.
func (p *Parser) synchronize(endTokens ...token.TokenType) {
	for {
		isEnd := false
		for _, et := range endTokens {
			if p.curTokenIs(et) {
				isEnd = true
				break
			}
		}
		if isEnd || p.curTokenIs(token.EOF) {
			break
		}
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
	// noPrefixParseFnError logs an error when no prefix parsing function is found for the current token type.
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

// ParseProgram is the main entry point for the parser. It parses the entire input
// from the lexer and returns the root `ast.Program` node of the Abstract Syntax Tree.
func (p *Parser) ParseProgram() *ast.Program {
	defer untrace(trace("ParseProgram")) // cspell:disable-line
	program := &ast.Program{}
	program.Statements = []ast.Statement{}
	lastPosition := -1 // Track the last token position to detect infinite loops

	for !p.curTokenIs(token.EOF) {
		// Infinite loop detection: if the token position hasn't changed since the
		// last iteration, we're stuck. This is the primary recovery mechanism.
		if p.curToken.Pos == lastPosition {
			// Log an error and force the parser to advance to the next token.
			p.currentError("parser stuck on token %s (%s), forcing advance to recover", p.curToken.Literal, p.curToken.Type)
			p.nextToken()
			continue // Restart the loop with the new token to prevent double-parsing.
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

			tracePrint(fmt.Sprintf("Current Token: %s (%s) at %d:%d", p.curToken.Literal, p.curToken.Type, p.curToken.Row, p.curToken.Column))

			errorCountBefore := len(p.errors)
			stmt := p.parseStatement()
			if stmt != nil {
				program.Statements = append(program.Statements, stmt)
			}
			// Only advance the token if the statement was parsed without adding new errors.
			// This prevents the loop from skipping past a token that a recovery function needs to see.
			if !isBlockStatement(stmt) && len(p.errors) == errorCountBefore {
				p.nextToken()
			}
		}()
	}

	return program
}

// parseStatement is the central dispatch function for parsing a single statement.
// It determines the statement type based on the current token and calls the appropriate parsing function.
func (p *Parser) parseStatement() ast.Statement {
	defer untrace(trace("parseStatement"))

	// Consume any comments before the statement starts. They will be stored in p.leadingComments
	// and attached to the AST node by the specific parsing function.
	p.consumeLeadingComments()

	// Handle empty statements (just a semicolon).
	if p.curTokenIs(token.SEMICOLON) {
		return nil // The main parsing loop will advance the token.
	}

	// If the current token looks like an IL instruction, decide whether to parse it as IL or ST.
	if p.isIlInstruction() {
		// This switch resolves ambiguity for tokens that are both IL mnemonics and ST operators.
		// For example, `AND` can be an infix operator in ST or an instruction in IL.
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
	case token.INTERFACE:
		return p.parseInterfaceDeclaration()
	case token.METHOD:
		return p.parseMethodImplementation()
	case token.EXIT:
		return p.parseExitStatement()
	case token.IDENT:
		return p.parseExpressionStatement()
	default:
		// Any other token that can start an expression.
		// isIlInstruction() was already checked and returned false.
		return p.parseExpressionStatement()
	}
}

// parseExitStatement parses an EXIT statement, which is used to terminate a loop.
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

// parseVarBlockStatement parses a `VAR ... END_VAR` block, containing one or more variable declarations.
func (p *Parser) parseVarBlockStatement() *ast.VarBlockDeclaration {
	defer untrace(trace("parseVarBlockStatement"))
	stmt := &ast.VarBlockDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}

	p.nextToken() // Consume VAR

	stmt.Declarations = p.parseVarDeclarations(token.END_VAR, token.VAR)

	// After parsing the declarations, we must be at the END_VAR token.
	if !p.curTokenIs(token.END_VAR) {
		// The error is already reported by parseVarDeclarations, so we don't need to report it again.
		// Just ensure we don't advance past the token that should start the next statement.
		return stmt
	}
	p.nextToken() // Consume END_VAR

	return stmt
}

// parseStructMember parses a single member declaration within a STRUCT definition.
func (p *Parser) parseStructMember() *ast.VarDeclStatement {
	defer untrace(trace("parseStructMember"))
	// cspell:disable-next-line
	// Struct members are like variable declarations but without the VAR keyword
	// and, according to the IEC 61131-3 standard, without initial values.
	stmt := &ast.VarDeclStatement{Token: p.curToken}

	if !p.curTokenIs(token.IDENT) {
		p.currentError("expected member name (identifier), got %s", p.curToken.Type)
		// Synchronize to the next potential member or end of struct
		p.synchronize(token.SEMICOLON, token.END_STRUCT)
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.COLON) {
		// Error logged by expectPeek. Synchronize to recover.
		p.synchronize(token.SEMICOLON, token.END_STRUCT)
		return nil
	}
	p.nextToken() // Move to the data type token

	stmt.DataType = p.parseTypeSpecifier()
	if stmt.DataType == nil {
		// Error logged by parseTypeSpecifier. Synchronize to recover.
		p.synchronize(token.SEMICOLON, token.END_STRUCT)
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

// parseGlobalVarDeclStatement parses a `VAR_GLOBAL ... END_VAR` block.
func (p *Parser) parseGlobalVarDeclStatement() *ast.GlobalVarDeclaration {
	defer untrace(trace("parseGlobalVarDeclStatement"))
	stmt := &ast.GlobalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_GLOBAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR, token.VAR_GLOBAL)
	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}

	return stmt
}

// parseExternalVarDeclStatement parses a `VAR_EXTERNAL ... END_VAR` block.
func (p *Parser) parseExternalVarDeclStatement() *ast.ExternalVarDeclaration {
	defer untrace(trace("parseExternalVarDeclStatement"))
	stmt := &ast.ExternalVarDeclaration{Token: p.curToken}

	p.nextToken() // Consume VAR_EXTERNAL

	stmt.Vars = p.parseVarDeclarations(token.END_VAR, token.VAR_EXTERNAL)
	if p.curTokenIs(token.END_VAR) {
		p.nextToken()
	}
	return stmt
}

// parseAccessVarDeclStatement parses a `VAR_ACCESS ... END_VAR` block.
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
			p.synchronize(token.SEMICOLON, token.END_VAR)
			if p.curTokenIs(token.SEMICOLON) {
				p.nextToken() // Consume recovery token
			}
			continue
		}
		localName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		if !p.expectPeek(token.COLON) {
			// Error logged by expectPeek. Synchronize and continue to next declaration.
			p.synchronize(token.SEMICOLON, token.END_VAR)
			if p.curTokenIs(token.SEMICOLON) {
				p.nextToken() // Consume recovery token
			}
			continue
		}

		p.nextToken() // Consume ':', move to access path
		// The access path can be a simple identifier (for global vars) or a
		// hierarchical path (e.g., resource.program.variable).
		// We parse it as a general expression.
		accessPath := p.parseExpression(LOWEST)

		if accessPath == nil {
			// Error already logged by parseExpression. Synchronize and continue.
			p.synchronize(token.SEMICOLON, token.END_VAR)
			if p.curTokenIs(token.SEMICOLON) {
				p.nextToken() // Consume recovery token
			}
			continue
		}

		var dataType ast.Expression
		// The data type is optional. If present, it's preceded by a colon.
		if p.peekTokenIs(token.COLON) {
			p.nextToken() // consume access path
			p.nextToken() // consume ':', move to data type
			dataType = p.parseTypeSpecifier()
		}
		// Check for optional READ_ONLY or READ_WRITE
		var accessType string
		if p.peekTokenIs(token.READ_ONLY) || p.peekTokenIs(token.READ_WRITE) {
			p.nextToken() // consume access path or data type
			accessType = p.curToken.Literal
		}

		decl := &ast.VarDeclStatement{Token: localName.Token, Name: localName, AccessPath: accessPath, DataType: dataType, AccessType: accessType, LeadingComments: p.leadingComments}
		varDecls = append(varDecls, decl)
		if !p.expectPeek(token.SEMICOLON) {
			// Error logged, recovery happened. Continue to next iteration.
			continue
		}
		// Semicolon was found, p.curToken is now ';'. Advance past it.
		p.nextToken()
	}
	return varDecls
}

// parseTempVarDeclStatement parses a `VAR_TEMP ... END_VAR` block.
func (p *Parser) parseTempVarDeclStatement() *ast.TempVarDeclaration {
	defer untrace(trace("parseTempVarDeclStatement"))
	stmt := &ast.TempVarDeclaration{Token: p.curToken}

	p.nextToken() // consume VAR_TEMP

	stmt.Vars = p.parseVarDeclarations(token.END_VAR, token.VAR_TEMP)
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}
	return stmt
}

// parseConfigVarDeclStatement parses a `VAR_CONFIG ... END_VAR` block, which is used to configure program instances.
func (p *Parser) parseConfigVarDeclStatement() *ast.ConfigVarDeclaration {
	defer untrace(trace("parseConfigVarDeclStatement"))
	// This block assigns instance-specific locations or initial values.
	// Syntax: VAR_CONFIG ... END_VAR
	stmt := &ast.ConfigVarDeclaration{Token: p.curToken}
	p.nextToken() // consume VAR_CONFIG

	// A VAR_CONFIG block can optionally be scoped to a single program instance.
	// e.g., VAR_CONFIG MyProgram ... END_VAR
	// Heuristic: If the token after VAR_CONFIG is an IDENT, and the token after that
	// is NOT a '.', ':', or 'AT', then we assume the first IDENT is a ProgramInstanceName.
	if p.curTokenIs(token.IDENT) && !(p.peekTokenIs(token.DOT) || p.peekTokenIs(token.COLON) || p.peekTokenIs(token.AT)) {
		stmt.ProgramInstanceName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume program instance name
	}

	declarations := []*ast.VarDeclStatement{}
	for !p.curTokenIs(token.END_VAR) && !p.curTokenIs(token.EOF) {
		// Allow comments between declarations
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}

		// Each line is an instance-specific assignment.
		// e.g., Station_1.P1.COUNT : INT := 1;
		// Add a check for a valid start of a declaration.
		if !p.curTokenIs(token.IDENT) {
			p.currentError("expected program instance name after VAR_CONFIG, got %s", p.curToken.Type)
			p.synchronize(token.SEMICOLON, token.END_VAR)
			if p.curTokenIs(token.SEMICOLON) {
				p.nextToken()
			}
			continue
		}
		// e.g., Station_1.P1.COUNT : INT := 1;
		// e.g., Station_2.P4.FB1.C2 AT %QB25 : BYTE;
		// Use LOWEST precedence to parse the full member access path (e.g., a.b.c)
		varPath := p.parseExpression(LOWEST)

		var atDecl *ast.AtDeclaration
		// The AT clause comes after the full path, so we check the peek token.
		if p.peekTokenIs(token.AT) {
			p.nextToken() // consume last part of path, curToken is now AT
			atDecl = p.parseAtDeclaration()
		}

		if !p.expectPeek(token.COLON) {
			p.synchronize(token.SEMICOLON, token.END_VAR)
			continue
		}
		p.nextToken() // consume colon

		dataType := p.parseTypeSpecifier()

		var initialValue ast.Expression
		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // to ASSIGN
			p.nextToken() // to expression start
			initialValue = p.parseExpression(LOWEST)
		}

		decl := &ast.VarDeclStatement{
			AccessPath: varPath,
			Location:   atDecl,
			DataType:   dataType,
			Value:      initialValue,
		}
		declarations = append(declarations, decl)

		if !p.expectPeek(token.SEMICOLON) {
			// Error logged, recovery happened. Continue to next iteration.
			continue
		}
		// Semicolon was found, p.curToken is now ';'. Advance past it.
		p.nextToken()
	}
	stmt.Declarations = declarations

	if !p.curTokenIs(token.END_VAR) {
		p.currentError("expected END_VAR, got %s", p.curToken.Type)
	} else {
		p.nextToken() // Consume END_VAR
	}

	return stmt
}

// parseTypeBlockDeclaration parses a `TYPE ... END_TYPE` block, containing one or more user-defined type declarations.
func (p *Parser) parseTypeBlockDeclaration() *ast.TypeBlockDeclaration {
	defer untrace(trace("parseTypeBlockDeclaration"))
	block := &ast.TypeBlockDeclaration{Token: p.curToken, LeadingComments: p.leadingComments}
	block.Declarations = []*ast.TypeDeclaration{}

	p.nextToken() // consume TYPE

	for !p.curTokenIs(token.END_TYPE) && !p.curTokenIs(token.EOF) {
		// Consume any comments before the next declaration.
		p.consumeLeadingComments()
		// If consuming comments landed us on END_TYPE, break the loop.
		if p.curTokenIs(token.END_TYPE) {
			break
		}

		decl := p.parseTypeDeclaration()
		if decl != nil {
			block.Declarations = append(block.Declarations, decl)
		}
		// If parseTypeDeclaration's error recovery (from a missing semicolon)
		// landed us on END_TYPE, we should break the loop.
		if p.curTokenIs(token.END_TYPE) {
			break
		}
		p.nextToken()
	}

	if p.curTokenIs(token.END_TYPE) {
		p.nextToken() // Consume END_TYPE
	}

	return block
}

// parseTypeDeclaration parses a single user-defined type declaration (e.g., a STRUCT, ENUM, or simple alias).
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
		// Or a string length declaration, e.g., STRING(10)
		isString := false
		if ts, ok := decl.DataType.(*ast.TypeSpecifier); ok {
			if ts.Token.Type == token.STRING || ts.Token.Type == token.WSTRING {
				isString = true
			}
		}
		p.nextToken() // consume type, curToken is now '('
		if isString {
			decl.StringLength = p.parseGroupedExpression()
		} else {
			decl.Subrange = p.parseGroupedExpression()
		}
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

// parseStructDefinition parses a `STRUCT ... END_STRUCT` definition.
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
		// If parseStructMember's error recovery landed us on END_STRUCT, break the loop.
		if p.curTokenIs(token.END_STRUCT) {
			break
		}
		p.nextToken() // parseStructMember consumed the ';', so advance to the next token.
	}

	if !p.curTokenIs(token.END_STRUCT) {
		p.specificError("missing 'END_STRUCT' for struct definition starting at row %d", structDef.Token.Row)
		// Do not return nil. Return the partially parsed struct to allow for better recovery.
	}

	return structDef
}

// parseEnumDefinition parses an enumerated type definition, which is a parenthesized list of identifiers.
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

// parseVarDeclarations parses a list of variable declarations within a block, until a specified end token is found.
func (p *Parser) parseVarDeclarations(endToken token.TokenType, blockType token.TokenType) []*ast.VarDeclStatement {
	defer untrace(trace(fmt.Sprintf("parseVarDeclarations (until %s)", endToken)))
	varDecls := []*ast.VarDeclStatement{}

	var isConstant, isRetain, isNonRetain, isRisingEdge, isFallingEdge bool

	// Loop to parse multiple qualifiers like CONSTANT, RETAIN, NON_RETAIN in any order.
	for {
		if p.curTokenIs(token.CONSTANT) {
			isConstant = true
			p.nextToken()
			continue
		}
		if p.curTokenIs(token.RETAIN) {
			isRetain = true
			p.nextToken()
			continue
		}
		if p.curTokenIs(token.NON_RETAIN) {
			isNonRetain = true
			p.nextToken()
			continue
		}
		if p.curTokenIs(token.R_EDGE) {
			isRisingEdge = true
			p.nextToken()
			continue
		}
		if p.curTokenIs(token.F_EDGE) {
			isFallingEdge = true
			p.nextToken()
			continue
		}
		// If no more qualifiers are found, break the loop.
		break
	}

	// After parsing qualifiers, perform validation.
	if isRisingEdge && isFallingEdge {
		// This error is reported at the block level, which is fine.
		p.currentError("cannot use R_EDGE and F_EDGE on the same variable")
	}
	if (isRisingEdge || isFallingEdge) && blockType != token.VAR_INPUT {
		// This error is also reported at the block level.
		p.currentError("R_EDGE and F_EDGE qualifiers can only be used in VAR_INPUT blocks")
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
		// Additional heuristic: if we see what looks like an assignment statement,
		// it's likely the start of the body, and END_VAR was missing.
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			p.currentError("expected next token to be %s, got %s instead", endToken, p.curToken.Type)
			// Return what we have, leaving the parser on the IDENT to parse the assignment.
			return varDecls
		}
		// Each iteration parses one or more variables of the same type.
		// e.g., Var1, Var2 : INT;
		names := p.parseIdentifierList()

		// The AT clause can appear before or after the data type.
		// We'll check for it in both places.
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
		// Validate that edge qualifiers are only used with BOOL.
		if (isRisingEdge || isFallingEdge) && dataType.String() != "BOOL" {
			// This error is reported at the line of the declaration.
			p.currentError("R_EDGE and F_EDGE qualifiers can only be applied to BOOL variables")
		}

		// Check for AT clause *after* the data type.
		if atDecl == nil && p.peekTokenIs(token.AT) {
			p.nextToken() // consume data type, move to AT
			atDecl = p.parseAtDeclaration()
		}
		// After parsing the type, we should be on the type token.

		var subrange, stringLength ast.Expression
		if p.peekTokenIs(token.LPAREN) {
			isString := false
			if ts, ok := dataType.(*ast.TypeSpecifier); ok {
				if ts.Token.Type == token.STRING || ts.Token.Type == token.WSTRING {
					isString = true
				}
			}

			p.nextToken() // consume data type or AT, curToken is now LPAREN
			if isString {
				stringLength = p.parseGroupedExpression()
			} else {
				subrange = p.parseGroupedExpression()
			}
		}

		// Now we advance to check for initialization or semicolon.

		var initialValue ast.Expression // cspell:disable-line
		if p.peekTokenIs(token.EQ) {
			p.peekError(token.ASSIGN) // Report that we expected := but got =
			p.synchronize(token.SEMICOLON)
			// Now that we are at the semicolon (or EOF), we can consume it and continue to the next declaration
			if p.curTokenIs(token.SEMICOLON) {
				p.nextToken()
			}
			continue // Continue to the next iteration of the `parseVarDeclarations` loop
		} else if p.peekTokenIs(token.ASSIGN) {
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
				StringLength:    stringLength,
				Subrange:        subrange,
				Value:           initialValue,
				IsConstant:      isConstant, // Apply the block-level qualifier
				IsRetain:        isRetain,
				IsNonRetain:     isNonRetain,
				IsRisingEdge:    isRisingEdge,
				IsFallingEdge:   isFallingEdge,
				LeadingComments: p.leadingComments,
			}
			varDecls = append(varDecls, decl)
		}
		if p.expectPeek(token.SEMICOLON) {
			p.nextToken() // Consume semicolon to move to the next declaration or end token
		}
		// If semicolon was missing, expectPeek already advanced us to the next token.
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

// parseAtDeclaration parses an `AT` clause, which maps a variable to a direct physical address.
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

// parseDirectVariable parses a directly addressed variable (e.g., %IX0.1).
func (p *Parser) parseDirectVariable() ast.Expression {
	defer untrace(trace("parseDirectVariable"))
	dv := &ast.DirectVariable{Token: p.curToken}
	// The lexer now provides the full direct variable literal (e.g., "%IX1.0")
	// We just need to strip the leading '%' for the AST node's value.
	dv.Address = strings.TrimPrefix(p.curToken.Literal, "%")
	return dv
}

// parseConfigurationDeclaration parses a `CONFIGURATION ... END_CONFIGURATION` block.
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
		case token.COMMENT:
			p.nextToken()
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

// parseResourceDeclaration parses a `RESOURCE ... END_RESOURCE` block within a configuration.
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

	for !p.curTokenIs(token.END_RESOURCE) && !p.curTokenIs(token.EOF) { // cspell:disable-line
		switch p.curToken.Type {
		case token.VAR_GLOBAL:
			globalVar := p.parseGlobalVarDeclStatement()
			if globalVar != nil {
				// This assumes ast.ResourceDeclaration has a GlobalVars field.
				stmt.GlobalVars = append(stmt.GlobalVars, globalVar)
			}
			// parseGlobalVarDeclStatement consumes its own end token (END_VAR),
			// so we continue to the next loop iteration immediately.
			continue
		case token.TASK:
			task := p.parseTaskDeclaration()
			if task != nil {
				stmt.Tasks = append(stmt.Tasks, task)
			}
			p.expectPeek(token.SEMICOLON)
		case token.PROGRAM:
			progConfig := p.parseProgramConfiguration()
			if progConfig != nil {
				stmt.Programs = append(stmt.Programs, progConfig)
			}
			p.expectPeek(token.SEMICOLON)
		case token.IDENT:
			// An IDENT at this level is likely a misplaced program instantiation.
			// The correct syntax is `PROGRAM <instance> ...`
			p.currentError("unexpected identifier '%s' in resource block, use PROGRAM keyword for instantiation", p.curToken.Literal)
			p.nextToken() // Skip to recover
			continue
		case token.COMMENT:
			p.nextToken()
			continue

		default:
			p.currentError("unexpected token '%s' in resource block", p.curToken.Type)
			p.nextToken() // Skip to recover
			continue
		}
		p.nextToken()
	}

	return stmt
}

// parseProgramConfiguration parses a program instantiation within a resource block.
// The new syntax is: `PROGRAM InstanceName WITH TaskName : ProgramType;`
func (p *Parser) parseProgramConfiguration() *ast.ProgramConfiguration {
	defer untrace(trace("parseProgramConfiguration"))
	// Current token is PROGRAM.
	stmt := &ast.ProgramConfiguration{Token: p.curToken}

	// After PROGRAM, check for optional RETAIN or NON_RETAIN
	if p.peekTokenIs(token.RETAIN) {
		p.nextToken() // consume PROGRAM, curToken is now RETAIN
		stmt.IsRetain = true
	} else if p.peekTokenIs(token.NON_RETAIN) {
		p.nextToken() // consume PROGRAM, curToken is now NON_RETAIN
		stmt.IsNonRetain = true
	}

	if !p.expectPeek(token.IDENT) {
		return nil // Expected program instance name
	}
	stmt.InstanceName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Check for optional WITH clause
	if p.peekTokenIs(token.WITH) {
		p.nextToken() // consume instance name, move to WITH
		p.nextToken() // consume WITH
		if !p.curTokenIs(token.IDENT) {
			p.currentError("expected task name after 'WITH', got %s", p.curToken.Type)
			return nil
		}
		stmt.TaskName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// After instance name and optional WITH clause, we expect a colon.
	if !p.expectPeek(token.COLON) {
		return nil
	}

	// After the colon is the program type name.
	if !p.expectPeek(token.IDENT) {
		p.currentError("expected program type name after ':', got %s", p.curToken.Type)
		return nil
	}
	stmt.TypeName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Optional parameters: (...)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // consume type name
		allParams := p.parseExpressionList(token.RPAREN)
		stmt.Parameters = []ast.Expression{}
		stmt.FbTasks = []*ast.FbTaskAssociation{}
		for _, param := range allParams {
			if fbTask, ok := param.(*ast.FbTaskAssociation); ok {
				stmt.FbTasks = append(stmt.FbTasks, fbTask)
			} else {
				stmt.Parameters = append(stmt.Parameters, param)
			}
		}
	}

	return stmt
}

// parseTaskDeclaration parses a `TASK` definition within a resource.
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

	// Handle empty list: `()`
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // consume ')'
		return stmt
	}

	p.nextToken() // consume '(', move to first keyword

	for {
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
		case token.COMMENT:
			p.nextToken()
			continue
		default:
			p.currentError("unexpected token in task configuration: %s", p.curToken.Type)
			// Synchronize to the end of the task declaration to allow parsing to continue.
			for !p.curTokenIs(token.RPAREN) && !p.curTokenIs(token.EOF) {
				p.nextToken()
			}
			return stmt
		}

		p.nextToken() // Advance past the expression value

		if p.curTokenIs(token.RPAREN) {
			break
		}
		if !p.curTokenIs(token.COMMA) {
			p.currentError("expected ',' or ')' in task configuration, got %s", p.curToken.Type)
			// Synchronize to the end of the task declaration to allow parsing to continue.
			for !p.curTokenIs(token.RPAREN) && !p.curTokenIs(token.EOF) {
				p.nextToken()
			}
			return stmt // Return partially parsed statement for recovery
		}
		p.nextToken() // consume comma, move to next keyword
	}

	return stmt
}

// parseIdentifierList parses a comma-separated list of identifiers.
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

// parseTypeSpecifier parses a data type specifier, which can be a simple type or a complex one like an ARRAY.
func (p *Parser) parseTypeSpecifier() ast.Expression {
	defer untrace(trace("parseTypeSpecifier"))
	if p.curTokenIs(token.ARRAY) {
		return p.parseArrayDefinition()
	} else if p.curTokenIs(token.REFERENCE) {
		return p.parseReferenceType()
	}
	if p.isDataTypeToken(p.curToken) {
		return &ast.TypeSpecifier{Token: p.curToken}
	}

	p.errors = append(p.errors, fmt.Sprintf("expected a data type, got %s", p.curToken.Type))
	return nil
}

// parseReferenceType parses a `REFERENCE TO <data_type>` specifier.
func (p *Parser) parseReferenceType() ast.Expression {
	defer untrace(trace("parseReferenceType"))
	refType := &ast.ReferenceType{Token: p.curToken}

	if !p.expectPeek(token.TO) {
		return nil // Error already logged
	}

	p.nextToken() // Consume 'TO', move to the base data type

	refType.BaseType = p.parseTypeSpecifier()
	if refType.BaseType == nil {
		return nil // Error already logged
	}
	return refType
}

// parseArrayDefinition parses an `ARRAY [...] OF ...` type definition.
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
	if !p.isDataTypeToken(p.curToken) {
		p.currentError("expected a data type after 'OF' in array definition, got %s", p.curToken.Type)
		return nil
	}

	def.DataType = &ast.TypeSpecifier{Token: p.curToken}
	return def
}

// isDataTypeToken checks if a token represents a valid data type keyword.
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

// parseReturnStatement parses a `RETURN` statement.
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	defer untrace(trace("parseReturnStatement"))
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	p.expectPeek(token.SEMICOLON) // Consume semicolon
	return stmt
}

// parseExpression is the core of the Pratt parser. It parses expressions based on
// operator precedence, handling prefix and infix operators.
func (p *Parser) parseExpression(precedence int) ast.Expression {
	defer untrace(trace(fmt.Sprintf("parseExpression (precedence %d)", precedence)))
	var prefix prefixParseFn
	// HACK: The 'fn' keyword for anonymous functions is a remnant of the Monkey
	// language and not part of the IEC standard. The tests still rely on it.
	// To support this without making 'fn' a reserved keyword, we treat it as a
	// special identifier that starts a function literal expression.
	if p.curTokenIs(token.IDENT) && p.curToken.Literal == "fn" {
		prefix = p.parseFunctionLiteral
	} else {
		prefix = p.prefixParseFns[p.curToken.Type]
	}
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

// peekPrecedence returns the precedence of the next token.
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

// curPrecedence returns the precedence of the current token.
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}

// parseIdentifier parses an identifier token into an `ast.Identifier` node.
func (p *Parser) parseIdentifier() ast.Expression {
	defer untrace(trace("parseIdentifier"))
	// An identifier is just an identifier. The Pratt parser's infix logic
	// will handle what comes next (like a '#' for a typed literal).
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// parseDataTypeKeyword parses a keyword that represents a data type (e.g., TIME) as an identifier.
func (p *Parser) parseDataTypeKeyword() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// parseIntegerLiteral parses an integer literal, handling different bases (2, 8, 10, 16) and distinguishing between signed and unsigned types.
func (p *Parser) parseIntegerLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))
	literal := strings.ReplaceAll(p.curToken.Literal, "_", "")

	// This function now only handles simple base-10 integers.
	// Based literals are handled by parseIntOrType.
	base := 10
	valueStr := literal

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
		lit := &ast.IntegerLiteral{Token: p.curToken}
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

// parseRealLiteral parses a floating-point number literal.
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

// parseStringLiteral parses a single-byte or wide-character string literal.
func (p *Parser) parseStringLiteral() ast.Expression {
	defer untrace(trace("parseStringLiteral")) // cspell:disable-line
	if p.curToken.Type == token.WSTRING_LITERAL {
		return &ast.WStringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	} else {
		return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	}
}

// parsePrefixExpression parses a prefix operator expression (e.g., -5, NOT flag).
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

// parseInfixExpression parses an infix operator expression (e.g., 5 + 5).
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseInfixExpression"))
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()

	if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.EOF) || isStatementEndToken(p.curToken.Type) {
		p.currentError("missing expression after operator '%s'", expression.Operator)
		return nil
	}

	// For right-associative operators like exponentiation, we need to use a slightly lower precedence
	// to allow the right-hand side to be grouped first. e.g., a ** b ** c -> a ** (b ** c)
	if expression.Operator == "**" {
		expression.Right = p.parseExpression(precedence - 1)
	} else {
		expression.Right = p.parseExpression(precedence)
	}

	return expression
}

// parseMemberAccessExpression parses a struct member access expression (e.g., myStruct.field).
func (p *Parser) parseMemberAccessExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseMemberAccessExpression"))
	exp := &ast.MemberAccessExpression{
		Token:  p.curToken,
		Struct: left,
	}

	// The member can be an identifier or a keyword used as an identifier (e.g., 'T', 'IN', 'Q').
	// We advance to the member token and then create an identifier from its literal.
	p.nextToken() // consume the '.'

	// Basic validation: a member name can't be a delimiter like a parenthesis or semicolon.
	if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.LPAREN) || p.curTokenIs(token.RPAREN) {
		p.currentError("expected identifier for member access, got %s", p.curToken.Type)
		return nil
	}

	exp.Member = &ast.Identifier{
		Token: p.curToken,
		Value: p.curToken.Literal,
	}

	return exp
}

// parseBoolean parses a boolean literal (TRUE or FALSE).
func (p *Parser) parseBoolean() ast.Expression {
	defer untrace(trace("parseBoolean"))
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

// parseGroupedExpression parses an expression enclosed in parentheses.
func (p *Parser) parseGroupedExpression() ast.Expression {
	defer untrace(trace("parseGroupedExpression"))
	startToken := p.curToken // This is '('

	// Heuristic for struct literals: if it starts with `IDENT :=` inside the parens,
	// or if it's an empty `()`, which could be an empty struct literal.
	if (p.peekTokenIs(token.IDENT) && p.peek2TokenIs(token.ASSIGN)) || p.peekTokenIs(token.RPAREN) {
		lit := &ast.StructLiteral{Token: startToken}
		// We can reuse parseExpressionList, which handles named arguments.
		// It expects to be called when the curToken is '('.
		lit.Initializers = p.parseExpressionList(token.RPAREN)
		return lit
	}

	p.nextToken() // consume '('
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

// parseIfStatement parses an `IF...THEN...ELSIF...ELSE...END_IF` statement.
func (p *Parser) parseIfStatement() *ast.IfStatement {
	defer untrace(trace("parseIfStatement"))
	ifStmt := &ast.IfStatement{Token: p.curToken, LeadingComments: p.leadingComments}

	p.nextToken()
	ifStmt.Condition = p.parseExpression(LOWEST)

	// After parsing the condition, the next token should be THEN.
	if !p.expectPeek(token.THEN) {
		// Error recovery: if THEN is missing, an error has been logged by expectPeek.
		// expectPeek has already advanced us to the start of the consequence,
		// so we don't need to do anything extra to recover here.
	} else {
		p.nextToken() // Consume THEN
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
			p.specificError("missing 'THEN' in ELSIF statement, got %s instead", p.peekToken.Type)
		}
		p.nextToken() // consume THEN
		newIf.Consequence = p.parseBlockStatementForIf()

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

// parseForStatement parses a `FOR...DO...END_FOR` loop.
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
	// If 'TO' is missing, expectPeek will log an error. We then proceed to parse
	// the EndValue from the current token (which would be the unexpected token).
	// If 'TO' was present, expectPeek would have consumed it, and p.curToken would be 'TO'.
	if !p.expectPeek(token.TO) {
		// Error logged by expectPeek. expectPeek has already advanced us
		// to the token that should have been the EndValue.
		// So we can just proceed to parse the expression now.
	} else {
		// If 'TO' was found, p.curToken is now 'TO'. Advance past it to the actual EndValue.
		p.nextToken()
	}
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
	if !p.curTokenIs(token.END_FOR) {
		p.currentError("missing 'END_FOR' for FOR statement starting at row %d", stmt.Token.Row)
	} else {
		p.nextToken() // Consume END_FOR
	}

	return stmt
}

// parseWhileStatement parses a `WHILE...DO...END_WHILE` loop.
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

// parseStepList parses a list of step identifiers, used in SFC `TRANSITION` statements.
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

// parseRepeatStatement parses a `REPEAT...UNTIL...END_REPEAT` loop.
func (p *Parser) parseRepeatStatement() ast.Statement {
	defer untrace(trace("parseRepeatStatement"))
	stmt := &ast.RepeatStatement{Token: p.curToken, LeadingComments: p.leadingComments}

	p.nextToken() // consume REPEAT
	stmt.Body = p.parseBlockStatementRepeatLoop()

	if !p.curTokenIs(token.UNTIL) {
		p.currentError("expected UNTIL, got %s", p.curToken.Type)
		// Recovery: if we are at END_REPEAT, we can assume UNTIL and condition were missing
		if p.curTokenIs(token.END_REPEAT) {
			p.nextToken() // consume END_REPEAT
		}
		return stmt
	}

	p.nextToken() // consume UNTIL
	stmt.Condition = p.parseExpression(LOWEST)

	// After parsing the expression, the current token is the last token of the expression.
	// We need to advance to the END_REPEAT token.
	if !p.peekTokenIs(token.END_REPEAT) {
		p.peekError(token.END_REPEAT)
		// Don't return, allow recovery by just returning the statement
	} else {
		p.nextToken() // consume expression's last token
		p.nextToken() // consume END_REPEAT
	}

	return stmt
}

// parseCaseStatement parses a `CASE...OF...ELSE...END_CASE` statement.
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

	// After parsing branches, if there are no branches and no ELSE, it's an error.
	if len(stmt.Cases) == 0 && !p.curTokenIs(token.ELSE) {
		p.currentError("no case branches found in CASE statement")
	}

	// Parse optional ELSE block
	if p.curTokenIs(token.ELSE) {
		p.nextToken() // consume ELSE
		stmt.Alternative = p.parseBlockStatementUntil(token.END_CASE)
	}

	if !p.curTokenIs(token.END_CASE) {
		p.currentError("missing 'END_CASE' for CASE statement starting at row %d", stmt.Token.Row)
	} else {
		p.nextToken() // Consume END_CASE
	}

	return stmt
}

// parseBlockStatementForIf is a specialized block parser for the consequence of an IF statement, stopping at ELSIF, ELSE, or END_IF.
func (p *Parser) parseBlockStatementForIf() *ast.BlockStatement {
	defer untrace(trace("parseBlockStatementForIf"))
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.ELSIF) && !p.curTokenIs(token.ELSE) && !p.curTokenIs(token.END_IF) && !p.curTokenIs(token.EOF) {
		// Handle comments inside the block before calling parseStatement
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}

		// The main loop in ParseProgram now handles the nextToken call,
		// so we don't need special logic here.
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

// isTimeDateKeyword checks if a string is a keyword for a time or date literal type.
func isTimeDateKeyword(name string) bool {
	upper := strings.ToUpper(name)
	switch upper {
	case "TIME", "T", "DATE", "D", "TIME_OF_DAY", "TOD", "DATE_AND_TIME", "DT":
		return true
	default:
		return false
	}
}

// parseTypedLiteralExpression parses a typed literal (e.g., `INT#10`, `COLOR#RED`).
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

// parseIntOrType disambiguates between an integer literal and a type keyword (like 'INT').
func (p *Parser) parseIntOrType() ast.Expression {
	literal := strings.ReplaceAll(p.curToken.Literal, "_", "")

	// Check for based literals first (e.g., 16#FF, 2#1010).
	if strings.Contains(literal, "#") {
		parts := strings.SplitN(literal, "#", 2)
		if len(parts) == 2 {
			base, err := strconv.Atoi(parts[0])
			if err == nil && (base == 2 || base == 8 || base == 10 || base == 16) {
				valueStr := parts[1]
				// It's a valid based literal. Parse it as such.
				switch p.curToken.Type {
				case token.USINT, token.UINT, token.UDINT, token.ULINT:
					uValue, err := strconv.ParseUint(valueStr, base, 64)
					if err != nil {
						p.currentError("could not parse %q as unsigned integer with base %d", valueStr, base)
						return nil
					}
					return &ast.UnsignedIntegerLiteral{Token: p.curToken, Value: uValue}
				default:
					value, err := strconv.ParseInt(valueStr, base, 64)
					if err != nil {
						p.currentError("could not parse %q as integer with base %d", valueStr, base)
						return nil
					}
					return &ast.IntegerLiteral{Token: p.curToken, Value: value}
				}
			}
		}
		// If it contains '#' but isn't a valid based literal, it's likely a type like `COLOR#RED`.
		// Treat it as an identifier to be handled by the infix '#' parser.
		return p.parseIdentifier()
	}

	// If it's a simple number without a base, try to parse it as base-10.
	if _, err := strconv.ParseInt(literal, 10, 64); err == nil {
		return p.parseIntegerLiteral()
	}
	// Also check for simple unsigned numbers.
	if _, err := strconv.ParseUint(literal, 10, 64); err == nil {
		return p.parseIntegerLiteral()
	}

	// If it's not a recognizable number, it must be a type keyword like 'INT'.
	// This allows it to be the start of a typed literal expression (e.g., INT#10).
	return p.parseIdentifier()
}

func (p *Parser) parseRealOrType() ast.Expression {
	if _, err := strconv.ParseFloat(p.curToken.Literal, 64); err == nil {
		return p.parseRealLiteral()
	}
	return p.parseIdentifier()
}

// parseExpressionStatement parses a statement that consists of a single expression followed by a semicolon.
// It also handles assignment statements, which are syntactically similar.
func (p *Parser) parseExpressionStatement() ast.Statement {
	defer untrace(trace("parseExpressionStatement"))
	startToken := p.curToken
	leftExp := p.parseExpression(LOWEST)

	// After parsing the left-hand side, check if it's an assignment.
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // consume leftExp, curToken is now ASSIGN
		stmt := &ast.AssignmentStatement{
			Token:           p.curToken, // The := token
			Left:            leftExp,
			LeadingComments: p.leadingComments,
		}
		p.nextToken() // consume ASSIGN
		stmt.Value = p.parseExpression(LOWEST)

		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		return stmt
	}

	// If not an assignment, it's a regular expression statement.
	stmt := &ast.ExpressionStatement{
		Token:           startToken,
		Expression:      leftExp,
		LeadingComments: p.leadingComments,
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	} else if !isBlockEndingToken(p.peekToken.Type) && !p.peekTokenIs(token.EOF) {
		p.peekError(token.SEMICOLON)
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
	decls := p.parseVarDeclarations(token.END_VAR, blockType)

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
	stmt.Vars = p.parseVarDeclarations(token.END_VAR, token.VAR_TEMP)

	// After parsing declarations, we should be on the END_VAR token.
	// We consume it here so the caller doesn't have to.
	if p.curTokenIs(token.END_VAR) {
		p.nextToken() // Consume END_VAR
	}

	return stmt
}

// parseBlockStatementUntil parses a block of statements until one of the specified end tokens is encountered.
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
		// Heuristic for error recovery: if we encounter a keyword that can only start a
		// top-level POU or a VAR block, it's a strong signal that the current block
		// was not closed correctly. We stop parsing this block and let the calling
		// function handle the error.
		isTopLevelKeyword := func(t token.TokenType) bool {
			return t == token.PROGRAM || t == token.FUNCTION_BLOCK || t == token.CONFIGURATION
		}

		// A VAR block is only allowed inside an ACTION body. For all other blocks
		// (IF, FOR, POU bodies), encountering a VAR keyword indicates the previous
		// block was not closed.
		isParsingActionBody := false
		for _, et := range end {
			if et == token.END_ACTION {
				isParsingActionBody = true
				break
			}
		}

		if isTopLevelKeyword(p.curToken.Type) || (p.curToken.Type == token.VAR && !isParsingActionBody) {
			break
		}

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

// parseBlockStatementRepeatLoop is a specialized block parser for `REPEAT` loops, stopping at `UNTIL`.
func (p *Parser) parseBlockStatementRepeatLoop() *ast.BlockStatement {
	defer untrace(trace("parseBlockStatementRepeatLoop"))
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	for !p.curTokenIs(token.UNTIL) && !p.curTokenIs(token.EOF) && !p.curTokenIs(token.END_REPEAT) {
		if p.curTokenIs(token.COMMENT) {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		if !isBlockStatement(stmt) {
			p.nextToken()
		}
	}
	// Let the caller (parseRepeatStatement) handle the error for missing UNTIL.
	return block
}

// isBlockEndingToken checks if a token type signifies the end of a statement block,
// where an optional semicolon is permissible for the last statement.
func isBlockEndingToken(tok token.TokenType) bool {
	switch tok {
	case token.RBRACE, // For anonymous functions `fn() { ... }`
		token.END_IF,
		token.END_FOR,
		token.END_WHILE,
		token.END_REPEAT,
		token.END_CASE,
		token.END_PROGRAM,
		token.END_FUNCTION,
		token.END_FUNCTION_BLOCK,
		token.END_ACTION,
		token.UNTIL, // For REPEAT loops
		token.ELSE,  // For IF and CASE statements
		token.ELSIF: // For IF statements
		return true
	default:
		return false
	}
}

// isStatementEndToken checks if a token type marks the end of a block statement.
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
		token.INTERFACE,
		token.ACTION,
		token.TRANSITION,
		token.THIS,
		token.SUPER,
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

// parseBlockStatement parses a block of statements enclosed in curly braces (a non-standard extension).
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
		// After a statement is parsed, advance to the next token, but only if
		// the statement was not a block statement, as block statements
		// consume their own end tokens.
		if !isBlockStatement(stmt) {
			p.nextToken()
		}
	}

	return block
}

// parseFunctionLiteral parses an anonymous function definition (a non-standard extension).
func (p *Parser) parseFunctionLiteral() ast.Expression {
	defer untrace(trace("parseFunctionLiteral"))
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters() // This will now return []*ast.FunctionParameter

	// Parse optional return type
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // Consume ')'
		p.nextToken() // Consume ':'
		lit.ReturnType = p.parseTypeSpecifier()
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()

	return lit
}

// parseFunctionParameters parses a comma-separated list of typed parameters for a function literal.
func (p *Parser) parseFunctionParameters() []*ast.FunctionParameter {
	defer untrace(trace("parseFunctionParameters"))
	parameters := []*ast.FunctionParameter{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // Consume ')'
		return parameters
	}

	p.nextToken() // Consume '(' or previous token

	// Parse the first parameter
	param := p.parseFunctionParameter()
	if param != nil {
		parameters = append(parameters, param)
	}

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume ','
		p.nextToken() // Move to the next parameter's identifier
		param = p.parseFunctionParameter()
		if param != nil {
			parameters = append(parameters, param)
		}
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return parameters
}

// parseFunctionParameter parses a single function parameter (e.g., `x : INT`).
func (p *Parser) parseFunctionParameter() *ast.FunctionParameter {
	defer untrace(trace("parseFunctionParameter"))
	param := &ast.FunctionParameter{}

	param.Name = p.parseIdentifier().(*ast.Identifier)
	p.expectPeek(token.COLON)
	p.nextToken() // Consume ':', move to data type
	param.DataType = p.parseTypeSpecifier()
	return param
}

// parseIdentifierParameters parses a comma-separated list of identifiers, used for macros.
func (p *Parser) parseIdentifierParameters() []*ast.Identifier {
	defer untrace(trace("parseIdentifierParameters"))
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // consume ')'
		return identifiers
	}

	p.nextToken() // consume '('

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume ','
		p.nextToken() // consume the identifier
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

// parseCallExpression parses a function or function block call, including its arguments.
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	defer untrace(trace("parseCallExpression"))
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

// parseExpressionList parses a comma-separated list of expressions, handling named and output arguments, until a specified end token.
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

// parseCallArgument parses a single argument in a function call, which can be positional, named input, or named output.
func (p *Parser) parseCallArgument() ast.Expression {
	defer untrace(trace("parseCallArgument"))

	// Check for fb_task: fb_name WITH task_name
	// This is a specific pattern IDENTIFIER WITH IDENTIFIER
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.WITH) && p.peek2TokenIs(token.IDENT) {
		fbName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume fb_name, curToken is now WITH
		p.nextToken() // consume WITH, curToken is now task_name
		taskName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		return &ast.FbTaskAssociation{
			Token:    fbName.Token,
			FbName:   fbName,
			TaskName: taskName,
		}
	}

	// Check for named arguments (IDENT := or IDENT =>)
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
		p.currentError("expected := or => in function block parameter, got :")
		// Recover by parsing as if it were a positional argument, which will likely fail but keeps the parser moving.
		return p.parseExpression(LOWEST)
	}
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

// parseArrayLiteral parses an array literal expression (e.g., `[1, 2, 3]`).
func (p *Parser) parseArrayLiteral() ast.Expression {
	defer untrace(trace("parseArrayLiteral"))
	array := &ast.ArrayLiteral{Token: p.curToken}

	array.Elements = p.parseArrayElementsList(token.RBRACKET)

	return array
}

// parseIndexExpression parses an array or string indexing expression (e.g., `myArray[i]`).
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

// parseHashLiteral parses a hash literal expression (a non-standard extension).
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

// parseMacroLiteral parses a macro definition (a non-standard extension).
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

// parseMacroLiteral parses a macro definition (a non-standard extension).
func (p *Parser) parseMacroLiteral() ast.Expression {
	defer untrace(trace("parseMacroLiteral"))
	lit := &ast.MacroLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseIdentifierParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// Explicitly parse the block statement here to correctly handle empty bodies `{}`
	// which can be misidentified as hash literals otherwise.
	if !p.curTokenIs(token.LBRACE) {
		return nil // expectPeek already logged the error
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

// registerPrefix registers a prefix parsing function for a given token type.
func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix registers an infix parsing function for a given token type.
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// parseThisExpression parses the THIS keyword into an ast.ThisExpression node.
func (p *Parser) parseThisExpression() ast.Expression {
	defer untrace(trace("parseThisExpression"))
	return &ast.ThisExpression{Token: p.curToken}
}

// parseSuperExpression parses the SUPER keyword into an ast.SuperExpression node.
func (p *Parser) parseSuperExpression() ast.Expression {
	defer untrace(trace("parseSuperExpression"))
	return &ast.SuperExpression{Token: p.curToken}
}

// parseDereferenceExpression parses the pointer dereference operator `^`.
func (p *Parser) parseDereferenceExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseDereferenceExpression"))
	return &ast.DereferenceExpression{
		Token:   p.curToken, // The '^' token
		Pointer: left,
	}
}
