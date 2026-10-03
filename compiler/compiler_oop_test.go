package compiler

import (
	"github.com/apiarytech/beedance/code"
	_ "github.com/apiarytech/beedance/stdlib"
	"strings"
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
			expectedConstants: []interface{}{0, 2, []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 2),
				code.Make(code.OpGetLocal, 1),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpMul),
				code.Make(code.OpSetLocal, 2),
				code.Make(code.OpGetLocal, 2),
				code.Make(code.OpReturnValue),
			}, "Internalvar", "MyMethod", 1, []code.Instructions{
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpIndex),
				code.Make(code.OpCall, 1),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpAdd),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			}, "main"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpClosure, 6, 0),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 0),
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
				0,  // 0
				10, // 1
				[]code.Instructions{ // 2
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 3
					code.Make(code.OpReturn),
				},
				"GetValue", // 4
				"main",     // 5
				20,         // 6
				[]code.Instructions{ // 7
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 4),
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 6),
					code.Make(code.OpAdd),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 8
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpConstant, 5),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpPop),
					code.Make(code.OpReturn),
				},
				"__parent__", // 9
				30,           // 10
				[]code.Instructions{ // 11
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 4),
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 10),
					code.Make(code.OpAdd),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 12
					code.Make(code.OpGetGlobal, 1),
					code.Make(code.OpConstant, 5),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpPop),
					code.Make(code.OpReturn),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 9),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 7, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 8, 0),
				code.Make(code.OpHash, 6),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpConstant, 9),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 11, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 12, 0),
				code.Make(code.OpHash, 6),
				code.Make(code.OpSetGlobal, 2),
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
				0,      // 0
				"_val", // 1
				[]code.Instructions{ // 2
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpIndex),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 3
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				[]code.Instructions{ // 4
					code.Make(code.OpReturn),
				},
				"get_Value", // 5
				"main",      // 6
				"set_Value", // 7
				2,           // 8
				[]code.Instructions{ // 9
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 5),
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 8),
					code.Make(code.OpMul),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				1, // 10
				[]code.Instructions{ // 11
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 7),
					code.Make(code.OpSuperIndex),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpConstant, 10),
					code.Make(code.OpAdd),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturn),
				},
				[]code.Instructions{ // 12
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpConstant, 6),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpPop),
					code.Make(code.OpReturn),
				},
				"__parent__", // 13
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpClosure, 4, 0),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 6),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 13),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 9, 0),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpClosure, 12, 0),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpClosure, 11, 0),
				code.Make(code.OpHash, 8),
				code.Make(code.OpSetGlobal, 1),
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
				"InVar",       // const[1]
				"OutVar",      // const[2]
				[]code.Instructions{ // const[3] Main Body
					// THIS=0. Inputs, outputs and VARs are all fields of the
					// instance, so they keep their values between calls.
					// InstanceVar := InstanceVar + InVar;
					// Assignment:
					code.Make(code.OpGetLocal, 0), // Get THIS for assignment
					code.Make(code.OpConstant, 0), // "InstanceVar"
					// RHS:
					code.Make(code.OpGetLocal, 0), // Get THIS (for implicit InstanceVar)
					code.Make(code.OpConstant, 0), // "InstanceVar"
					code.Make(code.OpIndex),       // Get value of InstanceVar
					code.Make(code.OpGetLocal, 0), // Get THIS (for implicit InVar)
					code.Make(code.OpConstant, 1), // "InVar"
					code.Make(code.OpIndex),       // Get value of InVar
					code.Make(code.OpAdd),         // Add
					code.Make(code.OpSetIndex),    // Set member

					// OutVar := InstanceVar;
					code.Make(code.OpGetLocal, 0), // Get THIS for assignment
					code.Make(code.OpConstant, 2), // "OutVar"
					code.Make(code.OpGetLocal, 0), // Get THIS (for implicit InstanceVar)
					code.Make(code.OpConstant, 0), // "InstanceVar"
					code.Make(code.OpIndex),       // Get value of InstanceVar
					code.Make(code.OpSetIndex),    // Set OutVar

					code.Make(code.OpReturn),
				},
				"main", // const[4]
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),   // "main"
				code.Make(code.OpClosure, 3, 0), // Main body closure
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
			name: "Override FINAL method",
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
				0,  // 0
				10, // 1
				[]code.Instructions{ // 2
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 3
					code.Make(code.OpReturn),
				},
				"DoSomething", // 4
				"main",        // 5
				5,             // 6
				[]code.Instructions{ // 7
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 4),
					code.Make(code.OpSuperIndex),
					code.Make(code.OpCall, 0),
					code.Make(code.OpConstant, 6),
					code.Make(code.OpAdd),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{ // 8
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpConstant, 5),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpPop),
					code.Make(code.OpReturn),
				},
				"__parent__", // 9
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 9),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 7, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 8, 0),
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
			expectedConstants: []interface{}{0, "internalvar", 2, []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 1),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpMul),
				code.Make(code.OpSetLocal, 1),
				code.Make(code.OpGetLocal, 1),
				code.Make(code.OpReturnValue),
			}, []code.Instructions{
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGetLocal, 1),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			}, []code.Instructions{
				code.Make(code.OpReturn),
			}, "get_MyProp", "main", "set_MyProp"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 6),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpClosure, 5, 0),
				code.Make(code.OpConstant, 8),
				code.Make(code.OpClosure, 4, 0),
				code.Make(code.OpHash, 6),
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
	`

	// derivedProgram reads Base's PRIVATE variable. It is only added to the case that
	// tests that error, so every other case contains exactly one error.
	derivedProgram := `
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
			` + baseProgram + derivedProgram,
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
			err := compiler.Compile(parse(t, tt.input))
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
			FUNCTION_BLOCK BaseFinal
				METHOD FINAL DoSomething : INT
					DoSomething := 10;
				END_METHOD
			END_FUNCTION_BLOCK

			FUNCTION_BLOCK DerivedFinal EXTENDS BaseFinal
				METHOD DoSomething : INT
					DoSomething := 20; // This method should be ignored
				END_METHOD
			END_FUNCTION_BLOCK
			`,
			expectedError: "cannot override FINAL Method 'DoSomething' from function block 'BaseFinal'",
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
		// expectParseError marks input the parser itself must reject.
		expectParseError bool
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
			` + baseProgram,
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
			` + baseProgram,
			expectedError: "function block 'ConcreteFB' must implement abstract property 'Value'",
		},
		{
			name: "Concrete class implements with another abstract method",
			input: `
				FUNCTION_BLOCK ConcreteFB EXTENDS AbstractBase
					METHOD ABSTRACT DoSomething : INT // Still abstract
					END_METHOD
				END_FUNCTION_BLOCK
			` + baseProgram,
			// The parser rejects abstract members in a concrete FB before the compiler runs.
			expectParseError: true,
			expectedError:    "abstract members are not allowed in a non-abstract function block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectParseError {
				_, errs := parseWithErrors(tt.input)
				if len(errs) == 0 || !strings.HasPrefix(errs[0], tt.expectedError) {
					t.Fatalf("expected parser error %q, got %v", tt.expectedError, errs)
				}
				return
			}
			compiler := New()
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
			err := compiler.Compile(parse(t, tt.input))
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
					// PROGRAM is not a valid namespace element in IEC 61131-3, so a FUNCTION is used.
					FUNCTION MyFunc : INT
						VAR myFb: InternalFB; END_VAR // OK: Same namespace
						MyFunc := 0;
					END_FUNCTION
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

					// PROGRAM is not a valid namespace element in IEC 61131-3, so a FUNCTION is used.
					FUNCTION MyFunc : INT
						VAR fb: FBWithInternal; END_VAR
						MyFunc := fb.MyInternalMethod(); // OK: Same namespace
					END_FUNCTION
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
			err := compiler.Compile(parse(t, tt.input))
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
	err := compiler.Compile(parse(t, input))
	if err != nil {
		t.Fatalf("Compiler failed with qualified name resolution: %s", err)
	}
	// A successful compilation (err == nil) is sufficient to validate
	// that the type was resolved correctly.
}
