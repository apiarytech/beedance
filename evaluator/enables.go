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

package evaluator

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
