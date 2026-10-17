/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// conformance holds a program per item of doc/conformance.md, by its ID:
// IEC 61131-3 behaviour each engine must have. A program counts its failed
// checks in failures and sets done.
var conformance = map[string]string{
	// LOWER_BOUND and UPPER_BOUND give the declared bounds of a dimension.
	"IEC-2": `
PROGRAM TEST_IEC2
VAR_OUTPUT failures : INT; done : BOOL; lo, hi, lo2, hi2, sum : DINT; END_VAR
VAR a : ARRAY[2..5] OF INT := [1, 2, 3, 4]; m : ARRAY[1..3, -1..2] OF INT; i : DINT; END_VAR
lo := LOWER_BOUND(a, 1); hi := UPPER_BOUND(a, 1);
lo2 := LOWER_BOUND(m, 2); hi2 := UPPER_BOUND(m, 2);
FOR i := LOWER_BOUND(a, 1) TO UPPER_BOUND(a, 1) DO sum := sum + a[i]; END_FOR;
IF lo <> 2 OR hi <> 5 OR lo2 <> -1 OR hi2 <> 2 OR UPPER_BOUND(m, 1) <> 3 OR sum <> 10 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// $ escapes in string literals are decoded.
	"IEC-3": `
PROGRAM TEST_IEC3
VAR_OUTPUT failures : INT; done : BOOL; n1, n2, n3, n4 : INT; END_VAR
VAR s : STRING; w : WSTRING; END_VAR
s := 'a$'b$$c$N$41';
n1 := LEN(s);
IF n1 <> 7 OR FIND(s, '$'') <> 2 OR RIGHT(s, 1) <> 'A' OR MID(s, 1, 4) <> '$$' THEN failures := failures + 1; END_IF;
n2 := LEN('$R$L$P$T$r$l$p$t');
IF n2 <> 8 THEN failures := failures + 1; END_IF;
w := "say $"hi$" $0041";
n3 := LEN(w);
IF w <> "say $"hi$" A" THEN failures := failures + 1; END_IF;
n4 := LEN('50$%');
done := TRUE;
END_PROGRAM
`,
	// A STRING(n) keeps the first n characters of what it is given.
	"IEC-4": `
FUNCTION_BLOCK FB_IEC4
VAR_INPUT code : STRING(3); END_VAR
VAR_OUTPUT got : STRING; END_VAR
got := code;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC4
VAR_OUTPUT failures : INT; done : BOOL; a, b : STRING(5); w : WSTRING(2); n : INT; END_VAR
VAR f : FB_IEC4; init : STRING(4) := 'abcdefgh'; END_VAR
a := 'abcdefgh';
b := 'xy';
w := "wide";
f(code := 'ABCDEF');
n := LEN(a);
IF a <> 'abcde' OR b <> 'xy' OR init <> 'abcd' OR w <> "wi" OR f.got <> 'ABC' OR n <> 5 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The length and positions of a WSTRING count characters, not bytes.
	"IEC-5": `
PROGRAM TEST_IEC5
VAR_OUTPUT failures : INT; done : BOOL; n, f : INT; END_VAR
VAR w : WSTRING := "héllo wörld"; END_VAR
n := LEN(w);
f := FIND(w, "wö");
IF n <> 11 OR f <> 7 THEN failures := failures + 1; END_IF;
IF LEFT(w, 2) <> "hé" OR RIGHT(w, 4) <> "örld" OR MID(w, 3, 2) <> "éll" THEN failures := failures + 1; END_IF;
IF INSERT(w, "!", 2) <> "hé!llo wörld" OR DELETE(w, 6, 2) <> "hörld" OR REPLACE(w, "e", 1, 2) <> "hello wörld" THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A narrow integer wraps into its range, two's complement, on overflow.
	"IEC-6": `
TYPE PAIR_IEC6 : STRUCT a : SINT; END_STRUCT; END_TYPE
FUNCTION_BLOCK FB_IEC6
VAR_INPUT x : SINT; END_VAR
VAR_OUTPUT y : SINT; END_VAR
y := x;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC6
VAR_OUTPUT failures : INT; done : BOOL;
  s : SINT := 127; i : INT := 32767; u : USINT := 255; ui : UINT; d : DINT := 2147483647; m : INT := -32768; k : INT; l : LINT;
END_VAR
VAR f : FB_IEC6; arr : ARRAY[0..1] OF SINT; v : INT := 200; p : PAIR_IEC6; END_VAR
s := s + 1; i := i + 1; u := u + 1; d := d + 1; m := m - 1; k := 300 * 300; ui := -1;
l := 2147483647; l := l + 1;
f(x := v);
arr[1] := v;
p.a := v;
IF s <> -128 OR i <> -32768 OR u <> 0 OR ui <> 65535 OR d <> -2147483648 OR m <> 32767 OR k <> 24464 OR l <> 2147483648 THEN failures := failures + 1; END_IF;
IF f.y <> -56 OR arr[1] <> -56 OR p.a <> -56 THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The conversions to STRING give IEC's literal forms, alike on every engine.
	"IEC-7": `
PROGRAM TEST_IEC7
VAR_OUTPUT failures : INT; done : BOOL;
  r1, r2, r3, r4, r5, l1, t1, t2, t3, t4, d1, tod1, dt1, b1 : STRING;
END_VAR
VAR r : REAL := 0.1; END_VAR
r1 := REAL_TO_STRING(1.5); r2 := REAL_TO_STRING(100.0); r3 := REAL_TO_STRING(-0.000125); r4 := REAL_TO_STRING(1.0E20);
r5 := REAL_TO_STRING(r);
l1 := LREAL_TO_STRING(3.14159265358979);
t1 := TIME_TO_STRING(T#1s); t2 := TIME_TO_STRING(T#1h2m3s4ms); t3 := TIME_TO_STRING(T#-250ms); t4 := TIME_TO_STRING(T#0s);
d1 := DATE_TO_STRING(D#2026-01-02); tod1 := TOD_TO_STRING(TOD#03:04:05.5); dt1 := DT_TO_STRING(DT#2026-01-02-03:04:05);
b1 := BOOL_TO_STRING(TRUE);
IF r1 <> '1.5' OR r2 <> '100.0' OR r3 <> '-0.000125' OR r4 <> '1.0E+20' OR r5 <> '0.1' OR l1 <> '3.14159265358979' THEN failures := failures + 1; END_IF;
IF t1 <> 'T#1s' OR t2 <> 'T#1h2m3s4ms' OR t3 <> 'T#-250ms' OR t4 <> 'T#0s' THEN failures := failures + 1; END_IF;
IF d1 <> 'D#2026-01-02' OR tod1 <> 'TOD#03:04:05.5' OR dt1 <> 'DT#2026-01-02-03:04:05' OR b1 <> 'TRUE' THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Interface variables refer to the instance assigned to them.
	"IEC-8": `
INTERFACE I_AREA8
METHOD Area : INT END_METHOD
END_INTERFACE
INTERFACE I_NAMED8 EXTENDS I_AREA8
METHOD Sides : INT END_METHOD
END_INTERFACE
FUNCTION_BLOCK SQ8 IMPLEMENTS I_NAMED8
VAR_INPUT k : INT; END_VAR
METHOD Area : INT
Area := k * k;
END_METHOD
METHOD Sides : INT
Sides := 4;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK TRI8 IMPLEMENTS I_AREA8
VAR_INPUT k : INT; END_VAR
METHOD Area : INT
Area := 3 * k;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK USER8
VAR_INPUT shape : I_AREA8; END_VAR
VAR_OUTPUT got : INT; END_VAR
got := shape.Area();
END_FUNCTION_BLOCK
PROGRAM TEST_IEC8
VAR_OUTPUT failures : INT; done : BOOL; a1, a2, a3, a4 : INT; END_VAR
VAR s : SQ8; t : TRI8; one : I_AREA8; named : I_NAMED8; many : ARRAY[0..1] OF I_AREA8; u : USER8; END_VAR
s.k := 2; t.k := 3;
one := s; a1 := one.Area();
s.k := 5; a2 := one.Area();
named := s;
many[0] := s; many[1] := t;
a3 := many[0].Area() + many[1].Area();
u(shape := t); a4 := u.got;
IF a1 <> 4 OR a2 <> 25 OR named.Sides() <> 4 OR named.Area() <> 25 OR a3 <> 34 OR a4 <> 9 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A function block instance passed VAR_IN_OUT to a function runs there
	// with the scan's time, as does a timer a method calls.
	"IEC-9": `
FUNCTION_BLOCK CNT9
VAR_OUTPUT n : INT; END_VAR
n := n + 1;
END_FUNCTION_BLOCK
FUNCTION PUSH9 : BOOL
VAR_IN_OUT c : CNT9; t : TON; END_VAR
c();
t(IN := TRUE, PT := T#30ms);
PUSH9 := t.Q;
END_FUNCTION
FUNCTION_BLOCK HOLDER9
VAR_OUTPUT q : BOOL; END_VAR
VAR tm : TON; END_VAR
METHOD Tick : BOOL
tm(IN := TRUE, PT := T#20ms);
Tick := tm.Q;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM TEST_IEC9
VAR_OUTPUT failures : INT; done : BOOL; scan : INT; fq, mq : BOOL; END_VAR
VAR c : CNT9; t : TON; h : HOLDER9; END_VAR
fq := PUSH9(c, t);
mq := h.Tick();
scan := scan + 1;
IF c.n <> scan THEN failures := failures + 1; END_IF;
IF scan = 1 AND (fq OR mq) THEN failures := failures + 1; END_IF;
IF scan = 6 THEN
  IF NOT fq OR NOT mq THEN failures := failures + 1; END_IF;
  done := TRUE;
END_IF;
END_PROGRAM
`,
	// Time arithmetic: a point in time plus or minus a TIME, and the TIME
	// between two points.
	"IEC-10": `
PROGRAM TEST_IEC10
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR d : DT; tod : TOD; t, t2, t3 : TIME; dd : DATE; END_VAR
d := DT#2026-01-01-00:00:00 + T#1h;
tod := TOD#10:00:00 - T#30m;
t := DT#2026-01-01-12:00:00 - DT#2026-01-01-10:30:00;
t2 := TOD#12:00:00 - TOD#11:59:30;
dd := D#2026-01-03;
t3 := dd - D#2026-01-01;
IF d <> DT#2026-01-01-01:00:00 OR tod <> TOD#09:30:00 OR t <> T#1h30m OR t2 <> T#30s OR t3 <> T#48h THEN failures := 1; END_IF;
IF d + T#1d <> DT#2026-01-02-01:00:00 OR T#1s + T#500ms <> T#1500ms THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Array bounds and string lengths given by named constants.
	"IEC-11": `
VAR_GLOBAL CONSTANT NMAX11 : INT := 4; NLEN11 : INT := 3; END_VAR
FUNCTION_BLOCK FB11
VAR CONSTANT K : INT := 2; END_VAR
VAR_OUTPUT n : INT; END_VAR
VAR b : ARRAY[1..K * 3] OF INT; END_VAR
b[K * 3] := 7;
n := b[6] + UPPER_BOUND(b, 1);
END_FUNCTION_BLOCK
PROGRAM TEST_IEC11
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR a : ARRAY[0..NMAX11] OF INT; m : ARRAY[1..NMAX11 + 1, 0..1] OF BOOL; s : STRING(NLEN11); f : FB11; END_VAR
a[NMAX11] := 1;
m[NMAX11 + 1, 1] := TRUE;
s := 'abcdef';
f();
IF a[NMAX11] <> 1 OR NOT m[5, 1] OR s <> 'abc' OR f.n <> 13 OR UPPER_BOUND(a, 1) <> 4 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// SFC step flags: step.X (active) and step.T (time active).
	"IEC-12": `
PROGRAM TEST_IEC12
VAR_OUTPUT failures : INT; done : BOOL; n : INT; wasX : BOOL; tAt : TIME; END_VAR
ACTION Count: n := n + 1; wasX := S0.X; END_ACTION
ACTION Check:
  IF NOT S1.X OR S0.X THEN failures := failures + 1; END_IF;
  tAt := S1.T;
END_ACTION
ACTION Finish: done := TRUE; END_ACTION
INITIAL_STEP S0: Count(N); END_STEP
TRANSITION FROM S0 TO S1 := S0.T >= T#30ms; END_TRANSITION
STEP S1: Check(N); END_STEP
TRANSITION FROM S1 TO S2 := S1.T >= T#20ms; END_TRANSITION
STEP S2: Finish(N); END_STEP
END_PROGRAM
`,
	// R, S and T name structure members.
	"IEC-27": `
TYPE REC27 : STRUCT R : INT; S : BOOL; T : TIME; END_STRUCT; END_TYPE
PROGRAM TEST_IEC27
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR v : REC27; x : INT; END_VAR
v.R := 3; v.S := TRUE; v.T := T#2s;
x := v.R;
IF x <> 3 OR NOT v.S OR v.T <> T#2s THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// SFC action flags: action.Q (active) and action.A (executing, as Q).
	"IEC-13": `
PROGRAM TEST_IEC13
VAR_OUTPUT failures : INT; done : BOOL; n : INT; seenQ, seenA : BOOL; END_VAR
ACTION Count: n := n + 1; seenQ := Count.Q; seenA := Count.A; END_ACTION
ACTION Finish:
  IF Count.Q OR Count.A OR NOT seenQ OR NOT seenA OR NOT Finish.Q THEN failures := failures + 1; END_IF;
  done := TRUE;
END_ACTION
INITIAL_STEP S0: Count(N); END_STEP
TRANSITION FROM S0 TO S1 := n >= 2 AND Count.Q; END_TRANSITION
STEP S1: Finish(N); END_STEP
END_PROGRAM
`,
	// A value of an alias type is a value of the type it names.
	"IEC-14": `
TYPE VOLT14 : REAL := 230.0; END_TYPE
TYPE MAINS14 : VOLT14; END_TYPE
TYPE COUNT14 : INT; END_TYPE
PROGRAM TEST_IEC14
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR v : VOLT14; m : MAINS14; c : COUNT14; r : REAL; END_VAR
c := c + 2;
r := v * 2.0;
IF v <> 230.0 OR m <> 230.0 OR r <> 460.0 OR c <> 2 OR c > 5 OR NOT (v > 100.0) THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// BOOL#1, BOOL#0, BOOL#TRUE and BOOL#FALSE.
	"IEC-15": `
PROGRAM TEST_IEC15
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR a, b, c, d : BOOL; END_VAR
a := BOOL#1; b := BOOL#0; c := BOOL#TRUE; d := BOOL#FALSE;
IF NOT a OR b OR NOT c OR d OR NOT (BOOL#1 AND TRUE) OR (BOOL#0 OR FALSE) THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A function called by its namespace, LIB.Twice(3).
	"IEC-16": `
NAMESPACE LIB16
FUNCTION Twice : INT
VAR_INPUT a : INT; END_VAR
Twice := a * 2;
END_FUNCTION
NAMESPACE Inner
FUNCTION Thrice : INT
VAR_INPUT a : INT; END_VAR
Thrice := a * 3;
END_FUNCTION
END_NAMESPACE
END_NAMESPACE
PROGRAM TEST_IEC16
VAR_OUTPUT failures : INT; done : BOOL; x, y : INT; END_VAR
x := LIB16.Twice(3);
y := LIB16.Inner.Thrice(a := 2);
IF x <> 6 OR y <> 6 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A method called by its name, or through THIS^, from its function
	// block's body or another of its methods; an inherited one too.
	"IEC-17": `
FUNCTION_BLOCK BASE17
METHOD Base : INT
Base := 10;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB17 EXTENDS BASE17
VAR_OUTPUT o, p : INT; END_VAR
METHOD Three : INT
Three := 3;
END_METHOD
METHOD Six : INT
Six := Three() + THIS^.Three();
END_METHOD
o := Three() + THIS^.Three();
p := Six() + Base();
END_FUNCTION_BLOCK
PROGRAM TEST_IEC17
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR f : FB17; END_VAR
f();
IF f.o <> 6 OR f.p <> 16 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Extensible comparisons: GT(a, b, c) is a > b AND b > c.
	"IEC-18": `
PROGRAM TEST_IEC18
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR x : INT := 7; END_VAR
IF NOT GT(9, x, 3, 1) OR GT(9, x, 8) OR NOT GE(5, 5, 2) OR NOT EQ(2, 2, 2) OR EQ(2, 2, 3) THEN failures := failures + 1; END_IF;
IF NOT LT(1, 2, x) OR NOT LE(1, 1, 1) OR LT(1, 3, 2) OR NOT GT(2, 1) OR NOT NE(1, 2) THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// CONCAT_DATE_TOD(DATE, TOD) : DT.
	"IEC-19": `
PROGRAM TEST_IEC19
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR d : DT; END_VAR
d := CONCAT_DATE_TOD(D#2026-01-02, TOD#03:04:05.5);
IF d <> DT#2026-01-02-03:04:05.5 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The power operator **.
	"IEC-20": `
PROGRAM TEST_IEC20
VAR_OUTPUT failures : INT; done : BOOL; i : DINT; r : REAL; l : LREAL; END_VAR
VAR n : DINT := 3; x : REAL := 1.5; END_VAR
i := 2 ** n;
r := x ** 2;
l := 2.0 ** 0.5;
IF i <> 8 OR r <> 2.25 OR l < 1.414 OR l > 1.415 OR 10 ** 2 <> 100 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// AND, OR and XOR of integers are bitwise.
	"IEC-21": `
PROGRAM TEST_IEC21
VAR_OUTPUT failures : INT; done : BOOL; a, o, x : INT; END_VAR
VAR i : INT := 16#0F; b : BYTE := 16#F0; END_VAR
a := 16#F0 AND 16#3C;
o := i OR 16#30;
x := i XOR 16#FF;
IF a <> 16#30 OR o <> 16#3F OR x <> 16#F0 OR (b AND 16#30) <> 16#30 OR NOT (TRUE AND NOT FALSE) THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Binary-coded decimal conversions.
	"IEC-22": `
PROGRAM TEST_IEC22
VAR_OUTPUT failures : INT; done : BOOL; w : WORD; d : DWORD; b : BYTE; n : INT; m : DINT; END_VAR
VAR i : INT := 42; big : DINT := 12345678; END_VAR
w := INT_TO_BCD(i);
d := DINT_TO_BCD(big);
b := USINT_TO_BCD(USINT#99);
n := BCD_TO_INT(16#0099);
m := BCD_TO_DINT(d);
IF w <> 16#42 OR d <> 16#12345678 OR b <> 16#99 OR n <> 99 OR m <> 12345678 OR BCD_TO_INT(w) <> 42 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// CHAR and WCHAR, and their conversions.
	"IEC-23": `
PROGRAM TEST_IEC23
VAR_OUTPUT failures : INT; done : BOOL; c : CHAR; w : WCHAR; code : BYTE; s : STRING; u : WORD; END_VAR
VAR empty : CHAR; END_VAR
c := 'A';
w := "é";
code := CHAR_TO_BYTE('B');
s := CONCAT(CHAR_TO_STRING(c), CHAR_TO_STRING(BYTE_TO_CHAR(66)));
u := WCHAR_TO_WORD(w);
IF c <> 'A' OR code <> 66 OR s <> 'AB' OR u <> 233 OR STRING_TO_CHAR('xyz') <> 'x' OR CHAR_TO_BYTE(empty) <> 0 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// CONTINUE goes on with the loop's next iteration; EXIT and CONTINUE
	// end the rest of the body, from inside an IF or a CASE too.
	"IEC-24": `
PROGRAM TEST_IEC24
VAR_OUTPUT failures : INT; done : BOOL; sf, sw, sr, se, sc, inner : INT; END_VAR
VAR i, j : INT; continue : INT; END_VAR
FOR i := 1 TO 5 DO
  IF i = 3 THEN CONTINUE; END_IF;
  sf := sf + i;
END_FOR;
i := 0;
WHILE i < 5 DO
  i := i + 1;
  IF i MOD 2 = 0 THEN CONTINUE; END_IF;
  sw := sw + i;
END_WHILE;
i := 0;
REPEAT
  i := i + 1;
  IF i = 2 THEN CONTINUE; END_IF;
  sr := sr + i;
UNTIL i >= 4 END_REPEAT;
FOR i := 1 TO 5 DO
  IF i = 3 THEN EXIT; END_IF;
  se := se + i;
END_FOR;
FOR i := 1 TO 4 DO
  CASE i OF
    2: CONTINUE;
    4: EXIT;
  END_CASE;
  sc := sc + i;
  FOR j := 1 TO 3 DO
    IF j = 2 THEN CONTINUE; END_IF;
    inner := inner + 1;
  END_FOR;
END_FOR;
continue := 7;
IF sf <> 12 OR sw <> 9 OR sr <> 8 OR se <> 3 OR sc <> 4 OR inner <> 4 OR continue <> 7 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// R_EDGE and F_EDGE after the type, as IEC 61131-3 writes them.
	"IEC-25": `
FUNCTION_BLOCK EDGES25
VAR_INPUT up : BOOL R_EDGE; down : BOOL F_EDGE; END_VAR
VAR_OUTPUT ups, downs : INT; END_VAR
IF up THEN ups := ups + 1; END_IF;
IF down THEN downs := downs + 1; END_IF;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC25
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR e : EDGES25; END_VAR
e(up := TRUE, down := TRUE);
e(up := TRUE, down := FALSE);
e(up := FALSE, down := FALSE);
e(up := TRUE, down := TRUE);
IF e.ups <> 2 OR e.downs <> 1 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// STRING[n] and WSTRING[n].
	"IEC-26": `
PROGRAM TEST_IEC26
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR s : STRING[5]; w : WSTRING[2] := "wide"; END_VAR
s := 'abcdefgh';
IF s <> 'abcde' OR LEN(s) <> 5 OR w <> "wi" THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Pragmas { ... } are ignored, wherever they stand.
	"IEC-28": `
{attribute 'qualified_only'}
PROGRAM TEST_IEC28
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR
  {attribute 'hide'}
  x : INT;
  s : STRING;
END_VAR
{info 'a pragma before a statement'} x := 1;
s := '{not a pragma}';
{IF defined (SIMULATION)} x := x + 1; {END_IF}
IF x <> 2 OR s <> '{not a pragma}' THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The OVERRIDE method modifier; a method named Get.
	"IEC-29": `
FUNCTION_BLOCK BASE29
METHOD PUBLIC Get : INT
Get := 1;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK DER29 EXTENDS BASE29
METHOD PUBLIC OVERRIDE Get : INT
Get := SUPER^.Get() + 1;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM TEST_IEC29
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR b : BASE29; d : DER29; END_VAR
IF b.Get() <> 1 OR d.Get() <> 2 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A FUNCTION without a result type, called as a statement.
	"IEC-30": `
FUNCTION BUMP30
VAR_IN_OUT v : INT; END_VAR
VAR_INPUT step : INT := 1; END_VAR
VAR_OUTPUT was : INT; END_VAR
was := v;
v := v + step;
END_FUNCTION
PROGRAM TEST_IEC30
VAR_OUTPUT failures : INT; done : BOOL; x, before : INT; END_VAR
BUMP30(v := x);
BUMP30(v := x, step := 5, was => before);
IF x <> 6 OR before <> 1 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Enumerations with numbers, read as their numbers by TO_<integer>.
	"IEC-31": `
TYPE COLOR31 : (RED := 1, GREEN := 5, BLUE); END_TYPE
TYPE PLAIN31 : (A, B, C); END_TYPE
PROGRAM TEST_IEC31
VAR_OUTPUT failures : INT; done : BOOL; g, b, p : INT; END_VAR
VAR c : COLOR31 := COLOR31#GREEN; x : PLAIN31 := PLAIN31#C; END_VAR
g := TO_INT(c);
b := TO_INT(COLOR31#BLUE);
p := TO_INT(x);
IF g <> 5 OR b <> 6 OR p <> 2 OR c <> COLOR31#GREEN OR TO_DINT(COLOR31#RED) <> 1 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The assignment attempt ?= (3rd edition): the instance if it is of the
	// target's type, NULL otherwise.
	"IEC-35": `
INTERFACE I_RUN35
METHOD Run : INT END_METHOD
END_INTERFACE
INTERFACE I_STOP35
METHOD Stop : INT END_METHOD
END_INTERFACE
FUNCTION_BLOCK MOTOR35 IMPLEMENTS I_RUN35, I_STOP35
VAR_OUTPUT speed : INT; END_VAR
METHOD Run : INT
speed := 100; Run := speed;
END_METHOD
METHOD Stop : INT
speed := 0; Stop := speed;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FAST35 EXTENDS MOTOR35
END_FUNCTION_BLOCK
FUNCTION_BLOCK LAMP35 IMPLEMENTS I_RUN35
METHOD Run : INT
Run := 1;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM TEST_IEC35
VAR_OUTPUT failures : INT; done : BOOL; s : INT; END_VAR
VAR m : MOTOR35; f : FAST35; l : LAMP35; runner : I_RUN35; stopper : I_STOP35; rm : REF_TO MOTOR35; END_VAR
runner := m;
stopper ?= runner;
IF stopper = NULL THEN failures := failures + 1; END_IF;
rm ?= runner;
IF rm = NULL THEN failures := failures + 1; END_IF;
m.Run();
s := rm^.speed;
IF s <> 100 THEN failures := failures + 1; END_IF;
runner := l;
stopper ?= runner;
IF stopper <> NULL THEN failures := failures + 1; END_IF;
rm ?= runner;
IF rm <> NULL THEN failures := failures + 1; END_IF;
runner := f;
rm ?= runner;
IF rm = NULL THEN failures := failures + 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// ARRAY[*] VAR_IN_OUT: arrays of any length and bounds.
	"IEC-36": `
FUNCTION SUM36 : DINT
VAR_IN_OUT v : ARRAY[*] OF INT; END_VAR
VAR i : DINT; END_VAR
FOR i := LOWER_BOUND(v, 1) TO UPPER_BOUND(v, 1) DO SUM36 := SUM36 + v[i]; END_FOR;
END_FUNCTION
FUNCTION DOUBLE36 : DINT
VAR_IN_OUT v : ARRAY[*] OF INT; END_VAR
VAR i : DINT; END_VAR
FOR i := LOWER_BOUND(v, 1) TO UPPER_BOUND(v, 1) DO v[i] := v[i] * 2; END_FOR;
DOUBLE36 := SUM36(v);
END_FUNCTION
FUNCTION_BLOCK FILL36
VAR_INPUT x : INT; END_VAR
VAR_IN_OUT buf : ARRAY[*] OF INT; END_VAR
VAR_OUTPUT n : DINT; END_VAR
buf[LOWER_BOUND(buf, 1)] := x;
n := UPPER_BOUND(buf, 1) - LOWER_BOUND(buf, 1) + 1;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC36
VAR_OUTPUT failures : INT; done : BOOL; s1, s2, d : DINT; END_VAR
VAR a : ARRAY[1..4] OF INT := [1, 2, 3, 4]; b : ARRAY[-2..0] OF INT := [10, 20, 30]; f : FILL36; END_VAR
s1 := SUM36(a);
s2 := SUM36(b);
d := DOUBLE36(b);
f(x := 7, buf := a);
IF s1 <> 10 OR s2 <> 60 OR d <> 120 OR b[-2] <> 20 OR a[1] <> 7 OR f.n <> 4 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Arrays of arrays, as the standard writes them: an array of a named
	// array type.
	"IEC-37": `
TYPE ROW37 : ARRAY[0..2] OF INT; END_TYPE
PROGRAM TEST_IEC37
VAR_OUTPUT failures : INT; done : BOOL; x : INT; END_VAR
VAR m : ARRAY[1..2] OF ROW37; r : ROW37; END_VAR
m[2][1] := 5;
r := m[2];
r[1] := 6;
x := r[1] + m[2][1];
IF x <> 11 OR UPPER_BOUND(m, 1) <> 2 OR UPPER_BOUND(r, 1) <> 2 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The long time types and their literals (3rd edition).
	"IEC-38": `
PROGRAM TEST_IEC38
VAR_OUTPUT failures : INT; done : BOOL; s : STRING; END_VAR
VAR
  lt : LTIME := LTIME#1s;
  lt2 : LTIME;
  ld : LDATE := LDATE#2026-01-02;
  ltod : LTOD := LTOD#12:00:00;
  ldt : LDT := LDT#2026-01-02-12:00:00;
  t : TIME;
END_VAR
lt2 := LT#1500ms + lt;
t := LTIME_TO_TIME(lt);
s := LTIME_TO_STRING(lt2);
IF lt <> LT#1000ms OR lt2 <> LTIME#2s500ms OR ld <> LD#2026-01-02 OR ltod <> LTOD#12:00:00 OR ldt <> LDT#2026-01-02-12:00:00 OR t <> T#1s OR s <> 'T#2s500ms' THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The generic conversions TO_<type> and TRUNC_<integer>.
	"IEC-39": `
PROGRAM TEST_IEC39
VAR_OUTPUT failures : INT; done : BOOL; i, t, n : INT; r : REAL; s : STRING; d : DINT; END_VAR
VAR x : REAL := 3.7; y : LREAL := -2.9; w : WORD := 16#00FF; k : INT := 42; END_VAR
i := TO_INT(x);
t := TRUNC_INT(x);
n := TRUNC_INT(y);
r := TO_REAL(k);
s := TO_STRING(k);
d := TO_DINT(w);
IF i <> 4 OR t <> 3 OR n <> -2 OR r <> 42.0 OR s <> '42' OR d <> 255 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// The functions of time data types: ADD_TIME, SUB_DT_DT, CONCAT_DT, ...
	"IEC-40": `
PROGRAM TEST_IEC40
VAR_OUTPUT failures : INT; done : BOOL; t : TIME; lt : LTIME; tod1 : TOD; dt1 : DT; d1 : DATE; END_VAR
VAR k : INT := 3; dt0 : DT := DT#2024-03-01-10:00:00; END_VAR
t := ADD_TIME(T#1s, T#500ms);
IF t <> T#1500ms THEN failures := failures + 1; END_IF;
IF SUB_TIME(T#2s, T#500ms) <> T#1500ms THEN failures := failures + 2; END_IF;
IF MUL_TIME(T#1s, k) <> T#3s OR MULTIME(T#2s, 2.5) <> T#5s THEN failures := failures + 4; END_IF;
IF DIV_TIME(T#3s, 2) <> T#1500ms OR DIVTIME(T#3s, k) <> T#1s THEN failures := failures + 8; END_IF;
tod1 := ADD_TOD_TIME(TOD#10:00:00, T#90m);
IF tod1 <> TOD#11:30:00 OR SUB_TOD_TOD(TOD#11:30:00, TOD#10:00:00) <> T#90m THEN failures := failures + 16; END_IF;
dt1 := ADD_DT_TIME(dt0, T#2h);
IF dt1 <> DT#2024-03-01-12:00:00 OR SUB_DT_DT(dt1, dt0) <> T#2h OR SUB_DT_TIME(dt1, T#1h) <> DT#2024-03-01-11:00:00 THEN failures := failures + 32; END_IF;
d1 := CONCAT_DATE(2024, 3, 2);
IF d1 <> D#2024-03-02 OR SUB_DATE_DATE(d1, D#2024-03-01) <> T#1d THEN failures := failures + 64; END_IF;
IF CONCAT_DT(2024, 3, 1, 10, 0, 0, 0) <> dt0 OR CONCAT_TOD(11, 30, 0, 0) <> tod1 THEN failures := failures + 128; END_IF;
lt := ADD_LTIME(LT#1s, LT#1s);
IF lt <> LT#2s OR CONCAT_DATE_LTOD(D#2024-03-01, LTOD#10:00:00) <> dt0 THEN failures := failures + 256; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Byte order: a value is little-endian.
	"IEC-41": `
PROGRAM TEST_IEC41
VAR_OUTPUT failures : INT; done : BOOL; i : INT; w : WORD; d : DINT; u : UDINT; l : LWORD; b : BYTE; r : REAL; END_VAR
VAR i0 : INT := 16#0102; n : INT := -2; w0 : WORD := 16#ABCD; d0 : DINT := 16#01020304; b0 : BYTE := 16#7F; r0 : REAL := 1.5; END_VAR
i := TO_BIG_ENDIAN(i0);
IF i <> 16#0201 OR FROM_BIG_ENDIAN(i) <> i0 OR TO_LITTLE_ENDIAN(i0) <> i0 OR FROM_LITTLE_ENDIAN(i0) <> i0 THEN failures := failures + 1; END_IF;
i := TO_BIG_ENDIAN(n);
IF i <> -257 THEN failures := failures + 2; END_IF;
w := TO_BIG_ENDIAN(w0);
IF w <> 16#CDAB THEN failures := failures + 4; END_IF;
d := TO_BIG_ENDIAN(d0);
u := TO_BIG_ENDIAN(UDINT#16#11223344);
IF d <> 16#04030201 OR u <> UDINT#16#44332211 THEN failures := failures + 8; END_IF;
l := TO_BIG_ENDIAN(LWORD#16#0102030405060708);
IF l <> LWORD#16#0807060504030201 THEN failures := failures + 16; END_IF;
b := TO_BIG_ENDIAN(b0);
r := FROM_BIG_ENDIAN(TO_BIG_ENDIAN(r0));
IF b <> 16#7F OR r <> 1.5 THEN failures := failures + 32; END_IF;
done := TRUE;
END_PROGRAM
`,
	// IS_VALID of a real and IS_VALID_BCD of a bit string.
	"IEC-42": `
PROGRAM TEST_IEC42
VAR_OUTPUT failures : INT; done : BOOL; ok1, ok2 : BOOL; END_VAR
VAR r : REAL := 1.5; big : LREAL := 1.0E308; w : WORD := 16#1234; bad : WORD := 16#12A4; b : BYTE := 16#99; END_VAR
ok1 := IS_VALID(r);
big := big * 10.0;
IF NOT ok1 OR IS_VALID(big) OR NOT IS_VALID(LREAL#2.0) THEN failures := failures + 1; END_IF;
ok2 := IS_VALID_BCD(w);
IF NOT ok2 OR IS_VALID_BCD(bad) OR NOT IS_VALID_BCD(b) OR IS_VALID_BCD(BYTE#16#F0) THEN failures := failures + 2; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Typed counters and timers: CTU_DINT, CTD_UDINT, CTUD_LINT, TON_LTIME.
	"IEC-43": `
PROGRAM TEST_IEC43
VAR_OUTPUT failures : INT; done : BOOL; cv : DINT; up : BOOL; END_VAR
VAR c1 : CTU_DINT; c2 : CTD_UDINT; c3 : CTUD_LINT; c4 : CTU_INT; t1 : TON_LTIME; tick : BOOL; n : INT; END_VAR
n := n + 1;
tick := NOT tick;
c1(CU := tick, R := FALSE, PV := 100000);
c2(CD := tick, LD := n = 1, PV := UDINT#3);
c3(CU := tick, CD := FALSE, R := FALSE, LD := FALSE, PV := LINT#2);
c4(CU := tick, R := FALSE, PV := 2);
t1(IN := TRUE, PT := LT#0s);
cv := c1.CV;
up := c3.QU;
IF n = 6 THEN
  IF c1.CV <> 3 OR c1.Q THEN failures := failures + 1; END_IF;
  IF c2.CV <> UDINT#1 OR c2.Q THEN failures := failures + 2; END_IF;
  IF c3.CV <> LINT#2 OR NOT c3.QU THEN failures := failures + 4; END_IF;
  IF c4.CV <> 2 OR NOT c4.Q THEN failures := failures + 8; END_IF;
  IF NOT t1.Q THEN failures := failures + 16; END_IF;
  done := TRUE;
END_IF;
END_PROGRAM
`,
	// CLASS ... END_CLASS: methods and variables, no body; EXTENDS and
	// IMPLEMENTS as for a function block.
	"IEC-44": `
INTERFACE ICOUNT44
METHOD Count : DINT
END_METHOD
END_INTERFACE
CLASS COUNTER44 IMPLEMENTS ICOUNT44
VAR n : DINT; step : DINT := 1; END_VAR
METHOD PUBLIC Inc : DINT
n := n + step;
Inc := n;
END_METHOD
METHOD PUBLIC Count : DINT
Count := n;
END_METHOD
END_CLASS
CLASS DOUBLE44 EXTENDS COUNTER44
METHOD PUBLIC Init
step := 2;
END_METHOD
END_CLASS
PROGRAM TEST_IEC44
VAR_OUTPUT failures : INT; done : BOOL; a, b : DINT; END_VAR
VAR c : COUNTER44; d : DOUBLE44; i : ICOUNT44; END_VAR
c.Inc();
a := c.Inc();
d.Init();
d.Inc();
i := d;
b := i.Count();
IF a <> 2 OR b <> 2 OR c.Count() <> 2 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// IL: a BOOL member of an instance, and a parenthesized expression.
	"IEC-46": `
PROGRAM TEST_IEC46
VAR_OUTPUT failures : INT; done : BOOL; q : BOOL; r : INT; END_VAR
VAR t : R_TRIG; a : INT := 2; b : INT := 3; c : INT := 4; x : REAL := 1.5; y : REAL; END_VAR
CAL t(CLK := TRUE)
LD t.Q
ST q
LD a
ADD( LD b
MUL c
)
ST r
LD x
MUL 2.0
ST y
LD q
NOT
JMPC bad
LD r
NE 14
JMPC bad
LD y
NE 3.0
JMPC bad
JMP fin
bad: LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// IL: a parenthesized expression in short form, ADD( b.
	"IEC-47": `
PROGRAM TEST_IEC47
VAR_OUTPUT failures : INT; done : BOOL; r, s : INT; END_VAR
VAR a : INT := 2; b : INT := 3; c : INT := 4; END_VAR
LD a
ADD( b
MUL c
)
ST r
LD a
MUL( b )
ST s
LD r
NE 14
JMPC bad
LD s
NE 6
JMPC bad
JMP fin
bad: LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// IL: & and &N for AND and ANDN.
	"IEC-48": `
PROGRAM TEST_IEC48
VAR_OUTPUT failures : INT; done : BOOL; x, y : BOOL; END_VAR
VAR a : BOOL := TRUE; b : BOOL := TRUE; c : BOOL := FALSE; END_VAR
LD a
& b
ST x
LD a
&N c
ST y
LDN c
AND x
STN c
LD x
AND y
ANDN c
JMPC fin
LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// IL: ST?, the assignment attempt.
	"IEC-49": `
INTERFACE I49
METHOD Get : INT
END_METHOD
END_INTERFACE
INTERFACE J49
END_INTERFACE
FUNCTION_BLOCK F49 IMPLEMENTS I49
METHOD PUBLIC Get : INT
Get := 49;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK CHECK49
VAR_INPUT a : I49; b : J49; END_VAR
VAR_OUTPUT ok : BOOL; END_VAR
ok := a <> NULL AND b = NULL;
IF ok THEN ok := a.Get() = 49; END_IF;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC49
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR f : F49; i : I49; j : J49; chk : CHECK49; END_VAR
LD f
ST? i
LD f
ST? j
CAL chk(a := i, b := j)
LD chk.ok
JMPC fin
LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// IL: calls of functions, non-formal and formal, and of methods.
	"IEC-50": `
FUNCTION SCALE50 : INT
VAR_INPUT x : INT; k : INT; END_VAR
SCALE50 := x * k;
END_FUNCTION
FUNCTION_BLOCK FB50
VAR n : INT; END_VAR
METHOD PUBLIC Add : INT
VAR_INPUT k : INT; END_VAR
n := n + k;
Add := n;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM TEST_IEC50
VAR_OUTPUT failures : INT; done : BOOL; a, b, c, d : INT; END_VAR
VAR f : FB50; v : INT := 50; END_VAR
LD 0
LIMIT v, 10
ST a
SCALE50(x := v, k := 2)
ST b
LD 3
SCALE50 4
ST c
f.Add(k := 5)
ST d
LD a
NE 10
JMPC bad
LD b
NE 100
JMPC bad
LD c
NE 12
JMPC bad
LD d
NE 5
JMPC bad
JMP fin
bad: LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// IL: the input operators of the standard function blocks, CU c.
	"IEC-51": `
PROGRAM TEST_IEC51
VAR_OUTPUT failures : INT; done : BOOL; q1, q2, q3, q4 : BOOL; n : INT; END_VAR
VAR c : CTU; f : SR; g : RS; t : TON; e : R_TRIG; END_VAR
LD 3
PV c
LD TRUE
CU c
LD FALSE
CU c
LD TRUE
CU c
LD c.CV
ST n
LD TRUE
S1 f
LD FALSE
S1 f
LD TRUE
R f
LD f.Q1
ST q1
LD TRUE
S g
LD g.Q1
ST q2
LD T#0s
PT t
LD TRUE
IN t
LD t.Q
ST q3
LD TRUE
CLK e
LD e.Q
ST q4
LD n
NE 2
JMPC bad
LD q1
JMPC bad
LD q2
AND q3
AND q4
JMPC fin
bad: LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// Non-formal calls of standard function blocks, in ST and IL.
	"IEC-52": `
FUNCTION_BLOCK BASE52
VAR_INPUT a : INT; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK SUM52 EXTENDS BASE52
VAR_INPUT b : INT; END_VAR
VAR_OUTPUT s : INT; END_VAR
s := a * 10 + b;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC52
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR t : TON; c : CTU; r : R_TRIG; s : SR; u : SUM52; END_VAR
t(TRUE, T#0s);
c(TRUE, FALSE, 1);
r(TRUE);
s(TRUE, FALSE);
u(1, 2);
IF NOT t.Q OR c.CV <> 1 OR NOT c.Q OR NOT r.Q OR NOT s.Q1 OR u.s <> 12 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	"IEC-52-IL": `
PROGRAM TEST_IEC52IL
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR c : CTU; END_VAR
CAL c(TRUE, FALSE, 1)
LD c.Q
JMPC fin
LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// A contextual keyword as a member name in an initial value.
	"IEC-53": `
TYPE S53 : STRUCT step : INT; priority : INT; END_STRUCT; END_TYPE
PROGRAM TEST_IEC53
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR s : S53 := (step := 3, priority := 4); END_VAR
IF s.step <> 3 OR s.priority <> 4 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A RESOURCE's VAR_GLOBAL, seen by a program.
	"IEC-54": `
PROGRAM TEST_IEC54
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR_EXTERNAL rg : INT; g : INT; END_VAR
IF rg <> 7 OR g <> 5 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
CONFIGURATION CF54
VAR_GLOBAL g : INT := 5; END_VAR
RESOURCE R1 ON PLC
VAR_GLOBAL rg : INT := 7; END_VAR
TASK T1(INTERVAL := T#10ms, PRIORITY := 1);
PROGRAM P1 WITH T1 : TEST_IEC54;
END_RESOURCE
END_CONFIGURATION
`,
	// A partly located variable, x AT %I*, and VAR_CONFIG.
	"IEC-55": `
PROGRAM TEST_IEC55
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR x AT %I* : INT; y AT %Q* : INT := 4; END_VAR
x := 3;
IF x <> 3 OR y <> 4 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
CONFIGURATION CF55
RESOURCE R1 ON PLC
TASK T1(INTERVAL := T#10ms, PRIORITY := 1);
PROGRAM P1 WITH T1 : TEST_IEC55;
END_RESOURCE
VAR_CONFIG
  R1.P1.y AT %QW10 : INT;
END_VAR
END_CONFIGURATION
`,
	// IL: RESET of a counter, which sets its input R.
	"IEC-59": `
PROGRAM TEST_IEC59
VAR_OUTPUT failures : INT; done : BOOL; a, b : INT; END_VAR
VAR c : CTU; d : CTUD; END_VAR
LD 5
PV c
LD TRUE
CU c
LD c.CV
ST a
LD TRUE
RESET c
LD c.CV
ADD a
ST a
LD 5
PV d
LD TRUE
CU d
LD TRUE
RESET d
LD d.CV
ST b
LD a
NE 1
JMPC bad
LD b
EQ 0
JMPC fin
bad: LD 1
ST failures
fin: LD TRUE
ST done
END_PROGRAM
`,
	// SPLIT_DATE, SPLIT_TOD and SPLIT_DT, functions with outputs.
	"IEC-60": `
PROGRAM TEST_IEC60
VAR_OUTPUT failures : INT; done : BOOL; y, mo, d, h, mi, s, ms : INT; y2 : DINT; h2, ms2 : INT; END_VAR
VAR dt : DT := DT#2026-10-14-16:45:30.250; END_VAR
SPLIT_DT(IN := dt, YEAR => y, MONTH => mo, DAY => d, HOUR => h, MINUTE => mi, SECOND => s, MILLISECOND => ms);
SPLIT_DATE(D#2024-02-29, YEAR => y2);
SPLIT_TOD(IN := TOD#08:05:00.125, HOUR => h2, MILLISECOND => ms2);
IF y <> 2026 OR mo <> 10 OR d <> 14 OR h <> 16 OR mi <> 45 OR s <> 30 OR ms <> 250 THEN failures := failures + 1; END_IF;
IF y2 <> 2024 OR h2 <> 8 OR ms2 <> 125 THEN failures := failures + 2; END_IF;
done := TRUE;
END_PROGRAM
`,
	// DAY_OF_WEEK of IEC 61131-3: 0 for Sunday to 6 for Saturday.
	"IEC-61": `
PROGRAM TEST_IEC61
VAR_OUTPUT failures : INT; done : BOOL; sun, wed : INT; END_VAR
sun := DAY_OF_WEEK(D#2026-10-18);
wed := DAY_OF_WEEK(IN := D#2026-10-14);
IF sun <> 0 OR wed <> 3 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// A program's own DAY_OF_WEEK (as OSCAT's library declares) is called.
	"IEC-61-declared": `
FUNCTION DAY_OF_WEEK : INT
VAR_INPUT IDATE : DATE; END_VAR
DAY_OF_WEEK := 42;
END_FUNCTION
PROGRAM TEST_IEC61D
VAR_OUTPUT failures : INT; done : BOOL; n : INT; END_VAR
n := DAY_OF_WEEK(D#2026-10-18);
IF n <> 42 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// ?= to variables named like the keywords R and S.
	"IEC-62": `
INTERFACE I62 END_INTERFACE
FUNCTION_BLOCK F62 IMPLEMENTS I62 END_FUNCTION_BLOCK
PROGRAM TEST_IEC62
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR a : I62; f : F62; r : REF_TO F62; s : I62; END_VAR
r ?= a;
a := f; r ?= a; s ?= a;
IF r = NULL OR s = NULL THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// SFC transition conditions in IL.
	"IEC-63": `
PROGRAM TEST_IEC63
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR n : INT; stop : BOOL; END_VAR
ACTION Count: n := n + 1; END_ACTION
ACTION Finish: done := TRUE; IF n <> 4 THEN failures := 1; END_IF; END_ACTION
INITIAL_STEP S0: Count(N); END_STEP
TRANSITION FROM S0 TO S1 :
  LD n
  GE 2
  ANDN stop
END_TRANSITION
STEP S1: Count(N); END_STEP
TRANSITION T12 FROM S1 TO S2 :
  LD 0
  LIMIT n, 3
  EQ( LD 1
  ADD 2 )
END_TRANSITION
STEP S2: Finish(N); END_STEP
END_PROGRAM
`,
	// A TOD from DT_TO_TOD compares by its clock.
	"IEC-64": `
PROGRAM TEST_IEC64
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR d : DT := DT#2026-01-01-01:00:00; t : TOD; END_VAR
t := DT_TO_TOD(d);
IF DT_TO_TOD(d) <> TOD#01:00:00 OR t <> TOD#01:00:00 OR NOT (t < TOD#01:00:01) OR NOT (t > TOD#00:59:59) THEN failures := 1; END_IF;
IF ADD_DT_TIME(DT#2026-01-01-00:00:00, T#1h) <> d OR SUB_DT_DT(d, DT#2026-01-01-00:00:00) <> T#1h THEN failures := failures + 2; END_IF;
done := TRUE;
END_PROGRAM
`,
	// DAY_OF_WEEK assigned to integers of other types.
	"IEC-65": `
PROGRAM TEST_IEC65
VAR_OUTPUT failures : INT; done : BOOL; u : USINT; l : DINT; r : REAL; END_VAR
u := DAY_OF_WEEK(D#2026-10-14);
l := DAY_OF_WEEK(D#2026-10-17);
r := DAY_OF_WEEK(D#2026-10-18);
IF u <> 3 OR l <> 6 OR r <> 0.0 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// An integer assigned to a REAL or LREAL becomes one.
	"IEC-67": `
TYPE S67 : STRUCT x : REAL; END_STRUCT; END_TYPE
FUNCTION_BLOCK FB67
VAR_INPUT x : REAL; END_VAR
VAR_OUTPUT half : REAL; END_VAR
half := x / 2;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC67
VAR_OUTPUT failures : INT; done : BOOL; r : REAL; l : LREAL; END_VAR
VAR i : INT := 3; s : S67; a : ARRAY[0..1] OF REAL; f : FB67; END_VAR
r := i; r := r / 2;
l := i; l := l / 2;
s.x := i; s.x := s.x / 2;
a[1] := i; a[1] := a[1] / 2;
f(x := i);
IF r <> 1.5 OR l <> 1.5 THEN failures := failures + 1; END_IF;
IF s.x <> 1.5 OR a[1] <> 1.5 THEN failures := failures + 2; END_IF;
IF f.half <> 1.5 THEN failures := failures + 4; END_IF;
done := TRUE;
END_PROGRAM
`,
	// PROPERTY with GET and SET; a setter reads the value as value or by
	// the property's name.
	"IEC-66": `
FUNCTION_BLOCK FB66
VAR lvl : INT; cnt : INT; END_VAR
PROPERTY Level : INT
GET Level := lvl; END_GET
SET lvl := Level; END_SET
END_PROPERTY
PROPERTY Count : INT
GET Count := cnt; END_GET
SET cnt := cnt + value; END_SET
END_PROPERTY
PROPERTY Double : INT
GET Double := lvl * 2; END_GET
END_PROPERTY
END_FUNCTION_BLOCK
PROGRAM TEST_IEC66
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR f : FB66; END_VAR
f.Level := 5;
f.Count := 2; f.Count := 3;
IF f.Level <> 5 OR f.Count <> 5 OR f.Double <> 10 OR f.Level + f.Double <> 15 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Formal calls of standard functions, LIMIT(MN := 0, IN := v, MX := 20).
	"IEC-56": `
PROGRAM TEST_IEC56
VAR_OUTPUT failures : INT; done : BOOL; a, b, c, d, e : INT; s : STRING; END_VAR
VAR v : INT := 50; gt : TON; END_VAR
a := LIMIT(MX := 20, IN := v, MN := 0);
b := SEL(G := TRUE, IN0 := 1, IN1 := 2);
c := MAX(IN2 := v, IN1 := 3);
d := MUX(K := 1, IN0 := 7, IN1 := 8, IN2 := 9);
e := ABS(IN := -4);
s := LEFT(IN := 'abcdef', L := 2);
gt(IN := TRUE, PT := T#0s);
IF a <> 20 OR b <> 2 OR c <> 50 OR d <> 8 OR e <> 4 OR s <> 'ab' OR NOT gt.Q THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Typed enumerations, INT (A := 1, B := 2) := A.
	"IEC-32": `
TYPE LEVEL32 : INT (LOW := 10, MID := 20, HIGH := 30) := MID; END_TYPE
TYPE RANGE32 : INT (0..100); END_TYPE
PROGRAM TEST_IEC32
VAR_OUTPUT failures : INT; done : BOOL; n : INT; END_VAR
VAR l : LEVEL32; h : LEVEL32 := LEVEL32#HIGH; r : RANGE32 := 50; END_VAR
n := TO_INT(l);
IF n <> 20 OR TO_INT(h) <> 30 OR l <> LEVEL32#MID OR r <> 50 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Enumeration values named without their type where that is clear.
	"IEC-33": `
TYPE COLOR33 : (RED, GREEN, BLUE); END_TYPE
TYPE SIGNAL33 : (STOP, GO, BLUE); END_TYPE
FUNCTION_BLOCK FB33
VAR_INPUT c : COLOR33; END_VAR
VAR_OUTPUT isGreen : BOOL; END_VAR
isGreen := c = GREEN;
END_FUNCTION_BLOCK
PROGRAM TEST_IEC33
VAR_OUTPUT failures : INT; done : BOOL; n : INT; END_VAR
VAR c : COLOR33 := RED; s : SIGNAL33; f : FB33; RED33 : INT := 7; END_VAR
CASE c OF
  RED: n := 1;
  GREEN: n := 2;
ELSE n := 3;
END_CASE;
c := GREEN;
s := GO;
f(c := GREEN);
IF n <> 1 OR c <> GREEN OR s <> SIGNAL33#GO OR NOT f.isGreen OR c = COLOR33#BLUE OR RED33 <> 7 THEN failures := 1; END_IF;
done := TRUE;
END_PROGRAM
`,
	// Named SFC transitions, with a priority.
	"IEC-34": `
PROGRAM TEST_IEC34
VAR_OUTPUT failures : INT; done : BOOL; n : INT; END_VAR
ACTION Count: n := n + 1; END_ACTION
ACTION Finish: done := TRUE; END_ACTION
INITIAL_STEP S0: Count(N); END_STEP
TRANSITION T1 FROM S0 TO S1 := n >= 2; END_TRANSITION
STEP S1: Finish(N); END_STEP
TRANSITION Back (PRIORITY := 1) FROM S1 TO S0 := FALSE; END_TRANSITION
END_PROGRAM
`,
}

// TestConformanceOnEveryEngine runs each item's program on the evaluator,
// the VM and the transpiled Go (where the Go toolchain is), which must pass
// and agree scan by scan.
func TestConformanceOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	for id, src := range conformance {
		t.Run(id, func(t *testing.T) {
			results, err := sil.Run(context.Background(), src, sil.Options{Engines: engines, MaxScans: 100, GoTimeout: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) == 0 {
				t.Fatal("no test program")
			}
			for _, r := range results {
				if !r.Passed {
					t.Errorf("%s failed: mismatch %q", r.Name, r.Mismatch)
					for _, e := range r.Engines {
						t.Logf("    %s: %v", e.Engine, e.Problems())
					}
				}
			}
		})
	}
}
