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

package transpiler

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// ST jumps in Go (IEC-68): JMP label; is goto L_LABEL and label: is
// L_LABEL:, as IL's are labels and gotos (transpileIlInstruction). Go's goto
// may leave a block but not enter one, which is the rule the parser checked.
// It must not jump over a variable declaration of the label's block either,
// and the transpiler declares temporaries as it goes; so in a statement list
// that holds a label, each statement goes in a block of its own. A label
// nobody jumps to is an error in Go, so each label is also "used" by a goto
// that never runs.

// goLabel is a label's Go name: IEC names ignore case, and the prefix keeps
// Go's keywords out.
func goLabel(name string) string { return "L_" + strings.ToUpper(name) }

// holdsLabel reports whether a statement list has a label of its own.
func holdsLabel(stmts []ast.Statement) bool {
	for _, s := range stmts {
		if _, ok := s.(*ast.LabelStatement); ok {
			return true
		}
	}
	return false
}

// transpileLabelledList writes a statement list holding a label.
func (t *Transpiler) transpileLabelledList(stmts []ast.Statement) error {
	for _, s := range stmts {
		if l, ok := s.(*ast.LabelStatement); ok {
			name := goLabel(l.Name.Value)
			t.write("\tif false {\n\t\tgoto %s\n\t}\n", name)
			t.write("%s:\n\t{\n\t}\n", name)
			continue
		}
		t.write("\t{\n")
		if err := t.transpileNode(s); err != nil {
			return err
		}
		t.write("\t}\n")
	}
	return nil
}

func (t *Transpiler) transpileJump(j *ast.JumpStatement) error {
	t.write("\tgoto %s\n", goLabel(j.Label.Value))
	return nil
}
