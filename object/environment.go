package object

// NewEnclosedEnvironment creates a new, nested environment that is enclosed by an outer one.
// This is used for creating local scopes, like inside a function call.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// NewEnvironment creates a new, top-level environment with no outer scope.
func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

// Environment holds the variables and functions available in a given scope.
// It supports nesting to create lexical scopes.
type Environment struct {
	store map[string]Object
	outer *Environment
}

// Get retrieves an object by name from the environment. If the object is not
// found in the current scope, it recursively searches in outer scopes.
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

// GetRaw retrieves an object from the current environment's store only,
// without searching in outer scopes and without dereferencing pointers.
func (e *Environment) GetRaw(name string) (Object, bool) {
	obj, ok := e.store[name]
	return obj, ok
}

// SetLocal sets a value in the current, local scope only, without checking outer scopes.
// It is an alias for Set.
func (e *Environment) SetLocal(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Set creates or updates a variable in the current, local scope.
// This is used for variable declarations (VAR) and for shadowing variables from an outer scope.
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Assign updates an existing variable in the current or any outer scope.
// It walks up the scope chain to find the variable. If not found, it creates it locally.
func (e *Environment) Assign(name string, val Object) Object {
	env := e
	for env != nil {
		if _, ok := env.store[name]; ok {
			env.store[name] = val
			return val
		}
		env = env.outer
	}
	e.store[name] = val
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
