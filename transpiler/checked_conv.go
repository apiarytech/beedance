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

// A conversion to an integer or bit-string type of a value that does not
// fit it is a runtime error on every engine (the owner's choice for IEC
// 61131-3's "type conversion errors", Annex E), as the evaluator and the VM
// report it: REAL_TO_INT(40000.0), INT_TO_SINT(300), INT_TO_WORD(-5).
// royaljelly's conversions saturate instead, so these are written here.

import (
	"math"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// checkedRanges are the ranges of the integer and bit-string types a
// checked conversion converts to: the lowest and highest value.
var checkedRanges = map[string]struct {
	lo       int64
	hi       uint64
	unsigned bool
}{
	"SINT": {math.MinInt8, math.MaxInt8, false}, "INT": {math.MinInt16, math.MaxInt16, false},
	"DINT": {math.MinInt32, math.MaxInt32, false}, "LINT": {math.MinInt64, math.MaxInt64, false},
	"USINT": {0, math.MaxUint8, true}, "UINT": {0, math.MaxUint16, true},
	"UDINT": {0, math.MaxUint32, true}, "ULINT": {0, math.MaxUint64, true},
	"BYTE": {0, math.MaxUint8, true}, "WORD": {0, math.MaxUint16, true},
	"DWORD": {0, math.MaxUint32, true}, "LWORD": {0, math.MaxUint64, true},
}

// transpileCheckedConversion writes X_TO_Y where Y is an integer or bit
// string type and X a number or TIME, checking that the value fits Y; done is
// false for any other call. A bit string to a bit string keeps its low bits
// (DWORD_TO_BYTE), as the evaluator does, and is left to royaljelly.
func (t *Transpiler) transpileCheckedConversion(name string, exp *ast.CallExpression) (done bool, err error) {
	from, to, ok := object.ParseConversion(name)
	if !ok || len(exp.Arguments) != 1 {
		return false, nil
	}
	target, isChecked := checkedRanges[to]
	if !isChecked {
		return false, nil
	}
	var source string // the helper's kind of input: S signed, U unsigned, F real, T time
	switch {
	case object.IsBitStringType(from) && object.IsBitStringType(to):
		return false, nil
	case from == "SINT" || from == "INT" || from == "DINT" || from == "LINT":
		source = "S"
	case from == "USINT" || from == "UINT" || from == "UDINT" || from == "ULINT" || object.IsBitStringType(from):
		source = "U"
	case from == "REAL" || from == "LREAL":
		source = "F"
	case from == "TIME":
		source = "T"
	default:
		return false, nil
	}
	t.usesCheckedConv = true
	kind := "S"
	if target.unsigned {
		kind = "U"
	}
	t.write("iec.%s(", to)
	switch source {
	case "S":
		t.write("iecConvS%s(int64(", kind)
	case "U":
		t.write("iecConvU%s(uint64(", kind)
	case "F":
		t.write("iecConvF%s(float64(", kind)
	case "T": // a TIME converts as its milliseconds
		t.write("iecConvS%s(int64(time.Duration(", kind)
	}
	if err := t.transpileExpression(exp.Arguments[0]); err != nil {
		return true, err
	}
	if source == "T" {
		t.write(") / time.Millisecond")
	}
	if target.unsigned {
		t.write("), %d, %q))", target.hi, name)
	} else {
		t.write("), %d, %d, %q))", target.lo, target.hi, name)
	}
	return true, nil
}

// checkedConvHelpers are the checks transpileCheckedConversion calls: a
// value that does not fit the target panics, which a test reports as a
// runtime error, as the other engines do.
const checkedConvHelpers = `
func iecConvFail(name string, v any) {
	panic(fmt.Sprintf("%s: value %v is out of range", name, v))
}

func iecConvSS(v, lo, hi int64, name string) int64 {
	if v < lo || v > hi {
		iecConvFail(name, v)
	}
	return v
}

func iecConvSU(v int64, hi uint64, name string) uint64 {
	if v < 0 || uint64(v) > hi {
		iecConvFail(name, v)
	}
	return uint64(v)
}

func iecConvUS(v uint64, lo, hi int64, name string) int64 {
	if v > uint64(hi) {
		iecConvFail(name, v)
	}
	return int64(v)
}

func iecConvUU(v, hi uint64, name string) uint64 {
	if v > hi {
		iecConvFail(name, v)
	}
	return v
}

func iecConvFS(f float64, lo, hi int64, name string) int64 {
	r := gomath.RoundToEven(f)
	if gomath.IsNaN(r) || r < float64(lo) || r >= -float64(gomath.MinInt64) || int64(r) > hi {
		iecConvFail(name, f)
	}
	return int64(r)
}

func iecConvFU(f float64, hi uint64, name string) uint64 {
	r := gomath.RoundToEven(f)
	if gomath.IsNaN(r) || r < 0 || r >= 18446744073709551616.0 || uint64(r) > hi {
		iecConvFail(name, f)
	}
	return uint64(r)
}
`
