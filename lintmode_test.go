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

	"github.com/apiarytech/beedance/analyzer"
)

// -lint reports each finding at its file's own row, and exits 1 for a
// warning, 0 for none, 2 for a source that does not parse.
func TestRunLint(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	half := write("half.st", "FUNCTION Half : INT\nVAR_INPUT a : INT; END_VAR\nHalf := a / 2;\nEND_FUNCTION\n")
	main := write("main.st", "PROGRAM Main\nVAR d : DINT; i : INT; END_VAR\ni := Half(a := d);\nEND_PROGRAM\n")

	var out, errOut bytes.Buffer
	if status := runLint([]string{half, main}, analyzer.Options{}, analyzer.Warning, &out, &errOut); status != 1 {
		t.Fatalf("status %d, want 1; out:\n%s%s", status, out.String(), errOut.String())
	}
	if want := main + ":3:16: warning: parameter a: DINT to INT may overflow"; !strings.Contains(out.String(), want) {
		t.Errorf("output has no %q:\n%s", want, out.String())
	}

	out.Reset()
	opts := analyzer.Options{Disable: lintDisabled("CP25")}
	if status := runLint([]string{half, main}, opts, analyzer.Warning, &out, &errOut); status != 0 || !strings.Contains(out.String(), "no findings") {
		t.Errorf("with CP25 disabled: status %d, output:\n%s", status, out.String())
	}

	out.Reset()
	if status := runLint([]string{half, main}, analyzer.Options{}, analyzer.Error, &out, &errOut); status != 0 || !strings.Contains(out.String(), "no findings") {
		t.Errorf("at level error, with only a warning: status %d, output:\n%s", status, out.String())
	}

	bad := write("bad.st", "PROGRAM P x := ; END_PROGRAM\n")
	errOut.Reset()
	if status := runLint([]string{bad}, analyzer.Options{}, analyzer.Warning, &out, &errOut); status != 2 || !strings.Contains(errOut.String(), bad) {
		t.Errorf("a source that does not parse: status %d, errors:\n%s", status, errOut.String())
	}
}
