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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/embedded"
	"github.com/apiarytech/beedance/embedded/harden"
	"github.com/apiarytech/beedance/object"
)

// -embedded writes the first PROGRAM of the -iec file as a scan loop on the
// VM, with -interval as its scan time.
func TestGenerateEmbedded(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "blink.st")
	out := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(`
PROGRAM Blink
VAR t : TON; led AT %QX0.0 : BOOL; END_VAR
t(IN := NOT t.Q, PT := T#500ms);
IF t.Q THEN led := NOT led; END_IF;
END_PROGRAM
`), 0o644); err != nil {
		t.Fatal(err)
	}
	object.FinalizeBuiltins() // as main does
	if err := generateEmbedded(src, out, embedded.ProgramOptions{Interval: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package main", "func main()", "const Interval = time.Duration(20000000)"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the generated file has no %q", want)
		}
	}

	if err := os.WriteFile(src, []byte("FUNCTION F : INT F := 1; END_FUNCTION"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generateEmbedded(src, out, embedded.ProgramOptions{}); err == nil || !strings.Contains(err.Error(), "no PROGRAM") {
		t.Errorf("a file without a PROGRAM: error %v, want one naming the missing PROGRAM", err)
	}
}

// -image writes the first PROGRAM of the -iec file as a program image that
// decodes to what the compiler made.
func TestGenerateImage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "blink.st")
	out := filepath.Join(dir, "blink.img")
	if err := os.WriteFile(src, []byte(`
PROGRAM Blink
VAR RETAIN n : DINT; END_VAR
VAR t : TON; led AT %QX0.0 : BOOL; END_VAR
t(IN := NOT t.Q, PT := T#500ms);
IF t.Q THEN led := NOT led; n := n + 1; END_IF;
END_PROGRAM
`), 0o644); err != nil {
		t.Fatal(err)
	}
	object.FinalizeBuiltins() // as main does
	if err := generateImage(src, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	im, err := harden.DecodeImage(b)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compileFirstProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := harden.ImageOf(prog); im.Checksum() != want.Checksum() || len(im.RetainSlots) != 1 {
		t.Errorf("decoded image differs from the compiled program")
	}
}
