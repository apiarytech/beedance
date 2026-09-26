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

import (
	"beedance/code"
	_ "beedance/stdlib"
	"testing"
)

func TestFunctionBlockWithMethod(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK MyFBWithMethod
				VAR
					Internalvar: INT := 1;
				END_VAR

				METHOD MyMethod : INT
					VAR_INPUT
						MethodIn : INT;
					END_VAR
					MyMethod := MethodIn * 2;
				END_METHOD

				THIS.Internalvar := THIS.MyMethod(THIS.Internalvar) + 1;
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				int64(2), // const[0] for the * 2 in the method
				[]code.Instructions{ // const[1] Method Body
					// THIS=0, MethodIn=1, MyMethod=2(ret)
					code.Make(code.OpNull),        // init return var
					code.Make(code.OpSetLocal, 2), // Set MyMethod (ret var)
					code.Make(code.OpGetLocal, 1), // Get MethodIn
					code.Make(code.OpConstant, 0), // Push 2
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
				},
				"MyMethod",    // const[2]
				"Internalvar", // const[3]
				int64(1),      // const[4] for the +1
				[]code.Instructions{ // const[5] Main Body
					// THIS=0
					// THIS.Internalvar := THIS.MyMethod(THIS.Internalvar) + 1;
					// RHS:
					code.Make(code.OpGetLocal, 0), // Get THIS for method call
					code.Make(code.OpConstant, 2), // "MyMethod"
					code.Make(code.OpIndex),       // Get method closure
					code.Make(code.OpGetLocal, 0), // Get THIS for argument
					code.Make(code.OpConstant, 3), // "Internalvar"
					code.Make(code.OpIndex),       // Get value of Internalvar
					code.Make(code.OpCall, 1),     // Call method
					code.Make(code.OpConstant, 4), // Push 1
					code.Make(code.OpAdd),         // Add
					// Assignment:
					code.Make(code.OpGetLocal, 0), // Get THIS for assignment
					code.Make(code.OpConstant, 3), // "Internalvar"
					code.Make(code.OpSetIndex),    // Set member
					code.Make(code.OpReturn),
				},
				"main", // const[6]
			},
			expectedInstructions: []code.Instructions{
				// The compiler sorts keys before creating the hash.
				// "MyMethod" comes before "main" alphabetically.
				code.Make(code.OpConstant, 2),   // "MyMethod"
				code.Make(code.OpClosure, 1, 0), // Method closure
				code.Make(code.OpConstant, 6),   // "main"
				code.Make(code.OpClosure, 5, 0), // Main body closure
				code.Make(code.OpHash, 4),       // Create hash with 2 key-value pairs
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFBWithMethod'
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionBlockThreeLevelInheritance(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK C
				METHOD GetValue : INT
					GetValue := 10;
				END_METHOD
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK B EXTENDS C
				METHOD GetValue : INT
					GetValue := SUPER^.GetValue() + 20;
				END_METHOD
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK A EXTENDS B
				METHOD GetValue : INT
					GetValue := SUPER^.GetValue() + 30;
				END_METHOD
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				// C's constants
				int64(10), // 0
				[]code.Instructions{ // 1: C.GetValue
					code.Make(code.OpNull),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{code.Make(code.OpReturn)}, // 2: C.main (reused by B and A)
				"main",     // 3
				"GetValue", // 4
				// B's constants
				int64(20), // 5
				[]code.Instructions{ // 6: B.GetValue
					code.Make(code.OpNull),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 4), // "GetValue"
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 5), // 20
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				"__parent__", // 7
				// A's constants
				int64(30), // 8
				[]code.Instructions{ // 9: A.GetValue
					code.Make(code.OpNull),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 4), // "GetValue"
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 8), // 30
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				// C compilation (main, then GetValue)
				code.Make(code.OpConstant, 4), code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpConstant, 3), code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpHash, 4), code.Make(code.OpSetGlobal, 0),
				// B compilation (__parent__, then sorted: GetValue, main)
				code.Make(code.OpConstant, 7), code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 4), code.Make(code.OpClosure, 6, 0),
				code.Make(code.OpConstant, 3), code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpHash, 6), code.Make(code.OpSetGlobal, 1),
				// A compilation (__parent__, then sorted: GetValue, main)
				code.Make(code.OpConstant, 7), code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpConstant, 4), code.Make(code.OpClosure, 9, 0),
				code.Make(code.OpConstant, 3), code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpHash, 6), code.Make(code.OpSetGlobal, 2),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionBlockPropertyOverride(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK BaseProp
				VAR
					_val : INT;
				END_VAR
				PROPERTY Value : INT
					GET
						Value := _val;
					END_GET
					SET
						_val := value;
					END_SET
				END_PROPERTY
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK DerivedProp EXTENDS BaseProp
				PROPERTY Value : INT
					GET
						Value := SUPER^.Value * 2;
					END_GET
					SET
						SUPER^.Value := value + 1;
					END_SET
				END_PROPERTY
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				// BaseProp constants
				"_val", // 0
				[]code.Instructions{ // 1: BaseProp.get_Value
					code.Make(code.OpNull), code.Make(code.OpSetLocal, 1), // Init ret var
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 0), // "_val"
					code.Make(code.OpIndex),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 2: BaseProp.set_Value
					code.Make(code.OpGetLocal, 1), // value
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 0), // "_val"
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				[]code.Instructions{code.Make(code.OpReturn)}, // 3: BaseProp.main
				"get_Value", // 4
				"main",      // 5
				"set_Value", // 6
				// DerivedProp constants
				int64(2), // 7
				[]code.Instructions{ // 8: DerivedProp.get_Value
					code.Make(code.OpNull), code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 4), // "get_Value"
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 7), // 2
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
				},
				int64(1), // 9
				[]code.Instructions{ // 10: DerivedProp.set_Value
					code.Make(code.OpGetLocal, 0), // THIS
					code.Make(code.OpConstant, 6), // "set_Value"
					code.Make(code.OpSuperIndex),
					code.Make(code.OpGetLocal, 1), // value
					code.Make(code.OpConstant, 9), // 1
					code.Make(code.OpAdd),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturn),
				},
				"__parent__", // 11
			},
			expectedInstructions: []code.Instructions{
				// BaseProp compilation (sorted: get_Value, main, set_Value)
				code.Make(code.OpConstant, 4), code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpConstant, 5), code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpConstant, 6), code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpHash, 6), code.Make(code.OpSetGlobal, 0),
				// DerivedProp compilation (__parent__, then sorted: get_Value, main, set_Value)
				// Note: The test failure indicates a bug causing an off-by-one error in constants.
				// This test expectation is based on the intended correct output.
				code.Make(code.OpConstant, 11), code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 4), code.Make(code.OpClosure, 8, 0),
				code.Make(code.OpConstant, 5), code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpConstant, 6), code.Make(code.OpClosure, 10, 0),
				code.Make(code.OpHash, 8), code.Make(code.OpSetGlobal, 1),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionBlockWithMixedVars(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK MyFBWithMixedVars
				VAR_INPUT
					InVar: INT;
				END_VAR
				VAR_OUTPUT
					OutVar: INT;
				END_VAR
				VAR
					InstanceVar: INT;
				END_VAR

				// Access instance var (implicitly THIS.InstanceVar), input var, and output var
				InstanceVar := InstanceVar + InVar;
				OutVar := InstanceVar;
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				"InstanceVar", // const[0]
				[]code.Instructions{ // const[1] Main Body
					// THIS=0, InVar=1, OutVar=2
					// InstanceVar := InstanceVar + InVar;
					// RHS:
					code.Make(code.OpGetLocal, 0), // Get THIS (for implicit InstanceVar)
					code.Make(code.OpConstant, 0), // "InstanceVar"
					code.Make(code.OpIndex),       // Get value of InstanceVar
					code.Make(code.OpGetLocal, 1), // Get InVar
					code.Make(code.OpAdd),         // Add
					// Assignment:
					code.Make(code.OpGetLocal, 0), // Get THIS for assignment
					code.Make(code.OpConstant, 0), // "InstanceVar"
					code.Make(code.OpSetIndex),    // Set member

					// OutVar := InstanceVar;
					// RHS:
					code.Make(code.OpGetLocal, 0), // Get THIS (for implicit InstanceVar)
					code.Make(code.OpConstant, 0), // "InstanceVar"
					code.Make(code.OpIndex),       // Get value of InstanceVar
					// Assignment:
					code.Make(code.OpSetLocal, 2), // Set OutVar

					code.Make(code.OpReturn),
				},
				"main", // const[2]
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 2),   // "main"
				code.Make(code.OpClosure, 1, 0), // Main body closure
				code.Make(code.OpHash, 2),       // Create hash { "main": <closure> }
				code.Make(code.OpSetGlobal, 0),  // Set the global 'MyFBWithMixedVars'
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionBlockInheritanceAndSuper(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK BaseFB
				METHOD DoSomething : INT
					DoSomething := 10;
				END_METHOD
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK DerivedFB EXTENDS BaseFB
				METHOD DoSomething : INT
					DoSomething := SUPER^.DoSomething() + 5;
				END_METHOD
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				// BaseFB constants
				int64(10), // 0
				[]code.Instructions{ // 1: BaseFB.DoSomething
					code.Make(code.OpNull),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{code.Make(code.OpReturn)}, // 2: BaseFB.main (reused)
				"main",        // 3
				"DoSomething", // 4
				// DerivedFB constants
				int64(5), // 5
				[]code.Instructions{ // 6: DerivedFB.DoSomething
					code.Make(code.OpNull),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0), // Get THIS for SUPER call
					code.Make(code.OpConstant, 4), // "DoSomething"
					code.Make(code.OpSuperIndex),  // Get parent method
					code.Make(code.OpCall, 0),     // Call SUPER^.DoSomething()
					code.Make(code.OpConstant, 5), // 5
					code.Make(code.OpAdd),         // +
					code.Make(code.OpReturnValue), // Return the result
				},
				"__parent__", // 7
			},
			expectedInstructions: []code.Instructions{
				// BaseFB compilation (sorted: DoSomething, main)
				code.Make(code.OpConstant, 4),   // "DoSomething"
				code.Make(code.OpClosure, 1, 0), // BaseFB.DoSomething
				code.Make(code.OpConstant, 3),   // "main"
				code.Make(code.OpClosure, 2, 0), // BaseFB.main
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 0),
				// DerivedFB compilation (__parent__, then sorted: DoSomething, main)
				code.Make(code.OpConstant, 7),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 4),   // "DoSomething"
				code.Make(code.OpClosure, 6, 0), // DerivedFB.DoSomething
				code.Make(code.OpConstant, 3),   // "main"
				code.Make(code.OpClosure, 2, 0), // main (reused)
				code.Make(code.OpHash, 6),
				code.Make(code.OpSetGlobal, 1),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionBlockProperty(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION_BLOCK MyFBWithProperty
				VAR
					internalvar : INT;
				END_VAR

				PROPERTY MyProp : INT
					GET
						MyProp := THIS.internalvar * 2;
					END_GET
					SET
						THIS.internalvar := value;
					END_SET
				END_PROPERTY
			END_FUNCTION_BLOCK
			`,
			expectedConstants: []interface{}{
				"internalvar", // const[0] (global)
				int64(2),      // const[1] (global, discovered during GET compilation)
				[]code.Instructions{ // const[2] GET body (global)
					// THIS=0, MyProp=1(ret)
					code.Make(code.OpNull),        // init return var
					code.Make(code.OpSetLocal, 1), //
					code.Make(code.OpGetLocal, 0), // Get THIS
					code.Make(code.OpConstant, 0), // "internalvar"
					code.Make(code.OpIndex),       // Get internal var
					code.Make(code.OpConstant, 1), // The constant '2'
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // const[3] SET body (global)
					// THIS=0, value=1
					code.Make(code.OpGetLocal, 1), // Get value
					code.Make(code.OpGetLocal, 0), // Get THIS
					code.Make(code.OpConstant, 0), // "internalvar"
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				[]code.Instructions{ // const[4] Main body (empty) (global)
					code.Make(code.OpReturn),
				},
				"main",       // const[5] (global)
				"get_MyProp", // const[6] (global)
				"set_MyProp", // const[7] (global)
			},
			expectedInstructions: []code.Instructions{
				// The order of hash elements is now deterministic due to sorting keys.
				// Alphabetical order: get_MyProp, main, set_MyProp
				code.Make(code.OpConstant, 6),   // "get_MyProp"
				code.Make(code.OpClosure, 2, 0), // Getter closure
				code.Make(code.OpConstant, 5),   // "main"
				code.Make(code.OpClosure, 4, 0), // Main body closure
				code.Make(code.OpConstant, 7),   // "set_MyProp"
				code.Make(code.OpClosure, 3, 0), // Setter closure
				code.Make(code.OpHash, 6),       // Create hash with 3 key-value pairs
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestAccessSpecifierErrors(t *testing.T) {
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
			METHOD AccessPrivate : INT
				// This should fail because privateVar is PRIVATE to Base
				AccessPrivate := THIS.privateVar;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Protected member access from outside",
			input: `
				PROGRAM TestProtectedExternal
					VAR myBase : Base; END_VAR
					myBase.protectedVar := 20;
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot assign to member variable 'protectedVar': member is protected",
		},
		{
			name: "Private member access from derived class",
			input: `
				PROGRAM TestPrivateDerived
					VAR myDerived : Derived; END_VAR
					myDerived.AccessPrivate();
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot access member variable 'privateVar': member is private",
		},
		{
			name: "Private member access from outside",
			input: `
				PROGRAM TestPrivateExternal
					VAR myBase : Base; END_VAR
					myBase.privateVar := 200;
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot assign to member variable 'privateVar': member is private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func TestFinalKeywordErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Extend FINAL function block",
			input: `
				FUNCTION_BLOCK FINAL BaseFinal END_FUNCTION_BLOCK
				FUNCTION_BLOCK Derived EXTENDS BaseFinal END_FUNCTION_BLOCK
			`,
			expectedError: "cannot extend from FINAL function block 'BaseFinal'",
		},
		{
			name: "Override FINAL method",
			input: `
				FUNCTION_BLOCK BaseWithFinalMethod
					METHOD FINAL MyMethod : INT
						MyMethod := 1;
					END_METHOD
				END_FUNCTION_BLOCK

				FUNCTION_BLOCK DerivedOverridingFinal EXTENDS BaseWithFinalMethod
					METHOD MyMethod : INT
						MyMethod := 2;
					END_METHOD
				END_FUNCTION_BLOCK
			`,
			expectedError: "cannot override FINAL method 'MyMethod' from function block 'BaseWithFinalMethod'",
		},
		{
			name: "Override FINAL property",
			input: `
				FUNCTION_BLOCK BaseWithFinalProperty
					PROPERTY FINAL MyProp : INT
						GET
							MyProp := 1;
						END_GET
					END_PROPERTY
				END_FUNCTION_BLOCK

				FUNCTION_BLOCK DerivedOverridingFinalProp EXTENDS BaseWithFinalProperty
					PROPERTY MyProp : INT
						GET
							MyProp := 2;
						END_GET
					END_PROPERTY
				END_FUNCTION_BLOCK
			`,
			expectedError: "cannot override FINAL property 'MyProp' from function block 'BaseWithFinalProperty'",
		},
		{
			name: "Override FINAL variable",
			input: `
				FUNCTION_BLOCK BaseWithFinalVar
					VAR FINAL
						MyFinalVar : INT := 1;
					END_VAR
				END_FUNCTION_BLOCK

				FUNCTION_BLOCK DerivedOverridingFinalVar EXTENDS BaseWithFinalVar
					VAR
						MyFinalVar : INT := 2; // This should be an error
					END_VAR
				END_FUNCTION_BLOCK
			`,
			expectedError: "cannot override FINAL variable 'MyFinalVar' from function block 'BaseWithFinalVar'",
		},
		{
			name: "Extend FINAL interface",
			input: `
				INTERFACE FINAL IBaseFinal END_INTERFACE
				INTERFACE IDerived EXTENDS IBaseFinal END_INTERFACE
			`,
			expectedError: "cannot extend from FINAL interface 'IBaseFinal'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func TestOverridingMethodSignatureErrors(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK BaseForSignature
			METHOD MyMethod : INT
				VAR_INPUT
					p1: INT;
					p2: BOOL;
				END_VAR
				MyMethod := 1;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Mismatched return type",
			input: `
				FUNCTION_BLOCK DerivedMismatchReturn EXTENDS BaseForSignature
					METHOD MyMethod : REAL // Should be INT
						VAR_INPUT
							p1: INT;
							p2: BOOL;
						END_VAR
						MyMethod := 1.0;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "return type mismatch for method 'MyMethod': derived is 'REAL', parent is 'INT'",
		},
		{
			name: "Mismatched parameter count",
			input: `
				FUNCTION_BLOCK DerivedMismatchParamCount EXTENDS BaseForSignature
					METHOD MyMethod : INT
						VAR_INPUT
							p1: INT; // Missing p2
						END_VAR
						MyMethod := 1;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "parameter count mismatch for method 'MyMethod': derived has 1, parent has 2",
		},
		{
			name: "Mismatched parameter type",
			input: `
				FUNCTION_BLOCK DerivedMismatchParamType EXTENDS BaseForSignature
					METHOD MyMethod : INT
						VAR_INPUT
							p1: INT;
							p2: STRING; // Should be BOOL
						END_VAR
						MyMethod := 1;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "parameter type mismatch for method 'MyMethod' at index 1: derived is 'STRING', parent is 'BOOL'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func checkCompilerError(t *testing.T, err error, expectedMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected compiler error but got none")
	}
	if err.Error() != expectedMessage {
		t.Fatalf("wrong compiler error. want=%q, got=%q", expectedMessage, err.Error())
	}
}

func TestCovariantReturnTypes(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK Animal END_FUNCTION_BLOCK
		FUNCTION_BLOCK Dog EXTENDS Animal END_FUNCTION_BLOCK
		FUNCTION_BLOCK Cat EXTENDS Animal END_FUNCTION_BLOCK
        FUNCTION_BLOCK Car END_FUNCTION_BLOCK

		FUNCTION_BLOCK AnimalFactory
			METHOD GetAnAnimal : Animal
				VAR myAnimal : Animal; END_VAR
				GetAnAnimal := myAnimal;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Valid covariant return type (direct subclass)",
			input: `
				FUNCTION_BLOCK DogFactory EXTENDS AnimalFactory
					METHOD GetAnAnimal : Dog // Valid: Dog is a subclass of Animal
						VAR myDog : Dog; END_VAR
						GetAnAnimal := myDog;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "", // No error
		},
		{
			name: "Invalid covariant return type (unrelated class)",
			input: `
				FUNCTION_BLOCK CarFactory EXTENDS AnimalFactory
					METHOD GetAnAnimal : Car // Invalid: Car is not a subclass of Animal
						VAR myCar : Car; END_VAR
						GetAnAnimal := myCar;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "incompatible return types for method 'GetAnAnimal': 'Car' is not a subclass of 'Animal'",
		},
		{
			name: "Invalid covariant return type (primitive instead of class)",
			input: `
				FUNCTION_BLOCK IntFactory EXTENDS AnimalFactory
					METHOD GetAnAnimal : INT // Invalid: INT is not a class
						GetAnAnimal := 1;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "return type mismatch for method 'GetAnAnimal': derived is 'INT', parent is 'Animal'",
		},
		{
			name: "Invalid covariant return type (class instead of primitive)",
			input: `
                FUNCTION_BLOCK BaseWithInt
                    METHOD GetValue : INT
                        GetValue := 1;
                    END_METHOD
                END_FUNCTION_BLOCK

				FUNCTION_BLOCK DerivedWithAnimal EXTENDS BaseWithInt
					METHOD GetValue : Animal // Invalid: Animal is not a subclass of INT
                        VAR a : Animal; END_VAR
						GetValue := a;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "return type mismatch for method 'GetValue': derived is 'Animal', parent is 'INT'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			if tt.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %s", err)
				}
			} else {
				checkCompilerError(t, err, tt.expectedError)
			}
		})
	}
}

func TestAbstractMemberErrors(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK ABSTRACT AbstractBase
			METHOD ABSTRACT DoSomething : INT
			END_METHOD

			PROPERTY ABSTRACT Value : INT
			END_PROPERTY
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Instantiate abstract function block",
			input: `
				PROGRAM TestAbstractInstantiation
					VAR myMotor : AbstractBase; END_VAR
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot instantiate abstract function block 'AbstractBase'",
		},
		{
			name: "Concrete class missing abstract method implementation",
			input: `
				FUNCTION_BLOCK ConcreteFB EXTENDS AbstractBase
					// Missing implementation for DoSomething
					PROPERTY Value : INT
						GET
							Value := 1;
						END_GET
					END_PROPERTY
				END_FUNCTION_BLOCK
			`,
			expectedError: "function block 'ConcreteFB' must implement abstract method 'DoSomething'",
		},
		{
			name: "Concrete class missing abstract property implementation",
			input: `
				FUNCTION_BLOCK ConcreteFB EXTENDS AbstractBase
					METHOD DoSomething : INT
						DoSomething := 1;
					END_METHOD
					// Missing implementation for Value property
				END_FUNCTION_BLOCK
			`,
			expectedError: "function block 'ConcreteFB' must implement abstract property 'Value'",
		},
		{
			name: "Concrete class implements with another abstract method",
			input: `
				FUNCTION_BLOCK ConcreteFB EXTENDS AbstractBase
					METHOD ABSTRACT DoSomething : INT // Still abstract
					END_METHOD
				END_FUNCTION_BLOCK
			`,
			expectedError: "function block 'ConcreteFB' must implement abstract method 'DoSomething'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func TestContravariantParameterTypes(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK Animal END_FUNCTION_BLOCK
		FUNCTION_BLOCK Dog EXTENDS Animal END_FUNCTION_BLOCK
		FUNCTION_BLOCK Poodle EXTENDS Dog END_FUNCTION_BLOCK
        FUNCTION_BLOCK Car END_FUNCTION_BLOCK

		FUNCTION_BLOCK BaseProcessor
			METHOD Process : VOID
                VAR_INPUT
                    item: Dog;
                END_VAR
			END_METHOD
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Valid contravariant parameter type (superclass)",
			input: `
				FUNCTION_BLOCK DerivedProcessor EXTENDS BaseProcessor
					METHOD Process : VOID
						VAR_INPUT
							item: Animal; // Valid: Animal is a superclass of Dog
						END_VAR
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "", // No error
		},
		{
			name: "Invalid contravariant parameter type (unrelated class)",
			input: `
				FUNCTION_BLOCK DerivedProcessor EXTENDS BaseProcessor
					METHOD Process : VOID
						VAR_INPUT
							item: Car; // Invalid: Car is not a superclass of Dog
						END_VAR
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "incompatible parameter type for method 'Process' at index 0: 'Car' is not a superclass of 'Dog'",
		},
		{
			name: "Invalid covariant parameter type (subclass)",
			input: `
				FUNCTION_BLOCK DerivedProcessor EXTENDS BaseProcessor
					METHOD Process : VOID
						VAR_INPUT
							item: Poodle; // Invalid: Poodle is a subclass of Dog, not a superclass
						END_VAR
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "incompatible parameter type for method 'Process' at index 0: 'Poodle' is not a superclass of 'Dog'",
		},
		{
			name: "Invalid parameter type (primitive instead of class)",
			input: `
				FUNCTION_BLOCK DerivedProcessor EXTENDS BaseProcessor
					METHOD Process : VOID
						VAR_INPUT
							item: INT; // Invalid: INT is not a class
						END_VAR
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "parameter type mismatch for method 'Process' at index 0: derived is 'INT', parent is 'Dog'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			if tt.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %s", err)
				}
			} else {
				checkCompilerError(t, err, tt.expectedError)
			}
		})
	}
}

func TestInterfaceInheritance(t *testing.T) {
	// This test assumes the parser is updated to support `EXTENDS` on INTERFACE declarations.
	baseProgram := `
		INTERFACE IBase
			METHOD GetNumber : INT
			END_METHOD
		END_INTERFACE

		INTERFACE IDerived EXTENDS IBase
			METHOD GetBoolean : BOOL
			END_METHOD
		END_INTERFACE

		INTERFACE IAnotherBase
			PROPERTY Name : STRING
			END_PROPERTY
		END_INTERFACE

		INTERFACE IMultiDerived EXTENDS IDerived, IAnotherBase
		END_INTERFACE
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Valid implementation of multiply-derived interface",
			input: `
				FUNCTION_BLOCK MyFB IMPLEMENTS IMultiDerived
					METHOD GetNumber : INT
						GetNumber := 1;
					END_METHOD
					METHOD GetBoolean : BOOL
						GetBoolean := TRUE;
					END_METHOD
					PROPERTY Name : STRING
						GET
							Name := 'test';
						END_GET
					END_PROPERTY
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "", // No error
		},
		{
			name: "Missing implementation from base interface",
			input: `
				FUNCTION_BLOCK MyFB IMPLEMENTS IDerived
					// Missing GetNumber from IBase
					METHOD GetBoolean : BOOL
						GetBoolean := TRUE;
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "function block 'MyFB' does not implement method 'GetNumber' from interface 'IDerived'",
		},
		{
			name: "Missing implementation from another base interface",
			input: `
				FUNCTION_BLOCK MyFB IMPLEMENTS IMultiDerived
					METHOD GetNumber : INT
						GetNumber := 1;
					END_METHOD
					METHOD GetBoolean : BOOL
						GetBoolean := TRUE;
					END_METHOD
					// Missing property Name from IAnotherBase
				END_FUNCTION_BLOCK
			` + baseProgram,
			expectedError: "function block 'MyFB' does not implement property 'Name' from interface 'IMultiDerived'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			if tt.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %s", err)
				}
			} else {
				checkCompilerError(t, err, tt.expectedError)
			}
		})
	}
}

func TestCircularInterfaceInheritanceError(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Direct circular inheritance",
			input: `
				INTERFACE A EXTENDS B END_INTERFACE
				INTERFACE B EXTENDS A END_INTERFACE
				FUNCTION_BLOCK MyFB IMPLEMENTS A END_FUNCTION_BLOCK
			`,
			expectedError: "circular interface inheritance detected: 'A' is already in the inheritance path",
		},
		{
			name: "Indirect circular inheritance",
			input: `
				INTERFACE A EXTENDS B END_INTERFACE
				INTERFACE B EXTENDS C END_INTERFACE
				INTERFACE C EXTENDS A END_INTERFACE
				FUNCTION_BLOCK MyFB IMPLEMENTS C END_FUNCTION_BLOCK
			`,
			expectedError: "circular interface inheritance detected: 'C' is already in the inheritance path",
		},
		{
			name: "Self inheritance",
			input: `
				INTERFACE A EXTENDS A END_INTERFACE
				FUNCTION_BLOCK MyFB IMPLEMENTS A END_FUNCTION_BLOCK
			`,
			expectedError: "circular interface inheritance detected: 'A' is already in the inheritance path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func TestCircularFunctionBlockInheritanceError(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Direct circular inheritance",
			input: `
				FUNCTION_BLOCK A EXTENDS B END_FUNCTION_BLOCK
				FUNCTION_BLOCK B EXTENDS A END_FUNCTION_BLOCK
			`,
			expectedError: "circular function block inheritance detected: 'A' is already in the inheritance path",
		},
		{
			name: "Indirect circular inheritance",
			input: `
				FUNCTION_BLOCK A EXTENDS B END_FUNCTION_BLOCK
				FUNCTION_BLOCK B EXTENDS C END_FUNCTION_BLOCK
				FUNCTION_BLOCK C EXTENDS A END_FUNCTION_BLOCK
			`,
			expectedError: "circular function block inheritance detected: 'A' is already in the inheritance path",
		},
		{
			name: "Self inheritance",
			input: `
				FUNCTION_BLOCK A EXTENDS A END_FUNCTION_BLOCK
			`,
			expectedError: "circular function block inheritance detected: 'A' is already in the inheritance path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func TestMethodAccessSpecifierErrors(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK BaseWithPrivateMethod
			VAR PUBLIC
				publicVar : INT;
			END_VAR

			PRIVATE METHOD MyPrivateMethod : INT
				MyPrivateMethod := 1;
			END_METHOD

			PUBLIC METHOD MyPublicMethod : INT
				// This is a valid call from within the same FB
				MyPublicMethod := THIS.MyPrivateMethod();
			END_METHOD
		END_FUNCTION_BLOCK

		FUNCTION_BLOCK DerivedFromBase EXTENDS BaseWithPrivateMethod
			PUBLIC METHOD CallBasesPrivateMethod : INT
				// This should fail because MyPrivateMethod is private to BaseWithPrivateMethod
				CallBasesPrivateMethod := SUPER^.MyPrivateMethod();
			END_METHOD
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Access private method from outside",
			input: `
				PROGRAM TestPrivateExternal
					VAR myBase : BaseWithPrivateMethod; END_VAR
					myBase.MyPrivateMethod();
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot access method 'MyPrivateMethod': member is private",
		},
		{
			name: "Access private method from derived class via SUPER",
			input: `
				PROGRAM TestPrivateDerived
					VAR myDerived : DerivedFromBase; END_VAR
					myDerived.CallBasesPrivateMethod();
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot access method 'MyPrivateMethod': member is private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			checkCompilerError(t, err, tt.expectedError)
		})
	}
}

func TestProtectedMemberAccess(t *testing.T) {
	baseProgram := `
		FUNCTION_BLOCK BaseWithProtected
			PROTECTED METHOD MyProtectedMethod : INT
				MyProtectedMethod := 10;
			END_METHOD

			PROTECTED PROPERTY MyProtectedProp : INT
				GET
					MyProtectedProp := 20;
				END_GET
			END_PROPERTY
		END_FUNCTION_BLOCK

		FUNCTION_BLOCK DerivedFromProtected EXTENDS BaseWithProtected
			PUBLIC METHOD AccessProtectedMembers : INT
				VAR
					a : INT;
					b : INT;
				END_VAR
				// Valid access from a derived class
				a := THIS.MyProtectedMethod();
				b := THIS.MyProtectedProp;
				AccessProtectedMembers := a + b;
			END_METHOD
		END_FUNCTION_BLOCK
	`

	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Access protected members from derived class",
			input: `
				PROGRAM TestProtectedDerived
					VAR myDerived : DerivedFromProtected; result : INT; END_VAR
					result := myDerived.AccessProtectedMembers();
				END_PROGRAM
			` + baseProgram,
			expectedError: "", // No error
		},
		{
			name: "Access protected property from outside",
			input: `
				PROGRAM TestProtectedExternalProperty
					VAR myBase : BaseWithProtected; result : INT; END_VAR
					result := myBase.MyProtectedProp;
				END_PROGRAM
			` + baseProgram,
			expectedError: "cannot access getter for property 'MyProtectedProp': member is protected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			if tt.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %s", err)
				}
			} else {
				checkCompilerError(t, err, tt.expectedError)
			}
		})
	}
}

func TestInternalAccessSpecifiers(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Valid internal FB instantiation in same namespace",
			input: `
				NAMESPACE MyLib
					INTERNAL FUNCTION_BLOCK InternalFB END_FUNCTION_BLOCK
					PROGRAM MyProg
						VAR myFb: InternalFB; END_VAR // OK: Same namespace
					END_PROGRAM
				END_NAMESPACE
			`,
			expectedError: "",
		},
		{
			name: "Invalid internal FB instantiation from global scope",
			input: `
				NAMESPACE MyLib
					INTERNAL FUNCTION_BLOCK InternalFB END_FUNCTION_BLOCK
				END_NAMESPACE

				PROGRAM MyProg
					VAR myFb: InternalFB; END_VAR // ERROR: Global scope cannot access internal
				END_PROGRAM
			`,
			expectedError: "cannot access INTERNAL function block 'InternalFB' from a different namespace",
		},
		{
			name: "Valid internal method call in same namespace",
			input: `
				NAMESPACE MyLib
					FUNCTION_BLOCK FBWithInternal
						INTERNAL METHOD MyInternalMethod : INT
							MyInternalMethod := 1;
						END_METHOD
					END_FUNCTION_BLOCK

					PROGRAM MyProg
						VAR fb: FBWithInternal; res: INT; END_VAR
						res := fb.MyInternalMethod(); // OK: Same namespace
					END_PROGRAM
				END_NAMESPACE
			`,
			expectedError: "",
		},
		{
			name: "Invalid internal method call from global scope",
			input: `
				NAMESPACE MyLib
					FUNCTION_BLOCK FBWithInternal
						INTERNAL METHOD MyInternalMethod : INT
							MyInternalMethod := 1;
						END_METHOD
					END_FUNCTION_BLOCK
				END_NAMESPACE

				PROGRAM MyProg
					VAR fb: FBWithInternal; res: INT; END_VAR
					res := fb.MyInternalMethod(); // ERROR: Global cannot access internal
				END_PROGRAM
			`,
			expectedError: "cannot access method 'MyInternalMethod': member is internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := New()
			err := compiler.Compile(parse(tt.input))
			if tt.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %s", err)
				}
			} else {
				checkCompilerError(t, err, tt.expectedError)
			}
		})
	}
}

func TestQualifiedNameResolution(t *testing.T) {
	input := `
		NAMESPACE MyCompany.MyLibrary
			FUNCTION_BLOCK MyFB
				VAR
					x : INT;
				END_VAR
			END_FUNCTION_BLOCK
		END_NAMESPACE

		PROGRAM MyMain
			VAR
				instance1 : MyCompany.MyLibrary.MyFB;
			END_VAR
			instance1.x := 10;
		END_PROGRAM
	`
	compiler := New()
	err := compiler.Compile(parse(input))
	if err != nil {
		t.Fatalf("Compiler failed with qualified name resolution: %s", err)
	}
	// A successful compilation (err == nil) is sufficient to validate
	// that the type was resolved correctly.
}
