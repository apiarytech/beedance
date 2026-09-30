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
	"bytes"
	"fmt"
	"strings"
	"testing"

	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
	"beedance/token"
)

// The programs below were checked by compiling the generated Go against
// royaljelly and running it; these tests pin the generated code.

const statementsProgram = `
TYPE
    Color : (Red, Green, Blue);
END_TYPE

FUNCTION Twice : INT
    VAR_INPUT x : INT; END_VAR
    Twice := x * 2;
END_FUNCTION

FUNCTION Bump : INT
    VAR_IN_OUT io : INT; END_VAR
    io := io + 1;
    Bump := io;
END_FUNCTION

INTERFACE IShape
    METHOD Area : INT VAR_INPUT scale : INT; END_VAR END_METHOD
    PROPERTY Sides : INT GET SET;
END_INTERFACE

FUNCTION_BLOCK Square IMPLEMENTS IShape
    VAR side : INT := 2; n : INT := 4; END_VAR
    METHOD Area : INT VAR_INPUT scale : INT; END_VAR Area := side * side * scale; END_METHOD
    PROPERTY Sides : INT GET Sides := n; END_GET SET n := value; END_SET END_PROPERTY
END_FUNCTION_BLOCK

PROGRAM Main
    VAR
        a : INT := 3;
        b : INT;
        c : Color := Green;
        arr : ARRAY[0..3] OF INT;
        sq : Square;
        r1 : INT;
        t1 : TIME;
        k : INT;
    END_VAR
    CASE a OF
        1: b := 10;
        2, 3: b := 20;
    ELSE
        b := 0;
    END_CASE
    CASE a OF
        1..2: b := b + 1;
        3..5: b := b + 2;
    ELSE
        b := b + 3;
    END_CASE
    CASE c OF
        Color#Red: b := b + 100;
        Color#Green: b := b + 200;
    END_CASE
    arr := [1, 2, 3, 4];
    arr[0] := INT#7;
    t1 := T#5s;
    r1 := Twice(a) + Bump(k) + sq.Area(2);
    sq.Sides := 5;
    IF a > 2 AND NOT (b = 0) THEN
        a := -a;
    ELSIF a = 0 THEN
        a := 1;
    ELSE
        a := 2;
    END_IF
    WHILE a < 0 DO a := a + 1; END_WHILE
    REPEAT a := a + 1; UNTIL a >= 3 END_REPEAT
    FOR k := 1 TO 3 BY 1 DO b := b + k; END_FOR
END_PROGRAM
`

const ilProgram = `
FUNCTION Triple : INT
    VAR_INPUT x : INT; END_VAR
    Triple := x * 3;
END_FUNCTION

PROGRAM IlAll
    VAR
        a : INT := 4;
        b : INT;
        flag : BOOL;
        done : BOOL;
        n : INT;
    END_VAR
    LD a
    SUB 1
    MUL 2
    DIV 3
    ST b
    LD a
    GE 4
    ST flag
    LD flag
    S done
    LD FALSE
    R done
    LD a
    EQ 4
    JMPC skip
    LD 100
    ST b
skip:
    LD a
    NE 4
    JMPCN over
    LD 200
    ST b
over:
    LD a
    LT 10
    ST flag
    LD a
    LE 3
    JMP tail
tail:
    CAL Triple(x := a)
    ST n
    LD (LD a ADD 1)
    ST a
    LD TRUE
    RETC
    LD 999
    ST a
END_PROGRAM
`

const structsProgram = `
TYPE
    Inner : STRUCT v : INT := 3; END_STRUCT;
    Outer : STRUCT inn : Inner; w : INT; END_STRUCT;
END_TYPE

FUNCTION_BLOCK Acc
    VAR_INPUT amount : INT; END_VAR
    VAR_IN_OUT total : INT; END_VAR
    VAR_OUTPUT done : BOOL; END_VAR
    total := total + amount;
    done := TRUE;
END_FUNCTION_BLOCK

PROGRAM Shared
    VAR_IN_OUT counter : INT; END_VAR
    counter := counter + 1;
END_PROGRAM

PROGRAM Main5
    VAR
        o : Outer;
        acc : Acc;
        sum : INT := 1;
        ok : BOOL;
        sensor AT %IX0.0 : BOOL;
        vals : ARRAY[0..4] OF INT := [2(7), 3(1)];
    END_VAR
    o.inn.v := o.inn.v + 1;
    o.w := o.inn.v * 2;
    acc(amount := 5, total := sum, done => ok);
    vals[4] := vals[0] + vals[1];
END_PROGRAM
`

func TestStatementsProgram(t *testing.T) {
	checkContains(t, statementsProgram,
		// CASE with lists, ranges and enumerated values.
		"switch p.a { case 1: p.b = 10 case 2, 3: p.b = 20 default: p.b = 0 }",
		"if (caseSelector >= 1 && caseSelector <= 2) {",
		"case Color_Red: p.b = (p.b + 100)",
		// Array and typed literals.
		"p.arr = [4]iec.INT{1, 2, 3, 4}",
		"p.arr[0] = iec.INT(7)",
		// Time literals are values, parsed when transpiling.
		"p.t1 = iec.TIME(5000000000)",
		// Calls, including a positional VAR_IN_OUT and a method.
		"p.r1 = ((Twice(p.a) + Bump(&p.k)) + p.sq.Area(2))",
		// A property is set through its setter, whose input is `value`.
		"p.sq.SetSides(5)",
		"func (s *Square) SetSides(value iec.INT) { s.n = value }",
		// The control variable is assigned, and keeps its value after the loop.
		"for p.k = 1; p.k <= 3; p.k += 1 {",
		"var _ IShape = (*Square)(nil)",
	)
}

func TestIlProgram(t *testing.T) {
	checkContains(t, ilProgram,
		// Operands take the accumulator's type.
		"cr_LINT = cr_LINT - iec.LINT(1)",
		"cr_BOOL = cr_LINT >= iec.LINT(4)",
		// Set, reset, and the jumps.
		"if cr_BOOL { p.done = true }",
		"if cr_BOOL { p.done = false }",
		"if cr_BOOL { goto skip; }",
		"if !cr_BOOL { goto over; }",
		"goto tail",
		// CAL passes the named argument and converts the result.
		"cr_LINT = iec.LINT(Triple(p.a))",
		// A parenthesized operand has its own accumulator.
		"cr_LINT = func() iec.LINT {",
		// RETC returns only when the result is TRUE.
		"if cr_BOOL { return }",
	)
}

func TestStructsProgram(t *testing.T) {
	checkContains(t, structsProgram,
		// Nested structure members.
		"p.o.inn.v = (p.o.inn.v + 1)",
		// A function block's VAR_IN_OUT is a pointer, dereferenced in its body
		// and passed by address.
		"total *iec.INT",
		"(*a.total) = ((*a.total) + a.amount)",
		"p.acc.total = &p.sum",
		// A program's VAR_IN_OUT too.
		"(*p.counter) = ((*p.counter) + 1)",
		// Located variables are linked to the I/O image.
		// A located input is read from the process image as a scan starts.
		"p.sensor = iec.BOOL(img.I.B[0])",
		"instance.vals = [5]iec.INT{7, 7, 1, 1, 1}",
	)
}

const typesProgram = `
TYPE
    Pt : STRUCT px : INT; py : INT; END_STRUCT;
END_TYPE

FUNCTION MakePt : Pt
    VAR_INPUT x : INT; END_VAR
    MakePt := (px := x, py := 2);
END_FUNCTION

PROGRAM IlTypes
    VAR
        r : REAL := 1.5;
        rr : REAL;
        tm : TIME;
        s : STRING := 'ab';
        s2 : STRING;
        b1 : BOOL := TRUE;
        b2 : BOOL;
    END_VAR
    LD r
    ADD 2.0
    ST rr
    LD T#2s
    ADD T#1s
    ST tm
    LD s
    ST s2
    LD b1
    AND TRUE
    OR FALSE
    XOR TRUE
    ST b2
END_PROGRAM

PROGRAM Lits
    VAR
        w : WORD;
        u : UDINT;
        l : LREAL;
        p : Pt;
        ref : REFERENCE TO INT;
        target : INT := 5;
        copy : INT;
    END_VAR
    w := WORD#16#FF;
    u := UDINT#7;
    l := 2.5;
    p := MakePt(3);
    ref := target;
    copy := ref^ + 1;
END_PROGRAM
`

const macrosProgram = `
PROGRAM M
VAR twice : MACRO := macro(a) { EXPR(EVAL(a) * 2); }; thrice : MACRO := macro(a) { EXPR(EVAL(a) * 3); }; x : INT; END_VAR
x := twice(4) + thrice(x);
END_PROGRAM
`

func TestTypesProgram(t *testing.T) {
	checkContains(t, typesProgram,
		// IL accumulators follow the operand's type family.
		"cr_LREAL = cr_LREAL + iec.LREAL(2.000000)",
		"p.rr = iec.REAL(cr_LREAL)",
		"cr_STRING = iec.STRING(p.s)",
		// Boolean IL logic uses Go's logical operators.
		"cr_BOOL = cr_BOOL && iec.BOOL(true)",
		"cr_BOOL = cr_BOOL || iec.BOOL(false)",
		"cr_BOOL = cr_BOOL != iec.BOOL(true)",
		// Based typed literals become Go literals.
		"p.w = iec.WORD(0xFF)",
		"p.u = iec.UDINT(7)",
		// A structure initializer as a function result.
		"MakePt = Pt{px: x, py: 2}",
		// References.
		"p.ref = &p.target",
		"p.copy_ = ((*p.ref) + 1)",
	)
}

func TestMacrosProgram(t *testing.T) {
	// Macros are expanded before transpiling, whatever their name.
	checkContains(t, macrosProgram, "p.x = ((4 * 2) + (p.x * 3))")
}

func TestLiteralConversions(t *testing.T) {
	program := func(body string) string {
		return "PROGRAM P VAR b : BOOL; n : INT; w : BYTE; END_VAR " + body + " END_PROGRAM"
	}
	checkContains(t, program("b := BOOL#TRUE; b := BOOL#0; n := INT#2#1010; n := INT#8#17; n := INT#1_000; w := 16#0F AND w;"),
		"p.b = iec.BOOL(true)", "p.b = iec.BOOL(false)",
		"p.n = iec.INT(0b1010)", "p.n = iec.INT(0o17)", "p.n = iec.INT(1000)")
	if _, err := transpileSource(t, program("b := BOOL#2;")); err == nil || !strings.Contains(err.Error(), "invalid BOOL literal: BOOL#2") {
		t.Errorf("expected an invalid BOOL literal error, got %v", err)
	}
	if _, err := transpileSource(t, "PROGRAM P VAR t1 : TIME; END_VAR t1 := T#5x; END_PROGRAM"); err == nil || !strings.Contains(err.Error(), `unknown duration unit "x"`) {
		t.Errorf("expected a TIME literal error, got %v", err)
	}
	if got, err := timeDateLiteral("2026-01-02", "DATE"); err != nil || got != "iec.DATE(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))" {
		t.Errorf("DATE literal = %q, %v", got, err)
	}
	for width, want := range map[int]string{8: "iec.BYTE", 16: "iec.WORD", 32: "iec.DWORD", 64: "iec.LWORD"} {
		if got := bitStringGoType(width); got != want {
			t.Errorf("bitStringGoType(%d) = %s, want %s", width, got, want)
		}
	}
}

func TestForLoopDirections(t *testing.T) {
	checkContains(t, "PROGRAM P VAR i : INT; s : INT := -1; END_VAR FOR i := 10 TO 1 BY -2 DO END_FOR FOR i := 1 TO 5 BY s DO END_FOR END_PROGRAM",
		// A constant negative step counts down.
		"for p.i = 10; p.i >= 1; p.i += (-2) {",
		// A variable step is checked when the loop runs.
		"for p.i = 1; (p.s >= 0 && p.i <= 5) || (p.s < 0 && p.i >= 5); p.i += p.s {")
}

func TestArraysAndBounds(t *testing.T) {
	checkContains(t, "PROGRAM P VAR a : ARRAY[-1..1] OF INT; b : ARRAY[0..2*2] OF INT; c : ARRAY[0..5-1] OF INT; END_VAR a := [2(4), 5]; END_PROGRAM",
		// Constant expressions as bounds.
		"instance.a = [3]iec.INT{}",
		"instance.b = [5]iec.INT{}",
		"instance.c = [5]iec.INT{}",
		// Repetition in an array literal.
		"p.a = [3]iec.INT{4, 4, 5}")
}

func TestIlCallOfFunctionBlockWithInOut(t *testing.T) {
	checkContains(t, "FUNCTION_BLOCK Acc VAR_IN_OUT total : INT; END_VAR VAR_OUTPUT done : BOOL; END_VAR total := total + 1; done := TRUE; END_FUNCTION_BLOCK PROGRAM P VAR acc : Acc; sum : INT; END_VAR CAL acc(total := sum) END_PROGRAM",
		"p.acc.total = &p.sum p.acc.Logic(now) cr_BOOL = p.acc.done")
}

func TestUnsupportedDeclarations(t *testing.T) {
	tests := []struct{ input, want string }{
		{"FUNCTION_BLOCK ABSTRACT Ab END_FUNCTION_BLOCK PROGRAM P VAR a1 : Ab; END_VAR END_PROGRAM", "cannot instantiate abstract function block 'Ab' for variable 'a1'"},
	}
	for _, tt := range tests {
		if _, err := transpileSource(t, tt.input); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", tt.input, tt.want, err)
		}
	}
}

func TestCallEdgeCases(t *testing.T) {
	// EN and ENO do not become Go arguments or results.
	checkContains(t, "FUNCTION F : INT VAR_INPUT a : INT; END_VAR VAR_OUTPUT o : INT; END_VAR F := a; o := a; END_FUNCTION PROGRAM P VAR r : INT; ok : BOOL; END_VAR r := F(EN := TRUE, a := 2, ENO => ok); END_PROGRAM",
		"p.r = func() iec.INT { __r, _ := F(2); return __r }()")
	// A method without a result: called as a statement, or wrapped to bind its outputs.
	checkContains(t, "FUNCTION_BLOCK Fb VAR v : INT; END_VAR METHOD M VAR_INPUT a : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := a + v; END_METHOD METHOD N VAR_INPUT a : INT; END_VAR v := a; END_METHOD END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; x : INT; END_VAR f.N(3); f.M(a := 1, o => x); END_PROGRAM",
		"p.f.N(3)", "func() { __o0 := p.f.M(1); p.x = __o0 }()")
	// SUPER finds the method in an ancestor further up, and passes its arguments.
	checkContains(t, "FUNCTION_BLOCK G METHOD Sum : INT VAR_INPUT a : INT; b : INT; END_VAR Sum := a + b; END_METHOD END_FUNCTION_BLOCK FUNCTION_BLOCK Pa EXTENDS G END_FUNCTION_BLOCK FUNCTION_BLOCK Ch EXTENDS Pa METHOD Sum : INT VAR_INPUT a : INT; b : INT; END_VAR Sum := SUPER^.Sum(a, b) + 1; END_METHOD END_FUNCTION_BLOCK",
		"Sum = (c.G.Sum(a, b) + 1)")
	// IL CAL of a standard function block loads its primary output.
	checkContains(t, "PROGRAM P VAR t1 : TON; END_VAR CAL t1(IN := TRUE, PT := T#1s) END_PROGRAM",
		"p.t1.IN = true p.t1.PT = iec.TIME(1000000000) p.t1.Execute(now) cr_BOOL = p.t1.Q")

	tests := []struct{ input, want string }{
		{"FUNCTION_BLOCK Fb VAR x : INT; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR CAL f() END_PROGRAM", "CAL instruction used on function block with no outputs: f"},
		{"FUNCTION_BLOCK Fb METHOD M : INT M := SUPER^.M(); END_METHOD END_FUNCTION_BLOCK", "SUPER call used outside of a derived FUNCTION_BLOCK"},
		{"FUNCTION_BLOCK G END_FUNCTION_BLOCK FUNCTION_BLOCK Ch EXTENDS G METHOD M : INT M := SUPER^.Nope(); END_METHOD END_FUNCTION_BLOCK", "could not find method 'Nope' in any parent for SUPER call"},
		{"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR F := 1; END_FUNCTION PROGRAM P VAR x : INT; END_VAR x := F(nope => x); END_PROGRAM", "'F' has no output 'nope'"},
		{"FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION PROGRAM P VAR x : INT; END_VAR x := F(1, 2); END_PROGRAM", "'F' takes 1 arguments, got more"},
	}
	for _, tt := range tests {
		if _, err := transpileSource(t, tt.input); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", tt.input, tt.want, err)
		}
	}
}

const initsProgram = `
TYPE
    Pt : STRUCT px : INT := 5; END_STRUCT;
    Color : (Red, Green);
    Row : ARRAY[0..2] OF INT;
    Row7 : ARRAY[0..2] OF INT := [7];
    MyInt : INT;
    Big : MyInt := 9;
END_TYPE

FUNCTION Opt : INT
    VAR_INPUT flag : BOOL; name : STRING; vals : ARRAY[0..1] OF INT; n : INT := 3; END_VAR
    Opt := n;
END_FUNCTION

PROGRAM Inits
    VAR
        q : Pt;
        p : Pt := q;
        c : Color;
        r1 : Row;
        r7 : Row7;
        m : MyInt;
        b : Big;
        pts : ARRAY[0..2] OF Pt := [(px := 1)];
        src : ARRAY[0..1] OF INT := [4, 5];
        dst : ARRAY[0..1] OF INT := src;
        res : INT;
    END_VAR
    res := Opt(n := 2);
END_PROGRAM
`

func TestInitialValueEdgeCases(t *testing.T) {
	checkContains(t, initsProgram,
		// A structure copied from another variable.
		"instance.p = instance.q",
		// Array types, with and without their own initial value.
		"instance.r1 = [3]iec.INT{}",
		"instance.r7 = [3]iec.INT{7}",
		// An alias of an alias takes the nearest initial value.
		"instance.b = 9",
		// Padding with a non-zero default fills the rest in a loop.
		"__a[__i] = Pt{px: 5}",
		// Omitted inputs of every kind take their defaults.
		"p.res = Opt(false, \"\", [2]iec.INT{}, 2)",
	)
	if _, err := transpileSource(t, "TYPE A : B; B : A; END_TYPE PROGRAM P VAR x : A; END_VAR END_PROGRAM"); err == nil || !strings.Contains(err.Error(), "contains itself") {
		t.Errorf("expected a type cycle error, got %v", err)
	}
}

// TestErrorsPropagateFromEveryPosition puts an invalid literal (T#5x) in each
// kind of position and checks the error reaches the caller, instead of being
// dropped and leaving incomplete Go code.
func TestErrorsPropagateFromEveryPosition(t *testing.T) {
	const bad = "T#5x"
	templates := []string{
		// Declarations.
		"VAR_GLOBAL t1 : TIME := %s; END_VAR",
		"PROGRAM P VAR t1 : TIME := %s; END_VAR END_PROGRAM",
		"FUNCTION_BLOCK Fb VAR_INPUT t1 : TIME := %s; END_VAR END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Fb VAR_TEMP t1 : TIME := %s; END_VAR END_FUNCTION_BLOCK",
		"FUNCTION F : INT VAR t1 : TIME := %s; END_VAR F := 1; END_FUNCTION",
		"FUNCTION F : INT VAR_OUTPUT t1 : TIME := %s; END_VAR F := 1; END_FUNCTION",
		"FUNCTION F : INT VAR_INPUT t1 : TIME := %s; END_VAR F := 1; END_FUNCTION PROGRAM P VAR x : INT; END_VAR x := F(); END_PROGRAM",
		"FUNCTION_BLOCK Fb METHOD M : INT VAR t1 : TIME := %s; END_VAR M := 1; END_METHOD END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Fb METHOD M : INT VAR_OUTPUT t1 : TIME := %s; END_VAR M := 1; END_METHOD END_FUNCTION_BLOCK",
		"PROGRAM P VAR a : ARRAY[0..1] OF TIME := [%s]; END_VAR END_PROGRAM",
		"TYPE Pt : STRUCT t1 : TIME := %s; END_STRUCT; END_TYPE PROGRAM P VAR p : Pt; END_VAR END_PROGRAM",
		// Statements.
		"PROGRAM P VAR t1 : TIME; END_VAR t1 := %s; END_PROGRAM",
		"PROGRAM P VAR b : BOOL; END_VAR IF b THEN b := %s > T#1s; END_IF END_PROGRAM",
		"PROGRAM P VAR b : BOOL; END_VAR IF %s > T#1s THEN b := TRUE; END_IF END_PROGRAM",
		"PROGRAM P VAR b : BOOL; END_VAR IF b THEN b := TRUE; ELSIF %s > T#1s THEN b := FALSE; END_IF END_PROGRAM",
		"PROGRAM P VAR b : BOOL; END_VAR IF b THEN b := TRUE; ELSE b := %s > T#1s; END_IF END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR CASE 1 OF 1: t1 := %s; END_CASE END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR CASE 1 OF 1..2: t1 := %s; END_CASE END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR CASE 1 OF 1: ; ELSE t1 := %s; END_CASE END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; i : INT; END_VAR FOR i := 1 TO 2 DO t1 := %s; END_FOR END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR WHILE FALSE DO t1 := %s; END_WHILE END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR WHILE %s > T#1s DO END_WHILE END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR REPEAT t1 := %s; UNTIL TRUE END_REPEAT END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR REPEAT UNTIL %s > T#1s END_REPEAT END_PROGRAM",
		// Expressions and calls.
		"PROGRAM P VAR b : BOOL; END_VAR b := NOT (%s > T#1s); END_PROGRAM",
		"FUNCTION F : INT VAR_INPUT t1 : TIME; END_VAR F := 1; END_FUNCTION PROGRAM P VAR x : INT; END_VAR x := F(%s); END_PROGRAM",
		"FUNCTION F : INT VAR_INPUT t1 : TIME; END_VAR F := 1; END_FUNCTION PROGRAM P VAR x : INT; END_VAR x := F(t1 := %s); END_PROGRAM",
		"FUNCTION F : TIME F := %s; END_FUNCTION",
		"FUNCTION_BLOCK Fb METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; t1 : TIME; END_VAR t1 := %s; END_PROGRAM",
		"PROGRAM P VAR t1 : TON; END_VAR t1(IN := TRUE, PT := %s); END_PROGRAM",
		// OOP bodies.
		"FUNCTION_BLOCK Fb VAR t1 : TIME; END_VAR t1 := %s; END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Fb METHOD M : TIME M := %s; END_METHOD END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Fb PROPERTY Pr : TIME GET Pr := %s; END_GET END_PROPERTY END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Fb VAR t1 : TIME; END_VAR PROPERTY Pr : TIME SET t1 := %s; END_SET END_PROPERTY END_FUNCTION_BLOCK",
		// IL.
		"PROGRAM P VAR t1 : TIME; END_VAR LD %s ST t1 END_PROGRAM",
		"FUNCTION F : INT VAR_INPUT t1 : TIME; END_VAR F := 1; END_FUNCTION PROGRAM P VAR x : INT; END_VAR CAL F(t1 := %s) END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR LD t1 ADD %s ST t1 END_PROGRAM",
		"PROGRAM P VAR b : BOOL; t1 : TIME; END_VAR LD t1 GT %s ST b END_PROGRAM",
		"PROGRAM P VAR t1 : TON; END_VAR CAL t1(IN := TRUE, PT := %s) END_PROGRAM",
		"FUNCTION_BLOCK Fb VAR_INPUT d : TIME; END_VAR VAR_OUTPUT q : BOOL; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR CAL f(d := %s) END_PROGRAM",
		// Loops, CASE labels, indexes and literals.
		"PROGRAM P VAR i : INT; t1 : TIME; END_VAR FOR i := 1 TO 2 BY 1 DO t1 := %s; END_FOR END_PROGRAM",
		"PROGRAM P VAR a : ARRAY[0..1] OF TIME; END_VAR a[0] := %s; END_PROGRAM",
		"PROGRAM P VAR a : ARRAY[0..1] OF TIME; END_VAR a := [%s, T#1s]; END_PROGRAM",
		"TYPE Pt : STRUCT t1 : TIME; END_STRUCT; END_TYPE PROGRAM P VAR p : Pt; END_VAR p.t1 := %s; END_PROGRAM",
		"TYPE Pt : STRUCT t1 : TIME; END_STRUCT; END_TYPE PROGRAM P VAR p : Pt; END_VAR p := (t1 := %s); END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; END_VAR CASE %s > T#1s OF TRUE: t1 := T#1s; END_CASE END_PROGRAM",
		"PROGRAM P VAR t1 : TIME; i : INT; END_VAR CASE (%s > T#1s) OF 1..2: t1 := T#1s; END_CASE END_PROGRAM",
		"PROGRAM P VAR i : INT; END_VAR FOR i := 1 TO (%s > T#1s) DO END_FOR END_PROGRAM",
		"PROGRAM P VAR i : INT; END_VAR FOR i := (%s > T#1s) TO 2 DO END_FOR END_PROGRAM",
		"PROGRAM P VAR i : INT; END_VAR FOR i := 1 TO 2 BY (%s > T#1s) DO END_FOR END_PROGRAM",
		"FUNCTION_BLOCK Fb VAR v : TIME; END_VAR PROPERTY Pr : TIME SET v := value; END_SET END_PROPERTY END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR f.Pr := %s; END_PROGRAM",
		"PROGRAM P VAR a : ARRAY[0..1] OF BOOL; END_VAR a[%s > T#1s] := TRUE; END_PROGRAM",
		"PROGRAM P VAR x : LREAL; END_VAR x := SQRT(%s > T#1s); END_PROGRAM",
		// Configurations.
		"PROGRAM Prg VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := %s, PRIORITY := 1); PROGRAM P1 WITH T1 : Prg; END_RESOURCE END_CONFIGURATION",
		// SFC.
		"PROGRAM Seq INITIAL_STEP S1: END_STEP TRANSITION FROM S1 TO S2 := %s > T#1s; END_TRANSITION STEP S2: END_STEP END_PROGRAM",
		"PROGRAM Seq VAR t1 : TIME; END_VAR INITIAL_STEP S1: A(N); END_STEP ACTION A: t1 := %s; END_ACTION END_PROGRAM",
		"PROGRAM Seq VAR t1 : TIME; END_VAR INITIAL_STEP S1: A(D, %s); END_STEP ACTION A: t1 := T#1s; END_ACTION END_PROGRAM",
		"PROGRAM Seq VAR t1 : TIME; END_VAR INITIAL_STEP S1: A(L, %s); END_STEP ACTION A: t1 := T#1s; END_ACTION END_PROGRAM",
		"PROGRAM Seq VAR t1 : TIME; END_VAR INITIAL_STEP S1: A(SD, %s); END_STEP ACTION A: t1 := T#1s; END_ACTION END_PROGRAM",
		"PROGRAM Seq VAR t1 : TIME; END_VAR INITIAL_STEP S1: A(SL, %s); END_STEP ACTION A: t1 := T#1s; END_ACTION END_PROGRAM",
	}
	for _, tmpl := range templates {
		input := fmt.Sprintf(tmpl, bad)
		if _, err := transpileSource(t, input); err == nil || !strings.Contains(err.Error(), `unknown duration unit "x"`) {
			t.Errorf("%s:\n  expected the literal's error, got %v", input, err)
		}
	}
}

func TestIlEdgeCases(t *testing.T) {
	// NOT negates logically for BOOL and bitwise otherwise.
	checkContains(t, "PROGRAM P VAR a : INT := 5; b : BOOL := TRUE; END_VAR LD b NOT ST b LD a NOT ST a END_PROGRAM",
		"cr_BOOL = !cr_BOOL", "cr_LINT = ^cr_LINT")

	tests := []struct{ input, want string }{
		{"PROGRAM P VAR a : INT; END_VAR LD 1 ST nope END_PROGRAM", "cannot determine type of ST target: nope"},
		{"PROGRAM P VAR a : INT; END_VAR ADD 1 ST a END_PROGRAM", "IL operator 'ADD' used before accumulator was loaded (LD)"},
		{"PROGRAM P VAR a : BOOL; END_VAR GT 1 ST a END_PROGRAM", "IL operator 'GT' used before accumulator was loaded (LD)"},
		{"PROGRAM P VAR a : INT; END_VAR SIN ST a END_PROGRAM", "IL function 'SIN' used before accumulator was loaded (LD)"},
	}
	for _, tt := range tests {
		if _, err := transpileSource(t, tt.input); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", tt.input, tt.want, err)
		}
	}
}

func TestFunctionScopeTypes(t *testing.T) {
	// A reference in a function is assigned an address.
	checkContains(t, "FUNCTION F : INT VAR r : REFERENCE TO INT; x : INT; END_VAR r := x; F := r^; END_FUNCTION",
		"r = &x F = (*r)")
	// A structure initializer as a function's result.
	checkContains(t, "TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE FUNCTION F : Pt F := (px := 3); END_FUNCTION",
		"F = Pt{px: 3}")
	// A property is set through its setter from outside the function block.
	checkContains(t, "FUNCTION_BLOCK Fb VAR v : INT; END_VAR PROPERTY Pr : INT SET v := value; END_SET END_PROPERTY END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR f.Pr := 3; END_PROGRAM",
		"p.f.SetPr(3)")
	// A qualified type name keeps its package-style path.
	checkContains(t, "PROGRAM P VAR q : Lib.Pt; END_VAR END_PROGRAM", "q Lib.Pt")
	if _, err := transpileSource(t, "PROGRAM P VAR x : INT; END_VAR y := (a := 1); END_PROGRAM"); err == nil || !strings.Contains(err.Error(), "cannot determine the type of y for the initializer (a := 1)") {
		t.Errorf("expected an untyped initializer error, got %v", err)
	}
}

// Literal nodes the parser does not currently produce are still transpiled.
func TestLiteralNodes(t *testing.T) {
	tests := []struct {
		exp  ast.Expression
		want string
	}{
		{&ast.UnsignedIntegerLiteral{Value: 7}, "7"},
		{&ast.LRealLiteral{Value: 1.5}, "1.500000"},
		{&ast.BitStringLiteral{Value: 255, Width: 8}, "iec.BYTE(255)"},
		{&ast.BitStringLiteral{Value: 1, Width: 64}, "iec.LWORD(1)"},
		{&ast.EnumeratedValueLiteral{TypeName: &ast.Identifier{Value: "Color"}, Value: &ast.Identifier{Value: "Red"}}, "Color_Red"},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		if err := New(&buf).transpileExpression(tt.exp); err != nil || buf.String() != tt.want {
			t.Errorf("%T: got %q (%v), want %q", tt.exp, buf.String(), err, tt.want)
		}
	}
	var buf bytes.Buffer
	if err := New(&buf).transpileExpression(&ast.StructLiteral{}); err == nil {
		t.Error("a structure initializer without a target type should be an error")
	}
}

// Function literals are an extension the parser does not enable; a FUNCTION
// variable holding one gets a Go func type.
func TestFunctionTypedVariables(t *testing.T) {
	intType := &ast.TypeSpecifier{Token: token.Token{Type: token.IDENT, Literal: "INT"}}
	fnType := &ast.TypeSpecifier{Token: token.Token{Type: token.FUNCTION, Literal: "FUNCTION"}}
	withLiteral := &ast.VarDeclStatement{Name: &ast.Identifier{Value: "f"}, DataType: fnType, Value: &ast.FunctionLiteral{
		Parameters: []*ast.FunctionParameter{{Name: &ast.Identifier{Value: "a"}, DataType: intType}},
		ReturnType: intType,
		Body:       &ast.BlockStatement{},
	}}
	without := &ast.VarDeclStatement{Name: &ast.Identifier{Value: "g"}, DataType: fnType}
	var buf bytes.Buffer
	tr := New(&buf)
	for _, decl := range []*ast.VarDeclStatement{withLiteral, without} {
		if err := tr.transpileVarDecl(decl); err != nil {
			t.Fatalf("transpileVarDecl: %s", err)
		}
	}
	if got := strings.Join(strings.Fields(buf.String()), " "); got != "f func(a iec.INT) iec.INT g func()" {
		t.Errorf("got %q", got)
	}
}

func TestInterfacesAndMembers(t *testing.T) {
	// An interface extending another, with in-outs and several results.
	checkContains(t, "INTERFACE IBase METHOD B : INT; END_INTERFACE INTERFACE IFull EXTENDS IBase METHOD Swap : BOOL VAR_IN_OUT x : INT; END_VAR VAR_OUTPUT o1 : INT; o2 : INT; END_VAR END_METHOD END_INTERFACE",
		"type IFull interface { IBase Swap(x *iec.INT) (iec.BOOL, iec.INT, iec.INT) }")
	// A function block's field is written directly.
	checkContains(t, "FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR f.v := 2; END_PROGRAM",
		"p.f.v = 2")
	// Declarations at the top level become Go package variables.
	checkContains(t, "VAR_GLOBAL g : INT := 4; END_VAR", "var g iec.INT = 4")
	if _, err := transpileSource(t, "FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR f := (v := 1); END_PROGRAM"); err == nil || !strings.Contains(err.Error(), "f is not a structure, so it cannot be assigned the initializer (v := 1)") {
		t.Errorf("expected an error for an initializer assigned to a function block, got %v", err)
	}
}

// Nodes the parser produces only in some layouts are handled when given
// directly: an assignment parsed as an expression, a declaration or a VAR
// block among statements, and an SFC body outside a program.
func TestStatementNodes(t *testing.T) {
	x := &ast.Identifier{Value: "x"}
	one := &ast.IntegerLiteral{Value: 1}
	decl := &ast.VarDeclStatement{Name: x, DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.IDENT, Literal: "INT"}}, Value: one}
	tests := []struct {
		node ast.Node
		want string
	}{
		{&ast.ExpressionStatement{Expression: &ast.InfixExpression{Operator: ":=", Left: x, Right: one}}, "x = 1"},
		{decl, "x iec.INT"},
		{&ast.VarBlockDeclaration{Declarations: []*ast.VarDeclStatement{decl}}, "var x iec.INT = 1"},
		{&ast.SFCProgram{}, ""},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		if err := New(&buf).transpileNode(tt.node); err != nil {
			t.Errorf("%T: %s", tt.node, err)
			continue
		}
		if got := strings.Join(strings.Fields(buf.String()), " "); got != tt.want {
			t.Errorf("%T: got %q, want %q", tt.node, got, tt.want)
		}
	}
	var buf bytes.Buffer
	if err := New(&buf).transpileNode(&ast.MacroLiteral{}); err == nil || !strings.Contains(err.Error(), "unhandled AST node type") {
		t.Errorf("expected an unhandled node error, got %v", err)
	}
}

func TestOperatorsAndIlCalls(t *testing.T) {
	// MOD and & (AND) have Go counterparts.
	checkContains(t, "PROGRAM P VAR a : INT := 7; b : BOOL := TRUE; c : BOOL; END_VAR a := a MOD 3; c := b & TRUE; END_PROGRAM",
		"p.a = (p.a % 3)", "p.c = (p.b && true)")
	// CAL of built-in and unknown functions loads their result.
	checkContains(t, "PROGRAM P VAR n : INT; END_VAR CAL LEN('ab') ST n END_PROGRAM", `cr_LINT = iec.LINT(iecstrings.LEN(iec.STRING("ab")))`)
	checkContains(t, "PROGRAM P VAR n : INT; END_VAR CAL Mystery(1) ST n END_PROGRAM", "cr_LINT = iec.LINT(Mystery(1))")
	// A method inherited from a parent is called on the derived instance.
	checkContains(t, "FUNCTION_BLOCK Base METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK FUNCTION_BLOCK Dv EXTENDS Base END_FUNCTION_BLOCK PROGRAM P VAR d : Dv; x : INT; END_VAR x := d.M(); END_PROGRAM",
		"p.x = p.d.M()")
	if _, err := transpileSource(t, "PROGRAM P VAR n : INT; END_VAR CAL n END_PROGRAM"); err == nil || !strings.Contains(err.Error(), "operand for CAL must be a function or function block call") {
		t.Errorf("expected a CAL operand error, got %v", err)
	}
}

func TestHelperFunctions(t *testing.T) {
	i := func(v int64) ast.Expression { return &ast.IntegerLiteral{Value: v} }
	constants := []struct {
		exp  ast.Expression
		want int64
		ok   bool
	}{
		{&ast.UnsignedIntegerLiteral{Value: 4}, 4, true},
		{&ast.PrefixExpression{Operator: "+", Right: i(3)}, 3, true},
		{&ast.PrefixExpression{Operator: "-", Right: i(3)}, -3, true},
		{&ast.InfixExpression{Operator: "+", Left: i(2), Right: i(3)}, 5, true},
		{&ast.InfixExpression{Operator: "/", Left: i(6), Right: i(3)}, 0, false},
		{&ast.PrefixExpression{Operator: "NOT", Right: i(1)}, 0, false},
		{&ast.Identifier{Value: "n"}, 0, false},
	}
	for _, c := range constants {
		if got, ok := constantInteger(c.exp); got != c.want || ok != c.ok {
			t.Errorf("constantInteger(%s) = %d, %v; want %d, %v", c.exp.String(), got, ok, c.want, c.ok)
		}
	}
	for goType, want := range map[string]string{"iec.BOOL": "false", "iec.STRING": `""`, "iec.INT": "0", "[]iec.INT": "nil", "*iec.INT": "nil", "Pt": "*new(Pt)"} {
		if got := zeroValue(goType); got != want {
			t.Errorf("zeroValue(%s) = %s, want %s", goType, got, want)
		}
	}
	for in, want := range map[string]string{"16#FF": "0xFF", "2#1010": "0b1010", "8#17": "0o17", "10#15": "15", "1_000": "1000"} {
		if got := goNumber(in); got != want {
			t.Errorf("goNumber(%s) = %s, want %s", in, got, want)
		}
	}
	tr := New(&bytes.Buffer{})
	if got := tr.mapIlOperatorToGo("NOPE"); got != "NOPE" {
		t.Errorf("an unknown IL operator should map to itself, got %s", got)
	}
	if got := tr.getBaseTypeFamily(&ast.Identifier{Value: "Color"}); got != "LINT" {
		t.Errorf("a user type's family should default to LINT, got %s", got)
	}
	if _, err := timeDateLiteral("5s", "INT"); err == nil {
		t.Error("an INT is not a time or date literal")
	}
	// A typed TIME or STRING literal node.
	for _, c := range []struct {
		lit  *ast.TypedLiteral
		want string
	}{
		{&ast.TypedLiteral{TypeName: "TIME", Value: &ast.Identifier{Value: "5s"}}, "iec.TIME(5000000000)"},
		{&ast.TypedLiteral{TypeName: "STRING", Value: &ast.Identifier{Value: "ab"}}, `"ab"`},
	} {
		var buf bytes.Buffer
		if err := New(&buf).transpileTypedLiteral(c.lit); err != nil || buf.String() != c.want {
			t.Errorf("transpileTypedLiteral(%s) = %q, %v; want %q", c.lit.TypeName, buf.String(), err, c.want)
		}
	}
}

func TestBitwiseOperatorsOnBitStrings(t *testing.T) {
	checkContains(t, "PROGRAM P VAR w : BYTE; v : BYTE; END_VAR w := w OR v; w := w XOR v; w := w AND v; END_PROGRAM",
		"p.w = (p.w | p.v)", "p.w = (p.w ^ p.v)", "p.w = (p.w & p.v)")
}

func TestFunctionBlockInitEdgeCases(t *testing.T) {
	// Init leaves references and located variables to be linked.
	checkContains(t, "FUNCTION_BLOCK Fb VAR r : REFERENCE TO INT; x AT %IX0.0 : BOOL; y : INT := 2; END_VAR END_FUNCTION_BLOCK",
		"func (f *Fb) Init() { f.y = 2 }")
	// A function's local instance is initialized on every call.
	checkContains(t, "FUNCTION_BLOCK Fb VAR y : INT := 2; END_VAR END_FUNCTION_BLOCK FUNCTION F : INT VAR f : Fb; END_VAR F := 1; END_FUNCTION",
		"var f Fb f.Init()")
	// A method's outputs may be left unbound.
	checkContains(t, "FUNCTION_BLOCK Fb METHOD M VAR_OUTPUT o : INT; END_VAR o := 1; END_METHOD END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR f.M(); END_PROGRAM",
		"p.f.M()")
	// A program's own VAR_GLOBAL is a Go package variable.
	checkContains(t, "PROGRAM P VAR_GLOBAL g : INT := 1; END_VAR g := 2; END_PROGRAM", "var g iec.INT = 1", "g = 2")
	// An access path is linked at run time.
	checkContains(t, "PROGRAM P VAR_ACCESS a : Nope.x : INT READ_ONLY; END_VAR END_PROGRAM", `if v, ok := resolve("Nope.x").(*iec.INT); ok {`)

	tests := []struct{ input, want string }{
		{"FUNCTION_BLOCK Base VAR bv : INT; END_VAR END_FUNCTION_BLOCK FUNCTION_BLOCK Dv EXTENDS Base END_FUNCTION_BLOCK PROGRAM P VAR d : Dv := (zz := 1); END_VAR END_PROGRAM", "function block 'Dv' has no variable 'zz'"},
		{"FUNCTION_BLOCK Fb VAR inner : Fb; END_VAR END_FUNCTION_BLOCK", "function block 'Fb' contains itself"},
		{"FUNCTION_BLOCK Other VAR x : TIME; END_VAR END_FUNCTION_BLOCK FUNCTION_BLOCK Fb VAR inner : Other := (x := T#5x); END_VAR END_FUNCTION_BLOCK", `unknown duration unit "x"`},
	}
	for _, tt := range tests {
		if _, err := transpileSource(t, tt.input); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", tt.input, tt.want, err)
		}
	}
}

func TestCallAndConfigHelpers(t *testing.T) {
	// VAR_CONFIG string values are passed to the program factory unquoted.
	checkContains(t, `PROGRAM Prg VAR name : STRING; wide : WSTRING; END_VAR END_PROGRAM
		CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prg; END_RESOURCE
		VAR_CONFIG Res.P1.name : STRING := 'pump'; Res.P1.wide : WSTRING := "w"; END_VAR END_CONFIGURATION`,
		`{"name": "pump", "wide": "w"}`, `instance.wide = iec.WSTRING(s)`)

	tr := New(&bytes.Buffer{})
	if err := tr.Transpile(parseForTest(t, "FUNCTION F : INT VAR_IN_OUT io : INT; END_VAR F := io; END_FUNCTION")); err != nil {
		t.Fatalf("transpile: %s", err)
	}
	// A FUNCTION's in-outs are known by the function's name.
	if !tr.isInOutArgument(&ast.Identifier{Value: "F"}, "io") || tr.isInOutArgument(&ast.Identifier{Value: "F"}, "x") {
		t.Error("isInOutArgument does not recognise F's VAR_IN_OUT io")
	}
	// A callee that is not a declared function is written as given.
	if got, err := tr.calleeString(&ast.Identifier{Value: "SQRT"}); err != nil || got != "SQRT" {
		t.Errorf("calleeString(SQRT) = %q, %v", got, err)
	}
}

// parseForTest parses input, failing the test on parse errors.
func parseForTest(t *testing.T, input string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors for %q: %v", input, p.Errors())
	}
	return program
}

const boundsProgram = `
TYPE Rec : STRUCT row1 : ARRAY[1..3] OF INT; END_STRUCT; END_TYPE

FUNCTION Second : INT
    VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR
    Second := v[2];
END_FUNCTION

PROGRAM Bounds
    VAR
        a : ARRAY[1..3] OF INT := [10, 20, 30];
        n : ARRAY[-1..1] OF INT := [1, 2, 3];
        m : ARRAY[1..2, 1..3] OF INT;
        x : Rec;
        i : INT;
        total : INT;
        mid : INT;
        first : INT;
    END_VAR
    FOR i := 1 TO 3 DO total := total + a[i]; END_FOR
    m[2][3] := n[-1] + n[1];
    x.row1[1] := 7;
    mid := Second(a);
    first := a[1];
END_PROGRAM
`

// Go slices start at 0, so indexes subtract the declared lower bound; a
// constant index is folded.
func TestArrayLowerBounds(t *testing.T) {
	checkContains(t, boundsProgram,
		"p.total = (p.total + p.a[(p.i - 1)])",
		"p.m[1][2] = (p.n[0] + p.n[2])",
		"p.x.row1[0] = 7",
		"Second = v[1]",
		"p.first = p.a[0]",
	)
}

// Arrays are Go arrays, so, like structures, they are assigned and passed by
// value. An array literal takes its type from its target or parameter.
func TestArraysAreValues(t *testing.T) {
	checkContains(t, `
TYPE Row : ARRAY[1..3] OF INT; END_TYPE
FUNCTION Second : INT VAR_INPUT v : ARRAY[1..3] OF INT; END_VAR Second := v[2]; END_FUNCTION
FUNCTION Fill : BOOL VAR_IN_OUT v : ARRAY[1..3] OF INT; END_VAR v[1] := 42; Fill := TRUE; END_FUNCTION
FUNCTION Pair : INT VAR_INPUT n : INT; END_VAR Pair := n; END_FUNCTION
PROGRAM Main
VAR
  a : ARRAY[1..3] OF INT := [1, 2, 3];
  b : ARRAY[1..3] OF INT;
  m : ARRAY[0..1, 0..2] OF INT;
  rw : Row;
  n : ARRAY[0..1] OF INT;
  res : INT;
  ok : BOOL;
END_VAR
  b := a;
  m[1] := [7, 8, 9];
  rw := [4, 5, 6];
  res := Second([4, 5, 6]) + Second(v := rw);
  ok := Fill(a);
END_PROGRAM
`,
		"type Row [3]iec.INT",
		"func Second(v [3]iec.INT) (Second iec.INT)",
		"func Fill(v *[3]iec.INT) (Fill iec.BOOL)",
		"m [2][3]iec.INT",
		"p.b = p.a",
		"p.m[1] = [3]iec.INT{7, 8, 9}",
		"p.rw = [3]iec.INT{4, 5, 6}",
		"p.res = (Second([3]iec.INT{4, 5, 6}) + Second(p.rw))",
		"p.ok = Fill(&p.a)")
	// An array whose bounds are not constant is a slice, and a literal
	// without a known target has the length of its elements.
	checkContains(t, "PROGRAM P VAR k : INT := 3; a : ARRAY[0..k] OF INT; END_VAR END_PROGRAM", "a []iec.INT")
	checkContains(t, "FUNCTION_BLOCK Fb VAR_INPUT v : ARRAY[0..1] OF INT; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; END_VAR f(v := [1, 2]); END_PROGRAM", "[...]iec.INT{1, 2}")
}

// A structure declared in place is a Go anonymous struct with its members'
// starting values, and a function block called without arguments runs.
func TestInlineStructuresAndBareCalls(t *testing.T) {
	checkContains(t, `FUNCTION_BLOCK Fb VAR n : INT; END_VAR n := n + 1; END_FUNCTION_BLOCK
PROGRAM P VAR rec : STRUCT a : INT := 5; b : BOOL; END_STRUCT; rec2 : STRUCT a : INT; END_STRUCT := (a := 2); f : Fb; END_VAR
rec.a := rec.a + 1; f(); END_PROGRAM`,
		"rec struct{ a iec.INT; b iec.BOOL }",
		"instance.rec = struct{ a iec.INT; b iec.BOOL }{a: 5}",
		"instance.rec2 = struct{ a iec.INT }{a: 2}",
		"p.rec.a = (p.rec.a + 1)",
		"p.f.Logic(now)")
}

// Standard function blocks and functions are royaljelly's.
func TestRoyaljellyStandardLibrary(t *testing.T) {
	checkContains(t, `PROGRAM P VAR t1 : TON; c : CTU; e : R_TRIG; sr1 : SR; x : INT; r : REAL; s : STRING; b : BYTE; END_VAR
t1(IN := TRUE, PT := T#1s); c(CU := t1.Q, PV := 3); e(CLK := c.Q); sr1(S1 := e.Q);
x := MAX(x, 5); x := LIMIT(0, x, 10); r := SQRT(2.0); r := SQRT(r); s := LEFT(s, 2); s := MID(s, 1, 2);
b := SHL(b, 2); x := REAL_TO_INT(r); x := ABS(-3); END_PROGRAM`,
		"t1 timers.TON", "c counters.CTU", "e triggers.R_TRIG", "sr1 triggers.SR_FB",
		"instance.c.EN = true", "instance.sr1.EN = true",
		"p.t1.Execute(now)", "p.c.Execute()", "p.e.R_TRIG()", "p.sr1.SR()",
		"p.x = iec.INT(stdValue(selection.MAX(p.x, 5)))",
		"p.x = iec.INT(selection.LIMIT(0, p.x, 10))",
		"p.r = iec.REAL(numerical.SQRT(iec.REAL(2.000000)))",
		"p.r = iec.REAL(numerical.SQRT(p.r))",
		"p.s = stdValue(iecstrings.LEFT(iec.STRING(p.s), iec.LINT(2)))",
		"p.b = iec.BYTE(bitwise.SHL(p.b, uint(2)))",
		"p.x = conversion.REAL_TO_INT(iec.REAL(p.r))",
		"p.x = iec.INT(numerical.ABS(iec.INT((-3))))",
		"func stdValue[T any](value T, _ error) T { return value }")
	// Standard function blocks without EN are not given one.
	out, err := transpileSource(t, "PROGRAM P VAR t1 : TON; END_VAR END_PROGRAM")
	if err != nil || strings.Contains(out, "t1.EN") {
		t.Errorf("TON has no EN, got %v:\n%s", err, out)
	}
	// Standard functions take their arguments in order.
	if _, err := transpileSource(t, "PROGRAM P VAR r : REAL; END_VAR r := SQRT(IN := 2.0); END_PROGRAM"); err == nil || !strings.Contains(err.Error(), "takes its arguments in order") {
		t.Errorf("expected an error for a named argument, got %v", err)
	}
}

// Located variables map to the process image; bad addresses are reported.
func TestProcessImageAddresses(t *testing.T) {
	for _, tt := range []struct{ address, goType, slot string }{
		{"%IX0.3", "iec.BOOL", "img.I.B[3]"},
		{"%IX2.1", "iec.BOOL", "img.I.B[17]"},
		{"%QX5", "iec.BOOL", "img.Q.B[5]"},
		{"%IB4", "iec.BYTE", "img.I.C[4]"},
		{"%MW3", "iec.INT", "img.M.W[3]"},
		{"%QW3", "iec.WORD", "img.Q.W[3]"},
		{"%QD1", "iec.DINT", "img.Q.D[1]"},
		{"%QL2", "iec.LWORD", "img.Q.L[2]"},
		{"%MD6", "iec.REAL", "img.M.R[6]"},
		{"%ML6", "iec.LREAL", "img.M.LR[6]"},
		{"%M7", "iec.STRING", "img.M.S[7]"},
		{"%M8", "iec.WSTRING", "img.M.WS[8]"},
	} {
		if _, slot, _, err := processImageSlot(tt.address, tt.goType); err != nil || slot != tt.slot {
			t.Errorf("%s (%s): got %s, %v; want %s", tt.address, tt.goType, slot, err, tt.slot)
		}
	}
	for _, tt := range []struct{ address, goType, want string }{
		{"%ZX0", "iec.BOOL", "must be in the I, Q or M area"},
		{"%IX*", "iec.BOOL", "must be a fixed address"},
		{"%IW0", "iec.BOOL", "a BOOL must be located at a bit address"},
		{"%IX0.9", "iec.BOOL", "is not a bit of a byte"},
		{"%IX0", "iec.INT", "must be located at a byte, word"},
		{"%IW1.2", "iec.INT", "must be a single number"},
		{"%IW300", "iec.INT", "outside royaljelly's process image"},
		{"%IX0", "iec.TIME", "cannot be located"},
	} {
		if _, _, _, err := processImageSlot(tt.address, tt.goType); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s (%s): expected an error containing %q, got %v", tt.address, tt.goType, tt.want, err)
		}
	}
	// A function block reads and writes its located variables too.
	checkContains(t, "FUNCTION_BLOCK Fb VAR x AT %IX0.0 : BOOL; y AT %QX0.1 : BOOL; END_VAR y := x; END_FUNCTION_BLOCK",
		"f.x = iec.BOOL(img.I.B[0])", "img.Q.B[1] = iec.BOOL(f.y)", "var processImage vars.ProcessImage")
	if _, err := transpileSource(t, "PROGRAM P VAR x AT %IW0 : BOOL; END_VAR END_PROGRAM"); err == nil || !strings.Contains(err.Error(), "'x': a BOOL must be located") {
		t.Errorf("expected a located address error, got %v", err)
	}
}

// GoFile writes a complete file importing the packages the code uses.
func TestGoFileImports(t *testing.T) {
	out, err := GoFile(parseForTest(t, "PROGRAM P VAR t1 : TON; s : STRING; END_VAR t1(IN := TRUE); s := CONCAT(s, 'x'); END_PROGRAM"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"package main", `"github.com/apiarytech/royaljelly/fb/timers"`, `"github.com/apiarytech/royaljelly/iec"`,
		`iecstrings "github.com/apiarytech/royaljelly/std/strings"`, `"time"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %s in:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), `"github.com/apiarytech/royaljelly/config"`) {
		t.Errorf("config is imported but not used:\n%s", out)
	}
	if _, err := goFileWithImports([]byte("func (")); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("expected a parse error, got %v", err)
	}
}

// A namespace's declarations take its path as a prefix, and references to
// them, qualified or from within the namespace, are renamed.
func TestNamespaces(t *testing.T) {
	checkContains(t, `NAMESPACE Lib
  TYPE Mode : (Idle, Busy); END_TYPE
  FUNCTION Twice : INT VAR_INPUT x : INT; END_VAR Twice := x * 2; END_FUNCTION
  FUNCTION GetMode : Mode GetMode := Mode#Busy; END_FUNCTION
  FUNCTION_BLOCK Cnt VAR_OUTPUT n : INT; m : Mode; END_VAR n := Twice(n + 1); m := GetMode(); n := Inner.Half(n); END_FUNCTION_BLOCK
  NAMESPACE Inner
    FUNCTION Half : INT VAR_INPUT x : INT; END_VAR Half := x / 2; END_FUNCTION
  END_NAMESPACE
END_NAMESPACE
NAMESPACE A.B FUNCTION F : INT F := 1; END_FUNCTION END_NAMESPACE
PROGRAM P
  VAR c : Lib.Cnt; m : Lib.Mode; y : INT; END_VAR
  y := Lib.Twice(3) + Lib.Inner.Half(8) + A.B.F() + c.n;
  c();
  m := Lib.Mode#Busy;
END_PROGRAM`,
		"type Lib_Mode int", "Lib_Mode_Busy",
		"func Lib_Twice(x iec.INT) (Lib_Twice iec.INT)",
		"func Lib_GetMode() (Lib_GetMode Lib_Mode)",
		"type Lib_Cnt struct",
		"l.n = Lib_Twice((l.n + 1))", "l.m = Lib_GetMode()", "l.n = Lib_Inner_Half(l.n)",
		"func A_B_F() (A_B_F iec.INT)",
		"c Lib_Cnt", "m Lib_Mode",
		"p.y = (((Lib_Twice(3) + Lib_Inner_Half(8)) + A_B_F()) + p.c.n)",
		"p.c.Logic(now)", "p.m = Lib_Mode_Busy")
	// A program without namespaces is unchanged.
	program := parseForTest(t, "PROGRAM P VAR x : INT; END_VAR END_PROGRAM")
	if flattenNamespaces(program) != program {
		t.Error("a program without namespaces was copied")
	}
}

// IEC names that Go cannot use as written, such as a Go keyword, a built-in
// the generated code calls, or a package it imports, gain a trailing
// underscore everywhere they are declared and used. Within a POU, a local
// named like the receiver or the scan time is renamed the same way.
func TestGoReservedNames(t *testing.T) {
	checkContains(t, `TYPE map : STRUCT len : INT; fallthrough : BOOL; END_STRUCT; END_TYPE
FUNCTION select : INT VAR_INPUT go : INT; min : INT; END_VAR select := go + min + MAX(go, 2); END_FUNCTION
FUNCTION_BLOCK Fb
  VAR_INPUT chan : INT; END_VAR
  VAR_OUTPUT defer : INT; END_VAR
  VAR_TEMP f : INT; now : INT; END_VAR
  METHOD goto : INT VAR_INPUT f : INT; END_VAR goto := f + chan; END_METHOD
  f := chan * 2; now := f; defer := goto(f := now);
END_FUNCTION_BLOCK
PROGRAM iec
  VAR m : map; fb : Fb; switch : INT; func : BOOL; append : INT; END_VAR
  VAR_TEMP p : INT; END_VAR
  m.len := select(go := 1, min := 2);
  m.fallthrough := TRUE;
  fb(chan := m.len);
  switch := fb.defer;
  p := switch;
  func := p > 0;
  append := LEN('abc');
END_PROGRAM
CONFIGURATION C RESOURCE R1 ON PLC TASK T1 (INTERVAL := T#10ms, PRIORITY := 1); PROGRAM go WITH T1 : iec; END_RESOURCE END_CONFIGURATION`,
		"type map_ struct { len_ iec.INT fallthrough_ iec.BOOL }",
		"func select_(go_ iec.INT, min_ iec.INT) (select_ iec.INT)",
		// A standard function keeps its name.
		"stdValue(selection.MAX(go_, 2))",
		"chan_ iec.INT", "defer_ iec.INT",
		// Locals named like the receiver f or the scan time.
		"var f_ iec.INT", "var now_ iec.INT", "f.defer_ = f.goto_(now_)",
		"func (f *Fb) goto_(f_ iec.INT) (goto_ iec.INT)",
		"type iec_ struct", "func (p *iec_) Logic(now time.Time)", "var p_ iec.INT",
		"p.m.len_ = select_(1, 2)", "p.fb.chan_ = p.m.len_", "p.switch_ = p.fb.defer_", "p.func_ = (p_ > 0)",
		`config.RegisterProgramFactory("iec_", Newiec_Factory)`, `&core.Program{Name: "go_"`)
	// Names Go can use are unchanged.
	checkContains(t, "PROGRAM Go VAR Len : INT; END_VAR Len := 1; END_PROGRAM", "type Go struct", "p.Len = 1")
}

// A function block calls its own methods by name or through THIS.
func TestOwnMethodCalls(t *testing.T) {
	checkContains(t, `FUNCTION_BLOCK Fb
  VAR_OUTPUT o : INT; END_VAR
  METHOD Inc : INT VAR_INPUT k : INT; END_VAR Inc := k + 1; END_METHOD
  METHOD Twice : INT VAR_INPUT k : INT; END_VAR Twice := Inc(k) * 2; END_METHOD
  o := Inc(k := 2) + Inc(3) + THIS.Inc(k := 4) + Twice(1);
END_FUNCTION_BLOCK
FUNCTION Inc2 : INT VAR_INPUT k : INT; END_VAR Inc2 := k; END_FUNCTION`,
		"f.o = (((f.Inc(2) + f.Inc(3)) + f.Inc(4)) + f.Twice(1))",
		"Twice = (f.Inc(k) * 2)")
}

// A bit is read as a BOOL and written by setting or clearing it.
func TestBitAccess(t *testing.T) {
	checkContains(t, `FUNCTION_BLOCK Fb VAR_OUTPUT f : INT; END_VAR f.1 := TRUE; END_FUNCTION_BLOCK
PROGRAM P VAR x : BYTE := 16#0A; a : ARRAY[0..1] OF WORD; b : BOOL; fb : Fb; END_VAR
  b := x.3 AND NOT x.0; a[1].15 := b; b := fb.f.1;
END_PROGRAM`,
		"f.f = iec.INT(uint64(f.f) | 1<<1)",
		"p.b = (iec.BOOL(uint64(p.x)>>3&1 == 1) && (!iec.BOOL(uint64(p.x)>>0&1 == 1)))",
		"p.a[1] = iec.WORD(uint64(p.a[1]) | 1<<15)", "p.a[1] = iec.WORD(uint64(p.a[1]) &^ (1 << 15))",
		"p.b = iec.BOOL(uint64(p.fb.f)>>1&1 == 1)")
	for _, tt := range []struct{ src, want string }{
		{"PROGRAM P VAR r : REAL; b : BOOL; END_VAR b := r.1; END_PROGRAM", "needs an integer or bit string"},
		{"PROGRAM P VAR x : BYTE; END_VAR x.8 := TRUE; END_PROGRAM", "bit 8 is outside the 8 bits of x"},
	} {
		if _, err := transpileSource(t, tt.src); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: expected an error containing %q, got %v", tt.src, tt.want, err)
		}
	}
}
