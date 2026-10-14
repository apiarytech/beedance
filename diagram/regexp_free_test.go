/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"testing"
)

// The regexps that isSimpleOperand and replaceElementRefs replace.
var (
	simpleOperandRE = regexp.MustCompile(`^(?:'(?:[^']|'')*'|"(?:[^"]|"")*"|[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*|\[[^\[\]]*\])*|[+-]?[0-9][0-9A-Za-z_.#:+-]*|[A-Za-z_]+#[0-9A-Za-z_.:+-]+)$`)
	elementRefRE    = regexp.MustCompile(`element (\d+)`)
)

func TestIsSimpleOperandMatchesRegexp(t *testing.T) {
	cases := []string{
		"", "a", "_", "a1", "1a", "x.y", "x.", "x..y", "x.1", "a[1]", "a[i+1].b", "a[", "a[]", "a[[1]]", "a]",
		"'a'", "''", "'''", "''''", "'a''b'", "'a'b'", "'", `"x"`, `""`, `"a""b"`, `"a"b"`,
		"5", "-5", "+5.0", "16#FF", "1e10", "T#5s", "INT#5", "INT#", "#5", "INT#5#6", "INT#-3", "a+b", "a b",
		"x.y[3].z", "TRUE", "t#1h_2m", "2026-01-01", "D#2026-01-01", "-", "+", "é", "'é'",
	}
	r := rand.New(rand.NewSource(1))
	const alphabet = "aZ_09.#[]'\"+-: é"
	for n := 0; n < 200000; n++ {
		b := make([]byte, r.Intn(8))
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		cases = append(cases, string(b))
	}
	for _, s := range cases {
		if got, want := isSimpleOperand(s), simpleOperandRE.MatchString(s); got != want {
			t.Fatalf("isSimpleOperand(%q) = %v, the regexp says %v", s, got, want)
		}
	}
}

func TestReplaceElementRefsMatchesRegexp(t *testing.T) {
	repl := func(id int, m string) string {
		if id%2 == 0 {
			return fmt.Sprintf("line %d", id*10)
		}
		return m
	}
	for _, msg := range []string{
		"", "element 4 is not connected", "element 3 and element 12", "element x", "element element 8",
		"subelement 2", "element ", "element 0004", "elements 5", "element 99999999999999999999", "element 6element 8",
	} {
		want := elementRefRE.ReplaceAllStringFunc(msg, func(m string) string {
			id, _ := strconv.Atoi(elementRefRE.FindStringSubmatch(m)[1]) // as text.go did
			return repl(id, m)
		})
		if got := replaceElementRefs(msg, repl); got != want {
			t.Errorf("replaceElementRefs(%q) = %q, the regexp gives %q", msg, got, want)
		}
	}
}
