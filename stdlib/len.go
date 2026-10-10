/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and a
 * commercial license. You may choose to use this software under either license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

import (
	"github.com/apiarytech/beedance/object"
	"strings"
)

func init() {
	// Register built-ins with their explicit index from the iota-generated constants.
	// This ensures a stable index across the compiler and VM.
	object.RegisterBuiltin(BuiltinLen, "LEN", lenFn)
	object.RegisterBuiltin(BuiltinPush, "PUSH", pushFn)
	object.RegisterBuiltin(BuiltinPop, "POP", popFn)
	object.RegisterBuiltin(BuiltinLowerBound, "LOWER_BOUND", lowerBoundFn)
	object.RegisterBuiltin(BuiltinUpperBound, "UPPER_BOUND", upperBoundFn)
	object.RegisterBuiltin(BuiltinFirst, "FIRST", firstFn)
	object.RegisterBuiltin(BuiltinLast, "LAST", lastFn)
	object.RegisterBuiltin(BuiltinRest, "REST", restFn)
	object.RegisterBuiltin(BuiltinInsert, "INSERT", insertFn)
	object.RegisterBuiltin(BuiltinDelete, "DELETE", deleteFn)
	object.RegisterBuiltin(BuiltinConcat, "CONCAT", concatFn)
	object.RegisterBuiltin(BuiltinLeft, "LEFT", leftFn)
	object.RegisterBuiltin(BuiltinRight, "RIGHT", rightFn)
	object.RegisterBuiltin(BuiltinMid, "MID", midFn)
	object.RegisterBuiltin(BuiltinFind, "FIND", findFn)
	object.RegisterBuiltin(BuiltinReplace, "REPLACE", replaceFn)
}

// lenFn is the implementation for the LEN built-in function.
var lenFn = func(args ...object.Object) object.Object {
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
}

// pushFn is the implementation for the PUSH built-in function.
var pushFn = func(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for PUSH. got=%d, want=2", len(args))
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return object.NewBuiltinError("argument to `PUSH` must be ARRAY, got %s", args[0].Type())
	}
	length := len(arr.Elements)
	newElements := make([]object.Object, length+1)
	copy(newElements, arr.Elements)
	newElements[length] = args[1]
	return &object.Array{Elements: newElements}
}

// popFn is the implementation for the POP built-in function. It returns a new
// array with the last element removed, which is consistent with the immutable
// style of other array functions like PUSH and REST.
var popFn = func(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("wrong number of arguments for POP. got=%d, want=1", len(args))
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return object.NewBuiltinError("argument to `POP` must be ARRAY, got %s", args[0].Type())
	}
	length := len(arr.Elements)
	if length == 0 {
		return object.NULL // Consistent with FIRST/LAST on empty array
	}
	// Returns a new array containing all but the last element.
	newElements := make([]object.Object, length-1)
	copy(newElements, arr.Elements[:length-1])
	return &object.Array{Elements: newElements}
}

var lowerBoundFn = func(args ...object.Object) object.Object {
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
}

var upperBoundFn = func(args ...object.Object) object.Object {
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
}

var firstFn = func(args ...object.Object) object.Object {
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
	return object.NULL
}

var lastFn = func(args ...object.Object) object.Object {
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
	return object.NULL
}

var restFn = func(args ...object.Object) object.Object {
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
	return object.NULL
}

var insertFn = func(args ...object.Object) object.Object {
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
		// IEC 61131-3: IN2 goes after the P-th character of IN1 (P 0: in front).
		idx := min(max(p, 0), strLen)
		return &object.String{Value: in1.Value[:idx] + in2.Value + in1.Value[idx:]}
	case *object.WString:
		in2, ok := args[1].(*object.WString)
		if !ok {
			return object.NewBuiltinError("argument 2 to `INSERT` for wstrings must be WSTRING, got %s", args[1].Type())
		}
		strLen := int64(len(in1.Value))
		// IEC 61131-3: IN2 goes after the P-th character of IN1 (P 0: in front).
		idx := min(max(p, 0), strLen)
		return &object.WString{Value: in1.Value[:idx] + in2.Value + in1.Value[idx:]}
	default:
		return object.NewBuiltinError("argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING, got %s", args[0].Type())
	}
}

var deleteFn = func(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("wrong number of arguments for DELETE. got=%d, want=3", len(args))
	}
	// IEC 61131-3: DELETE(IN, L, P) deletes L characters from position P.
	l, _, ok := object.GetIntegerObjectValue(args[1]) // L is the length
	if !ok {
		return object.NewBuiltinError("argument 2 to `DELETE` must be INTEGER, got %s", args[1].Type())
	}
	p, _, ok := object.GetIntegerObjectValue(args[2]) // P is the position
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
		// Build a new array; the input array is left unchanged.
		newElements := make([]object.Object, 0, arrLen-(end-start))
		newElements = append(newElements, in1.Elements[:start]...)
		newElements = append(newElements, in1.Elements[end:]...)
		return &object.Array{Elements: newElements}
	case *object.String:
		strLen := int64(len(in1.Value))
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
		strLen := int64(len(in1.Value))
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
}

var concatFn = func(args ...object.Object) object.Object {
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
}

var leftFn = func(args ...object.Object) object.Object {
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
}

var rightFn = func(args ...object.Object) object.Object {
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
}

var midFn = func(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("wrong number of arguments for MID. got=%d, want=3", len(args))
	}
	// IEC 61131-3: MID(IN, L, P) is L characters from position P.
	l, _, okL := object.GetIntegerObjectValue(args[1]) // L is the 2nd argument
	if !okL {
		return object.NewBuiltinError("argument 2 to `MID` must be INTEGER, got %s", args[1].Type())
	}
	p, _, okP := object.GetIntegerObjectValue(args[2]) // P is the 3rd argument
	if !okP {
		return object.NewBuiltinError("argument 3 to `MID` must be INTEGER, got %s", args[2].Type())
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
}

var findFn = func(args ...object.Object) object.Object {
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
}

var replaceFn = func(args ...object.Object) object.Object {
	if len(args) != 4 {
		return object.NewBuiltinError("wrong number of arguments for REPLACE. got=%d, want=4", len(args))
	}
	// IEC 61131-3: REPLACE(IN1, IN2, L, P) replaces L characters of IN1
	// from position P with IN2.
	l, _, okL := object.GetIntegerObjectValue(args[2]) // L is the 3rd argument
	if !okL {
		return object.NewBuiltinError("argument 3 to `REPLACE` must be INTEGER, got %s", args[2].Type())
	}
	p, _, okP := object.GetIntegerObjectValue(args[3]) // P is the 4th argument
	if !okP {
		return object.NewBuiltinError("argument 4 to `REPLACE` must be INTEGER, got %s", args[3].Type())
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
}
