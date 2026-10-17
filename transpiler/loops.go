/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// EXIT and CONTINUE act on the loop they are in. A Go break inside a switch
// (a CASE) would leave only the switch, so a loop with an EXIT or CONTINUE
// of its own is labelled, and they name it: break loopN, continue loopN. A
// loop without them is written as it was, unlabelled (Go refuses a label
// that is not used).

import (
	"fmt"

	"github.com/apiarytech/beedance/ast"
)

// loopExits reports whether a loop body has, of its own (not of a loop
// nested in it), an EXIT inside a CASE, and a CONTINUE.
func loopExits(body *ast.BlockStatement) (exitInCase, cont bool) {
	var walk func(s ast.Statement, inCase bool)
	walk = func(s ast.Statement, inCase bool) {
		switch n := s.(type) {
		case *ast.ExitStatement:
			exitInCase = exitInCase || inCase
		case *ast.ContinueStatement:
			cont = true
		case *ast.BlockStatement:
			if n != nil {
				for _, x := range n.Statements {
					walk(x, inCase)
				}
			}
		case *ast.IfStatement:
			if n.Consequence != nil {
				walk(n.Consequence, inCase)
			}
			if n.Alternative != nil {
				walk(n.Alternative, inCase)
			}
		case *ast.CaseStatement:
			for _, b := range n.Cases {
				if b.Consequence != nil {
					walk(b.Consequence, true)
				}
			}
			if n.Alternative != nil {
				walk(n.Alternative, true)
			}
		}
	}
	if body != nil {
		walk(body, false)
	}
	return exitInCase, cont
}

// enterLoop writes the label of a loop whose body has a CONTINUE, or an
// EXIT inside a CASE, of its own, and makes it the loop EXIT and CONTINUE
// name; it returns whether the body has a CONTINUE.
func (t *Transpiler) enterLoop(body *ast.BlockStatement) bool {
	exitInCase, cont := loopExits(body)
	label := ""
	if exitInCase || cont {
		t.loopSeq++
		label = fmt.Sprintf("loop%d", t.loopSeq)
		t.write("%s:\n", label)
	}
	t.loopLabels = append(t.loopLabels, label)
	return cont
}

// leaveLoop ends the loop enterLoop began.
func (t *Transpiler) leaveLoop() {
	t.loopLabels = t.loopLabels[:len(t.loopLabels)-1]
}

// loopLabel is the label EXIT and CONTINUE name, or "" outside a labelled
// loop.
func (t *Transpiler) loopLabel() string {
	if len(t.loopLabels) == 0 {
		return ""
	}
	return t.loopLabels[len(t.loopLabels)-1]
}

// transpileContinueStatement writes CONTINUE as a continue of its loop.
func (t *Transpiler) transpileContinueStatement() error {
	if label := t.loopLabel(); label != "" {
		t.write("\tcontinue %s\n", label)
		return nil
	}
	t.write("\tcontinue\n")
	return nil
}
