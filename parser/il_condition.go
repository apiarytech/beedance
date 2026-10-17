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

// A transition condition in IL (IEC 61131-3, Table 41, 6b and 7c):
//
//	TRANSITION FROM STEP7 TO STEP8 :
//	    LD  %IX2.4
//	    AND %IX2.3
//	END_TRANSITION
//
// is a simple instruction list: operations on the current result,
// parenthesized expressions and function calls, without jumps or stores.
// Such a list is an expression, %IX2.4 AND %IX2.3, which the transition
// holds as its condition, for every engine alike.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
)

// ilInfixOperators are the ST operators of the IL operators that combine the
// current result with an operand.
var ilInfixOperators = map[string]string{
	"AND": "AND", "OR": "OR", "XOR": "XOR",
	"ADD": "+", "SUB": "-", "MUL": "*", "DIV": "/", "MOD": "MOD",
	"GT": ">", "GE": ">=", "EQ": "=", "NE": "<>", "LE": "<=", "LT": "<",
}

// ilExpression returns the expression a simple IL instruction list
// computes.
func ilExpression(block *ast.BlockStatement) (ast.Expression, error) {
	var acc ast.Expression
	for _, s := range block.Statements {
		ins, ok := s.(*ast.IlInstructionStatement)
		if !ok {
			return nil, fmt.Errorf("a transition condition in IL has only IL instructions")
		}
		op, mod := strings.ToUpper(ins.Operator), strings.ToUpper(ins.Modifier)
		if ins.Label != nil || strings.Contains(mod, "C") {
			return nil, fmt.Errorf("a transition condition in IL has no labels, jumps or conditional instructions, got %s%s", op, mod)
		}
		operand := ins.Operand
		if inner, isBlock := operand.(*ast.BlockStatement); isBlock {
			e, err := ilExpression(inner) // AND( LD b OR c )
			if err != nil {
				return nil, err
			}
			operand = e
		}
		if call, isCall := operand.(*ast.CallExpression); isCall && acc != nil {
			operand = withCurrentResult(call, acc) // LIMIT 0, 10
		}
		if strings.Contains(mod, "N") && operand != nil {
			operand = &ast.PrefixExpression{Token: ins.Token, Operator: "NOT", Right: operand}
		}
		switch {
		case op == "LD":
			acc = operand
		case op == "NOT":
			if acc == nil {
				return nil, fmt.Errorf("NOT before LD in a transition condition")
			}
			acc = &ast.PrefixExpression{Token: ins.Token, Operator: "NOT", Right: acc}
		case ilInfixOperators[op] != "":
			if acc == nil || operand == nil {
				return nil, fmt.Errorf("%s needs a current result and an operand in a transition condition", op)
			}
			acc = &ast.InfixExpression{Token: ins.Token, Left: acc, Operator: ilInfixOperators[op], Right: operand}
		case operand == nil && acc != nil:
			// A function of the current result alone, ABS or SQRT.
			acc = &ast.CallExpression{Token: ins.Token, Function: &ast.Identifier{Token: ins.Token, Value: ins.Operator}, Arguments: []ast.Expression{acc}}
		default:
			return nil, fmt.Errorf("%s is not a simple instruction of a transition condition", op)
		}
	}
	if acc == nil {
		return nil, fmt.Errorf("an empty transition condition")
	}
	return acc, nil
}

// withCurrentResult returns call with the current result, as the IL parser
// marks it (ast.IlCurrentResult), replaced by acc.
func withCurrentResult(call *ast.CallExpression, acc ast.Expression) *ast.CallExpression {
	out := *call
	out.Arguments = make([]ast.Expression, len(call.Arguments))
	for i, a := range call.Arguments {
		switch arg := a.(type) {
		case *ast.IlCurrentResult:
			out.Arguments[i] = acc
		case *ast.NamedArgument:
			if _, isCR := arg.Value.(*ast.IlCurrentResult); isCR {
				named := *arg
				named.Value = acc
				out.Arguments[i] = &named
				continue
			}
			out.Arguments[i] = a
		default:
			out.Arguments[i] = a
		}
	}
	return &out
}

// parseIlTransitionCondition parses the IL condition of a transition after
// its ':', up to END_TRANSITION, which it leaves as the current token.
func (p *Parser) parseIlTransitionCondition(colon token.Token) ast.Expression {
	p.nextToken() // past ':'
	body := p.parseIlProgramBody(token.END_TRANSITION)
	cond, err := ilExpression(body)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("%s at row %d, column %d", err, colon.Row, colon.Column))
		return nil
	}
	return cond
}
