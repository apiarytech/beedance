package evaluator

import (
	"beedance/object"
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
	testEvalWithEnv(t, input, env) // Define POUs
	testEvalWithEnv(t, "TestOOP();", env)

	progObj, _ := env.Get("TestOOP")
	progEnv := progObj.(*object.Program).Env

	testIntegerObjectInEnv(t, progEnv, "res1", 2)
	testIntegerObjectInEnv(t, progEnv, "res2", 5)
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
	testEvalWithEnv(t, input, env) // Define POUs
	testEvalWithEnv(t, "TestOOP();", env)

	progObj, _ := env.Get("TestOOP")
	progEnv := progObj.(*object.Program).Env

	// Initial count is 5. Offset adds 10. So val1 should be 15.
	testIntegerObjectInEnv(t, progEnv, "val1", 15)
	// Increment adds 1. So val2 should be 16.
	testIntegerObjectInEnv(t, progEnv, "val2", 16)
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
	testEvalWithEnv(t, input, env) // Define POUs
	testEvalWithEnv(t, "TestOOP();", env)

	progObj, _ := env.Get("TestOOP")
	progEnv := progObj.(*object.Program).Env

	testIntegerObjectInEnv(t, progEnv, "res", 22)
}

func TestInterfaceAndAbstractEnforcement(t *testing.T) {
	baseProgram := `
		INTERFACE iMotor
			METHOD Start : BOOL;
			METHOD Stop : VOID;
			PROPERTY Speed : INT GET;
		END_INTERFACE

		ABSTRACT FUNCTION_BLOCK AbstractMotor IMPLEMENTS iMotor
			VAR PROTECTED
				currentSpeed : INT;
			END_VAR

			METHOD Start : BOOL ABSTRACT;

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
		input := baseProgram + `
			PROGRAM TestAbstract
				VAR
					myMotor : AbstractMotor;
				END_VAR
			END_PROGRAM
		`
		evaluated := testEval(t, input)
		testErrorObjectContains(t, evaluated, "cannot instantiate abstract function block 'AbstractMotor'")
	})

	t.Run("Cannot instantiate FB that does not implement interface", func(t *testing.T) {
		input := baseProgram + `
			PROGRAM TestBadMotor
				VAR
					myMotor : BadMotor;
				END_VAR
			END_PROGRAM
		`
		evaluated := testEval(t, input)
		testErrorObjectContains(t, evaluated, "function block 'BadMotor' does not implement method 'Stop' from interface 'iMotor'")
	})

	t.Run("Successfully instantiate and use concrete implementation", func(t *testing.T) {
		input := baseProgram + `
			PROGRAM TestConcrete
				VAR
					myMotor : ConcreteMotor;
					isStarted : BOOL;
					currentSpeed : INT;
				END_VAR

				isStarted := myMotor.Start();
				currentSpeed := myMotor.Speed;
			END_PROGRAM
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env) // Define POUs
		testEvalWithEnv(t, "TestConcrete();", env)

		progObj, _ := env.Get("TestConcrete")
		progEnv := progObj.(*object.Program).Env

		testBooleanObjectInEnv(t, progEnv, "isStarted", true)
		testIntegerObjectInEnv(t, progEnv, "currentSpeed", 100)

		// Now test the inherited 'Stop' method
		testEvalWithEnv(t, "myMotor.Stop();", progEnv)
		testEvalWithEnv(t, "currentSpeed := myMotor.Speed;", progEnv)
		testIntegerObjectInEnv(t, progEnv, "currentSpeed", 0)
	})
}
