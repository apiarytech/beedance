/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ide

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apiarytech/beedance/token"
)

// tokenAt is the index of the name token at p, or just before p (the
// cursor at a word's end); -1 if there is none.
func (ix *Index) tokenAt(p Pos) int {
	at := -1
	for i, t := range ix.toks {
		if t.Row != p.Row || !isWord(t.Literal) {
			if t.Row > p.Row {
				break
			}
			continue
		}
		if t.Column <= p.Column && p.Column < t.Column+len(t.Literal) {
			return i
		}
		if p.Column == t.Column+len(t.Literal) {
			at = i
		}
	}
	return at
}

// SymbolAt is the symbol named at p, and the span of the name there.
func (ix *Index) SymbolAt(p Pos) (*Symbol, Span, bool) {
	i := ix.tokenAt(p)
	if i < 0 || ix.refs[i] == nil {
		return nil, Span{}, false
	}
	return ix.refs[i], ix.span(ix.toks[i]), true
}

// Definition is where the name at p is declared. A standard function or
// block has no place in the source: ok is false for it too.
func (ix *Index) Definition(p Pos) (Span, bool) {
	sym, _, ok := ix.SymbolAt(p)
	if !ok || sym.Decl == (Span{}) {
		return Span{}, false
	}
	return sym.Decl, true
}

// References are the places the name at p stands for the same symbol, in
// source order; with decl, its declaration too.
func (ix *Index) References(p Pos, decl bool) []Span {
	sym, _, ok := ix.SymbolAt(p)
	if !ok {
		return nil
	}
	return ix.uses(sym, decl)
}

func (ix *Index) uses(sym *Symbol, decl bool) []Span {
	var out []Span
	for i, t := range ix.toks {
		if ix.refs[i] != sym {
			continue
		}
		s := ix.span(t)
		if !decl && s == sym.Decl {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Hover describes the name at p in Markdown: what it is, where it is
// declared, and its declaration as written.
func (ix *Index) Hover(p Pos) (string, Span, bool) {
	sym, at, ok := ix.SymbolAt(p)
	if !ok {
		return "", Span{}, false
	}
	return Describe(sym), at, true
}

// Describe is sym in Markdown, as Hover gives it.
func Describe(sym *Symbol) string {
	var b strings.Builder
	what := strings.ReplaceAll(string(sym.Kind), "_", " ")
	switch {
	case sym.Section != "" && sym.Kind == Variable:
		what = sym.Section
	case sym.Kind == Builtin:
		what = "standard function"
	case sym.Standard && sym.Kind == FunctionBlock:
		what = "standard function block"
	}
	fmt.Fprintf(&b, "**%s** — %s", sym.Name, what)
	if sym.Type != "" && sym.Kind != Variable {
		fmt.Fprintf(&b, " : %s", sym.Type)
	}
	if sym.Container != "" {
		fmt.Fprintf(&b, " of %s", sym.Container)
	}
	if sym.Detail != "" {
		fmt.Fprintf(&b, "\n\n```iecst\n%s\n```", sym.Detail)
	}
	if sym.Kind == Variable && sym.Type != "" && !strings.Contains(sym.Detail, sym.Type) {
		fmt.Fprintf(&b, "\n\nType: `%s`", sym.Type)
	}
	return b.String()
}

// Rename gives the edits that rename the symbol named at p to name: each
// span is replaced by name. It refuses a standard name, a name that is not
// an identifier or is a keyword, and one the symbol's scope already has.
func (ix *Index) Rename(p Pos, name string) ([]Span, error) {
	sym, _, ok := ix.SymbolAt(p)
	if !ok {
		return nil, fmt.Errorf("no name to rename here")
	}
	if sym.Standard || sym.Decl == (Span{}) {
		return nil, fmt.Errorf("%s is not declared in this source", sym.Name)
	}
	if !isWord(name) {
		return nil, fmt.Errorf("%q is not a name: letters, digits and _, starting with a letter or _", name)
	}
	if token.LookupIdent(name) != token.IDENT {
		return nil, fmt.Errorf("%s is a keyword", strings.ToUpper(name))
	}
	if token.SameIdent(name, sym.Name) {
		return nil, nil
	}
	if other := ix.sibling(sym, name); other != nil {
		return nil, fmt.Errorf("%s already names a %s in %s", other.Name, strings.ReplaceAll(string(other.Kind), "_", " "), orSource(other.Container))
	}
	return ix.uses(sym, true), nil
}

// sibling is a symbol declared beside sym under name, or nil.
func (ix *Index) sibling(sym *Symbol, name string) *Symbol {
	key := strings.ToUpper(name)
	for _, s := range ix.pous {
		if s.vars[strings.ToUpper(sym.Name)] == sym {
			return s.vars[key]
		}
	}
	for _, s := range ix.mem {
		if s.vars[strings.ToUpper(sym.Name)] == sym {
			return s.vars[key]
		}
	}
	if sym.Container == "" || sym.Kind == Variable && sym.Section == "VAR_GLOBAL" {
		if o := ix.named[key]; o != nil {
			return o
		}
		if o := ix.glob[key]; o != nil {
			return o
		}
	}
	return nil
}

func orSource(container string) string {
	if container == "" {
		return "this source"
	}
	return container
}

// Completion is one name that may be written at a place.
type Completion struct {
	Label  string `json:"label"`
	Kind   Kind   `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// Complete lists what may be written at p, whose word so far (the letters
// before p) they all start with: after a dot, the members of what is left
// of it; elsewhere the names in scope, then the globals, the POUs and
// types, the standard library and the keywords.
func (ix *Index) Complete(p Pos) []Completion {
	off := ix.offset(p)
	if off < 0 {
		return nil
	}
	ws := off
	for ws > 0 && isWord(ix.src[ws-1:ws]) {
		ws--
	}
	for ws < off && ix.src[ws] >= '0' && ix.src[ws] <= '9' { // a word starts with a letter
		ws++
	}
	prefix := strings.ToUpper(ix.src[ws:off])
	var out []Completion
	seen := map[string]bool{}
	add := func(sym *Symbol) {
		key := strings.ToUpper(sym.Name)
		if seen[key] || !strings.HasPrefix(key, prefix) {
			return
		}
		seen[key] = true
		out = append(out, Completion{Label: sym.Name, Kind: sym.Kind, Detail: detail(sym)})
	}
	if ws > 0 && ix.src[ws-1] == '.' {
		d := ix.first(ws - 1)
		if d < len(ix.toks) && ix.toks[d].Type == token.DOT && ix.toks[d].Pos == ws-1 {
			if m := ix.members(ix.leftOf(d)); m != nil {
				for c := m; c != nil; c = ix.baseOf(c) {
					for _, sym := range c.order {
						add(sym)
					}
				}
			}
		}
		return out
	}
	for s := ix.scopeAt(off); s != nil; s = s.parent {
		for c := s; c != nil; c = ix.baseOf(c) {
			for _, sym := range c.order {
				add(sym)
			}
		}
	}
	var rest []*Symbol
	for _, m := range []map[string]*Symbol{ix.glob, ix.named, ix.enums} {
		for _, sym := range m {
			rest = append(rest, sym)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].Name < rest[j].Name })
	for _, sym := range rest {
		add(sym)
	}
	for _, n := range standardBlocks {
		if s := ix.standardFB(n); s != nil {
			add(s.sym)
		}
	}
	for _, n := range builtinNames() {
		add(ix.standardFunction(n))
	}
	for _, k := range keywords {
		if !seen[k] && strings.HasPrefix(k, prefix) {
			seen[k] = true
			out = append(out, Completion{Label: k, Kind: "keyword"})
		}
	}
	return out
}

// baseOf is the scope of the function block c extends, or nil.
func (ix *Index) baseOf(c *scope) *scope {
	if c.sym.base == "" {
		return nil
	}
	if b := ix.named[strings.ToUpper(c.sym.base)]; b != nil && b.scope != c {
		return b.scope
	}
	return nil
}

func detail(sym *Symbol) string {
	switch {
	case sym.Kind == Variable && sym.Section != "":
		return sym.Section + " " + sym.Type
	case sym.Type != "":
		return string(sym.Kind) + " : " + sym.Type
	}
	return strings.ReplaceAll(string(sym.Kind), "_", " ")
}

// standardBlocks are the standard function blocks completion offers.
var standardBlocks = []string{"TON", "TOF", "TP", "CTU", "CTD", "CTUD", "R_TRIG", "F_TRIG", "SR", "RS"}

// keywords are the Structured Text keywords completion offers.
var keywords = []string{
	"IF", "THEN", "ELSIF", "ELSE", "END_IF", "CASE", "OF", "END_CASE", "FOR", "TO", "BY", "DO", "END_FOR",
	"WHILE", "END_WHILE", "REPEAT", "UNTIL", "END_REPEAT", "EXIT", "RETURN", "AND", "OR", "XOR", "NOT", "MOD",
	"TRUE", "FALSE", "VAR", "VAR_INPUT", "VAR_OUTPUT", "VAR_IN_OUT", "VAR_TEMP", "VAR_EXTERNAL", "VAR_GLOBAL",
	"END_VAR", "CONSTANT", "RETAIN", "PROGRAM", "END_PROGRAM", "FUNCTION", "END_FUNCTION", "FUNCTION_BLOCK",
	"END_FUNCTION_BLOCK", "CLASS", "END_CLASS", "METHOD", "END_METHOD", "TYPE", "END_TYPE", "STRUCT", "END_STRUCT", "ARRAY",
	"BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT", "REAL", "LREAL", "TIME",
	"DATE", "TIME_OF_DAY", "DATE_AND_TIME", "STRING", "WSTRING", "BYTE", "WORD", "DWORD", "LWORD",
}
