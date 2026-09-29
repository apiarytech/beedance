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
	"testing"
)

func TestForLoopCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			name:              "default step of 1",
			input:             "VAR i : INT; END_VAR FOR i := 1 TO 3 DO END_FOR",
			expectedConstants: []interface{}{0, 1, 3},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpLessThanOrEqual),
				code.Make(code.OpJumpNotTruthy, 35),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpJump, 12),
			},
		},
		{
			name:              "constant negative step counts down",
			input:             "VAR i : INT; END_VAR FOR i := 3 TO 1 BY -1 DO END_FOR",
			expectedConstants: []interface{}{0, 3, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpGreaterThanOrEqual),
				code.Make(code.OpJumpNotTruthy, 36),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpMinus),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpJump, 12),
			},
		},
		{
			name:              "variable step is checked at runtime",
			input:             "VAR i : INT; d : INT; END_VAR FOR i := 1 TO 3 BY d DO END_FOR",
			expectedConstants: []interface{}{0, 1, 3},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpGreaterThanOrEqual),
				code.Make(code.OpJumpNotTruthy, 38),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpLessThanOrEqual),
				code.Make(code.OpJump, 45),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpGreaterThanOrEqual),
				code.Make(code.OpJumpNotTruthy, 61),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpJump, 18),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestWhileAndRepeatCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			name:              "WHILE",
			input:             "VAR i : INT; END_VAR WHILE i < 3 DO i := i + 1; END_WHILE",
			expectedConstants: []interface{}{0, 3, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpLessThan),
				code.Make(code.OpJumpNotTruthy, 29),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpAdd),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpJump, 6),
			},
		},
		{
			name:              "REPEAT with EXIT",
			input:             "REPEAT EXIT; UNTIL TRUE END_REPEAT",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpJump, 7), // EXIT jumps past the loop
				code.Make(code.OpTrue),
				code.Make(code.OpJumpNotTruthy, 0), // UNTIL FALSE repeats the body
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestCaseCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "CASE 5 OF 1, 2: 10; 3..4: 20; ELSE 30; END_CASE",
			expectedConstants: []interface{}{5, 1, 2, 10, 3, 4, 20, 30},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0), // selector
				// Branch 1, label 1
				code.Make(code.OpDup), code.Make(code.OpConstant, 1), code.Make(code.OpEqual),
				code.Make(code.OpJumpNotTruthy, 14), code.Make(code.OpJump, 28),
				// Branch 1, label 2
				code.Make(code.OpDup), code.Make(code.OpConstant, 2), code.Make(code.OpEqual),
				code.Make(code.OpJumpNotTruthy, 25), code.Make(code.OpJump, 28),
				code.Make(code.OpJump, 36), // no label matched: next branch
				// 0028: branch 1 body
				code.Make(code.OpPop), code.Make(code.OpConstant, 3), code.Make(code.OpPop), code.Make(code.OpJump, 71),
				// 0036: branch 2, range 3..4
				code.Make(code.OpDup), code.Make(code.OpConstant, 4), code.Make(code.OpGreaterThanOrEqual),
				code.Make(code.OpJumpNotTruthy, 55),
				code.Make(code.OpDup), code.Make(code.OpConstant, 5), code.Make(code.OpLessThanOrEqual),
				code.Make(code.OpJumpNotTruthy, 55),
				code.Make(code.OpJump, 58),
				code.Make(code.OpJump, 66),
				// 0058: branch 2 body
				code.Make(code.OpPop), code.Make(code.OpConstant, 6), code.Make(code.OpPop), code.Make(code.OpJump, 71),
				// 0066: ELSE
				code.Make(code.OpPop), code.Make(code.OpConstant, 7), code.Make(code.OpPop),
			},
		},
	}
	runCompilerTests(t, tests)
}
