/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package evaluator

import (
	"beedance/object"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"time"
)

var builtins = map[string]*object.Builtin{
	"len": &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
		}

		switch arg := args[0].(type) {
		case *object.Array:
			return &object.Integer{Value: int64(len(arg.Elements))}
		case *object.String:
			return &object.Integer{Value: int64(len(arg.Value))}
		default:
			return newBuiltinError("argument to `len` not supported, got %s", args[0].Type())
		}
	},
	},
	"puts": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			for _, arg := range args {
				fmt.Println(arg.Inspect())
			}

			return NULL
		},
	},
	"first": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newBuiltinError("argument to `first` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			if len(arr.Elements) > 0 {
				return arr.Elements[0]
			}

			return NULL
		},
	},
	"last": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newBuiltinError("argument to `last` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				return arr.Elements[length-1]
			}

			return NULL
		},
	},
	"rest": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newBuiltinError("argument to `rest` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				newElements := make([]object.Object, length-1, length-1)
				copy(newElements, arr.Elements[1:length])
				return &object.Array{Elements: newElements}
			}

			return NULL
		},
	},
	"push": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newBuiltinError("argument to `push` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)

			newElements := make([]object.Object, length+1, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]

			return &object.Array{Elements: newElements}
		},
	},
	"INSERT": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newBuiltinError("wrong number of arguments for INSERT. got=%d, want=3", len(args))
			}

			pos, ok := args[2].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 3 to `INSERT` must be INTEGER, got %s", args[2].Type())
			}
			p := pos.Value

			switch in1 := args[0].(type) {
			case *object.Array:
				elem := args[1]
				length := int64(len(in1.Elements))

				// Clamp position to be within bounds [1, length+1]
				if p < 1 {
					p = 1
				}
				if p > length+1 {
					p = length + 1
				}

				idx := p - 1 // Convert to 0-based index

				newElements := make([]object.Object, length+1)
				copy(newElements, in1.Elements[:idx])
				newElements[idx] = elem
				copy(newElements[idx+1:], in1.Elements[idx:])

				return &object.Array{Elements: newElements}

			case *object.String:
				in2, ok := args[1].(*object.String)
				if !ok {
					return newBuiltinError("argument 2 to `INSERT` for strings must be STRING, got %s", args[1].Type())
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
			default:
				return newBuiltinError("argument 1 to `INSERT` must be ARRAY or STRING, got %s", args[0].Type())
			}
		},
	},
	"DELETE": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newBuiltinError("wrong number of arguments for DELETE. got=%d, want=3", len(args))
			}

			length, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `DELETE` must be INTEGER, got %s", args[1].Type())
			}

			pos, ok := args[2].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 3 to `DELETE` must be INTEGER, got %s", args[2].Type())
			}

			switch in1 := args[0].(type) {
			case *object.Array:
				l := length.Value
				p := pos.Value
				arrLen := int64(len(in1.Elements))

				if l <= 0 || p < 1 || p > arrLen {
					return in1 // Return original array if params are invalid/noop
				}

				start := p - 1
				end := start + l
				if end > arrLen {
					end = arrLen
				}

				newElements := append(in1.Elements[:start], in1.Elements[end:]...)
				return &object.Array{Elements: newElements}

			case *object.String:
				l := length.Value
				p := pos.Value
				strLen := int64(len(in1.Value))

				if l <= 0 || p < 1 || p > strLen {
					return in1 // Return original string if params are invalid/noop
				}

				start := p - 1
				end := start + l
				if end > strLen {
					end = strLen
				}

				return &object.String{Value: in1.Value[:start] + in1.Value[end:]}
			default:
				return newBuiltinError("argument 1 to `DELETE` must be ARRAY or STRING, got %s", args[0].Type())
			}
		},
	},
	"CONCAT": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 {
				// In case of no arguments, we can't determine the type.
				// Returning an error is the safest option.
				return newBuiltinError("wrong number of arguments for CONCAT. got=0, want>=1")
			}

			// Overload CONCAT for ARRAYs and STRINGs based on the first argument type.
			switch args[0].Type() {
			case object.ARRAY_OBJ:
				totalSize := 0
				for _, arg := range args {
					if arr, ok := arg.(*object.Array); ok {
						totalSize += len(arr.Elements)
					} else {
						return newBuiltinError("all arguments to `CONCAT` must be of the same type (ARRAY), got %s", arg.Type())
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
						return newBuiltinError("all arguments to `CONCAT` must be of the same type (STRING), got %s", arg.Type())
					}
				}
				return &object.String{Value: sb.String()}

			default:
				return newBuiltinError("arguments to `CONCAT` must be either all ARRAYs or all STRINGs, got %s", args[0].Type())
			}
		},
	},
	"LEFT": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for LEFT. got=%d, want=2", len(args))
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return newBuiltinError("argument 1 to `LEFT` must be STRING, got %s", args[0].Type())
			}
			length, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `LEFT` must be INTEGER, got %s", args[1].Type())
			}

			l := length.Value
			if l <= 0 {
				return &object.String{Value: ""}
			}
			if l >= int64(len(str.Value)) {
				return str
			}
			return &object.String{Value: str.Value[:l]}
		},
	},
	"RIGHT": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for RIGHT. got=%d, want=2", len(args))
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return newBuiltinError("argument 1 to `RIGHT` must be STRING, got %s", args[0].Type())
			}
			length, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `RIGHT` must be INTEGER, got %s", args[1].Type())
			}

			l := length.Value
			sLen := int64(len(str.Value))
			if l <= 0 {
				return &object.String{Value: ""}
			}
			if l >= sLen {
				return str
			}
			return &object.String{Value: str.Value[sLen-l:]}
		},
	},
	"MID": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newBuiltinError("wrong number of arguments for MID. got=%d, want=3", len(args))
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return newBuiltinError("argument 1 to `MID` must be STRING, got %s", args[0].Type())
			}
			length, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `MID` must be INTEGER, got %s", args[1].Type())
			}
			pos, ok := args[2].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 3 to `MID` must be INTEGER, got %s", args[2].Type())
			}

			l := length.Value
			p := pos.Value
			sLen := int64(len(str.Value))

			if l <= 0 || p <= 0 || p > sLen {
				return &object.String{Value: ""}
			}
			start := p - 1 // Convert from 1-based to 0-based index
			end := start + l
			if end > sLen {
				end = sLen
			}
			return &object.String{Value: str.Value[start:end]}
		},
	},
	"FIND": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for FIND. got=%d, want=2", len(args))
			}

			switch in1 := args[0].(type) {
			case *object.String:
				in2, ok := args[1].(*object.String)
				if !ok {
					return newBuiltinError("argument 2 to `FIND` for strings must be STRING, got %s", args[1].Type())
				}
				index := strings.Index(in1.Value, in2.Value)
				return &object.Integer{Value: int64(index + 1)}

			case *object.Array:
				toFind := args[1]
				for i, elem := range in1.Elements {
					// For simplicity, we use the equality rules of the language.
					// This means direct comparison for basic types.
					// A more complex implementation could use `evalInfixExpression` for `==`.
					if isEqual(elem, toFind) {
						return &object.Integer{Value: int64(i + 1)} // 1-based index
					}
				}
				return &object.Integer{Value: 0} // Not found

			default:
				return newBuiltinError("argument 1 to `FIND` must be STRING or ARRAY, got %s", args[0].Type())
			}
		},
	},
	"REPLACE": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 4 {
				return newBuiltinError("wrong number of arguments for REPLACE. got=%d, want=4", len(args))
			}
			in1, ok := args[0].(*object.String)
			if !ok {
				return newBuiltinError("argument 1 to `REPLACE` must be STRING, got %s", args[0].Type())
			}
			in2, ok := args[1].(*object.String)
			if !ok {
				return newBuiltinError("argument 2 to `REPLACE` must be STRING, got %s", args[1].Type())
			}
			length, ok := args[2].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 3 to `REPLACE` must be INTEGER, got %s", args[2].Type())
			}
			pos, ok := args[3].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 4 to `REPLACE` must be INTEGER, got %s", args[3].Type())
			}

			l := length.Value
			p := pos.Value
			str1 := in1.Value
			str2 := in2.Value
			sLen := int64(len(str1))

			if p < 1 {
				p = 1
			}
			if l < 0 {
				l = 0
			}

			start := p - 1 // Convert to 0-based index

			if start >= sLen {
				return &object.String{Value: str1 + str2}
			}

			endDelete := start + l
			if endDelete > sLen {
				endDelete = sLen
			}

			return &object.String{Value: str1[:start] + str2 + str1[endDelete:]}
		},
	},
	"SHL": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for SHL. got=%d, want=2", len(args))
			}
			in, ok := args[0].(*object.BitString)
			if !ok {
				return newBuiltinError("argument 1 to `SHL` must be a bitstring type, got %s", args[0].Type())
			}
			n, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `SHL` must be INTEGER, got %s", args[1].Type())
			}
			if n.Value < 0 {
				return newBuiltinError("shift amount for `SHL` must be non-negative, got %d", n.Value)
			}

			shiftAmount := uint(n.Value)
			result := in.Value << shiftAmount

			// Apply mask to ensure the result stays within the bitstring's width
			if in.Width < 64 {
				result &= (1 << in.Width) - 1
			}
			return &object.BitString{Value: result, Width: in.Width}
		},
	},
	"SHR": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for SHR. got=%d, want=2", len(args))
			}
			in, ok := args[0].(*object.BitString)
			if !ok {
				return newBuiltinError("argument 1 to `SHR` must be a bitstring type, got %s", args[0].Type())
			}
			n, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `SHR` must be INTEGER, got %s", args[1].Type())
			}
			if n.Value < 0 {
				return newBuiltinError("shift amount for `SHR` must be non-negative, got %d", n.Value)
			}
			shiftAmount := uint(n.Value)
			result := in.Value >> shiftAmount

			// Apply mask to ensure the result stays within the bitstring's width
			if in.Width < 64 {
				result &= (1 << in.Width) - 1
			}
			return &object.BitString{Value: result, Width: in.Width}
		},
	},
	"ROL": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for ROL. got=%d, want=2", len(args))
			}
			in, ok := args[0].(*object.BitString)
			if !ok {
				return newBuiltinError("argument 1 to `ROL` must be a bitstring type, got %s", args[0].Type())
			}
			n, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `ROL` must be INTEGER, got %s", args[1].Type())
			}
			if n.Value < 0 {
				return newBuiltinError("rotate amount for `ROL` must be non-negative, got %d", n.Value)
			}

			k := int(n.Value)
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
		},
	},
	"ROR": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for ROR. got=%d, want=2", len(args))
			}
			in, ok := args[0].(*object.BitString)
			if !ok {
				return newBuiltinError("argument 1 to `ROR` must be a bitstring type, got %s", args[0].Type())
			}
			n, ok := args[1].(*object.Integer)
			if !ok {
				return newBuiltinError("argument 2 to `ROR` must be INTEGER, got %s", args[1].Type())
			}
			if n.Value < 0 {
				return newBuiltinError("rotate amount for `ROR` must be non-negative, got %d", n.Value)
			}
			k := int(n.Value)
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
		},
	},
	"SIN": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for SIN. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `SIN` must be INTEGER or REAL, got %s", args[0].Type())
			}
			return &object.Real{Value: math.Sin(val)}
		},
	},
	"COS": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for COS. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `COS` must be INTEGER or REAL, got %s", args[0].Type())
			}
			return &object.Real{Value: math.Cos(val)}
		},
	},
	"TAN": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for TAN. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `TAN` must be INTEGER or REAL, got %s", args[0].Type())
			}
			return &object.Real{Value: math.Tan(val)}
		},
	},
	"ASIN": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for ASIN. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `ASIN` must be INTEGER or REAL, got %s", args[0].Type())
			}
			if val < -1.0 || val > 1.0 {
				return newBuiltinError("argument to `ASIN` must be between -1 and 1, got %f", val)
			}
			return &object.Real{Value: math.Asin(val)}
		},
	},
	"ACOS": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for ACOS. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `ACOS` must be INTEGER or REAL, got %s", args[0].Type())
			}
			if val < -1.0 || val > 1.0 {
				return newBuiltinError("argument to `ACOS` must be between -1 and 1, got %f", val)
			}
			return &object.Real{Value: math.Acos(val)}
		},
	},
	"ATAN": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for ATAN. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `ATAN` must be INTEGER or REAL, got %s", args[0].Type())
			}
			return &object.Real{Value: math.Atan(val)}
		},
	},
	"ATAN2": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newBuiltinError("wrong number of arguments for ATAN2. got=%d, want=2", len(args))
			}
			y, okY := getFloat64Value(args[0])
			if !okY {
				return newBuiltinError("argument 1 to `ATAN2` must be INTEGER or REAL, got %s", args[0].Type())
			}
			x, okX := getFloat64Value(args[1])
			if !okX {
				return newBuiltinError("argument 2 to `ATAN2` must be INTEGER or REAL, got %s", args[1].Type())
			}
			// IEC 61131-3 specifies ATAN2(Y, X)
			return &object.Real{Value: math.Atan2(y, x)}
		},
	},
	"LN": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for LN. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `LN` must be INTEGER or REAL, got %s", args[0].Type())
			}
			if val <= 0 {
				return newBuiltinError("argument to `LN` must be positive, got %f", val)
			}
			return &object.Real{Value: math.Log(val)}
		},
	},
	"LOG": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for LOG. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `LOG` must be INTEGER or REAL, got %s", args[0].Type())
			}
			if val <= 0 {
				return newBuiltinError("argument to `LOG` must be positive, got %f", val)
			}
			return &object.Real{Value: math.Log10(val)}
		},
	},
	"EXP": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for EXP. got=%d, want=1", len(args))
			}
			val, ok := getFloat64Value(args[0])
			if !ok {
				return newBuiltinError("argument to `EXP` must be INTEGER or REAL, got %s", args[0].Type())
			}
			return &object.Real{Value: math.Exp(val)}
		},
	},
	"SQRT": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for SQRT. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				if arg.Value < 0 {
					return newBuiltinError("argument to `SQRT` must be non-negative, got %d", arg.Value)
				}
				return &object.Real{Value: math.Sqrt(float64(arg.Value))}
			case *object.Real:
				if arg.Value < 0 {
					return newBuiltinError("argument to `SQRT` must be non-negative, got %f", arg.Value)
				}
				return &object.Real{Value: math.Sqrt(arg.Value)}
			default:
				return newBuiltinError("argument to `SQRT` not supported, got %s", args[0].Type())
			}
		},
	},
	"ROUND": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for ROUND. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.REAL_OBJ {
				return newBuiltinError("argument to `ROUND` must be REAL, got %s", args[0].Type())
			}
			realVal := args[0].(*object.Real).Value
			// Per IEC 60559 (IEEE 754), the default rounding mode is "round half to even".
			return &object.Integer{Value: int64(math.Round(realVal))}
		},
	},
	"ABS": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for ABS. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Integer{Value: int64(math.Abs(float64(arg.Value)))}
			case *object.Real:
				return &object.Real{Value: math.Abs(arg.Value)}
			default:
				return newBuiltinError("argument to `ABS` not supported, got %s", args[0].Type())
			}
		},
	},
	"TRUNC": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for TRUNC. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.REAL_OBJ {
				return newBuiltinError("argument to `TRUNC` must be REAL, got %s", args[0].Type())
			}
			realVal := args[0].(*object.Real).Value
			return &object.Integer{Value: int64(math.Trunc(realVal))}
		},
	},
	"ADD":   &object.Builtin{Fn: addBuiltin},
	"SUB":   &object.Builtin{Fn: subBuiltin},
	"MUL":   &object.Builtin{Fn: mulBuiltin},
	"DIV":   &object.Builtin{Fn: divBuiltin},
	"MOD":   &object.Builtin{Fn: modBuiltin},
	"EXPT":  &object.Builtin{Fn: exptBuiltin},
	"GT":    {Fn: comparisonBuiltin("GT")},
	"GE":    {Fn: comparisonBuiltin("GE")},
	"EQ":    {Fn: comparisonBuiltin("EQ")},
	"LE":    {Fn: comparisonBuiltin("LE")},
	"LT":    {Fn: comparisonBuiltin("LT")},
	"NE":    {Fn: comparisonBuiltin("NE")},
	"LEN":   builtins["len"], // IEC 61131-3 standard function
	"MUX":   {Fn: muxBuiltin},
	"LIMIT": {Fn: limitWrapperBuiltin},
	"SEL":   {Fn: selWrapperBuiltin},
	"MOVE":  {Fn: moveBuiltin},
	"MIN": {Fn: func(args ...object.Object) object.Object {
		return minMaxBuiltin("MIN", args...)
	}},
	"MAX": {Fn: func(args ...object.Object) object.Object {
		return minMaxBuiltin("MAX", args...)
	}},
	"AND": {Fn: func(args ...object.Object) object.Object {
		return bitwiseBuiltin("AND", args...)
	}},
	"OR": {Fn: func(args ...object.Object) object.Object {
		return bitwiseBuiltin("OR", args...)
	}},
	"XOR": {Fn: func(args ...object.Object) object.Object {
		return bitwiseBuiltin("XOR", args...)
	}},
	"NAND": {Fn: func(args ...object.Object) object.Object {
		return bitwiseBuiltin("NAND", args...)
	}},
	"NOR": {Fn: func(args ...object.Object) object.Object {
		return bitwiseBuiltin("NOR", args...)
	}},
}

func limitBuiltin(min, in, max object.Object) object.Object {
	args := []object.Object{min, in, max}
	hasReal := false
	for _, arg := range args {
		switch arg.Type() {
		case object.INTEGER_OBJ:
		case object.REAL_OBJ:
			hasReal = true
		default:
			return newBuiltinError("all arguments to `LIMIT` must be INTEGER or REAL, got %s", arg.Type())
		}
	}

	if hasReal {
		minVal, _ := getFloat64Value(min)
		inVal, _ := getFloat64Value(in)
		maxVal, _ := getFloat64Value(max)
		if inVal < minVal {
			return &object.Real{Value: minVal}
		}
		if inVal > maxVal {
			return &object.Real{Value: maxVal}
		}
		return &object.Real{Value: inVal}
	}

	// All are integers
	minVal := min.(*object.Integer).Value
	inVal := in.(*object.Integer).Value
	maxVal := max.(*object.Integer).Value
	if inVal < minVal {
		return &object.Integer{Value: minVal}
	}
	if inVal > maxVal {
		return &object.Integer{Value: maxVal}
	}
	return &object.Integer{Value: inVal}
}

func limitWrapperBuiltin(args ...object.Object) object.Object {
	if len(args) != 3 {
		return newBuiltinError("wrong number of arguments for LIMIT. got=%d, want=3", len(args))
	}
	// The standard specifies the arguments as LIMIT(MN, IN, MX).
	// We will assume this order for positional arguments.
	// Named arguments are handled by the function application logic.
	mn := args[0]
	in := args[1]
	mx := args[2]

	// For named arguments, we need to find them if they exist.
	// This is a simplified approach. A full implementation would get named args from the call site.
	// However, the current `applyFunction` logic evaluates args positionally for built-ins.
	// So we rely on the order: MN, IN, MX.

	return limitBuiltin(mn, in, mx)
}

func selWrapperBuiltin(args ...object.Object) object.Object {
	if len(args) != 3 {
		return newBuiltinError("wrong number of arguments for SEL. got=%d, want=3", len(args))
	}
	return selBuiltin(args[0], args[1], args[2])
}

func selBuiltin(g, in0, in1 object.Object) object.Object {
	gBool, ok := g.(*object.Boolean)
	if !ok {
		return newBuiltinError("argument 1 to `SEL` must be BOOLEAN, got %s", g.Type())
	}

	// The standard is strict about types matching.
	if in0.Type() != in1.Type() {
		return newBuiltinError("arguments 2 and 3 to `SEL` must be of the same type, got %s and %s", in0.Type(), in1.Type())
	}

	if gBool.Value {
		return in1
	}
	return in0
}

func minMaxBuiltin(op string, args ...object.Object) object.Object {
	if len(args) == 0 {
		return newBuiltinError("wrong number of arguments for %s. got=0, want>=1", op)
	}

	hasReal := false
	for _, arg := range args {
		switch arg.Type() {
		case object.INTEGER_OBJ:
			// continue
		case object.REAL_OBJ:
			hasReal = true
		default:
			return newBuiltinError("all arguments to `%s` must be INTEGER or REAL, got %s", op, arg.Type())
		}
	}

	if hasReal {
		result, _ := getFloat64Value(args[0])
		for i := 1; i < len(args); i++ {
			val, _ := getFloat64Value(args[i])
			if op == "MIN" {
				result = math.Min(result, val)
			} else { // MAX
				result = math.Max(result, val)
			}
		}
		return &object.Real{Value: result}
	}

	// All arguments are integers
	result := args[0].(*object.Integer).Value
	for i := 1; i < len(args); i++ {
		val := args[i].(*object.Integer).Value
		if op == "MIN" {
			if val < result {
				result = val
			}
		} else { // MAX
			if val > result {
				result = val
			}
		}
	}
	return &object.Integer{Value: result}
}

// bitwiseBuiltin is a generic helper for extensible bitwise functions (AND, OR, XOR).
func bitwiseBuiltin(op string, args ...object.Object) object.Object {
	if len(args) < 2 {
		return newBuiltinError("wrong number of arguments for %s. got=%d, want>=2", op, len(args))
	}

	// Check that all arguments are bitstrings of the same type (width).
	firstArg, ok := args[0].(*object.BitString)
	if !ok {
		return newBuiltinError("all arguments to `%s` must be bit-string types, got %s", op, args[0].Type())
	}
	width := firstArg.Width

	for i := 1; i < len(args); i++ {
		arg, ok := args[i].(*object.BitString)
		if !ok {
			return newBuiltinError("all arguments to `%s` must be bit-string types, got %s", op, args[i].Type())
		}
		if arg.Width != width {
			return newBuiltinError("all arguments to `%s` must have the same width, got %d and %d", op, width, arg.Width)
		}
	}

	// Perform the operation.
	result := firstArg.Value

	for i := 1; i < len(args); i++ {
		nextVal := args[i].(*object.BitString).Value
		switch op {
		case "AND":
			result &= nextVal
		case "OR":
			result |= nextVal
		case "XOR":
			result ^= nextVal
		default:
			return newBuiltinError("internal error: unknown bitwise operator %s", op)
		}
	}

	// For NAND and NOR, we need to negate the final result and apply a mask.
	if op == "NAND" || op == "NOR" {
		var mask uint64
		if width < 64 {
			mask = (1 << width) - 1
		} else {
			mask = 0xFFFFFFFFFFFFFFFF
		}

		if op == "NAND" {
			// The operation was AND, now we negate.
			result = ^result & mask
		} else { // NOR
			// The operation was OR, now we negate.
			result = ^result & mask
		}
	}

	return &object.BitString{Value: result, Width: width}
}

// addBuiltin implements the ADD standard function.
func addBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return newBuiltinError("wrong number of arguments for ADD. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]

	// Handle commutative cases by swapping arguments if necessary
	// e.g., Time + TimeOfDay should be treated as TimeOfDay + Time
	if _, ok := arg1.(*object.Time); ok {
		if _, ok := arg2.(*object.TimeOfDay); ok {
			return evalAddOperation(arg2, arg1) // Swap and call core logic
		}
		if _, ok := arg2.(*object.DateAndTime); ok {
			return evalAddOperation(arg2, arg1) // Swap and call core logic
		}
	}

	return evalAddOperation(arg1, arg2) // Call core logic
}

// isEqual compares two objects for equality. This is a simplified version for built-ins.
func isEqual(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}

	switch a := a.(type) {
	case *object.Integer:
		return a.Value == b.(*object.Integer).Value
	case *object.Real:
		return a.Value == b.(*object.Real).Value
	case *object.String:
		return a.Value == b.(*object.String).Value
	case *object.Boolean:
		return a == b // Can compare pointers for TRUE/FALSE singletons
	case *object.Null:
		return true // NULL is always equal to NULL
	case *object.Time:
		return a.Value == b.(*object.Time).Value
	case *object.Date:
		return a.Value.Equal(b.(*object.Date).Value)
	// Other types can be added here. For now, unhandled types are not considered equal.
	default:
		return false
	}
}

// evalAddOperation contains the core logic for the ADD builtin function.
// It handles type checking and performs the addition.
func evalAddOperation(arg1, arg2 object.Object) object.Object {
	switch a1 := arg1.(type) {
	case *object.Time:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.Time{Value: a1.Value + a2.Value}
		}
	case *object.TimeOfDay:
		if a2, ok := arg2.(*object.Time); ok {
			// Adding a duration to a time of day, with wrap-around
			newTime := a1.Value.Add(a2.Value)
			return &object.TimeOfDay{Value: newTime}
		}
	case *object.DateAndTime:
		if a2, ok := arg2.(*object.Time); ok {
			return &object.DateAndTime{Value: a1.Value.Add(a2.Value)}
		}
	case *object.Integer:
		if a2, ok := arg2.(*object.Integer); ok {
			return &object.Integer{Value: a1.Value + a2.Value}
		}
		if a2, ok := arg2.(*object.Real); ok {
			// Promote INTEGER to REAL
			return &object.Real{Value: float64(a1.Value) + a2.Value}
		}
	case *object.Real:
		if a2, ok := arg2.(*object.Real); ok {
			return &object.Real{Value: a1.Value + a2.Value}
		}
		if a2, ok := arg2.(*object.Integer); ok {
			// Promote INTEGER to REAL
			return &object.Real{Value: a1.Value + float64(a2.Value)}
		}
	}
	return newBuiltinError("unsupported argument types for ADD: %s + %s", arg1.Type(), arg2.Type())
}

// subBuiltin implements the SUB standard function.
func subBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return newBuiltinError("wrong number of arguments for SUB. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]

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
	case *object.Integer:
		if a2, ok := arg2.(*object.Integer); ok {
			return &object.Integer{Value: a1.Value - a2.Value}
		}
		if a2, ok := arg2.(*object.Real); ok {
			// Promote INTEGER to REAL
			return &object.Real{Value: float64(a1.Value) - a2.Value}
		}
	case *object.Real:
		if a2, ok := arg2.(*object.Real); ok {
			return &object.Real{Value: a1.Value - a2.Value}
		}
		if a2, ok := arg2.(*object.Integer); ok {
			// Promote INTEGER to REAL
			return &object.Real{Value: a1.Value - float64(a2.Value)}
		}
	}

	return newBuiltinError("unsupported argument types for SUB: %s - %s", arg1.Type(), arg2.Type())
}

// moveBuiltin implements the MOVE standard function.
func moveBuiltin(args ...object.Object) object.Object {
	if len(args) != 1 {
		return newBuiltinError("wrong number of arguments for MOVE. got=%d, want=1", len(args))
	}
	// MOVE function simply returns its input, which can then be assigned.
	// The actual assignment is handled by the calling expression.
	return args[0]
}

// mulBuiltin implements the MUL standard function.
func mulBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return newBuiltinError("wrong number of arguments for MUL. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]

	// Handle TIME * ANY_NUM and ANY_NUM * TIME
	if t, ok := arg1.(*object.Time); ok {
		if num, ok := getFloat64Value(arg2); ok {
			return &object.Time{Value: time.Duration(float64(t.Value) * num)}
		}
	}
	if t, ok := arg2.(*object.Time); ok {
		if num, ok := getFloat64Value(arg1); ok {
			return &object.Time{Value: time.Duration(float64(t.Value) * num)}
		}
	}

	// Handle numeric types
	switch a1 := arg1.(type) {
	case *object.Integer:
		if a2, ok := arg2.(*object.Integer); ok {
			return &object.Integer{Value: a1.Value * a2.Value}
		}
		if a2, ok := arg2.(*object.Real); ok {
			return &object.Real{Value: float64(a1.Value) * a2.Value}
		}
	case *object.Real:
		if a2, ok := arg2.(*object.Real); ok {
			return &object.Real{Value: a1.Value * a2.Value}
		}
		if a2, ok := arg2.(*object.Integer); ok {
			return &object.Real{Value: a1.Value * float64(a2.Value)}
		}
	}

	return newBuiltinError("unsupported argument types for MUL: %s * %s", arg1.Type(), arg2.Type())
}

// divBuiltin implements the DIV standard function.
func divBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return newBuiltinError("wrong number of arguments for DIV. got=%d, want=2", len(args))
	}
	arg1 := args[0]
	arg2 := args[1]

	// Handle TIME / ANY_NUM
	if t, ok := arg1.(*object.Time); ok {
		if num, ok := getFloat64Value(arg2); ok {
			if num == 0 {
				return newBuiltinError("division by zero")
			} // time.Duration is int64 nanoseconds, so convert to float64 for division
			return &object.Time{Value: time.Duration(float64(t.Value) / num)}
		}
	}

	// Handle numeric types
	switch a1 := arg1.(type) {
	case *object.Integer:
		if a2, ok := arg2.(*object.Integer); ok {
			if a2.Value == 0 {
				return newBuiltinError("division by zero")
			}
			return &object.Integer{Value: a1.Value / a2.Value}
		}
		if a2, ok := arg2.(*object.Real); ok {
			if a2.Value == 0.0 {
				return newBuiltinError("division by zero")
			}
			return &object.Real{Value: float64(a1.Value) / a2.Value}
		}
	case *object.Real:
		if num, ok := getFloat64Value(arg2); ok {
			if num == 0.0 {
				return newBuiltinError("division by zero")
			}
			return &object.Real{Value: a1.Value / num}
		}
	}

	return newBuiltinError("unsupported argument types for DIV: %s / %s", arg1.Type(), arg2.Type())
}

// modBuiltin implements the MOD standard function.
func modBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return newBuiltinError("wrong number of arguments for MOD. got=%d, want=2", len(args))
	}
	arg1, ok1 := args[0].(*object.Integer)
	arg2, ok2 := args[1].(*object.Integer)

	if !ok1 || !ok2 {
		return newBuiltinError("arguments to `MOD` must be INTEGER, got %s and %s", args[0].Type(), args[1].Type())
	}

	if arg2.Value == 0 {
		return newBuiltinError("division by zero in MOD")
	}

	return &object.Integer{Value: arg1.Value % arg2.Value}
}

// exptBuiltin implements the EXPT standard function.
func exptBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return newBuiltinError("wrong number of arguments for EXPT. got=%d, want=2", len(args))
	}

	base, ok1 := getFloat64Value(args[0])
	if !ok1 {
		return newBuiltinError("argument 1 to `EXPT` must be numeric, got %s", args[0].Type())
	}

	exponent, ok2 := getFloat64Value(args[1])
	if !ok2 {
		return newBuiltinError("argument 2 to `EXPT` must be numeric, got %s", args[1].Type())
	}

	// IEC 61131-3 specifies that EXPT returns a REAL.
	result := math.Pow(base, exponent)

	return &object.Real{Value: result}
}

// comparisonBuiltin is a factory for creating comparison functions (GT, GE, EQ, LE, LT, NE).
func comparisonBuiltin(op string) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return newBuiltinError("wrong number of arguments for %s. got=%d, want=2", op, len(args))
		}
		return evalComparison(op, args[0], args[1])
	}
}

// evalComparison centralizes the logic for all comparison operations.
func evalComparison(op string, left, right object.Object) object.Object {
	// Type promotion for REAL and INTEGER
	if l, ok := left.(*object.Integer); ok {
		if _, ok := right.(*object.Real); ok {
			left = &object.Real{Value: float64(l.Value)}
		}
	}
	if _, ok := left.(*object.Real); ok {
		if r, ok := right.(*object.Integer); ok {
			right = &object.Real{Value: float64(r.Value)}
		}
	}

	if left.Type() != right.Type() {
		return newBuiltinError("type mismatch for comparison: %s %s %s", left.Type(), op, right.Type())
	}

	var result bool
	switch l := left.(type) {
	case *object.Integer:
		r := right.(*object.Integer).Value
		switch op {
		case "GT":
			result = l.Value > r
		case "GE":
			result = l.Value >= r
		case "EQ":
			result = l.Value == r
		case "LE":
			result = l.Value <= r
		case "LT":
			result = l.Value < r
		case "NE":
			result = l.Value != r
		}
	case *object.Real:
		r := right.(*object.Real).Value
		switch op {
		case "GT":
			result = l.Value > r
		case "GE":
			result = l.Value >= r
		case "EQ":
			result = l.Value == r
		case "LE":
			result = l.Value <= r
		case "LT":
			result = l.Value < r
		case "NE":
			result = l.Value != r
		}
	case *object.String:
		r := right.(*object.String).Value
		switch op {
		case "GT":
			result = l.Value > r
		case "GE":
			result = l.Value >= r
		case "EQ":
			result = l.Value == r
		case "LE":
			result = l.Value <= r
		case "LT":
			result = l.Value < r
		case "NE":
			result = l.Value != r
		}
	case *object.Boolean:
		r := right.(*object.Boolean).Value
		// For booleans, only EQ and NE are typically used, but others are valid.
		// We can treat FALSE as 0 and TRUE as 1 for comparison.
		li, ri := 0, 0
		if l.Value {
			li = 1
		}
		if r {
			ri = 1
		}
		return evalComparison(op, &object.Integer{Value: int64(li)}, &object.Integer{Value: int64(ri)})

	default:
		// For other types, fall back to simple equality/inequality checks.
		// This covers TIME, DATE, etc., where direct value comparison is meaningful.
		if op == "EQ" {
			return nativeBoolToBooleanObject(isEqual(left, right))
		}
		if op == "NE" {
			return nativeBoolToBooleanObject(!isEqual(left, right))
		}
		return newBuiltinError("unsupported operand types for %s: %s", op, left.Type())
	}

	return nativeBoolToBooleanObject(result)
}

// muxBuiltin implements the MUX standard function.
func muxBuiltin(args ...object.Object) object.Object {
	if len(args) < 2 {
		return newBuiltinError("wrong number of arguments for MUX. got=%d, want>=2", len(args))
	}

	k, ok := args[0].(*object.Integer)
	if !ok {
		return newBuiltinError("argument 1 to `MUX` must be INTEGER, got %s", args[0].Type())
	}

	valueArgs := args[1:]
	numInputs := len(valueArgs)

	if k.Value < 0 || k.Value >= int64(numInputs) {
		return newBuiltinError("index %d out of bounds for MUX with %d inputs", k.Value, numInputs)
	}

	// Check that all value arguments are of the same type
	if numInputs > 1 {
		firstType := valueArgs[0].Type()
		for i := 1; i < numInputs; i++ {
			if valueArgs[i].Type() != firstType {
				return newBuiltinError("all value arguments to `MUX` must be of the same type, got %s but expected %s", valueArgs[i].Type(), firstType)
			}
		}
	}

	return valueArgs[k.Value]
}

// genericConversionBuiltin creates a built-in function on the fly for `*_TO_*` conversions.
func genericConversionBuiltin(fromType, toType string) *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for %s_TO_%s. got=%d, want=1", fromType, toType, len(args))
			}
			return applyConversion(args[0], fromType, toType)
		},
	}
}

var integerTypeRanges = map[string]struct {
	minSigned   int64
	maxSigned   int64
	maxUnsigned uint64
}{
	"SINT":  {math.MinInt8, math.MaxInt8, 0},
	"INT":   {math.MinInt16, math.MaxInt16, 0},
	"DINT":  {math.MinInt32, math.MaxInt32, 0},
	"LINT":  {math.MinInt64, math.MaxInt64, 0},
	"USINT": {0, 0, math.MaxUint8},
	"UINT":  {0, 0, math.MaxUint16},
	"UDINT": {0, 0, math.MaxUint32},
	"ULINT": {0, 0, math.MaxUint64},
}

var bitStringTypeRanges = map[string]uint64{
	"BYTE":  math.MaxUint8,
	"WORD":  math.MaxUint16,
	"DWORD": math.MaxUint32,
	"LWORD": math.MaxUint64,
}

// applyConversion handles the logic for converting an object from one type to another.
func applyConversion(input object.Object, fromType, toType string) object.Object {
	// Validate that the input object's type matches the 'fromType' part of the function name.
	// This is a sanity check; the language is strongly typed, but this adds robustness.
	if !strings.HasPrefix(string(input.Type()), fromType) && fromType != "ANY_INT" && fromType != "ANY_REAL" {
		// Allow ANY_INT to be converted from any integer type, etc.
		// This is a simplification; a full implementation would check generic type hierarchies.
		isNumericConversion := (fromType == "ANY_INT" && isIntegerType(string(input.Type()))) ||
			(fromType == "ANY_REAL" && isNumeric(input))

		if !isNumericConversion {
			return newBuiltinError("type mismatch for %s_TO_%s: input is %s, expected %s", fromType, toType, input.Type(), fromType)
		}
	}

	// Handle conversions to Integer types
	if isIntegerType(toType) {
		switch val := input.(type) {
		case *object.Integer:
			targetRange, ok := integerTypeRanges[toType]
			if !ok {
				return newBuiltinError("internal error: unknown integer type %s", toType)
			}
			// Check signed vs unsigned ranges
			if strings.HasPrefix(toType, "U") { // Unsigned
				if val.Value < 0 || uint64(val.Value) > targetRange.maxUnsigned {
					return newBuiltinError("value %d is out of range for type %s (0 to %d)", val.Value, toType, targetRange.maxUnsigned)
				}
			} else { // Signed
				if val.Value < targetRange.minSigned || val.Value > targetRange.maxSigned {
					return newBuiltinError("value %d is out of range for type %s (%d to %d)", val.Value, toType, targetRange.minSigned, targetRange.maxSigned)
				}
			}
			// The value fits, so we can return it.
			// Our internal object.Integer is int64, which can represent all target types.
			return &object.Integer{Value: val.Value}
		case *object.Real:
			// Per IEC 61131-3 (Table 22, footnote b), REAL to INT conversion uses rounding.
			return &object.Integer{Value: int64(math.Round(val.Value))}
		case *object.String:
			i, err := strconv.ParseInt(val.Value, 10, 64)
			if err != nil {
				return newBuiltinError("could not parse string to integer: %s", val.Value)
			}
			return &object.Integer{Value: i}
		default:
			return newBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
		}
	}

	// Handle conversions to Real types
	if isRealType(toType) {
		switch val := input.(type) {
		case *object.Integer:
			return &object.Real{Value: float64(val.Value)}
		case *object.Real:
			return &object.Real{Value: val.Value}
		case *object.String:
			f, err := strconv.ParseFloat(val.Value, 64)
			if err != nil {
				return newBuiltinError("could not parse string to real: %s", val.Value)
			}
			return &object.Real{Value: f}
		default:
			return newBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
		}
	}

	// Handle conversions to String types
	if isStringType(toType) {
		return &object.String{Value: input.Inspect()}
	}

	// Handle conversions to Bit-string types (BYTE, WORD, etc.)
	if isBitStringType(toType) {
		maxVal, ok := bitStringTypeRanges[toType]
		if !ok {
			return newBuiltinError("internal error: unknown bitstring type %s", toType)
		}
		width, _ := getBitStringWidth(toType)
		switch val := input.(type) {
		case *object.Integer:
			if val.Value < 0 || uint64(val.Value) > maxVal {
				return newBuiltinError("value %d is out of range for type %s (0 to %d)", val.Value, toType, maxVal)
			}
			return &object.BitString{Value: uint64(val.Value), Width: width}
		default:
			return newBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
		}
	}

	// Handle BCD conversions
	if toType == "BCD" {
		switch val := input.(type) {
		case *object.Integer:
			bcd, err := intToBcd(val.Value)
			if err != nil {
				return newBuiltinError(err.Error())
			}
			// BCD is represented as a WORD (16-bit)
			return &object.BitString{Value: uint64(bcd), Width: 16}
		default:
			return newBuiltinError("conversion from %s to BCD is not supported", input.Type())
		}
	}
	if fromType == "BCD" {
		if isIntegerType(toType) {
			return bcdToInt(input)
		}
	}

	return newBuiltinError("conversion to type %s is not supported", toType)
}

func isIntegerType(typeName string) bool {
	return typeName == "SINT" || typeName == "INT" || typeName == "DINT" || typeName == "LINT" ||
		typeName == "USINT" || typeName == "UINT" || typeName == "UDINT" || typeName == "ULINT"
}

func isRealType(typeName string) bool {
	return typeName == "REAL" || typeName == "LREAL"
}

func isStringType(typeName string) bool {
	return typeName == "STRING" || typeName == "WSTRING"
}

func isBitStringType(typeName string) bool {
	_, ok := getBitStringWidth(typeName)
	return ok
}

func getBitStringWidth(typeName string) (int, bool) {
	switch typeName {
	case "BYTE":
		return 8, true
	case "WORD":
		return 16, true
	case "DWORD":
		return 32, true
	case "LWORD":
		return 64, true
	default:
		return 0, false
	}
}

// intToBcd converts an integer to its 4-digit BCD representation in a uint16.
func intToBcd(val int64) (uint16, error) {
	if val < 0 || val > 9999 {
		return 0, fmt.Errorf("value %d out of range for 4-digit BCD conversion (0-9999)", val)
	}

	var bcd uint16
	shift := uint(0)

	// Handle the case of 0 explicitly
	if val == 0 {
		return 0, nil
	}

	tempVal := val
	for i := 0; i < 4; i++ {
		digit := tempVal % 10
		bcd |= uint16(digit) << shift
		tempVal /= 10
		shift += 4
	}

	return bcd, nil
}

// bcdToInt converts a BCD value (from a BitString) to an Integer object.
func bcdToInt(input object.Object) object.Object {
	bs, ok := input.(*object.BitString)
	if !ok || bs.Width != 16 {
		return newBuiltinError("argument for BCD_TO_INT must be a WORD (16-bit BitString), got %s", input.Type())
	}

	bcdVal := uint16(bs.Value)
	var result int64
	var multiplier int64 = 1

	for i := 0; i < 4; i++ {
		nibble := (bcdVal >> (i * 4)) & 0xF
		if nibble > 9 {
			return newBuiltinError("invalid BCD format: nibble %d has value %d > 9", i, nibble)
		}
		result += int64(nibble) * multiplier
		multiplier *= 10
	}

	return &object.Integer{Value: result}
}

// evalTON implements the logic for the TON (Timer On-Delay) standard function block.
func evalTON(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get inputs from the instance environment (set by applyFunction)
	in, _ := instanceEnv.Get("IN")
	pt, _ := instanceEnv.Get("PT")

	// 2. Get internal state variables from the instance environment
	startTimeObj, _ := instanceEnv.Get("__startTime")
	timerActiveObj, _ := instanceEnv.Get("__timerActive")

	// Type assertions
	inBool, _ := in.(*object.Boolean)
	ptDuration, _ := pt.(*object.Time)
	if inBool == nil || ptDuration == nil {
		return newBuiltinError("TON requires IN (BOOL) and PT (TIME) inputs")
	}

	var startTime time.Time
	if startTimeObj != nil {
		startTime = startTimeObj.(*object.TimeOfDay).Value
	}
	timerActive := timerActiveObj == TRUE

	var et time.Duration
	q := FALSE

	if inBool == TRUE {
		if !timerActive {
			// Rising edge of IN: start the timer
			instanceEnv.Set("__startTime", &object.TimeOfDay{Value: nowFunc()})
			instanceEnv.Set("__timerActive", TRUE)
			timerActive = true
			startTime = nowFunc()
		}

		if timerActive {
			et = nowFunc().Sub(startTime)
			if et >= ptDuration.Value {
				et = ptDuration.Value
				q = TRUE
			}
		}
	} else {
		// IN is FALSE: reset the timer
		instanceEnv.Set("__timerActive", FALSE)
		instanceEnv.Set("__startTime", nil)
		et = 0
		q = FALSE
	}

	// 3. Set outputs in the instance environment
	instanceEnv.Set("Q", q)
	instanceEnv.Set("ET", &object.Time{Value: et})

	return q // The primary output of TON is Q
}

// evalTOF implements the logic for the TOF (Timer Off-Delay) standard function block.
func evalTOF(instanceEnv, callEnv *object.Environment) object.Object {
	in, _ := instanceEnv.Get("IN")
	pt, _ := instanceEnv.Get("PT")
	stopTimeObj, _ := instanceEnv.Get("__stopTime")

	inBool, _ := in.(*object.Boolean)
	ptDuration, _ := pt.(*object.Time)
	if inBool == nil || ptDuration == nil {
		return newBuiltinError("TOF requires IN (BOOL) and PT (TIME) inputs")
	}

	var stopTime time.Time
	if stopTimeObj != nil {
		stopTime = stopTimeObj.(*object.TimeOfDay).Value
	}

	var et time.Duration
	q := FALSE

	if inBool == TRUE {
		instanceEnv.Set("__stopTime", nil)
		q = TRUE
		et = 0
	} else {
		// Falling edge of IN
		if stopTime.IsZero() {
			stopTime = nowFunc()
			instanceEnv.Set("__stopTime", &object.TimeOfDay{Value: stopTime})
		}

		et = nowFunc().Sub(stopTime)
		if et < ptDuration.Value {
			q = TRUE
		} else {
			et = ptDuration.Value
			q = FALSE
		}
	}

	instanceEnv.Set("Q", q)
	instanceEnv.Set("ET", &object.Time{Value: et})

	return q
}

// evalCTU implements the logic for the CTU (Counter Up) standard function block.
func evalCTU(instanceEnv, callEnv *object.Environment) object.Object {
	cu, _ := instanceEnv.Get("CU")
	r, _ := instanceEnv.Get("R")
	pv, _ := instanceEnv.Get("PV")
	lastCU, _ := instanceEnv.Get("__lastCU")
	cvObj, _ := instanceEnv.Get("CV")

	cuBool, _ := cu.(*object.Boolean)
	rBool, _ := r.(*object.Boolean)
	pvInt, _ := pv.(*object.Integer)
	if cuBool == nil || rBool == nil || pvInt == nil {
		return newBuiltinError("CTU requires CU (BOOL), R (BOOL), and PV (INT) inputs")
	}

	lastCUBool := lastCU == TRUE
	var cv int64
	if cvInt, ok := cvObj.(*object.Integer); ok {
		cv = cvInt.Value
	}

	if rBool == TRUE {
		cv = 0
	} else if cuBool == TRUE && !lastCUBool { // Rising edge on CU
		if cv < pvInt.Value { // Standard says count up to max value, but PV is a practical limit
			cv++
		}
	}

	q := nativeBoolToBooleanObject(cv >= pvInt.Value)

	instanceEnv.Set("__lastCU", cuBool)
	instanceEnv.Set("Q", q)
	instanceEnv.Set("CV", &object.Integer{Value: cv})

	return q
}

// evalCTD implements the logic for the CTD (Counter Down) standard function block.
func evalCTD(instanceEnv, callEnv *object.Environment) object.Object {
	cd, _ := instanceEnv.Get("CD")
	ld, _ := instanceEnv.Get("LD")
	pv, _ := instanceEnv.Get("PV")
	lastCD, _ := instanceEnv.Get("__lastCD")
	cvObj, _ := instanceEnv.Get("CV")

	cdBool, _ := cd.(*object.Boolean)
	ldBool, _ := ld.(*object.Boolean)
	pvInt, _ := pv.(*object.Integer)
	if cdBool == nil || ldBool == nil || pvInt == nil {
		return newBuiltinError("CTD requires CD (BOOL), LD (BOOL), and PV (INT) inputs")
	}

	lastCDBool := lastCD == TRUE
	var cv int64
	if cvInt, ok := cvObj.(*object.Integer); ok {
		cv = cvInt.Value
	}

	if ldBool == TRUE {
		cv = pvInt.Value
	} else if cdBool == TRUE && !lastCDBool { // Rising edge on CD
		if cv > 0 { // Standard says count down to min value
			cv--
		}
	}

	q := nativeBoolToBooleanObject(cv <= 0)

	instanceEnv.Set("__lastCD", cdBool)
	instanceEnv.Set("Q", q)
	instanceEnv.Set("CV", &object.Integer{Value: cv})

	return q
}

// evalR_TRIG implements the logic for the R_TRIG (Rising Edge Trigger) standard function block.
func evalR_TRIG(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get CLK input
	clkObj, _ := instanceEnv.Get("CLK")

	// 2. Get internal state (the edge memory bit)
	edgeMemObj, _ := instanceEnv.Get("__edge_mem")

	// 3. Type assertions and defaults
	clk, ok := clkObj.(*object.Boolean)
	if !ok {
		// If CLK is not provided or not a BOOL, Q is FALSE.
		instanceEnv.Set("Q", FALSE)
		instanceEnv.Set("__edge_mem", FALSE) // Ensure memory is reset
		return FALSE
	}

	edgeMem := edgeMemObj == TRUE

	// 4. R_TRIG Logic: Q is TRUE if CLK is TRUE and the memory bit is FALSE.
	q := nativeBoolToBooleanObject(clk.Value && !edgeMem)

	// 5. Update internal state and output
	// The memory bit follows the CLK input.
	instanceEnv.Set("__edge_mem", clk)
	instanceEnv.Set("Q", q)

	// 6. Return the primary output Q
	return q
}

// evalF_TRIG implements the logic for the F_TRIG (Falling Edge Trigger) standard function block.
func evalF_TRIG(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get CLK input
	clkObj, _ := instanceEnv.Get("CLK")

	// 2. Get internal state (the edge memory bit)
	edgeMemObj, _ := instanceEnv.Get("__edge_mem")

	// 3. Type assertions and defaults
	clk, ok := clkObj.(*object.Boolean)
	if !ok {
		// If CLK is not provided or not a BOOL, Q is FALSE.
		instanceEnv.Set("Q", FALSE)
		instanceEnv.Set("__edge_mem", FALSE) // Ensure memory is reset
		return FALSE
	}

	edgeMem := edgeMemObj == TRUE

	// 4. F_TRIG Logic: Q is TRUE if CLK is FALSE and the memory bit is TRUE.
	q := nativeBoolToBooleanObject(!clk.Value && edgeMem)

	// 5. Update internal state and output
	// The memory bit follows the CLK input.
	instanceEnv.Set("__edge_mem", clk)
	instanceEnv.Set("Q", q)

	// 6. Return the primary output Q
	return q
}

// evalCTUD implements the logic for the CTUD (Up/Down Counter) standard function block.
func evalCTUD(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get inputs
	cu, _ := instanceEnv.Get("CU")
	cd, _ := instanceEnv.Get("CD")
	r, _ := instanceEnv.Get("R")
	ld, _ := instanceEnv.Get("LD")
	pv, _ := instanceEnv.Get("PV")

	// 2. Get internal state
	lastCU, _ := instanceEnv.Get("__lastCU")
	lastCD, _ := instanceEnv.Get("__lastCD")
	cvObj, _ := instanceEnv.Get("CV")

	// 3. Type assertions and defaults
	cuBool, _ := cu.(*object.Boolean)
	cdBool, _ := cd.(*object.Boolean)
	rBool, _ := r.(*object.Boolean)
	ldBool, _ := ld.(*object.Boolean)
	pvInt, _ := pv.(*object.Integer)
	if cuBool == nil || cdBool == nil || rBool == nil || ldBool == nil || pvInt == nil {
		return newBuiltinError("CTUD requires CU, CD, R, LD (BOOL) and PV (INT) inputs")
	}

	lastCUBool := lastCU == TRUE
	lastCDBool := lastCD == TRUE
	var cv int64
	if cvInt, ok := cvObj.(*object.Integer); ok {
		cv = cvInt.Value
	}

	// 4. CTUD Logic
	// Reset has priority over Load
	if rBool == TRUE {
		cv = 0
	} else if ldBool == TRUE {
		cv = pvInt.Value
	} else {
		cuRising := cuBool == TRUE && !lastCUBool
		cdRising := cdBool == TRUE && !lastCDBool

		// Per the standard, if both count up and count down are triggered, nothing happens.
		if cuRising && !cdRising {
			// The standard allows counting up to the max value of the integer type.
			if cv < math.MaxInt16 { // Assuming INT for now, could be DINT/LINT
				cv++
			}
		} else if cdRising && !cuRising {
			// The standard allows counting down to the min value of the integer type.
			if cv > math.MinInt16 {
				cv--
			}
		}
	}

	// 5. Update outputs and internal state
	instanceEnv.Set("__lastCU", cuBool)
	instanceEnv.Set("__lastCD", cdBool)
	instanceEnv.Set("CV", &object.Integer{Value: cv})
	instanceEnv.Set("QU", nativeBoolToBooleanObject(cv >= pvInt.Value))
	instanceEnv.Set("QD", nativeBoolToBooleanObject(cv <= 0))

	// CTUD does not have a primary return value. Outputs are accessed via member variables.
	return NULL
}

// evalTP implements the logic for the TP (Pulse Timer) standard function block.
func evalTP(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get inputs
	in, _ := instanceEnv.Get("IN")
	pt, _ := instanceEnv.Get("PT")

	// 2. Get internal state
	startTimeObj, _ := instanceEnv.Get("__startTime")
	pulseActiveObj, _ := instanceEnv.Get("__pulseActive")
	lastIN, _ := instanceEnv.Get("__lastIN")

	// 3. Type assertions and defaults
	inBool, _ := in.(*object.Boolean)
	ptDuration, _ := pt.(*object.Time)
	if inBool == nil || ptDuration == nil {
		return newBuiltinError("TP requires IN (BOOL) and PT (TIME) inputs")
	}

	var startTime time.Time
	if startTimeObj != nil {
		startTime = startTimeObj.(*object.TimeOfDay).Value
	}
	pulseActive := pulseActiveObj == TRUE
	lastINBool := lastIN == TRUE

	var et time.Duration
	q := FALSE

	// 4. TP Logic
	// A rising edge on IN starts the pulse, but only if a pulse is not already active.
	if inBool == TRUE && !lastINBool && !pulseActive {
		pulseActive = true
		startTime = nowFunc()
		instanceEnv.Set("__pulseActive", TRUE)
		instanceEnv.Set("__startTime", &object.TimeOfDay{Value: startTime})
	}

	if pulseActive {
		et = nowFunc().Sub(startTime)
		if et < ptDuration.Value {
			q = TRUE
		} else {
			// Pulse has finished
			et = ptDuration.Value
			q = FALSE
			// Reset internal state
			pulseActive = false
			instanceEnv.Set("__pulseActive", FALSE)
			instanceEnv.Set("__startTime", nil)
		}
	} else {
		// Not active, Q is FALSE and ET is 0
		q = FALSE
		et = 0
	}

	// 5. Update outputs and internal state
	instanceEnv.Set("__lastIN", inBool)
	instanceEnv.Set("Q", q)
	instanceEnv.Set("ET", &object.Time{Value: et})

	return q
}

// evalSR implements the logic for the SR (Set-Reset) bistable function block.
// Reset (R) input has priority over the Set (S1) input.
func evalSR(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get inputs
	s1, _ := instanceEnv.Get("S1")
	r, _ := instanceEnv.Get("R")

	// 2. Get internal state (the output Q1)
	q1Obj, _ := instanceEnv.Get("Q1")

	// 3. Type assertions and defaults
	s1Bool, _ := s1.(*object.Boolean)
	rBool, _ := r.(*object.Boolean)
	if s1Bool == nil || rBool == nil {
		return newBuiltinError("SR requires S1 (BOOL) and R (BOOL) inputs")
	}

	q1 := q1Obj == TRUE

	// 4. SR Logic: Reset has priority.
	// Q1 := (S1 OR Q1) AND (NOT R);
	// Or, as a clearer sequence:
	if r == TRUE {
		q1 = false
	} else if s1 == TRUE {
		q1 = true
	}
	// If both are FALSE, q1 remains unchanged.

	// 5. Update output and return
	q1Result := nativeBoolToBooleanObject(q1)
	instanceEnv.Set("Q1", q1Result)

	return q1Result
}

// evalRS implements the logic for the RS (Reset-Set) bistable function block.
// Set (S) input has priority over the Reset (R1) input.
func evalRS(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get inputs
	s, _ := instanceEnv.Get("S")
	r1, _ := instanceEnv.Get("R1")

	// 2. Get internal state (the output Q1)
	q1Obj, _ := instanceEnv.Get("Q1")

	// 3. Type assertions and defaults
	sBool, _ := s.(*object.Boolean)
	r1Bool, _ := r1.(*object.Boolean)
	if sBool == nil || r1Bool == nil {
		return newBuiltinError("RS requires S (BOOL) and R1 (BOOL) inputs")
	}

	q1 := q1Obj == TRUE

	// 4. RS Logic: Set has priority.
	// Q1 := S OR (Q1 AND (NOT R1));
	// Or, as a clearer sequence:
	if s == TRUE {
		q1 = true
	} else if r1 == TRUE {
		q1 = false
	}
	// If both are FALSE, q1 remains unchanged.

	// 5. Update output and return
	q1Result := nativeBoolToBooleanObject(q1)
	instanceEnv.Set("Q1", q1Result)

	return q1Result
}
