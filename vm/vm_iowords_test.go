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

// ioWordCases compute with bit strings a host put in the I/O image as the
// honeycomb connector does (object.Word, Byte, DWord), each in a located
// and a direct form; the evaluator runs the same cases.
var ioWordCases = []struct {
	name, src, out string
	want           string // Inspect of the output
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
}

func ioWords() map[string]object.Object {
	return map[string]object.Object{
		"%IW0": &object.Word{Value: 0x0029},
		"%IB2": &object.Byte{Value: 0},
		"%ID1": &object.DWord{Value: 0x00001234},
	}
}

func TestIOWordsCompute(t *testing.T) {
	object.FinalizeBuiltins()
	for _, c := range ioWordCases {
		comp := compiler.New()
		if err := comp.Compile(parse(t, c.src)); err != nil {
			t.Errorf("%s: compile: %v", c.name, err)
			continue
		}
		m := New(comp.Bytecode())
		for k, v := range ioWords() {
			m.IO()[k] = v
		}
		if err := m.Run(); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := m.IO()[c.out]; got == nil || got.Inspect() != c.want {
			t.Errorf("%s: %s = %v, want %s", c.name, c.out, got, c.want)
		}
	}
}

func TestAsBitString(t *testing.T) {
	for _, c := range []struct {
		in    object.Object
		value uint64
		width int
	}{
		{&object.Byte{Value: 0xAB}, 0xAB, 8}, {&object.Word{Value: 0xABCD}, 0xABCD, 16},
		{&object.DWord{Value: 0xDEADBEEF}, 0xDEADBEEF, 32}, {&object.LWord{Value: 1 << 63}, 1 << 63, 64},
	} {
		b, ok := asBitString(c.in).(*object.BitString)
		if !ok || b.Value != c.value || b.Width != c.width {
			t.Errorf("%v: %v", c.in, asBitString(c.in))
		}
	}
	if i := (&object.LInt{Value: 5}); asBitString(i) != i {
		t.Error("an integer changed")
	}
}
