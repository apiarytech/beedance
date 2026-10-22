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

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

const debugSource = `TYPE Point : STRUCT x : INT; y : INT; END_STRUCT; END_TYPE
TYPE Color : (Red, Green); END_TYPE

FUNCTION Twice : INT
VAR_INPUT a : INT; END_VAR
VAR tmp : INT; END_VAR
tmp := a * 2;
Twice := tmp;
END_FUNCTION

FUNCTION_BLOCK Counter
VAR_INPUT step : INT; END_VAR
VAR_OUTPUT total : INT; END_VAR
VAR tmp : INT; END_VAR
METHOD Reset : BOOL
total := 0;
Reset := TRUE;
END_METHOD
tmp := total + step;
total := tmp;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    n : INT;
    c : Counter;
    t : TON;
    p : Point;
    col : Color;
    a : ARRAY[1..3] OF INT;
    x AT %IX0.0 : BOOL;
    y AT %QX0.0 : BOOL;
    w AT %QW1 : INT;
END_VAR
n := Twice(n) + 1;
c(step := 2);
t(IN := TRUE, PT := T#50ms);
IF t.Q THEN
    y := x;
END_IF;
w := n;
END_PROGRAM
`

// TestDebugHooks: with Options.Debug, the file has a hook before each
// statement, hooks around each body, the variable tables and the located
// variables' points; without it, nothing of the kind.
func TestDebugHooks(t *testing.T) {
	parse := func() *ast.Program {
		p := parser.New(lexer.New(debugSource))
		program := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		return program
	}
	plain, err := GoFileWith(parse(), Options{Package: "p", HostBinding: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "dbgAt(") || strings.Contains(string(plain), "dbgVars") {
		t.Error("hooks in a file transpiled without Debug")
	}
	code, err := GoFileWith(parse(), Options{Package: "p", HostBinding: true, Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	src := string(code)
	line := func(text string) int {
		for i, l := range strings.Split(debugSource, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), text) {
				return i + 1
			}
		}
		t.Fatalf("no line %q", text)
		return 0
	}
	for _, want := range []string{
		"dbgAt(" + itoa(line("n := Twice")) + ", ",
		"dbgAt(" + itoa(line("tmp := a * 2")) + ", ",
		"dbgAt(" + itoa(line("total := 0")) + ", ",
		"dbgAt(" + itoa(line("y := x")) + ", ",
		"dbgAt(" + itoa(line("IF t.Q")) + ", ",
		`dbgEnter("Main", p.dbgVars)`,
		`dbgEnter("Counter", c.dbgVars)`,
		`dbgEnter("Counter.Reset", func() []dbgVar`,
		`dbgEnter("Twice", func() []dbgVar`,
		"func (p *Main) dbgVars() []dbgVar",
		"func (p *Counter) dbgVars() []dbgVar",
		"func (p *Point) dbgVars() []dbgVar",
		`dbgVar{Name: "c", Type: "Counter", Address: "", Members: p.c.dbgVars}`,
		`dbgVar{Name: "t", Type: "TON", Address: "", Members: func() []dbgVar`,
		`dbgVar{Name: "x", Type: "BOOL", Address: "%IX0.0"`,
		`dbgVar{Name: "col", Type: "Color", Address: "", Text: func() string { return strconv.FormatInt(int64(p.col), 10) }`,
		`Name: "[" + strconv.FormatInt(int64(i)+1, 10) + "]"`,
		`{Address: "%IX0.0", Type: "BOOL"`,
		`{Address: "%QW1", Type: "INT"`,
		"func dbgPoints() []dbgPoint",
		"var dbgHook dbgHooks",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the debug file lacks %q", want)
		}
	}
	if t.Failed() {
		t.Log(src)
	}
}

func itoa(n int) string { return strings.TrimSpace(strings.Repeat(" ", 0) + fmtInt(n)) }

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
