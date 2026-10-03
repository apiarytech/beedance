package stdlib

import (
	"github.com/apiarytech/beedance/object"
	"math"
	"testing"
)

func TestBuiltinBitShiftRotateFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// SHL
		{"SHL(BYTE#16#A5, 1);", uint64(0x4A)}, // 10100101 << 1 -> 01001010
		{"SHL(WORD#16#FF00, 8);", uint64(0x00)},
		{"SHL(DWORD#16#1, 31);", uint64(1 << 31)},
		{"SHL(BYTE#16#FF, 9);", uint64(0x00)}, // Shifted out

		// SHR
		{"SHR(BYTE#16#A5, 1);", uint64(0x52)}, // 10100101 >> 1 -> 01010010
		{"SHR(WORD#16#FF00, 8);", uint64(0x00FF)},
		{"SHR(DWORD#16#80000000, 31);", uint64(1)},

		// ROL
		{"ROL(BYTE#16#A5, 1);", uint64(0x4B)}, // 10100101 ROL 1 -> 01001011
		{"ROL(WORD#16#C0F0, 4);", uint64(0x0F0C)},
		{"ROL(DWORD#16#1, 32);", uint64(1)}, // Full rotation

		// ROR
		{"ROR(BYTE#16#A5, 1);", uint64(0xD2)}, // 10100101 ROR 1 -> 11010010
		{"ROR(WORD#16#0F0C, 4);", uint64(0xC0F0)},
		{"ROR(DWORD#16#1, 32);", uint64(1)}, // Full rotation

		// Error cases
		{"SHL(10, 1);", "BUILTIN ERROR: argument 1 to `SHL` must be a bitstring type, got LINT"},
		{"SHR(BYTE#16#FF, -1);", "BUILTIN ERROR: shift amount for `SHR` must be non-negative, got -1"},
		{"ROL(BYTE#16#FF, TRUE);", "BUILTIN ERROR: argument 2 to `ROL` must be INTEGER, got BOOLEAN"},
		{"ROR();", "BUILTIN ERROR: wrong number of arguments for ROR. got=0, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case uint64:
				bs, ok := evaluated.(*object.BitString)
				if !ok {
					t.Fatalf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
				}
				// Mask the result to the width of the bitstring to handle overflow cases in tests correctly
				mask := uint64(math.MaxUint64)
				if bs.Width < 64 {
					mask = (1 << bs.Width) - 1
				}
				if (bs.Value & mask) != (expected & mask) {
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value&mask, bs.Value&mask)
				}
			case string:
				testStringOrError(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}
