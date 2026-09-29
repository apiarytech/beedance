package evaluator

import "testing"

func TestLiteralEvaluation(t *testing.T) {
	checkEval(t, []evalCase{
		{"TIME#5s;", "T#5s"},
		{"DATE#2026-01-02;", "D#2026-01-02"},
		{"TOD#12:30:00;", "TOD#12:30:00"},
		{"DT#2026-01-02-12:30:00;", "DT#2026-01-02-12:30:00"},
		{"UDINT#5;", "5"},
		{"INT#16#FF;", "255"},
		{"REAL#1.5;", "1.500000"},
		{"BYTE#16#FF;", "BYTE#16#FF"},
		{"T#1d2h;", "T#26h0m0s"},
		{"T#5us;", "T#5µs"},
		{"T#5ns;", "T#5ns"},
		{"[3(0)];", "[0, 0, 0]"},
		{"[2(1, 2), 5];", "[1, 2, 1, 2, 5]"},
		{"{1: 2};", "{1: 2}"},
		{"INT_TO_REAL;", "builtin function"},
		{"BOOL#2;", "ERROR: invalid value for BOOL literal: 2"},
		{"TYPE Color : (Red, Green); END_TYPE Color#Purple;", "ERROR: enumerated value 'Purple' not found in type 'COLOR'"},
		{"FOO#1;", "ERROR: unknown type: FOO"},
		{"TOD#bad;", "ERROR: could not parse TIME_OF_DAY literal"},
		{"DT#bad;", "ERROR: could not parse DATE_AND_TIME literal"},
		{"T#5x;", `ERROR: unknown duration unit "x"`},
		{"UDINT#99999999999;", "ERROR: value 99999999999 is out of range for type UDINT"},
		{"DINT#99999999999999999999;", "ERROR: value 99999999999999999999 is out of range for type DINT"},
		{"REAL#abc;", `ERROR: could not parse "abc" as REAL`},
		{"UDINT#abc;", `ERROR: could not parse "abc" as UDINT`},
		{"INT#abc;", `ERROR: could not parse "abc" as INT`},
		{"{missing: 2};", "ERROR: identifier not found: missing"},
		{"{1: missing};", "ERROR: identifier not found: missing"},
		{"{[1]: 2};", "ERROR: unusable as hash key: ARRAY"},
		{"[missing(0)];", "ERROR: identifier not found: missing"},
		{"[2(missing)];", "ERROR: identifier not found: missing"},
		{"THIS;", "ERROR: THIS keyword used outside of a method or property context"},
		{"SUPER^.M();", "ERROR: SUPER keyword used outside of a method or property context"},
		{"EXPR(1, 2);", "ERROR: wrong number of arguments for EXPR. got=2, want=1"},
		{"1 + missing;", "ERROR: identifier not found: missing"},
		{"- 'abc';", "ERROR: unknown operator: - STRING"},
		{"NOT 'abc';", "ERROR: unknown operator: NOT STRING"},
		{"VAR r : REAL := 1.5; END_VAR -r;", "-1.500000"},
	})
}

func TestDeclarationDefaults(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR w : WSTRING; END_VAR w;", ""},
		{"VAR b : BYTE; END_VAR b;", "BYTE#16#0"},
		{"VAR d : DATE; END_VAR d;", "D#0001-01-01"},
		{"VAR d : TOD; END_VAR d;", "TOD#00:00:00"},
		{"VAR d : DT; END_VAR d;", "DT#0001-01-01-00:00:00"},
		{"VAR_TEMP t1 : INT := 3; END_VAR t1;", "3"},
		{"VAR CONSTANT k : INT := 3; END_VAR k;", "3"},
		{"TYPE Pt : STRUCT px : INT := 2; END_STRUCT; END_TYPE VAR CONSTANT p : Pt; END_VAR p.px;", "2"},
		{"VAR x : INT := missing; END_VAR", "ERROR: identifier not found: missing"},
		{"VAR x : SINT := 300; END_VAR", "ERROR: value 300 is out of range for type SINT (-128 to 127)"},
	})
}

func TestArrayVariables(t *testing.T) {
	checkEval(t, []evalCase{
		// Elements start at the element type's default.
		{"VAR a : ARRAY[0..2] OF INT; END_VAR a;", "[0, 0, 0]"},
		{"VAR a : ARRAY[0..1, 0..1] OF BOOL; END_VAR a;", "[[false, false], [false, false]]"},
		{"TYPE Pt : STRUCT px : INT := 4; END_STRUCT; END_TYPE VAR a : ARRAY[0..1] OF Pt; END_VAR a[1].px;", "4"},
		// A short literal is padded with defaults.
		{"VAR a : ARRAY[0..2] OF INT := [10]; END_VAR a;", "[10, 0, 0]"},
		{"VAR CONSTANT n : INT := 2; END_VAR VAR a : ARRAY[0..n] OF INT; END_VAR a;", "[0, 0, 0]"},
		// Elements can be assigned and read.
		{"VAR a : ARRAY[0..2] OF INT; END_VAR a[1] := 5; a[1];", "5"},
		{"VAR a : ARRAY[0..2] OF INT := [10, 20, 30]; END_VAR a[1] := 5; a;", "[10, 5, 30]"},
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3, 4]; END_VAR", "ERROR: array 'a' holds 3 elements, but its initial value has 4"},
		{"VAR a : ARRAY[0..missing] OF INT; END_VAR", "ERROR: identifier not found: missing"},
		{"VAR a : ARRAY[0..'x'] OF INT; END_VAR", "ERROR: bound must be an integer, got STRING"},
		{"VAR a : ARRAY[3..1] OF INT; END_VAR", "ERROR: upper bound 1 is below lower bound 3"},
		{"VAR a : ARRAY[0..2] OF INT; END_VAR a[7] := 5;", "ERROR: index out of bounds: 7"},
		{"VAR a : ARRAY[0..2] OF INT; END_VAR a['x'];", "ERROR: array index must be an integer, got STRING"},
		{"VAR i : INT; END_VAR i[0] := 5;", "ERROR: index operator not supported for assignment: LINT"},
		{"missing[0] := 1;", "ERROR: identifier not found: missing"},
		{"VAR a : ARRAY[0..2] OF INT; END_VAR a[missing] := 1;", "ERROR: identifier not found: missing"},
	})
}

func TestAssignmentErrors(t *testing.T) {
	const pt = "TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR p : Pt; END_VAR "
	checkEval(t, []evalCase{
		{"missing.x := 1;", "ERROR: identifier not found: missing"},
		{pt + "p.nope := 1;", "ERROR: structure has no member 'nope'"},
		{pt + "p.nope;", "ERROR: structure has no member 'nope'"},
		{"VAR i : INT; END_VAR i.x := 1;", "ERROR: left side of member assignment is not a function block instance, got LINT"},
		{"VAR i : INT; END_VAR i.x;", "ERROR: member access not supported for type LINT"},
		{"missing.x;", "ERROR: identifier not found: missing"},
	})
}

func TestControlFlowEdges(t *testing.T) {
	checkEval(t, []evalCase{
		// A CASE with no matching branch and no ELSE does nothing.
		{"VAR x : INT; END_VAR CASE 5 OF 1: x := 1; END_CASE", "null"},
		// Non-numeric ranges compare with the generic operators.
		{"VAR x : INT; END_VAR CASE 'b' OF 'a'..'c': x := 1; END_CASE x;", "1"},
		{"VAR x : INT; END_VAR REPEAT x := x + 1; EXIT; UNTIL FALSE END_REPEAT x;", "1"},
		{"VAR x : INT; END_VAR CASE missing OF 1: x := 1; END_CASE", "ERROR: identifier not found: missing"},
		{"CASE 1 OF 1: missing := missing + 1; END_CASE", "ERROR: identifier not found: missing"},
		{"CASE 1 OF missing..3: ; END_CASE", "ERROR: identifier not found: missing"},
		{"CASE 1 OF 1..missing: ; END_CASE", "ERROR: identifier not found: missing"},
		{"CASE 1 OF missing: ; END_CASE", "ERROR: identifier not found: missing"},
		{"FOR i := missing TO 3 DO END_FOR", "ERROR: identifier not found: missing"},
		{"FOR i := 'a' TO 3 DO END_FOR", "ERROR: FOR loop start value must be an integer, got STRING"},
		{"FOR i := 1 TO missing DO END_FOR", "ERROR: identifier not found: missing"},
		{"FOR i := 1 TO 'a' DO END_FOR", "ERROR: FOR loop end value must be an integer, got STRING"},
		{"FOR i := 1 TO 3 BY missing DO END_FOR", "ERROR: identifier not found: missing"},
		{"FOR i := 1 TO 3 BY 'a' DO END_FOR", "ERROR: FOR loop step value must be an integer, got STRING"},
		{"WHILE missing DO END_WHILE", "ERROR: identifier not found: missing"},
		{"WHILE TRUE DO missing := missing + 1; END_WHILE", "ERROR: identifier not found: missing"},
		{"REPEAT missing := missing + 1; UNTIL TRUE END_REPEAT", "ERROR: identifier not found: missing"},
		{"REPEAT UNTIL missing END_REPEAT", "ERROR: identifier not found: missing"},
		{"IF missing THEN END_IF", "ERROR: identifier not found: missing"},
	})
}

func TestStandaloneVarConfig(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR_CONFIG P1.x : INT := 1; END_VAR", "ERROR: name 'P1' not found in VAR_CONFIG path"},
	})
}
