/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

func TestHostBinding(t *testing.T) {
	src := `PROGRAM Guard
		VAR_INPUT temp : REAL; END_VAR
		VAR_OUTPUT fan : BOOL; END_VAR
		VAR RETAIN starts : DINT; END_VAR
		VAR CONSTANT limit : REAL := 30.0; END_VAR
		VAR lamp AT %QX0.1 : BOOL; t : TON; END_VAR
		t(IN := temp > limit, PT := T#2s);
		fan := t.Q;
		lamp := fan;
	END_PROGRAM`
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	out, err := GoFileWith(program, Options{Package: "programs", HostBinding: true})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"package programs",
		`"github.com/apiarytech/royaljelly/core"`,
		"func NewGuard() *Guard {",
		"func (p *Guard) Variables() []core.Variable {",
		`{Name: "temp", Block: "VAR_INPUT", Type: "REAL", Address: "", Retain: false, Constant: false,`,
		`{Name: "starts", Block: "VAR", Type: "DINT", Address: "", Retain: true, Constant: false,`,
		`{Name: "limit", Block: "VAR", Type: "REAL", Address: "", Retain: false, Constant: true,`,
		`{Name: "lamp", Block: "VAR", Type: "BOOL", Address: "%QX0.1"`,
		"x, ok := v.(iec.DINT)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `Name: "t"`) {
		t.Error("a function block instance is listed as a variable")
	}

	// Without the option the output is what it always was.
	plain, err := GoFile(program)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(plain), "package main") || strings.Contains(string(plain), "Variables()") {
		t.Errorf("GoFile changed:\n%s", plain)
	}
}
