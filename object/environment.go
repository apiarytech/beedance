package object

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

// GetRaw retrieves an object from the environment without dereferencing pointers.
func (e *Environment) GetRaw(name string) (Object, bool) {
	obj, ok := e.store[name]
	return obj, ok
}

func (e *Environment) SetLocal(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Set creates or updates a variable in the current, local scope.
// This is used for variable declaration and shadowing.
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

// Names returns a slice of all variable names in the environment's store.
func (e *Environment) Names() []string {
	names := make([]string, 0, len(e.store))
	for name := range e.store {
		names = append(names, name)
	}
	return names
}
