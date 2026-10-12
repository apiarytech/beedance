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

	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/object"
)

// Every X_TO_Y compiles to the one generic built-in $CONVERT and gives what
// the evaluator gives for it, and what it is expected to be.
func TestConversionsAgreeWithEvaluator(t *testing.T) {
	tests := []struct{ input, want string }{
		{"INT_TO_REAL(3);", "3.000000"},
		{"REAL_TO_INT(2.6);", "3"},
		{"REAL_TO_INT(-2.5);", ""},
		{"DINT_TO_INT(40000);", ""},
		{"INT_TO_SINT(300);", ""},
		{"UINT_TO_INT(65535);", ""},
		{"LREAL_TO_REAL(1.5);", "1.500000"},
		{"BOOL_TO_INT(TRUE);", "1"},
		{"INT_TO_BOOL(0);", "false"},
		{"INT_TO_BCD(42);", ""},
		{"BCD_TO_INT(16#42);", ""},
		{"ANY_INT_TO_ANY_REAL(7);", ""},
		{"WORD_TO_INT(WORD#16#FFFF);", ""},
		{"DWORD_TO_REAL(DWORD#16#3F800000);", ""},
		{"BYTE_TO_WORD(BYTE#16#7F);", ""},
		{"TIME_TO_DINT(T#1s500ms);", ""},
		{"DINT_TO_TIME(250);", ""},
		{"INT_TO_STRING(-12);", ""},
		{"STRING_TO_INT('42');", "42"},
		{"real_to_int(1.4);", "1"}, // any case
		{"INT_TO_REAL(INT_TO_REAL(2));", ""},
	}
	for _, tt := range tests {
		bytecode := compileForTest(t, tt.input)
		// The types travel as string constants, $CONVERT by its index.
		from, to, ok := object.ParseConversion(tt.input[:strings.Index(tt.input, "(")])
		if !ok || !hasConstant(bytecode.Constants, from) || !hasConstant(bytecode.Constants, to) {
			t.Errorf("%s: the types %q and %q are not constants of the bytecode", tt.input, from, to)
		}
		machine := New(bytecode)
		if err := machine.Run(); err != nil {
			t.Errorf("%s: vm: %v", tt.input, err)
			continue
		}
		got := machine.LastPoppedStackElem().Inspect()
		ev := evaluator.Eval(parse(t, tt.input), object.NewEnvironment())
		if ev == nil || ev.Inspect() != got {
			t.Errorf("%s: vm %s, evaluator %v", tt.input, got, ev)
		}
		if tt.want != "" && got != tt.want {
			t.Errorf("%s = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func hasConstant(constants []object.Object, s string) bool {
	for _, c := range constants {
		if str, ok := c.(*object.String); ok && str.Value == s {
			return true
		}
	}
	return false
}

// A function the program declares under a conversion's name is called, not
// the conversion; a conversion with the wrong number of arguments fails.
func TestConversionNamesAndErrors(t *testing.T) {
	src := `
	FUNCTION INT_TO_BCD : INT
		VAR_INPUT x : INT; END_VAR
		INT_TO_BCD := x * 100;
	END_FUNCTION;
	INT_TO_BCD(7);`
	machine := New(compileForTest(t, src))
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}
	if got := machine.LastPoppedStackElem().Inspect(); got != "700" {
		t.Errorf("user INT_TO_BCD(7) = %s, want 700", got)
	}

	machine = New(compileForTest(t, "INT_TO_REAL(1, 2);"))
	err := machine.Run()
	got := machine.LastPoppedStackElem()
	if err == nil && (got == nil || !strings.Contains(got.Inspect(), "wrong number of arguments for INT_TO_REAL")) {
		t.Errorf("INT_TO_REAL(1, 2): err %v, result %v", err, got)
	}
}

// Only the one generic built-in is registered for conversions.
func TestConversionsRegisterOneBuiltin(t *testing.T) {
	object.FinalizeBuiltins()
	n := 0
	for _, e := range object.Builtins {
		if _, _, ok := object.ParseConversion(e.Name); ok {
			n++
		}
	}
	if n != 0 {
		t.Errorf("%d X_TO_Y built-ins registered, want 0 ($CONVERT does them all)", n)
	}
	if b, ok := object.GetBuiltinByName("INT_TO_REAL"); !ok || b == nil {
		t.Error("GetBuiltinByName(INT_TO_REAL) not found")
	}
	if _, ok := object.GetBuiltinByName("INT_TO_INT"); ok {
		t.Error("GetBuiltinByName(INT_TO_INT) found: not a conversion")
	}
}
