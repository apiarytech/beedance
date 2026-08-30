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
	"sort"
)

// BuiltinEntry holds the name and implementation of a built-in function.
// This is used for the indexed list required by the compiler and VM.
type BuiltinEntry struct {
	Name    string
	Builtin *Builtin
}

// Builtins is a slice of all registered built-in functions, sorted by name.
// This provides a stable index for the compiler and VM. It is populated by
// the FinalizeBuiltins function at startup.
var Builtins []BuiltinEntry

// builtinsByName is a map used for registration and for the evaluator's lookup.
var builtinsByName = map[string]*Builtin{}

// RegisterBuiltin is called by packages (like evaluator) to register
// the implementation of a built-in function. This function is not thread-safe
// and is intended to be called during application initialization (in init() functions).
func RegisterBuiltin(name string, fn BuiltinFunction) {
	builtinsByName[name] = &Builtin{Fn: fn}
}

// FinalizeBuiltins sorts the built-ins by name and populates the public
// `Builtins` slice. This MUST be called after all built-ins have been
// registered, typically in a main init() or at the start of main().
func FinalizeBuiltins() {
	names := make([]string, 0, len(builtinsByName))
	for name := range builtinsByName {
		names = append(names, name)
	}
	sort.Strings(names)

	Builtins = make([]BuiltinEntry, len(names))
	for i, name := range names {
		Builtins[i] = BuiltinEntry{Name: name, Builtin: builtinsByName[name]}
	}
}

// GetBuiltinByName is used by the evaluator to look up built-in functions by name.
func GetBuiltinByName(name string) (*Builtin, bool) {
	b, ok := builtinsByName[name]
	return b, ok
}
