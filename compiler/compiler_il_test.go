package compiler

import (
	"beedance/code"
	_ "beedance/stdlib"
	"testing"
)

func TestIlCompilerOperators(t *testing.T) {
	tests := []compilerTestCase{
		{
			name: "LD, ADD, ST",
			input: `
			PROGRAM MyIlProgram
				VAR a: INT := 10; b: INT := 20; c: INT; END_VAR
				LD a
				ADD b
				ST c
			END_PROGRAM
			`,
			expectedConstants: []interface{}{10, 20, 0},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 2),
			},
		},
		{
			name: "JMPCN",
			input: `
			PROGRAM JmpTest
				VAR a: INT; b: BOOL := FALSE; END_VAR
				LD b
				JMPCN end_it
				LD 10
			end_it:
				ST a
			END_PROGRAM
			`,
			expectedConstants: []interface{}{0, 10},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpJumpNotTruthy, 19),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestIlCompiler_JMPC(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "JMPC",
		input: `
			PROGRAM JmpTest
				VAR a: INT; b: BOOL := TRUE; END_VAR
				LD b
				JMPC end_it
				LD 10
			end_it:
				ST a
			END_PROGRAM
			`,
		expectedConstants: []interface{}{0, 10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpTrue),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpBang),
			code.Make(code.OpJumpNotTruthy, 20),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpSetGlobal, 0),
		},
	}})
}

func TestIlCompiler_SetAndReset(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "Set and Reset",
		input: `
			PROGRAM SetResetTest
				VAR cond: BOOL; target: BOOL; END_VAR
				LD cond
				S target
				R target
			END_PROGRAM
			`,
		expectedConstants: []interface{}{},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpDup),
			code.Make(code.OpJumpNotTruthy, 19),
			code.Make(code.OpTrue),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpDup),
			code.Make(code.OpJumpNotTruthy, 27),
			code.Make(code.OpFalse),
			code.Make(code.OpSetGlobal, 1),
		},
	}})
}

func TestIlCompiler_GtOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "GT operator",
		input: `
			PROGRAM GtTest
				VAR a: INT := 20; b: INT := 10; c: BOOL; END_VAR
				LD a
				GT b
				ST c
			END_PROGRAM
			`,
		expectedConstants: []interface{}{20, 10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 2),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpGreaterThan),
			code.Make(code.OpSetGlobal, 2),
		},
	}})
}

func TestIlCompiler_LtOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "LT operator",
		input: `
			PROGRAM LtTest
				VAR a: INT := 5; b: INT := 10; c: BOOL; END_VAR
				LD a
				LT b
				ST c
			END_PROGRAM
			`,
		expectedConstants: []interface{}{5, 10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 2),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpLessThan),
			code.Make(code.OpSetGlobal, 2),
		},
	}})
}

func TestIlCompiler_EqOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "EQ operator",
		input: `
			PROGRAM EqTest
				VAR a: INT := 7; b: INT := 7; c: BOOL; END_VAR
				LD a
				EQ b
				ST c
			END_PROGRAM
			`,
		expectedConstants: []interface{}{7},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 2),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpEqual),
			code.Make(code.OpSetGlobal, 2),
		},
	}})
}

func TestIlCompiler_GeOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "GE operator",
		input: `
			PROGRAM GeTest
				VAR a: INT := 10; b: INT := 10; c: BOOL; END_VAR
				LD a
				GE b
				ST c
			END_PROGRAM
			`,
		expectedConstants: []interface{}{10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 2),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpGreaterThanOrEqual),
			code.Make(code.OpSetGlobal, 2),
		},
	}})
}

func TestIlCompiler_LeOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "LE operator",
		input: `
			PROGRAM LeTest
				VAR a: INT := 9; b: INT := 10; c: BOOL; END_VAR
				LD a
				LE b
				ST c
			END_PROGRAM
			`,
		expectedConstants: []interface{}{9, 10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 2),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpLessThanOrEqual),
			code.Make(code.OpSetGlobal, 2),
		},
	}})
}

func TestIlCompiler_NeOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "NE operator",
		input: `
			PROGRAM NeTest
				VAR a: INT := 9; b: INT := 10; c: BOOL; END_VAR
				LD a
				NE b
				ST c
			END_PROGRAM
			`,
		expectedConstants: []interface{}{9, 10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpFalse), // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 2),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpNotEqual),
			code.Make(code.OpSetGlobal, 2),
		},
	}})
}

func TestIlCompiler_NotOperator(t *testing.T) {
	runCompilerTests(t, []compilerTestCase{{
		name: "NOT operator",
		input: `
			PROGRAM NotTest
				VAR a: BOOL := TRUE; b: BOOL; END_VAR
				LD a
				NOT
				ST b
			END_PROGRAM
			`,
		expectedConstants: []interface{}{},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpTrue),
			code.Make(code.OpSetGlobal, 0), // a := TRUE
			code.Make(code.OpFalse),        // BOOL defaults to FALSE
			code.Make(code.OpSetGlobal, 1), // b
			code.Make(code.OpGetGlobal, 0), // LD a
			code.Make(code.OpBang),         // NOT
			code.Make(code.OpSetGlobal, 1), // ST b
		},
	}})
}

func TestIlCompiler_ConditionalCalRet(t *testing.T) {
	tests := []compilerTestCase{
		{
			name: "CALC when true",
			input: `
			VAR_GLOBAL result: INT; END_VAR
			FUNCTION MyFunc : INT
				result := 10;
				MyFunc := 1;
			END_FUNCTION
			PROGRAM CalcTest
				VAR cond: BOOL := TRUE; END_VAR
				LD cond
				CALC MyFunc()
			END_PROGRAM
			`,
			// The function should be called, setting result to 10.
			expectedConstants: []interface{}{0, 10, 1, []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSetLocal, 0),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpReturnValue),
			}},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpJumpNotTruthy, 29),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			name: "CALCN when false",
			input: `
			VAR_GLOBAL result: INT; END_VAR
			FUNCTION MyFunc : INT
				result := 10;
				MyFunc := 0;
			END_FUNCTION
			PROGRAM CalcnTest
				VAR cond: BOOL := FALSE; END_VAR
				LD cond
				CALCN MyFunc()
			END_PROGRAM
			`,
			// The function should be called because condition is FALSE.
			expectedConstants: []interface{}{0, 10, []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 0),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpReturnValue),
			}},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpBang),
				code.Make(code.OpJumpNotTruthy, 30),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			name: "RETC when true",
			input: `
			FUNCTION MyFunc : INT
				VAR cond: BOOL := TRUE; END_VAR
				LD 5
				LD cond
				RETC
				LD 10
			END_FUNCTION
			PROGRAM RetcTest
				VAR result: INT; END_VAR
				result := MyFunc();
			END_PROGRAM
			`,
			// MyFunc should return 5 because RETC is executed.
			expectedConstants: []interface{}{0, 5, 10, []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpSetLocal, 1),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGetLocal, 1),
				code.Make(code.OpJumpNotTruthy, 17),
				code.Make(code.OpReturnValue),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpReturnValue),
			}},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpSetGlobal, 1),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestIlCompiler_NestedDeferredModifiers(t *testing.T) {
	tests := []compilerTestCase{
		{
			name: "Nested LD with deferred operand",
			input: `
			PROGRAM LdNestedDeferredTest
				VAR a: INT; END_VAR
				LD(
					LD 5
					LD(
						ADD 10
					)
				)
				ST a
			END_PROGRAM
			`,
			expectedConstants: []interface{}{0, 5, 10},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 0),
			},
		},
		{
			name: "AND with nested deferred operand",
			input: `
			PROGRAM AndNestedDeferredTest
				VAR a: BOOL; END_VAR
				LD TRUE
				AND(
					LD FALSE
					OR(
						LD TRUE
						AND TRUE
					)
				)
				ST a
			END_PROGRAM
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpTrue),
				code.Make(code.OpTrue),
				code.Make(code.OpAnd),
				code.Make(code.OpOr),
				code.Make(code.OpAnd),
				code.Make(code.OpSetGlobal, 0),
			},
		},
		{
			name: "Mixed standard and deferred operators",
			input: `
			PROGRAM MixedDeferredTest
				VAR
					A: BOOL := TRUE;
					B: BOOL := FALSE;
					C: BOOL := TRUE;
					D: BOOL := TRUE;
					Result: BOOL;
				END_VAR

				LD      A
				AND     B
				OR(
					LD      C
					AND     D
				)
				ST      Result
			END_PROGRAM
			`,
			expectedConstants: []interface{}{}, // All values are TRUE/FALSE/NULL, no constants needed
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 3),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 4),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpAnd),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpGetGlobal, 3),
				code.Make(code.OpAnd),
				code.Make(code.OpOr),
				code.Make(code.OpSetGlobal, 4),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestIlCompiler_DeferredModifiers(t *testing.T) {
	tests := []compilerTestCase{
		{
			name: "LD with deferred operand",
			input: `
			PROGRAM LdDeferredTest
				VAR a: INT; b: INT := 10; END_VAR
				LD(
					LD b
					ADD 5
				)
				ST a
			END_PROGRAM
			`,
			expectedConstants: []interface{}{0, 10, 5},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 0),
			},
		},
		{
			name: "AND with deferred operand",
			input: `
			PROGRAM AndDeferredTest
				VAR a: BOOL := TRUE; b: BOOL; c: BOOL := FALSE; END_VAR
				LD a
				AND(
					LD c
					NOT
				)
				ST b
			END_PROGRAM
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpBang),
				code.Make(code.OpAnd),
				code.Make(code.OpSetGlobal, 1),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestIlCompiler_Modifiers(t *testing.T) {
	tests := []compilerTestCase{
		{
			name: "LDN operator",
			input: `
			PROGRAM LdnTest
				VAR a: BOOL := TRUE; b: BOOL; END_VAR
				LDN a
				ST b
			END_PROGRAM
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpBang),
				code.Make(code.OpSetGlobal, 1),
			},
		},
		{
			name: "STN operator",
			input: `
			PROGRAM StnTest
				VAR a: BOOL := TRUE; b: BOOL; END_VAR
				LD a
				STN b
			END_PROGRAM
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpBang),
				code.Make(code.OpSetGlobal, 1),
			},
		},
		{
			name: "ANDN operator",
			input: `
			PROGRAM AndnTest
				VAR a: BOOL := TRUE; b: BOOL := TRUE; c: BOOL; END_VAR
				LD a
				ANDN b
				ST c
			END_PROGRAM
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpBang),
				code.Make(code.OpAnd),
				code.Make(code.OpSetGlobal, 2),
			},
		},
	}
	runCompilerTests(t, tests)
}
