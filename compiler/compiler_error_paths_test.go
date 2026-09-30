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

import "testing"

// compileErrorCase is an input the compiler must reject with exactly this error.
type compileErrorCase struct {
	name  string
	input string
	want  string
}

func runCompileErrorTests(t *testing.T, tests []compileErrorCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New().Compile(parse(t, tt.input))
			if err == nil {
				t.Fatalf("expected error %q, got none", tt.want)
			}
			if err.Error() != tt.want {
				t.Fatalf("wrong error.\nwant: %s\ngot:  %s", tt.want, err)
			}
		})
	}
}

// TestErrorsInsideStatements checks that an error in a nested expression or
// statement is reported from every kind of enclosing construct.
func TestErrorsInsideStatements(t *testing.T) {
	const undefined = "undefined variable nope"
	runCompileErrorTests(t, []compileErrorCase{
		{"program body", "PROGRAM P nope; END_PROGRAM", undefined},
		{"assignment value", "nope := 1;", undefined},
		{"array index target", "VAR a : ARRAY[0..2] OF INT; END_VAR a[nope] := 1;", undefined},
		{"array element value", "VAR a : ARRAY[0..2] OF INT; END_VAR a[0] := nope;", undefined},
		{"array base", "nope[0] := 1;", undefined},
		{"IF condition", "IF nope THEN 1; END_IF", undefined},
		{"IF branch", "IF TRUE THEN nope; END_IF", undefined},
		{"ELSIF condition", "IF FALSE THEN 1; ELSIF nope THEN 2; END_IF", undefined},
		{"ELSE branch", "IF FALSE THEN 1; ELSE nope; END_IF", undefined},
		{"WHILE condition", "WHILE nope DO END_WHILE", undefined},
		{"WHILE body", "WHILE TRUE DO nope; END_WHILE", undefined},
		{"REPEAT body", "REPEAT nope; UNTIL TRUE END_REPEAT", undefined},
		{"REPEAT condition", "REPEAT UNTIL nope END_REPEAT", undefined},
		{"FOR start", "VAR i : INT; END_VAR FOR i := nope TO 3 DO END_FOR", undefined},
		{"FOR end", "VAR i : INT; END_VAR FOR i := 1 TO nope DO END_FOR", undefined},
		{"FOR step", "VAR i : INT; END_VAR FOR i := 1 TO 3 BY nope DO END_FOR", undefined},
		{"FOR body", "VAR i : INT; END_VAR FOR i := 1 TO 3 DO nope; END_FOR", undefined},
		{"CASE selector", "CASE nope OF 1: 2; END_CASE", undefined},
		{"CASE label", "CASE 1 OF nope: 2; END_CASE", undefined},
		{"CASE range low", "CASE 1 OF nope..2: 2; END_CASE", undefined},
		{"CASE range high", "CASE 1 OF 1..nope: 2; END_CASE", undefined},
		{"CASE branch", "CASE 1 OF 1: nope; END_CASE", undefined},
		{"CASE ELSE", "CASE 1 OF 1: 2; ELSE nope; END_CASE", undefined},
		{"negation", "-nope;", undefined},
		{"NOT", "NOT nope;", undefined},
		{"infix left", "nope + 1;", "undefined identifier: nope"},
		{"infix right", "1 + nope;", "undefined identifier: nope"},
		{"array literal", "[1, nope];", undefined},
		{"hash value", `{"a": nope};`, undefined},
		{"hash key", `{nope: 1};`, undefined},
		{"index", "[1, 2][nope];", undefined},
		{"indexed value", "nope[1];", undefined},
		{"call target", "nope();", undefined},
		{"positional argument", "FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION F(nope);", undefined},
		{"named argument", "FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION F(a := nope);", undefined},
		{"output target", "FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR F := 1; END_FUNCTION F(o => nope);", undefined},
		{"RETURN value", "FUNCTION F : INT RETURN nope; END_FUNCTION", undefined},
		{"function local initializer", "FUNCTION F : INT VAR x : INT := nope; END_VAR END_FUNCTION", undefined},
		{"function result", "FUNCTION F : INT F := nope; END_FUNCTION", undefined},
		{"FB body", "FUNCTION_BLOCK FB VAR_OUTPUT o : INT; END_VAR o := nope; END_FUNCTION_BLOCK", undefined},
		{"method body", "FUNCTION_BLOCK FB METHOD M : INT M := nope; END_METHOD END_FUNCTION_BLOCK", undefined},
		{"property getter", "FUNCTION_BLOCK FB PROPERTY P : INT GET P := nope; END_GET END_PROPERTY END_FUNCTION_BLOCK", undefined},
		{"property setter", "FUNCTION_BLOCK FB PROPERTY P : INT SET x := nope; END_SET END_PROPERTY END_FUNCTION_BLOCK", undefined},
	})
}

func TestStatementContextErrors(t *testing.T) {
	runCompileErrorTests(t, []compileErrorCase{
		{"EXIT outside a loop", "EXIT;", "EXIT statement not within a loop"},
		{"THIS outside an FB", "THIS;", "cannot use THIS outside of a function block context"},
		{"SUPER outside an FB", "SUPER^.X();", "cannot use SUPER outside of a function block context"},
		{"VAR_CONFIG outside a configuration", "VAR_CONFIG x : INT := 1; END_VAR", "VAR_CONFIG is only valid inside a CONFIGURATION block"},
		{"output the function lacks", "FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR F := 1; END_FUNCTION VAR x : INT; END_VAR F(z => x);",
			"function 'F' has no output 'z'"},
		{"output of a function with none", "FUNCTION F : INT F := 1; END_FUNCTION VAR x : INT; END_VAR F(z => x);",
			"function 'F' has no output 'z'"},
	})
}

func TestTypeErrors(t *testing.T) {
	runCompileErrorTests(t, []compileErrorCase{
		{"MOD on a real", "5 MOD 2.0;", "type error in expression '(5 MOD 2.0)': operator 'MOD' not defined for types LINT and LREAL"},
		{"string subtraction", "'a' - 'b';", "type error in expression '(a - b)': operator '-' not defined for types STRING and STRING"},
		{"BOOL arithmetic", "TRUE + 1;", "type error in expression '(TRUE + 1)': operator '+' not defined for types BOOLEAN and LINT"},
		{"string and number", "'a' < 1;", "type error in expression '(a < 1)': comparison operator '<' not defined for types STRING and LINT"},
		{"logic on reals", "1.5 AND 2.5;", "type error in expression '(1.5 AND 2.5)': logical operator 'AND' not defined for types LREAL and LREAL"},
		{"TIME times TIME", "T#1s * T#1s;", "type error in expression '(T#1s * T#1s)': operator '*' not defined for types TIME and TIME"},
	})
}

func TestInvalidTypedLiterals(t *testing.T) {
	runCompileErrorTests(t, []compileErrorCase{
		{"INT", "INT#abc;", `invalid INT literal 'abc': strconv.ParseInt: parsing "abc": invalid syntax`},
		{"UINT", "UINT#-1;", `invalid UINT literal '-1': strconv.ParseUint: parsing "-1": invalid syntax`},
		{"REAL", "REAL#abc;", `invalid REAL/LREAL literal 'abc': strconv.ParseFloat: parsing "abc": invalid syntax`},
		{"BYTE too wide", "BYTE#16#FFF;", "BYTE literal '16#FFF' does not fit in 8 bits"},
		{"TIME", "T#bad;", `invalid TIME literal 'bad': time: invalid duration "bad"`},
		{"DATE", "D#bad;", `invalid DATE literal 'bad': parsing time "bad" as "2006-01-02": cannot parse "bad" as "2006"`},
		{"TIME_OF_DAY", "TOD#bad;", `invalid TIME_OF_DAY literal 'bad': parsing time "bad" as "15:04:05": cannot parse "bad" as "15"`},
		{"DATE_AND_TIME", "DT#bad;", `invalid DATE_AND_TIME literal 'bad': parsing time "bad" as "2006-01-02-15:04:05": cannot parse "bad" as "2006"`},
	})
}
