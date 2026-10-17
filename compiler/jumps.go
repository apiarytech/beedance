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

package compiler

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/code"
)

// ST jumps (IEC-68): JMP label; is an OpJump to where label: is, patched
// when the label comes later, as IL's jumps are (compiler_il.go). The parser
// has checked that the label is in the same POU body and that no jump enters
// an IF, CASE or loop. A loop keeps nothing on the VM's stack between its
// statements (EXIT is a plain jump too), so a jump may leave one as it is.

// stJumps are the labels and unpatched jumps of the POU body being compiled.
type stJumps struct {
	labels  map[string]int   // label -> its position
	pending map[string][]int // label -> jumps to it not yet patched
}

// jumpState is the current scope's: each POU body compiles in a scope of its
// own (enterScope starts it afresh).
func (c *Compiler) jumpState() *stJumps {
	if c.stJumps == nil {
		c.stJumps = map[int]*stJumps{}
	}
	s := c.stJumps[c.scopeIndex]
	if s == nil {
		s = &stJumps{labels: map[string]int{}, pending: map[string][]int{}}
		c.stJumps[c.scopeIndex] = s
	}
	return s
}

func (c *Compiler) compileLabel(node *ast.LabelStatement) {
	s := c.jumpState()
	key := strings.ToUpper(node.Name.Value)
	pos := len(c.currentInstructions())
	s.labels[key] = pos
	for _, j := range s.pending[key] {
		c.changeOperand(j, pos)
	}
	delete(s.pending, key)
}

func (c *Compiler) compileJump(node *ast.JumpStatement) {
	s := c.jumpState()
	key := strings.ToUpper(node.Label.Value)
	pos := c.emit(code.OpJump, 9999)
	if target, seen := s.labels[key]; seen {
		c.changeOperand(pos, target) // a jump back
		return
	}
	s.pending[key] = append(s.pending[key], pos)
}
