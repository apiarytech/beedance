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

	"github.com/apiarytech/beedance/object"
)

// Directly represented variables used in statements: read from and
// written to the I/O image by address, with the type their size gives.
func TestDirectVariables(t *testing.T) {
	m := New(compileForTest(t, "%QW3 := %IW0; %QX0.1 := %IX0.3 AND NOT %MX5.0; %QB2 := %IB9;"))
	m.IO()["%IW0"] = &object.Word{Value: 0x0029}
	m.IO()["%IX0.3"] = True
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if w, ok := m.IO()["%QW3"].(*object.Word); !ok || w.Value != 0x0029 {
		t.Errorf("%%QW3 = %v", m.IO()["%QW3"])
	}
	if b, ok := m.IO()["%QX0.1"].(*object.Boolean); !ok || !b.Value {
		t.Errorf("%%QX0.1 = %v", m.IO()["%QX0.1"])
	}
	// an address nothing set reads as its type's zero
	if b, ok := m.IO()["%QB2"].(*object.Byte); !ok || b.Value != 0 {
		t.Errorf("%%QB2 = %v", m.IO()["%QB2"])
	}
}

func TestDirectZero(t *testing.T) {
	for addr, want := range map[string]object.ObjectType{
		"%IX0.0": object.BOOLEAN_OBJ, "%I0.0": object.BOOLEAN_OBJ, "%IB1": object.BYTE_OBJ,
		"%IW2": object.WORD_OBJ, "%ID3": object.DWORD_OBJ, "%IL4": object.LWORD_OBJ,
	} {
		if got := directZero(addr).Type(); got != want {
			t.Errorf("%s: %s, want %s", addr, got, want)
		}
	}
	if directZero("Main.path") != Null {
		t.Error("an access path should start NULL")
	}
}
