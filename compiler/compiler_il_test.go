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
			expectedConstants: []interface{}{
				"a", // 0
				"b", // 1
				"c", // 2
				[]code.Instructions{ // 3
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpIndex),
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 4
				"__class__", // 5
				10,          // 6
				20,          // 7
				0,           // 8
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 8),
				code.Make(code.OpHash, 8),
				code.Make(code.OpSetGlobal, 1),
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
			expectedConstants: []interface{}{
				"b", // 0
				10,  // 1
				"a", // 2
				[]code.Instructions{ // 3
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpJumpNotTruthy, 12),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 4
				"__class__", // 5
				0,           // 6
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 6),
				code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"b", // 0
			10,  // 1
			"a", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpBang),
				code.Make(code.OpJumpNotTruthy, 13),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			0,           // 6
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpTrue),
			code.Make(code.OpHash, 6),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"cond",   // 0
			"target", // 1
			[]code.Instructions{ // 2
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpDup),
				code.Make(code.OpJumpNotTruthy, 19),
				code.Make(code.OpTrue),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpDup),
				code.Make(code.OpJumpNotTruthy, 32),
				code.Make(code.OpFalse),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 3
			"__class__", // 4
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 3),
			code.Make(code.OpClosure, 2, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 4),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpFalse),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 6),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			"c", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpGreaterThan),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			20,          // 6
			10,          // 7
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpConstant, 7),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 8),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			"c", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpLessThan),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			5,           // 6
			10,          // 7
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpConstant, 7),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 8),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			"c", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpEqual),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			7,           // 6
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 8),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			"c", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpGreaterThanOrEqual),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			10,          // 6
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 8),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			"c", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpLessThanOrEqual),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			9,           // 6
			10,          // 7
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpConstant, 7),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 8),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			"c", // 2
			[]code.Instructions{ // 3
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpNotEqual),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 4
			"__class__", // 5
			9,           // 6
			10,          // 7
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 4),
			code.Make(code.OpClosure, 3, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 5),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpConstant, 6),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpConstant, 7),
			code.Make(code.OpConstant, 2),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 8),
			code.Make(code.OpSetGlobal, 1),
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
		expectedConstants: []interface{}{
			"a", // 0
			"b", // 1
			[]code.Instructions{ // 2
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpIndex),
				code.Make(code.OpBang),
				code.Make(code.OpGetLocal, 0),
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSwap),
				code.Make(code.OpSetIndex),
				code.Make(code.OpReturn),
			},
			"main",      // 3
			"__class__", // 4
		},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpConstant, 3),
			code.Make(code.OpClosure, 2, 0),
			code.Make(code.OpHash, 2),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpConstant, 4),
			code.Make(code.OpGetGlobal, 0),
			code.Make(code.OpConstant, 0),
			code.Make(code.OpTrue),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpFalse),
			code.Make(code.OpHash, 6),
			code.Make(code.OpSetGlobal, 1),
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
			expectedConstants: []interface{}{
				0,  // 0
				10, // 1
				1,  // 2
				[]code.Instructions{ // 3
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetGlobal, 0),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
				"cond", // 4
				[]code.Instructions{ // 5
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 4),
					code.Make(code.OpIndex),
					code.Make(code.OpJumpNotTruthy, 15),
					code.Make(code.OpGetGlobal, 1),
					code.Make(code.OpCall, 0),
					code.Make(code.OpPop),
					code.Make(code.OpReturn),
				},
				"main",      // 6
				"__class__", // 7
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpClosure, 5, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpTrue),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 3),
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
			expectedConstants: []interface{}{
				0,  // 0
				10, // 1
				[]code.Instructions{ // 2
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetGlobal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
				"cond", // 3
				[]code.Instructions{ // 4
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 3),
					code.Make(code.OpIndex),
					code.Make(code.OpBang),
					code.Make(code.OpJumpNotTruthy, 16),
					code.Make(code.OpGetGlobal, 1),
					code.Make(code.OpCall, 0),
					code.Make(code.OpPop),
					code.Make(code.OpReturn),
				},
				"main",      // 5
				"__class__", // 6
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpClosure, 4, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 2),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 3),
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
			expectedConstants: []interface{}{
				0,  // 0
				5,  // 1
				10, // 2
				[]code.Instructions{ // 3
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
				},
				"result", // 4
				[]code.Instructions{ // 5
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 4),
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpCall, 0),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 6
				"__class__", // 7
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpClosure, 5, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 2),
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
			expectedConstants: []interface{}{
				5,   // 0
				10,  // 1
				"a", // 2
				[]code.Instructions{ // 3
					code.Make(code.OpConstant, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 4
				"__class__", // 5
				0,           // 6
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 1),
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
			expectedConstants: []interface{}{
				"a", // 0
				[]code.Instructions{ // 1
					code.Make(code.OpTrue),
					code.Make(code.OpFalse),
					code.Make(code.OpTrue),
					code.Make(code.OpTrue),
					code.Make(code.OpAnd),
					code.Make(code.OpOr),
					code.Make(code.OpAnd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 2
				"__class__", // 3
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 2),
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 4),
				code.Make(code.OpSetGlobal, 1),
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
			expectedConstants: []interface{}{
				"A",      // 0
				"B",      // 1
				"C",      // 2
				"D",      // 3
				"Result", // 4
				[]code.Instructions{ // 5
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpIndex),
					code.Make(code.OpAnd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 3),
					code.Make(code.OpIndex),
					code.Make(code.OpAnd),
					code.Make(code.OpOr),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 4),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 6
				"__class__", // 7
			}, // All values are TRUE/FALSE/NULL, no constants needed
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 6),
				code.Make(code.OpClosure, 5, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 12),
				code.Make(code.OpSetGlobal, 1),
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
			expectedConstants: []interface{}{
				"b", // 0
				5,   // 1
				"a", // 2
				[]code.Instructions{ // 3
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 4
				"__class__", // 5
				0,           // 6
				10,          // 7
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 6),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 7),
				code.Make(code.OpHash, 6),
				code.Make(code.OpSetGlobal, 1),
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
			expectedConstants: []interface{}{
				"a", // 0
				"c", // 1
				"b", // 2
				[]code.Instructions{ // 3
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpIndex),
					code.Make(code.OpBang),
					code.Make(code.OpAnd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 4
				"__class__", // 5
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpFalse),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 8),
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
			expectedConstants: []interface{}{
				"a", // 0
				"b", // 1
				[]code.Instructions{ // 2
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpBang),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 3
				"__class__", // 4
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 3),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 6),
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
			expectedConstants: []interface{}{
				"a", // 0
				"b", // 1
				[]code.Instructions{ // 2
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpBang),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 3
				"__class__", // 4
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 3),
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 6),
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
			expectedConstants: []interface{}{
				"a", // 0
				"b", // 1
				"c", // 2
				[]code.Instructions{ // 3
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpIndex),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpIndex),
					code.Make(code.OpBang),
					code.Make(code.OpAnd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpSwap),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSwap),
					code.Make(code.OpSetIndex),
					code.Make(code.OpReturn),
				},
				"main",      // 4
				"__class__", // 5
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 4),
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpHash, 2),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpTrue),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpFalse),
				code.Make(code.OpHash, 8),
				code.Make(code.OpSetGlobal, 1),
			},
		},
	}
	runCompilerTests(t, tests)
}
