/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

// SymbolScope represents the scope in which a symbol is defined (e.g., global, local).
type SymbolScope string

// Constants for the different symbol scopes recognized by the compiler.
const (
	LocalScope    SymbolScope = "LOCAL"    // A local variable in the current function/FB scope.
	GlobalScope   SymbolScope = "GLOBAL"   // A global variable accessible from anywhere.
	BuiltinScope  SymbolScope = "BUILTIN"  // A built-in function like LEN or ABS.
	FreeScope     SymbolScope = "FREE"     // A variable from an outer scope that is used in a closure.
	ExternalScope SymbolScope = "EXTERNAL" // A variable accessed via a VAR_ACCESS path.
	FunctionScope SymbolScope = "FUNCTION" // The name of the current function, for recursion.
)

// Symbol represents an identifier in the code, storing its name, scope,
// index within that scope, and whether it is read-only.
type Symbol struct {
	Name       string
	Scope      SymbolScope
	Index      int
	IsConstant bool
	IsReadOnly bool
}

// SymbolTable manages symbols for a given scope and can be chained to represent
// nested scopes. It is crucial for resolving variable names and handling closures.
type SymbolTable struct {
	Outer *SymbolTable // A pointer to the enclosing (outer) scope's symbol table.

	store          map[string]Symbol // The storage for symbols defined in this scope.
	numDefinitions int               // The number of symbols defined in this scope.

	FreeSymbols []Symbol // A list of symbols that are "free" (captured from an outer scope).
}

// NewEnclosedSymbolTable creates a new symbol table that is enclosed by an outer one,
// forming a nested scope.
func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewSymbolTable()
	s.Outer = outer
	return s
}

// NewSymbolTable creates a new, empty symbol table for the outermost (global) scope.
func NewSymbolTable() *SymbolTable {
	s := make(map[string]Symbol)
	free := []Symbol{}
	return &SymbolTable{store: s, FreeSymbols: free}
}

// Define adds a new symbol to the current symbol table. It determines the scope
// (Global or Local) based on whether the table has an outer scope.
func (s *SymbolTable) Define(name string, isConstant bool) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions, IsConstant: isConstant}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}

	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

// DefineExternal defines a symbol for a VAR_ACCESS variable. The symbol's index
// points to the access path string in the constant pool.
func (s *SymbolTable) DefineExternal(name string, index int) Symbol {
	// For external variables, the "Index" refers to the index of the access
	// path string in the constants table.
	symbol := Symbol{Name: name, Scope: ExternalScope, Index: index}
	s.store[name] = symbol
	return symbol
}

// Resolve finds a symbol by name, searching the current scope and then recursively
// searching outer scopes. If a symbol is found in an outer scope (but not
// global, builtin, or external), it is added to the current table's `FreeSymbols` list.
func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	if !ok && s.Outer != nil {
		obj, ok = s.Outer.Resolve(name)
		if !ok {
			return obj, ok
		}

		// ExternalScope is also resolved from outer scopes without becoming "free"
		if obj.Scope == GlobalScope || obj.Scope == BuiltinScope || obj.Scope == ExternalScope {
			return obj, ok
		}

		free := s.defineFree(obj)
		return free, true
	}
	return obj, ok
}

// DefineVarInput defines a symbol for a VAR_INPUT parameter, marking it as read-only.
// This is crucial for enforcing the semantics of IEC 61131-3 functions.
func (s *SymbolTable) DefineVarInput(name string) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions, IsReadOnly: true}
	if s.Outer == nil {
		// This case is unlikely for a VAR_INPUT but included for robustness.
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}
	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

// DefineBuiltin adds a symbol for a built-in function to the symbol table.
func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
	s.store[name] = symbol
	return symbol
}

// DefineFunctionName defines a special symbol for the current function's name.
// This is used to implement recursive function calls.
func (s *SymbolTable) DefineFunctionName(name string) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: FunctionScope}
	s.store[name] = symbol
	return symbol
}

// defineFree takes a symbol from an outer scope, adds it to the current scope's
// list of free symbols, and creates a new `FreeScope` symbol in the current table.
func (s *SymbolTable) defineFree(original Symbol) Symbol {
	s.FreeSymbols = append(s.FreeSymbols, original)

	symbol := Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, IsConstant: original.IsConstant}
	symbol.Scope = FreeScope

	s.store[original.Name] = symbol
	return symbol
}
