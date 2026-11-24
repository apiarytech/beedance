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
	"github.com/apiarytech/beedance/code"
	_ "github.com/apiarytech/beedance/stdlib"
	"testing"
)

func TestIecCompilerOperators(t *testing.T) {
	tests := []compilerTestCase{
		// Arithmetic
		{
			input:             "10 MOD 3;",
			expectedConstants: []interface{}{10, 3},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpMod),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "2 ** 10;",
			expectedConstants: []interface{}{2, 10},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpExponent),
				code.Make(code.OpPop),
			},
		},
		// Comparison
		{
			input:             "1 < 2;",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpLessThan),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 <= 2;",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpLessThanOrEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 >= 2;",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterThanOrEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 = 2;",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 <> 2;",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpNotEqual),
				code.Make(code.OpPop),
			},
		},
		// Logical
		{
			input:             "true AND false;",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpAnd),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "true OR false;",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpOr),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "true XOR false;",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpXor),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "true NAND false;",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpNand),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "true NOR false;",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpNor),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}
