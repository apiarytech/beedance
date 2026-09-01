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
	"fmt"
	"math"
	"math/bits"
	"strings"
	"time"
)

var (
	// NULL is a singleton object representing the null value.
	NULL = &object.Null{}
	// TRUE is a singleton object representing the boolean true value.
	TRUE = &object.Boolean{Value: true}
	// FALSE is a singleton object representing the boolean false value.
	FALSE = &object.Boolean{Value: false}
)

func init() {
	// Array Built-ins
	object.RegisterBuiltin("LOWER_BOUND", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for LOWER_BOUND. got=%d, want=2", len(args))
		}
		if _, ok := args[0].(*object.Array); !ok {
			return object.NewBuiltinError("argument 1 to `LOWER_BOUND` must be of type ARRAY, got %s", args[0].Type())
		}
		dim, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `LOWER_BOUND` must be of type INT, got %s", args[1].Type())
		}
		if dim != 1 {
			return object.NewBuiltinError("invalid dimension %d for 1D array", dim)
		}
		return &object.LInt{Value: 0}
	})

	object.RegisterBuiltin("UPPER_BOUND", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for UPPER_BOUND. got=%d, want=2", len(args))
		}
		arr, ok := args[0].(*object.Array)
		if !ok {
			return object.NewBuiltinError("argument 1 to `UPPER_BOUND` must be of type ARRAY, got %s", args[0].Type())
		}
		dim, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `UPPER_BOUND` must be of type INT, got %s", args[1].Type())
		}
		if dim != 1 {
			return object.NewBuiltinError("invalid dimension %d for 1D array", dim)
		}
		upper := int64(len(arr.Elements) - 1)
		return &object.LInt{Value: upper}
	})

	// Standard Built-ins
	object.RegisterBuiltin("PUTS", func(args ...object.Object) object.Object {
		for _, arg := range args {
			fmt.Println(arg.Inspect())
		}
		return NULL
	})

	object.RegisterBuiltin("FIRST", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
		}
		if args[0].Type() != object.ARRAY_OBJ {
			return object.NewBuiltinError("argument to `FIRST` must be ARRAY, got %s", args[0].Type())
		}
		arr := args[0].(*object.Array)
		if len(arr.Elements) > 0 {
			return arr.Elements[0]
		}
		return NULL
	})

	object.RegisterBuiltin("LAST", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
		}
		if args[0].Type() != object.ARRAY_OBJ {
			return object.NewBuiltinError("argument to `LAST` must be ARRAY, got %s", args[0].Type())
		}
		arr := args[0].(*object.Array)
		length := len(arr.Elements)
		if length > 0 {
			return arr.Elements[length-1]
		}
		return NULL
	})

	object.RegisterBuiltin("REST", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
		}
		if args[0].Type() != object.ARRAY_OBJ {
			return object.NewBuiltinError("argument to `REST` must be ARRAY, got %s", args[0].Type())
		}
		arr := args[0].(*object.Array)
		length := len(arr.Elements)
		if length > 0 {
			newElements := make([]object.Object, length-1)
			copy(newElements, arr.Elements[1:length])
			return &object.Array{Elements: newElements}
		}
		return NULL
	})

	object.RegisterBuiltin("PUSH", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments. got=%d, want=2", len(args))
		}
		if args[0].Type() != object.ARRAY_OBJ {
			return object.NewBuiltinError("argument to `PUSH` must be ARRAY, got %s", args[0].Type())
		}
		arr := args[0].(*object.Array)
		length := len(arr.Elements)
		newElements := make([]object.Object, length+1)
		copy(newElements, arr.Elements)
		newElements[length] = args[1]
		return &object.Array{Elements: newElements}
	})

	object.RegisterBuiltin("INSERT", func(args ...object.Object) object.Object {
		if len(args) != 3 {
			return object.NewBuiltinError("wrong number of arguments for INSERT. got=%d, want=3", len(args))
		}
		p, _, ok := object.GetIntegerObjectValue(args[2])
		if !ok {
			return object.NewBuiltinError("argument 3 to `INSERT` must be INTEGER, got %s", args[2].Type())
		}
		switch in1 := args[0].(type) {
		case *object.Array:
			elem := args[1]
			length := int64(len(in1.Elements))
			if p < 1 {
				p = 1
			}
			if p > length+1 {
				p = length + 1
			}
			idx := p - 1
			newElements := make([]object.Object, length+1)
			copy(newElements, in1.Elements[:idx])
			newElements[idx] = elem
			copy(newElements[idx+1:], in1.Elements[idx:])
			return &object.Array{Elements: newElements}
		case *object.String:
			in2, ok := args[1].(*object.String)
			if !ok {
				return object.NewBuiltinError("argument 2 to `INSERT` for strings must be STRING, got %s", args[1].Type())
			}
			strLen := int64(len(in1.Value))
			if p < 1 {
				p = 1
			}
			if p > strLen+1 {
				p = strLen + 1
			}
			idx := p - 1
			return &object.String{Value: in1.Value[:idx] + in2.Value + in1.Value[idx:]}
		case *object.WString:
			in2, ok := args[1].(*object.WString)
			if !ok {
				return object.NewBuiltinError("argument 2 to `INSERT` for wstrings must be WSTRING, got %s", args[1].Type())
			}
			strLen := int64(len(in1.Value))
			if p < 1 {
				p = 1
			}
			if p > strLen+1 {
				p = strLen + 1
			}
			idx := p - 1
			return &object.WString{Value: in1.Value[:idx] + in2.Value + in1.Value[idx:]}
		default:
			return object.NewBuiltinError("argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("DELETE", func(args ...object.Object) object.Object {
		if len(args) != 3 {
			return object.NewBuiltinError("wrong number of arguments for DELETE. got=%d, want=3", len(args))
		}
		// Standard signature is DELETE(IN, P, L).
		p, _, ok := object.GetIntegerObjectValue(args[1]) // P is the position
		if !ok {
			return object.NewBuiltinError("argument 2 to `DELETE` must be INTEGER, got %s", args[1].Type())
		}
		l, _, ok := object.GetIntegerObjectValue(args[2]) // L is the length
		if !ok {
			return object.NewBuiltinError("argument 3 to `DELETE` must be INTEGER, got %s", args[2].Type())
		}
		switch in1 := args[0].(type) {
		case *object.Array:
			arrLen := int64(len(in1.Elements))
			if l <= 0 || p < 1 || p > arrLen { // If L is non-positive or P is out of bounds, do nothing.
				return in1
			}
			start := p - 1
			end := start + l
			if end > arrLen { // If deletion goes past the end, truncate to the end.
				end = arrLen
			}
			newElements := append(in1.Elements[:start], in1.Elements[end:]...)
			return &object.Array{Elements: newElements}
		case *object.String:
			strLen := int64(len(in1.Value))    // cspell:disable-line
			if l <= 0 || p < 1 || p > strLen { // If L is non-positive or P is out of bounds, do nothing.
				return in1
			}
			start := p - 1
			end := start + l
			if end > strLen { // If deletion goes past the end, truncate to the end.
				end = strLen
			}
			return &object.String{Value: in1.Value[:start] + in1.Value[end:]}
		case *object.WString:
			strLen := int64(len(in1.Value))    // cspell:disable-line
			if l <= 0 || p < 1 || p > strLen { // If L is non-positive or P is out of bounds, do nothing.
				return in1
			}
			start := p - 1
			end := start + l
			if end > strLen { // If deletion goes past the end, truncate to the end.
				end = strLen
			}
			return &object.WString{Value: in1.Value[:start] + in1.Value[end:]}
		default:
			return object.NewBuiltinError("argument 1 to `DELETE` must be ARRAY, STRING, or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("CONCAT", func(args ...object.Object) object.Object {
		if len(args) < 2 {
			return object.NewBuiltinError("wrong number of arguments for CONCAT. got=%d, want>=2", len(args))
		}
		switch args[0].Type() {
		case object.ARRAY_OBJ:
			totalSize := 0
			for _, arg := range args {
				if arr, ok := arg.(*object.Array); ok {
					totalSize += len(arr.Elements)
				} else {
					return object.NewBuiltinError("all arguments to `CONCAT` must be of the same type (ARRAY), got %s", arg.Type())
				}
			}
			newElements := make([]object.Object, 0, totalSize)
			for _, arg := range args {
				newElements = append(newElements, arg.(*object.Array).Elements...)
			}
			return &object.Array{Elements: newElements}
		case object.STRING_OBJ:
			var sb strings.Builder
			for _, arg := range args {
				if str, ok := arg.(*object.String); ok {
					sb.WriteString(str.Value)
				} else {
					return object.NewBuiltinError("all arguments to `CONCAT` must be of the same type (STRING), got %s", arg.Type())
				}
			}
			return &object.String{Value: sb.String()}
		case object.WSTRING_OBJ:
			var sb strings.Builder
			for _, arg := range args {
				if str, ok := arg.(*object.WString); ok {
					sb.WriteString(str.Value)
				} else {
					return object.NewBuiltinError("all arguments to `CONCAT` must be of the same type (WSTRING), got %s", arg.Type())
				}
			}
			return &object.WString{Value: sb.String()}
		default:
			return object.NewBuiltinError("arguments to `CONCAT` must be ARRAY, STRING, or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("LEFT", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for LEFT. got=%d, want=2", len(args))
		}
		length, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `LEFT` must be INTEGER, got %s", args[1].Type())
		}
		switch str := args[0].(type) {
		case *object.String:
			l := length
			if l <= 0 {
				return &object.String{Value: ""}
			}
			if l >= int64(len(str.Value)) {
				return str
			}
			return &object.String{Value: str.Value[:l]}
		case *object.WString:
			l := length
			if l <= 0 {
				return &object.WString{Value: ""}
			}
			if l >= int64(len(str.Value)) {
				return str
			}
			return &object.WString{Value: str.Value[:l]}
		default:
			return object.NewBuiltinError("argument 1 to `LEFT` must be STRING or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("RIGHT", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for RIGHT. got=%d, want=2", len(args))
		}
		length, _, ok := object.GetIntegerObjectValue(args[1])
		if !ok {
			return object.NewBuiltinError("argument 2 to `RIGHT` must be INTEGER, got %s", args[1].Type())
		}
		l := length
		switch str := args[0].(type) {
		case *object.String:
			sLen := int64(len(str.Value))
			if l <= 0 {
				return &object.String{Value: ""}
			}
			if l >= sLen {
				return str
			}
			return &object.String{Value: str.Value[sLen-l:]}
		case *object.WString:
			sLen := int64(len(str.Value))
			if l <= 0 {
				return &object.WString{Value: ""}
			}
			if l >= sLen {
				return str
			}
			return &object.WString{Value: str.Value[sLen-l:]}
		default:
			return object.NewBuiltinError("argument 1 to `RIGHT` must be STRING or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("MID", func(args ...object.Object) object.Object {
		if len(args) != 3 {
			return object.NewBuiltinError("wrong number of arguments for MID. got=%d, want=3", len(args))
		}
		l, _, okL := object.GetIntegerObjectValue(args[2]) // L is the 3rd argument
		if !okL {
			return object.NewBuiltinError("argument 3 to `MID` must be INTEGER, got %s", args[2].Type())
		}
		p, _, okP := object.GetIntegerObjectValue(args[1]) // P is the 2nd argument
		if !okP {
			return object.NewBuiltinError("argument 2 to `MID` must be INTEGER, got %s", args[1].Type())
		}
		switch str := args[0].(type) {
		case *object.String:
			sLen := int64(len(str.Value))
			if l <= 0 || p <= 0 || p > sLen {
				return &object.String{Value: ""}
			}
			start := p - 1
			end := start + l
			if end > sLen {
				end = sLen
			}
			return &object.String{Value: str.Value[start:end]}
		case *object.WString:
			sLen := int64(len(str.Value))
			if l <= 0 || p <= 0 || p > sLen {
				return &object.WString{Value: ""}
			}
			start := p - 1
			end := start + l
			if end > sLen {
				end = sLen
			}
			return &object.WString{Value: str.Value[start:end]}
		default:
			return object.NewBuiltinError("argument 1 to `MID` must be STRING or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("FIND", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for FIND. got=%d, want=2", len(args))
		}
		switch in1 := args[0].(type) {
		case *object.String:
			in2, ok := args[1].(*object.String)
			if !ok {
				return object.NewBuiltinError("argument 2 to `FIND` for strings must be STRING, got %s", args[1].Type())
			}
			index := strings.Index(in1.Value, in2.Value)
			return &object.LInt{Value: int64(index + 1)}
		case *object.WString:
			in2, ok := args[1].(*object.WString)
			if !ok {
				return object.NewBuiltinError("argument 2 to `FIND` for wstrings must be WSTRING, got %s", args[1].Type())
			}
			index := strings.Index(in1.Value, in2.Value)
			return &object.LInt{Value: int64(index + 1)}
		case *object.Array:
			toFind := args[1]
			for i, elem := range in1.Elements {
				if object.IsEqual(elem, toFind) {
					return &object.LInt{Value: int64(i + 1)}
				}
			}
			return &object.LInt{Value: 0}
		default:
			return object.NewBuiltinError("argument 1 to `FIND` must be STRING, WSTRING, or ARRAY, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("REPLACE", func(args ...object.Object) object.Object {
		if len(args) != 4 {
			return object.NewBuiltinError("wrong number of arguments for REPLACE. got=%d, want=4", len(args))
		}
		l, _, okL := object.GetIntegerObjectValue(args[3]) // L is the 4th argument
		if !okL {
			return object.NewBuiltinError("argument 4 to `REPLACE` must be INTEGER, got %s", args[3].Type())
		}
		p, _, okP := object.GetIntegerObjectValue(args[2]) // P is the 3rd argument
		if !okP {
			return object.NewBuiltinError("argument 3 to `REPLACE` must be INTEGER, got %s", args[2].Type())
		}
		if l < 0 {
			l = 0
		}
		if p < 1 {
			p = 1
		}
		switch in1 := args[0].(type) {
		case *object.String:
			in2, ok := args[1].(*object.String)
			if !ok {
				return object.NewBuiltinError("argument 2 to `REPLACE` must be STRING, got %s", args[1].Type())
			}
			str1 := in1.Value
			str2 := in2.Value
			sLen := int64(len(str1))
			start := p - 1
			if start >= sLen {
				return &object.String{Value: str1 + str2}
			}
			endDelete := start + l
			if endDelete > sLen {
				endDelete = sLen
			}
			return &object.String{Value: str1[:start] + str2 + str1[endDelete:]}
		case *object.WString:
			in2, ok := args[1].(*object.WString)
			if !ok {
				return object.NewBuiltinError("argument 2 to `REPLACE` must be WSTRING, got %s", args[1].Type())
			}
			str1 := in1.Value
			str2 := in2.Value
			sLen := int64(len(str1))
			start := p - 1
			if start >= sLen {
				return &object.WString{Value: str1 + str2}
			}
			endDelete := start + l
			if endDelete > sLen {
				endDelete = sLen
			}
			return &object.WString{Value: str1[:start] + str2 + str1[endDelete:]}
		default:
			return object.NewBuiltinError("argument 1 to `REPLACE` must be STRING or WSTRING, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("SHL", func(args ...object.Object) object.Object {
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

	object.RegisterBuiltin("SHR", func(args ...object.Object) object.Object {
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

	object.RegisterBuiltin("ROL", func(args ...object.Object) object.Object {
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

	object.RegisterBuiltin("ROR", func(args ...object.Object) object.Object {
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

	object.RegisterBuiltin("SIN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for SIN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `SIN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Sin(val)}
	})

	object.RegisterBuiltin("COS", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for COS. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `COS` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Cos(val)}
	})

	object.RegisterBuiltin("TAN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for TAN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `TAN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Tan(val)}
	})

	object.RegisterBuiltin("ASIN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ASIN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `ASIN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val < -1.0 || val > 1.0 {
			return object.NewBuiltinError("argument to `ASIN` must be between -1 and 1, got %f", val)
		}
		return &object.Real{Value: math.Asin(val)}
	})

	object.RegisterBuiltin("ACOS", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ACOS. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `ACOS` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val < -1.0 || val > 1.0 {
			return object.NewBuiltinError("argument to `ACOS` must be between -1 and 1, got %f", val)
		}
		return &object.Real{Value: math.Acos(val)}
	})

	object.RegisterBuiltin("ATAN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ATAN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `ATAN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Atan(val)}
	})

	object.RegisterBuiltin("ATAN2", func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for ATAN2. got=%d, want=2", len(args))
		}
		y, okY := object.GetFloat64Value(args[0])
		if !okY {
			return object.NewBuiltinError("argument 1 to `ATAN2` must be INTEGER or REAL, got %s", args[0].Type())
		}
		x, okX := object.GetFloat64Value(args[1])
		if !okX {
			return object.NewBuiltinError("argument 2 to `ATAN2` must be INTEGER or REAL, got %s", args[1].Type())
		}
		return &object.Real{Value: math.Atan2(y, x)}
	})

	object.RegisterBuiltin("LN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for LN. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `LN` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val <= 0 {
			return object.NewBuiltinError("argument to `LN` must be positive, got %f", val)
		}
		return &object.Real{Value: math.Log(val)}
	})

	object.RegisterBuiltin("LOG", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for LOG. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `LOG` must be INTEGER or REAL, got %s", args[0].Type())
		}
		if val <= 0 {
			return object.NewBuiltinError("argument to `LOG` must be positive, got %f", val)
		}
		return &object.Real{Value: math.Log10(val)}
	})

	object.RegisterBuiltin("EXP", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for EXP. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `EXP` must be INTEGER or REAL, got %s", args[0].Type())
		}
		return &object.Real{Value: math.Exp(val)}
	})

	object.RegisterBuiltin("SQRT", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for SQRT. got=%d, want=1", len(args))
		}
		val, ok := object.GetFloat64Value(args[0])
		if !ok {
			return object.NewBuiltinError("argument to `SQRT` not supported, got %s", args[0].Type())
		}
		if val < 0 {
			return object.NewBuiltinError("argument to `SQRT` must be non-negative, got %f", val)
		}
		return &object.Real{Value: math.Sqrt(val)}
	})

	object.RegisterBuiltin("ROUND", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ROUND. got=%d, want=1", len(args))
		}
		var val float64
		var ok bool
		switch arg := args[0].(type) {
		case *object.Real:
			val, ok = arg.Value, true
		case *object.LReal:
			val, ok = arg.Value, true
		default:
			ok = false
		}
		if !ok {
			return object.NewBuiltinError("argument to `ROUND` must be REAL, got %s", args[0].Type())
		}
		return &object.LInt{Value: int64(math.Round(val))}
	})

	object.RegisterBuiltin("ABS", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for ABS. got=%d, want=1", len(args))
		}
		arg := args[0]
		switch v := arg.(type) {
		case *object.SInt:
			if v.Value < 0 {
				return &object.SInt{Value: -v.Value}
			}
			return v
		case *object.Int:
			if v.Value < 0 {
				return &object.Int{Value: -v.Value}
			}
			return v
		case *object.DInt:
			if v.Value < 0 {
				return &object.DInt{Value: -v.Value}
			}
			return v
		case *object.LInt:
			if v.Value < 0 {
				return &object.LInt{Value: -v.Value}
			}
			return v
		case *object.USInt, *object.UInt, *object.UDInt, *object.ULInt:
			return v
		case *object.Real:
			return &object.Real{Value: math.Abs(v.Value)}
		case *object.LReal:
			return &object.LReal{Value: math.Abs(v.Value)}
		default:
			return object.NewBuiltinError("argument to `ABS` not supported, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("TRUNC", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for TRUNC. got=%d, want=1", len(args))
		}
		var val float64
		var ok bool
		switch arg := args[0].(type) {
		case *object.Real:
			val, ok = arg.Value, true
		case *object.LReal:
			val, ok = arg.Value, true
		default:
			ok = false
		}
		if !ok {
			return object.NewBuiltinError("argument to `TRUNC` must be REAL, got %s", args[0].Type())
		}
		return &object.LInt{Value: int64(math.Trunc(val))}
	})

	object.RegisterBuiltin("ADD", addBuiltin)
	object.RegisterBuiltin("SUB", subBuiltin)
	object.RegisterBuiltin("MUL", mulBuiltin)
	object.RegisterBuiltin("DIV", divBuiltin)
	object.RegisterBuiltin("MOD", modBuiltin)
	object.RegisterBuiltin("EXPT", exptBuiltin)
	object.RegisterBuiltin("GT", comparisonBuiltin("GT"))
	object.RegisterBuiltin("GE", comparisonBuiltin("GE"))
	object.RegisterBuiltin("EQ", comparisonBuiltin("EQ"))
	object.RegisterBuiltin("LE", comparisonBuiltin("LE"))
	object.RegisterBuiltin("LT", comparisonBuiltin("LT"))
	object.RegisterBuiltin("NE", comparisonBuiltin("NE"))

	object.RegisterBuiltin("LEN", func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewBuiltinError("wrong number of arguments for LEN. got=%d, want=1", len(args))
		}
		switch arg := args[0].(type) {
		case *object.Array:
			return &object.LInt{Value: int64(len(arg.Elements))}
		case *object.String:
			return &object.LInt{Value: int64(len(arg.Value))}
		case *object.WString:
			return &object.LInt{Value: int64(len(arg.Value))}
		default:
			return object.NewBuiltinError("argument to `LEN` not supported, got %s", args[0].Type())
		}
	})

	object.RegisterBuiltin("MUX", muxBuiltin)
	object.RegisterBuiltin("LIMIT", limitWrapperBuiltin)
	object.RegisterBuiltin("SEL", selWrapperBuiltin)
	object.RegisterBuiltin("MOVE", moveBuiltin)

	object.RegisterBuiltin("MIN", func(args ...object.Object) object.Object {
		return minMaxBuiltin("MIN", args...)
	})
	object.RegisterBuiltin("MAX", func(args ...object.Object) object.Object {
		return minMaxBuiltin("MAX", args...)
	})

	object.RegisterBuiltin("AND", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("AND", args...)
	})
	object.RegisterBuiltin("OR", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("OR", args...)
	})
	object.RegisterBuiltin("XOR", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("XOR", args...)
	})
	object.RegisterBuiltin("NAND", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("NAND", args...)
	})
	object.RegisterBuiltin("NOR", func(args ...object.Object) object.Object {
		return bitwiseBuiltin("NOR", args...)
	})

	// Generic Type Conversions
	types := []string{"BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT", "REAL", "LREAL", "TIME", "DATE", "TOD", "DT", "STRING", "WSTRING", "BYTE", "WORD", "DWORD", "LWORD", "ANY_INT", "ANY_REAL", "BCD"}
	for _, from := range types {
		for _, to := range types {
			if from == to {
				continue
			}
			funcName := fmt.Sprintf("%s_TO_%s", from, to)
			object.RegisterBuiltin(funcName, object.GenericConversionBuiltin(from, to).Fn)
		}
	}
}

// --- Arithmetic Implementation (self-contained, no dependency on evaluator) ---

func addBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for ADD. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]
	if object.IsNumeric(arg1) && object.IsNumeric(arg2) {
		return object.EvalNumericInfix(arg1, arg2, "+")
	}
	switch a1 := arg1.(type) {
	case *object.Time:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.Time{Value: a1.Value + a2.Value}
		}
	case *object.TimeOfDay:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.TimeOfDay{Value: a1.Value.Add(a2.Value)}
		}
	case *object.DateAndTime:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.DateAndTime{Value: a1.Value.Add(a2.Value)}
		}
	}
	return object.NewBuiltinError("unsupported argument types for ADD: %s + %s", arg1.Type(), arg2.Type())
}

func subBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for SUB. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]
	if object.IsNumeric(arg1) && object.IsNumeric(arg2) {
		return object.EvalNumericInfix(arg1, arg2, "-")
	}
	switch a1 := arg1.(type) {
	case *object.Time:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.Time{Value: a1.Value - a2.Value}
		}
	case *object.Date:
		if a2, ok := arg2.(*object.Date); ok {
			return &object.Time{Value: a1.Value.Sub(a2.Value)}
		}
	case *object.TimeOfDay:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.TimeOfDay{Value: a1.Value.Add(-a2.Value)}
		}
		if a2, ok := arg2.(*object.TimeOfDay); ok {
			return &object.Time{Value: a1.Value.Sub(a2.Value)}
		}
	case *object.DateAndTime:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.DateAndTime{Value: a1.Value.Add(-a2.Value)}
		}
		if a2, ok := arg2.(*object.DateAndTime); ok {
			return &object.Time{Value: a1.Value.Sub(a2.Value)}
		}
	}
	return object.NewBuiltinError("unsupported argument types for SUB: %s - %s", arg1.Type(), arg2.Type())
}

func mulBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for MUL. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]
	if t, ok := arg1.(*object.Time); ok {
		if num, ok := object.GetFloat64Value(arg2); ok {
			return &object.Time{Value: time.Duration(float64(t.Value) * num)}
		}
	}
	if t, ok := arg2.(*object.Time); ok {
		if num, ok := object.GetFloat64Value(arg1); ok {
			return &object.Time{Value: time.Duration(float64(t.Value) * num)}
		}
	}
	if object.IsNumeric(arg1) && object.IsNumeric(arg2) {
		return object.EvalNumericInfix(arg1, arg2, "*")
	}
	return object.NewBuiltinError("unsupported argument types for MUL: %s * %s", arg1.Type(), arg2.Type())
}

func divBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for DIV. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]
	if t, ok := arg1.(*object.Time); ok {
		if num, ok := object.GetFloat64Value(arg2); ok {
			if num == 0 {
				return object.NewBuiltinError("division by zero")
			}
			return &object.Time{Value: time.Duration(float64(t.Value) / num)}
		}
	}
	if object.IsNumeric(arg1) && object.IsNumeric(arg2) {
		val2, num := object.GetFloat64Value(arg2)
		if num && val2 == 0.0 {
			return object.NewBuiltinError("division by zero")
		}
		return object.EvalNumericInfix(arg1, arg2, "/")
	}
	return object.NewBuiltinError("unsupported argument types for DIV: %s / %s", arg1.Type(), arg2.Type())
}

func modBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for MOD. got=%d, want=2", len(args))
	}
	_, _, ok1 := object.GetIntegerObjectValue(args[0])
	_, _, ok2 := object.GetIntegerObjectValue(args[1])
	if !ok1 || !ok2 {
		return object.NewBuiltinError("arguments to `MOD` must be INTEGER, got %s and %s", args[0].Type(), args[1].Type())
	}

	return object.EvalIntegerInfix(args[0], "MOD", args[1])
}

func exptBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for EXPT. got=%d, want=2", len(args))
	}
	base, ok1 := object.GetFloat64Value(args[0])
	if !ok1 {
		return object.NewBuiltinError("argument 1 to `EXPT` must be numeric, got %s", args[0].Type())
	}
	exponent, ok2 := object.GetFloat64Value(args[1])
	if !ok2 {
		return object.NewBuiltinError("argument 2 to `EXPT` must be numeric, got %s", args[1].Type())
	}
	result := math.Pow(base, exponent)
	return &object.Real{Value: result}
}

func comparisonBuiltin(op string) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewBuiltinError("wrong number of arguments for %s. got=%d, want=2", op, len(args))
		}
		return evalComparison(op, args[0], args[1])
	}
}

func evalComparison(op string, left, right object.Object) object.Object {
	opMap := map[string]string{"GT": ">", "GE": ">=", "EQ": "=", "LE": "<=", "LT": "<", "NE": "<>"}
	symbolicOp, ok := opMap[op]
	if !ok {
		return object.NewBuiltinError("internal error: unknown comparison operator %s", op)
	}
	return evalInfix(left, symbolicOp, right)
}

func evalInfix(left object.Object, operator string, right object.Object) object.Object {
	switch {
	case object.IsNumeric(left) && object.IsNumeric(right):
		return object.EvalNumericInfix(left, right, operator)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		leftVal := left.(*object.String).Value
		rightVal := right.(*object.String).Value
		return evalGenericComparison(operator, leftVal, rightVal)
	case left.Type() == object.WSTRING_OBJ && right.Type() == object.WSTRING_OBJ:
		leftVal := left.(*object.WString).Value
		rightVal := right.(*object.WString).Value
		return evalGenericComparison(operator, leftVal, rightVal)
	case left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ:
		leftVal := left.(*object.Boolean).Value
		rightVal := right.(*object.Boolean).Value
		switch operator {
		case "=":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "<>":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return object.NewBuiltinError("unknown operator for BOOLEAN: %s", operator)
		}
	case left.Type() == object.TIME_OBJ && right.Type() == object.TIME_OBJ:
		leftVal := left.(*object.Time).Value
		rightVal := right.(*object.Time).Value
		return evalGenericComparison(operator, int64(leftVal), int64(rightVal))
	case left.Type() == object.DATE_OBJ && right.Type() == object.DATE_OBJ:
		leftVal := left.(*object.Date).Value
		rightVal := right.(*object.Date).Value
		return evalGenericComparison(operator, leftVal.UnixNano(), rightVal.UnixNano())
	case left.Type() == object.TIME_OF_DAY_OBJ && right.Type() == object.TIME_OF_DAY_OBJ:
		leftVal := left.(*object.TimeOfDay).Value
		rightVal := right.(*object.TimeOfDay).Value
		leftNs := int64(leftVal.Hour())*int64(time.Hour) + int64(leftVal.Minute())*int64(time.Minute) + int64(leftVal.Second())*int64(time.Second) + int64(leftVal.Nanosecond())
		rightNs := int64(rightVal.Hour())*int64(time.Hour) + int64(rightVal.Minute())*int64(time.Minute) + int64(rightVal.Second())*int64(time.Second) + int64(rightVal.Nanosecond())
		return evalGenericComparison(operator, leftNs, rightNs)
	case left.Type() == object.DATE_AND_TIME_OBJ && right.Type() == object.DATE_AND_TIME_OBJ:
		leftVal := left.(*object.DateAndTime).Value
		rightVal := right.(*object.DateAndTime).Value
		return evalGenericComparison(operator, leftVal.UnixNano(), rightVal.UnixNano())
	case left.Type() != right.Type():
		return object.NewBuiltinError("type mismatch for comparison: %s %s %s", left.Type(), operator, right.Type())
	default:
		return object.NewBuiltinError("unsupported types for infix operation: %s %s %s", left.Type(), operator, right.Type())
	}
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
		if op == "NAND" {
			result = ^result & mask
		} else {
			result = ^result & mask
		}
	}
	return &object.BitString{Value: result, Width: width}
}

func limitBuiltin(min, in, max object.Object) object.Object {
	args := []object.Object{min, in, max}
	hasReal := false
	for _, arg := range args {
		if !object.IsNumeric(arg) {
			return object.NewBuiltinError("all arguments to `LIMIT` must be INTEGER or REAL, got %s", arg.Type())
		}
		if arg.Type() == object.REAL_OBJ || arg.Type() == object.LREAL_OBJ {
			hasReal = true
		}
	}

	if hasReal {
		minVal, _ := object.GetFloat64Value(min)
		inVal, _ := object.GetFloat64Value(in)
		maxVal, _ := object.GetFloat64Value(max)
		if inVal < minVal {
			return &object.Real{Value: minVal}
		}
		if inVal > maxVal {
			return &object.Real{Value: maxVal}
		}
		return &object.Real{Value: inVal}
	}

	// All are integers
	minVal, _, _ := object.GetIntegerObjectValue(min)
	inVal, _, _ := object.GetIntegerObjectValue(in)
	maxVal, _, _ := object.GetIntegerObjectValue(max)
	if inVal < minVal {
		return &object.LInt{Value: minVal}
	}
	if inVal > maxVal {
		return &object.LInt{Value: maxVal}
	}
	return &object.LInt{Value: inVal}
}

func limitWrapperBuiltin(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("wrong number of arguments for LIMIT. got=%d, want=3", len(args))
	}
	mn := args[0]
	in := args[1]
	mx := args[2]
	return limitBuiltin(mn, in, mx)
}

func selWrapperBuiltin(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("wrong number of arguments for SEL. got=%d, want=3", len(args))
	}
	return selBuiltin(args[0], args[1], args[2])
}

func selBuiltin(g, in0, in1 object.Object) object.Object {
	gBool, ok := g.(*object.Boolean)
	if !ok {
		return object.NewBuiltinError("argument 1 to `SEL` must be BOOLEAN, got %s", g.Type())
	}
	if in0.Type() != in1.Type() {
		return object.NewBuiltinError("arguments 2 and 3 to `SEL` must be of the same type, got %s and %s", in0.Type(), in1.Type())
	}
	if gBool.Value {
		return in1
	}
	return in0
}

func minMaxBuiltin(op string, args ...object.Object) object.Object {
	if len(args) == 0 {
		return object.NewBuiltinError("wrong number of arguments for %s. got=0, want>=1", op)
	}

	result := args[0]
	firstType := result.Type()
	isFirstNumeric := object.IsNumeric(result)

	// Check if the type is orderable by the logic in EvalInfix.
	switch firstType {
	case object.SINT_OBJ, object.INT_OBJ, object.DINT_OBJ, object.LINT_OBJ,
		object.USINT_OBJ, object.UINT_OBJ, object.UDINT_OBJ, object.ULINT_OBJ,
		object.REAL_OBJ, object.LREAL_OBJ, object.STRING_OBJ, object.WSTRING_OBJ,
		object.TIME_OBJ, object.DATE_OBJ, object.TIME_OF_DAY_OBJ, object.DATE_AND_TIME_OBJ:
		// These types are orderable.
	default:
		return object.NewBuiltinError("arguments to `%s` must be of an orderable elementary type, got %s", op, firstType)
	}

	for i := 1; i < len(args); i++ {
		nextArg := args[i]
		isNextNumeric := object.IsNumeric(nextArg)

		if isFirstNumeric {
			if !isNextNumeric {
				return object.NewBuiltinError("all arguments to `%s` must be INTEGER or REAL, got %s", op, nextArg.Type())
			}
		} else if nextArg.Type() != firstType {
			return object.NewBuiltinError("all arguments to `%s` must be of the same type, got %s and %s", op, firstType, nextArg.Type())
		}

		var comparisonOp string
		if op == "MIN" {
			comparisonOp = "<" // If nextArg < result, we'll update result
		} else { // MAX
			comparisonOp = ">" // If nextArg > result, we'll update result
		}

		isCompareTrue := object.EvalInfix(nextArg, comparisonOp, result)

		if err, ok := isCompareTrue.(*object.Error); ok {
			return err
		}

		if boolResult, ok := isCompareTrue.(*object.Boolean); ok && boolResult.Value {
			result = nextArg
		}
	}

	return result
}

func moveBuiltin(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("wrong number of arguments for MOVE. got=%d, want=1", len(args))
	}
	return args[0]
}

func muxBuiltin(args ...object.Object) object.Object {
	if len(args) < 2 {
		return object.NewBuiltinError("wrong number of arguments for MUX. got=%d, want>=2", len(args))
	}

	kVal, _, ok := object.GetIntegerObjectValue(args[0])
	if !ok {
		return object.NewBuiltinError("argument 1 to `MUX` must be INTEGER, got %s", args[0].Type())
	}

	valueArgs := args[1:]
	numInputs := len(valueArgs)

	if kVal < 0 || kVal >= int64(numInputs) {
		return object.NewBuiltinError("index %d out of bounds for MUX with %d inputs", kVal, numInputs)
	}

	// All value arguments must be of the same type.
	if numInputs > 1 {
		firstType := valueArgs[0].Type()
		for i := 1; i < numInputs; i++ {
			// A stricter check would not allow mixing numeric types, but for now this is fine.
			if valueArgs[i].Type() != firstType && !(object.IsNumeric(valueArgs[0]) && object.IsNumeric(valueArgs[i])) {
				return object.NewBuiltinError("all value arguments to `MUX` must be of the same type, got %s but expected %s", valueArgs[i].Type(), firstType)
			}
		}
	}

	return valueArgs[kVal]
}

func isEqual(left, right object.Object) bool {
	result := evalInfix(left, "=", right)
	return result == TRUE
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalGenericComparison[T ~string | ~int64](op string, leftVal, rightVal T) object.Object {
	switch op {
	case "=":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "<>":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return object.NewBuiltinError("unknown operator '%s' for generic comparison", op)
	}
}
