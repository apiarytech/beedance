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
	"strings"
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

			result := in.Value << uint(n.Value)
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
			result := in.Value >> uint(n.Value)
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
			// math.Round rounds half to even, we need half up.
			return &object.Integer{Value: int64(math.Floor(realVal + 0.5))}
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
	"TO_INT": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for TO_INT. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Real:
				return &object.Integer{Value: int64(arg.Value)}
			case *object.Integer:
				return arg
			default:
				return newBuiltinError("argument to `TO_INT` not supported, got %s", args[0].Type())
			}
		},
	},
	"TO_REAL": &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newBuiltinError("wrong number of arguments for TO_REAL. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Real{Value: float64(arg.Value)}
			case *object.Real:
				return arg
			default:
				return newBuiltinError("argument to `TO_REAL` not supported, got %s", args[0].Type())
			}
		},
	},
	"ADD": &object.Builtin{Fn: addBuiltin},
	"SUB": &object.Builtin{Fn: subBuiltin},
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

// Helper to get a float64 value from an INTEGER or REAL object.
func getFloat64Value(obj object.Object) (float64, bool) {
	switch o := obj.(type) {
	case *object.Integer:
		return float64(o.Value), true
	case *object.Real:
		return o.Value, true
	default:
		return 0, false
	}
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
