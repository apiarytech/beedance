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
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
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

// echoRig is a rig that wires output %QX0.0 back to input %IX0.0.
type echoRig struct{ q bool }

func (r *echoRig) Begin(context.Context, string, sil.Engine, []sil.Point) error {
	r.q = false
	return nil
}

func (r *echoRig) Read(context.Context, time.Duration) (map[string]object.Object, error) {
	return map[string]object.Object{"%IX0.0": &object.Boolean{Value: r.q}}, nil
}

func (r *echoRig) Write(_ context.Context, _ time.Duration, out map[string]object.Object) error {
	if b, ok := out["%QX0.0"].(*object.Boolean); ok {
		r.q = b.Value
	}
	return nil
}

func (r *echoRig) End(context.Context) error { return nil }

func TestRunTestsWithRig(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		sil.ServeRig(context.Background(), conn, &echoRig{})
	}()
	p := filepath.Join(t.TempDir(), "loop.st")
	src := "PROGRAM TEST_Loopback\nVAR_OUTPUT failures : INT; done : BOOL; END_VAR\n" +
		"VAR i AT %IX0.0 : BOOL; q AT %QX0.0 : BOOL; END_VAR\nq := TRUE;\ndone := i;\nEND_PROGRAM\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	rig, err := sil.DialRig(context.Background(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer rig.Close()
	var out, errOut bytes.Buffer
	opts := sil.Options{IO: rig, RealTime: true, Interval: time.Millisecond}
	if code := runTests([]string{p}, opts, "", &out, &errOut); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "PASS (2 scans)") || !strings.Contains(out.String(), "not compared") {
		t.Errorf("output:\n%s", out.String())
	}
}
