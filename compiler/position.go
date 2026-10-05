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
	"errors"

	"github.com/apiarytech/beedance/ast"
)

// PositionError is a compile error with the source position of the node
// that failed, for editors (errors.As finds it). Its message is the
// error's own: callers that print errors see no difference.
type PositionError struct {
	Row, Column int
	Err         error
}

func (e *PositionError) Error() string { return e.Err.Error() }

func (e *PositionError) Unwrap() error { return e.Err }

// positioned gives err the position of node, unless it has one already (an
// inner node's) or node has none.
func positioned(node ast.Node, err error) error {
	if err == nil || node == nil {
		return err
	}
	var pe *PositionError
	if errors.As(err, &pe) {
		return err
	}
	row, col := safePos(node)
	if row <= 0 {
		return err
	}
	return &PositionError{Row: row, Column: col, Err: err}
}

// safePos is node's position; a node whose Pos reads a nil field has none.
func safePos(node ast.Node) (row, col int) {
	defer func() {
		if recover() != nil {
			row, col = 0, 0
		}
	}()
	return node.Pos()
}
