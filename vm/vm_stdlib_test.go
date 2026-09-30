/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"beedance/code"
	"beedance/compiler"
	"beedance/object"
	"strings"
	"testing"
)

func TestRealArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1.5 + 2.25;", 3.75},
		{"5.5 - 2.0;", 3.5},
		{"1.5 * 4.0;", 6.0},
		{"7.5 / 2.5;", 3.0},
		{"2.0 ** 3.0;", 8.0},
		{"-1.5;", -1.5},
		{"-(2.5 * 2.0);", -5.0},
	}
	runVmTests(t, tests)
}

func TestRealComparisons(t *testing.T) {
	tests := []vmTestCase{
		{"1.5 = 1.5;", true},
		{"1.5 <> 1.5;", false},
		{"1.5 < 2.25;", true},
		{"1.5 > 2.25;", false},
		{"2.25 >= 2.25;", true},
		{"2.5 <= 2.25;", false},
	}
	runVmTests(t, tests)
}

func TestIntegerArithmeticEdgeCases(t *testing.T) {
	tests := []vmTestCase{
		{"7 MOD 3;", 1},
		{"-7 MOD 3;", -1},
		{"2 ** 10;", 1024},
		{"7 / 2;", 3},
		{"-5;", -5},
		{"3 <> 4;", true},
		{"4 >= 4;", true},
		{"5 <= 4;", false},
	}
	runVmTests(t, tests)
}

func TestArithmeticErrors(t *testing.T) {
	runVmErrorTests(t, []vmErrorTestCase{
		{"7 / 0;", "division by zero"},
		{"7 MOD 0;", "division by zero"},
		{"1.5 / 0.0;", "division by zero"},
		{"-'text';", "unsupported type for negation: STRING"},
	})
}

func TestStringOperations(t *testing.T) {
	tests := []vmTestCase{
		{"'ab' + 'cd';", "abcd"},
		{"'abc' = 'abc';", true},
		{"'abc' <> 'abd';", true},
		{"'apple' < 'banana';", true},
		{"'apple' > 'banana';", false},
		{"'b' >= 'b';", true},
		{"'a' <= 'B';", false}, // Character codes: 'B' sorts before 'a'.
	}
	runVmTests(t, tests)
}

func TestWStringOperations(t *testing.T) {
	object.FinalizeBuiltins()
	tests := []struct {
		input    string
		expected object.Object
	}{
		{`"ab" + "cd";`, &object.WString{Value: "abcd"}},
		{`"ab" + 'cd';`, &object.WString{Value: "abcd"}},
		{`"abc" < "abd";`, True},
		{`"abc" = 'abc';`, True},
	}
	for _, tt := range tests {
		comp := compiler.New()
		if err := comp.Compile(parse(t, tt.input)); err != nil {
			t.Fatalf("%s: compiler error: %s", tt.input, err)
		}
		machine := New(comp.Bytecode())
		if err := machine.Run(); err != nil {
			t.Fatalf("%s: vm error: %s", tt.input, err)
		}
		got := machine.LastPoppedStackElem()
		if got.Type() != tt.expected.Type() || got.Inspect() != tt.expected.Inspect() {
			t.Fatalf("%s: expected %s %s, got %s %s", tt.input, tt.expected.Type(), tt.expected.Inspect(), got.Type(), got.Inspect())
		}
	}
}

// TestMixedRealTypes covers REAL (32-bit) operands, which the compiler does not
// produce from literals but which built-ins and conversions can return.
func TestMixedRealTypes(t *testing.T) {
	constants := []object.Object{&object.Real{Value: 1.5}, &object.LReal{Value: 2.5}}
	load := func() [][]byte {
		return [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1)}
	}

	for _, tc := range []struct {
		op   code.Opcode
		want object.Object
	}{
		{code.OpAdd, &object.LReal{Value: 4.0}},
		{code.OpSub, &object.LReal{Value: -1.0}},
		{code.OpLessThan, True},
		{code.OpGreaterThan, False},
		{code.OpNotEqual, True},
	} {
		ins := append(load(), code.Make(tc.op), code.Make(code.OpPop))
		machine, err := runBytecode(constants, nil, ins...)
		if err != nil {
			t.Fatalf("%s: vm error: %s", opcodeName(tc.op), err)
		}
		got := machine.LastPoppedStackElem()
		if got.Inspect() != tc.want.Inspect() {
			t.Fatalf("%s: expected %s, got %s", opcodeName(tc.op), tc.want.Inspect(), got.Inspect())
		}
	}

	// REAL on the right, and negating a REAL.
	machine, err := runBytecode(constants, nil,
		code.Make(code.OpConstant, 1), code.Make(code.OpConstant, 0), code.Make(code.OpMul), code.Make(code.OpPop))
	if err != nil || machine.LastPoppedStackElem().Inspect() != "3.750000" {
		t.Fatalf("LREAL * REAL: expected 3.750000, got %v (err %v)", machine.LastPoppedStackElem(), err)
	}
	machine, err = runBytecode(constants, nil, code.Make(code.OpConstant, 0), code.Make(code.OpMinus), code.Make(code.OpPop))
	if neg, ok := machine.LastPoppedStackElem().(*object.Real); err != nil || !ok || neg.Value != -1.5 {
		t.Fatalf("-REAL: expected REAL -1.5, got %v (err %v)", machine.LastPoppedStackElem(), err)
	}
}

// TestUnsupportedOperators checks operators that no operand type supports,
// such as MOD on reals or '-' on strings. The compiler rejects these, so they
// are assembled directly.
func TestUnsupportedOperators(t *testing.T) {
	tests := []struct {
		name      string
		constants []object.Object
		op        code.Opcode
		want      string
	}{
		{"MOD on reals", []object.Object{&object.LReal{Value: 1}, &object.LReal{Value: 2}}, code.OpMod, "unknown real operator: OpMod"},
		{"'-' on strings", []object.Object{&object.String{Value: "a"}, &object.String{Value: "b"}}, code.OpSub, "unknown string operator: OpSub"},
		{"'+' on booleans", []object.Object{True, False}, code.OpAdd, "unknown boolean operator: OpAdd"},
		{"'<' on booleans", []object.Object{True, False}, code.OpLessThan, "unsupported operator OpLessThan for types BOOLEAN and BOOLEAN"},
		{"'+' on array and integer", []object.Object{&object.Array{}, &object.LInt{Value: 1}}, code.OpAdd, "unsupported types for binary operation: ARRAY LINT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runBytecode(tt.constants, nil, code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(tt.op))
			if err == nil || err.Error() != tt.want {
				t.Fatalf("expected error %q, got %v", tt.want, err)
			}
		})
	}
}

func TestBuiltinCalls(t *testing.T) {
	tests := []vmTestCase{
		{"ABS(-5);", 5},
		{"LEN('hello');", 5},
		{"SQRT(16.0);", 4.0},
	}
	runVmTests(t, tests)

	// A built-in that returns nothing leaves NULL as its result.
	var called []object.Object
	noResult := &object.Builtin{Fn: func(args ...object.Object) object.Object {
		called = args
		return nil
	}}
	machine, err := runBytecode([]object.Object{&object.LInt{Value: 7}}, []*object.Builtin{noResult},
		code.Make(code.OpGetBuiltin, 0), code.Make(code.OpConstant, 0), code.Make(code.OpCall, 1), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if machine.LastPoppedStackElem() != Null {
		t.Fatalf("expected NULL, got %v", machine.LastPoppedStackElem())
	}
	if len(called) != 1 || called[0].Inspect() != "7" {
		t.Fatalf("expected the built-in to receive [7], got %v", called)
	}

	// An index past the registered built-ins is an error.
	_, err = runBytecode(nil, []*object.Builtin{noResult}, code.Make(code.OpGetBuiltin, 3))
	if err == nil || err.Error() != "invalid builtin index: 3" {
		t.Fatalf("expected an invalid builtin index error, got %v", err)
	}
}

// TestNewUsesRegisteredBuiltins checks that New wires up the globally
// registered built-ins by their own index.
func TestNewUsesRegisteredBuiltins(t *testing.T) {
	object.FinalizeBuiltins()
	comp := compiler.New()
	if err := comp.Compile(parse(t, "ABS(-3) + LEN('abcd');")); err != nil {
		t.Fatalf("compiler error: %s", err)
	}
	machine := New(comp.Bytecode())
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(7, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
}

func TestCompileCommaSeparatedIndexExpressions(t *testing.T) {
	tests := []struct {
		input                string
		expectedOpIndexCount int
		expectedValue        int64
	}{
		{
			input:                "[[1, 2], [3, 4]][0, 1];",
			expectedOpIndexCount: 2,
			expectedValue:        2,
		},
		{
			input:                "[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][1, 0, 1];",
			expectedOpIndexCount: 3,
			expectedValue:        6,
		},
		{
			input:                "[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][1 + 0, 2 - 2, 1 * 1];",
			expectedOpIndexCount: 3,
			expectedValue:        6,
		},
	}

	for _, tt := range tests {
		comp := compiler.New()
		program := parse(t, tt.input)
		if err := comp.Compile(program); err != nil {
			t.Fatalf("%s: compiler error: %s", tt.input, err)
		}

		bytecode := comp.Bytecode()
		disassembly := bytecode.Instructions.String()
		count := strings.Count(disassembly, "OpIndex")
		if count != tt.expectedOpIndexCount {
			t.Errorf("%s: wrong OpIndex count. want=%d, got=%d\nDisassembly:\n%s",
				tt.input, tt.expectedOpIndexCount, count, disassembly)
		}

		machine := New(bytecode)
		if err := machine.Run(); err != nil {
			t.Fatalf("%s: vm error: %s", tt.input, err)
		}

		if err := testIntegerObject(tt.expectedValue, machine.LastPoppedStackElem()); err != nil {
			t.Errorf("%s: %v", tt.input, err)
		}
	}
}

func TestVmCommaSeparatedIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[[1, 2, 3], [4, 5, 6]][0, 0];", 1},
		{"[[1, 2, 3], [4, 5, 6]][0, 1];", 2},
		{"[[1, 2, 3], [4, 5, 6]][0, 2];", 3},
		{"[[1, 2, 3], [4, 5, 6]][1, 0];", 4},
		{"[[1, 2, 3], [4, 5, 6]][1, 1];", 5},
		{"[[1, 2, 3], [4, 5, 6]][1, 2];", 6},
		{"[[10, 20], [30, 40]][1 - 1, 0 + 1];", 20},
		{"[[10, 20], [30, 40]][2 - 1, 2 - 2];", 30},
		{"[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][0, 0, 0];", 1},
		{"[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][0, 1, 1];", 4},
		{"[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][1, 0, 0];", 5},
		{"[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][1, 0, 1];", 6},
		{"[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][1, 1, 0];", 7},
		{"[[[1, 2], [3, 4]], [[5, 6], [7, 8]]][1, 1, 1];", 8},
		{"[[1, 2], [3, 4]][0, 0] + [[10, 20], [30, 40]][1, 1];", 41},
		{"[[1, 2], [3, 4]][1, 1] * [[1, 2], [3, 4]][0, 1];", 8},
		{"[[1, 2], [3, 4]][0, 1] < [[1, 2], [3, 4]][1, 0];", true},
		{"[[1, 2], [3, 4]][0, 0] = 1;", true},
		{"[[1, 2], [3, 4]][0, 0] <> [[1, 2], [3, 4]][1, 1];", true},
	}
	runVmTests(t, tests)
}
