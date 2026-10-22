/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"github.com/apiarytech/beedance/ast"
)

// The lines a breakpoint can be on, read from the syntax tree, for an
// engine that has no line table of its own (the evaluator, a transpiled
// program): the lines the statements that run start on, the same kinds
// the compiler marks for the VM (compiler.markStatement).

// statementRows are the lines the statements of node start on, and the
// first of node's own statements in source order (0 if it has none).
func statementRows(node ast.Node, rows map[int]bool) (first int) {
	if node == nil {
		return 0
	}
	ast.Modify(node, func(n ast.Node) ast.Node {
		switch n.(type) {
		case *ast.AssignmentStatement, *ast.ExpressionStatement, *ast.IfStatement,
			*ast.CaseStatement, *ast.ForLoopStatement, *ast.WhileStatement,
			*ast.RepeatStatement, *ast.ReturnStatement, *ast.ExitStatement,
			*ast.ContinueStatement, *ast.JumpStatement, *ast.IlInstructionStatement:
		default:
			return n
		}
		row, _ := safePos(n)
		if row > 0 {
			rows[row] = true
			if first == 0 || row < first {
				first = row
			}
		}
		return n
	})
	return first
}

// safePos is node's position, or 0 for a node that has none.
func safePos(node ast.Node) (row, col int) {
	defer func() {
		if recover() != nil {
			row, col = 0, 0
		}
	}()
	return node.Pos()
}

// sourceRows are the lines a statement starts on in img's body and in the
// source's own POUs, and the body's first statement's line.
func sourceRows(img *image) (rows map[int]bool, first int) {
	rows = map[int]bool{}
	first = statementRows(img.decl.Body, rows)
	for _, st := range img.program.Statements {
		switch n := st.(type) {
		case *ast.FunctionBlockDeclaration:
			statementRows(n, rows)
		case *ast.FunctionDeclaration:
			statementRows(n, rows)
		}
	}
	return rows, first
}
