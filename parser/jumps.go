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

package parser

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// checkJumps checks the ST jumps of every POU body (IEC-68), so every engine
// refuses the same sources: a label is used once in its POU body; a jump's
// label is in the same POU body (not in another POU, method or action); and
// a jump may leave an IF, CASE, FOR, WHILE or REPEAT but not enter one, so
// its label is in the jump's own statement list or one around it (what Go's
// goto allows too).
func (p *Parser) checkJumps(stmts []ast.Statement) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.ProgramDeclaration:
			p.checkPOUJumps(n.Body)
		case *ast.FunctionDeclaration:
			p.checkPOUJumps(n.Body)
		case *ast.FunctionBlockDeclaration:
			p.checkPOUJumps(n.Body)
		case *ast.ActionStatement:
			p.checkPOUJumps(n.Body)
		case *ast.NamespaceDeclaration:
			p.checkJumps(n.Statements)
		}
	}
}

// jumpScope is one POU body's labels and jumps.
type jumpScope struct {
	labels map[string]*ast.LabelStatement
	list   map[string]int // label -> the statement list it is in
	jumps  []pendingJump
	next   int
}

type pendingJump struct {
	jump  *ast.JumpStatement
	lists []int // the jump's statement list and those around it
}

func (p *Parser) checkPOUJumps(body ast.Statement) {
	if body == nil {
		return
	}
	sc := &jumpScope{labels: map[string]*ast.LabelStatement{}, list: map[string]int{}}
	p.walkJumps(sc, body, nil)
	for _, j := range sc.jumps {
		key := strings.ToUpper(j.jump.Label.Value)
		label, ok := sc.labels[key]
		row, col := j.jump.Pos()
		switch {
		case !ok:
			p.errors = append(p.errors, fmt.Sprintf("JMP %s: no label %s in this body at row %d, column %d", j.jump.Label.Value, j.jump.Label.Value, row, col))
		case !contains(j.lists, sc.list[key]):
			lr, lc := label.Pos()
			p.errors = append(p.errors, fmt.Sprintf("JMP %s: a jump may leave an IF, CASE or loop but not enter one (label at row %d, column %d) at row %d, column %d", j.jump.Label.Value, lr, lc, row, col))
		}
	}
}

func contains(lists []int, id int) bool {
	for _, l := range lists {
		if l == id {
			return true
		}
	}
	return false
}

// walkJumps records the labels and jumps of a statement and what it holds;
// outer are the statement lists around it.
func (p *Parser) walkJumps(sc *jumpScope, s ast.Statement, outer []int) {
	list := func(b *ast.BlockStatement) {
		if b == nil {
			return
		}
		id := sc.next
		sc.next++
		lists := append(append([]int(nil), outer...), id)
		for _, st := range b.Statements {
			p.walkStatementJumps(sc, st, id, lists)
		}
	}
	switch n := s.(type) {
	case *ast.BlockStatement:
		list(n)
	default:
		// A body that is a single statement.
		list(&ast.BlockStatement{Statements: []ast.Statement{s}})
	}
}

func (p *Parser) walkStatementJumps(sc *jumpScope, s ast.Statement, id int, lists []int) {
	switch n := s.(type) {
	case *ast.LabelStatement:
		key := strings.ToUpper(n.Name.Value)
		if first, dup := sc.labels[key]; dup {
			fr, fc := first.Pos()
			row, col := n.Pos()
			p.errors = append(p.errors, fmt.Sprintf("label %s is already defined in this body (at row %d, column %d) at row %d, column %d", n.Name.Value, fr, fc, row, col))
			return
		}
		sc.labels[key], sc.list[key] = n, id
	case *ast.JumpStatement:
		sc.jumps = append(sc.jumps, pendingJump{jump: n, lists: lists})
	case *ast.BlockStatement:
		p.walkJumps(sc, n, lists)
	case *ast.IfStatement:
		for cur := n; cur != nil; {
			p.walkJumps(sc, cur.Consequence, lists)
			switch alt := cur.Alternative.(type) {
			case *ast.IfStatement:
				cur = alt
				continue
			case *ast.BlockStatement:
				p.walkJumps(sc, alt, lists)
			}
			cur = nil
		}
	case *ast.ForLoopStatement:
		p.walkJumps(sc, n.Body, lists)
	case *ast.WhileStatement:
		p.walkJumps(sc, n.Body, lists)
	case *ast.RepeatStatement:
		p.walkJumps(sc, n.Body, lists)
	case *ast.CaseStatement:
		for _, br := range n.Cases {
			p.walkJumps(sc, br.Consequence, lists)
		}
		p.walkJumps(sc, n.Alternative, lists)
	case *ast.MethodImplementation:
		p.checkPOUJumps(n.Body) // a body of its own
	case *ast.ActionStatement:
		p.checkPOUJumps(n.Body)
	}
}
