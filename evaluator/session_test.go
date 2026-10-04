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
	"time"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// TestSession runs a program scan by scan on a simulated clock: a TON
// follows the session's clock, not the wall clock.
func TestSession(t *testing.T) {
	t0 := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	now := t0
	end := Session(func() time.Time { return now })
	defer end()

	src := `PROGRAM Delay VAR_INPUT go : BOOL; END_VAR VAR_OUTPUT q : BOOL; END_VAR VAR t : TON; END_VAR
		t(IN := go, PT := T#2s); q := t.Q; END_PROGRAM`
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	env := object.NewEnvironment()
	if res := Eval(program, env); isError(res) {
		t.Fatal(res.Inspect())
	}
	call := parser.New(lexer.New("Delay();")).ParseProgram()
	prog, _ := env.Get("Delay")
	pe := prog.(*object.Program).Env
	pe.Set("go", TRUE)
	q := func() bool {
		if res := Eval(call, env); isError(res) {
			t.Fatal(res.Inspect())
		}
		v, _ := pe.Get("q")
		return v == TRUE
	}
	if q() {
		t.Fatal("Q at 0 s")
	}
	now = t0.Add(1 * time.Second)
	if q() {
		t.Fatal("Q at 1 s")
	}
	now = t0.Add(2 * time.Second)
	if !q() {
		t.Fatal("no Q at 2 s of the session's clock")
	}
}

// TestBudget: an endless loop, even an empty one, ends with an error when
// the budget is spent.
func TestBudget(t *testing.T) {
	end := Session(time.Now)
	defer end()
	for _, src := range []string{
		"VAR n : DINT; END_VAR WHILE TRUE DO n := n + 1; END_WHILE;",
		"WHILE TRUE DO END_WHILE;",
		"REPEAT UNTIL FALSE END_REPEAT;",
	} {
		SetBudget(1000)
		out := Eval(parser.New(lexer.New(src)).ParseProgram(), object.NewEnvironment())
		if !isError(out) || !strings.Contains(out.Inspect(), "execution budget exceeded") {
			t.Errorf("%s: %v, want the budget error", src, out)
		}
	}
	SetBudget(1000)
	if out := Eval(parser.New(lexer.New("VAR n : INT; END_VAR n := 1 + 1; n;")).ParseProgram(), object.NewEnvironment()); isError(out) {
		t.Errorf("a short program: %v", out.Inspect())
	}
}

// TestSessionIO: a host sets an input before the program declares it, the
// declaration keeps it, an input the host has not set starts with its type's
// default, and IOTypes gives each address's declared type.
func TestSessionIO(t *testing.T) {
	end := Session(time.Now)
	defer end()
	IO()["%IW1"] = &object.Int{Value: 41}
	src := "VAR w AT %IW1 : INT; b AT %IX0.2 : BOOL; o AT %QW3 : INT; END_VAR o := w + 1;"
	if res := Eval(parser.New(lexer.New(src)).ParseProgram(), object.NewEnvironment()); isError(res) {
		t.Fatal(res.Inspect())
	}
	if got := IO()["%QW3"]; got == nil || got.Inspect() != "42" {
		t.Errorf("%%QW3 = %v, want 42", got)
	}
	if got := IO()["%IX0.2"]; got != FALSE {
		t.Errorf("%%IX0.2 = %v, want FALSE", got)
	}
	want := map[string]string{"%IW1": "INT", "%IX0.2": "BOOL", "%QW3": "INT"}
	for address, typ := range want {
		if got := IOTypes()[address]; got != typ {
			t.Errorf("IOTypes()[%s] = %q, want %q", address, got, typ)
		}
	}
}
