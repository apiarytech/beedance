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

// This file gives compiled programs the IEC 61131-3 standard function
// blocks: the timers TON, TOF and TP, the counters CTU, CTD and CTUD, the edge
// triggers R_TRIG and F_TRIG, and the bistables SR and RS. They are written
// in Structured Text and compiled like any user FB, only when a program uses
// them and does not declare an FB of the same name. They behave as the
// evaluator's standard FBs do. The timers read the clock builtin __CLOCK,
// whose name no IEC identifier can take.

import (
	"strings"
	"sync"

	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
)

const standardFBSource = `
FUNCTION_BLOCK TON
VAR_INPUT IN : BOOL; PT : TIME; END_VAR
VAR_OUTPUT Q : BOOL; ET : TIME; END_VAR
VAR running : BOOL; start : TIME; END_VAR
IF IN THEN
	IF NOT running THEN
		running := TRUE;
		start := __CLOCK();
	END_IF;
	ET := __CLOCK() - start;
	IF ET >= PT THEN
		ET := PT;
		Q := TRUE;
	ELSE
		Q := FALSE;
	END_IF;
ELSE
	running := FALSE;
	ET := T#0s;
	Q := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TOF
VAR_INPUT IN : BOOL; PT : TIME; END_VAR
VAR_OUTPUT Q : BOOL; ET : TIME; END_VAR
VAR running : BOOL; start : TIME; END_VAR
IF IN THEN
	running := FALSE;
	ET := T#0s;
	Q := TRUE;
ELSIF Q THEN
	IF NOT running THEN
		running := TRUE;
		start := __CLOCK();
	END_IF;
	ET := __CLOCK() - start;
	IF ET >= PT THEN
		ET := PT;
		Q := FALSE;
		running := FALSE;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TP
VAR_INPUT IN : BOOL; PT : TIME; END_VAR
VAR_OUTPUT Q : BOOL; ET : TIME; END_VAR
VAR running : BOOL; start : TIME; lastIN : BOOL; END_VAR
IF IN AND NOT lastIN AND NOT running THEN
	running := TRUE;
	start := __CLOCK();
END_IF;
IF running THEN
	ET := __CLOCK() - start;
	IF ET >= PT THEN
		ET := PT;
		running := FALSE;
	END_IF;
END_IF;
IF NOT running AND NOT IN THEN
	ET := T#0s;
END_IF;
Q := running;
lastIN := IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTU
VAR_INPUT CU : BOOL; R : BOOL; PV : INT; END_VAR
VAR_OUTPUT Q : BOOL; CV : INT; END_VAR
VAR lastCU : BOOL; END_VAR
IF R THEN
	CV := 0;
ELSIF CU AND NOT lastCU AND CV < PV THEN
	CV := CV + 1;
END_IF;
Q := CV >= PV;
lastCU := CU;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTD
VAR_INPUT CD : BOOL; LD : BOOL; PV : INT; END_VAR
VAR_OUTPUT Q : BOOL; CV : INT; END_VAR
VAR lastCD : BOOL; END_VAR
IF LD THEN
	CV := PV;
ELSIF CD AND NOT lastCD AND CV > 0 THEN
	CV := CV - 1;
END_IF;
Q := CV <= 0;
lastCD := CD;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTUD
VAR_INPUT CU : BOOL; CD : BOOL; R : BOOL; LD : BOOL; PV : INT; END_VAR
VAR_OUTPUT QU : BOOL; QD : BOOL; CV : INT; END_VAR
VAR lastCU : BOOL; lastCD : BOOL; END_VAR
IF R THEN
	CV := 0;
ELSIF LD THEN
	CV := PV;
ELSIF CU AND NOT lastCU AND NOT (CD AND NOT lastCD) THEN
	IF CV < PV THEN
		CV := CV + 1;
	END_IF;
ELSIF CD AND NOT lastCD AND NOT (CU AND NOT lastCU) THEN
	IF CV > 0 THEN
		CV := CV - 1;
	END_IF;
END_IF;
QU := CV >= PV;
QD := CV <= 0;
lastCU := CU;
lastCD := CD;
END_FUNCTION_BLOCK

FUNCTION_BLOCK R_TRIG
VAR_INPUT CLK : BOOL; END_VAR
VAR_OUTPUT Q : BOOL; END_VAR
VAR M : BOOL; END_VAR
Q := CLK AND NOT M;
M := CLK;
END_FUNCTION_BLOCK

FUNCTION_BLOCK F_TRIG
VAR_INPUT CLK : BOOL; END_VAR
VAR_OUTPUT Q : BOOL; END_VAR
VAR M : BOOL; END_VAR
Q := NOT CLK AND M;
M := CLK;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SR
VAR_INPUT S1 : BOOL; R : BOOL; END_VAR
VAR_OUTPUT Q1 : BOOL; END_VAR
Q1 := S1 OR (NOT R AND Q1);
END_FUNCTION_BLOCK

FUNCTION_BLOCK RS
VAR_INPUT S : BOOL; R1 : BOOL; END_VAR
VAR_OUTPUT Q1 : BOOL; END_VAR
Q1 := NOT R1 AND (S OR Q1);
END_FUNCTION_BLOCK
`

var (
	standardFBsOnce sync.Once
	standardFBDecls map[string]*ast.FunctionBlockDeclaration
)

// standardFBs returns the standard function blocks, by upper-case name.
func standardFBs() map[string]*ast.FunctionBlockDeclaration {
	standardFBsOnce.Do(func() {
		p := parser.New(lexer.New(standardFBSource))
		program := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			panic("standard function blocks: " + strings.Join(errs, "; "))
		}
		standardFBDecls = make(map[string]*ast.FunctionBlockDeclaration)
		for _, s := range program.Statements {
			if fb, ok := s.(*ast.FunctionBlockDeclaration); ok {
				standardFBDecls[strings.ToUpper(fb.Name.Value)] = fb
			}
		}
	})
	return standardFBDecls
}

// withStandardFBs returns a program's statements preceded by the standard
// function blocks it uses and does not declare itself. Each program gets its
// own copy of their declarations, parsed afresh, since compiling may record
// information on the AST.
func (c *Compiler) withStandardFBs(program *ast.Program) []ast.Statement {
	standard := standardFBs()
	used := map[string]bool{}
	ast.Modify(program, func(n ast.Node) ast.Node {
		if ident, ok := n.(*ast.Identifier); ok {
			if name := strings.ToUpper(ident.Value); standard[name] != nil {
				used[name] = true
			}
		}
		return n
	})
	if len(used) == 0 {
		return program.Statements
	}
	for name := range used {
		if _, declared := c.typeInfo[name]; declared {
			delete(used, name)
		}
	}
	if len(used) == 0 {
		return program.Statements
	}
	p := parser.New(lexer.New(standardFBSource))
	fresh := p.ParseProgram()
	stmts := []ast.Statement{}
	for _, s := range fresh.Statements {
		if fb, ok := s.(*ast.FunctionBlockDeclaration); ok && used[strings.ToUpper(fb.Name.Value)] {
			c.typeInfo[strings.ToUpper(fb.Name.Value)] = fb
			stmts = append(stmts, fb)
		}
	}
	return append(stmts, program.Statements...)
}
