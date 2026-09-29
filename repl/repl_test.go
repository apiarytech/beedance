/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package repl

import (
	"beedance/object"
	_ "beedance/stdlib"
	"bytes"
	"strings"
	"testing"
)

// runSession feeds lines to the REPL and returns the output for each line.
func runSession(t *testing.T, engine string, lines []string) []string {
	t.Helper()
	object.FinalizeBuiltins()
	var out bytes.Buffer
	Start(strings.NewReader(strings.Join(lines, "\n")), &out, engine)
	// Each line's output follows a prompt; the final prompt has no output.
	parts := strings.Split(out.String(), PROMPT)
	results := []string{}
	for _, p := range parts[1 : len(parts)-1] {
		results = append(results, strings.TrimSpace(p))
	}
	if len(results) != len(lines) {
		t.Fatalf("expected output for %d lines, got %d:\n%s", len(lines), len(results), out.String())
	}
	return results
}

func TestReplStatePersistsAcrossLines(t *testing.T) {
	lines := []string{
		"VAR x : INT := 5; END_VAR",
		"x * 2;",
		"FUNCTION sq : INT VAR_INPUT a : INT; END_VAR sq := a * a; END_FUNCTION",
		"sq(7);",
		"LEN('hello');",
		"FUNCTION_BLOCK Counter VAR count : INT := 0; END_VAR METHOD Bump : INT count := count + 1; Bump := count; END_METHOD END_FUNCTION_BLOCK",
		"VAR counter : Counter; END_VAR",
		"counter.Bump();",
		"counter.Bump();",
	}
	// Results of the lines that produce a value, by line index.
	expected := map[int]string{1: "10", 3: "49", 4: "5", 7: "1", 8: "2"}

	for _, engine := range []string{"vm", "eval"} {
		t.Run(engine, func(t *testing.T) {
			results := runSession(t, engine, lines)
			for i, want := range expected {
				if results[i] != want {
					t.Fatalf("line %d %q: expected %q, got %q\nall output: %q", i, lines[i], want, results[i], results)
				}
			}
		})
	}
}

func TestReplRecoversFromFailedLines(t *testing.T) {
	lines := []string{
		"FUNCTION_BLOCK ABSTRACT Shape END_FUNCTION_BLOCK",
		"VAR sh : Shape; END_VAR", // fails: abstract FB, but `sh` is left defined
		"sh;",                     // must not crash the session
		"1 + 1;",
	}
	results := runSession(t, "vm", lines)
	if !strings.Contains(results[1], "cannot instantiate abstract function block 'Shape'") {
		t.Fatalf("expected a compilation error for line 2, got %q", results[1])
	}
	if results[2] != "null" {
		t.Fatalf("expected the unassigned variable to read as null, got %q", results[2])
	}
	if results[3] != "2" {
		t.Fatalf("expected the session to continue, got %q", results[3])
	}
}
