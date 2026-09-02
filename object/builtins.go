/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and a
 * commercial license. You may choose to use this software under either license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

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
var builtinsByName = make(map[string]*Builtin)

// builtinsByIndex is a temporary map to hold builtins before finalization.
// It maps the stable iota-generated index to the builtin's definition.
var builtinsByIndex = make(map[int]BuiltinEntry)
var maxBuiltinIndex = -1

// RegisterBuiltin is called by packages (like evaluator) to register
// the implementation of a built-in function. This function is not thread-safe
// and is intended to be called during application initialization (in init() functions).
func RegisterBuiltin(index int, name string, fn BuiltinFunction) {
	entry := BuiltinEntry{
		Name:    name,
		Builtin: &Builtin{Fn: fn},
	}
	builtinsByIndex[index] = entry
	builtinsByName[name] = entry.Builtin // Keep for evaluator's name-based lookup

	if index > maxBuiltinIndex {
		maxBuiltinIndex = index
	}
}

// FinalizeBuiltins sorts the built-ins by name and populates the public
// `Builtins` slice. This MUST be called after all built-ins have been
// registered, typically in a main init() or at the start of main().
func FinalizeBuiltins() {
	if len(builtinsByIndex) == 0 {
		Builtins = []BuiltinEntry{}
		return
	}

	// Create a slice large enough to hold all builtins up to the max registered index.
	Builtins = make([]BuiltinEntry, maxBuiltinIndex+1)

	// Place each builtin at its designated index. This creates a sparse slice if some
	// indices are not registered, but ensures that the index remains stable and
	// consistent with the iota constants.
	for index, entry := range builtinsByIndex {
		Builtins[index] = entry
	}
}

// GetBuiltinByName is used by the evaluator to look up built-in functions by name.
func GetBuiltinByName(name string) (*Builtin, bool) {
	b, ok := builtinsByName[name]
	return b, ok
}
