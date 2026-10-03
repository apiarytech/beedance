/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
	"testing"
)

func TestFunctionBlockMethods(t *testing.T) {
	tests := []vmTestCase{
		{
			// Methods with inputs, called positionally and by name in any order.
			input: `
			FUNCTION_BLOCK Calc
				METHOD Diff : INT
					VAR_INPUT a : INT; b : INT; END_VAR
					Diff := a - b;
				END_METHOD
			END_FUNCTION_BLOCK
			VAR c : Calc; END_VAR
			c.Diff(10, 3) * 100 + c.Diff(b := 1, a := 10);`,
			expected: 709,
		},
		{
			// A method calls another method of the same instance through THIS.
			input: `
			FUNCTION_BLOCK Counter
				VAR n : INT := 1; END_VAR
				METHOD Twice : INT
					Twice := THIS.Base() * 2;
				END_METHOD
				METHOD Base : INT
					Base := n + 1;
				END_METHOD
			END_FUNCTION_BLOCK
			VAR c : Counter; END_VAR
			c.Twice();`,
			expected: 4,
		},
		{
			// A method of a nested FB instance.
			input: `
			FUNCTION_BLOCK Inner
				VAR v : INT := 7; END_VAR
				METHOD Read : INT
					Read := v;
				END_METHOD
			END_FUNCTION_BLOCK
			FUNCTION_BLOCK Outer
				VAR i : Inner; END_VAR
				METHOD Read : INT
					Read := i.Read() + 1;
				END_METHOD
			END_FUNCTION_BLOCK
			VAR o : Outer; END_VAR
			o.Read();`,
			expected: 8,
		},
	}
	runVmTests(t, tests)
}

func TestFunctionBlockProperties(t *testing.T) {
	const tank = `
		FUNCTION_BLOCK Tank
			VAR amount : INT := 5; END_VAR
			PROPERTY Level : INT
				GET
					Level := amount;
				END_GET
				SET
					amount := value;
				END_SET
			END_PROPERTY
		END_FUNCTION_BLOCK
		VAR t : Tank; END_VAR
	`
	tests := []vmTestCase{
		{tank + "t.Level;", 5},
		{tank + "t.Level := 42; t.Level;", 42},
	}
	runVmTests(t, tests)
}

func TestFunctionBlockInstanceMembers(t *testing.T) {
	tests := []vmTestCase{
		// A member that is neither a variable nor a method reads as NULL.
		{"FUNCTION_BLOCK Empty END_FUNCTION_BLOCK VAR e : Empty; END_VAR e.Missing;", Null},
		// Instance variables keep their initial values, including inherited ones.
		{`FUNCTION_BLOCK Base VAR x : INT := 3; END_VAR END_FUNCTION_BLOCK
		  FUNCTION_BLOCK Derived EXTENDS Base VAR y : INT := 4; END_VAR END_FUNCTION_BLOCK
		  VAR d : Derived; END_VAR d.x * 10 + d.y;`, 34},
	}
	runVmTests(t, tests)
}

func TestMethodAsValueIsBound(t *testing.T) {
	machine := New(compileForTest(t, `
		FUNCTION_BLOCK FB
			METHOD M : INT
				M := 1;
			END_METHOD
		END_FUNCTION_BLOCK
		VAR x : FB; END_VAR
		x.M;`))
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	bound, ok := machine.LastPoppedStackElem().(*BoundMethod)
	if !ok {
		t.Fatalf("expected a bound method, got %T", machine.LastPoppedStackElem())
	}
	if bound.Type() != "BOUND_METHOD" || bound.Inspect() != "bound method" {
		t.Fatalf("unexpected bound method description: %s / %s", bound.Type(), bound.Inspect())
	}
	if _, ok := bound.Receiver.(*object.Hash); !ok {
		t.Fatalf("expected the receiver to be the instance hash, got %T", bound.Receiver)
	}
}

func TestCallingNonCallable(t *testing.T) {
	runVmErrorTests(t, []vmErrorTestCase{
		{"VAR x : INT := 1; END_VAR x();", "calling non-closure and non-builtin"},
	})
}

// newClass builds a class hash with the given methods and optional parent.
func newClass(parent object.Object, methods map[string]object.Object) *object.Hash {
	pairs := map[object.HashKey]object.HashPair{}
	for name, m := range methods {
		key := &object.String{Value: name}
		pairs[key.HashKey()] = object.HashPair{Key: key, Value: m}
	}
	if parent != nil {
		key := &object.String{Value: "__parent__"}
		pairs[key.HashKey()] = object.HashPair{Key: key, Value: parent}
	}
	return &object.Hash{Pairs: pairs}
}

// newInstance builds an instance hash whose "__class__" is class.
func newInstance(class object.Object) *object.Hash {
	key := &object.String{Value: "__class__"}
	return &object.Hash{Pairs: map[object.HashKey]object.HashPair{key.HashKey(): {Key: key, Value: class}}}
}

func TestInstanceClassLookup(t *testing.T) {
	class := newClass(nil, nil)
	if got, ok := instanceClass(newInstance(class)); !ok || got != class {
		t.Fatalf("expected the instance's class, got %v %v", got, ok)
	}
	if _, ok := instanceClass(newInstance(&object.LInt{Value: 1})); ok {
		t.Fatalf("a non-hash __class__ must not be treated as a class")
	}
	if _, ok := instanceClass(&object.Hash{Pairs: map[object.HashKey]object.HashPair{}}); ok {
		t.Fatalf("a hash without __class__ is not an instance")
	}
}

func TestMethodLookupThroughInheritance(t *testing.T) {
	method := &object.Closure{Fn: compiledFunction(1, 1, code.Make(code.OpReturn))}
	grandparent := newClass(nil, map[string]object.Object{"Run": method})
	child := newClass(newClass(grandparent, nil), nil)
	machine := New(emptyBytecode())

	found, err := machine.findMethodInHierarchy(child, &object.String{Value: "Run"})
	if err != nil || found != method {
		t.Fatalf("expected the grandparent's method, got %v (err %v)", found, err)
	}
	if _, err := machine.findMethodInHierarchy(child, &object.String{Value: "Nope"}); err == nil || err.Error() != "method 'Nope' not found in class hierarchy" {
		t.Fatalf("expected a not-found error, got %v", err)
	}
	if _, err := machine.findClosureOwner(child, &object.Closure{}); err == nil || err.Error() != "internal VM error: closure owner not found in class hierarchy" {
		t.Fatalf("expected a closure-owner error, got %v", err)
	}
}

// TestSuperIndexErrors covers SUPER lookups that valid programs never produce.
func TestSuperIndexErrors(t *testing.T) {
	current := &object.Closure{Fn: compiledFunction(0, 0)}
	parentMethod := &object.Closure{Fn: compiledFunction(1, 1)}
	name := &object.String{Value: "Run"}

	// executeSuperIndex looks up the currently running closure in the class.
	withFrame := func() *VM {
		machine := New(emptyBytecode())
		machine.pushFrame(NewFrame(current, 0))
		return machine
	}
	classWithBadParent := newClass(&object.LInt{Value: 1}, map[string]object.Object{"Run": current})

	tests := []struct {
		name     string
		instance object.Object
		method   object.Object
		want     string
	}{
		{"method name is not a string", newInstance(newClass(nil, nil)), &object.LInt{Value: 1}, "super index method name must be a string, got *object.LInt"},
		{"base is not an instance", &object.LInt{Value: 1}, name, "base of super call must be a function block instance (hash), got *object.LInt"},
		{"__class__ is not a hash", newInstance(&object.LInt{Value: 1}), name, "internal VM error: __class__ is not a hash"},
		{"current method is not in the class", newInstance(newClass(nil, nil)), name, "internal VM error: closure owner not found in class hierarchy"},
		{"class has no parent", newInstance(newClass(nil, map[string]object.Object{"Run": current})), name, "super call on a class with no parent"},
		{"parent is not a hash", newInstance(classWithBadParent), name, "internal VM error: parent class is not a hash"},
		{"parent lacks the method", newInstance(newClass(newClass(nil, nil), map[string]object.Object{"Run": current})), name, "method 'Run' not found in class hierarchy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := withFrame().executeSuperIndex(tt.instance, tt.method)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("expected error %q, got %v", tt.want, err)
			}
		})
	}

	// A class hash used directly as the instance (no __class__) still resolves SUPER.
	machine := withFrame()
	class := newClass(newClass(nil, map[string]object.Object{"Run": parentMethod}), map[string]object.Object{"Run": current})
	if err := machine.executeSuperIndex(class, name); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	bound, ok := machine.stack[machine.sp-1].(*BoundMethod)
	if !ok || bound.Fn != parentMethod {
		t.Fatalf("expected the parent's method bound to the instance, got %v", machine.stack[machine.sp-1])
	}
}

func TestBoundMethodCallStackOverflow(t *testing.T) {
	machine := New(emptyBytecode())
	machine.sp = StackSize
	machine.stack[machine.sp-1] = &BoundMethod{Fn: &object.Closure{Fn: compiledFunction(1, 1)}, Receiver: Null}
	if err := machine.executeCall(0); err == nil || err.Error() != "stack overflow" {
		t.Fatalf("expected a stack overflow, got %v", err)
	}
}

// TestPropertyByNameInsideMethod checks that a method can read and assign its
// FB's property by name, without THIS.
func TestPropertyByNameInsideMethod(t *testing.T) {
	runVmTests(t, []vmTestCase{
		{`FUNCTION_BLOCK B1
			VAR v : INT := 1; END_VAR
			PROPERTY P : INT
				GET P := v; END_GET
				SET v := value; END_SET
			END_PROPERTY
			METHOD M : INT
				P := 3;
				M := P * 10;
			END_METHOD
		  END_FUNCTION_BLOCK
		  VAR x : B1; END_VAR
		  x.M();`, 30},
	})
}
