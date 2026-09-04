/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and a
 * commercial license. You may choose to use this software under either license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

import "sort"

// BuiltinEntry holds the name and implementation of a built-in function.
// This is used for the indexed list required by the compiler and VM.
type BuiltinEntry struct {
	Name    string   // The name of the built-in function (e.g., "LEN")
	Builtin *Builtin // The function implementation
	Index   int      // The stable, iota-generated index
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

// RegisterBuiltin is called by packages (like evaluator) to register
// the implementation of a built-in function. This function is not thread-safe
// and is intended to be called during application initialization (in init() functions).
func RegisterBuiltin(index int, name string, fn BuiltinFunction) {
	entry := BuiltinEntry{
		Name:    name,
		Builtin: &Builtin{Fn: fn},
		Index:   index,
	}
	builtinsByIndex[index] = entry
	builtinsByName[name] = entry.Builtin // Keep for evaluator's name-based lookup
}

// FinalizeBuiltins sorts the built-ins by name and populates the public
// `Builtins` slice. This function is now designed to create a dense slice
// from the registered built-ins and sort it by the `iota`-generated index.
// This ensures a stable and predictable order for the compiler and VM.
func FinalizeBuiltins() {
	// Convert the map to a slice for sorting.
	// This creates a dense slice, which is more robust than a sparse one.
	builtinsList := make([]BuiltinEntry, 0, len(builtinsByIndex))
	for _, entry := range builtinsByIndex {
		builtinsList = append(builtinsList, entry)
	}

	// Sort the slice based on the iota-generated Index.
	// This is the critical step to ensure the compiler and VM have a
	// consistent index for each built-in function.
	sort.Slice(builtinsList, func(i, j int) bool {
		return builtinsList[i].Index < builtinsList[j].Index
	})

	Builtins = builtinsList
}

// GetBuiltinByName is used by the evaluator to look up built-in functions by name.
func GetBuiltinByName(name string) (*Builtin, bool) {
	b, ok := builtinsByName[name]
	return b, ok
}
