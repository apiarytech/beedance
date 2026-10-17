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

// IS_VALID and IS_VALID_BCD (IEC 61131-3 3rd edition, Table 39), as
// stdlib/validate.go has them.

import (
	"fmt"

	"github.com/apiarytech/beedance/ast"
)

// transpileValidate writes a call of IS_VALID or IS_VALID_BCD; done is
// false if name is neither.
func (t *Transpiler) transpileValidate(name string, exp *ast.CallExpression) (done bool, err error) {
	if name != "IS_VALID" && name != "IS_VALID_BCD" {
		return false, nil
	}
	if len(exp.Arguments) != 1 {
		return true, fmt.Errorf("%s takes 1 argument, got %d", name, len(exp.Arguments))
	}
	t.usesValidate = true
	if name == "IS_VALID" {
		t.write("iecIsValid(float64(")
	} else {
		t.write("iecIsValidBCD(uint64(")
	}
	if err := t.transpileExpression(exp.Arguments[0]); err != nil {
		return true, err
	}
	t.write("))")
	return true, nil
}

// validateHelpers are IS_VALID and IS_VALID_BCD.
const validateHelpers = `
func iecIsValid(f float64) iec.BOOL {
	return iec.BOOL(!gomath.IsNaN(f) && !gomath.IsInf(f, 0))
}

func iecIsValidBCD(v uint64) iec.BOOL {
	for ; v != 0; v >>= 4 {
		if v&0xF > 9 {
			return false
		}
	}
	return true
}
`
