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

// A VAR_IN_OUT declared ARRAY[*] OF T (IEC 61131-3 3rd edition) takes an
// array of any length. A Go array's length is part of its type, so the
// parameter is an *iecAny[T]: the array's lower bound and a slice of its
// elements, which shares them with the caller's array. v[i] is
// (*v.At(i)), LOWER_BOUND(v, 1) is v.lo, UPPER_BOUND(v, 1) is
// v.lo + len(v.data) - 1. Arrays of one dimension only.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// anyArrayHelper is the type of an ARRAY[*] parameter.
const anyArrayHelper = `
// iecAny is an ARRAY[*] VAR_IN_OUT: the lower bound of the array given and
// its elements, shared with it.
type iecAny[T any] struct {
	lo   int64
	data []T
}

// At returns the element of index i, from the array's lower bound.
func (a *iecAny[T]) At(i int64) *T { return &a.data[i-a.lo] }
`

// anyArrayType returns the Go type of an ARRAY[*] OF T parameter.
func (t *Transpiler) anyArrayType(def *ast.ArrayDefinition) string {
	t.usesAnyArray = true
	return "iecAny[" + t.mapIecTypeToGo(def.DataType) + "]"
}

// anyArrayArgument returns the argument of an ARRAY[*] VAR_IN_OUT: an
// ARRAY[*] parameter passed on as it is, or a declared array's lower bound
// and elements.
func (t *Transpiler) anyArrayArgument(param *ast.ArrayDefinition, value ast.Expression) (string, error) {
	v, err := t.expressionString(value)
	if err != nil {
		return "", err
	}
	def := t.arrayDefinitionOf(t.valueDataType(value))
	if def == nil {
		return "", fmt.Errorf("the argument for an ARRAY[*] must be an array, got %s", value.String())
	}
	if def.HasAnyRange() {
		return t.anyArrayPointer(value, v), nil
	}
	if len(def.Ranges) != 1 {
		return "", fmt.Errorf("an ARRAY[*] of more than one dimension is not supported by the Go transpiler")
	}
	low := int64(0)
	if rng, ok := def.Ranges[0].(*ast.InfixExpression); ok {
		low, _ = constantInteger(rng.Left)
	}
	return fmt.Sprintf("&%s{lo: %d, data: %s[:]}", t.anyArrayType(param), low, v), nil
}

// anyArrayPointer returns the *iecAny of an ARRAY[*] parameter given its Go
// expression, which dereferences it: (*v) is v.
func (t *Transpiler) anyArrayPointer(_ ast.Expression, goExpr string) string {
	if len(goExpr) > 3 && goExpr[:2] == "(*" && goExpr[len(goExpr)-1] == ')' {
		return goExpr[2 : len(goExpr)-1]
	}
	return "&" + goExpr
}

// namesLongTimeType reports whether a conversion's name names a long time
// type, LTIME_TO_STRING or TIME_TO_LDT.
func namesLongTimeType(name string) bool {
	from, to, _ := strings.Cut(strings.ToUpper(name), "_TO_")
	for _, part := range []string{from, to} {
		switch part {
		case "LTIME", "LDATE", "LTOD", "LTIME_OF_DAY", "LDT", "LDATE_AND_TIME":
			return true
		}
	}
	return false
}
