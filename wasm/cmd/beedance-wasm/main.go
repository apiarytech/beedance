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

// Command beedance-wasm is beedance's toolchain as a WebAssembly module
// (package wasm). Built for a WASI runtime (GOOS=wasip1), or natively, it
// talks over stdin and stdout:
//
//	beedance-wasm [serve]                     one JSON request per line in, one reply per line out
//	beedance-wasm test [flags] [FILE.st...]   run the TEST_ programs of the files, or of stdin
//
// test prints a report and exits 0 when every test passes, 1 when one fails
// and 2 when the source cannot run. Built for the web (GOOS=js), it gives
// the page the same requests as globalThis.beedance.handle (main_js.go).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apiarytech/beedance/wasm"
)

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "serve":
		if err := serve(os.Stdin, retryWriter{os.Stdout}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "test":
		os.Exit(test(args, os.Stdin, retryWriter{os.Stdout}, retryWriter{os.Stderr}))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\nusage: beedance-wasm [serve] | test [-engines eval,vm] [-run NAME] [-interval-ms 10] [-max-scans 50000] [-tol 1e-3] < FILE.st\n", cmd)
		os.Exit(2)
	}
}

// serve answers JSON requests, one per line, until in ends.
func serve(in io.Reader, out io.Writer) error {
	h := wasm.New()
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<16), 64<<20) // a request carries a whole source
	w := bufio.NewWriter(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		w.Write(h.Handle([]byte(line)))
		w.WriteByte('\n')
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return sc.Err()
}

// The reply of a test request, as far as the report reads it.
type testReply struct {
	Error   string   `json:"error"`
	Errors  []string `json:"errors"`
	Passed  bool     `json:"passed"`
	Failed  int      `json:"failed"`
	Results []struct {
		Name     string `json:"name"`
		Passed   bool   `json:"passed"`
		Mismatch string `json:"mismatch"`
		Engines  []struct {
			Engine   string   `json:"engine"`
			Passed   bool     `json:"passed"`
			Error    string   `json:"error"`
			Problems []string `json:"problems"`
		} `json:"engines"`
	} `json:"results"`
}

func test(args []string, in io.Reader, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(errOut)
	engines := fs.String("engines", "eval,vm", "engines to run on: eval, vm")
	filter := fs.String("run", "", "run only tests whose name contains this")
	interval := fs.Float64("interval-ms", 10, "simulated time between scans, in milliseconds")
	maxScans := fs.Int("max-scans", 50000, "scans a test with done may take")
	tol := fs.Float64("tol", 1e-3, "relative tolerance comparing REAL values between engines")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	src, err := source(fs.Args(), in)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	req, _ := json.Marshal(wasm.Request{Op: "test", Source: string(src), Engines: strings.Split(*engines, ","),
		Filter: *filter, IntervalMs: *interval, MaxScans: *maxScans, Tolerance: *tol})
	var r testReply
	if err := json.Unmarshal(wasm.New().Handle(req), &r); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if r.Error != "" {
		fmt.Fprintln(errOut, r.Error)
		for _, e := range r.Errors {
			fmt.Fprintln(errOut, "\t"+e)
		}
		return 2
	}

	fmt.Fprintf(out, "beedance (wasm): %d tests\n\n", len(r.Results))
	for _, res := range r.Results {
		line := fmt.Sprintf("%-34s", res.Name)
		for _, e := range res.Engines {
			status := "pass"
			switch {
			case e.Error != "":
				status = "ERROR"
			case !e.Passed:
				status = "FAIL"
			}
			line += fmt.Sprintf(" %s:%-6s", e.Engine, status)
		}
		errored := false
		for _, e := range res.Engines {
			errored = errored || e.Error != ""
		}
		switch {
		case len(res.Engines) < 2:
		case res.Mismatch != "":
			line += " DIFFER: " + res.Mismatch
		case errored:
			line += " -" // not all compared
		default:
			line += " agree"
		}
		fmt.Fprintln(out, strings.TrimRight(line, " "))
		for _, e := range res.Engines {
			for _, p := range e.Problems {
				fmt.Fprintf(out, "    %s: %s\n", e.Engine, p)
			}
		}
	}
	fmt.Fprintln(out)
	if !r.Passed {
		fmt.Fprintf(out, "FAILED: %d of %d tests\n", r.Failed, len(r.Results))
		return 1
	}
	fmt.Fprintf(out, "PASSED: all %d tests\n", len(r.Results))
	return 0
}

// source is the files named, one after another, or in when none are; a
// WASI runtime must give the module the directories they are in.
func source(files []string, in io.Reader) (string, error) {
	if len(files) == 0 {
		b, err := io.ReadAll(in)
		return string(b), err
	}
	var b strings.Builder
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		b.Write(src)
		b.WriteString("\n")
	}
	return b.String(), nil
}
