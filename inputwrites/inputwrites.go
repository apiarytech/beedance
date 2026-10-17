/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under:
 * - GPL v2.0
 * - Commercial
 *
 * You may choose to use this software under the terms of either license.
 * See the LICENSE files in the project root for full license text.
 */

// Package inputwrites checks that no POU assigns its own inputs (IEC-69),
// for the evaluator and the transpiler, as the VM's compiler does: IEC
// 61131-3 makes a VAR_INPUT read-only inside its POU. CODESYS allows the
// write, and libraries written for it (OSCAT, beeguard's alarm blocks)
// clear their one-shot requests so, which each engine allows when its
// AllowInputWrites option is on.
//
// What is refused is what the VM refuses: an assignment (:= or ?=) to an
// input by name, in a function, a function block (its own inputs and those
// of the blocks it extends) or a method (its own inputs and its block's). A
// member or an element of an input (a.b, a[1]), a FOR counting with one and
// IL's ST store into it are not refused, as the VM does not; nor is a write
// through THIS (edges' lowering of R_EDGE inputs).
package inputwrites

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// Check returns the first assignment of an input in program, as the VM's
// error, or nil.
func Check(program *ast.Program) error {
	if program == nil {
		return nil
	}
	fbs := map[string]*ast.FunctionBlockDeclaration{}
	collect(program.Statements, fbs)
	return check(program.Statements, fbs)
}

func collect(stmts []ast.Statement, fbs map[string]*ast.FunctionBlockDeclaration) {
	for _, s := range stmts {
		switch d := s.(type) {
		case *ast.FunctionBlockDeclaration:
			if d.Name != nil {
				fbs[strings.ToUpper(d.Name.Value)] = d
			}
		case *ast.NamespaceDeclaration:
			collect(d.Statements, fbs)
		}
	}
}

func check(stmts []ast.Statement, fbs map[string]*ast.FunctionBlockDeclaration) error {
	for _, s := range stmts {
		switch d := s.(type) {
		case *ast.FunctionDeclaration:
			if err := body(d.Body, names(nil, d.VarInputs)); err != nil {
				return err
			}
		case *ast.FunctionBlockDeclaration:
			inputs := chainInputs(d, fbs)
			blk, ok := d.Body.(*ast.BlockStatement)
			if !ok {
				if err := body(d.Body, inputs); err != nil {
					return err
				}
				continue
			}
			for _, st := range blk.Statements {
				if m, isMethod := st.(*ast.MethodImplementation); isMethod {
					if err := body(m.Body, names(inputs, m.VarInputs)); err != nil {
						return err
					}
					continue
				}
				if err := statement(st, inputs); err != nil {
					return err
				}
			}
		case *ast.NamespaceDeclaration:
			if err := check(d.Statements, fbs); err != nil {
				return err
			}
		}
	}
	return nil
}

// chainInputs are a block's inputs and those of the blocks it extends.
func chainInputs(fb *ast.FunctionBlockDeclaration, fbs map[string]*ast.FunctionBlockDeclaration) map[string]bool {
	out := map[string]bool{}
	seen := map[*ast.FunctionBlockDeclaration]bool{}
	for cur := fb; cur != nil && !seen[cur]; {
		seen[cur] = true
		out = names(out, cur.VarInputs)
		if cur.Extends == nil {
			break
		}
		name := cur.Extends.String()
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		cur = fbs[strings.ToUpper(name)]
	}
	return out
}

func names(into map[string]bool, decls []*ast.VarDeclStatement) map[string]bool {
	out := map[string]bool{}
	for k := range into {
		out[k] = true
	}
	for _, d := range decls {
		if d != nil && d.Name != nil {
			out[strings.ToUpper(d.Name.Value)] = true
		}
	}
	return out
}

func body(b ast.Statement, inputs map[string]bool) error {
	if b == nil || len(inputs) == 0 {
		return nil
	}
	return statement(b, inputs)
}

// statement checks a statement and those it holds.
func statement(s ast.Statement, inputs map[string]bool) error {
	switch n := s.(type) {
	case *ast.AssignmentStatement:
		if id, ok := n.Left.(*ast.Identifier); ok && inputs[strings.ToUpper(id.Value)] {
			row, col := id.Pos()
			return fmt.Errorf("cannot assign to read-only variable '%s' at row %d, column %d", id.Value, row, col)
		}
	case *ast.BlockStatement:
		if n == nil {
			return nil
		}
		for _, st := range n.Statements {
			if err := statement(st, inputs); err != nil {
				return err
			}
		}
	case *ast.IfStatement:
		if err := statement(n.Consequence, inputs); err != nil {
			return err
		}
		if n.Alternative != nil {
			return statement(n.Alternative, inputs)
		}
	case *ast.CaseStatement:
		for _, br := range n.Cases {
			if err := statement(br.Consequence, inputs); err != nil {
				return err
			}
		}
		if n.Alternative != nil {
			return statement(n.Alternative, inputs)
		}
	case *ast.ForLoopStatement:
		return statement(n.Body, inputs)
	case *ast.WhileStatement:
		return statement(n.Body, inputs)
	case *ast.RepeatStatement:
		return statement(n.Body, inputs)
	}
	return nil
}
