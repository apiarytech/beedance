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

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// The OSCAT BUILDING and NETWORK libraries, cleaned for beedance by beebread's
// tools/stclean, compile with OSCAT BASIC, in CODESYS's dialect
// (AllowInputWrites). They are copies of beebread's building/ and network/
// sources.
const (
	buildingPath = "../reference/beedance_building_100.st"
	networkPath  = "../reference/beedance_network_135.st"
)

// knownMissing are the declarations that cannot compile yet, and why.
var knownMissing = map[string]string{
	// TwinCAT's TCP/IP blocks are not ST; the host provides them.
	"FUNCTION_BLOCK IP_CONTROL_RESET": "FB_SOCKETCLOSEALL",
}

func TestOscatBuildingCompiles(t *testing.T) { compileWithBasic(t, buildingPath) }
func TestOscatNetworkCompiles(t *testing.T)  { compileWithBasic(t, networkPath) }

// compileWithBasic compiles a library with OSCAT BASIC as one program,
// leaving out what fails, and fails for each declaration of the library left
// out that knownMissing does not explain.
func compileWithBasic(t *testing.T, path string) {
	object.FinalizeBuiltins()
	basic := oscatDeclarations(t)
	lib := oscatDeclarationsAt(t, path)
	own := map[ast.Statement]bool{}
	for _, s := range lib {
		own[s] = true
	}
	stmts := append(append([]ast.Statement{}, basic...), lib...)
	failed := map[string]string{}
	for {
		c := New()
		c.AllowInputWrites = true
		var bad ast.Statement
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			program := &ast.Program{Statements: stmts}
			c.buildPouInfo(program)
			c.rootProgram = program
			all := c.withStandardFBs(program)
			c.predefineGlobals(all)
			c.predefineFunctionBlocks(all)
			for _, s := range c.orderByInheritance(all) {
				bad = s
				if err := c.Compile(s); err != nil {
					return err
				}
			}
			return nil
		}()
		if err == nil {
			break
		}
		name := declarationName(bad)
		if !own[bad] {
			name = "BASIC " + name
		}
		failed[name] = err.Error()
		kept := stmts[:0:0]
		for _, s := range stmts {
			if s != bad {
				kept = append(kept, s)
			}
		}
		if len(kept) == len(stmts) {
			t.Fatalf("%s: %v", name, err)
		}
		stmts = kept
	}
	names := make([]string, 0, len(failed))
	for n := range failed {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if why, ok := knownMissing[n]; ok && strings.Contains(failed[n], why) {
			t.Logf("left out %s: %s (known)", n, failed[n])
			continue
		}
		t.Errorf("%s does not compile: %s", n, failed[n])
	}
	t.Logf("%d of %d declarations of %s compile with OSCAT BASIC", len(lib)-len(failed), len(lib), path)
}

// TestCODESYSDialect checks what CODESYS allows and IEC 61131-3 does not:
// writing a POU's own inputs (with AllowInputWrites), arithmetic on bit
// strings, array bounds named by constants, SIZEOF, a DATE moved by a TIME,
// a TIME scaled by a bit string and an integer compared with a bit string.
func TestCODESYSDialect(t *testing.T) {
	inputs := `FUNCTION F : INT VAR_INPUT x : INT; END_VAR x := x * 2; F := x; END_FUNCTION
FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR i := i + 1; o := i; END_FUNCTION_BLOCK`
	if err := New().Compile(parse(t, inputs)); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("IEC 61131-3: writing an input compiled: %v", err)
	}
	c := New()
	c.AllowInputWrites = true
	if err := c.Compile(parse(t, inputs)); err != nil {
		t.Errorf("CODESYS: writing an input: %v", err)
	}

	for _, src := range []string{
		"VAR a : DWORD; b : DWORD; c : DWORD; END_VAR c := a - b;",
		"VAR x : BYTE; END_VAR x := x + 10; x := 160 + x;",
		"VAR x : BYTE; u : UDINT; n : LINT; END_VAR n := x * u;",
		"VAR_GLOBAL CONSTANT N : INT := 4; M : INT := N * 2; END_VAR TYPE Rec : STRUCT a : ARRAY[0..N] OF INT; END_STRUCT; END_TYPE VAR rc : Rec; b : ARRAY[1..M - 1] OF BYTE; END_VAR",
		"VAR b : ARRAY[1..64] OF BYTE; n : INT; END_VAR n := SIZEOF(b);",
		"VAR d : DATE; END_VAR d := d + T#1d;",
		"VAR t : TIME; b : BYTE; END_VAR t := t * b;",
		"VAR u : UDINT; w : DWORD; q : BOOL; END_VAR q := u < w;",
	} {
		if err := New().Compile(parse(t, src)); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	if err := New().Compile(parse(t, "VAR b : ARRAY[1..N] OF BYTE; END_VAR")); err == nil {
		t.Error("an array bounded by an undeclared name compiled")
	}
}
