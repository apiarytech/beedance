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
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/edges"
	"github.com/apiarytech/beedance/enables"
)

// edgesThenEnables gives edge inputs their meaning (package edges), then a
// POU's own EN and ENO theirs around the whole body (package enables,
// IEC-75), the same on every engine.
func edgesThenEnables(program *ast.Program) error {
	if err := edges.Lower(program); err != nil {
		return err
	}
	return enables.Lower(program)
}

// declaredEnables reports whether a function block, or one it extends,
// declares EN or ENO itself (IEC-75): then the field is its own, named as it
// declares it, and package enables gives it its meaning.
func (t *Transpiler) declaredEnables(fb *ast.FunctionBlockDeclaration) (en, eno bool) {
	for seen := map[*ast.FunctionBlockDeclaration]bool{}; fb != nil && !seen[fb]; {
		seen[fb] = true
		e, o := enables.Declares(fb.VarInputs, fb.VarOutputs)
		en, eno = en || e, eno || o
		if fb.Extends == nil {
			break
		}
		fb = t.lookupFunctionBlock(fb.Extends)
	}
	return en, eno
}

// startsEN reports whether an instance of dataType starts with the engine's
// own EN set TRUE: one of royaljelly's blocks that has EN, or a user block
// that does not declare EN itself (its declaration gives its start).
func (t *Transpiler) startsEN(dataType ast.Expression) bool {
	if std, isStd := t.standardFunctionBlockOf(dataType); isStd {
		return std.hasEN
	}
	en, _ := t.declaredEnables(t.lookupFunctionBlock(dataType))
	return !en
}
