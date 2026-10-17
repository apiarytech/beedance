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
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
	"strings"
)

// parseIlProgramBody parses a sequence of Instruction List (IL) statements,
// typically forming the body of a Program Organization Unit (POU) or a
// parenthesized expression. It continues parsing until it encounters a specified
// end token (e.g., `END_PROGRAM`, `RPAREN`).
func (p *Parser) parseIlProgramBody(endToken token.TokenType) *ast.BlockStatement {
	// The body of an IL program is a block of IL instructions.
	body := &ast.BlockStatement{Token: p.curToken}
	body.Statements = []ast.Statement{}

	// Loop until we hit the end of the block (e.g., END_FUNCTION_BLOCK) or EOF.
	for {
		// Comments may stand on their own lines between instructions.
		for p.curTokenIs(token.COMMENT) {
			p.nextToken()
		}
		if p.curTokenIs(endToken) || p.curTokenIs(token.EOF) {
			break
		}
		stmt := p.parseIlInstruction()
		if stmt != nil {
			body.Statements = append(body.Statements, stmt)
		}

		// In IL, each instruction is typically on a new line, ending with a semicolon
		// or implicitly ended by the newline. We advance to the next token to start
		// parsing the next instruction. If a semicolon is present, it will be consumed.
		// If not, we move to the next token on the new line.
		p.skipPeekComments()
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken()
	}

	return body
}

// parseIlInstruction parses a single line of an Instruction List (IL) program.
// An IL instruction can have an optional label, an operator (like `LD` or `ADD`),
// an optional operand (a variable or literal), and optional modifiers (like `N` for
// negation or `C` for conditional execution).
func (p *Parser) parseIlInstruction() ast.Statement {
	stmt := &ast.IlInstructionStatement{Token: p.curToken}

	// 1. Check for an optional label (e.g., "MyLabel:").
	// A label is an identifier followed by a colon.
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
		stmt.Label = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken() // consume the identifier
		p.nextToken() // consume the ':'
		for p.curTokenIs(token.COMMENT) {
			p.nextToken()
		}
	}

	// 2. Parse the operator (e.g., LD, ST, ADD) and its modifiers.
	if !p.isIlMnemonic() {
		// A call of a function or a method loads its result.
		if call, operator := p.parseIlCall(); call != nil {
			stmt.Operator = operator
			stmt.Operand = call
			return stmt
		}
		p.currentError("expected IL instruction mnemonic (e.g., LD, ST, ADD), got %q", p.curToken.Literal)
		return nil
	}
	operatorStr := p.curToken.Literal

	// The lexer provides the full identifier (e.g., "LDN", "JMPC"). We need to split
	// it into the base operator ("LD") and the modifier ("N", "C").
	baseOp, modifier := p.extractIlModifiers(operatorStr)
	// & is AND, and &N, which lexes as & and N, is ANDN.
	if operatorStr == "&" {
		baseOp = "AND"
		if p.peekTokenIs(token.IDENT) && strings.EqualFold(p.peekToken.Literal, "N") && p.peekToken.Pos == p.curToken.Pos+1 {
			modifier = "N"
			p.nextToken() // consume &, leaving N as the operator's last token
		}
	}
	// ST? is the assignment attempt (Table 68, 24): the modifier ?.
	if strings.EqualFold(baseOp, "ST") && modifier == "" && p.peekTokenIs(token.QUESTION) {
		modifier = "?"
		p.nextToken() // consume ST, leaving ? as the operator's last token
	}
	stmt.Operator = baseOp
	stmt.Modifier = modifier
	p.skipPeekComments() // a comment may stand between operator and operand

	// 3. Check for the '(' modifier, which defers the operation and takes precedence.
	if p.peekTokenIs(token.LPAREN) {
		// This is a deferred operation, e.g., `AND(`
		stmt.Modifier += "(" // Add '(' to the list of modifiers
		p.nextToken()        // Consume the operator (e.g., AND)
		p.nextToken()        // Consume the '('
		// The short form, AND( b, starts with an operand, which LD loads.
		var first *ast.IlInstructionStatement
		for p.curTokenIs(token.COMMENT) {
			p.nextToken()
		}
		if !p.curTokenIs(token.RPAREN) && !p.isIlMnemonic() && !(p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON)) {
			first = &ast.IlInstructionStatement{Token: p.curToken, Operator: "LD", Operand: p.parseExpression(LOGICAL_AND)}
			p.skipPeekComments()
			if p.peekTokenIs(token.SEMICOLON) {
				p.nextToken()
			}
			p.nextToken()
		}
		// The operand is the entire block of instructions inside the parentheses.
		stmt.Operand = p.parseIlProgramBody(token.RPAREN)
		if first != nil {
			block := stmt.Operand.(*ast.BlockStatement)
			block.Statements = append([]ast.Statement{first}, block.Statements...)
		}
		// parseIlProgramBody will stop at RPAREN, so we don't need to consume it here.
		return stmt
	}

	// Check for standard instructions that are known to have no operands.
	// This prevents them from consuming the next instruction as an operand.
	switch strings.ToUpper(baseOp) {
	case "RET", "NOT", "ABS", "SQRT", "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "LN", "LOG", "EXP":
		// These instructions operate on the current result and have no operand.
		return stmt
	}

	// 4. Parse the optional operand for non-deferred operators.
	// The operand is an expression that follows the operator.
	// Heuristic: An operand is present if the next token is not an end-of-block
	// or another statement keyword. This switch determines if an operand should be parsed.
	switch p.peekToken.Type {
	case token.END_PROGRAM, token.END_FUNCTION, token.END_FUNCTION_BLOCK, token.END_ACTION, token.END_STEP, token.END_TRANSITION, token.EOF, token.RPAREN, token.SEMICOLON:
		return stmt
	}
	if isStatementStartKeyword(p.peekToken.Type) {
		return stmt
	}

	// If we are here, an operand exists.
	// We check for EOF again just in case.
	if !p.peekTokenIs(token.EOF) {
		p.nextToken() // Consume the operator, move to the operand
		// The operand of an IL instruction is a "simple expression".
		// We parse it with a precedence that is higher than logical operators
		// to prevent the parser from greedily consuming the next IL instruction
		// (like AND, OR) as part of the current operand's expression.
		stmt.Operand = p.parseExpression(LOGICAL_AND)
	}

	return stmt
}

// extractIlModifiers takes a raw operator string from the lexer (e.g., "JMPC", "LDN")
// and splits it into its base operator ("JMP", "LD") and any associated modifiers
// ("C", "N"). This allows the parser to handle complex instructions correctly.
func (p *Parser) extractIlModifiers(op string) (baseOp string, modifier string) {
	opUpper := strings.ToUpper(op)
	mod := ""
	// Modifiers are checked from right to left. 'N' can appear before 'C'.
	if strings.HasSuffix(opUpper, "N") {
		mod = "N" + mod
		opUpper = opUpper[:len(opUpper)-1]
	}
	if strings.HasSuffix(opUpper, "C") {
		mod = "C" + mod
		opUpper = opUpper[:len(opUpper)-1]
	}

	// Only operators that take modifiers lose them; SIN, LN, TAN and ATAN are
	// mnemonics in their own right.
	if mod != "" && !ilModifiableOperators[opUpper] {
		return op, ""
	}

	// After stripping modifiers, what remains is the base operator. We need to find
	// the original casing of the base operator from the input string.
	baseOp = op[:len(opUpper)]

	return baseOp, mod
}

// ilModifiableOperators are the IL operators that take the N (negate) or C
// (conditional) modifier, as in LDN, ANDN, JMPC and RETCN.
var ilModifiableOperators = map[string]bool{
	"LD": true, "ST": true, "AND": true, "&": true, "OR": true, "XOR": true,
	"ADD": true, "SUB": true, "MUL": true, "DIV": true, "MOD": true,
	"GT": true, "GE": true, "EQ": true, "NE": true, "LE": true, "LT": true,
	"JMP": true, "CAL": true, "RET": true,
}

// isIlOperator provides a heuristic check to see if a token corresponds to a
// common Instruction List operator. This helps the parser decide whether to
// interpret a POU body as IL or as another language like Structured Text (ST).
func isIlOperator(tok token.TokenType) bool {
	switch tok { // cspell:disable-line
	case token.LD, token.ST, token.S, token.R, token.CAL, token.JMP, token.RET,
		token.AND, token.OR, token.XOR, token.NOT:
		return true
	default:
		return false
	}
}

// To integrate this, you would modify the POU parsing functions in `parser.go`.
// For example, `parseFunctionBlockDeclaration` would need to detect if the body
// is ST or IL and call the appropriate body parser.
//
// Example modification in `parser.go`:
/*
func (p *Parser) parseFunctionBlockDeclaration() ast.Statement {
    ...
    // After parsing VAR blocks:
    // if language is IL {
    //     stmt.Body = p.parseIlProgramBody(token.END_FUNCTION_BLOCK)
    // } else {
    //     stmt.Body = p.parseBlockStatementUntil(token.END_FUNCTION_BLOCK)
    // }
    ...
}
*/

// skipPeekComments drops comment tokens from the lookahead, so the next
// token seen is the first one that is not a comment. IL keeps no comments.
func (p *Parser) skipPeekComments() {
	for p.peekTokenIs(token.COMMENT) {
		p.peekToken = p.peek2Token
		p.peek2Token = p.l.NextToken()
	}
}

// ilBodyFollows reports whether a POU body starting here is IL: it starts
// with an IL instruction or a label, after any comments. When it is IL the
// comments are consumed; otherwise the parser is left as it was, so an ST
// body keeps its first statement's leading comments.
func (p *Parser) ilBodyFollows() bool {
	startsIL := func() bool {
		if p.isStJump() {
			return false // JMP label; is ST (IEC-68)
		}
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
			return p.labelBeforeIL()
		}
		return p.isIlInstruction()
	}
	if !p.curTokenIs(token.COMMENT) {
		return startsIL()
	}
	cur, peek, peek2, lex, errs := p.curToken, p.peekToken, p.peek2Token, *p.l, len(p.errors)
	for p.curTokenIs(token.COMMENT) {
		p.nextToken()
	}
	if startsIL() {
		return true
	}
	p.curToken, p.peekToken, p.peek2Token, *p.l = cur, peek, peek2, lex
	p.errors = p.errors[:errs]
	return false
}

// labelBeforeIL reports, at a label that starts a body, whether the
// statement after it is IL: a body may start with a label in ST too
// (IEC-68). The parser is left as it was.
func (p *Parser) labelBeforeIL() bool {
	cur, peek, peek2, lex, errs := p.curToken, p.peekToken, p.peek2Token, *p.l, len(p.errors)
	defer func() {
		p.curToken, p.peekToken, p.peek2Token, *p.l = cur, peek, peek2, lex
		p.errors = p.errors[:errs]
	}()
	p.nextToken() // the colon
	p.nextToken() // what it labels
	for p.curTokenIs(token.COMMENT) {
		p.nextToken()
	}
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
		return true // label after label: IL's way, as before
	}
	return !p.isStJump() && p.isIlInstruction()
}

// parseIlCall parses an instruction that calls a function or a method
// (Table 69): LIMIT 0, 10, whose first input is the current result;
// LIMIT(MN := 0, IN := a, MX := 10); or f.Add(k := 5). It returns the call,
// and the operator of the instruction: LD of a call of a function, CAL of an
// input operator, CU c. It returns nil if the instruction is not one.
func (p *Parser) parseIlCall() (call ast.Expression, operator string) {
	if !p.curTokenIs(token.IDENT) {
		return nil, ""
	}
	tok := p.curToken
	// An input operator of a standard function block (Table 70), CU c, is
	// CAL c(CU := the current result).
	if ilFBInputOperators[strings.ToUpper(tok.Literal)] && p.peekToken.Row == tok.Row && !p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		instance := p.parseExpression(LOGICAL_AND)
		return ast.IlInputCall(tok, instance), "CAL"
	}
	if p.peekTokenIs(token.LPAREN) || p.peekTokenIs(token.DOT) {
		if call, ok := p.parseExpression(LOWEST).(*ast.CallExpression); ok {
			return call, "LD"
		}
		p.currentError("expected a call of a function or a method, got %q", tok.Literal)
		return nil, ""
	}
	// The inputs after the first are on the instruction's line.
	args := []ast.Expression{&ast.IlCurrentResult{Token: tok}}
	for p.peekToken.Row == tok.Row && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.COMMENT) {
		p.nextToken()
		args = append(args, p.parseExpression(LOWEST))
		if !p.peekTokenIs(token.COMMA) {
			break
		}
		p.nextToken() // the comma
	}
	return &ast.CallExpression{Token: tok, Function: &ast.Identifier{Token: tok, Value: tok.Literal}, Arguments: args}, "LD"
}

// ilFBInputOperators are the input operators of the standard function
// blocks (Table 70) that are not also IL operators; S and R, which are, set
// an input of an RS or SR instance as the engines see its type.
var ilFBInputOperators = map[string]bool{
	"S1": true, "R1": true, "CLK": true, "CU": true, "CD": true, "PV": true, "IN": true, "PT": true, "RESET": true,
}
