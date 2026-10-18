/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package analyzer is a static analyzer for IEC 61131-3 sources: it reads
// what beedance's parser reads (ST, IL, SFC, and LD and FBD in their text
// form, which the parser lowers to ST) and reports what is likely wrong or
// against the PLCopen Coding Guidelines (v1.0, 2016), without running it.
//
// Its checks:
//
//   - unused variables, inputs never read, outputs never set (PLCopen CP24)
//   - unreachable code: after RETURN, EXIT, CONTINUE or JMP, and branches a
//     constant condition never takes (CP2)
//   - implicit narrowing: an assignment or a parameter that loses range,
//     sign or precision, and literals out of their type's range (CP25)
//   - a physical output written in more than one place (CP12), a global
//     written by more than one PROGRAM (CP26)
//   - loops that do not end, and loops whose worst case in a scan exceeds
//     Options.MaxIterations, which could overrun the cycle time
//   - and these PLCopen rules: floating point (CP8) and time (CP28)
//     equality, recursion (CP13), writes to an input (CP17), an instance
//     called more than once (CP20), external variables in a function block
//     (CP6), JMP and RETURN (CP19), CONTINUE and EXIT (L10), a FOR counter
//     changed in its loop (L12), IF without ELSE (L17), physical addresses
//     in code (N1), a local shadowing a global (N5)
//
// Types come from the declarations (aliases resolved), the standard
// functions and function blocks, and the IEC 61131-3 Ed.3 implicit
// conversions; an expression whose type cannot be worked out is not
// checked, so the analyzer errs on the side of saying nothing.
package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// Severity is how serious a finding is.
type Severity int

// The severities, least serious first.
const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	}
	return "info"
}

// ParseSeverity reads "info", "warning" or "error".
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(s) {
	case "info":
		return Info, nil
	case "warning":
		return Warning, nil
	case "error":
		return Error, nil
	}
	return Info, fmt.Errorf("unknown severity %q: info, warning or error", s)
}

// Finding is one thing the analyzer reports.
type Finding struct {
	Rule     string   // the rule's ID, e.g. "narrowing"
	PLCopen  string   // the PLCopen Coding Guidelines rule, e.g. "CP25", or ""
	Severity Severity //
	Row      int      // 1-based, in the source checked
	Column   int      //
	POU      string   // the POU it is in, or "" for a global declaration
	Message  string
}

// String writes a finding as "row:column: severity: message [rule, PLCopen ID]".
func (f Finding) String() string {
	ref := f.Rule
	if f.PLCopen != "" {
		ref += ", PLCopen " + f.PLCopen
	}
	return fmt.Sprintf("%d:%d: %s: %s [%s]", f.Row, f.Column, f.Severity, f.Message, ref)
}

// Rule describes one of the analyzer's rules.
type Rule struct {
	ID      string
	PLCopen string // the PLCopen Coding Guidelines rule it checks, or ""
	Title   string
}

// rules are the analyzer's rules, in the order Rules lists them.
var rules = []Rule{
	{"unused-variable", "CP24", "Do not declare variables that are not used"},
	{"unread-variable", "CP24", "A temporary or function variable written but never read"},
	{"unused-input", "CP24", "An input of a function, function block or method never read"},
	{"unset-output", "CP24", "An output or a function's result never set"},
	{"unreachable-code", "CP2", "All code shall be used in the application"},
	{"narrowing", "CP25", "Data types conversion should be explicit"},
	{"literal-range", "CP25", "A literal out of the range of its type"},
	{"multiple-output-writes", "CP12", "Physical outputs shall be written once per PLC cycle"},
	{"global-writers", "CP26", "A global variable may be written only by one PROGRAM"},
	{"unbounded-loop", "", "A loop that does not end, or whose iterations a scan cannot bound"},
	{"loop-iterations", "", "A loop whose worst case in a scan exceeds the iteration limit"},
	{"for-counter-modified", "L12", "Loop variables should not be modified inside a FOR loop"},
	{"float-equality", "CP8", "Floating point comparison shall not be equality or inequality"},
	{"time-equality", "CP28", "Time and physical measures comparison shall not be equality or inequality"},
	{"recursion", "CP13", "POUs shall not call themselves directly or indirectly"},
	{"input-write", "CP17", "Usage of parameters shall match their declaration mode"},
	{"instance-calls", "CP20", "Function block instances should be called only once"},
	{"external-in-fb", "CP6", "Avoid external variables in functions and function blocks"},
	{"jump-return", "CP19", "Usage of jump and return should be avoided"},
	{"continue-exit", "L10", "Usage of CONTINUE and EXIT instruction should be avoided"},
	{"if-without-else", "L17", "Each IF instruction should have an ELSE clause"},
	{"direct-address", "N1", "Avoid physical addresses"},
	{"shadowed-global", "N5", "Local names shall not shadow global names"},
}

var plcopenOf = func() map[string]string {
	m := map[string]string{}
	for _, r := range rules {
		m[r.ID] = r.PLCopen
	}
	return m
}()

// Rules lists the analyzer's rules.
func Rules() []Rule { return append([]Rule(nil), rules...) }

// DefaultMaxIterations is Options.MaxIterations when it is 0.
const DefaultMaxIterations = 100_000

// Options tune a check.
type Options struct {
	// Disable holds the IDs of rules not to report (see Rules).
	Disable map[string]bool
	// MaxIterations is the most iterations a scan may make of a loop, with
	// the loops around it, before loop-iterations reports it; 0 is
	// DefaultMaxIterations.
	MaxIterations int64
}

// Check analyzes a parsed program and returns its findings by position.
func Check(program *ast.Program, opts Options) []Finding {
	if program == nil {
		return nil
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = DefaultMaxIterations
	}
	a := newAnalysis(program, opts)
	a.run()
	sort.SliceStable(a.findings, func(i, j int) bool {
		fi, fj := a.findings[i], a.findings[j]
		if fi.Row != fj.Row {
			return fi.Row < fj.Row
		}
		return fi.Column < fj.Column
	})
	return a.findings
}

// CheckSource parses source and analyzes it. A source that does not parse
// is not analyzed: its parser errors are returned instead.
func CheckSource(source string, opts Options) ([]Finding, []string) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, errs
	}
	return Check(program, opts), nil
}
