/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package typedfbs gives a program the typed standard function blocks of
// IEC 61131-3 (3rd edition, Table 44 and 45): the counters CTU_INT,
// CTU_DINT, CTU_LINT, CTU_UDINT and CTU_ULINT, with CTD_ and CTUD_ of the
// same types, and the timers TON_LTIME, TOF_LTIME and TP_LTIME. Each is the
// untyped block (CTU, TON) of that type, in Structured Text, which the
// program gets if it uses the block and does not declare it, for every
// engine alike.
package typedfbs

import (
	"sort"
	"strings"
	"sync"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// counterTypes are the types of the typed counters.
var counterTypes = []string{"INT", "DINT", "LINT", "UDINT", "ULINT"}

const counterSource = `
FUNCTION_BLOCK CTU_$T
VAR_INPUT CU : BOOL; R : BOOL; PV : $T; END_VAR
VAR_OUTPUT Q : BOOL; CV : $T; END_VAR
VAR lastCU : BOOL; END_VAR
IF R THEN
	CV := 0;
ELSIF CU AND NOT lastCU AND CV < PV THEN
	CV := CV + 1;
END_IF;
Q := CV >= PV;
lastCU := CU;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTD_$T
VAR_INPUT CD : BOOL; LD : BOOL; PV : $T; END_VAR
VAR_OUTPUT Q : BOOL; CV : $T; END_VAR
VAR lastCD : BOOL; END_VAR
IF LD THEN
	CV := PV;
ELSIF CD AND NOT lastCD AND CV > 0 THEN
	CV := CV - 1;
END_IF;
Q := CV <= 0;
lastCD := CD;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTUD_$T
VAR_INPUT CU : BOOL; CD : BOOL; R : BOOL; LD : BOOL; PV : $T; END_VAR
VAR_OUTPUT QU : BOOL; QD : BOOL; CV : $T; END_VAR
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
`

// timerSource is the timers of LTIME, which the standard ones (the
// compiler's) are of TIME. __CLOCK is the scan's clock.
const timerSource = `
FUNCTION_BLOCK TON_LTIME
VAR_INPUT IN : BOOL; PT : LTIME; END_VAR
VAR_OUTPUT Q : BOOL; ET : LTIME; END_VAR
VAR running : BOOL; start : LTIME; END_VAR
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
	ET := LT#0s;
	Q := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TOF_LTIME
VAR_INPUT IN : BOOL; PT : LTIME; END_VAR
VAR_OUTPUT Q : BOOL; ET : LTIME; END_VAR
VAR running : BOOL; start : LTIME; END_VAR
IF IN THEN
	running := FALSE;
	ET := LT#0s;
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

FUNCTION_BLOCK TP_LTIME
VAR_INPUT IN : BOOL; PT : LTIME; END_VAR
VAR_OUTPUT Q : BOOL; ET : LTIME; END_VAR
VAR running : BOOL; start : LTIME; lastIN : BOOL; END_VAR
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
	ET := LT#0s;
END_IF;
Q := running;
lastIN := IN;
END_FUNCTION_BLOCK
`

var (
	sourceOnce sync.Once
	sources    map[string]string // each block's source, by upper-case name
)

// blockSources returns the source of each typed block, by name.
func blockSources() map[string]string {
	sourceOnce.Do(func() {
		sources = map[string]string{}
		split := func(src string) {
			for _, part := range strings.SplitAfter(src, "END_FUNCTION_BLOCK") {
				i := strings.Index(part, "FUNCTION_BLOCK ")
				if i < 0 {
					continue
				}
				name := strings.Fields(part[i+len("FUNCTION_BLOCK "):])[0]
				sources[name] = part[i:]
			}
		}
		for _, t := range counterTypes {
			split(strings.ReplaceAll(counterSource, "$T", t))
		}
		split(timerSource)
	})
	return sources
}

// IsTyped reports whether name (any case) is one of the typed blocks.
func IsTyped(name string) bool {
	_, ok := blockSources()[strings.ToUpper(name)]
	return ok
}

// Declare puts before the program's statements the typed blocks it names
// and does not declare, in place. A program it declared them for has them,
// so declaring again adds nothing.
func Declare(program *ast.Program) {
	if program == nil {
		return
	}
	all := blockSources()
	declared := map[string]bool{}
	for _, s := range program.Statements {
		if fb, ok := s.(*ast.FunctionBlockDeclaration); ok && fb.Name != nil {
			declared[strings.ToUpper(fb.Name.Value)] = true
		}
	}
	used := map[string]bool{}
	ast.Modify(program, func(n ast.Node) ast.Node {
		if ident, ok := n.(*ast.Identifier); ok {
			if name := strings.ToUpper(ident.Value); all[name] != "" && !declared[name] {
				used[name] = true
			}
		}
		return n
	})
	if len(used) == 0 {
		return
	}
	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	sort.Strings(names)
	var src strings.Builder
	for _, name := range names {
		src.WriteString(all[name])
		src.WriteString("\n")
	}
	p := parser.New(lexer.New(src.String()))
	blocks := p.ParseProgram()
	if len(p.Errors()) > 0 {
		panic("typed function blocks: " + strings.Join(p.Errors(), "; "))
	}
	program.Statements = append(blocks.Statements, program.Statements...)
}
