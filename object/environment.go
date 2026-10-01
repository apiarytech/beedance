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

import "strings"

// NewEnclosedEnvironment creates a new, nested environment that is enclosed by an outer one.
// This is used for creating local scopes, like inside a function call.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// NewEnvironment creates a new, top-level environment with no outer scope.
func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object), names: make(map[string]string), outer: nil}
}

// Environment holds the variables and functions available in a given scope.
// It supports nesting to create lexical scopes. Names are case-insensitive,
// as identifiers are in IEC 61131-3: a variable declared as `count` is also
// `COUNT`. Each name keeps the spelling it was first stored with.
type Environment struct {
	store map[string]Object
	names map[string]string // The stored spelling of each name, by its upper case.
	outer *Environment
}

// key returns the spelling name is stored with in this scope, if it is.
func (e *Environment) key(name string) (string, bool) {
	if _, ok := e.store[name]; ok {
		return name, true
	}
	stored, ok := e.names[strings.ToUpper(name)]
	return stored, ok
}

// put stores a value under name, or under the spelling of the same name
// already stored in this scope.
func (e *Environment) put(name string, val Object) {
	if stored, ok := e.key(name); ok {
		name = stored
	} else {
		e.names[strings.ToUpper(name)] = name
	}
	e.store[name] = val
}

// Get retrieves an object by name from the environment. If the object is not
// found in the current scope, it recursively searches in outer scopes.
func (e *Environment) Get(name string) (Object, bool) {
	if key, ok := e.key(name); ok {
		return e.store[key], true
	}
	if e.outer != nil {
		return e.outer.Get(name)
	}
	return nil, false
}

// GetRaw retrieves an object from the current environment's store only,
// without searching in outer scopes and without dereferencing pointers.
func (e *Environment) GetRaw(name string) (Object, bool) {
	if key, ok := e.key(name); ok {
		return e.store[key], true
	}
	return nil, false
}

// SetLocal sets a value in the current, local scope only, without checking outer scopes.
// It is an alias for Set.
func (e *Environment) SetLocal(name string, val Object) Object {
	e.put(name, val)
	return val
}

// Set creates or updates a variable in the current, local scope.
// This is used for variable declarations (VAR) and for shadowing variables from an outer scope.
func (e *Environment) Set(name string, val Object) Object {
	e.put(name, val)
	return val
}

// Assign updates an existing variable in the current or any outer scope.
// It walks up the scope chain to find the variable. If not found, it creates it locally.
func (e *Environment) Assign(name string, val Object) Object {
	for env := e; env != nil; env = env.outer {
		if key, ok := env.key(name); ok {
			env.store[key] = val
			return val
		}
	}
	e.put(name, val)
	return val
}

// Names returns a slice of all variable names defined directly in the current
// environment's store, not including names from outer scopes.
func (e *Environment) Names() []string {
	names := make([]string, 0, len(e.store))
	for name := range e.store {
		names = append(names, name)
	}
	return names
}

// Outer returns the enclosing environment.
func (e *Environment) Outer() *Environment {
	return e.outer
}
