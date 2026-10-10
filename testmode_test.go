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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/sil"
)

func TestRunTestsExitStatus(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	lib := write("lib.st", "FUNCTION Twice : INT\nVAR_INPUT x : INT; END_VAR\nTwice := x * 2;\nEND_FUNCTION\n")
	pass := write("pass.st", "PROGRAM TEST_Twice\nVAR_OUTPUT failures : INT; END_VAR\nIF Twice(x := 21) <> 42 THEN failures := 1; END_IF;\nEND_PROGRAM\n")
	fail := write("fail.st", "PROGRAM TEST_Bad\nVAR_OUTPUT failures : INT; message : STRING; END_VAR\nfailures := 1; message := 'boom';\nEND_PROGRAM\n")
	bad := write("bad.st", "PROGRAM TEST_X VAR x : INT END_VAR END_PROGRAM\n")

	for _, tc := range []struct {
		name  string
		files []string
		code  int
		out   string
	}{
		{"pass across files", []string{lib, pass}, 0, "PASSED: all 1 tests"},
		{"failing test", []string{lib, pass, fail}, 1, "vm: 1 failed checks, last: boom"},
		{"parse error", []string{bad}, 2, "Parser errors"},
		{"no tests", []string{lib}, 2, "no test programs"},
		{"missing file", []string{filepath.Join(dir, "none.st")}, 2, ""},
		{"no files", nil, 2, "usage"},
	} {
		var out, errOut bytes.Buffer
		csvDir := filepath.Join(dir, "csv")
		code := runTests(tc.files, sil.Options{}, csvDir, &out, &errOut)
		if code != tc.code {
			t.Errorf("%s: exit %d, want %d\n%s%s", tc.name, code, tc.code, out.String(), errOut.String())
		}
		if !strings.Contains(out.String()+errOut.String(), tc.out) {
			t.Errorf("%s: output lacks %q:\n%s%s", tc.name, tc.out, out.String(), errOut.String())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "csv", "TEST_Twice.vm.csv")); err != nil {
		t.Errorf("CSV not written: %v", err)
	}
}
