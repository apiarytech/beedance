/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

// The functions of byte order (IEC 61131-3 3rd edition, Table 37):
// TO_BIG_ENDIAN, TO_LITTLE_ENDIAN, FROM_BIG_ENDIAN and FROM_LITTLE_ENDIAN.
// A value is held in little-endian order, the order of the machines the
// engines run on, so the big-endian functions reverse the bytes of an
// integer, a bit string or a real and the little-endian ones return it.

import (
	"math"
	"math/bits"
	"strings"

	"github.com/apiarytech/beedance/object"
)

func init() {
	object.RegisterBuiltin(BuiltinToBigEndian, "TO_BIG_ENDIAN", endianFn("TO_BIG_ENDIAN", true))
	object.RegisterBuiltin(BuiltinToLittleEndian, "TO_LITTLE_ENDIAN", endianFn("TO_LITTLE_ENDIAN", false))
	object.RegisterBuiltin(BuiltinFromBigEndian, "FROM_BIG_ENDIAN", endianFn("FROM_BIG_ENDIAN", true))
	object.RegisterBuiltin(BuiltinFromLittleEndian, "FROM_LITTLE_ENDIAN", endianFn("FROM_LITTLE_ENDIAN", false))
}

// endianFn is a function of byte order; swap reverses the bytes. A second
// argument, which the compiler adds, names the type of the first: the VM
// holds an INT as a LINT.
func endianFn(name string, swap bool) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 1 && len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for %s. got=%d, want=1", name, len(args))
		}
		in := args[0]
		typeName := string(in.Type())
		if b, isBits := in.(*object.BitString); isBits {
			typeName = map[int]string{8: "BYTE", 16: "WORD", 32: "DWORD", 64: "LWORD"}[b.Width]
		}
		if len(args) == 2 {
			s, ok := args[1].(*object.String)
			if !ok {
				return object.NewBuiltinError("%s: the type must be a string", name)
			}
			typeName = strings.ToUpper(s.Value)
		}
		switch typeName {
		case "REAL", "LREAL":
			f, ok := object.GetFloat64Value(in)
			if !ok {
				return object.NewBuiltinError("argument to `%s` must be a %s, got %s", name, typeName, in.Type())
			}
			if typeName == "REAL" {
				if swap {
					f = float64(math.Float32frombits(bits.ReverseBytes32(math.Float32bits(float32(f)))))
				}
				return &object.Real{Value: f}
			}
			if swap {
				f = math.Float64frombits(bits.ReverseBytes64(math.Float64bits(f)))
			}
			return &object.LReal{Value: f}
		}
		size, ok := endianSizes[typeName]
		if !ok {
			return object.NewBuiltinError("argument to `%s` must be an integer, a bit string or a real, got %s", name, typeName)
		}
		var u uint64
		if b, isBits := in.(*object.BitString); isBits {
			u = b.Value
		} else if v, _, isInt := object.GetIntegerObjectValue(in); isInt {
			u = uint64(v)
		} else {
			return object.NewBuiltinError("argument to `%s` must be a %s, got %s", name, typeName, in.Type())
		}
		u &= math.MaxUint64 >> (64 - 8*size)
		if swap {
			u = bits.ReverseBytes64(u) >> (64 - 8*size)
		}
		shift := 64 - 8*size
		signed := int64(u<<shift) >> shift
		switch typeName {
		case "SINT":
			return &object.SInt{Value: int8(signed)}
		case "INT":
			return &object.Int{Value: int16(signed)}
		case "DINT":
			return &object.DInt{Value: int32(signed)}
		case "LINT":
			return &object.LInt{Value: signed}
		case "USINT":
			return &object.USInt{Value: uint8(u)}
		case "UINT":
			return &object.UInt{Value: uint16(u)}
		case "UDINT":
			return &object.UDInt{Value: uint32(u)}
		case "ULINT":
			return &object.ULInt{Value: u}
		}
		return &object.BitString{Value: u, Width: 8 * size}
	}
}

// endianSizes is the size in bytes of each type whose bytes reverse.
var endianSizes = map[string]int{
	"SINT": 1, "USINT": 1, "BYTE": 1, "INT": 2, "UINT": 2, "WORD": 2,
	"DINT": 4, "UDINT": 4, "DWORD": 4, "LINT": 8, "ULINT": 8, "LWORD": 8,
}
