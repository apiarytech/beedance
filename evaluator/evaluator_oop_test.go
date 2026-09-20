package evaluator

import (
	"beedance/ast"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"testing"
)

func TestFunctionBlockMethodCall(t *testing.T) {
	input := `
		PROGRAM TestOOP
			VAR
				myFb : Counter;
				res1 : INT;
				res2 : INT;
			END_VAR

			res1 := myFb.Increment(Amount := 2);
			res2 := myFb.Increment(Amount := 3);
		END_PROGRAM

		FUNCTION_BLOCK Counter
			VAR
				Value : INT := 0;
			END_VAR

			METHOD Increment : INT
				VAR_INPUT
					Amount : INT;
				END_VAR
				Value := Value + Amount;
				Increment := Value;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	env := object.NewEnvironment()
	Eval(testParseProgramOOP(t, input), env) // Define POUs

	progObj, _ := env.Get("TestOOP")
	prog := progObj.(*object.Program)
	Eval(prog.Body, prog.Env) // Execute the program body

	testIntegerObjectInEnv(t, prog.Env, "res1", 2)
	testIntegerObjectInEnv(t, prog.Env, "res2", 5)
}

func TestFunctionBlock_THIS_and_Properties(t *testing.T) {
	input := `
		PROGRAM TestOOP
			VAR
				myFb : StatefulCounter;
				val1 : INT;
				val2 : INT;
			END_VAR

			myFb.Offset := 10;
			val1 := myFb.CurrentValue;
			myFb.Increment();
			val2 := myFb.CurrentValue;
		END_PROGRAM

		FUNCTION_BLOCK StatefulCounter
			VAR
				count : INT := 5;
			END_VAR

			PROPERTY CurrentValue : INT
				GET
					CurrentValue := THIS.count;
				END_GET
				SET
					THIS.count := value;
				END_SET
			END_PROPERTY

			PROPERTY Offset : INT
				GET
					Offset := 0; // Not used
				END_GET
				SET
					THIS.count := THIS.count + value;
				END_SET
			END_PROPERTY

			METHOD Increment : VOID
				THIS.count := THIS.count + 1;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	env := object.NewEnvironment()
	Eval(testParseProgramOOP(t, input), env) // Define POUs

	progObj, _ := env.Get("TestOOP")
	prog := progObj.(*object.Program)
	Eval(prog.Body, prog.Env) // Execute the program body

	// Initial count is 5. Offset adds 10. So val1 should be 15.
	testIntegerObjectInEnv(t, prog.Env, "val1", 15)
	// Increment adds 1. So val2 should be 16.
	testIntegerObjectInEnv(t, prog.Env, "val2", 16)
}

func TestFunctionBlockInheritanceAndSuper(t *testing.T) {
	input := `
		PROGRAM TestOOP
			VAR
				myFb : DerivedCounter;
				res : INT;
			END_VAR

			myFb.Increment(); // Base adds 1, Derived adds 10 -> 11
			myFb.Increment(); // Base adds 1, Derived adds 10 -> 22
			res := myFb.Value;
		END_PROGRAM

		FUNCTION_BLOCK BaseCounter
			VAR PUBLIC
				Value : INT := 0;
			END_VAR

			METHOD Increment : VOID
				THIS.Value := THIS.Value + 1;
			END_METHOD
		END_FUNCTION_BLOCK

		FUNCTION_BLOCK DerivedCounter EXTENDS BaseCounter
			METHOD Increment : VOID
				SUPER^.Increment(); // Call parent's method
				THIS.Value := THIS.Value + 10;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	env := object.NewEnvironment()
	Eval(testParseProgramOOP(t, input), env) // Define POUs

	progObj, _ := env.Get("TestOOP")
	prog := progObj.(*object.Program)
	Eval(prog.Body, prog.Env) // Execute the program body

	testIntegerObjectInEnv(t, prog.Env, "res", 22)
}

func TestInterfaceAndAbstractEnforcement(t *testing.T) {
	baseProgram := `
		INTERFACE iMotor
			METHOD Start : BOOL;
			METHOD Stop : VOID;
			PROPERTY Speed : INT GET;
		END_INTERFACE

		FUNCTION_BLOCK ABSTRACT AbstractMotor IMPLEMENTS iMotor
			VAR PROTECTED
				currentSpeed : INT;
			END_VAR

			METHOD ABSTRACT Start : BOOL
			END_METHOD

			METHOD Stop : VOID
				THIS.currentSpeed := 0;
			END_METHOD

			PROPERTY Speed : INT
				GET
					Speed := THIS.currentSpeed;
				END_GET
			END_PROPERTY
		END_FUNCTION_BLOCK

		FUNCTION_BLOCK ConcreteMotor EXTENDS AbstractMotor
			METHOD Start : BOOL
				THIS.currentSpeed := 100;
				Start := TRUE;
			END_METHOD
		END_FUNCTION_BLOCK

		FUNCTION_BLOCK BadMotor IMPLEMENTS iMotor
			// Missing the 'Stop' method
			METHOD Start : BOOL
				Start := TRUE;
			END_METHOD
			PROPERTY Speed : INT GET
				Speed := 0;
			END_PROPERTY
		END_FUNCTION_BLOCK
	`

	t.Run("Cannot instantiate ABSTRACT function block", func(t *testing.T) {
		input := `
		PROGRAM TestAbstract
			VAR
				myMotor : AbstractMotor;
			END_VAR
		END_PROGRAM
		` + baseProgram
		evaluated := testEval(t, input)
		testErrorObjectContains(t, evaluated, "cannot instantiate abstract function block 'AbstractMotor'")
	})

	t.Run("Cannot instantiate FB that does not implement interface", func(t *testing.T) {
		input := `
		PROGRAM TestBadMotor
			VAR
				myMotor : BadMotor;
			END_VAR
		END_PROGRAM
		` + baseProgram
		evaluated := testEval(t, input)
		testErrorObjectContains(t, evaluated, "function block 'BadMotor' does not implement method 'Stop' from interface 'iMotor'")
	})

	t.Run("Successfully instantiate and use concrete implementation", func(t *testing.T) {
		input := `
			PROGRAM TestConcrete
				VAR
					myMotor : ConcreteMotor;
					isStarted : BOOL;
					currentSpeed : INT;
				END_VAR

				isStarted := myMotor.Start(); (* Call method *)
				currentSpeed := myMotor.Speed; (* Read property *)
			END_PROGRAM
		` + baseProgram
		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env) // Define POUs

		progObj, _ := env.Get("TestConcrete")
		progEnv := progObj.(*object.Program).Env
		prog := progObj.(*object.Program)
		Eval(prog.Body, prog.Env) // Execute the program body

		testBooleanObjectInEnv(t, progEnv, "isStarted", true)
		testIntegerObjectInEnv(t, progEnv, "currentSpeed", 100)

		// Now test the inherited 'Stop' method
		Eval(testParseProgramOOP(t, "myMotor.Stop();"), progEnv)
		Eval(testParseProgramOOP(t, "currentSpeed := myMotor.Speed;"), progEnv)
		testIntegerObjectInEnv(t, progEnv, "currentSpeed", 0)
	})
}

func TestPrivatePublicandProtected(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK Base
			VAR PROTECTED
				protectedVar : INT := 10;
			END_VAR
			VAR PRIVATE
				privateVar : INT := 100;
			END_VAR
		END_FUNCTION_BLOCK

		FUNCTION_BLOCK Derived EXTENDS Base
			METHOD AccessProtected : INT
				// This is allowed because protectedVar is PROTECTED
				THIS.protectedVar := THIS.protectedVar + 5;
				AccessProtected := THIS.protectedVar;
			END_METHOD

			METHOD AccessPrivate : INT
				// This should fail because privateVar is PRIVATE to Base
				AccessPrivate := THIS.privateVar;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	t.Run("Protected member access from derived class", func(t *testing.T) {
		input := `
		PROGRAM TestProtectedAccess
			VAR
				myDerived : Derived;
				result : INT;
			END_VAR
			result := myDerived.AccessProtected();
		END_PROGRAM
		` + baseProgram

		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env)
		progObj, _ := env.Get("TestProtectedAccess")
		prog := progObj.(*object.Program)
		Eval(prog.Body, prog.Env)

		// Check that the protected variable was successfully accessed and modified.
		// Initial value was 10, method adds 5.
		testIntegerObjectInEnv(t, prog.Env, "result", 15)
	})

	t.Run("Protected member access from outside", func(t *testing.T) {
		input := `
		PROGRAM TestProtectedExternalAccess
			VAR
				myDerived : Derived;
			END_VAR
			myDerived.protectedVar := 20; // This should fail
		END_PROGRAM
		` + baseProgram

		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env) // Define POUs

		progObj, _ := env.Get("TestProtectedExternalAccess")
		prog := progObj.(*object.Program)
		evaluated := Eval(prog.Body, prog.Env) // Execute the program body

		// This test assumes the evaluator has logic to produce this specific error.
		testErrorObjectContains(t, evaluated, "cannot assign to member variable 'protectedVar': member is protected")
	})

	t.Run("Private member access from derived class", func(t *testing.T) {
		input := `
		PROGRAM TestPrivateAccess
			VAR
				myDerived : Derived;
				res : INT;
			END_VAR
			res := myDerived.AccessPrivate(); // This method call should fail internally
		END_PROGRAM
		` + baseProgram

		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env) // Define POUs

		progObj, _ := env.Get("TestPrivateAccess")
		prog := progObj.(*object.Program)
		evaluated := Eval(prog.Body, prog.Env) // Execute the program body

		// This test assumes the evaluator has logic to produce this specific error.
		testErrorObjectContains(t, evaluated, "cannot access member variable 'privateVar': member is private and cannot be accessed from derived function block")
	})

	t.Run("Private member access from outside", func(t *testing.T) {
		input := `
		PROGRAM TestPrivateExternalAccess
			VAR
				myBase : Base;
			END_VAR
			myBase.privateVar := 200; // This should fail
		END_PROGRAM
		` + baseProgram

		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env) // Define POUs

		progObj, _ := env.Get("TestPrivateExternalAccess")
		prog := progObj.(*object.Program)
		evaluated := Eval(prog.Body, prog.Env) // Execute the program body

		// This test assumes the evaluator has logic to produce this specific error.
		testErrorObjectContains(t, evaluated, "cannot assign to member variable 'privateVar': member is private")
	})

	t.Run("Public member access from outside", func(t *testing.T) {
		program := `
		FUNCTION_BLOCK MyFB
			VAR PUBLIC
				publicVar : INT := 500;
			END_VAR
		END_FUNCTION_BLOCK

		PROGRAM TestPublicAccess
			VAR
				instance : MyFB;
				result : INT;
			END_VAR
			instance.publicVar := 25; // This should be allowed
			result := instance.publicVar;
		END_PROGRAM
		`
		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, program), env)
		progObj, _ := env.Get("TestPublicAccess")
		Eval(progObj.(*object.Program).Body, progObj.(*object.Program).Env)
		testIntegerObjectInEnv(t, progObj.(*object.Program).Env, "result", 25)
	})
}

func TestPropertyAccessorPermissions(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK AccessorTest
			VAR PRIVATE
				_internal : INT := 42;
			END_VAR

			PROPERTY Value : INT
				PUBLIC GET
					Value := THIS._internal;
				END_GET
				PRIVATE SET
					THIS._internal := value;
				END_SET
			END_PROPERTY

			METHOD InternalWrite : VOID
				VAR_INPUT NewValue : INT; END_VAR
				THIS.Value := NewValue; // This should be allowed
			END_METHOD
		END_FUNCTION_BLOCK
	`

	t.Run("Public GET is allowed from outside", func(t *testing.T) {
		input := `
		PROGRAM TestPublicGet
			VAR myFb : AccessorTest; result : INT; END_VAR
			result := myFb.Value;
		END_PROGRAM
		` + baseProgram
		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env)
		progObj, _ := env.Get("TestPublicGet")
		Eval(progObj.(*object.Program).Body, progObj.(*object.Program).Env)
		testIntegerObjectInEnv(t, progObj.(*object.Program).Env, "result", 42)
	})

	t.Run("Private SET is not allowed from outside", func(t *testing.T) {
		input := `
		PROGRAM TestPrivateSet
			VAR myFb : AccessorTest; END_VAR
			myFb.Value := 100;
		END_PROGRAM
		` + baseProgram // cspell:disable-line

		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env) // Define POUs

		progObj, _ := env.Get("TestPrivateSet")
		prog := progObj.(*object.Program)
		evaluated := Eval(prog.Body, prog.Env) // Execute the program body

		testErrorObjectContains(t, evaluated, "cannot access setter for property 'Value': member is private") // cspell:disable-line
	})

	t.Run("Private SET is allowed from inside", func(t *testing.T) {
		input := `
		PROGRAM TestInternalSet
			VAR myFb : AccessorTest; result : INT; END_VAR
			myFb.InternalWrite(NewValue := 99);
			result := myFb.Value;
		END_PROGRAM
		` + baseProgram // cspell:disable-line
		env := object.NewEnvironment()
		Eval(testParseProgramOOP(t, input), env) // Define POUs

		progObj, _ := env.Get("TestInternalSet")
		prog := progObj.(*object.Program)
		Eval(prog.Body, prog.Env) // Execute the program body

		testIntegerObjectInEnv(t, prog.Env, "result", 99)
	})
}

func TestNamespacedOOP(t *testing.T) {
	t.Run("Successful implementation across namespaces", func(t *testing.T) {
		input := `
			NAMESPACE MyLib
				INTERFACE IGreeter
					METHOD Greet : STRING;
				END_INTERFACE

				FUNCTION_BLOCK Greeter IMPLEMENTS IGreeter
					VAR_INPUT
						Greeting : STRING := 'Hello';
					END_VAR

					METHOD Greet : STRING
						Greet := THIS.Greeting;
					END_METHOD
				END_FUNCTION_BLOCK
			END_NAMESPACE

			PROGRAM TestNamespace
				VAR
					myGreeter : MyLib.Greeter;
					result : STRING;
				END_VAR

				myGreeter.Greeting := 'Bonjour';
				result := myGreeter.Greet();
			END_PROGRAM
		`

		env := object.NewEnvironment()
		// First call defines all POUs from the input string.
		testEvalWithEnv(t, input, env)

		// Second call executes the main program logic.
		testEvalWithEnv(t, "TestNamespace();", env)

		// Get the program's environment to check the result.
		progObj, ok := env.Get("TestNamespace")
		if !ok {
			t.Fatalf("Program 'TestNamespace' not found in environment")
		}
		progEnv := progObj.(*object.Program).Env

		testStringObjectInEnv(t, progEnv, "result", "Bonjour")
	})

	t.Run("Failed implementation of namespaced interface", func(t *testing.T) {
		input := `
			NAMESPACE MyLib
				INTERFACE IRunner
					METHOD Run : VOID;
				END_INTERFACE

				FUNCTION_BLOCK BadRunner IMPLEMENTS IRunner
					// Missing the 'Run' method
				END_FUNCTION_BLOCK
			END_NAMESPACE

			PROGRAM TestBadRunner
				VAR
					myRunner : MyLib.BadRunner;
				END_VAR
			END_PROGRAM
		`
		evaluated := testEval(t, input)
		testErrorObjectContains(t, evaluated, "function block 'BadRunner' does not implement method 'Run' from interface 'IRunner'")
	})
}

func testParseProgramOOP(t *testing.T, input string) *ast.Program {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, t.Name(), input)
	return program
}
