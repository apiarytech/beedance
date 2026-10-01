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

import "testing"

// These tests cover what transpiling the converted OSCAT BASIC library
// needed.

// A comment that spans several lines is a Go comment on each line.
func TestMultiLineComments(t *testing.T) {
	checkContains(t, "(* first\nsecond *)\nFUNCTION_BLOCK Fb VAR x : INT; END_VAR x := 1; END_FUNCTION_BLOCK",
		"// first // second //")
}

// A member is spelled as declared, whatever case the program writes it in:
// of a standard function block, of the program's own function blocks and
// structures, through an array element and through a global.
func TestMemberSpelling(t *testing.T) {
	checkContains(t, "PROGRAM P VAR t : TON; b : BOOL; END_VAR t(in := TRUE, pt := T#1s); b := t.q; END_PROGRAM",
		"p.t.IN = true", "p.t.PT = ", "p.b = p.t.Q")
	checkContains(t, `TYPE Rec : STRUCT Adress : INT; END_STRUCT; END_TYPE
FUNCTION_BLOCK Fb VAR_INPUT SET : BOOL; END_VAR END_FUNCTION_BLOCK
VAR_GLOBAL g : Rec; END_VAR
PROGRAM P VAR f : Fb; r : ARRAY[0..1] OF Rec; x : INT; END_VAR f(Set := TRUE); x := r[1].adress + g.ADRESS; END_PROGRAM`,
		"p.f.SET = true", "p.r[1].Adress", "g.Adress")
}

// A type written in another case than declared is the declared type.
func TestTypeCase(t *testing.T) {
	checkContains(t, "TYPE Real2 : STRUCT R1 : REAL; END_STRUCT; END_TYPE FUNCTION F : REAL VAR_INPUT x : real2; END_VAR F := x.r1; END_FUNCTION",
		"func F(x Real2)", "F = x.R1")
}

// A TIME is multiplied and divided by numbers, as IEC 61131-3 allows.
func TestTimeScaling(t *testing.T) {
	checkContains(t, "FUNCTION F : TIME VAR_INPUT d : TIME; n : INT; r : REAL; END_VAR F := d / n + d * r + 2 * d; END_FUNCTION",
		"(d / iec.TIME(n))", "iec.TIME(float64(d) * float64(r))", "(d * iec.TIME(2))")
}

// 0 and 1 are BOOL literals.
func TestBoolLiterals(t *testing.T) {
	checkContains(t, "FUNCTION F : BOOL VAR on : BOOL := 1; off : BOOL := 0; END_VAR off := 1; F := on AND off; END_FUNCTION",
		"var on iec.BOOL = true", "var off iec.BOOL = false", "off = true")
}

// A function block named with a leading underscore has a letter receiver.
func TestReceiverNames(t *testing.T) {
	checkContains(t, "FUNCTION_BLOCK _RMP_B VAR_INPUT E : BOOL := TRUE; END_VAR END_FUNCTION_BLOCK",
		"func (r *_RMP_B) Init()", "func (r *_RMP_B) Logic(now time.Time)")
	if got := receiverOf("_9"); got != "f" {
		t.Errorf("receiverOf(_9) = %s", got)
	}
}

// An integer literal too big for the other operand widens the operation;
// a bit string keeps its width.
func TestLiteralWidth(t *testing.T) {
	checkContains(t, "FUNCTION F : DINT VAR_INPUT m : INT; END_VAR F := m * 60000; END_FUNCTION",
		"iec.DINT(m) * 60000")
	checkContains(t, "FUNCTION F : DWORD VAR_INPUT w : DWORD; END_VAR F := w AND 16#FF00FF00; END_FUNCTION",
		"(w & 4278255360)")
	checkContains(t, "FUNCTION F : LINT VAR_INPUT m : INT; END_VAR F := m + 5000000000; END_FUNCTION",
		"iec.LINT(m) + 5000000000")
}

// A standard function with only literal arguments takes the type of the
// value it is compared with.
func TestLiteralCallTyping(t *testing.T) {
	checkContains(t, "FUNCTION F : BOOL VAR_INPUT b : BYTE; g : BOOL; END_VAR F := b <> SEL(g, 0, 255); END_FUNCTION",
		"(b != iec.BYTE(selection.SEL(")
}

// A value passed to a function block input is converted to its type.
func TestFunctionBlockInputWidening(t *testing.T) {
	checkContains(t, "FUNCTION_BLOCK Fb VAR_INPUT x : DWORD; END_VAR END_FUNCTION_BLOCK PROGRAM P VAR f : Fb; b : BYTE; END_VAR f(x := b); END_PROGRAM",
		"p.f.x = iec.DWORD(p.b)")
}

// The difference of two times of day is a TIME, compared as one.
func TestTimeDifferenceType(t *testing.T) {
	checkContains(t, "FUNCTION F : BOOL VAR_INPUT a, b : DT; END_VAR F := a - b >= T#25s; END_FUNCTION",
		"time.Time(a).Sub(time.Time(b))) >= iec.TIME(25000000000)")
}

// A function's result variable has the function's type, members included.
func TestFunctionResultMembers(t *testing.T) {
	checkContains(t, "TYPE Rec : STRUCT Adress : INT; END_STRUCT; END_TYPE FUNCTION F : Rec F.adress := 1; END_FUNCTION",
		"F.Adress = 1")
}
