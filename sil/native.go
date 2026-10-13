/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/transpiler"
)

// Module versions the transpiled tests build with, unless
// Options.GoReplace points a module at a local directory.
const (
	RoyaljellyVersion = "v0.3.1"
	BeebreadVersion   = "v0.1.0"
)

// harnessPackage is the package of the transpiled source and its harness.
const harnessPackage = "siltests"

// native is the transpiled tests, built once for a run: as an executable
// for the go engine, as a WASI module for the wasm engine, or both.
type native struct {
	dir     string // the temporary module
	bin     string // its executable
	wasm    string // its WebAssembly module
	runtime []string
	// errs holds why an engine cannot run, its build having failed.
	errs map[Engine]error
}

// buildNative transpiles source to Go, adds a harness that runs each test
// of tests scan by scan on a simulated clock, and builds it with the Go
// toolchain in a temporary module, for each of engines (Go, Wasm).
func buildNative(ctx context.Context, source string, tests []*testInfo, opts Options, engines []Engine) (*native, error) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("the %s engine needs the Go toolchain: go not found in PATH", engines[0])
	}
	// A fresh AST: the transpiler rewrites the one it is given, which the
	// VM must not compile.
	program, _ := parse(source)
	code, err := transpiler.GoFileWith(program, transpiler.Options{Package: harnessPackage, HostBinding: true})
	if err != nil {
		return nil, fmt.Errorf("transpile: %v", err)
	}
	dir, err := os.MkdirTemp("", "beedance-sil-")
	if err != nil {
		return nil, err
	}
	// Not named like the package directory: go build -o onto an existing
	// directory writes into it, so without .exe (Linux, macOS) the
	// executable would not be where it is run from.
	n := &native{dir: dir, bin: filepath.Join(dir, "silrun"), wasm: filepath.Join(dir, "silrun.wasm"), errs: map[Engine]error{}}
	if runtime.GOOS == "windows" {
		n.bin += ".exe"
	}
	hasClock := bytes.Contains(code, []byte("var plcClock"))
	harness, main := harnessSource(tests, hasClock), mainSource
	if opts.IO != nil {
		ioSrc, err := harnessIOSource(tests, hasClock)
		if err != nil {
			n.close()
			return nil, err
		}
		harness = strings.Replace(harness, "\t\"time\"\n)", "\t\"time\"\n"+harnessIOImports+")", 1) + ioSrc
		main = mainIOSource
	}
	files := map[string]string{ // by path in the module
		"go.mod":  goMod(string(code), opts.GoReplace),
		"main.go": main,
		filepath.Join(harnessPackage, "tests.go"): string(code),
		filepath.Join(harnessPackage, "sil.go"):   harness,
	}
	if err := os.MkdirAll(filepath.Join(dir, harnessPackage), 0o755); err != nil {
		n.close()
		return nil, err
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			n.close()
			return nil, err
		}
	}
	goCmd := func(env []string, args ...string) error {
		cmd := exec.CommandContext(ctx, goTool, args...)
		cmd.Dir = dir
		cmd.Env = append(append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off"), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go %s: %v\n%s", args[0], err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := goCmd(nil, "mod", "tidy"); err != nil {
		n.close()
		return nil, err
	}
	// A failed build fails only its engine's runs.
	for _, e := range engines {
		out, env := n.bin, []string(nil)
		if e == Wasm {
			if n.runtime, err = wasmRuntime(ctx, opts, dir); err != nil {
				n.errs[e] = err
				continue
			}
			out, env = n.wasm, []string{"GOOS=wasip1", "GOARCH=wasm"}
		}
		if err := goCmd(env, "build", "-o", out, "."); err != nil {
			n.errs[e] = err
		} else if fi, err := os.Stat(out); err != nil || !fi.Mode().IsRegular() {
			n.errs[e] = fmt.Errorf("go build: nothing built at %s", out)
		}
	}
	return n, nil
}

// command runs the transpiled tests with args on engine e: the executable,
// or the module under the WASI runtime.
func (n *native) command(ctx context.Context, e Engine, args ...string) *exec.Cmd {
	if e == Wasm {
		argv := append(append(append([]string{}, n.runtime[1:]...), n.wasm), args...)
		return exec.CommandContext(ctx, n.runtime[0], argv...)
	}
	return exec.CommandContext(ctx, n.bin, args...)
}

func (n *native) close() { os.RemoveAll(n.dir) }

// nativeResult is what the harness prints for one test.
type nativeResult struct {
	Scans [][]string `json:"scans"`
	Err   string     `json:"err"`
}

// run runs one test in its own process, so a runaway loop is stopped by
// the timeout instead of holding the run.
func (n *native) run(ctx context.Context, e Engine, t *testInfo, opts Options) (r EngineResult) {
	ctx, cancel := context.WithTimeout(ctx, opts.GoTimeout)
	defer cancel()
	cmd := n.command(ctx, e, t.decl.Name.Value, strconv.Itoa(t.limit), opts.Interval.String())
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		r.Err = fmt.Sprintf("no result after %v: a runaway loop? (see -go-timeout)", opts.GoTimeout)
		return r
	}
	var res nativeResult
	if jerr := json.Unmarshal(stdout.Bytes(), &res); jerr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && err != nil {
			msg = err.Error()
		}
		r.Err = string(e) + ": " + msg
		return r
	}
	for _, row := range res.Scans {
		s := Scan{}
		for i, v := range t.vars {
			if i < len(row) {
				s[v.Name] = row[i]
			}
		}
		r.Scans = append(r.Scans, s)
	}
	r.Err = res.Err
	return r
}

// goMod is the temporary module's go.mod: royaljelly, and beebread when the
// transpiled code uses OSCAT, at the versions above or replaced.
func goMod(code string, replace map[string]string) string {
	var b strings.Builder
	b.WriteString("module beedance.sil/run\n\ngo 1.27.2\n\n")
	b.WriteString("require github.com/apiarytech/royaljelly " + RoyaljellyVersion + "\n")
	if strings.Contains(code, "github.com/apiarytech/beebread") {
		b.WriteString("require github.com/apiarytech/beebread " + BeebreadVersion + "\n")
	}
	mods := make([]string, 0, len(replace))
	for m := range replace {
		mods = append(mods, m)
	}
	sort.Strings(mods)
	for _, m := range mods {
		dir, err := filepath.Abs(replace[m])
		if err != nil {
			dir = replace[m]
		}
		fmt.Fprintf(&b, "replace %s => %s\n", m, filepath.ToSlash(dir))
	}
	return b.String()
}

const mainSource = `package main

import (
	"errors"
	"os"
	"strconv"
	"syscall"
	"time"

	"beedance.sil/run/siltests"
)

func main() {
	limit, _ := strconv.Atoi(os.Args[2])
	interval, _ := time.ParseDuration(os.Args[3])
	siltests.Run(os.Args[1], limit, interval, stdio{os.Stdout})
}
` + stdioSource

// mainIOSource is main for a run with Options.IO: a fourth argument "io"
// runs the test on request (RunIO).
const mainIOSource = `package main

import (
	"errors"
	"os"
	"strconv"
	"syscall"
	"time"

	"beedance.sil/run/siltests"
)

func main() {
	if len(os.Args) > 4 && os.Args[4] == "io" {
		siltests.RunIO(os.Args[1], stdio{os.Stdin}, stdio{os.Stdout})
		return
	}
	limit, _ := strconv.Atoi(os.Args[2])
	interval, _ := time.ParseDuration(os.Args[3])
	siltests.Run(os.Args[1], limit, interval, stdio{os.Stdout})
}
` + stdioSource

// stdioSource is the harness's stdin and stdout. A WASI runtime may give the
// module non-blocking pipes (Node.js does), on which a read or a write fails
// with EAGAIN until the pipe is ready: output past the 64 KB a pipe holds
// would be lost.
const stdioSource = `
type stdio struct{ f *os.File }

func (s stdio) Read(p []byte) (int, error) {
	for {
		n, err := s.f.Read(p)
		if n > 0 || !errors.Is(err, syscall.EAGAIN) {
			return n, err
		}
		time.Sleep(time.Millisecond)
	}
}

func (s stdio) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n, err := s.f.Write(p)
		written += n
		p = p[n:]
		if errors.Is(err, syscall.EAGAIN) {
			time.Sleep(time.Millisecond)
			continue
		}
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
`

// harnessSource is the Go that runs each test of tests in the transpiled
// package: scan by scan, the clock interval apart, until done, recording
// the watched values formatted as the evaluator and the VM print them.
func harnessSource(tests []*testInfo, hasClock bool) string {
	var b strings.Builder
	b.WriteString(`package ` + harnessPackage + `

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type silResult struct {
	Scans [][]string ` + "`json:\"scans\"`" + `
	Err   string     ` + "`json:\"err\"`" + `
}

func silTime(d time.Duration) string { return "T#" + d.String() }

// Run runs the test called name and writes its scans as JSON to w.
func Run(name string, limit int, interval time.Duration, w io.Writer) {
	var r silResult
	defer func() {
		if x := recover(); x != nil {
			r.Err = fmt.Sprintf("scan %d: panic: %v", len(r.Scans)+1, x)
		}
		json.NewEncoder(w).Encode(r)
	}()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var elapsed time.Duration
	_, _ = start, elapsed
`)
	if hasClock {
		b.WriteString("\tplcClock = func() time.Duration { return elapsed }\n")
	}
	b.WriteString("\tswitch name {\n")
	for _, t := range tests {
		if t.out.failures == "" {
			continue
		}
		name := t.decl.Name.Value
		fmt.Fprintf(&b, "\tcase %q:\n\t\tp := New%s()\n", name, name)
		b.WriteString("\t\tfor n := 0; n < limit; n++ {\n")
		b.WriteString("\t\t\telapsed = time.Duration(n) * interval\n")
		b.WriteString("\t\t\tp.Logic(start.Add(elapsed))\n")
		b.WriteString("\t\t\tr.Scans = append(r.Scans, []string{\n")
		for _, v := range t.vars {
			fmt.Fprintf(&b, "\t\t\t\t%s,\n", goFormat(goSelector(v.Path), v.Type))
		}
		b.WriteString("\t\t\t})\n")
		if t.out.done != "" {
			fmt.Fprintf(&b, "\t\t\tif p.%s {\n\t\t\t\tbreak\n\t\t\t}\n", transpiler.GoName(t.out.done))
		}
		b.WriteString("\t\t}\n")
	}
	b.WriteString("\tdefault:\n\t\tr.Err = \"no such test\"\n\t}\n}\n")
	return b.String()
}

// goSelector is the Go expression of a watched value of the program p.
func goSelector(path []string) string {
	parts := []string{"p"}
	for _, name := range path {
		parts = append(parts, transpiler.GoName(name))
	}
	return strings.Join(parts, ".")
}

// goFormat formats the Go value x of IEC type typ as the evaluator and the
// VM print it (object.Inspect).
func goFormat(x, typ string) string {
	switch typ {
	case "BOOL":
		return fmt.Sprintf("fmt.Sprintf(\"%%t\", bool(%s))", x)
	case "REAL", "LREAL":
		return fmt.Sprintf("fmt.Sprintf(\"%%f\", float64(%s))", x)
	case "TIME":
		return fmt.Sprintf("silTime(time.Duration(%s))", x)
	case "STRING":
		return fmt.Sprintf("string(%s)", x)
	case "BYTE", "WORD", "DWORD", "LWORD":
		return fmt.Sprintf("fmt.Sprintf(\"%s#16#%%X\", uint64(%s))", typ, x)
	}
	return fmt.Sprintf("fmt.Sprintf(\"%%d\", %s)", x)
}

// defaultGoTimeout bounds one test's process on the go engine.
const defaultGoTimeout = time.Minute
