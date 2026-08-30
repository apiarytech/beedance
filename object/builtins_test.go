package object

import (
	"testing"
)

// A dummy builtin function for testing purposes.
func dummyBuiltin(args ...Object) Object {
	return &Null{}
}

func TestBuiltinRegistrationAndFinalization(t *testing.T) {
	// Reset the global state for a clean test run.
	// This is important because builtins are global.
	builtinsByName = make(map[string]*Builtin)
	Builtins = nil

	// 1. Register some built-ins in a non-alphabetical order.
	RegisterBuiltin("Z_FUNC", dummyBuiltin)
	RegisterBuiltin("A_FUNC", dummyBuiltin)
	RegisterBuiltin("M_FUNC", dummyBuiltin)

	// 2. Check if they are in the internal map.
	if _, ok := builtinsByName["A_FUNC"]; !ok {
		t.Fatal("RegisterBuiltin failed to register 'A_FUNC'")
	}
	if _, ok := builtinsByName["Z_FUNC"]; !ok {
		t.Fatal("RegisterBuiltin failed to register 'Z_FUNC'")
	}
	if len(builtinsByName) != 3 {
		t.Fatalf("Expected 3 builtins to be registered, got %d", len(builtinsByName))
	}

	// 3. Finalize the built-ins.
	FinalizeBuiltins()

	// 4. Check if the public `Builtins` slice is populated and sorted.
	if len(Builtins) != 3 {
		t.Fatalf("FinalizeBuiltins did not populate the Builtins slice correctly. want=3, got=%d", len(Builtins))
	}

	expectedOrder := []string{"A_FUNC", "M_FUNC", "Z_FUNC"}
	for i, name := range expectedOrder {
		if Builtins[i].Name != name {
			t.Errorf("Builtins slice is not sorted correctly. want %s at index %d, got %s", name, i, Builtins[i].Name)
		}
	}
}

func TestGetBuiltinByName(t *testing.T) {
	// Reset global state
	builtinsByName = make(map[string]*Builtin)
	Builtins = nil

	// Register a known function
	RegisterBuiltin("TEST_GET", dummyBuiltin)

	// Test getting an existing builtin
	t.Run("get existing builtin", func(t *testing.T) {
		builtin, ok := GetBuiltinByName("TEST_GET")
		if !ok {
			t.Fatal("GetBuiltinByName failed to find an existing builtin 'TEST_GET'")
		}
		if builtin == nil {
			t.Fatal("GetBuiltinByName returned a nil object for an existing builtin")
		}
		// We can't compare functions directly, but we can check it's not nil.
		if builtin.Fn == nil {
			t.Error("The returned builtin function is nil")
		}
	})

	// Test getting a non-existent builtin
	t.Run("get non-existent builtin", func(t *testing.T) {
		builtin, ok := GetBuiltinByName("NON_EXISTENT")
		if ok {
			t.Fatal("GetBuiltinByName unexpectedly found a non-existent builtin")
		}
		if builtin != nil {
			t.Fatal("GetBuiltinByName returned a non-nil object for a non-existent builtin")
		}
	})
}
