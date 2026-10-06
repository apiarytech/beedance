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

// The evaluator computes with a host's I/O words as the VM does
// (vm/vm_iowords_test.go has the same cases).
func TestIOWordsCompute(t *testing.T) {
	for _, c := range []struct {
		name, src, out, want string
	}{
		{"located OR literal", "VAR lvl AT %IW0 : WORD; END_VAR %QW3 := lvl OR 16#0100;", "%QW3", "WORD#16#129"},
		{"located OR typed", "VAR lvl AT %IW0 : WORD; END_VAR %QW3 := lvl OR WORD#16#0100;", "%QW3", "WORD#16#129"},
		{"direct OR literal", "%QW3 := %IW0 OR 16#0100;", "%QW3", "WORD#16#129"},
		{"direct AND typed", "%QW3 := %IW0 AND WORD#16#000F;", "%QW3", "WORD#16#9"},
		{"with a WORD variable", "VAR m : WORD := 16#00F0; END_VAR %QW3 := %IW0 AND m;", "%QW3", "WORD#16#20"},
		{"NOT", "%QW3 := NOT %IW0;", "%QW3", "WORD#16#FFD6"},
		{"compare literal", "%QX0.0 := %IW0 > 16#20;", "%QX0.0", "true"},
		{"compare byte to 0", "%QX0.1 := %IB2 = 0;", "%QX0.1", "true"},
		{"located compare", "VAR lvl AT %IW0 : WORD; END_VAR %QX0.2 := lvl = WORD#16#29;", "%QX0.2", "true"},
		{"double word", "%QD1 := %ID1 XOR 16#FFFF0000;", "%QD1", "DWORD#16#FFFF1234"},
	} {
		for k := range ioMap {
			delete(ioMap, k)
		}
		ioMap["%IW0"] = &object.Word{Value: 0x0029}
		ioMap["%IB2"] = &object.Byte{Value: 0}
		ioMap["%ID1"] = &object.DWord{Value: 0x00001234}
		if r := testEval(t, c.src); isError(r) {
			t.Errorf("%s: %s", c.name, r.Inspect())
			continue
		}
		if got := ioMap[c.out]; got == nil || got.Inspect() != c.want {
			t.Errorf("%s: %s = %v, want %s", c.name, c.out, got, c.want)
		}
	}
}
