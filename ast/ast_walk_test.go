/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
)

// corpus covers every kind of declaration, statement and expression, so the
// walk below reaches every node type the parser produces.
var corpus = []string{
	`TYPE
		Color : (Red, Green) := Green;
		Pt : STRUCT px : INT := 1; py : REAL; END_STRUCT;
		Small : INT(1..10);
		Name : STRING(1);
		Row : ARRAY[1..3] OF INT := [1, 2(1)];
		PtRef : REFERENCE TO Pt;
	END_TYPE`,
	`VAR_GLOBAL CONSTANT g : INT := 1; END_VAR`,
	`FUNCTION F : INT
		VAR_INPUT a : INT := 1; END_VAR
		VAR_OUTPUT o : INT := 1; END_VAR
		VAR_IN_OUT io : INT; END_VAR
		VAR v : INT := 1; END_VAR
		IF a > 1 THEN F := 1; ELSIF a < 1 THEN F := -1; ELSE F := NOT 1; END_IF
		CASE a OF 1: F := 1; 1..3, 4: F := 2; ELSE F := 1; END_CASE
		FOR v := 1 TO 1 BY 1 DO EXIT; END_FOR
		WHILE a = 1 DO a := a + 1; END_WHILE
		REPEAT a := a - 1; UNTIL a <= 1 END_REPEAT
		RETURN;
	END_FUNCTION`,
	`INTERFACE IBase METHOD Base : INT; END_INTERFACE
	INTERFACE IRun EXTENDS IBase
		METHOD Run : BOOL VAR_INPUT speed : INT := 1; END_VAR END_METHOD
		PROPERTY Level : INT GET SET;
	END_INTERFACE`,
	`FUNCTION_BLOCK ABSTRACT Base
		VAR PROTECTED level : INT := 1; END_VAR
		METHOD ABSTRACT Run : BOOL END_METHOD
	END_FUNCTION_BLOCK
	FUNCTION_BLOCK FINAL Motor EXTENDS Base IMPLEMENTS IRun
		VAR_INPUT speed : INT := 1; END_VAR
		VAR_OUTPUT ok : BOOL; END_VAR
		VAR_IN_OUT shared : INT; END_VAR
		VAR ref : REFERENCE TO INT; pt : Pt := (px := 1); END_VAR
		VAR_TEMP t1 : INT := 1; END_VAR
		VAR_EXTERNAL g : INT; END_VAR
		METHOD PUBLIC Run : BOOL VAR_INPUT n : INT := 1; END_VAR VAR x : INT := 1; END_VAR
			Run := SUPER^.Run() OR THIS^.level > 1; ref^ := 1; x := pt.px;
		END_METHOD
		METHOD Base : INT Base := 1; END_METHOD
		PROPERTY Level : INT
			PUBLIC GET Level := level + 1; END_GET
			PRIVATE SET level := value + 1; END_SET
		END_PROPERTY
		speed := speed + 1;
	END_FUNCTION_BLOCK`,
	`NAMESPACE Lib
		FUNCTION G : INT G := 1; END_FUNCTION
	END_NAMESPACE`,
	`PROGRAM Main
		VAR_INPUT i : INT := 1; END_VAR
		VAR_OUTPUT q : INT := 1; END_VAR
		VAR m : Motor; arr : ARRAY[0..1] OF INT := [1, 1]; x AT %QX0.1 : BOOL; s : STRING := 'one'; END_VAR
		VAR_ACCESS acc : Main.i : INT READ_ONLY; END_VAR
		m(speed := 1, ok => x);
		arr[1] := F(a := 1, o => q, io := i);
		q := Lib.G() + INT#1 + {1: 1}[1] + [1, 2][1];
		s := CONCAT(s, 'x');
	END_PROGRAM`,
	`PROGRAM Seq
		INITIAL_STEP Start: Act(N); END_STEP
		STEP Run: Act(S, T#1s); END_STEP
		TRANSITION FROM Start TO Run := 1 = 1; END_TRANSITION
		TRANSITION FROM Run TO Start := TRUE; END_TRANSITION
		ACTION Act: ; END_ACTION
	END_PROGRAM`,
	`PROGRAM IlProg
		VAR a : INT; END_VAR
		start: LD 1
		ADD( LD 1 )
		JMPC start
		ST a
	END_PROGRAM`,
	`CONFIGURATION Cell
		VAR_GLOBAL cg : INT := 1; END_VAR
		RESOURCE Station ON CPU
			VAR_GLOBAL rg : INT := 1; END_VAR
			TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
			TASK Slow(SINGLE := cg, PRIORITY := 1);
			PROGRAM P1 WITH Fast : Main (i := 1, q => cg, m WITH Slow);
		END_RESOURCE
		VAR_ACCESS ca : Station.P1.q : INT READ_ONLY; END_VAR
		VAR_CONFIG Station.P1.i : INT := 1; END_VAR
	END_CONFIGURATION`,
	`VAR mac : MACRO(p) { EXPR(EVAL(p) * 2); }; END_VAR`,
	`D#2026-01-02; TOD#12:00:00; DT#2026-01-02-12:00:00; 16#FF; BYTE#16#1; 1.5; "w";`,
}

// parseCorpus parses one corpus entry, failing the test on parse errors.
func parseCorpus(t *testing.T, input string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors in %q:\n%s", input, strings.Join(p.Errors(), "\n"))
	}
	return program
}

// walk calls visit for every AST node reachable from v through exported
// fields, slices, maps and interfaces.
func walk(v reflect.Value, visit func(ast.Node)) {
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			walk(v.Elem(), visit)
		}
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if node, ok := v.Interface().(ast.Node); ok {
			visit(node)
		}
		walk(v.Elem(), visit)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				walk(v.Field(i), visit)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walk(v.Index(i), visit)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			walk(key, visit)
			walk(v.MapIndex(key), visit)
		}
	}
}

// TestEveryNodeDescribesItself visits every node of the corpus and checks
// that it reports a token, a position and a printed form without panicking.
func TestEveryNodeDescribesItself(t *testing.T) {
	seen := map[string]bool{}
	for _, input := range corpus {
		walk(reflect.ValueOf(parseCorpus(t, input)), func(n ast.Node) {
			typeName := fmt.Sprintf("%T", n)
			seen[typeName] = true
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s panicked: %v", typeName, r)
					}
				}()
				n.TokenLiteral()
				if positioned, ok := n.(interface{ Pos() (int, int) }); ok {
					if row, _ := positioned.Pos(); row < 0 {
						t.Errorf("%s has a negative row", typeName)
					}
				}
				if commented, ok := n.(interface{ GetLeadingComments() []string }); ok {
					commented.GetLeadingComments()
				}
				_ = n.String()
			}()
		})
	}
	// The corpus must keep reaching the node types it was written for.
	for _, want := range []string{
		"*ast.InterfaceDeclaration", "*ast.MethodDeclaration", "*ast.MethodImplementation",
		"*ast.PropertyDeclaration", "*ast.PropertyGetter", "*ast.PropertySetter",
		"*ast.ThisExpression", "*ast.SuperExpression", "*ast.DereferenceExpression",
		"*ast.StructLiteral", "*ast.ReferenceType", "*ast.ResourceDeclaration",
		"*ast.FbTaskAssociation", "*ast.NamespaceDeclaration",
	} {
		if !seen[want] {
			t.Errorf("the corpus no longer produces %s", want)
		}
	}
}

// TestModifyReachesEveryExpression rewrites every integer literal 1 to 2 and
// checks that no 1 is left anywhere in the tree, so a macro expansion (which
// uses Modify) reaches expressions in every kind of node.
func TestModifyReachesEveryExpression(t *testing.T) {
	oneToTwo := func(n ast.Node) ast.Node {
		if lit, ok := n.(*ast.IntegerLiteral); ok && lit.Value == 1 {
			return &ast.IntegerLiteral{Token: lit.Token, Value: 2}
		}
		return n
	}
	for _, input := range corpus {
		program := parseCorpus(t, input)
		modified := ast.Modify(program, oneToTwo)

		var parents []string
		var find func(v reflect.Value, parent string)
		find = func(v reflect.Value, parent string) {
			switch v.Kind() {
			case reflect.Interface:
				if !v.IsNil() {
					find(v.Elem(), parent)
				}
			case reflect.Pointer:
				if v.IsNil() {
					return
				}
				if lit, ok := v.Interface().(*ast.IntegerLiteral); ok && lit.Value == 1 {
					parents = append(parents, parent)
					return
				}
				if _, ok := v.Interface().(ast.Node); ok {
					parent = v.Type().String()
				}
				find(v.Elem(), parent)
			case reflect.Struct:
				for i := 0; i < v.NumField(); i++ {
					if v.Type().Field(i).IsExported() {
						find(v.Field(i), parent+"."+v.Type().Field(i).Name)
					}
				}
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					find(v.Index(i), parent)
				}
			case reflect.Map:
				for _, key := range v.MapKeys() {
					find(key, parent)
					find(v.MapIndex(key), parent)
				}
			}
		}
		find(reflect.ValueOf(modified), "")
		if len(parents) > 0 {
			t.Errorf("Modify left literal 1 in:\n  %s\nsource: %.80q", strings.Join(parents, "\n  "), input)
		}
	}
}

// Nodes that once panicked or printed badly in String.
func TestStringOfPrototypesAndConfigEntries(t *testing.T) {
	tests := []struct{ input, want string }{
		// Interface property prototypes have no accessor bodies.
		{"INTERFACE I PROPERTY L : INT GET SET; END_INTERFACE", "INTERFACE I\n\tPROPERTY L : INT\n\tGET\n\tSET\nEND_PROPERTY\nEND_INTERFACE"},
		// A VAR_CONFIG entry names its variable by access path.
		{"PROGRAM Prg VAR_CONFIG P1.x : INT := 1; END_VAR END_PROGRAM", "PROGRAM Prg\nVAR_CONFIG\n\tP1.x : INT := 1;\nEND_VAR\nEND_PROGRAM"},
		// The single-resource form of a configuration has no RESOURCE wrapper.
		{"CONFIGURATION C TASK T1(PRIORITY := 1); PROGRAM P1 WITH T1 : Prg; PROGRAM P2 : Prg; END_CONFIGURATION", "CONFIGURATION C\nTASK T1(PRIORITY := 1)\nPROGRAM P1 WITH T1 : Prg;\nPROGRAM P2 : Prg;\nEND_CONFIGURATION"},
	}
	for _, tt := range tests {
		if got := parseCorpus(t, tt.input).String(); got != tt.want {
			t.Errorf("%s:\n got  %q\n want %q", tt.input, got, tt.want)
		}
	}
}

func TestVarDeclStatementQualifiers(t *testing.T) {
	fb := parseCorpus(t, "FUNCTION_BLOCK Fb VAR FINAL k : INT := 1; END_VAR VAR_INPUT R_EDGE re : BOOL; END_VAR VAR_INPUT F_EDGE fe : BOOL; END_VAR VAR s : STRING(8); n : INT(1..5); END_VAR END_FUNCTION_BLOCK").
		Statements[0].(*ast.FunctionBlockDeclaration)
	decls := append(append([]*ast.VarDeclStatement{}, fb.VarInputs...), fb.Vars...)
	got := []string{}
	for _, d := range decls {
		got = append(got, d.String())
	}
	joined := strings.Join(got, " | ")
	for _, want := range []string{"FINAL k : INT := 1", "re : R_EDGE BOOL", "fe : F_EDGE BOOL", "s : STRING(8)", "n : INT (1 .. 5)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("declarations print as %q; missing %q", joined, want)
		}
	}
}

func TestHasMethod(t *testing.T) {
	fb := parseCorpus(t, "FUNCTION_BLOCK Fb METHOD Start : BOOL END_METHOD PROPERTY Speed : INT GET Speed := 1; END_GET SET END_SET END_PROPERTY END_FUNCTION_BLOCK").
		Statements[0].(*ast.FunctionBlockDeclaration)
	// A property counts under its own name, which SUPER calls look up.
	for name, want := range map[string]bool{"Start": true, "Speed": true, "GetSpeed": false, "Stop": false} {
		if got := fb.HasMethod(name); got != want {
			t.Errorf("HasMethod(%q) = %v, want %v", name, got, want)
		}
	}
}
