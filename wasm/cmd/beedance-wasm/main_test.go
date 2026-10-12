//go:build !js

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const suite = `
PROGRAM TEST_Add
VAR_OUTPUT failures : INT; END_VAR
IF 2 + 3 <> 5 THEN failures := 1; END_IF;
END_PROGRAM
`

func TestServe(t *testing.T) {
	in := strings.Join([]string{
		`{"op":"version"}`,
		``,
		`{"op":"load","source":"PROGRAM P VAR n : INT; END_VAR n := n + 1; END_PROGRAM"}`,
		`{"op":"scan","session":1,"count":3}`,
		`nonsense`,
	}, "\n")
	var out bytes.Buffer
	if err := serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("%d replies, want 4 (blank lines are skipped):\n%s", len(lines), out.String())
	}
	var scan struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &scan); err != nil || scan.Values["n"] != 3.0 {
		t.Errorf("scan reply %s (%v)", lines[2], err)
	}
	if !strings.Contains(lines[3], `"error"`) {
		t.Errorf("reply to nonsense: %s", lines[3])
	}
}

func TestTestCommand(t *testing.T) {
	for _, c := range []struct {
		source string
		args   []string
		code   int
		want   string
	}{
		{suite, nil, 0, "PASSED: all 1 tests"},
		{suite, []string{"-engines", "vm"}, 0, "vm:pass"},
		{strings.Replace(suite, "<> 5", "= 5", 1), nil, 1, "FAILED: 1 of 1 tests"},
		{"PROGRAM P VAR x : INT END_VAR", nil, 2, "parse errors"},
		{suite, []string{"-engines", "cobol"}, 2, "unknown engine"},
	} {
		var out, errOut bytes.Buffer
		code := test(c.args, strings.NewReader(c.source), &out, &errOut)
		if code != c.code || !strings.Contains(out.String()+errOut.String(), c.want) {
			t.Errorf("test %v: exit %d, want %d with %q\n%s%s", c.args, code, c.code, c.want, out.String(), errOut.String())
		}
	}
}
