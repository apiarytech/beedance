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
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib" // The standard functions OSCAT calls.
	"github.com/apiarytech/beedance/token"
)

// oscatPath is the OSCAT BASIC 3.35 library converted to the IEC 61131-3
// beedance accepts, kept out of the test sources; the test that reads it is
// skipped without it.
const oscatPath = "../reference/beedance_oscat_basic.st"

// oscatDeclarations returns the declarations of the OSCAT library units that
// parse: its POUs, TYPE blocks and global variable blocks.
func oscatDeclarations(t *testing.T) []ast.Statement { return oscatDeclarationsAt(t, oscatPath) }

// oscatDeclarationsAt reads the declarations of an OSCAT library source,
// each parsed on its own so that one that does not parse is left out.
func oscatDeclarationsAt(t *testing.T, path string) []ast.Statement {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("OSCAT library not present: %v", err)
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	ends := map[token.TokenType]token.TokenType{
		token.FUNCTION_BLOCK: token.END_FUNCTION_BLOCK, token.FUNCTION: token.END_FUNCTION,
		token.PROGRAM: token.END_PROGRAM, token.TYPE: token.END_TYPE, token.VAR_GLOBAL: token.END_VAR,
	}
	stmts := []ast.Statement{}
	start, end := -1, token.TokenType("")
	l := lexer.New(src)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		switch {
		case start < 0:
			if e, ok := ends[tok.Type]; ok {
				start, end = tok.Pos, e
			}
		case tok.Type == end:
			p := parser.New(lexer.New(src[start : tok.Pos+len(tok.Literal)]))
			start = -1
			program := p.ParseProgram()
			if len(p.Errors()) == 0 {
				stmts = append(stmts, program.Statements...)
			}
		}
	}
	return stmts
}

// declarationName names a declaration in the test's report.
func declarationName(s ast.Statement) string {
	switch d := s.(type) {
	case *ast.FunctionDeclaration:
		return "FUNCTION " + d.Name.Value
	case *ast.FunctionBlockDeclaration:
		return "FUNCTION_BLOCK " + d.Name.Value
	case *ast.TypeBlockDeclaration:
		return "TYPE " + d.Declarations[0].Name.Value
	}
	return fmt.Sprintf("%T", s)
}

// TestOscatLibraryCompiles compiles the OSCAT library as one program. A
// declaration that fails is left out and the rest compiled again, until what
// is left compiles; the test reports why each declaration was left out,
// most common cause first.
func TestOscatLibraryCompiles(t *testing.T) {
	stmts := oscatDeclarations(t)
	object.FinalizeBuiltins()
	failed := map[string]string{}
	for {
		c := New()
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
		failed[declarationName(bad)] = err.Error()
		t.Logf("left out %s: %v", declarationName(bad), err)
		kept := stmts[:0:0]
		for _, s := range stmts {
			if s != bad {
				kept = append(kept, s)
			}
		}
		if len(kept) == len(stmts) {
			t.Fatalf("%s: %v", declarationName(bad), err)
		}
		stmts = kept
	}
	t.Logf("%d declarations compile together; %d are left out", len(stmts), len(failed))

	quoted := regexp.MustCompile(`'[^']*'|\b(variable|identifier:) \S+`)
	causes := map[string][]string{}
	for name, msg := range failed {
		cause := quoted.ReplaceAllString(msg, "$1 …")
		causes[cause] = append(causes[cause], name+": "+msg)
	}
	keys := make([]string, 0, len(causes))
	for k := range causes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(causes[keys[i]]) > len(causes[keys[j]]) })
	for _, k := range keys {
		t.Logf("%4d  %s\n        e.g. %s", len(causes[k]), k, causes[k][0])
	}
	if len(failed) > 0 {
		t.Errorf("%d declarations do not compile", len(failed))
	}
}
