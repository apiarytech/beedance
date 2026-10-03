package vm

import (
	_ "github.com/apiarytech/beedance/stdlib"
	"testing"
)

func TestIecVmOperators(t *testing.T) {
	tests := []vmTestCase{
		// Arithmetic
		{"10 MOD 3", 1},
		{"2 ** 10", 1024},

		// Comparison
		{"1 < 2", true},
		{"2 < 1", false},
		{"1 <= 2", true},
		{"2 <= 2", true},
		{"3 <= 2", false},
		{"2 >= 1", true},
		{"2 >= 2", true},
		{"1 >= 2", false},
		{"1 = 2", false},
		{"2 = 2", true},
		{"1 <> 2", true},
		{"2 <> 2", false},

		// Logical (Boolean)
		{"true AND false", false},
		{"true AND true", true},
		{"false AND false", false},
		{"true OR false", true},
		{"true OR true", true},
		{"false OR false", false},
		{"true XOR false", true},
		{"true XOR true", false},
		{"false XOR false", false},
		{"true NAND false", true},
		{"true NAND true", false},
		{"false NAND true", true},
		{"true NOR false", false},
		{"true NOR true", false},
		{"false NOR false", true},

		// Bitwise (Integer)
		{"10 AND 12", 8},
		{"10 OR 12", 14},
		{"10 XOR 12", 6},
		// Note: In Go, bitwise NOT (^) on an int64 results in a two's complement negative number.
		// ^(10 & 12) = ^8 = -9
		{"10 NAND 12", -9},
		// ^(10 | 12) = ^14 = -15
		{"10 NOR 12", -15},
	}

	runVmTests(t, tests)
}
