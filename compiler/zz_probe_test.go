package compiler

import (
	"beedance/lexer"
	"beedance/parser"
	"fmt"
	"testing"
)

func zzCompile(src string) {
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Printf("PARSE  | %-60.60s | %s\n", src, p.Errors()[0])
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC  | %-60.60s | %v\n", src, r)
		}
	}()
	err := New().Compile(prog)
	fmt.Printf("RESULT | %-60.60s | %v\n", src, err)
}

func TestZZProbe(t *testing.T) {
	for _, src := range []string{
		// error propagation inside constructs
		"PROGRAM P missing; END_PROGRAM",
		"VAR_CONFIG x : INT := 1; END_VAR",
		"TYPE T1 : STRUCT a : INT := nope; END_STRUCT; END_TYPE",
		"TYPE Alias : INT; END_TYPE",
		"INTERFACE I1 EXTENDS Nope END_INTERFACE",
		"FUNCTION_BLOCK FB1 END_FUNCTION_BLOCK INTERFACE I2 EXTENDS FB1 END_INTERFACE",
		"FUNCTION_BLOCK D EXTENDS Nope END_FUNCTION_BLOCK",
		"FUNCTION F1 : INT END_FUNCTION FUNCTION_BLOCK D EXTENDS F1 END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK FB2 VAR_INPUT a : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := nope; END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK FB3 METHOD M : INT M := nope; END_METHOD END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK FB4 PROPERTY P : INT GET P := nope; END_GET END_PROPERTY END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK FB5 PROPERTY P : INT SET x := nope; END_SET END_PROPERTY END_FUNCTION_BLOCK",
		"FUNCTION F2 : INT VAR x : INT := nope; END_VAR END_FUNCTION",
		"FUNCTION F3 : INT F3 := nope; END_FUNCTION",
		"nope := 1;",
		"VAR a : ARRAY[0..2] OF INT; END_VAR a[nope] := 1;",
		"VAR a : ARRAY[0..2] OF INT; END_VAR a[0] := nope;",
		"VAR a : ARRAY[0..2] OF INT; END_VAR nope[0] := 1;",
		"IF nope THEN 1; END_IF",
		"IF TRUE THEN nope; END_IF",
		"IF FALSE THEN 1; ELSIF nope THEN 2; END_IF",
		"IF FALSE THEN 1; ELSE nope; END_IF",
		"WHILE nope DO END_WHILE",
		"WHILE TRUE DO nope; END_WHILE",
		"REPEAT nope; UNTIL TRUE END_REPEAT",
		"REPEAT UNTIL nope END_REPEAT",
		"VAR i : INT; END_VAR FOR i := nope TO 3 DO END_FOR",
		"VAR i : INT; END_VAR FOR i := 1 TO nope DO END_FOR",
		"VAR i : INT; END_VAR FOR i := 1 TO 3 BY nope DO END_FOR",
		"VAR i : INT; END_VAR FOR i := 1 TO 3 DO nope; END_FOR",
		"FOR k := 1 TO 3 DO END_FOR; k;",
		"CASE nope OF 1: 2; END_CASE",
		"CASE 1 OF nope: 2; END_CASE",
		"CASE 1 OF 1..nope: 2; END_CASE",
		"CASE 1 OF nope..2: 2; END_CASE",
		"CASE 1 OF 1: nope; END_CASE",
		"CASE 1 OF 1: 2; ELSE nope; END_CASE",
		"EXIT;",
		"-nope;",
		"NOT nope;",
		"1 + nope;",
		"nope + 1;",
		"[1, nope];",
		`{"a": nope};`,
		`{nope: 1};`,
		"[1,2][nope];",
		"nope[1];",
		"THIS;",
		"SUPER^.X();",
		"nope();",
		"FUNCTION F4 : INT VAR_INPUT a : INT; END_VAR F4 := a; END_FUNCTION F4(a := nope);",
		"FUNCTION F5 : INT VAR_INPUT a : INT; END_VAR F5 := a; END_FUNCTION F5(nope);",
		"FUNCTION F6 : INT VAR_OUTPUT o : INT; END_VAR F6 := 1; END_FUNCTION F6(o => nope);",
		"FUNCTION F7 : INT RETURN nope; END_FUNCTION",
		"RETURN;",
		"RETURN 5;",
		"TON#1;",
		"INT#abc;",
		"T#bad;",
		"D#bad;",
		"TOD#bad;",
		"DT#bad;",
		"UINT#-1;",
		"REAL#abc;",
		"BYTE#16#FFF;",
		"Color#Red;",
		"TOD#10:30:00.5;",
		"DT#2024-01-01-10:30:00.5;",
		"USINT#16#FF;",
		"DWORD#16#FF;",
		"LWORD#16#FF;",
		"5 MOD 2.0;",
		"'a' - 'b';",
		"TRUE + 1;",
		"'a' < 1;",
		"1.5 AND 2.5;",
		"T#1s * T#1s;",
		`"a" + "b";`,
		`'a' + "b";`,
		"UINT#5 + 1;",
		"LREAL#1.5 + 1;",
		"BYTE#16#0F OR 16#F0;",
		"TOD#10:00:00 - TOD#09:00:00;",
		"DT#2024-01-01-10:00:00 - T#1h;",
		"TOD#10:00:00 < TOD#11:00:00;",
		"-T#1s;",
	} {
		zzCompile(src)
	}
}
