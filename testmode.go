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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/apiarytech/beedance/sil"
)

// runTests runs the PROGRAM TEST_... unit tests of files (see package sil),
// prints a report to out and errors to errOut, and returns the exit status:
// 0 all passed, 1 a test failed or the engines disagree, 2 the files could
// not be read or parsed or hold no tests.
func runTests(files []string, opts sil.Options, csvDir string, out, errOut io.Writer) int {
	if len(files) == 0 {
		fmt.Fprintln(errOut, "usage: beedance -test [-interval 10ms] [-max-scans 50000] [-engines eval,vm|all] [-tol 1e-3] [-run NAME] [-members outputs|all] [-depth 2] [-csv DIR] [-go-timeout 1m] [-go-replace MOD=DIR] FILE.st...")
		return 2
	}
	var b strings.Builder
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		b.Write(src)
		b.WriteString("\n")
	}

	results, err := sil.Run(context.Background(), b.String(), opts)
	if err != nil {
		var pe *sil.ParseError
		if errors.As(err, &pe) {
			printParserErrors(errOut, pe.Errors)
		} else {
			fmt.Fprintln(errOut, err)
		}
		return 2
	}

	engines := opts.Engines
	if len(engines) == 0 {
		engines = []sil.Engine{sil.Evaluator, sil.VM}
	}
	header := fmt.Sprintf("%-34s", "TEST")
	for _, e := range engines {
		header += fmt.Sprintf(" %-22s", strings.ToUpper(string(e)))
	}
	if len(engines) > 1 {
		header += " ENGINES"
	}
	fmt.Fprintf(out, "beedance: %d tests\n\n%s\n", len(results), strings.TrimRight(header, " "))

	failed := 0
	for _, res := range results {
		if !res.Passed {
			failed++
		}
		line := fmt.Sprintf("%-34s", res.Name)
		for _, r := range res.Engines {
			line += fmt.Sprintf(" %-22s", status(r))
		}
		if len(res.Engines) > 1 {
			errored := false
			for _, r := range res.Engines {
				errored = errored || r.Err != ""
			}
			switch {
			case res.Mismatch != "":
				line += " DIFFER: " + res.Mismatch
			case errored:
				line += " -" // not all compared
			default:
				line += " agree"
			}
		}
		fmt.Fprintln(out, strings.TrimRight(line, " "))
		for _, r := range res.Engines {
			for _, p := range r.Problems() {
				fmt.Fprintf(out, "    %s: %s\n", r.Engine, p)
			}
		}
		if csvDir != "" {
			for _, r := range res.Engines {
				if err := writeTestCSV(csvDir, res, r); err != nil {
					fmt.Fprintln(errOut, err)
				}
			}
		}
	}

	fmt.Fprintln(out)
	if failed > 0 {
		fmt.Fprintf(out, "FAILED: %d of %d tests\n", failed, len(results))
		return 1
	}
	fmt.Fprintf(out, "PASSED: all %d tests\n", len(results))
	return 0
}

func status(r sil.EngineResult) string {
	switch {
	case r.Passed:
		return fmt.Sprintf("PASS (%d scans)", len(r.Scans))
	case r.Err != "":
		return "ERROR"
	default:
		return fmt.Sprintf("FAIL (%d)", r.Failures)
	}
}

func writeTestCSV(dir string, res sil.Result, r sil.EngineResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, res.Name+"."+string(r.Engine)+".csv"))
	if err != nil {
		return err
	}
	if err := sil.WriteCSV(f, res.Variables, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
