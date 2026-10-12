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
	"strings"
	"testing"

	"github.com/apiarytech/beedance/object"
)

// A small stack and call depth run what fits in them, and fail what does not
// with an error instead of a panic.
func TestWithStackSizeAndMaxFrames(t *testing.T) {
	const recursive = `
	FUNCTION down : INT
		VAR_INPUT n : INT; END_VAR
		IF n <= 0 THEN down := 0; ELSE down := down(n - 1) + 1; END_IF;
	END_FUNCTION;
	down(%s);`
	tests := []struct {
		depth      string
		opts       []Option
		want       int64
		wantErrHas string
	}{
		{"10", []Option{WithStackSize(64), WithMaxFrames(16)}, 10, ""},
		{"40", []Option{WithStackSize(64), WithMaxFrames(16)}, 0, "call stack overflow"},
		{"40", []Option{WithStackSize(16), WithMaxFrames(256)}, 0, "stack overflow"},
		{"40", nil, 40, ""}, // the defaults
	}
	for _, tt := range tests {
		bytecode := compileForTest(t, strings.Replace(recursive, "%s", tt.depth, 1))
		machine := New(bytecode, tt.opts...)
		err := machine.Run()
		if tt.wantErrHas != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErrHas) {
				t.Errorf("down(%s) with %d options: err %v, want %q", tt.depth, len(tt.opts), err, tt.wantErrHas)
			}
			continue
		}
		if err != nil {
			t.Fatalf("down(%s): %v", tt.depth, err)
		}
		if got, _, ok := object.GetIntegerObjectValue(machine.LastPoppedStackElem()); !ok || got != tt.want {
			t.Errorf("down(%s) = %s, want %d", tt.depth, machine.LastPoppedStackElem().Inspect(), tt.want)
		}
	}
}

// NewWithGlobalsStore takes the sizes too; its store stays the one given.
func TestNewWithGlobalsStoreOptions(t *testing.T) {
	store := make([]object.Object, 4)
	machine := NewWithGlobalsStore(emptyBytecode(), store, WithStackSize(32), WithMaxFrames(4), WithGlobalsSize(99))
	if len(machine.stack) != 32 || len(machine.frames) != 4 || len(machine.Globals()) != 4 {
		t.Errorf("stack %d, frames %d, globals %d: want 32, 4, 4", len(machine.stack), len(machine.frames), len(machine.Globals()))
	}
	if machine := NewWithGlobalsStore(emptyBytecode(), store); len(machine.stack) != StackSize || len(machine.frames) != MaxFrames {
		t.Errorf("defaults: stack %d, frames %d", len(machine.stack), len(machine.frames))
	}
}
