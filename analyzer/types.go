/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package analyzer

// The elementary types, the implicit conversions between them, and the
// ranges of the integer types. Types are upper-case names; an integer or a
// real literal without a type is "ANY_INT" or "ANY_REAL" until the context
// gives it one.

import (
	"math"
	"strings"
)

const (
	anyInt  = "ANY_INT"
	anyReal = "ANY_REAL"
)

// family groups the elementary types.
type family int

const (
	famNone family = iota
	famSigned
	famUnsigned
	famBits
	famReal
	famBool
	famTime
	famDate
	famString
)

var families = map[string]family{
	"SINT": famSigned, "INT": famSigned, "DINT": famSigned, "LINT": famSigned,
	"USINT": famUnsigned, "UINT": famUnsigned, "UDINT": famUnsigned, "ULINT": famUnsigned,
	"BYTE": famBits, "WORD": famBits, "DWORD": famBits, "LWORD": famBits,
	"REAL": famReal, "LREAL": famReal,
	"BOOL": famBool,
	"TIME": famTime, "LTIME": famTime,
	"DATE": famDate, "TOD": famDate, "DT": famDate, "LDATE": famDate, "LTOD": famDate, "LDT": famDate,
	"STRING": famString, "WSTRING": famString, "CHAR": famString, "WCHAR": famString,
}

// normalize writes the long names of the date and time types short.
func normalize(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))
	switch t {
	case "TIME_OF_DAY":
		return "TOD"
	case "DATE_AND_TIME":
		return "DT"
	case "LTIME_OF_DAY":
		return "LTOD"
	case "LDATE_AND_TIME":
		return "LDT"
	}
	return t
}

func familyOf(t string) family { return families[t] }

func isElementary(t string) bool { return families[t] != famNone }

func isInteger(t string) bool {
	f := families[t]
	return f == famSigned || f == famUnsigned || t == anyInt
}

func isReal(t string) bool { return families[t] == famReal || t == anyReal }

// numeric is an integer, bit string or real type, or an untyped literal.
func numeric(t string) bool {
	f := families[t]
	return f == famSigned || f == famUnsigned || f == famBits || f == famReal || t == anyInt || t == anyReal
}

// implicitTo lists, for each elementary type, the types IEC 61131-3 Ed.3
// converts it to implicitly (its Figure 11): each conversion that keeps
// every value.
var implicitTo = map[string][]string{
	"SINT":  {"INT", "DINT", "LINT", "REAL", "LREAL"},
	"INT":   {"DINT", "LINT", "REAL", "LREAL"},
	"DINT":  {"LINT", "LREAL"},
	"USINT": {"UINT", "UDINT", "ULINT", "INT", "DINT", "LINT", "REAL", "LREAL"},
	"UINT":  {"UDINT", "ULINT", "DINT", "LINT", "REAL", "LREAL"},
	"UDINT": {"ULINT", "LINT", "LREAL"},
	"REAL":  {"LREAL"},
	"BOOL":  {"BYTE", "WORD", "DWORD", "LWORD"},
	"BYTE":  {"WORD", "DWORD", "LWORD"},
	"WORD":  {"DWORD", "LWORD"},
	"DWORD": {"LWORD"},
	"TIME":  {"LTIME"},
	"DATE":  {"LDATE"},
	"TOD":   {"LTOD"},
	"DT":    {"LDT"},
	"CHAR":  {"STRING"},
	"WCHAR": {"WSTRING"},
}

// implicit reports whether a value of type from converts to type to
// implicitly, without loss.
func implicit(from, to string) bool {
	if from == to {
		return true
	}
	for _, t := range implicitTo[from] {
		if t == to {
			return true
		}
	}
	return false
}

// wider returns the type of an arithmetic or bitwise operation on values of
// types a and b: the one the other converts to, or "" if neither does.
func wider(a, b string) string {
	switch {
	case a == "" || b == "":
		return ""
	case a == b:
		return a
	case a == anyInt && (numeric(b) && b != anyReal):
		return b
	case b == anyInt && (numeric(a) && a != anyReal):
		return a
	case a == anyReal || b == anyReal:
		other := a
		if a == anyReal {
			other = b
		}
		switch {
		case other == anyInt || other == anyReal:
			return anyReal
		case isReal(other):
			return other
		case implicit(other, "REAL"):
			return "REAL"
		case implicit(other, "LREAL"):
			return "LREAL"
		}
		return ""
	case implicit(a, b):
		return b
	case implicit(b, a):
		return a
	}
	return ""
}

// intRange is the range of an integer or bit string type.
func intRange(t string) (lo, hi float64, ok bool) {
	switch t {
	case "SINT":
		return math.MinInt8, math.MaxInt8, true
	case "INT":
		return math.MinInt16, math.MaxInt16, true
	case "DINT":
		return math.MinInt32, math.MaxInt32, true
	case "LINT":
		return math.MinInt64, math.MaxInt64, true
	case "USINT", "BYTE":
		return 0, math.MaxUint8, true
	case "UINT", "WORD":
		return 0, math.MaxUint16, true
	case "UDINT", "DWORD":
		return 0, math.MaxUint32, true
	case "ULINT", "LWORD":
		return 0, math.MaxUint64, true
	case "BOOL":
		return 0, 1, true
	}
	return 0, 0, false
}

// directType is the type of a direct address such as QX0.1 or IW4.
func directType(address string) string {
	a := strings.ToUpper(address)
	if len(a) < 2 {
		return ""
	}
	switch a[1] {
	case 'X':
		return "BOOL"
	case 'B':
		return "BYTE"
	case 'W':
		return "WORD"
	case 'D':
		return "DWORD"
	case 'L':
		return "LWORD"
	case '*':
		return ""
	}
	if a[1] >= '0' && a[1] <= '9' {
		return "BOOL" // %IX0.0 written %I0.0
	}
	return ""
}
