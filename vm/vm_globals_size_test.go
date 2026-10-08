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
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"strings"
	"testing"
)

// TestWithGlobalsSize runs programs with exactly GlobalsNeeded global slots,
// and checks that one slot fewer fails the run with an error.
func TestWithGlobalsSize(t *testing.T) {
	tests := []vmTestCase{
		{"10 + 20;", 30},
		{"VAR_GLOBAL g_one : INT := 1; g_two : INT := 2; END_VAR g_one + g_two;", 3},
		{"VAR x : INT := 5; r : REF_TO INT; END_VAR r := REF(x); r^ := 7; x;", 7},
		{`
		VAR_GLOBAL seed : INT := 50; END_VAR;
		FUNCTION minusOne : INT
			VAR_INPUT s : INT; END_VAR
			minusOne := s - 1;
		END_FUNCTION;
		PROGRAM P VAR_OUTPUT result : INT; END_VAR
			VAR_EXTERNAL seed : INT; END_VAR
			result := minusOne(seed);
		END_PROGRAM;
		P();
		P.result;`, 49},
	}

	for i, tt := range tests {
		bytecode := compileForTest(t, tt.input)
		n := GlobalsNeeded(bytecode)
		if n >= GlobalsSize {
			t.Fatalf("test %d: GlobalsNeeded = %d, want fewer than GlobalsSize", i, n)
		}

		machine := New(bytecode, WithGlobalsSize(n))
		if err := machine.Run(); err != nil {
			t.Fatalf("test %d: with %d globals: %s", i, n, err)
		}
		testExpectedObject(t, i, tt.expected, machine.LastPoppedStackElem())

		if n == 0 {
			continue
		}
		err := New(bytecode, WithGlobalsSize(n-1)).Run()
		if err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Errorf("test %d: with %d globals: got error %v, want global out of range", i, n-1, err)
		}
	}
}

func TestNewDefaultGlobalsSize(t *testing.T) {
	if got := len(New(emptyBytecode()).Globals()); got != GlobalsSize {
		t.Errorf("New: %d globals, want %d", got, GlobalsSize)
	}
	if got := len(NewWithBuiltins(emptyBytecode(), nil, WithGlobalsSize(8)).Globals()); got != 8 {
		t.Errorf("NewWithBuiltins with WithGlobalsSize(8): %d globals, want 8", got)
	}
}

func TestGlobalsNeeded(t *testing.T) {
	fn := &object.CompiledFunction{Instructions: code.Instructions(code.Make(code.OpGetGlobal, 9))}
	tests := []struct {
		name     string
		bytecode *compiler.Bytecode
		want     int
	}{
		{"empty", emptyBytecode(), 0},
		{"set and get", &compiler.Bytecode{Instructions: concat(
			code.Make(code.OpSetGlobal, 3),
			code.Make(code.OpGetGlobal, 1),
		)}, 4},
		{"reference", &compiler.Bytecode{Instructions: concat(
			code.Make(code.OpRef, code.RefGlobal, 6),
			code.Make(code.OpRef, code.RefLocal, 40),
		)}, 7},
		{"in a function", &compiler.Bytecode{
			Instructions: concat(code.Make(code.OpGetGlobal, 2)),
			Constants:    []object.Object{&object.LInt{Value: 1}, fn},
		}, 10},
		{"undecodable", &compiler.Bytecode{Instructions: code.Instructions{0xff}}, GlobalsSize},
		{"truncated", &compiler.Bytecode{Instructions: code.Make(code.OpGetGlobal, 1)[:2]}, GlobalsSize},
	}
	for _, tt := range tests {
		if got := GlobalsNeeded(tt.bytecode); got != tt.want {
			t.Errorf("%s: GlobalsNeeded = %d, want %d", tt.name, got, tt.want)
		}
	}
}
