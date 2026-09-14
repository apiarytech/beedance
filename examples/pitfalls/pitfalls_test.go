package parser

import (
	"io/ioutil"
	"testing"

	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
)

func TestPitfalls(t *testing.T) {
	input, err := ioutil.ReadFile("pitfalls.st")
	if err != nil {
		t.Fatalf("could not read test file: %v", err)
	}

	l := lexer.New(string(input))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Errorf("parser has %d errors", len(p.Errors()))
		for _, msg := range p.Errors() {
			t.Errorf("parser error: %q", msg)
		}
		t.FailNow()
	}

	env := object.NewEnvironment()
	// The initial Eval call will parse and declare all POUs (PROGRAM, FUNCTION, TYPE).
	evaluator.Eval(program, env)

	t.Run("Test_Symbol_Disambiguation", func(t *testing.T) {
		progObj, ok := env.Get("Test_Symbol_Disambiguation")
		if !ok {
			t.Fatal("Program 'Test_Symbol_Disambiguation' not found in environment")
		}
		prog, ok := progObj.(*object.Program)
		if !ok {
			t.Fatalf("Object is not a Program. got=%T", progObj)
		}

		// Evaluate the program body to execute its logic and populate variables.
		evaluator.Eval(prog.Body, prog.Env)

		testIntegerObject(t, prog.Env, "Result1", 6)
		testIntegerObject(t, prog.Env, "Result2", 40)
		testIntegerObject(t, prog.Env, "MyTypedVar", 10)
	})

	t.Run("Test_IL_Operators_As_Vars", func(t *testing.T) {
		progObj, ok := env.Get("Test_IL_Operators_As_Vars")
		if !ok {
			t.Fatal("Program 'Test_IL_Operators_As_Vars' not found in environment")
		}
		prog, ok := progObj.(*object.Program)
		if !ok {
			t.Fatalf("Object is not a Program. got=%T", progObj)
		}

		// Evaluate the program body
		evaluator.Eval(prog.Body, prog.Env)

		testIntegerObject(t, prog.Env, "ST", 30)
		testIntegerObject(t, prog.Env, "Result", 60)
	})

	t.Run("Test_Type_Declaration_Ambiguity", func(t *testing.T) {
		progObj, ok := env.Get("Test_Type_Declaration_Ambiguity")
		if !ok {
			t.Fatal("Program 'Test_Type_Declaration_Ambiguity' not found in environment")
		}
		prog, ok := progObj.(*object.Program)
		if !ok {
			t.Fatalf("Object is not a Program. got=%T", progObj)
		}

		// The variables are initialized during the declaration evaluation,
		// which happened in the initial Eval call. We just need to check them.
		// NOTE: This part of the test may fail if the evaluator does not yet
		// support inheriting default initial values from TYPE definitions.
		// This is expected and highlights an area for improvement in the evaluator.
		testStringObject(t, prog.Env, "Greeting", "Default Greeting")
		testStringObject(t, prog.Env, "AnotherGreeting", "Hello There")
		testIntegerObject(t, prog.Env, "MyNum", 50)
	})
}

// testIntegerObject is a helper function to check the value of an integer variable in the environment.
func testIntegerObject(t *testing.T, env *object.Environment, name string, expected int64) {
	t.Helper()
	obj, ok := env.Get(name)
	if !ok {
		t.Errorf("variable %s not found in environment", name)
		return
	}

	val, _, ok := object.GetIntegerObjectValue(obj)
	if !ok {
		t.Errorf("object for %s is not an integer. got=%T (%+v)", name, obj, obj)
		return
	}

	if val != expected {
		t.Errorf("variable %s has wrong value. got=%d, want=%d", name, val, expected)
	}
}

// testStringObject is a helper function to check the value of a string variable in the environment.
func testStringObject(t *testing.T, env *object.Environment, name string, expected string) {
	t.Helper()
	obj, ok := env.Get(name)
	if !ok {
		t.Errorf("variable %s not found in environment", name)
		return
	}

	str, ok := obj.(*object.String)
	if !ok {
		t.Errorf("object for %s is not a String. got=%T (%+v)", name, obj, obj)
		return
	}

	if str.Value != expected {
		t.Errorf("variable %s has wrong value. got=%q, want=%q", name, str.Value, expected)
	}
}
