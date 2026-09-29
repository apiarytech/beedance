package main

import (
	"os"
	"testing"

	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
)

func TestOopExample(t *testing.T) {
	// Read the ST source code
	input, err := os.ReadFile("oop_example.st")
	if err != nil {
		t.Fatalf("could not read example file: %v", err)
	}

	// Create a new environment for the evaluator
	env := object.NewEnvironment()

	// Lex, Parse, and Evaluate the program
	l := lexer.New(string(input))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		t.Fatalf("parser had %d errors: %v", len(p.Errors()), p.Errors())
	}

	// The initial Eval call will parse and declare all POUs (PROGRAM, FUNCTION, TYPE).
	evaluator.Eval(program, env)

	// Get the specific program we want to test
	progObj, ok := env.Get("OopTestProgram")
	if !ok {
		t.Fatal("Program 'OopTestProgram' not found in environment")
	}
	prog, ok := progObj.(*object.Program)
	if !ok {
		t.Fatalf("Object is not a Program. got=%T", progObj)
	}

	// The evaluator will run the program logic within the program's own environment
	evaluated := evaluator.Eval(prog.Body, prog.Env)
	if evaluated != nil && evaluated.Type() == object.ERROR_OBJ {
		t.Fatalf("evaluator error: %s", evaluated.Inspect())
	}

	// --- Assert the final state of the program variables ---

	// Check Motor1 state. Variables are now in the program's environment (prog.Env)
	isMotor1Running, ok := prog.Env.Get("IsMotor1Running")
	if !ok {
		t.Fatal("variable 'IsMotor1Running' not found in environment")
	}
	if running, ok := isMotor1Running.(*object.Boolean); !ok || !running.Value {
		t.Errorf("expected IsMotor1Running to be BOOL TRUE, got %T %v", isMotor1Running, isMotor1Running.Inspect())
	}

	testSpeed, ok := prog.Env.Get("TestSpeed")
	if !ok {
		t.Fatal("variable 'TestSpeed' not found in environment")
	}
	// TestSpeed is declared LREAL. The second Motor1 call runs the FB body while
	// the motor is running, so `internalSpeed := Voltage * 100.0` gives 1200.0.
	if speed, ok := testSpeed.(*object.LReal); !ok || speed.Value != 1200.0 {
		t.Errorf("expected TestSpeed to be LREAL 1200.0, got %T %v", testSpeed, testSpeed.Inspect())
	}

	// Check Motor2 state from the ST file (which is not present in this version)
	// isMotor2Running, ok := env.Get("IsMotor2Running")
	// if !ok {
	// 	t.Fatal("variable 'IsMotor2Running' not found in environment")
	// }
	// if isMotor2Running.(*object.Boolean).Value != false {
	// 	t.Errorf("expected IsMotor2Running to be FALSE, got TRUE")
	// }
}
