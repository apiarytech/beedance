/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/sil"
)

// buildModule writes files to a new directory and runs go there with env.
func buildModule(t *testing.T, files map[string]string, env []string, args ...string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range [][]string{{"mod", "tidy"}, args} {
		cmd := exec.Command("go", a...)
		cmd.Dir = dir
		cmd.Env = append(append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off"), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", a[0], err, out)
		}
	}
	return dir
}

// pipes is a process's stdin and stdout as one connection.
type pipes struct {
	io.WriteCloser
	io.Reader
	cmd *exec.Cmd
}

func (p *pipes) Close() error {
	p.WriteCloser.Close()
	return p.cmd.Wait()
}

// TestGoBuildStart: a host that builds and starts the program itself
// (GoBuild.Start, as a web page's build service and worker do) gets
// GoProgramFiles' module, and the session debugs the program it returns.
// The module also builds for GOOS=js, where it serves a worker's messages.
func TestGoBuildStart(t *testing.T) {
	if err := sil.GoEngineAvailable(); err != nil {
		t.Skip("the go engine cannot build here:", err)
	}
	var started GoStart
	start := func(g GoStart) (io.ReadWriteCloser, error) {
		started = g
		files, err := GoProgramFiles(g.Source, g.Program, g.AllowInputWrites)
		if err != nil {
			return nil, err
		}
		dir := buildModule(t, files, nil, "build", "-o", "prog.exe", ".")
		cmd := exec.Command(filepath.Join(dir, "prog.exe"))
		in, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return &pipes{WriteCloser: in, Reader: out, cmd: cmd}, nil
	}
	s, err := Load(source, Options{Engine: EngineGo, GoBuild: GoBuild{Start: start}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if started.Program != "Main" || !strings.Contains(started.Source, "PROGRAM Main") {
		t.Fatalf("GoStart %q", started.Program)
	}
	row := line(t, source, "n := n + 1;")
	s.SetBreakpoints([]BreakpointSpec{{Line: row}})
	for i := 1; i <= 2; i++ {
		stop, err := s.Continue(context.Background())
		if err != nil || stop.Reason != OnBreakpoint || stop.Line != row {
			t.Fatalf("stop %d: %+v %v", i, stop, err)
		}
	}
	if got := value(t, s, "n"); got != "1" {
		t.Errorf("n = %s, want 1", got)
	}

	files, err := GoProgramFiles(source, "Main", false)
	if err != nil {
		t.Fatal(err)
	}
	if files["stdio_js.go"] == "" || !strings.Contains(files["dbgprog/prog.go"], "package dbgprog") {
		t.Fatalf("files %v", len(files))
	}
	buildModule(t, files, []string{"GOOS=js", "GOARCH=wasm"}, "build", "-o", "prog.wasm", ".")
}
