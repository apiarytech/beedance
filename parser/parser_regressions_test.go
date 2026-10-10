/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package parser

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
)

// IL modifiers are split off only operators that take them; SIN, LN, TAN and
// ATAN keep their names.
func TestIlMnemonicModifiers(t *testing.T) {
	tests := []struct{ mnemonic, operator, modifier string }{
		{"LDN", "LD", "N"},
		{"JMPCN", "JMP", "CN"},
		{"RETC", "RET", "C"},
		{"ANDN", "AND", "N"},
		{"SIN", "SIN", ""},
		{"LN", "LN", ""},
		{"TAN", "TAN", ""},
		{"ATAN", "ATAN", ""},
	}
	p := New(lexer.New(""))
	for _, tt := range tests {
		op, mod := p.extractIlModifiers(tt.mnemonic)
		if op != tt.operator || mod != tt.modifier {
			t.Errorf("%s: got operator %q modifier %q, want %q %q", tt.mnemonic, op, mod, tt.operator, tt.modifier)
		}
	}

	p = New(lexer.New("PROGRAM Pg LD 1 SIN END_PROGRAM"))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	body := program.Statements[0].(*ast.ProgramDeclaration).Body.(*ast.BlockStatement)
	if il := body.Statements[1].(*ast.IlInstructionStatement); il.Operator != "SIN" {
		t.Errorf("expected operator SIN, got %q", il.Operator)
	}
}

// A time or date literal without a value is a parse error, not a crash.
func TestTypedLiteralWithoutValue(t *testing.T) {
	for _, input := range []string{"T#;", "D#;", "TOD#;", "DT#;"} {
		p := New(lexer.New(input))
		p.ParseProgram()
		if len(p.Errors()) == 0 || !strings.Contains(p.Errors()[0], "expected a value for typed literal") {
			t.Errorf("%s: expected a parse error, got %v", input, p.Errors())
		}
	}
}

// A typed literal may name a type declared in a namespace.
func TestQualifiedTypedLiteral(t *testing.T) {
	p := New(lexer.New("x := Lib.Inner.Mode#Busy;"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	lit, ok := program.Statements[0].(*ast.AssignmentStatement).Value.(*ast.TypedLiteral)
	if !ok || lit.TypeName != "Lib.Inner.Mode" {
		t.Fatalf("expected a typed literal of Lib.Inner.Mode, got %#v", program.Statements[0])
	}
	// A left side that is not a name is still an error.
	p = New(lexer.New("x := a[1]#Busy;"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected an error for a[1]#Busy")
	}
}

// A source may end with comments.
func TestTrailingComments(t *testing.T) {
	for _, src := range []string{
		"FUNCTION F : INT F := 1; END_FUNCTION\n(* trailing *)\n",
		"FUNCTION F : INT F := 1; END_FUNCTION\n// trailing\n",
		"x := 1; (* one *) (* two *)",
		"(* only a comment *)",
	} {
		p := New(lexer.New(src))
		p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Errorf("%q: %v", src, p.Errors())
		}
	}
}

// CODESYS dialect accepted in declarations: STRING(n) results and members,
// STRING[n] types, comments between structure members, and END_STRUCT
// without a semicolon.
func TestCodesysDeclarations(t *testing.T) {
	for _, src := range []string{
		"FUNCTION F : STRING(80) F := 'a'; END_FUNCTION",
		"FUNCTION F : STRING[80] F := 'a'; END_FUNCTION",
		"FUNCTION F : WSTRING(LEN_MAX) F := \"a\"; END_FUNCTION",
		"TYPE Str20 : STRING[20]; END_TYPE",
		"TYPE Rec : STRUCT a : INT; (* the a *) b : STRING(10); (* the b *) END_STRUCT END_TYPE",
		"TYPE Rec : STRUCT a : INT; END_STRUCT\nRec2 : STRUCT b : INT; END_STRUCT\nEND_TYPE",
	} {
		p := New(lexer.New(src))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Errorf("%s: %v", src, p.Errors())
			continue
		}
		if fn, ok := program.Statements[0].(*ast.FunctionDeclaration); ok && fn.ReturnLength == nil {
			t.Errorf("%s: the result's length was not kept", src)
		}
	}
	p := New(lexer.New("TYPE Rec : STRUCT b : STRING(10); END_STRUCT END_TYPE"))
	program := p.ParseProgram()
	member := program.Statements[0].(*ast.TypeBlockDeclaration).Declarations[0].DataType.(*ast.StructDefinition).Members[0]
	if member.StringLength == nil || member.StringLength.String() != "10" {
		t.Errorf("member length = %v", member.StringLength)
	}
	fn := New(lexer.New("FUNCTION F : STRING(80) F := 'a'; END_FUNCTION")).ParseProgram().Statements[0]
	if got := fn.String(); !strings.Contains(got, "FUNCTION F : STRING[80]") {
		t.Errorf("String() = %s", got)
	}
}

// CODESYS code: contextual keywords as names, bit access, the FUNCTIONBLOCK
// spelling, arrays of sized strings and array initializers without brackets.
func TestCodesysStatements(t *testing.T) {
	for _, tt := range []struct{ src, want string }{
		{"VAR SET : BOOL; STEP : INT; ON, GET : BOOL; END_VAR IF SET AND ON THEN STEP := STEP + 1; END_IF", "IF (SET AND ON) THEN"},
		{"VAR r_edge : BOOL; F_EDGE : BOOL; END_VAR r_edge := NOT F_EDGE;", "r_edge := (NOT F_EDGE)"},
		{"VAR x : BYTE; b : BOOL; END_VAR b := x.3; x.0 := b; b := a[1].7;", "b := x.3"},
		{"FUNCTIONBLOCK Fb VAR n : INT; END_VAR n := 1; END_FUNCTION_BLOCK", "FUNCTION_BLOCK Fb"},
		{"VAR a : ARRAY[1..3] OF STRING(10); END_VAR", "ARRAY [1 .. 3] OF STRING[10]"},
		{"VAR a : ARRAY[1..4] OF INT := 1, 3, 7, 15; END_VAR", "[1, 3, 7, 15]"},
		{"TYPE Rec : STRUCT a : ARRAY[0..1] OF INT := 4, 5; END_STRUCT END_TYPE", "[4, 5]"},
	} {
		p := New(lexer.New(tt.src))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Errorf("%s: %v", tt.src, p.Errors())
			continue
		}
		if got := program.String(); !strings.Contains(got, tt.want) {
			t.Errorf("%s:\n  expected %q in %s", tt.src, tt.want, got)
		}
	}
	// The written bit is kept, and a PROPERTY still reads SET as its setter.
	p := New(lexer.New("x.12 := TRUE;"))
	program := p.ParseProgram()
	bit, ok := program.Statements[0].(*ast.AssignmentStatement).Left.(*ast.BitAccessExpression)
	if !ok || bit.Bit != 12 || bit.Target.String() != "x" {
		t.Fatalf("expected a bit access of bit 12 of x, got %#v", program.Statements[0])
	}
	p = New(lexer.New("FUNCTION_BLOCK Fb VAR v : INT; END_VAR PROPERTY P : INT GET P := v; END_GET SET v := value; END_SET END_PROPERTY END_FUNCTION_BLOCK"))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Errorf("property: %v", p.Errors())
	}
	// A real literal still cannot start with a dot.
	p = New(lexer.New("x := .5;"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Error("expected an error for .5")
	}
}

// SR, RS, S and R name the standard bistables and their inputs, so they can
// name function blocks and variables, while S and R stay IL operators.
func TestStandardBistableNames(t *testing.T) {
	for src, want := range map[string]string{
		"FUNCTION_BLOCK SR VAR_INPUT S1 : BOOL; R : BOOL; END_VAR VAR_OUTPUT Q1 : BOOL; END_VAR Q1 := S1 OR (NOT R AND Q1); END_FUNCTION_BLOCK": "SR",
		"FUNCTION_BLOCK RS VAR_INPUT S : BOOL; R1 : BOOL; END_VAR END_FUNCTION_BLOCK":                                                           "RS",
		"VAR s : SR; END_VAR s(S1 := TRUE, R := FALSE); x := s.Q1;":                                                                             "",
		"VAR s : RS; END_VAR s(S := TRUE, R1 := TRUE); s.Q1;":                                                                                   "",
		"VAR r : ARRAY[0..1] OF BOOL; END_VAR r[1] := TRUE;":                                                                                    "",
	} {
		p := New(lexer.New(src))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Errorf("%s: %v", src, p.Errors())
			continue
		}
		if want == "" {
			continue
		}
		fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
		if !ok || fb.Name.Value != want {
			t.Errorf("%s: got %#v", src, program.Statements[0])
		}
	}
	// In IL, S and R still set and reset.
	p := New(lexer.New("PROGRAM P VAR a, b : BOOL; END_VAR LD a S b R a END_PROGRAM"))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("IL S and R: %v", p.Errors())
	}
}

// A reserved keyword declared as a variable name is one error, at the
// declaration, not one per use (issue #4). Keywords that read as variables
// where they are used, like CTD's input LD, still name variables.
func TestReservedKeywordAsVariableName(t *testing.T) {
	input := `PROGRAM P
VAR
    at : BOOL;
    x, ld, step : BOOL;
END_VAR
x := at;
IF at AND x THEN at := ld OR step; END_IF;
END_PROGRAM`
	p := New(lexer.New(input))
	p.ParseProgram()
	want := "AT is a reserved keyword and cannot be used as a variable name at row 3, column 5"
	if errs := p.Errors(); len(errs) != 1 || errs[0] != want {
		t.Errorf("errors = %q, want [%q]", errs, want)
	}

	p = New(lexer.New("PROGRAM P\nVAR\n    x, then : BOOL;\nEND_VAR\nEND_PROGRAM"))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) == 0 || !strings.HasPrefix(errs[0], "THEN is a reserved keyword and cannot be used as a variable name at row 3, column 8") {
		t.Errorf("second name: errors = %q", errs)
	}
}

// Keywords of one position only (task settings, access and OOP specifiers,
// EN/ENO, IL operators) name variables, declared and used, as they did
// before reserved keywords were reported.
func TestPositionKeywordsAsVariableNames(t *testing.T) {
	input := `FUNCTION_BLOCK Fb
VAR internal, interval, priority, single, task, resource, method, property : INT;
    read_only, read_write, en, eno : BOOL; cal, jmp, ret : INT; END_VAR
internal := interval + priority + single + task + resource + method + property;
en := read_only AND read_write OR eno;
cal := jmp + ret;
END_FUNCTION_BLOCK`
	p := New(lexer.New(input))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Errorf("parser errors: %q", errs)
	}
}
