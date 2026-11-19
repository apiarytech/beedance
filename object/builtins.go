/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and a
 * commercial license. You may choose to use this software under either license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

import (
	"sort"
	"sync"
)

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

// registry guards the registration maps against FinalizeBuiltins, and says
// whether Builtins must be built again.
var registry struct {
	sync.Mutex
	stale bool // a built-in was registered since Builtins was built
}

func init() { registry.stale = true }

// RegisterBuiltin is called by packages (like evaluator) to register
// the implementation of a built-in function. This function is not thread-safe
// and is intended to be called during application initialization (in init() functions).
func RegisterBuiltin(index int, name string, fn BuiltinFunction) {
	entry := BuiltinEntry{
		Name:    name,
		Builtin: &Builtin{Fn: fn},
		Index:   index,
	}
	registry.Lock()
	defer registry.Unlock()
	builtinsByIndex[index] = entry
	builtinsByName[name] = entry.Builtin // Keep for evaluator's name-based lookup
	registry.stale = true
}

// FinalizeBuiltins sorts the built-ins by name and populates the public
// `Builtins` slice. This function is now designed to create a dense slice
// from the registered built-ins and sort it by the `iota`-generated index.
// This ensures a stable and predictable order for the compiler and VM.
//
// It is safe to call from many goroutines, as every engine's entry point
// does: Builtins is built again only when a built-in was registered since
// it was last built, so once built it is only read, and programs compiled
// and run at the same time do not race on it.
func FinalizeBuiltins() {
	registry.Lock()
	defer registry.Unlock()
	if !registry.stale {
		return
	}
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
	registry.stale = false
}

// GetBuiltinByName is used by the evaluator to look up built-in functions by
// name. A type conversion X_TO_Y is made on first use (see ParseConversion).
func GetBuiltinByName(name string) (*Builtin, bool) {
	if b, ok := builtinsByName[name]; ok {
		return b, ok
	}
	if b, ok := timeFunctionBuiltin(name); ok {
		return b, ok
	}
	return conversionBuiltin(name)
}

// BuiltinNames names the registered built-in functions, sorted. Unlike
// Builtins it needs no FinalizeBuiltins and changes nothing: an editor's
// completion reads it while programs compile.
func BuiltinNames() []string {
	names := make([]string, 0, len(builtinsByName))
	for name := range builtinsByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
