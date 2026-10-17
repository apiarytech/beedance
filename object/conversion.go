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
	"strings"
	"sync"
)

// ConvertBuiltinName is the built-in every type conversion X_TO_Y compiles
// to: $CONVERT(value, 'X', 'Y'). One generic built-in stands in for the 552
// X_TO_Y pairs, which a microcontroller's heap could not hold registered
// one by one. '$' cannot start an identifier, so no program can name it.
const ConvertBuiltinName = "$CONVERT"

// isConversionType reports whether t is a type an X_TO_Y conversion names.
func isConversionType(t string) bool {
	switch t {
	case "BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT",
		"REAL", "LREAL", "TIME", "DATE", "TOD", "DT", "STRING", "WSTRING", "CHAR", "WCHAR",
		"BYTE", "WORD", "DWORD", "LWORD", "ANY_INT", "ANY_REAL", "BCD":
		return true
	}
	return false
}

// ParseConversion reports whether name, in any case, is a standard type
// conversion X_TO_Y between two different types, and which.
func ParseConversion(name string) (from, to string, ok bool) {
	upper := strings.ToUpper(name)
	// The generic conversions (IEC 61131-3 3rd edition): TO_INT(x) converts
	// from whatever type x has (from is "ANY"), TRUNC_INT(x) truncates a
	// real toward zero (from is "TRUNC").
	for prefix, generic := range map[string]string{"TO_": "ANY", "TRUNC_": "TRUNC"} {
		if rest, found := strings.CutPrefix(upper, prefix); found && isConversionType(rest) && rest != "BCD" {
			if generic == "TRUNC" && !IsIntegerType(rest) {
				return "", "", false
			}
			return generic, rest, true
		}
	}
	i := strings.Index(upper, "_TO_")
	if i <= 0 {
		return "", "", false
	}
	from, to = upper[:i], upper[i+len("_TO_"):]
	// The long time types are held as the short ones: LTIME_TO_STRING is
	// TIME_TO_STRING, and LTIME_TO_TIME keeps the value as it is.
	long := false
	if short, ok := longTimeConversionTypes[from]; ok {
		from, long = short, true
	}
	if short, ok := longTimeConversionTypes[to]; ok {
		to, long = short, true
	}
	if (from == to && !long) || !isConversionType(from) || !isConversionType(to) {
		return "", "", false
	}
	return from, to, true
}

// ConvertBuiltin is $CONVERT: it converts its first argument from the type
// its second names to the type its third names, as X_TO_Y does.
func ConvertBuiltin(args ...Object) Object {
	if len(args) < 2 {
		return NewBuiltinError("wrong number of arguments for a type conversion. got=%d, want=1", max(len(args)-2, 0))
	}
	from, okFrom := args[len(args)-2].(*String)
	to, okTo := args[len(args)-1].(*String)
	if !okFrom || !okTo {
		return NewBuiltinError("%s: the types must be strings", ConvertBuiltinName)
	}
	if len(args) != 3 {
		return NewBuiltinError("wrong number of arguments for %s_TO_%s. got=%d, want=1", from.Value, to.Value, len(args)-2)
	}
	return ApplyConversion(args[0], from.Value, to.Value)
}

// conversions caches the X_TO_Y built-ins GetBuiltinByName has made, so
// they are made only for the names a program uses.
var conversions struct {
	sync.Mutex
	byName map[string]*Builtin
}

// conversionBuiltin is the built-in X_TO_Y for a conversion name.
func conversionBuiltin(name string) (*Builtin, bool) {
	from, to, ok := ParseConversion(name)
	if !ok {
		return nil, false
	}
	key := from + "_TO_" + to
	conversions.Lock()
	defer conversions.Unlock()
	if b, ok := conversions.byName[key]; ok {
		return b, true
	}
	if conversions.byName == nil {
		conversions.byName = map[string]*Builtin{}
	}
	b := GenericConversionBuiltin(from, to)
	conversions.byName[key] = b
	return b, true
}

// charConversion converts to or from CHAR and WCHAR, held as a STRING or a
// WSTRING of one character (empty for the NUL character): to a character
// from a number (its code) or a string (its first character), from a
// character to a number (its code) or a string; it reports whether the
// conversion is one of those.
func charConversion(input Object, from, to string) (Object, bool) {
	if from != "CHAR" && from != "WCHAR" && to != "CHAR" && to != "WCHAR" {
		return nil, false
	}
	// The character code of the input: a number, or the first character
	// of a string.
	code := func() (int64, bool) {
		switch v := input.(type) {
		case *String:
			if v.Value == "" {
				return 0, true
			}
			if from == "STRING" || from == "CHAR" {
				return int64(v.Value[0]), true
			}
			return int64([]rune(v.Value)[0]), true
		case *WString:
			if v.Value == "" {
				return 0, true
			}
			return int64([]rune(v.Value)[0]), true
		case *BitString:
			return int64(v.Value), true
		}
		n, _, ok := GetIntegerObjectValue(input)
		return n, ok
	}
	n, ok := code()
	if !ok {
		return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), to), true
	}
	switch {
	case to == "CHAR" || (to == "STRING" && from == "CHAR"):
		if n == 0 {
			return &String{Value: ""}, true
		}
		if s, isString := input.(*String); isString && to == "STRING" {
			return &String{Value: s.Value}, true
		}
		return &String{Value: string([]byte{byte(n)})}, true
	case to == "WCHAR" || to == "WSTRING":
		if n == 0 {
			return &WString{Value: ""}, true
		}
		return &WString{Value: string(rune(n))}, true
	case to == "STRING": // from WCHAR
		if n == 0 {
			return &String{Value: ""}, true
		}
		return &String{Value: string([]byte{byte(n)})}, true
	}
	// From a character to a number or a bit string: its code.
	return ApplyConversion(&LInt{Value: n}, "LINT", to), true
}

// ConversionTypeOf returns the type name a value converts from, as X in
// X_TO_Y names it: LINT, REAL, WORD (a 16-bit bit string), TIME, TOD, ...
func ConversionTypeOf(v Object) string {
	switch x := v.(type) {
	case *BitString:
		switch x.Width {
		case 8:
			return "BYTE"
		case 16:
			return "WORD"
		case 32:
			return "DWORD"
		}
		return "LWORD"
	case *Time:
		return "TIME"
	case *Date:
		return "DATE"
	case *TimeOfDay:
		return "TOD"
	case *DateAndTime:
		return "DT"
	case *Boolean:
		return "BOOL"
	}
	return string(v.Type())
}

// longTimeConversionTypes are the long time types (3rd edition) as the
// types an X_TO_Y conversion names, which hold them.
var longTimeConversionTypes = map[string]string{
	"LTIME": "TIME", "LDATE": "DATE", "LTOD": "TOD", "LTIME_OF_DAY": "TOD",
	"LDT": "DT", "LDATE_AND_TIME": "DT", "TIME_OF_DAY": "TOD", "DATE_AND_TIME": "DT",
}
