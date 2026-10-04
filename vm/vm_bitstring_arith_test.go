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
	"testing"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
)

// TestBitStringArithmetic checks arithmetic on BYTE, WORD and DWORD as in
// CODESYS: as unsigned integers of their width that wrap around, a literal
// taking the bit string's type, and an integer variable promoting the bit
// string to an integer. OSCAT computes timestamps and checksums this way.
func TestBitStringArithmetic(t *testing.T) {
	object.FinalizeBuiltins()
	tests := []struct {
		input string
		value uint64
		width int // 0: an integer result
	}{
		{"VAR a : DWORD := 16#10; b : DWORD := 3; END_VAR a - b;", 13, 32},
		{"VAR a : DWORD := 1; b : DWORD := 2; END_VAR a - b;", 0xFFFFFFFF, 32},
		{"VAR x : BYTE := 250; END_VAR x + 10;", 4, 8},
		{"VAR x : BYTE := 5; END_VAR x - 10;", 251, 8},
		{"VAR x : BYTE := 7; END_VAR 160 + x;", 167, 8},
		{"VAR w : WORD := 16#FFFF; END_VAR w * 2;", 65534, 16},
		{"VAR d : DWORD := 100; END_VAR d / 7;", 14, 32},
		{"VAR d : DWORD := 100; END_VAR d MOD 7;", 2, 32},
		{"VAR x : BYTE := 200; w : WORD := 100; END_VAR x + w;", 300, 16},
		{"VAR x : BYTE := 200; u : UDINT := 3; END_VAR x * u;", 600, 0},
	}
	for _, tt := range tests {
		comp := compiler.NewCompilerWithBuiltins(object.Builtins)
		if err := comp.Compile(parse(t, tt.input)); err != nil {
			t.Fatalf("%s: compile: %v", tt.input, err)
		}
		machine := New(comp.Bytecode())
		if err := machine.Run(); err != nil {
			t.Fatalf("%s: run: %v", tt.input, err)
		}
		got := machine.LastPoppedStackElem()
		if tt.width == 0 {
			if n, _, ok := object.GetIntegerObjectValue(got); !ok || uint64(n) != tt.value {
				t.Errorf("%s = %s, want the integer %d", tt.input, got.Inspect(), tt.value)
			}
			continue
		}
		b, ok := got.(*object.BitString)
		if !ok || b.Value != tt.value || b.Width != tt.width {
			t.Errorf("%s = %#v, want %d in %d bits", tt.input, got, tt.value, tt.width)
		}
	}

	comp := compiler.NewCompilerWithBuiltins(object.Builtins)
	if err := comp.Compile(parse(t, "VAR d : DWORD := 1; z : DWORD; END_VAR d / z;")); err != nil {
		t.Fatal(err)
	}
	if err := New(comp.Bytecode()).Run(); err == nil {
		t.Error("DWORD division by zero did not fail")
	}
}

// TestCODESYSDialectRuns runs what the CODESYS dialect adds: a function and
// a function block writing their own inputs (AllowInputWrites), arrays sized
// by named constants, SIZEOF, a DATE moved by a TIME and a TIME scaled by a
// bit string.
func TestCODESYSDialectRuns(t *testing.T) {
	object.FinalizeBuiltins()
	run := func(src string) object.Object {
		t.Helper()
		comp := compiler.NewCompilerWithBuiltins(object.Builtins)
		comp.AllowInputWrites = true
		if err := comp.Compile(parse(t, src)); err != nil {
			t.Fatalf("%s: compile: %v", src, err)
		}
		machine := New(comp.Bytecode())
		if err := machine.Run(); err != nil {
			t.Fatalf("%s: run: %v", src, err)
		}
		return machine.LastPoppedStackElem()
	}
	integer := func(src string, want int64) {
		t.Helper()
		if n, _, ok := object.GetIntegerObjectValue(run(src)); !ok || n != want {
			t.Errorf("%s = %d, want %d", src, n, want)
		}
	}
	integer("FUNCTION F : INT VAR_INPUT x : INT; END_VAR x := x * 2; F := x + 1; END_FUNCTION F(3);", 7)
	integer(`FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR i := i + 1; o := i; END_FUNCTION_BLOCK
VAR f : Fb; END_VAR f(i := 4); f.o;`, 5)
	integer("VAR_GLOBAL CONSTANT N : INT := 4; END_VAR VAR a : ARRAY[0..N] OF INT; END_VAR LEN(a);", 5)
	integer("VAR_GLOBAL CONSTANT N : INT := 4; END_VAR VAR a : ARRAY[1..N * 2] OF BYTE; END_VAR SIZEOF(a);", 8)
	integer("VAR s : STRING(15); END_VAR SIZEOF(s);", 16)
	integer("VAR b : BYTE := 3; x : TIME; END_VAR x := T#2s * b; TIME_TO_DINT(x);", 6000)
	if b, ok := run("VAR d : DATE := D#2024-02-28; END_VAR d + T#1d = D#2024-02-29;").(*object.Boolean); !ok || !b.Value {
		t.Error("D#2024-02-28 + T#1d is not D#2024-02-29")
	}
}
