/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// A debugger needs to know which statement an instruction belongs to (for
// breakpoints and stepping) and what a function's local slots are called.
// The compiler keeps both per scope: a line mark where each statement that
// runs starts, and, when the scope is left, the names of its locals. They
// go with the code into Bytecode.Lines and object.CompiledFunction.

// markStatement records that node, if it is a statement that runs, starts
// at the next instruction of the current scope.
func (c *Compiler) markStatement(node ast.Node) {
	switch node.(type) {
	case *ast.AssignmentStatement, *ast.ExpressionStatement, *ast.IfStatement,
		*ast.CaseStatement, *ast.ForLoopStatement, *ast.WhileStatement,
		*ast.RepeatStatement, *ast.ReturnStatement, *ast.ExitStatement,
		*ast.ContinueStatement, *ast.JumpStatement, *ast.IlInstructionStatement:
	default:
		return
	}
	row, col := safePos(node)
	if row <= 0 {
		return // made by a lowering, with no place in the source
	}
	scope := &c.scopes[c.scopeIndex]
	mark := object.SourceLine{Offset: len(scope.instructions), Row: row, Column: col}
	if n := len(scope.lines); n > 0 && scope.lines[n-1].Offset == mark.Offset {
		// The last statement emitted nothing of its own before this one,
		// which is inside it: this is the one that runs.
		scope.lines[n-1] = mark
		return
	}
	scope.lines = append(scope.lines, mark)
}

// trimLines drops the line marks of the current scope at or past pos, when
// the instructions from pos on are dropped.
func (c *Compiler) trimLines(pos int) {
	scope := &c.scopes[c.scopeIndex]
	n := len(scope.lines)
	for n > 0 && scope.lines[n-1].Offset >= pos {
		n--
	}
	scope.lines = scope.lines[:n]
}

// scopeLines are the line marks of the current scope that point at an
// instruction.
func (c *Compiler) scopeLines() []object.SourceLine {
	scope := c.scopes[c.scopeIndex]
	out := scope.lines
	for len(out) > 0 && out[len(out)-1].Offset >= len(scope.instructions) {
		out = out[:len(out)-1]
	}
	return out
}

// leftScope is what leaveScope keeps of the scope it leaves for the
// function compiled from it.
type leftScope struct {
	lines  []object.SourceLine
	locals []string
}

// noteLeaving keeps the line marks and local names of the current scope
// before leaveScope drops it.
func (c *Compiler) noteLeaving() {
	c.left = leftScope{lines: c.scopeLines(), locals: localNames(c.symbolTable)}
}

// localNames are the names of s's local slots by index.
func localNames(s *SymbolTable) []string {
	if s == nil || s.numDefinitions == 0 {
		return nil
	}
	names := make([]string, s.numDefinitions)
	for _, sym := range s.store {
		if sym.Scope == LocalScope && sym.Index >= 0 && sym.Index < len(names) {
			names[sym.Index] = sym.Name
		}
	}
	return names
}

// debugInfo gives fn the name name and what the scope it was compiled in
// left for a debugger.
func (c *Compiler) debugInfo(fn *object.CompiledFunction, name string) *object.CompiledFunction {
	fn.Name = name
	fn.Lines = c.left.lines
	fn.LocalNames = c.left.locals
	return fn
}

// memberName is name qualified by the function block being compiled, if
// any: "Motor.Start" for its method Start.
func (c *Compiler) memberName(name string) string {
	if c.currentFB != nil && c.currentFB.Name != nil {
		return c.currentFB.Name.Value + "." + name
	}
	return name
}
