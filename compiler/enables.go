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
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lower"
)

// edgesThenEnables gives edge inputs their meaning (lower.Edges), then a
// POU's own EN and ENO theirs around the whole body (lower.Enables,
// IEC-75), the same on every engine.
func edgesThenEnables(program *ast.Program) error {
	if err := lower.Edges(program); err != nil {
		return err
	}
	return lower.Enables(program)
}
