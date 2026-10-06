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
	"strings"
	"testing"

	"github.com/apiarytech/beedance/object"
)

// The evaluator handles directly represented variables as the VM does
// (vm/vm_direct_test.go).
func TestDirectVariables(t *testing.T) {
	for k := range ioMap {
		delete(ioMap, k)
	}
	ioMap["%IW0"] = &object.BitString{Value: 0x0029, Width: 16}
	ioMap["%IX0.3"] = TRUE
	if r := testEval(t, "%QW3 := %IW0 OR 16#0100; %QX0.1 := %IX0.3 AND NOT %MX5.0; %QB2 := %IB9;"); isError(r) {
		t.Fatal(r.Inspect())
	}
	if w, ok := ioMap["%QW3"].(*object.BitString); !ok || w.Value != 0x0129 || w.Width != 16 {
		t.Errorf("%%QW3 = %v", ioMap["%QW3"])
	}
	if b, ok := ioMap["%QX0.1"].(*object.Boolean); !ok || !b.Value {
		t.Errorf("%%QX0.1 = %v", ioMap["%QX0.1"])
	}
	if b, ok := ioMap["%QB2"].(*object.BitString); !ok || b.Value != 0 || b.Width != 8 {
		t.Errorf("%%QB2 = %v", ioMap["%QB2"])
	}
	if ioTypes["%QW3"] != "WORD" || ioTypes["%IX0.3"] != "BOOL" {
		t.Errorf("types %v", ioTypes)
	}
	r := testEval(t, "%IW0 := 16#FF;")
	if e, ok := r.(*object.Error); !ok || !strings.Contains(e.Message, "is an input") {
		t.Errorf("writing an input: %v", r)
	}
}
