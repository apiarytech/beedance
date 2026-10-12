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
		"REAL", "LREAL", "TIME", "DATE", "TOD", "DT", "STRING", "WSTRING",
		"BYTE", "WORD", "DWORD", "LWORD", "ANY_INT", "ANY_REAL", "BCD":
		return true
	}
	return false
}

// ParseConversion reports whether name, in any case, is a standard type
// conversion X_TO_Y between two different types, and which.
func ParseConversion(name string) (from, to string, ok bool) {
	upper := strings.ToUpper(name)
	i := strings.Index(upper, "_TO_")
	if i <= 0 {
		return "", "", false
	}
	from, to = upper[:i], upper[i+len("_TO_"):]
	if from == to || !isConversionType(from) || !isConversionType(to) {
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
