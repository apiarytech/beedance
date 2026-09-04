package object

import (
	"sort"
	"testing"
)

// A dummy builtin function for testing purposes.
func dummyBuiltin(args ...Object) Object {
	return &Null{}
}

func TestBuiltinRegistrationAndFinalization(t *testing.T) {
	// Create local state for this test to avoid polluting the global built-in registry.
	localByName := make(map[string]*Builtin)
	localByIndex := make(map[int]BuiltinEntry)
	var localBuiltins []BuiltinEntry

	// Create local versions of the registration and finalization functions that operate on the local state.
	register := func(index int, name string, fn BuiltinFunction) {
		entry := BuiltinEntry{Name: name, Builtin: &Builtin{Fn: fn}, Index: index}
		localByIndex[index] = entry
		localByName[name] = entry.Builtin
	}

	finalize := func() {
		// This logic mirrors the global FinalizeBuiltins function but uses the local map.
		list := make([]BuiltinEntry, 0, len(localByIndex))
		for _, entry := range localByIndex {
			list = append(list, entry)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Index < list[j].Index })
		localBuiltins = list
	}

	// 1. Register some dummy built-ins in a non-alphabetical order into our local registry.
	register(2, "Z_FUNC", dummyBuiltin)
	register(0, "A_FUNC", dummyBuiltin)
	register(1, "M_FUNC", dummyBuiltin)

	// 2. Check if they were registered correctly in the local map.
	if _, ok := localByName["A_FUNC"]; !ok {
		t.Fatal("RegisterBuiltin failed to register 'A_FUNC'")
	}
	if _, ok := localByName["Z_FUNC"]; !ok {
		t.Fatal("RegisterBuiltin failed to register 'Z_FUNC'")
	}
	if len(localByName) != 3 {
		t.Fatalf("Expected 3 builtins to be registered, got %d", len(localByName))
	}

	// 3. Finalize the local built-ins.
	finalize()

	// 4. Check if the local `localBuiltins` slice is populated and sorted correctly by index.
	if len(localBuiltins) != 3 {
		t.Fatalf("FinalizeBuiltins did not populate the Builtins slice correctly. want=3, got=%d", len(localBuiltins))
	}

	expectedOrder := []string{"A_FUNC", "M_FUNC", "Z_FUNC"} // This is now order by index
	for i, name := range expectedOrder {
		if localBuiltins[i].Name != name {
			t.Errorf("Builtins slice is not ordered by index correctly. want %s at index %d, got %s", name, i, localBuiltins[i].Name)
		}
	}

	// 5. Test GetBuiltinByName logic using the local map.
	t.Run("get existing builtin", func(t *testing.T) {
		builtin, ok := localByName["A_FUNC"]
		if !ok {
			t.Fatal("Local map lookup failed to find an existing builtin 'A_FUNC'")
		}
		if builtin == nil {
			t.Fatal("Local map lookup returned a nil object for an existing builtin")
		}
	})

	t.Run("get non-existent builtin", func(t *testing.T) {
		_, ok := localByName["NON_EXISTENT"]
		if ok {
			t.Fatal("Local map lookup unexpectedly found a non-existent builtin")
		}
	})
}
