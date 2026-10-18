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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apiarytech/beedance/analyzer"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// runLint checks files with the static analyzer (package analyzer), as one
// source so that a POU may use another file's, and prints the findings of
// level or above as FILE:ROW:COLUMN: severity: message [rule]. It returns
// the exit status: 0 when no warning or error is printed, 1 when one is, 2
// when a file cannot be read or parsed.
func runLint(files []string, opts analyzer.Options, level analyzer.Severity, out, errOut io.Writer) int {
	if len(files) == 0 {
		fmt.Fprintln(errOut, "usage: beedance -lint [-lint-level info|warning|error] [-lint-disable RULE,...] [-lint-max-iterations N] FILE.st...")
		return 2
	}
	var b strings.Builder
	starts := make([]int, len(files)) // the row before each file's first
	rows := 0
	for i, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		// Each file is parsed on its own first, so its errors carry its rows.
		p := parser.New(lexer.New(string(src)))
		p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			fmt.Fprintf(errOut, "%s:\n", f)
			printParserErrors(errOut, errs)
			return 2
		}
		starts[i] = rows
		b.Write(src)
		b.WriteString("\n")
		rows += strings.Count(string(src), "\n") + 1
	}
	findings, errs := analyzer.CheckSource(b.String(), opts)
	if len(errs) > 0 {
		printParserErrors(errOut, errs)
		return 2
	}
	where := func(row int) (string, int) {
		for i := len(files) - 1; i >= 0; i-- {
			if row > starts[i] {
				return files[i], row - starts[i]
			}
		}
		return files[0], row
	}
	counts := map[analyzer.Severity]int{}
	for _, f := range findings {
		if f.Severity < level {
			continue
		}
		counts[f.Severity]++
		file, row := where(f.Row)
		f.Row = row
		fmt.Fprintf(out, "%s:%s\n", file, f)
	}
	shown := counts[analyzer.Error] + counts[analyzer.Warning] + counts[analyzer.Info]
	if shown == 0 {
		fmt.Fprintln(out, "beedance: no findings")
		return 0
	}
	fmt.Fprintf(out, "beedance: %d errors, %d warnings, %d infos\n", counts[analyzer.Error], counts[analyzer.Warning], counts[analyzer.Info])
	if counts[analyzer.Error]+counts[analyzer.Warning] > 0 {
		return 1
	}
	return 0
}

// lintDisabled reads -lint-disable: rule IDs and PLCopen rules (CP19
// disables the rules that check it).
func lintDisabled(list string) map[string]bool {
	out := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out[name] = true
		for _, r := range analyzer.Rules() {
			if r.PLCopen != "" && strings.EqualFold(r.PLCopen, name) {
				out[r.ID] = true
			}
		}
	}
	return out
}

// listLintRules prints the analyzer's rules.
func listLintRules(out io.Writer) {
	fmt.Fprintf(out, "%-24s %-8s %s\n", "RULE", "PLCOPEN", "")
	for _, r := range analyzer.Rules() {
		fmt.Fprintf(out, "%-24s %-8s %s\n", r.ID, r.PLCopen, r.Title)
	}
}
