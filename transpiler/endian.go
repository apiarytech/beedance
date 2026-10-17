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

// TO_BIG_ENDIAN, TO_LITTLE_ENDIAN, FROM_BIG_ENDIAN and FROM_LITTLE_ENDIAN
// (IEC 61131-3 3rd edition, Table 37), as stdlib/endian.go has them: a value
// is little-endian, so the big-endian functions reverse its bytes.

import (
	"fmt"

	"github.com/apiarytech/beedance/ast"
)

// endianSizes is the size in bytes of each Go type whose bytes reverse.
var endianSizes = map[string]int{
	"iec.SINT": 1, "iec.USINT": 1, "iec.BYTE": 1,
	"iec.INT": 2, "iec.UINT": 2, "iec.WORD": 2,
	"iec.DINT": 4, "iec.UDINT": 4, "iec.DWORD": 4,
	"iec.LINT": 8, "iec.ULINT": 8, "iec.LWORD": 8,
}

// transpileEndian writes a call of a function of byte order; done is false
// if name is not one.
func (t *Transpiler) transpileEndian(name string, exp *ast.CallExpression) (done bool, err error) {
	var swap bool
	switch name {
	case "TO_BIG_ENDIAN", "FROM_BIG_ENDIAN":
		swap = true
	case "TO_LITTLE_ENDIAN", "FROM_LITTLE_ENDIAN":
	default:
		return false, nil
	}
	if len(exp.Arguments) != 1 {
		return true, fmt.Errorf("%s takes 1 argument, got %d", name, len(exp.Arguments))
	}
	in := exp.Arguments[0]
	goType := t.exprGoType(in)
	size, isInt := endianSizes[goType]
	if !isInt && goType != "iec.REAL" && goType != "iec.LREAL" {
		return true, fmt.Errorf("the argument of %s must be an integer, a bit string or a real of a known type", name)
	}
	if !swap {
		return true, t.transpileExpression(in)
	}
	t.usesEndian = true
	switch goType {
	case "iec.REAL":
		t.write("iecSwapReal(")
	case "iec.LREAL":
		t.write("iecSwapLReal(")
	default:
		t.write("iecSwapBytes(%s(", goType)
	}
	if err := t.transpileExpression(in); err != nil {
		return true, err
	}
	if isInt {
		t.write("), %d)", size)
	} else {
		t.write(")")
	}
	return true, nil
}

// endianHelpers reverse the bytes of a value of size bytes.
const endianHelpers = `
func iecSwapBytes[T ~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~uint32 | ~int64 | ~uint64](v T, size int) T {
	return T(gobits.ReverseBytes64(uint64(v)) >> (64 - 8*size))
}

func iecSwapReal[T ~float32](v T) T {
	return T(gomath.Float32frombits(gobits.ReverseBytes32(gomath.Float32bits(float32(v)))))
}

func iecSwapLReal[T ~float64](v T) T {
	return T(gomath.Float64frombits(gobits.ReverseBytes64(gomath.Float64bits(float64(v)))))
}
`
