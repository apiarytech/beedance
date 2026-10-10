/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package parser

import (
	"regexp"
	"strconv"
	"sync"
)

// Diagnostic is a parse error with its position, for editors. Row and
// Column are 1-based; 0 when the error has no position.
type Diagnostic struct {
	Row, Column int
	Message     string
}

// The positions errors are written with: "... at row 3, column 7" (most),
// "line 3, column 7: ..." (LD and FBD bodies), "ERROR (3:7): ...".
// errorPositions compiles them on first use, so a program that never reads
// a diagnostic (firmware) does not carry the regexp package.
func errorPositions() []*regexp.Regexp {
	errorPositionsOnce.Do(func() {
		errorPositionsRE = []*regexp.Regexp{
			regexp.MustCompile(`at row (\d+), column (\d+)`),
			regexp.MustCompile(`line (\d+), column (\d+)`),
			regexp.MustCompile(`\((\d+):(\d+)\)`),
			regexp.MustCompile(`(?:at row|line) (\d+)()`),
		}
	})
	return errorPositionsRE
}

var (
	errorPositionsOnce sync.Once
	errorPositionsRE   []*regexp.Regexp
)

// Diagnostics are Errors with their positions.
func (p *Parser) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, 0, len(p.errors))
	for _, msg := range p.errors {
		out = append(out, ErrorDiagnostic(msg))
	}
	return out
}

// ErrorDiagnostic finds the position in an error message of beedance's.
func ErrorDiagnostic(msg string) Diagnostic {
	d := Diagnostic{Message: msg}
	for _, re := range errorPositions() {
		if m := re.FindStringSubmatch(msg); m != nil {
			d.Row, _ = strconv.Atoi(m[1])
			d.Column, _ = strconv.Atoi(m[2])
			break
		}
	}
	return d
}
