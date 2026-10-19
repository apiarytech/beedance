/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

// The monitor, a PLC's online view of its code: beedance/monitor gives the
// values of the variables named on lines of the program's source, each
// where it stands, so an editor shows them beside the code as it runs.
// A name is read as the program sees it: in the program's body, its own;
// in a function block's, through an instance of the block (the page's
// choice, else the program's first of its type); a function's locals,
// which live only during a call, are left out. Stopped, the values are as
// they are; running, as a recent scan left them.

import (
	"slices"
	"strings"

	"github.com/apiarytech/beedance/ide"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/token"
)

// monitorArgs are beedance/monitor's: the lines (1-based, inclusive) of
// the program's source, and, by POU, an instance to see a function
// block's body through (as beedance/diagrams').
type monitorArgs struct {
	From     int               `json:"from"`
	To       int               `json:"to"`
	Prefixes map[string]string `json:"prefixes"`
}

// monitored is a name on a line and its value.
type monitored struct {
	Column int    `json:"column"`
	Name   string `json:"name"`  // as written: "t.Q"
	Path   string `json:"path"`  // as read: "b.t.Q" in an instance b
	Value  string `json:"value"` // empty when it has none (yet)
}

// maxMonitorLines bounds a request: an editor asks for what it shows.
const maxMonitorLines = 400

func (s *Server) monitor(a monitorArgs) map[string]any {
	ix := s.analyzed()
	program := s.sess.Program()
	if a.From < 1 {
		a.From = 1
	}
	if a.To < a.From {
		a.To = a.From
	}
	a.To = min(a.To, a.From+maxMonitorLines-1)

	lines := map[int][]monitored{}
	var paths []string
	for _, ch := range s.chains(a.From, a.To) {
		sym, _, ok := ix.SymbolAt(ide.Pos{Row: ch.row, Column: ch.col})
		if !ok || sym.Kind != ide.Variable {
			continue
		}
		prefix := ""
		if sym.Container != "" && !strings.EqualFold(sym.Container, program) {
			p, ok := s.prefixFor(ix, program, sym.Container, a.Prefixes)
			if !ok {
				continue // a function's local, or a block with no instance
			}
			prefix = p
		}
		m := monitored{Column: ch.col, Name: ch.text, Path: prefix + ch.text}
		if ch.text == sym.Name && !elementary(sym.Type) {
			continue // an instance, a structure, an array: its members are
		}
		lines[ch.row] = append(lines[ch.row], m)
		paths = append(paths, m.Path)
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)

	var texts map[string]string
	scan := 0
	if s.isRunning() {
		s.sampleFor("monitor", paths)
		smp := s.sess.Live()
		texts, scan = smp.Extra, smp.Scan
	} else {
		texts, scan = s.sess.Texts(paths), s.sess.Scans()
	}
	for row, ms := range lines {
		for i := range ms {
			ms[i].Value = texts[ms[i].Path]
		}
		lines[row] = ms
	}
	return map[string]any{"program": program, "scan": scan, "lines": lines}
}

// chain is a name as written on a line: an identifier, and the members
// after it ("t.Q", "a.b.c").
type chain struct {
	row, col int
	text     string
}

// chains are the names on lines from..to of the source debugged, outside
// comments and strings (the lexer's tokens).
func (s *Server) chains(from, to int) []chain {
	src := s.sess.Source()
	if s.monitorSource != src || s.monitorToks == nil {
		s.monitorToks = s.monitorToks[:0]
		l := lexer.New(src)
		for t := l.NextToken(); t.Type != token.EOF; t = l.NextToken() {
			s.monitorToks = append(s.monitorToks, t)
		}
		s.monitorSource = src
	}
	toks := s.monitorToks
	var out []chain
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		// A keyword may be a name (STEP, a block's input): any word is a
		// candidate, and the analysis says if it is a variable.
		if t.Row < from || t.Row > to || !isWord(t.Literal) {
			continue
		}
		if i > 0 && toks[i-1].Type == token.DOT {
			continue // a member: part of the chain before it
		}
		text := t.Literal
		for i+2 < len(toks) && toks[i+1].Type == token.DOT && isWord(toks[i+2].Literal) && toks[i+2].Row == t.Row {
			text += "." + toks[i+2].Literal
			i += 2
		}
		out = append(out, chain{row: t.Row, col: t.Column, text: text})
	}
	return out
}

// isWord reports whether s is a name's spelling: a letter or _, then
// letters, digits and _.
func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// elementary reports whether a declared type names an elementary value
// (BOOL, INT, REAL, TIME, STRING(20), ...), which has a value to show; a
// block's, a structure's or an array's has members instead.
func elementary(typ string) bool {
	t := strings.ToUpper(strings.TrimSpace(typ))
	if i := strings.IndexAny(t, "[("); i > 0 {
		t = strings.TrimSpace(t[:i])
	}
	switch t {
	case "BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT",
		"BYTE", "WORD", "DWORD", "LWORD", "REAL", "LREAL", "TIME", "LTIME",
		"DATE", "LDATE", "TIME_OF_DAY", "TOD", "LTOD", "DATE_AND_TIME", "DT", "LDT",
		"STRING", "WSTRING", "CHAR", "WCHAR":
		return true
	}
	return false
}

// analyzed is the source debugged, analyzed (cached while it stays).
func (s *Server) analyzed() *ide.Index {
	src := s.sess.Source()
	if s.ideIndex == nil || s.ideSource != src {
		s.ideIndex, s.ideSource = ide.Analyze(src), src
	}
	return s.ideIndex
}

// sampleFor asks the session to sample names for one purpose (the
// diagrams, the monitor): it samples the union of what each asked for.
func (s *Server) sampleFor(purpose string, names []string) {
	if s.sampled == nil {
		s.sampled = map[string][]string{}
	}
	if slices.Equal(s.sampled[purpose], names) {
		return
	}
	s.sampled[purpose] = names
	var all []string
	for _, ns := range s.sampled {
		all = append(all, ns...)
	}
	slices.Sort(all)
	s.sess.SetSampled(slices.Compact(all))
}
