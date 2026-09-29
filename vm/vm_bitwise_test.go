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
	"beedance/object"
	"testing"
)

func TestBooleanLogicTruthTables(t *testing.T) {
	type row struct{ a, b, and, or, xor, nand, nor bool }
	rows := []row{
		{false, false, false, false, false, true, true},
		{false, true, false, true, true, true, false},
		{true, false, false, true, true, true, false},
		{true, true, true, true, false, false, false},
	}
	name := map[bool]string{true: "TRUE", false: "FALSE"}
	tests := []vmTestCase{}
	for _, r := range rows {
		a, b := name[r.a], name[r.b]
		tests = append(tests,
			vmTestCase{a + " AND " + b + ";", r.and},
			vmTestCase{a + " OR " + b + ";", r.or},
			vmTestCase{a + " XOR " + b + ";", r.xor},
			vmTestCase{a + " NAND " + b + ";", r.nand},
			vmTestCase{a + " NOR " + b + ";", r.nor},
		)
	}
	runVmTests(t, tests)
}

func TestIntegerBitwiseOperations(t *testing.T) {
	tests := []vmTestCase{
		{"12 AND 10;", 8},
		{"12 OR 10;", 14},
		{"12 XOR 10;", 6},
		{"12 NAND 10;", -9}, // ^(12 & 10) on a 64-bit integer
		{"12 NOR 10;", -15}, // ^(12 | 10) on a 64-bit integer
		{"255 AND 15 OR 256;", 271},
	}
	runVmTests(t, tests)
}

func TestNotOperator(t *testing.T) {
	tests := []vmTestCase{
		{"NOT TRUE;", false},
		{"NOT FALSE;", true},
		{"NOT (1 < 2);", false},
		// NOT on a non-BOOL, non-bit-string value gives FALSE.
		{"NOT 5;", false},
	}
	runVmTests(t, tests)
}

// TestNotOnBitStrings checks that NOT inverts only the bits of the value's width.
func TestNotOnBitStrings(t *testing.T) {
	tests := []struct {
		value, width int
		want         uint64
	}{
		{0x0F, 8, 0xF0},
		{0x00FF, 16, 0xFF00},
		{0x0000FFFF, 32, 0xFFFF0000},
		{0, 64, 0xFFFFFFFFFFFFFFFF},
	}
	for _, tt := range tests {
		constant := &object.BitString{Value: uint64(tt.value), Width: tt.width}
		machine, err := runBytecode([]object.Object{constant}, nil,
			code.Make(code.OpConstant, 0), code.Make(code.OpBang), code.Make(code.OpPop))
		if err != nil {
			t.Fatalf("NOT on %d-bit value: vm error: %s", tt.width, err)
		}
		got, ok := machine.LastPoppedStackElem().(*object.BitString)
		if !ok || got.Value != tt.want || got.Width != tt.width {
			t.Fatalf("NOT on %d-bit value %#x: expected %#x, got %v", tt.width, tt.value, tt.want, machine.LastPoppedStackElem())
		}
	}
}

func TestNotOnNull(t *testing.T) {
	machine, err := runBytecode(nil, nil, code.Make(code.OpNull), code.Make(code.OpBang), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if machine.LastPoppedStackElem() != True {
		t.Fatalf("expected NOT NULL to be TRUE, got %v", machine.LastPoppedStackElem())
	}
}

func TestNotOnBitStringLiterals(t *testing.T) {
	object.FinalizeBuiltins()
	for input, want := range map[string]string{
		"NOT BYTE#16#0F;":   "BYTE#16#F0",
		"NOT WORD#16#00FF;": "WORD#16#FF00",
	} {
		m := New(compileForTest(t, input))
		if err := m.Run(); err != nil {
			t.Fatalf("%s: vm error: %s", input, err)
		}
		if got := m.LastPoppedStackElem().Inspect(); got != want {
			t.Fatalf("%s: expected %s, got %s", input, want, got)
		}
	}
}
