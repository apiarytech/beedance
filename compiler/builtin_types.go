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
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// conversionTargets are the types a conversion function X_TO_Y can return,
// by the name it has in the function's name.
var conversionTargets = map[string]object.ObjectType{
	"BOOL": object.BOOLEAN_OBJ, "SINT": object.SINT_OBJ, "INT": object.INT_OBJ, "DINT": object.DINT_OBJ,
	"LINT": object.LINT_OBJ, "USINT": object.USINT_OBJ, "UINT": object.UINT_OBJ, "UDINT": object.UDINT_OBJ,
	"ULINT": object.ULINT_OBJ, "REAL": object.REAL_OBJ, "LREAL": object.LREAL_OBJ,
	"BYTE": object.BYTE_OBJ, "WORD": object.WORD_OBJ, "DWORD": object.DWORD_OBJ, "LWORD": object.LWORD_OBJ,
	"STRING": object.STRING_OBJ, "WSTRING": object.WSTRING_OBJ,
	"TIME": object.TIME_OBJ, "DATE": object.DATE_OBJ, "TOD": object.TIME_OF_DAY_OBJ, "DT": object.DATE_AND_TIME_OBJ,
}

// builtinResultType returns the type of a built-in function's result where
// it is known without running it: a conversion X_TO_Y returns a Y, and a
// shift or rotation of a bit string returns a bit string of the same type.
func (c *Compiler) builtinResultType(name string, args []ast.Expression) (object.ObjectType, bool) {
	upper := strings.ToUpper(name)
	switch upper {
	case "SHL", "SHR", "ROL", "ROR":
		if len(args) == 0 {
			return "", false
		}
		t, err := c.getExpressionType(args[0])
		if err == nil && object.IsBitStringType(string(t)) {
			return t, true
		}
		return "", false
	}
	if i := strings.LastIndex(upper, "_TO_"); i > 0 {
		if t, ok := conversionTargets[upper[i+len("_TO_"):]]; ok {
			return t, true
		}
	}
	return "", false
}
