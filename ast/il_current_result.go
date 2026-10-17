/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast

import (
	"strings"

	"github.com/apiarytech/beedance/token"
)

// IlCurrentResult is the current result of an IL body as a value: the first
// input of a function an IL instruction calls with its other inputs listed,
// LIMIT 0, 10, which the parser reads as LD LIMIT(<current result>, 0, 10).
type IlCurrentResult struct {
	Token token.Token
}

// expressionNode marks IlCurrentResult as an expression node.
func (cr *IlCurrentResult) expressionNode() {}

// Pos returns the position of the instruction.
func (cr *IlCurrentResult) Pos() (int, int) { return cr.Token.Row, cr.Token.Column }

// TokenLiteral returns the literal of the instruction's token.
func (cr *IlCurrentResult) TokenLiteral() string { return cr.Token.Literal }

// String returns the current result's name in the text of a call.
func (cr *IlCurrentResult) String() string { return "CR" }

// IlInputCall is the call an input operator of a standard function block
// makes (Table 70): op instance, CU c, is CAL instance(op := the current
// result). RESET c, of a counter, sets its input R.
func IlInputCall(op token.Token, instance Expression) *CallExpression {
	name := strings.ToUpper(op.Literal)
	if name == "RESET" {
		name = "R"
	}
	input := &Identifier{Token: op, Value: name}
	return &CallExpression{
		Token:     op,
		Function:  instance,
		Arguments: []Expression{&NamedArgument{Token: op, Name: input, Value: &IlCurrentResult{Token: op}}},
	}
}
