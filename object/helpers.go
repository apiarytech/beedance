/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// IsNumeric is a helper function that returns true if an object is one of the
// standard numeric types (integer or real).
func IsNumeric(obj Object) bool {
	t := obj.Type()
	return t == SINT_OBJ || t == INT_OBJ || t == DINT_OBJ || t == LINT_OBJ ||
		t == USINT_OBJ || t == UINT_OBJ || t == UDINT_OBJ || t == ULINT_OBJ ||
		t == REAL_OBJ || t == LREAL_OBJ
}

// GetIntegerObjectValue safely extracts an int64 value from any integer-like
// object, also returning whether the original type was unsigned.
func GetIntegerObjectValue(obj Object) (val int64, isUnsigned bool, success bool) {
	switch o := obj.(type) {
	case *SInt:
		return int64(o.Value), false, true
	case *Int:
		return int64(o.Value), false, true
	case *DInt:
		return int64(o.Value), false, true
	case *LInt:
		return o.Value, false, true
	case *USInt:
		return int64(o.Value), true, true
	case *UInt:
		return int64(o.Value), true, true
	case *UDInt:
		return int64(o.Value), true, true
	case *ULInt:
		// This can lose precision if the ULINT value is > MaxInt64,
		// but it's necessary for mixed-sign arithmetic.
		return int64(o.Value), true, true
	default:
		return 0, false, false
	}
}

// GetFloat64Value is a helper function that safely extracts a `float64` value
// from any numeric object type, performing the necessary type conversion.
func GetFloat64Value(obj Object) (float64, bool) {
	switch o := obj.(type) {
	case *SInt:
		return float64(o.Value), true
	case *Int:
		return float64(o.Value), true
	case *DInt:
		return float64(o.Value), true
	case *LInt:
		return float64(o.Value), true
	case *USInt:
		return float64(o.Value), true
	case *UInt:
		return float64(o.Value), true
	case *UDInt:
		return float64(o.Value), true
	case *ULInt:
		return float64(o.Value), true
	case *Real:
		return o.Value, true
	case *LReal:
		return o.Value, true
	default:
		return 0, false
	}
}

// IsIntegerType checks if a type name is a standard integer type.
func IsIntegerType(typeName string) bool {
	return typeName == "SINT" || typeName == "INT" || typeName == "DINT" || typeName == "LINT" ||
		typeName == "USINT" || typeName == "UINT" || typeName == "UDINT" || typeName == "ULINT"
}

// IsBooleanType checks if a type name is a standard boolean type.
func IsBooleanType(typeName string) bool {
	upper := strings.ToUpper(typeName)
	return upper == "BOOL" || upper == "BOOLEAN"
}

// IsRealType checks if a type name is a standard real type.
func IsRealType(typeName string) bool {
	upper := strings.ToUpper(typeName)
	return upper == "REAL" || upper == "LREAL"
}

// IsStringType checks if a type name is a standard string type.
func IsStringType(typeName string) bool {
	upper := strings.ToUpper(typeName)
	return upper == "STRING" || upper == "WSTRING"
}

// IsBitStringType checks if a type name is a standard bit-string type.
func IsBitStringType(typeName string) bool {
	_, ok := GetBitStringWidth(typeName)
	return ok
}

// GetBitStringWidth returns the width in bits for a standard bit-string type name.
func GetBitStringWidth(typeName string) (int, bool) {
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

// IsComparisonOperator checks if an operator is a comparison operator.
func IsComparisonOperator(op string) bool {
	switch op {
	case "=", "EQ", "!=", "<>", "NE", "<", "LT", ">", "GT", "<=", "LE", ">=", "GE":
		return true
	default:
		return false
	}
}

// IsBitwiseOperator checks if an operator is a bitwise operator.
func IsBitwiseOperator(op string) bool {
	switch op {
	case "AND", "&", "OR", "XOR", "NAND", "NOR":
		return true
	default:
		return false
	}
}

// IsTimeDateKeyword checks if a given string is the name of a standard IEC 61131-3 time or date type.
func IsTimeDateKeyword(name string) bool {
	upper := strings.ToUpper(name)
	switch upper {
	case "TIME", "T", "DATE", "D", "TIME_OF_DAY", "TOD", "DATE_AND_TIME", "DT":
		return true
	}
	return false
}

// NewBuiltinError creates a new Error object specifically for errors originating from built-in functions.
func NewBuiltinError(format string, a ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf("BUILTIN ERROR: %s", fmt.Sprintf(format, a...)),
	}
}

// TRUE is a singleton object representing the boolean true value.
var TRUE = &Boolean{Value: true}

// FALSE is a singleton object representing the boolean false value.
var FALSE = &Boolean{Value: false}

// nativeBoolToBooleanObject returns one of the singleton TRUE or FALSE objects.
func nativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
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

// GenericConversionBuiltin is a factory for creating `*_TO_*` conversion functions.
func GenericConversionBuiltin(fromType, toType string) *Builtin {
	return &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return NewBuiltinError("wrong number of arguments for %s_TO_%s. got=%d, want=1", fromType, toType, len(args))
			}
			return ApplyConversion(args[0], fromType, toType)
		},
	}
}

// ApplyConversion handles the logic for converting an object from one type to another.
func ApplyConversion(input Object, fromType, toType string) Object {
	actualType := string(input.Type())
	isValidFromType := false
	if (IsIntegerType(fromType) && IsIntegerType(actualType)) ||
		(IsBooleanType(fromType) && IsBooleanType(actualType)) ||
		(IsRealType(fromType) && IsRealType(actualType)) ||
		(IsStringType(fromType) && IsStringType(actualType)) ||
		(IsBitStringType(fromType) && IsBitStringType(actualType)) ||
		(IsBitStringType(fromType) && isBitStringOfWidth(input, fromType)) ||
		(fromType == "ANY_INT" && IsIntegerType(actualType)) ||
		(fromType == "ANY_REAL" && IsNumeric(input)) ||
		(fromType == "BCD" && actualType == string(BITSTRING_OBJ)) ||
		(actualType == fromType) {
		isValidFromType = true
	}
	if !isValidFromType {
		return NewBuiltinError("type mismatch for %s_TO_%s: input is %s, expected a %s type", fromType, toType, actualType, fromType)
	}
	if fromType == "BCD" {
		if IsIntegerType(toType) {
			return bcdToInt(input)
		}
	}
	if IsIntegerType(toType) {
		switch val := input.(type) {
		case *LInt, *SInt, *Int, *DInt, *USInt, *UInt, *UDInt, *ULInt:
			iVal, isUnsigned, _ := GetIntegerObjectValue(val)
			targetRange, ok := integerTypeRanges[toType]
			if !ok {
				return NewBuiltinError("internal error: unknown integer type %s", toType)
			}
			if strings.HasPrefix(toType, "U") {
				if iVal < 0 || uint64(iVal) > targetRange.maxUnsigned {
					return NewBuiltinError("value %d is out of range for type %s (0 to %d)", iVal, toType, targetRange.maxUnsigned)
				}
			} else {
				if iVal < targetRange.minSigned || iVal > targetRange.maxSigned {
					return NewBuiltinError("value %d is out of range for type %s (%d to %d)", iVal, toType, targetRange.minSigned, targetRange.maxSigned)
				}
			}
			return checkAndCreateIntegerObject(ObjectType(toType), iVal, uint64(iVal), isUnsigned)
		case *Real:
			rounded := int64(math.Round(val.Value))
			return checkAndCreateIntegerObject(ObjectType(toType), rounded, uint64(rounded), false)
		case *LReal:
			rounded := int64(math.Round(val.Value))
			return checkAndCreateIntegerObject(ObjectType(toType), rounded, uint64(rounded), false)
		case *BitString:
			return checkAndCreateIntegerObject(ObjectType(toType), int64(val.Value), val.Value, true)
		case *String:
			// Trim whitespace before parsing, as per IEC standard for STRING_TO_*
			i, err := strconv.ParseInt(strings.TrimSpace(val.Value), 10, 64)
			if err != nil {
				return NewBuiltinError("could not parse string to integer: %s", val.Value)
			}
			return &LInt{Value: i}
		default:
			return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
		}
	}
	if IsRealType(toType) {
		val, ok := GetFloat64Value(input)
		if ok {
			return &Real{Value: val}
		}
		switch val := input.(type) {
		case *Real:
			return &Real{Value: val.Value}
		case *String:
			f, err := strconv.ParseFloat(val.Value, 64)
			if err != nil {
				return NewBuiltinError("could not parse string to real: %s", val.Value)
			}
			return &Real{Value: f}
		default:
			return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
		}
	}
	if IsStringType(toType) {
		switch val := input.(type) {
		case *Boolean:
			if val.Value {
				return &String{Value: "TRUE"}
			}
			return &String{Value: "FALSE"}
		case *Time:
			return &String{Value: val.Value.String()}
		case *Real, *LReal:
			floatVal, _ := GetFloat64Value(val)
			return &String{Value: fmt.Sprintf("%f", floatVal)}
		default:
			return &String{Value: input.Inspect()}
		}
	}
	if IsBooleanType(toType) {
		switch input.(type) {
		case *Boolean:
			return input // It's already a boolean, no conversion needed.
		case *LInt, *SInt, *Int, *DInt, *USInt, *UInt, *UDInt, *ULInt:
			iVal, _, _ := GetIntegerObjectValue(input)
			return nativeBoolToBooleanObject(iVal != 0)
		case *Real, *LReal:
			fVal, _ := GetFloat64Value(input)
			return nativeBoolToBooleanObject(fVal != 0.0)
		case *BitString:
			return nativeBoolToBooleanObject(input.(*BitString).Value != 0)
		}
		return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
	}
	if IsBitStringType(toType) {
		maxVal, ok := bitStringTypeRanges[toType]
		if !ok {
			return NewBuiltinError("internal error: unknown bitstring type %s", toType)
		}
		width, _ := GetBitStringWidth(toType)
		switch val := input.(type) {
		case *LInt, *SInt, *Int, *DInt, *USInt, *UInt, *UDInt, *ULInt:
			iVal, _, _ := GetIntegerObjectValue(val)
			if iVal < 0 || uint64(iVal) > maxVal {
				return NewBuiltinError("value %d is out of range for type %s (0 to %d)", iVal, toType, maxVal)
			}
			return &BitString{Value: uint64(iVal), Width: width}
		case *BitString:
			if val.Value > maxVal {
				return NewBuiltinError("value %d is out of range for type %s (0 to %d)", val.Value, toType, maxVal)
			}
			return &BitString{Value: val.Value, Width: width}
		default:
			return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), toType)
		}
	}
	if toType == "BCD" {
		switch val := input.(type) {
		case *LInt, *SInt, *Int, *DInt:
			iVal, _, _ := GetIntegerObjectValue(val)
			bcd, err := intToBcd(iVal)
			if err != nil {
				return NewBuiltinError("%s", err.Error())
			}
			return &BitString{Value: uint64(bcd), Width: 16}
		default:
			return NewBuiltinError("conversion from %s to BCD is not supported", input.Type())
		}
	}
	return NewBuiltinError("conversion to type %s is not supported", toType)
}

func intToBcd(val int64) (uint16, error) {
	if val < 0 || val > 9999 {
		return 0, fmt.Errorf("value %d out of range for 4-digit BCD conversion (0-9999)", val)
	}
	var bcd uint16
	shift := uint(0)
	if val == 0 {
		return 0, nil
	}
	tempVal := val
	for tempVal > 0 {
		digit := tempVal % 10
		bcd |= uint16(digit) << shift
		tempVal /= 10
		shift += 4
	}
	return bcd, nil
}

func bcdToInt(input Object) Object {
	bs, ok := input.(*BitString)
	if !ok || bs.Width != 16 {
		return NewBuiltinError("argument for BCD_TO_INT must be a WORD (16-bit BitString), got %s", input.Type())
	}
	bcdVal := uint16(bs.Value)
	var result int64
	for i := 3; i >= 0; i-- {
		nibble := (bcdVal >> (i * 4)) & 0xF
		if nibble > 9 {
			return NewBuiltinError("invalid BCD format: nibble %d has value %d > 9", i, nibble)
		}
		result = result*10 + int64(nibble)
	}
	return &LInt{Value: result}
}

// checkAndCreateIntegerObject validates a computed integer value against the
// bounds of a target IEC integer type and, if valid, creates and returns the
// corresponding object (e.g., SInt, UINT).
func checkAndCreateIntegerObject(t ObjectType, val int64, uval uint64, isUnsigned bool) Object {
	targetRange, ok := integerTypeRanges[string(t)]
	if !ok {
		return NewBuiltinError("internal error: unknown integer type %s", t)
	}

	if strings.HasPrefix(string(t), "U") { // Unsigned target
		var checkVal uint64
		if isUnsigned {
			checkVal = uval
		} else {
			if val < 0 {
				return NewBuiltinError("%s underflow: %d", t, val)
			}
			checkVal = uint64(val)
		}
		if checkVal > targetRange.maxUnsigned {
			return NewBuiltinError("%s overflow: %d", t, checkVal)
		}
	} else { // Signed target
		checkVal := val
		if isUnsigned {
			if uval > uint64(targetRange.maxSigned) {
				return NewBuiltinError("%s overflow: %d", t, uval)
			}
			checkVal = int64(uval)
		}
		if checkVal < targetRange.minSigned {
			return NewBuiltinError("%s underflow: %d", t, checkVal)
		}
		if checkVal > targetRange.maxSigned {
			return NewBuiltinError("%s overflow: %d", t, checkVal)
		}
	}

	// If checks pass, create the object from whichever value was given; the
	// checks above guarantee it fits the target type.
	if isUnsigned {
		val = int64(uval)
	} else {
		uval = uint64(val)
	}
	switch t {
	case SINT_OBJ:
		return &SInt{Value: int8(val)}
	case INT_OBJ:
		return &Int{Value: int16(val)}
	case DINT_OBJ:
		return &DInt{Value: int32(val)}
	case LINT_OBJ:
		return &LInt{Value: val}
	case USINT_OBJ:
		return &USInt{Value: uint8(uval)}
	case UINT_OBJ:
		return &UInt{Value: uint16(uval)}
	case UDINT_OBJ:
		return &UDInt{Value: uint32(uval)}
	case ULINT_OBJ:
		return &ULInt{Value: uval}
	}
	return &LInt{Value: val}
}

// IsEqual performs a deep equality check between two objects, respecting IEC 61131-3 type semantics.
func IsEqual(left, right Object) bool {
	// evalInfixForIsEqual is a simplified, self-contained version of the evaluator's logic.
	result := EvalInfix(left, "=", right)
	// Only a TRUE object means they are equal. An error or FALSE means they are not.
	return result == TRUE
}

// EvalInfix is a helper that performs infix operations.
func EvalInfix(left Object, operator string, right Object) Object {
	switch {
	case IsNumeric(left) && IsNumeric(right):
		return EvalNumericInfix(left, right, operator)
	case left.Type() == BOOLEAN_OBJ && right.Type() == BOOLEAN_OBJ:
		leftVal := left.(*Boolean).Value
		rightVal := right.(*Boolean).Value
		switch operator {
		case "AND", "&":
			return nativeBoolToBooleanObject(leftVal && rightVal)
		case "OR":
			return nativeBoolToBooleanObject(leftVal || rightVal)
		case "XOR":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		case "NAND":
			return nativeBoolToBooleanObject(!(leftVal && rightVal))
		case "NOR":
			return nativeBoolToBooleanObject(!(leftVal || rightVal))
		case "=", "EQ":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "!=", "<>", "NE":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		case "<", "LT":
			return nativeBoolToBooleanObject(!leftVal && rightVal) // FALSE < TRUE
		case ">", "GT":
			return nativeBoolToBooleanObject(leftVal && !rightVal) // TRUE > FALSE
		case "<=", "LE":
			return nativeBoolToBooleanObject(leftVal == rightVal || !leftVal)
		case ">=", "GE":
			return nativeBoolToBooleanObject(leftVal == rightVal || leftVal)
		default:
			return NewBuiltinError("unknown operator for BOOLEAN: %s", operator)
		}
	case left.Type() == STRING_OBJ && right.Type() == STRING_OBJ:
		return evalStringInfix(left.(*String), operator, right.(*String))
	case left.Type() == WSTRING_OBJ && right.Type() == WSTRING_OBJ:
		return evalWStringInfix(left.(*WString), operator, right.(*WString))
	// Time and Date arithmetic and comparison (order is important: specific to general)
	case left.Type() == DATE_AND_TIME_OBJ || right.Type() == DATE_AND_TIME_OBJ:
		return evalDateAndTimeInfix(left, operator, right)
	case left.Type() == TIME_OF_DAY_OBJ || right.Type() == TIME_OF_DAY_OBJ:
		return evalTimeOfDayInfix(left, operator, right)
	case left.Type() == DATE_OBJ || right.Type() == DATE_OBJ:
		return evalDateInfix(left, operator, right)
	case left.Type() == TIME_OBJ || right.Type() == TIME_OBJ:
		return evalTimeInfix(left, operator, right)
	case left.Type() == BITSTRING_OBJ && right.Type() == BITSTRING_OBJ:
		return evalBitStringInfix(left, operator, right)
	case left.Type() == ENUMERATED_VALUE_OBJ && right.Type() == ENUMERATED_VALUE_OBJ:
		// Enumerated values support equality only. They are equal when they
		// name the same value of the same type; names are case-insensitive.
		l, r := left.(*EnumeratedValue), right.(*EnumeratedValue)
		same := strings.EqualFold(l.TypeName, r.TypeName) && strings.EqualFold(l.Value, r.Value)
		switch operator {
		case "=", "EQ":
			return nativeBoolToBooleanObject(same)
		case "<>", "!=", "NE":
			return nativeBoolToBooleanObject(!same)
		}
		return NewBuiltinError("operator '%s' is not defined for enumerated values", operator)
	case left.Type() == NULL_OBJ || right.Type() == NULL_OBJ:
		if operator == "=" {
			return nativeBoolToBooleanObject(left.Type() == right.Type())
		}
	}
	if IsBitwiseOperator(operator) && (left.Type() == BITSTRING_OBJ || right.Type() == BITSTRING_OBJ) {
		return NewBuiltinError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	}
	if IsComparisonOperator(operator) {
		return NewBuiltinError("type mismatch for comparison: %s %s %s", left.Type(), operator, right.Type())
	}
	return NewBuiltinError("unsupported operator '%s' for types %s and %s", operator, left.Type(), right.Type())
}

func evalStringInfix(left *String, operator string, right *String) Object {
	switch operator {
	case "+":
		return &String{Value: left.Value + right.Value}
	case "=", "EQ":
		return nativeBoolToBooleanObject(left.Value == right.Value)
	case "!=", "<>", "NE":
		return nativeBoolToBooleanObject(left.Value != right.Value)
	case "<", "LT":
		return nativeBoolToBooleanObject(left.Value < right.Value)
	case ">", "GT":
		return nativeBoolToBooleanObject(left.Value > right.Value)
	case "<=", "LE":
		return nativeBoolToBooleanObject(left.Value <= right.Value)
	case ">=", "GE":
		return nativeBoolToBooleanObject(left.Value >= right.Value)
	default:
		return NewBuiltinError("unsupported operator '%s' for types STRING and STRING", operator)
	}
}

func evalWStringInfix(left *WString, operator string, right *WString) Object {
	switch operator {
	case "+":
		return &WString{Value: left.Value + right.Value}
	case "=", "EQ":
		return nativeBoolToBooleanObject(left.Value == right.Value)
	case "!=", "<>", "NE":
		return nativeBoolToBooleanObject(left.Value != right.Value)
	case "<", "LT":
		return nativeBoolToBooleanObject(left.Value < right.Value)
	case ">", "GT":
		return nativeBoolToBooleanObject(left.Value > right.Value)
	case "<=", "LE":
		return nativeBoolToBooleanObject(left.Value <= right.Value)
	case ">=", "GE":
		return nativeBoolToBooleanObject(left.Value >= right.Value)
	default:
		return NewBuiltinError("unsupported operator '%s' for types WSTRING and WSTRING", operator)
	}
}

func evalBitStringInfix(left Object, operator string, right Object) Object {
	leftBitString := left.(*BitString)
	rightBitString := right.(*BitString)

	if leftBitString.Width != rightBitString.Width {
		return NewBuiltinError("type mismatch: bitstring operands must have same width, got %d and %d", leftBitString.Width, rightBitString.Width)
	}

	leftVal := leftBitString.Value
	rightVal := rightBitString.Value
	width := leftBitString.Width

	switch operator {
	case "AND", "&":
		return &BitString{Value: leftVal & rightVal, Width: width}
	case "OR":
		return &BitString{Value: leftVal | rightVal, Width: width}
	case "XOR":
		return &BitString{Value: leftVal ^ rightVal, Width: width}
	case "NAND":
		var mask uint64 = math.MaxUint64
		if width < 64 {
			mask = (1 << width) - 1
		}
		return &BitString{Value: ^(leftVal & rightVal) & mask, Width: width}
	case "NOR":
		var mask uint64 = math.MaxUint64
		if width < 64 {
			mask = (1 << width) - 1
		}
		return &BitString{Value: ^(leftVal | rightVal) & mask, Width: width}
	case "=", "EQ":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=", "<>", "NE":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=", "LE":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=", "GE":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return NewBuiltinError("unknown operator for bitstrings: %s", operator)
	}
}

func evalTimeInfix(left Object, operator string, right Object) Object {
	if left.Type() == TIME_OBJ && right.Type() == TIME_OBJ {
		lVal := left.(*Time).Value
		rVal := right.(*Time).Value
		switch operator {
		case "+":
			return &Time{Value: lVal + rVal}
		case "-":
			return &Time{Value: lVal - rVal}
		case "*", "/":
			return NewBuiltinError("unsupported operator '%s' for types %s and %s", operator, left.Type(), right.Type())
		default: // Comparison operators
			return evalGenericComparison(operator, int64(lVal), int64(rVal))
		}
	}
	if left.Type() == TIME_OBJ && IsNumeric(right) {
		lVal := left.(*Time).Value
		rVal, _ := GetFloat64Value(right)
		switch operator {
		case "*":
			return &Time{Value: time.Duration(float64(lVal) * rVal)}
		case "/":
			if rVal == 0 {
				return NewBuiltinError("division by zero")
			}
			return &Time{Value: time.Duration(float64(lVal) / rVal)}
		}
	}
	if IsNumeric(left) && right.Type() == TIME_OBJ {
		lVal, _ := GetFloat64Value(left)
		rVal := right.(*Time).Value
		if operator == "*" {
			return &Time{Value: time.Duration(lVal * float64(rVal))}
		}
	}
	return NewBuiltinError("unsupported operator '%s' for types %s and %s", operator, left.Type(), right.Type())
}

func evalDateInfix(left Object, operator string, right Object) Object {
	if left.Type() == DATE_OBJ && right.Type() == DATE_OBJ {
		lVal := left.(*Date).Value
		rVal := right.(*Date).Value
		if operator == "-" {
			return &Time{Value: lVal.Sub(rVal)}
		}
		return evalGenericComparison(operator, lVal.UnixNano(), rVal.UnixNano())
	}
	return NewBuiltinError("unsupported operator '%s' for types %s and %s", operator, left.Type(), right.Type())
}

func evalTimeOfDayInfix(left Object, operator string, right Object) Object {
	if left.Type() == TIME_OF_DAY_OBJ {
		lVal := left.(*TimeOfDay).Value
		if right.Type() == TIME_OBJ {
			rVal := right.(*Time).Value
			if operator == "+" {
				return &TimeOfDay{Value: lVal.Add(rVal)}
			} else if operator == "-" {
				return &TimeOfDay{Value: lVal.Add(-rVal)}
			}
		} else if right.Type() == TIME_OF_DAY_OBJ {
			rVal := right.(*TimeOfDay).Value
			if operator == "-" {
				return &Time{Value: lVal.Sub(rVal)}
			}
			leftNs := int64(lVal.Hour())*int64(time.Hour) + int64(lVal.Minute())*int64(time.Minute) + int64(lVal.Second())*int64(time.Second) + int64(lVal.Nanosecond())
			rightNs := int64(rVal.Hour())*int64(time.Hour) + int64(rVal.Minute())*int64(time.Minute) + int64(rVal.Second())*int64(time.Second) + int64(rVal.Nanosecond())
			return evalGenericComparison(operator, leftNs, rightNs)
		}
	}
	return NewBuiltinError("unsupported operator '%s' for types %s and %s", operator, left.Type(), right.Type())
}

func evalDateAndTimeInfix(left Object, operator string, right Object) Object {
	if left.Type() == DATE_AND_TIME_OBJ {
		lVal := left.(*DateAndTime).Value
		if right.Type() == TIME_OBJ {
			rVal := right.(*Time).Value
			if operator == "+" {
				return &DateAndTime{Value: lVal.Add(rVal)}
			} else if operator == "-" {
				return &DateAndTime{Value: lVal.Add(-rVal)}
			}
		} else if right.Type() == DATE_AND_TIME_OBJ {
			rVal := right.(*DateAndTime).Value
			if operator == "-" {
				return &Time{Value: lVal.Sub(rVal)}
			}
			return evalGenericComparison(operator, lVal.UnixNano(), rVal.UnixNano())
		}
	}
	return NewBuiltinError("unsupported operator '%s' for types %s and %s", operator, left.Type(), right.Type())
}

func evalGenericComparison[T ~string | ~int64](op string, leftVal, rightVal T) Object {
	switch op {
	case "=", "EQ":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=", "<>", "NE":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<", "LT":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">", "GT":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=", "LE":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=", "GE":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return NewBuiltinError("unknown operator '%s' for generic comparison", op)
	}
}

func EvalNumericInfix(left, right Object, operator string) Object {
	if left.Type() == LREAL_OBJ || right.Type() == LREAL_OBJ {
		leftVal, _ := GetFloat64Value(left)
		rightVal, _ := GetFloat64Value(right)
		return EvalFloatInfix(leftVal, operator, rightVal, true)
	}
	if left.Type() == REAL_OBJ || right.Type() == REAL_OBJ {
		leftVal, _ := GetFloat64Value(left)
		rightVal, _ := GetFloat64Value(right)
		return EvalFloatInfix(leftVal, operator, rightVal, false)
	}
	return EvalIntegerInfix(left, operator, right)
}

func EvalFloatInfix(leftVal float64, operator string, rightVal float64, isLReal bool) Object {
	var result Object
	switch operator {
	case "+":
		result = &Real{Value: leftVal + rightVal}
	case "-":
		result = &Real{Value: leftVal - rightVal}
	case "*":
		result = &Real{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0.0 {
			return NewBuiltinError("division by zero")
		}
		result = &Real{Value: leftVal / rightVal}
	case "<", "LT":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">", "GT":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "=", "EQ":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=", "<>", "NE":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=", "LE":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=", "GE":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return NewBuiltinError("unknown operator for REAL/LREAL: %s", operator)
	}
	if isLReal {
		return &LReal{Value: result.(*Real).Value}
	}
	return result
}

func EvalIntegerInfix(left Object, operator string, right Object) Object {
	leftType := left.Type()
	rightType := right.Type()
	resultType := getResultIntegerType(leftType, rightType)

	leftVal, isLeftUnsigned, ok := GetIntegerObjectValue(left)
	if !ok {
		return NewBuiltinError("could not get value from left operand of type %s", left.Type())
	}
	rightVal, isRightUnsigned, ok := GetIntegerObjectValue(right)
	if !ok {
		return NewBuiltinError("could not get value from right operand of type %s", right.Type())
	}

	var resultValue int64
	var uResultValue uint64
	resultIsUnsigned := isLeftUnsigned && isRightUnsigned

	if resultIsUnsigned {
		uLeft, uRight := uint64(leftVal), uint64(rightVal)
		switch operator {
		case "+":
			if math.MaxUint64-uLeft < uRight {
				return NewBuiltinError("unsigned integer overflow")
			}
			uResultValue = uLeft + uRight
		case "-":
			if uLeft < uRight {
				return NewBuiltinError("unsigned integer underflow")
			}
			uResultValue = uLeft - uRight
		case "*":
			if uRight > 0 && uLeft > math.MaxUint64/uRight {
				return NewBuiltinError("unsigned integer overflow")
			}
			uResultValue = uLeft * uRight
		case "/":
			if uRight == 0 {
				return NewBuiltinError("division by zero")
			}
			uResultValue = uLeft / uRight
		case "MOD":
			if uRight == 0 {
				return NewBuiltinError("division by zero in MOD")
			}
			uResultValue = uLeft % uRight
		case "<", "LT":
			return nativeBoolToBooleanObject(uLeft < uRight)
		case ">", "GT":
			return nativeBoolToBooleanObject(uLeft > uRight)
		case "=", "EQ":
			return nativeBoolToBooleanObject(uLeft == uRight)
		case "!=", "<>", "NE":
			return nativeBoolToBooleanObject(uLeft != uRight)
		case "<=", "LE":
			return nativeBoolToBooleanObject(uLeft <= uRight)
		case ">=", "GE":
			return nativeBoolToBooleanObject(uLeft >= uRight)
		default:
			return NewBuiltinError("unknown operator for unsigned integers: %s", operator)
		}
	} else {
		// Signed arithmetic
		switch operator {
		case "+":
			if (rightVal > 0 && leftVal > math.MaxInt64-rightVal) || (rightVal < 0 && leftVal < math.MinInt64-rightVal) {
				return NewBuiltinError("signed integer overflow")
			}
			resultValue = leftVal + rightVal
		case "-":
			if (rightVal > 0 && leftVal < math.MinInt64+rightVal) || (rightVal < 0 && leftVal > math.MaxInt64+rightVal) {
				return NewBuiltinError("signed integer underflow")
			}
			resultValue = leftVal - rightVal
		case "*":
			if rightVal != 0 && ((leftVal > math.MaxInt64/rightVal && rightVal > 0) || (leftVal < math.MinInt64/rightVal && rightVal > 0) || (leftVal > math.MinInt64/rightVal && rightVal < -1) || (leftVal < math.MaxInt64/rightVal && rightVal < -1)) {
				return NewBuiltinError("signed integer overflow")
			}
			resultValue = leftVal * rightVal
		case "/":
			if rightVal == 0 {
				return NewBuiltinError("division by zero")
			}
			if leftVal == math.MinInt64 && rightVal == -1 {
				return NewBuiltinError("signed integer overflow")
			}
			resultValue = leftVal / rightVal
		case "MOD":
			if rightVal == 0 {
				return NewBuiltinError("division by zero in MOD")
			}
			resultValue = leftVal % rightVal
		case "<", "LT", ">", "GT", "=", "EQ", "!=", "<>", "NE", "<=", "LE", ">=", "GE":
			return evalGenericComparison(operator, leftVal, rightVal)
		default:
			return NewBuiltinError("unknown operator for signed integers: %s", operator)
		}
	}

	return checkAndCreateIntegerObject(resultType, resultValue, uResultValue, resultIsUnsigned)
}

func getResultIntegerType(t1, t2 ObjectType) ObjectType {
	if t1 == t2 {
		return t1
	}
	typeInfo := map[ObjectType]struct {
		rank     int
		isSigned bool
	}{
		SINT_OBJ:  {1, true},
		USINT_OBJ: {1, false},
		INT_OBJ:   {2, true},
		UINT_OBJ:  {2, false},
		DINT_OBJ:  {3, true},
		UDINT_OBJ: {3, false},
		LINT_OBJ:  {4, true},
		ULINT_OBJ: {4, false},
	}
	info1, ok1 := typeInfo[t1]
	info2, ok2 := typeInfo[t2]
	if !ok1 {
		return t2
	}
	if !ok2 {
		return t1
	}
	if info1.rank == info2.rank && info1.isSigned != info2.isSigned {
		switch info1.rank {
		case 1:
			return INT_OBJ
		case 2:
			return DINT_OBJ
		case 3:
			return LINT_OBJ
		case 4:
			return LINT_OBJ
		}
	}
	if info1.rank > info2.rank {
		return t1
	}
	return t2
}

// IsTruthy determines if an object is considered true in a boolean context.
// Following IEC 61131-3 logic, only a `Boolean` with a value of `true` is
// considered truthy. All other objects, including `NULL`, are considered false.
func IsTruthy(obj Object) bool {
	switch o := obj.(type) {
	case *Boolean:
		return o.Value
	case *Null:
		return false
	default:
		// Any non-boolean result is implicitly not "truthy".
		return false
	}
}

// isBitStringOfWidth reports whether obj is a bit string that fits the
// bit-string type typeName (BYTE, WORD, DWORD or LWORD).
func isBitStringOfWidth(obj Object, typeName string) bool {
	bs, ok := obj.(*BitString)
	width, known := GetBitStringWidth(typeName)
	return ok && known && bs.Width <= width
}

// SetLowerBounds sets the lower bound of each dimension of an array: the
// first on the array itself and the rest on the arrays it contains. A value
// that is not an array is left alone.
func SetLowerBounds(value Object, bounds []int64) {
	array, ok := value.(*Array)
	if !ok || len(bounds) == 0 {
		return
	}
	array.LowerBound = bounds[0]
	for _, element := range array.Elements {
		SetLowerBounds(element, bounds[1:])
	}
}

// LowerBounds returns the lower bound of each dimension of an array, reading
// nested dimensions from the first element; it is nil for a non-array.
func LowerBounds(value Object) []int64 {
	bounds := []int64{}
	for {
		array, ok := value.(*Array)
		if !ok {
			return bounds
		}
		bounds = append(bounds, array.LowerBound)
		if len(array.Elements) == 0 {
			return bounds
		}
		value = array.Elements[0]
	}
}

// CopyValue returns a copy of an array or structure, as IEC 61131-3 assigns
// and passes them by value: changing the copy leaves the original alone.
// Nested arrays and structures are copied too, and arrays keep their lower
// bounds. A function block instance (a hash with a "__class__" entry) and
// any other value are returned as they are.
func CopyValue(value Object) Object {
	switch v := value.(type) {
	case *Array:
		elements := make([]Object, len(v.Elements))
		for i, element := range v.Elements {
			elements[i] = CopyValue(element)
		}
		return &Array{Elements: elements, LowerBound: v.LowerBound}
	case *Hash:
		if _, isInstance := v.Pairs[(&String{Value: "__class__"}).HashKey()]; isInstance {
			return v
		}
		pairs := make(map[HashKey]HashPair, len(v.Pairs))
		for key, pair := range v.Pairs {
			pairs[key] = HashPair{Key: pair.Key, Value: CopyValue(pair.Value)}
		}
		return &Hash{Pairs: pairs}
	}
	return value
}
