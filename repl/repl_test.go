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
	"bytes"
	"flag"
	"github.com/apiarytech/beedance/object"
	_ "github.com/apiarytech/beedance/stdlib"
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
		"VAR sh : Shape; END_VAR",            // fails before `sh` is defined
		"VAR kept : INT := 5; END_VAR nope;", // fails after `kept` is defined, before it is set
		"kept;",                              // must not crash the session
		"1 + 1;",
	}
	results := runSession(t, "vm", lines)
	if !strings.Contains(results[1], "cannot instantiate abstract function block 'Shape'") {
		t.Fatalf("expected a compilation error for line 2, got %q", results[1])
	}
	if !strings.Contains(results[2], "undefined variable nope") {
		t.Fatalf("expected a compilation error for line 3, got %q", results[2])
	}
	if results[3] != "null" {
		t.Fatalf("expected the unassigned variable to read as null, got %q", results[3])
	}
	if results[4] != "2" {
		t.Fatalf("expected the session to continue, got %q", results[4])
	}
}

func TestReplReportsErrorsAndContinues(t *testing.T) {
	for _, engine := range []string{"vm", "eval"} {
		t.Run(engine, func(t *testing.T) {
			results := runSession(t, engine, []string{
				"VAR x : INT := ; END_VAR", // a syntax error
				"10 / 0;",                  // a runtime error
				"2 + 3;",
			})
			if !strings.Contains(results[0], "Beedance! parser errors:") {
				t.Errorf("expected parser errors for line 1, got %q", results[0])
			}
			if !strings.Contains(results[1], "division by zero") {
				t.Errorf("expected a division-by-zero error for line 2, got %q", results[1])
			}
			if results[2] != "5" {
				t.Errorf("expected the session to continue, got %q", results[2])
			}
		})
	}
	if results := runSession(t, "vm", []string{"10 / 0;"}); !strings.HasPrefix(results[0], "Woops! Executing bytecode failed:") {
		t.Errorf("expected the VM's error prefix, got %q", results[0])
	}
}

// A line that panics is reported, and the session goes on.
func TestRunLineRecoversFromPanics(t *testing.T) {
	var out bytes.Buffer
	runLine(&out, func() { panic("boom") })
	if !strings.Contains(out.String(), "Woops! Internal error while running this line:") || !strings.Contains(out.String(), "boom") {
		t.Fatalf("expected the panic to be reported, got %q", out.String())
	}
}

// The -go flag, defined by main, only works with an input file.
func TestReplWarnsAboutGoFlag(t *testing.T) {
	goFlag := flag.Lookup("go")
	if goFlag == nil {
		flag.String("go", "", "output Go file")
		goFlag = flag.Lookup("go")
	}
	if err := flag.Set("go", "out.go"); err != nil {
		t.Fatalf("setting -go: %s", err)
	}
	defer flag.Set("go", "")

	var out bytes.Buffer
	Start(strings.NewReader(""), &out, "vm")
	if !strings.Contains(out.String(), "Transpilation to Go (-go) is not supported in REPL mode.") {
		t.Fatalf("expected the -go warning, got %q", out.String())
	}
}
