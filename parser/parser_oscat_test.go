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
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"beedance/lexer"
	"beedance/token"
)

// oscatPath is the OSCAT BASIC 3.35 library converted to the IEC 61131-3
// beedance accepts, kept out of the test sources; the tests that read it are
// skipped without it. The units that use POINTER TO or ADR, and those that
// call them, are commented out in it.
const oscatPath = "../reference/beedance_oscat_basic.st"

// oscatUnits splits the OSCAT library into its POUs, TYPE blocks and global
// variable blocks, each a separate source.
func oscatUnits(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(oscatPath)
	if err != nil {
		t.Skipf("OSCAT library not present: %v", err)
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	// Units are found from the lexer's tokens, so that code quoted in a
	// comment does not start one.
	ends := map[token.TokenType]token.TokenType{
		token.FUNCTION_BLOCK: token.END_FUNCTION_BLOCK, token.FUNCTION: token.END_FUNCTION,
		token.PROGRAM: token.END_PROGRAM, token.TYPE: token.END_TYPE, token.VAR_GLOBAL: token.END_VAR,
	}
	units := []string{}
	start, end := -1, token.TokenType("")
	l := lexer.New(src)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		switch {
		case start < 0:
			if e, ok := ends[tok.Type]; ok {
				start, end = tok.Pos, e
			}
		case tok.Type == end:
			units = append(units, src[start:tok.Pos+len(tok.Literal)])
			start = -1
		}
	}
	return units
}

// TestOscatLibraryParses parses each unit of the OSCAT library and reports
// the causes of the failures, most common first.
func TestOscatLibraryParses(t *testing.T) {
	units := oscatUnits(t)
	where := regexp.MustCompile(` at row \d+, column \d+`)
	quoted := regexp.MustCompile(`"[^"]*"`)
	causes := map[string][]string{}
	parsed := 0
	for _, unit := range units {
		p := New(lexer.New(unit))
		p.ParseProgram()
		if len(p.Errors()) == 0 {
			parsed++
			continue
		}
		cause := quoted.ReplaceAllString(where.ReplaceAllString(p.Errors()[0], ""), `"…"`)
		causes[cause] = append(causes[cause], strings.TrimSpace(strings.SplitN(unit, "\n", 2)[0])+": "+p.Errors()[0])
	}
	t.Logf("%d of %d units parse", parsed, len(units))
	keys := make([]string, 0, len(causes))
	for k := range causes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(causes[keys[i]]) > len(causes[keys[j]]) })
	for _, k := range keys {
		t.Logf("%4d  %s\n        e.g. %s", len(causes[k]), k, causes[k][0])
	}
	if parsed != len(units) {
		t.Errorf("only %d of %d units parse", parsed, len(units))
	}
}
