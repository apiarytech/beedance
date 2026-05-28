package object

import "testing"

func TestNewEnvironment(t *testing.T) {
	env := NewEnvironment()

	if env == nil {
		t.Fatal("NewEnvironment() returned nil")
	}
	if env.store == nil {
		t.Fatal("NewEnvironment() store is nil")
	}
	if env.outer != nil {
		t.Fatalf("NewEnvironment() outer is not nil. got=%+v", env.outer)
	}
}

func TestEnvironmentGetSet(t *testing.T) {
	env := NewEnvironment()
	val := &Int{Value: 10}
	name := "myVar"

	// Test Set
	setObj := env.Set(name, val)
	if setObj != val {
		t.Errorf("Set() returned wrong object. expected=%+v, got=%+v", val, setObj)
	}

	// Test Get existing variable
	getObj, ok := env.Get(name)
	if !ok {
		t.Errorf("Get() failed to retrieve existing variable %q", name)
	}
	if getObj != val {
		t.Errorf("Get() retrieved wrong object. expected=%+v, got=%+v", val, getObj)
	}

	// Test Get non-existent variable
	_, ok = env.Get("nonExistentVar")
	if ok {
		t.Errorf("Get() unexpectedly found non-existent variable")
	}
}

func TestNewEnclosedEnvironment(t *testing.T) {
	outer := NewEnvironment()
	inner := NewEnclosedEnvironment(outer)

	if inner == nil {
		t.Fatal("NewEnclosedEnvironment() returned nil")
	}
	if inner.outer != outer {
		t.Errorf("NewEnclosedEnvironment() outer is wrong. expected=%+v, got=%+v", outer, inner.outer)
	}
}

func TestEnclosedEnvironmentGet(t *testing.T) {
	outer := NewEnvironment()
	outerVar := &Int{Value: 100}
	outer.Set("outerVar", outerVar)

	inner := NewEnclosedEnvironment(outer)
	innerVar := &Int{Value: 200}
	inner.Set("innerVar", innerVar)

	// Test Get from inner for inner variable
	obj, ok := inner.Get("innerVar")
	if !ok {
		t.Errorf("inner.Get() failed to retrieve inner variable")
	}
	if obj != innerVar {
		t.Errorf("inner.Get() retrieved wrong inner variable. expected=%+v, got=%+v", innerVar, obj)
	}

	// Test Get from inner for outer variable
	obj, ok = inner.Get("outerVar")
	if !ok {
		t.Errorf("inner.Get() failed to retrieve outer variable")
	}
	if obj != outerVar {
		t.Errorf("inner.Get() retrieved wrong outer variable. expected=%+v, got=%+v", outerVar, obj)
	}

	// Test Get from outer for inner variable (should not be found)
	_, ok = outer.Get("innerVar")
	if ok {
		t.Errorf("outer.Get() unexpectedly found inner variable")
	}

	// Test shadowing: inner variable with same name as outer
	shadowVar := &Int{Value: 300}
	inner.Set("outerVar", shadowVar) // Shadowing the outerVar

	obj, ok = inner.Get("outerVar")
	if !ok {
		t.Errorf("inner.Get() failed to retrieve shadowed variable")
	}
	if obj != shadowVar {
		t.Errorf("inner.Get() retrieved wrong shadowed variable. expected=%+v, got=%+v", shadowVar, obj)
	}
	// Ensure outer environment's variable is unchanged
	obj, _ = outer.Get("outerVar")
	if obj != outerVar {
		t.Errorf("outer.Get() variable changed by inner environment. expected=%+v, got=%+v", outerVar, obj)
	}
}

func TestEnvironmentGetRaw(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("outerVar", &Int{Value: 1})

	inner := NewEnclosedEnvironment(outer)
	inner.Set("innerVar", &Int{Value: 2})

	// 1. GetRaw should find a variable in the current environment.
	obj, ok := inner.GetRaw("innerVar")
	if !ok {
		t.Fatal("GetRaw() failed to retrieve variable from current environment")
	}
	if obj.(*Int).Value != 2 {
		t.Fatalf("GetRaw() retrieved wrong value. want=2, got=%d", obj.(*Int).Value)
	}

	// 2. GetRaw should NOT find a variable in the outer environment.
	_, ok = inner.GetRaw("outerVar")
	if ok {
		t.Fatal("GetRaw() unexpectedly found a variable in the outer environment")
	}

	// 3. For contrast, confirm Get() DOES find the outer variable.
	obj, ok = inner.Get("outerVar")
	if !ok {
		t.Fatal("Get() failed to retrieve variable from outer environment for comparison")
	}
	if obj.(*Int).Value != 1 {
		t.Fatalf("Get() retrieved wrong value from outer environment. want=1, got=%d", obj.(*Int).Value)
	}
}

func TestEnvironmentNames(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("var1", &Int{Value: 1})
	outer.Set("var2", &Boolean{Value: true})

	inner := NewEnclosedEnvironment(outer)
	inner.Set("var3", &String{Value: "test"})
	inner.Set("var1", &Int{Value: 99}) // Shadow outer var

	names := inner.Names()

	if len(names) != 2 {
		t.Fatalf("Names() returned wrong number of names. want=2, got=%d", len(names))
	}

	expectedNames := map[string]bool{
		"var3": true,
		"var1": true,
	}

	for _, name := range names {
		if !expectedNames[name] {
			t.Errorf("Names() returned unexpected name: %s", name)
		}
	}
}
