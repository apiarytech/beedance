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

import (
	"beedance/object"
	"math/bits"
)

func init() {
	object.RegisterBuiltin(BuiltinShl, "SHL", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for SHL. got=%d, want=2", len(args))
		}
		in, ok := args[0].(*object.BitString)
		if !ok {
			return object.NewBuiltinError("argument 1 to `SHL` must be a bitstring type, got %s", args[0].Type())
		}
		nVal, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `SHL` must be INTEGER, got %s", args[1].Type())
		}
		if nVal < 0 {
			return object.NewBuiltinError("shift amount for `SHL` must be non-negative, got %d", nVal)
		}
		shiftAmount := uint(nVal)
		result := in.Value << shiftAmount
		if in.Width < 64 {
			result &= (1 << in.Width) - 1
		}
		return &object.BitString{Value: result, Width: in.Width}
	})

	object.RegisterBuiltin(BuiltinShr, "SHR", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for SHR. got=%d, want=2", len(args))
		}
		in, ok := args[0].(*object.BitString)
		if !ok {
			return object.NewBuiltinError("argument 1 to `SHR` must be a bitstring type, got %s", args[0].Type())
		}
		nVal, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `SHR` must be INTEGER, got %s", args[1].Type())
		}
		if nVal < 0 {
			return object.NewBuiltinError("shift amount for `SHR` must be non-negative, got %d", nVal)
		}
		shiftAmount := uint(nVal)
		result := in.Value >> shiftAmount
		if in.Width < 64 {
			result &= (1 << in.Width) - 1
		}
		return &object.BitString{Value: result, Width: in.Width}
	})

	object.RegisterBuiltin(BuiltinRol, "ROL", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for ROL. got=%d, want=2", len(args))
		}
		in, ok := args[0].(*object.BitString)
		if !ok {
			return object.NewBuiltinError("argument 1 to `ROL` must be a bitstring type, got %s", args[0].Type())
		}
		nVal, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `ROL` must be INTEGER, got %s", args[1].Type())
		}
		if nVal < 0 {
			return object.NewBuiltinError("rotate amount for `ROL` must be non-negative, got %d", nVal)
		}
		k := int(nVal)
		var result uint64
		switch in.Width {
		case 8:
			result = uint64(bits.RotateLeft8(uint8(in.Value), k))
		case 16:
			result = uint64(bits.RotateLeft16(uint16(in.Value), k))
		case 32:
			result = uint64(bits.RotateLeft32(uint32(in.Value), k))
		case 64:
			result = bits.RotateLeft64(in.Value, k)
		}
		return &object.BitString{Value: result, Width: in.Width}
	})

	object.RegisterBuiltin(BuiltinRor, "ROR", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for ROR. got=%d, want=2", len(args))
		}
		in, ok := args[0].(*object.BitString)
		if !ok {
			return object.NewBuiltinError("argument 1 to `ROR` must be a bitstring type, got %s", args[0].Type())
		}
		nVal, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `ROR` must be INTEGER, got %s", args[1].Type())
		}
		if nVal < 0 {
			return object.NewBuiltinError("rotate amount for `ROR` must be non-negative, got %d", nVal)
		}
		k := int(nVal)
		var result uint64
		switch in.Width {
		case 8:
			result = uint64(bits.RotateLeft8(uint8(in.Value), -k))
		case 16:
			result = uint64(bits.RotateLeft16(uint16(in.Value), -k))
		case 32:
			result = uint64(bits.RotateLeft32(uint32(in.Value), -k))
		case 64:
			result = bits.RotateLeft64(in.Value, -k)
		}
		return &object.BitString{Value: result, Width: in.Width}
	})

	object.RegisterBuiltin(BuiltinAnd, "AND", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("AND", args...)
	})
	object.RegisterBuiltin(BuiltinOr, "OR", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("OR", args...)
	})
	object.RegisterBuiltin(BuiltinXor, "XOR", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("XOR", args...)
	})
	object.RegisterBuiltin(BuiltinNand, "NAND", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("NAND", args...)
	})
	object.RegisterBuiltin(BuiltinNor, "NOR", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("NOR", args...)
	})
}

func bitwiseBuiltin(op string, args ...object.Object) object.Object {
	if len(args) < 2 {
		return object.NewBuiltinError("wrong number of arguments for %s. got=%d, want>=2", op, len(args))
	}
	firstArg, ok := args[0].(*object.BitString)
	if !ok {
		return object.NewBuiltinError("all arguments to `%s` must be bit-string types, got %s", op, args[0].Type())
	}
	width := firstArg.Width
	for i := 1; i < len(args); i++ {
		arg, ok := args[i].(*object.BitString)
		if !ok {
			return object.NewBuiltinError("all arguments to `%s` must be bit-string types, got %s", op, args[i].Type())
		}
		if arg.Width != width {
			return object.NewBuiltinError("all arguments to `%s` must have the same width, got %d and %d", op, width, arg.Width)
		}
	}
	result := firstArg.Value
	for i := 1; i < len(args); i++ {
		nextVal := args[i].(*object.BitString).Value
		switch op {
		case "AND", "NAND":
			result &= nextVal
		case "OR", "NOR":
			result |= nextVal
		case "XOR":
			result ^= nextVal
		default:
			return object.NewBuiltinError("internal error: unknown bitwise operator %s", op)
		}
	}
	if op == "NAND" || op == "NOR" {
		var mask uint64
		if width < 64 {
			mask = (1 << width) - 1
		} else {
			mask = 0xFFFFFFFFFFFFFFFF
		}
		result = ^result & mask
	}
	return &object.BitString{Value: result, Width: width}
}
