/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// A program's direct variables are listed with its variables, named by
// their address, so a host binds them as it binds located variables; one
// declared AT the same address is listed once.
func TestDirectVariablesListed(t *testing.T) {
	object.FinalizeBuiltins()
	src := `
FUNCTION_BLOCK Pump VAR_OUTPUT run : BOOL; END_VAR run := %IX1.0; END_FUNCTION_BLOCK
PROGRAM Main
VAR level AT %IW0 : WORD; p : Pump; END_VAR
%QW3 := %IW0 OR 16#0100;
%QX0.1 := %IX0.3;
p();
END_PROGRAM`
	cp, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(parse(t, src), "Main")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, v := range cp.Variables {
		if v.Address != "" {
			got[v.Name+" "+v.Address] = v.Type
		}
	}
	want := map[string]string{
		"level %IW0": "WORD", "%QW3 %QW3": "WORD", "%QX0.1 %QX0.1": "BOOL", "%IX0.3 %IX0.3": "BOOL", "%IX1.0 %IX1.0": "BOOL",
	}
	for k, typ := range want {
		if got[k] != typ {
			t.Errorf("%s: %q, want %q (all: %v)", k, got[k], typ, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("listed %v", got)
	}
}

func TestDirectVariableErrors(t *testing.T) {
	object.FinalizeBuiltins()
	for src, want := range map[string]string{
		"PROGRAM Main %IW0 := 16#FF; END_PROGRAM":  "is an input",
		"PROGRAM Main %IX0.0 := TRUE; END_PROGRAM": "is an input",
	} {
		_, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(parse(t, src), "Main")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", src, err)
		}
	}
}

func TestDirectType(t *testing.T) {
	for addr, want := range map[string]string{
		"%IX0.0": "BOOL", "%I0.0": "BOOL", "%QX1": "BOOL", "%MB4": "BYTE", "%IW0": "WORD",
		"%QD2": "DWORD", "%ML8": "LWORD", "iw0": "WORD", "%IW1.2.3": "WORD",
	} {
		if got, ok := ast.DirectType(addr); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", addr, got, ok, want)
		}
	}
	for _, bad := range []string{"%XW0", "%IZ0", "%IW", "IW0x", "Main.level", ""} {
		if _, ok := ast.DirectType(bad); ok {
			t.Errorf("%q taken for an address", bad)
		}
	}
}
