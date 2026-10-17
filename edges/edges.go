/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package edges gives the R_EDGE and F_EDGE inputs of function blocks their
// meaning, for every engine alike. An input declared
//
//	VAR_INPUT start : BOOL R_EDGE; END_VAR
//
// is TRUE in the body only on the call it rises (F_EDGE: falls). The body
// starts by keeping the value the caller gave, running an R_TRIG (F_TRIG)
// on it and setting the input to the trigger's Q; it ends, and returns, by
// putting the caller's value back, so that a call that does not give the
// input sees the value it had.
package edges

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// Lower gives the edge inputs of the program's function blocks (in
// namespaces too) their meaning, in place. A function block already
// lowered is left as it is, so Lower may run twice on a program.
func Lower(program *ast.Program) error {
	if program == nil {
		return nil
	}
	return lowerStatements(program.Statements)
}

func lowerStatements(stmts []ast.Statement) error {
	for _, s := range stmts {
		switch d := s.(type) {
		case *ast.FunctionBlockDeclaration:
			if err := lowerFunctionBlock(d); err != nil {
				return err
			}
		case *ast.NamespaceDeclaration:
			if err := lowerStatements(d.Statements); err != nil {
				return err
			}
		}
	}
	return nil
}

// rawName and trigName are the hidden variables of an edge input: the value
// the caller gave, and its trigger.
func rawName(input string) string  { return "_edge_" + input + "_raw" }
func trigName(input string) string { return "_edge_" + input + "_trig" }

func lowerFunctionBlock(fb *ast.FunctionBlockDeclaration) error {
	body, ok := fb.Body.(*ast.BlockStatement)
	if !ok || body == nil {
		return nil // a chart or a diagram body: see the package comment
	}
	var inputs []*ast.VarDeclStatement
	for _, d := range fb.VarInputs {
		if d != nil && d.Name != nil && (d.IsRisingEdge || d.IsFallingEdge) {
			inputs = append(inputs, d)
		}
	}
	if len(inputs) == 0 {
		return nil
	}
	for _, d := range fb.Vars {
		if d != nil && d.Name != nil && strings.EqualFold(d.Name.Value, rawName(inputs[0].Name.Value)) {
			return nil // lowered already
		}
	}

	var decls, start, restore strings.Builder
	for _, in := range inputs {
		name := in.Name.Value
		trig := "R_TRIG"
		if in.IsFallingEdge {
			trig = "F_TRIG"
		}
		fmt.Fprintf(&decls, "%s : BOOL; %s : %s; ", rawName(name), trigName(name), trig)
		// The input is written through THIS: the body may not assign its
		// inputs by name (the VM's compiler refuses it).
		fmt.Fprintf(&start, "%s := %s; %s(CLK := %s); THIS.%s := %s.Q;\n", rawName(name), name, trigName(name), rawName(name), name, trigName(name))
		fmt.Fprintf(&restore, "THIS.%s := %s;\n", name, rawName(name))
	}
	vars, startStmts, err := parse(decls.String(), start.String())
	if err != nil {
		return fmt.Errorf("%s: %w", fb.Name.Value, err)
	}
	_, restoreStmts, err := parse("", restore.String())
	if err != nil {
		return fmt.Errorf("%s: %w", fb.Name.Value, err)
	}

	fb.Vars = append(fb.Vars, vars...)
	// The body's own statements, with the inputs put back before a RETURN;
	// its methods are left alone.
	var own, methods []ast.Statement
	for _, s := range body.Statements {
		if _, isMethod := s.(*ast.MethodImplementation); isMethod {
			methods = append(methods, s)
			continue
		}
		own = append(own, s)
	}
	own = restoreBeforeReturns(own, restoreStmts)
	stmts := append(append(append(append([]ast.Statement{}, startStmts...), own...), restoreStmts...), methods...)
	body.Statements = stmts
	return nil
}

// parse parses variable declarations and statements as a program's.
func parse(decls, stmts string) ([]*ast.VarDeclStatement, []ast.Statement, error) {
	src := "PROGRAM _edges\n"
	if decls != "" {
		src += "VAR " + decls + "END_VAR\n"
	}
	src += stmts + "END_PROGRAM\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, nil, fmt.Errorf("the edge inputs lowered to ST that does not parse (a beedance defect): %s", errs[0])
	}
	pd, ok := prog.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		return nil, nil, fmt.Errorf("the edge inputs did not lower to a program (a beedance defect)")
	}
	body, _ := pd.Body.(*ast.BlockStatement)
	if body == nil {
		return pd.Vars, nil, nil
	}
	return pd.Vars, body.Statements, nil
}

// restoreBeforeReturns returns stmts with restore put before each RETURN,
// in nested IF, CASE and loop bodies too.
func restoreBeforeReturns(stmts []ast.Statement, restore []ast.Statement) []ast.Statement {
	out := make([]ast.Statement, 0, len(stmts))
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.ReturnStatement:
			out = append(out, restore...)
		case *ast.BlockStatement:
			if n != nil {
				n.Statements = restoreBeforeReturns(n.Statements, restore)
			}
		case *ast.IfStatement:
			restoreInIf(n, restore)
		case *ast.CaseStatement:
			for _, b := range n.Cases {
				if b.Consequence != nil {
					b.Consequence.Statements = restoreBeforeReturns(b.Consequence.Statements, restore)
				}
			}
			if n.Alternative != nil {
				n.Alternative.Statements = restoreBeforeReturns(n.Alternative.Statements, restore)
			}
		case *ast.ForLoopStatement:
			if n.Body != nil {
				n.Body.Statements = restoreBeforeReturns(n.Body.Statements, restore)
			}
		case *ast.WhileStatement:
			if n.Body != nil {
				n.Body.Statements = restoreBeforeReturns(n.Body.Statements, restore)
			}
		case *ast.RepeatStatement:
			if n.Body != nil {
				n.Body.Statements = restoreBeforeReturns(n.Body.Statements, restore)
			}
		}
		out = append(out, s)
	}
	return out
}

func restoreInIf(n *ast.IfStatement, restore []ast.Statement) {
	if n.Consequence != nil {
		n.Consequence.Statements = restoreBeforeReturns(n.Consequence.Statements, restore)
	}
	switch alt := n.Alternative.(type) {
	case *ast.IfStatement:
		restoreInIf(alt, restore)
	case *ast.BlockStatement:
		if alt != nil {
			alt.Statements = restoreBeforeReturns(alt.Statements, restore)
		}
	}
}
