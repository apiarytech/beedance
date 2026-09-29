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
			expectedConstants: []interface{}{10, 20},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpNull),
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
			expectedConstants: []interface{}{10},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpJumpNotTruthy, 17),
				code.Make(code.OpConstant, 0),
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
		expectedConstants: []interface{}{10},
		expectedInstructions: []code.Instructions{
			code.Make(code.OpNull),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpTrue),
			code.Make(code.OpSetGlobal, 1),
			code.Make(code.OpGetGlobal, 1),
			code.Make(code.OpBang),
			code.Make(code.OpJumpNotTruthy, 18),
			code.Make(code.OpConstant, 0),
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
			code.Make(code.OpNull),
			code.Make(code.OpSetGlobal, 0),
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			code.Make(code.OpNull),
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
			expectedConstants: []interface{}{
				10, 1, // No initial 0
				[]code.Instructions{ // MyFunc body (now const index 2)
					code.Make(code.OpNull),         // init return var
					code.Make(code.OpSetLocal, 0),  // MyFunc is local 0
					code.Make(code.OpConstant, 0),  // 10 (now const index 0)
					code.Make(code.OpSetGlobal, 0), // result := 10
					code.Make(code.OpConstant, 1),  // 1 (now const index 1)
					code.Make(code.OpSetLocal, 0),  // MyFunc := 1
					code.Make(code.OpGetLocal, 0),  // return the result at the end
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				// Globals
				code.Make(code.OpNull), // result is uninitialized
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 2, 0), // MyFunc (const index 2)
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 2), // cond := TRUE
				// IL Body
				code.Make(code.OpGetGlobal, 2),      // LD cond
				code.Make(code.OpJumpNotTruthy, 27), // Jump over CAL if false
				code.Make(code.OpGetGlobal, 1),      // Get MyFunc
				code.Make(code.OpCall, 0),           // Call it
				code.Make(code.OpPop),               // Pop result of call
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
				10, 0,
				[]code.Instructions{ // MyFunc body (now const index 2)
					code.Make(code.OpNull),         // init return var
					code.Make(code.OpSetLocal, 0),  // MyFunc is local 0
					code.Make(code.OpConstant, 0),  // 10 (now const index 0)
					code.Make(code.OpSetGlobal, 0), // result := 10
					code.Make(code.OpConstant, 1),  // 0 (now const index 1)
					code.Make(code.OpSetLocal, 0),  // MyFunc := 0
					code.Make(code.OpGetLocal, 0),  // return the result at the end
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				// Globals
				code.Make(code.OpNull), // result is uninitialized
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 2, 0), // MyFunc (const index 2)
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 2),
				// IL Body
				code.Make(code.OpGetGlobal, 2),      // LD cond
				code.Make(code.OpBang),              // Invert condition for CALCN
				code.Make(code.OpJumpNotTruthy, 28), // Jump over CAL if original was true
				code.Make(code.OpGetGlobal, 1),      // Get MyFunc
				code.Make(code.OpCall, 0),           // Call it
				code.Make(code.OpPop),               // Pop result
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
				5, 10,
				[]code.Instructions{ // MyFunc body
					code.Make(code.OpNull),        // Init return var 'MyFunc'
					code.Make(code.OpSetLocal, 0), // MyFunc is local 0
					code.Make(code.OpTrue),        // cond := TRUE
					code.Make(code.OpSetLocal, 1), // cond is local 1
					code.Make(code.OpConstant, 0), // LD 5
					code.Make(code.OpGetLocal, 1), // LD cond
					code.Make(code.OpJumpNotTruthy, 15),
					code.Make(code.OpReturnValue), // RET
					code.Make(code.OpConstant, 1), // LD 10
					code.Make(code.OpReturnValue), // Implicit return at end
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0), // MyFunc is defined first
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpNull), // result is defined second
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 0), // Correctly get MyFunc (index 0)
				code.Make(code.OpCall, 0),
				code.Make(code.OpSetGlobal, 1), // Correctly set result (index 1)
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
			expectedConstants: []interface{}{5, 10},
			expectedInstructions: []code.Instructions{
				// VAR a
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 0),
				// IL Body
				// Outer LD( ... )
				// Inner LD( LD 5 )
				code.Make(code.OpConstant, 0), // LD 5
				// ADD 10
				code.Make(code.OpConstant, 1), // 10
				code.Make(code.OpAdd),
				// ST a
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
				// VAR a
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 0),
				// IL Body
				code.Make(code.OpTrue), // LD TRUE
				// AND( ... )
				code.Make(code.OpFalse), // LD FALSE
				// OR( ... )
				code.Make(code.OpTrue), // LD TRUE
				code.Make(code.OpTrue), // operand for AND TRUE
				code.Make(code.OpAnd),  // AND TRUE
				code.Make(code.OpOr),   // OR
				code.Make(code.OpAnd),  // AND
				// ST a
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
				// VAR Decls
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0), // A
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 1), // B
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 2), // C
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 3), // D
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 4), // Result
				// IL Body
				code.Make(code.OpGetGlobal, 0), // LD A
				code.Make(code.OpGetGlobal, 1), // operand for AND B
				code.Make(code.OpAnd),          // AND
				code.Make(code.OpGetGlobal, 2), // LD C (deferred block)
				code.Make(code.OpGetGlobal, 3), // operand for AND D
				code.Make(code.OpAnd),          // AND
				code.Make(code.OpOr),           // Deferred OR
				code.Make(code.OpSetGlobal, 4), // ST Result
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
			expectedConstants: []interface{}{10, 5},
			expectedInstructions: []code.Instructions{
				// VAR a
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 0),
				// VAR b := 10
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 1),
				// IL Body
				// LD( ... )
				code.Make(code.OpGetGlobal, 1), // LD b
				code.Make(code.OpConstant, 1),  // 5
				code.Make(code.OpAdd),          // ADD 5
				// ST a
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
				// VARs
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 0), // a
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 1), // b
				code.Make(code.OpFalse),
				code.Make(code.OpSetGlobal, 2), // c
				// IL Body
				code.Make(code.OpGetGlobal, 0), // LD a
				// AND( ... )
				code.Make(code.OpGetGlobal, 2), // LD c
				code.Make(code.OpBang),         // NOT
				code.Make(code.OpAnd),          // AND
				// ST b
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
				code.Make(code.OpSetGlobal, 0), // a := TRUE
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 1), // b
				code.Make(code.OpGetGlobal, 0), // LD a
				code.Make(code.OpBang),         // N modifier
				code.Make(code.OpSetGlobal, 1), // ST b
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
				code.Make(code.OpSetGlobal, 0), // a := TRUE
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 1), // b
				code.Make(code.OpGetGlobal, 0), // LD a
				code.Make(code.OpBang),         // N modifier for STN
				code.Make(code.OpSetGlobal, 1), // ST b
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
				code.Make(code.OpSetGlobal, 0), // a := TRUE
				code.Make(code.OpTrue),
				code.Make(code.OpSetGlobal, 1), // b := TRUE
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 2), // c
				code.Make(code.OpGetGlobal, 0), // LD a
				code.Make(code.OpGetGlobal, 1), // operand b for ANDN
				code.Make(code.OpBang),         // N modifier
				code.Make(code.OpAnd),          // AND
				code.Make(code.OpSetGlobal, 2), // ST c
			},
		},
	}
	runCompilerTests(t, tests)
}
