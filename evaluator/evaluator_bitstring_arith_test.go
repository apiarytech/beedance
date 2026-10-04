/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

import (
	"testing"

	"github.com/apiarytech/beedance/object"
)

// TestBitStringArithmetic checks the evaluator computes with bit strings as
// the compiler and the VM do (see vm.TestBitStringArithmetic).
func TestBitStringArithmetic(t *testing.T) {
	tests := []struct {
		input string
		value uint64
		width int // 0: an integer result
	}{
		{"VAR a : DWORD := 1; b : DWORD := 2; END_VAR a - b;", 0xFFFFFFFF, 32},
		{"VAR x : BYTE := 250; END_VAR x + 10;", 4, 8},
		{"VAR x : BYTE := 7; END_VAR 160 + x;", 167, 8},
		{"VAR w : WORD := 16#FFFF; END_VAR w * 2;", 65534, 16},
		{"VAR d : DWORD := 100; END_VAR d MOD 7;", 2, 32},
		{"VAR x : BYTE := 200; u : UDINT := 3; END_VAR x * u;", 600, 0},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if tt.width == 0 {
			if n, _, ok := object.GetIntegerObjectValue(got); !ok || uint64(n) != tt.value {
				t.Errorf("%s = %s, want the integer %d", tt.input, got.Inspect(), tt.value)
			}
			continue
		}
		b, ok := got.(*object.BitString)
		if !ok || b.Value != tt.value || b.Width != tt.width {
			t.Errorf("%s = %s, want %d in %d bits", tt.input, got.Inspect(), tt.value, tt.width)
		}
	}
}
