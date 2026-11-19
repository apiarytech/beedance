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
	"sync"
	"testing"
)

// withRegistry runs f on an empty registry and restores the real one.
func withRegistry(t *testing.T, f func()) {
	t.Helper()
	savedByName, savedByIndex, savedList, savedStale := builtinsByName, builtinsByIndex, Builtins, registry.stale
	builtinsByName, builtinsByIndex, Builtins = map[string]*Builtin{}, map[int]BuiltinEntry{}, nil
	defer func() {
		builtinsByName, builtinsByIndex, Builtins, registry.stale = savedByName, savedByIndex, savedList, savedStale
	}()
	f()
}

// TestFinalizeBuiltinsOnlyWhenStale checks that FinalizeBuiltins builds
// Builtins once, and again only after a registration.
func TestFinalizeBuiltinsOnlyWhenStale(t *testing.T) {
	withRegistry(t, func() {
		fn := func(args ...Object) Object { return &Null{} }
		RegisterBuiltin(0, "A_FUNC", fn)
		FinalizeBuiltins()
		first := Builtins
		if len(first) != 1 {
			t.Fatalf("Builtins %v", first)
		}
		FinalizeBuiltins()
		if &Builtins[0] != &first[0] {
			t.Fatal("Builtins was built again with nothing registered")
		}
		RegisterBuiltin(1, "B_FUNC", fn)
		FinalizeBuiltins()
		if len(Builtins) != 2 || Builtins[1].Name != "B_FUNC" {
			t.Fatalf("a later registration is missing: %v", Builtins)
		}
	})
}

// TestFinalizeBuiltinsConcurrently calls FinalizeBuiltins from many
// goroutines while others read Builtins, as engines loading programs at
// the same time do. Run with -race.
func TestFinalizeBuiltinsConcurrently(t *testing.T) {
	withRegistry(t, func() {
		fn := func(args ...Object) Object { return &Null{} }
		for i, name := range []string{"A_FUNC", "B_FUNC", "C_FUNC"} {
			RegisterBuiltin(i, name, fn)
		}
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					FinalizeBuiltins()
					if n := len(Builtins); n != 3 {
						t.Errorf("%d built-ins", n)
						return
					}
					for k, e := range Builtins {
						if e.Index != k {
							t.Errorf("Builtins[%d].Index = %d", k, e.Index)
							return
						}
					}
				}
			}()
		}
		wg.Wait()
	})
}
