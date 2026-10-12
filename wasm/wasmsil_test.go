//go:build wasmsil

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// These tests run the WebAssembly modules in the beedance-wasm image:
//
//	docker build -f wasm/Dockerfile -t beedance-wasm .
//	go test -tags wasmsil ./wasm/
//
// WASMSIL_IMAGE overrides the image.

package wasm

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func image() string {
	if img := os.Getenv("WASMSIL_IMAGE"); img != "" {
		return img
	}
	return "beedance-wasm"
}

// docker runs the image with stdin and returns stdout and the exit code.
func docker(t *testing.T, stdin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("docker", append([]string{"run", "--rm", "-i"}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exit):
		return out.String() + errOut.String(), exit.ExitCode()
	}
	t.Fatalf("docker %v: %v\n%s", args, err, errOut.String())
	return "", 0
}

func TestWASITestCommand(t *testing.T) {
	out, code := docker(t, source, image(), "test", "-run", "add")
	if code != 0 || !strings.Contains(out, "PASSED: all 1 tests") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	out, code = docker(t, source, image(), "test")
	if code != 1 || !strings.Contains(out, "FAILED: 1 of 3 tests") {
		t.Errorf("exit %d, want 1:\n%s", code, out)
	}
	out, code = docker(t, "PROGRAM P VAR x : INT END_VAR", image(), "test")
	if code != 2 {
		t.Errorf("a source that does not parse: exit %d, want 2:\n%s", code, out)
	}
}

// TestModulesAgree sends the same requests to the native Host, the WASI
// module and the web module, which must answer alike.
func TestModulesAgree(t *testing.T) {
	var reqs []string
	for _, r := range []map[string]any{
		{"op": "check", "source": source},
		{"op": "test", "source": source, "intervalMs": 100, "scans": true},
		// TEST_Blink for 10,000 scans: a reply past the 64 KB a pipe holds.
		{"op": "test", "source": source, "filter": "blink", "intervalMs": 0.1, "scans": true},
		{"op": "transpile", "source": source},
		{"op": "load", "source": source, "intervalMs": 250},
		{"op": "scan", "session": 1, "count": 5, "values": map[string]any{"start": true}},
		{"op": "set", "session": 1, "values": map[string]any{"limit": 10}},
		{"op": "scan", "session": 1, "count": 20},
		{"op": "unload", "session": 1},
		{"op": "read", "session": 1},
	} {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, string(b))
	}
	h := New()
	var want []string
	for _, r := range reqs {
		want = append(want, string(h.Handle([]byte(r))))
	}
	if len(want[2]) < 1<<17 {
		t.Fatalf("the long reply is %d bytes, want over 128 KB", len(want[2]))
	}
	in := strings.Join(reqs, "\n") + "\n"
	for _, m := range []struct {
		name string
		args []string
	}{
		{"wasip1", []string{image(), "serve"}},
		{"js", []string{"--entrypoint", "node", image(), "/beedance/js.mjs", "/beedance/wasm_exec.js", "/beedance/beedance.wasm"}},
	} {
		out, code := docker(t, in, m.args...)
		if code != 0 {
			t.Errorf("%s: exit %d:\n%s", m.name, code, out)
			continue
		}
		got := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(got) != len(want) {
			t.Errorf("%s: %d replies, want %d:\n%s", m.name, len(got), len(want), out)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: reply to %.60s...\n got %.300s\nwant %.300s", m.name, reqs[i], got[i], want[i])
			}
		}
	}
}
