/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdcalls

// The functions that split a time data type (IEC 61131-3, Table 36):
//
//	SPLIT_DT(IN := dt, YEAR => y, MONTH => m, DAY => d, HOUR => h,
//	         MINUTE => mi, SECOND => s, MILLISECOND => ms);
//
// have only outputs, which no engine gives a built-in function, so a call
// is the assignments of each output, y := $DATEPART(dt, 'YEAR'); ..., the
// built-in $DATEPART (package stdlib) taking a part of a date or time.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
)

// splitOutputs are the outputs of each split function, in order.
var splitOutputs = map[string][]string{
	"SPLIT_DATE": {"YEAR", "MONTH", "DAY"},
	"SPLIT_TOD":  {"HOUR", "MINUTE", "SECOND", "MILLISECOND"},
	"SPLIT_LTOD": {"HOUR", "MINUTE", "SECOND", "MILLISECOND"},
	"SPLIT_DT":   {"YEAR", "MONTH", "DAY", "HOUR", "MINUTE", "SECOND", "MILLISECOND"},
	"SPLIT_LDT":  {"YEAR", "MONTH", "DAY", "HOUR", "MINUTE", "SECOND", "MILLISECOND"},
}

// splits rewrites the calls of split functions in the program's statement
// lists, unless the program declares a function of that name.
func splits(program *ast.Program, declared map[string]bool) {
	ast.Modify(program, func(n ast.Node) ast.Node {
		block, ok := n.(*ast.BlockStatement)
		if !ok {
			return n
		}
		var out []ast.Statement
		changed := false
		for _, s := range block.Statements {
			if assigns, ok := splitCall(s, declared); ok {
				out = append(out, assigns...)
				changed = true
				continue
			}
			out = append(out, s)
		}
		if changed {
			block.Statements = out
		}
		return n
	})
}

// splitCall returns the assignments a statement that calls a split
// function is, or false.
func splitCall(s ast.Statement, declared map[string]bool) ([]ast.Statement, bool) {
	es, ok := s.(*ast.ExpressionStatement)
	if !ok {
		return nil, false
	}
	call, ok := es.Expression.(*ast.CallExpression)
	if !ok {
		return nil, false
	}
	ident, ok := call.Function.(*ast.Identifier)
	if !ok {
		return nil, false
	}
	name := strings.ToUpper(ident.Value)
	outputs, ok := splitOutputs[name]
	if !ok || declared[name] {
		return nil, false
	}
	var in ast.Expression
	targets := map[string]ast.Expression{}
	for _, a := range call.Arguments {
		switch arg := a.(type) {
		case *ast.NamedArgument:
			if !strings.EqualFold(arg.Name.Value, "IN") {
				return nil, false
			}
			in = arg.Value
		case *ast.OutputArgument:
			targets[strings.ToUpper(arg.Source.Value)] = arg.Target
		default:
			if in != nil {
				return nil, false
			}
			in = arg // SPLIT_DT(dt, YEAR => y, ...)
		}
	}
	if in == nil {
		return nil, false
	}
	tok := call.Token
	var assigns []ast.Statement
	for _, output := range outputs {
		target, ok := targets[output]
		if !ok {
			continue
		}
		delete(targets, output)
		part := &ast.CallExpression{
			Token:     tok,
			Function:  &ast.Identifier{Token: tok, Value: "$DATEPART"},
			Arguments: []ast.Expression{in, &ast.StringLiteral{Token: tok, Value: output}},
		}
		assigns = append(assigns, &ast.AssignmentStatement{
			Token: token.Token{Type: token.ASSIGN, Literal: ":=", Row: tok.Row, Column: tok.Column, Pos: tok.Pos},
			Left:  target,
			Value: part,
		})
	}
	if len(targets) > 0 {
		return nil, false // an output the function does not have: left to report
	}
	return assigns, true
}
