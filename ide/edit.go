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

// This file turns an editor's gestures on a diagram into text edits. The
// text is the only truth: a gesture names an element by where it starts
// (Element.At, from Diagrams), changes as little text as it can, and the
// result is read again; a gesture whose result reads worse than the
// source did (a new error) is refused, so a drawing can never be edited
// into text that does not compile for that reason.
//
// The host checks the document version: a gesture computed against a
// version that has changed since is stale and must not be applied.

import (
	"fmt"
	"slices"
	"strings"
)

// Gesture is one edit asked for on a diagram. Op says what; the other
// fields are its arguments, each op using some:
//
// LD: ld.contact (At, Where before|after|parallel, Name, Mode "" / + -),
// ld.coil (At a rung or coil, Name, Mode "" / S R P N), ld.block (At,
// Where, Name the instance, Type, Text its arguments), ld.function (At,
// Where, Type, Text, Mode "" /), ld.parallel (At to To, Name, Mode),
// ld.mode (At, Mode), ld.rename (At, Name), ld.delete (At), ld.rung (At a
// rung or none, Where before|after, Name, Text its elements),
// ld.deleteRung (At), ld.moveRung (At, Where up|down).
//
// FBD: fbd.statement (At or none, Where, Text), fbd.delete (At), fbd.move
// (At, Where up|down), fbd.input (At, Pin, Text), fbd.value (At, Text),
// fbd.target (At, Name), fbd.negate (At, Pin or "" the value).
//
// SFC: sfc.step (At a step or none, Name, Text a condition from it),
// sfc.transition (At a step, Name the step to, Text), sfc.condition (At a
// transition, Text), sfc.initial (At), sfc.qualify (At a step, Name an
// action, Mode its qualifier, Text a time), sfc.unqualify (At, Name),
// sfc.delete (At a step, transition or action).
//
// Any language: rename (At, Name) renames the symbol everywhere.
type Gesture struct {
	Op    string `json:"op"`
	At    Pos    `json:"at"`
	To    Pos    `json:"to,omitempty"`
	Where string `json:"where,omitempty"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"type,omitempty"`
	Mode  string `json:"mode,omitempty"`
	Pin   string `json:"pin,omitempty"`
	Text  string `json:"text,omitempty"`
}

// Edit replaces Span of the source by Text.
type Edit struct {
	Span Span   `json:"span"`
	Text string `json:"text"`
}

// Result is a gesture done: its edits, and the source they make.
type Result struct {
	Edits  []Edit `json:"edits"`
	Source string `json:"source"`
}

type change struct {
	start, end int
	text       string
}

// Apply does g on the source and checks the result.
func (ix *Index) Apply(g Gesture) (*Result, error) {
	var cs []change
	var err error
	switch {
	case g.Op == "rename":
		var spans []Span
		spans, err = ix.Rename(g.At, g.Name)
		for _, s := range spans {
			cs = append(cs, change{ix.offset(s.Start), ix.offset(s.End), g.Name})
		}
	case strings.HasPrefix(g.Op, "ld."):
		cs, err = ix.ldGesture(g)
	case strings.HasPrefix(g.Op, "fbd."):
		cs, err = ix.fbdGesture(g)
	case strings.HasPrefix(g.Op, "sfc."):
		cs, err = ix.sfcGesture(g)
	default:
		err = fmt.Errorf("unknown gesture %q", g.Op)
	}
	if err != nil {
		return nil, err
	}
	if len(cs) == 0 {
		return &Result{Edits: []Edit{}, Source: ix.src}, nil
	}
	slices.SortFunc(cs, func(a, b change) int { return a.start - b.start })
	for i := 1; i < len(cs); i++ {
		if cs[i].start < cs[i-1].end {
			return nil, fmt.Errorf("%s: edits overlap", g.Op)
		}
	}
	var b strings.Builder
	last := 0
	res := &Result{}
	for _, c := range cs {
		b.WriteString(ix.src[last:c.start])
		b.WriteString(c.text)
		last = c.end
		res.Edits = append(res.Edits, Edit{Span: ix.spanOf(c.start, c.end), Text: c.text})
	}
	b.WriteString(ix.src[last:])
	res.Source = b.String()
	after := Analyze(res.Source)
	if n, m := len(ix.diags), len(after.diags); m > n {
		return nil, fmt.Errorf("%s would make the source not read: %s", g.Op, newError(ix.diags, after.diags))
	}
	return res, nil
}

// newError is the first error of after that before does not have.
func newError(before, after []parserDiagnostic) string {
	seen := map[string]int{}
	for _, d := range before {
		seen[d.Message]++
	}
	for _, d := range after {
		if seen[d.Message] > 0 {
			seen[d.Message]--
			continue
		}
		return d.Message
	}
	return after[len(after)-1].Message
}

// ---- finding what a gesture names ----

// place is an element found: its rung, the series it is in, its index
// there, and the branch holding that series (nil at the rung's top).
type place struct {
	d      *Diagram
	rung   *Rung
	series []*Element
	i      int
	branch *Element
}

func (ix *Index) diagramOf(g Gesture, lang string) (*Diagram, error) {
	d := ix.DiagramAt(g.At)
	if d == nil && g.At == (Pos{}) {
		for _, x := range ix.Diagrams() {
			if x.Lang == lang {
				d = x
				break
			}
		}
	}
	if d == nil || d.Lang != lang {
		return nil, fmt.Errorf("%s: no %s body at %d:%d", g.Op, lang, g.At.Row, g.At.Column)
	}
	return d, nil
}

func (ix *Index) findElement(d *Diagram, at Pos) (*place, *Rung) {
	var walk func(r *Rung, series []*Element, branch *Element) *place
	walk = func(r *Rung, series []*Element, branch *Element) *place {
		for i, e := range series {
			if e.At == at {
				return &place{d: d, rung: r, series: series, i: i, branch: branch}
			}
			for _, leg := range e.Legs {
				if p := walk(r, leg, e); p != nil {
					return p
				}
			}
		}
		return nil
	}
	for _, r := range d.Rungs {
		if r.At == at {
			return nil, r
		}
		if p := walk(r, r.Elements, nil); p != nil {
			return p, r
		}
	}
	return nil, nil
}

// ---- LD ----

func (ix *Index) ldGesture(g Gesture) ([]change, error) {
	d, err := ix.diagramOf(g, "LD")
	if err != nil {
		return nil, err
	}
	p, rung := ix.findElement(d, g.At)
	needElement := func() error {
		if p == nil {
			return fmt.Errorf("%s: no element at %d:%d", g.Op, g.At.Row, g.At.Column)
		}
		return nil
	}
	switch g.Op {
	case "ld.contact", "ld.block", "ld.function":
		text, err := ldText(g)
		if err != nil {
			return nil, err
		}
		if p == nil && rung != nil { // on the rung: first, or before its coils
			if len(rung.Elements) == 0 {
				return []change{{rung.end, rung.end, " " + text}}, nil
			}
			first := rung.Elements[0]
			if g.Where == "after" {
				for _, e := range rung.Elements {
					if e.Kind == "coil" {
						return []change{{e.start, e.start, text + " "}}, nil
					}
				}
				last := rung.Elements[len(rung.Elements)-1]
				return []change{{last.end, last.end, " " + text}}, nil
			}
			return []change{{first.start, first.start, text + " "}}, nil
		}
		if err := needElement(); err != nil {
			return nil, err
		}
		e := p.series[p.i]
		switch g.Where {
		case "", "after":
			if e.Kind == "coil" {
				return nil, fmt.Errorf("%s: a rung's coils come last; put it before the coil", g.Op)
			}
			return []change{{e.end, e.end, " " + text}}, nil
		case "before":
			return []change{{e.start, e.start, text + " "}}, nil
		case "parallel":
			if e.Kind == "coil" {
				return nil, fmt.Errorf("%s: a coil has no parallel contact; add a coil instead", g.Op)
			}
			if e.Kind == "branch" { // one more leg
				return []change{{e.end - 1, e.end - 1, "| " + text + " "}}, nil
			}
			return []change{{e.start, e.end, "[ " + e.Text + " | " + text + " ]"}}, nil
		}
		return nil, fmt.Errorf("%s: where is before, after or parallel, not %q", g.Op, g.Where)
	case "ld.parallel":
		if err := needElement(); err != nil {
			return nil, err
		}
		text, err := ldText(Gesture{Op: "ld.contact", Name: g.Name, Mode: g.Mode, Type: g.Type, Text: g.Text})
		if err != nil {
			return nil, err
		}
		j := -1
		for k, e := range p.series {
			if e.At == g.To {
				j = k
			}
		}
		if j < p.i {
			return nil, fmt.Errorf("%s: %d:%d is not after %d:%d in the same series", g.Op, g.To.Row, g.To.Column, g.At.Row, g.At.Column)
		}
		for _, e := range p.series[p.i : j+1] {
			if e.Kind == "coil" {
				return nil, fmt.Errorf("%s: a branch holds contacts and blocks, not coils", g.Op)
			}
		}
		a, b := p.series[p.i].start, p.series[j].end
		return []change{{a, b, "[ " + ix.src[a:b] + " | " + text + " ]"}}, nil
	case "ld.coil":
		if g.Name == "" {
			return nil, fmt.Errorf("ld.coil: a coil writes a variable: name it")
		}
		text, err := coilText(g.Name, g.Mode)
		if err != nil {
			return nil, err
		}
		if rung == nil {
			return nil, fmt.Errorf("ld.coil: no rung at %d:%d", g.At.Row, g.At.Column)
		}
		if len(rung.Elements) == 0 {
			return []change{{rung.end, rung.end, " " + text}}, nil
		}
		last := rung.Elements[len(rung.Elements)-1]
		return []change{{last.end, last.end, " " + text}}, nil
	case "ld.mode":
		if err := needElement(); err != nil {
			return nil, err
		}
		e := p.series[p.i]
		switch e.Kind {
		case "contact":
			if !slices.Contains([]string{"", "/", "+", "-"}, g.Mode) {
				return nil, fmt.Errorf("ld.mode: a contact is \"\", / (negated), + or - (an edge), not %q", g.Mode)
			}
			return []change{{e.start, e.end, g.Mode + e.Name}}, nil
		case "function":
			if g.Mode != "" && g.Mode != "/" {
				return nil, fmt.Errorf("ld.mode: a function contact is \"\" or / (negated), not %q", g.Mode)
			}
			return []change{{e.start, e.end, g.Mode + strings.TrimPrefix(e.Text, e.Mode)}}, nil
		case "coil":
			text, err := coilText(e.Name, g.Mode)
			if err != nil {
				return nil, err
			}
			return []change{{e.start, e.end, text}}, nil
		}
		return nil, fmt.Errorf("ld.mode: a %s has no mode", e.Kind)
	case "ld.rename":
		if err := needElement(); err != nil {
			return nil, err
		}
		if !isDotted(g.Name) {
			return nil, fmt.Errorf("ld.rename: %q is not a variable", g.Name)
		}
		e := p.series[p.i]
		switch e.Kind {
		case "contact":
			return []change{{e.start, e.end, e.Mode + g.Name}}, nil
		case "coil":
			text, _ := coilText(g.Name, e.Mode)
			return []change{{e.start, e.end, text}}, nil
		case "block":
			at := e.start + len(e.Mode)
			return []change{{at, at + len(e.Name), g.Name}}, nil
		}
		return nil, fmt.Errorf("ld.rename: a %s has no variable of its own", e.Kind)
	case "ld.delete":
		if err := needElement(); err != nil {
			return nil, err
		}
		e := p.series[p.i]
		if p.branch != nil {
			var legs []string
			for _, leg := range p.branch.Legs {
				var parts []string
				for _, x := range leg {
					if x != e {
						parts = append(parts, x.Text)
					}
				}
				if len(parts) > 0 {
					legs = append(legs, strings.Join(parts, " "))
				}
			}
			text := "[ " + strings.Join(legs, " | ") + " ]"
			if len(legs) == 1 {
				text = legs[0]
			}
			return []change{{p.branch.start, p.branch.end, text}}, nil
		}
		if p.i+1 < len(p.series) {
			return []change{{e.start, p.series[p.i+1].start, ""}}, nil
		}
		if p.i > 0 {
			return []change{{p.series[p.i-1].end, e.end, ""}}, nil
		}
		return []change{{e.start, e.end, ""}}, nil
	case "ld.rung":
		if g.Text == "" {
			return nil, fmt.Errorf("ld.rung: a rung needs its elements, ending in a coil or a block, e.g. \"Start ( Run )\"")
		}
		head := "RUNG"
		if g.Name != "" {
			if !isWord(g.Name) {
				return nil, fmt.Errorf("ld.rung: %q is not a name", g.Name)
			}
			head += " " + g.Name
		}
		nl := ix.newline()
		rungIndent, elemIndent := "  ", "    "
		if len(d.Rungs) > 0 {
			rungIndent = ix.indentOf(d.Rungs[0].start)
			if len(d.Rungs[0].Elements) > 0 {
				elemIndent = ix.indentOf(d.Rungs[0].Elements[0].start)
			}
		}
		block := rungIndent + head + nl + elemIndent + g.Text
		switch {
		case rung != nil && g.Where == "before":
			at := ix.lineStart(rung.start)
			return []change{{at, at, block + nl}}, nil
		case rung != nil:
			at := ix.lineEnd(rung.end)
			return []change{{at, at, nl + block}}, nil
		case len(d.Rungs) > 0:
			at := ix.lineEnd(d.Rungs[len(d.Rungs)-1].end)
			return []change{{at, at, nl + block}}, nil
		}
		return []change{{d.start, d.start, nl + block}}, nil
	case "ld.deleteRung":
		if rung == nil || p != nil {
			return nil, fmt.Errorf("ld.deleteRung: no rung at %d:%d", g.At.Row, g.At.Column)
		}
		return []change{{ix.lineStart(rung.start), ix.lineAfter(rung.end), ""}}, nil
	case "ld.moveRung":
		if rung == nil || p != nil {
			return nil, fmt.Errorf("ld.moveRung: no rung at %d:%d", g.At.Row, g.At.Column)
		}
		k := slices.Index(d.Rungs, rung)
		other := k - 1
		if g.Where == "down" {
			other = k + 1
		} else if g.Where != "up" {
			return nil, fmt.Errorf("ld.moveRung: where is up or down, not %q", g.Where)
		}
		if other < 0 || other >= len(d.Rungs) {
			return nil, nil // already first or last
		}
		a, b := d.Rungs[min(k, other)], d.Rungs[max(k, other)]
		return ix.swapLines(a.start, a.end, b.start, b.end), nil
	}
	return nil, fmt.Errorf("unknown gesture %q", g.Op)
}

// ldText is the text of the element an ld.contact, ld.block or
// ld.function gesture adds.
func ldText(g Gesture) (string, error) {
	switch g.Op {
	case "ld.contact":
		if !isDotted(g.Name) {
			return "", fmt.Errorf("ld.contact: %q is not a variable", g.Name)
		}
		if !slices.Contains([]string{"", "/", "+", "-"}, g.Mode) {
			return "", fmt.Errorf("ld.contact: mode is \"\", / (negated), + or - (an edge), not %q", g.Mode)
		}
		return g.Mode + g.Name, nil
	case "ld.block":
		if !isWord(g.Name) || !isWord(g.Type) {
			return "", fmt.Errorf("ld.block: a block is an instance name and a type, e.g. t1 and TON")
		}
		if err := oneExpression(g.Text); err != nil {
			return "", fmt.Errorf("ld.block: %w", err)
		}
		return g.Name + ":" + g.Type + "(" + g.Text + ")", nil
	case "ld.function":
		if !isWord(g.Type) {
			return "", fmt.Errorf("ld.function: %q is not a function", g.Type)
		}
		if g.Mode != "" && g.Mode != "/" {
			return "", fmt.Errorf("ld.function: mode is \"\" or /, not %q", g.Mode)
		}
		if err := oneExpression(g.Text); err != nil {
			return "", fmt.Errorf("ld.function: %w", err)
		}
		return g.Mode + g.Type + "(" + g.Text + ")", nil
	}
	return "", fmt.Errorf("unknown gesture %q", g.Op)
}

// coilText is a coil writing name in mode.
func coilText(name, mode string) (string, error) {
	if !isDotted(name) {
		return "", fmt.Errorf("%q is not a variable", name)
	}
	switch mode {
	case "":
		return "( " + name + " )", nil
	case "/":
		return "( /" + name + " )", nil
	case "S", "R", "P", "N":
		return "( " + mode + " " + name + " )", nil
	}
	return "", fmt.Errorf("a coil is \"\", /, S, R, P or N, not %q", mode)
}

// ---- FBD ----

func (ix *Index) fbdGesture(g Gesture) ([]change, error) {
	d, err := ix.diagramOf(g, "FBD")
	if err != nil {
		return nil, err
	}
	var st *Statement
	k := -1
	for i, s := range d.Statements {
		if s.At == g.At {
			st, k = s, i
		}
	}
	need := func() error {
		if st == nil {
			return fmt.Errorf("%s: no statement at %d:%d", g.Op, g.At.Row, g.At.Column)
		}
		return nil
	}
	switch g.Op {
	case "fbd.statement":
		if err := oneExpression(g.Text); err != nil {
			return nil, fmt.Errorf("fbd.statement: %w", err)
		}
		nl := ix.newline()
		indent := "  "
		if len(d.Statements) > 0 {
			indent = ix.indentOf(d.Statements[0].start)
		}
		switch {
		case st != nil && g.Where == "before":
			at := ix.lineStart(st.start)
			return []change{{at, at, indent + g.Text + nl}}, nil
		case st != nil:
			at := ix.lineEnd(st.end)
			return []change{{at, at, nl + indent + g.Text}}, nil
		case len(d.Statements) > 0:
			at := ix.lineEnd(d.Statements[len(d.Statements)-1].end)
			return []change{{at, at, nl + indent + g.Text}}, nil
		}
		return []change{{d.start, d.start, nl + indent + g.Text}}, nil
	case "fbd.delete":
		if err := need(); err != nil {
			return nil, err
		}
		return []change{{ix.lineStart(st.start), ix.lineAfter(st.end), ""}}, nil
	case "fbd.move":
		if err := need(); err != nil {
			return nil, err
		}
		other := k - 1
		if g.Where == "down" {
			other = k + 1
		} else if g.Where != "up" {
			return nil, fmt.Errorf("fbd.move: where is up or down, not %q", g.Where)
		}
		if other < 0 || other >= len(d.Statements) {
			return nil, nil
		}
		a, b := d.Statements[min(k, other)], d.Statements[max(k, other)]
		return ix.swapLines(a.start, a.end, b.start, b.end), nil
	case "fbd.input":
		if err := need(); err != nil {
			return nil, err
		}
		if err := oneExpression(g.Text); err != nil || strings.TrimSpace(g.Text) == "" {
			return nil, fmt.Errorf("fbd.input: %q is not a value", g.Text)
		}
		for _, a := range st.Args {
			if strings.EqualFold(a.Pin, g.Pin) {
				return []change{{a.vstart, a.vend, g.Text}}, nil
			}
		}
		if st.Call == "" {
			return nil, fmt.Errorf("fbd.input: the statement at %d:%d calls nothing", g.At.Row, g.At.Column)
		}
		close := ix.closingParen(st)
		if close < 0 {
			return nil, fmt.Errorf("fbd.input: the call has no )")
		}
		positional := st.Kind != "call"
		if positional {
			if g.Pin != itoa(len(st.Args)+1) {
				return nil, fmt.Errorf("fbd.input: %s has %d inputs; the next is %d", st.Call, len(st.Args), len(st.Args)+1)
			}
			if len(st.Args) == 0 {
				return []change{{close, close, g.Text}}, nil
			}
			return []change{{close, close, ", " + g.Text}}, nil
		}
		if !isWord(g.Pin) {
			return nil, fmt.Errorf("fbd.input: %q is not an input", g.Pin)
		}
		if len(st.Args) == 0 {
			return []change{{close, close, g.Pin + " := " + g.Text}}, nil
		}
		return []change{{close, close, ", " + g.Pin + " := " + g.Text}}, nil
	case "fbd.value":
		if err := need(); err != nil {
			return nil, err
		}
		if st.Kind != "assign" && st.Kind != "wire" {
			return nil, fmt.Errorf("fbd.value: a %s has no value", st.Kind)
		}
		if err := oneExpression(g.Text); err != nil || strings.TrimSpace(g.Text) == "" {
			return nil, fmt.Errorf("fbd.value: %q is not a value", g.Text)
		}
		return []change{{st.vstart, st.vend, g.Text}}, nil
	case "fbd.target":
		if err := need(); err != nil {
			return nil, err
		}
		if !isDotted(g.Name) {
			return nil, fmt.Errorf("fbd.target: %q is not a name", g.Name)
		}
		return []change{{st.start, st.start + len(st.Target), g.Name}}, nil
	case "fbd.negate":
		if err := need(); err != nil {
			return nil, err
		}
		a, b := st.vstart, st.vend
		if g.Pin != "" {
			found := false
			for _, x := range st.Args {
				if strings.EqualFold(x.Pin, g.Pin) {
					a, b, found = x.vstart, x.vend, true
				}
			}
			if !found {
				return nil, fmt.Errorf("fbd.negate: no input %s", g.Pin)
			}
		} else if st.Kind != "assign" && st.Kind != "wire" {
			return nil, fmt.Errorf("fbd.negate: name the input to negate")
		}
		return []change{{a, b, negate(ix.src[a:b])}}, nil
	}
	return nil, fmt.Errorf("unknown gesture %q", g.Op)
}

// negate is NOT v, or v when it is NOT of something already.
func negate(v string) string {
	u := strings.ToUpper(v)
	if strings.HasPrefix(u, "NOT(") && strings.HasSuffix(v, ")") && balanced(v[4:len(v)-1]) {
		return v[4 : len(v)-1]
	}
	if strings.HasPrefix(u, "NOT ") && isDotted(strings.TrimSpace(v[4:])) {
		return strings.TrimSpace(v[4:])
	}
	return "NOT(" + v + ")"
}

func balanced(s string) bool {
	depth := 0
	for _, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// closingParen is the offset of the ) closing a statement's call.
func (ix *Index) closingParen(st *Statement) int {
	if len(st.Args) > 0 {
		last := st.Args[len(st.Args)-1]
		for i := ix.first(last.end); i < len(ix.toks) && ix.toks[i].Pos <= st.end; i++ {
			if ix.toks[i].Literal == ")" {
				return ix.toks[i].Pos
			}
		}
		return -1
	}
	for i := ix.first(st.start); i < len(ix.toks) && ix.toks[i].Pos <= st.end; i++ {
		if ix.toks[i].Literal == "(" && i+1 < len(ix.toks) && ix.toks[i+1].Literal == ")" {
			return ix.toks[i+1].Pos
		}
	}
	return -1
}

// ---- SFC ----

func (ix *Index) sfcGesture(g Gesture) ([]change, error) {
	d, err := ix.diagramOf(g, "SFC")
	if err != nil {
		return nil, err
	}
	var step *SFCStep
	var tr *SFCTransition
	var act *SFCAction
	for _, s := range d.Steps {
		if s.At == g.At {
			step = s
		}
	}
	for _, t := range d.Transits {
		if t.At == g.At {
			tr = t
		}
	}
	for _, a := range d.Actions {
		if a.At == g.At {
			act = a
		}
	}
	nl := ix.newline()
	needStep := func() error {
		if step == nil {
			return fmt.Errorf("%s: no step at %d:%d", g.Op, g.At.Row, g.At.Column)
		}
		return nil
	}
	transition := func(from, to, cond string) (string, error) {
		if err := oneExpression(cond); err != nil || strings.TrimSpace(cond) == "" {
			return "", fmt.Errorf("%s: %q is not a condition", g.Op, cond)
		}
		return "TRANSITION FROM " + from + " TO " + to + " := " + cond + "; END_TRANSITION", nil
	}
	switch g.Op {
	case "sfc.step":
		if !isWord(g.Name) {
			return nil, fmt.Errorf("sfc.step: %q is not a name", g.Name)
		}
		text := "STEP " + g.Name + ": END_STEP"
		at := d.end
		indent := ""
		if step != nil {
			at, indent = ix.lineEnd(step.end), ix.indentOf(step.start)
			if g.Text != "" {
				t, err := transition(step.Name, g.Name, g.Text)
				if err != nil {
					return nil, err
				}
				text = t + nl + indent + text
			}
		} else if len(d.Steps) > 0 {
			last := d.Steps[len(d.Steps)-1]
			at, indent = ix.lineEnd(last.end), ix.indentOf(last.start)
		}
		return []change{{at, at, nl + indent + text}}, nil
	case "sfc.transition":
		if err := needStep(); err != nil {
			return nil, err
		}
		if !isWord(g.Name) {
			return nil, fmt.Errorf("sfc.transition: %q is not a step", g.Name)
		}
		t, err := transition(step.Name, g.Name, g.Text)
		if err != nil {
			return nil, err
		}
		at := ix.lineEnd(step.end)
		return []change{{at, at, nl + ix.indentOf(step.start) + t}}, nil
	case "sfc.condition":
		if tr == nil {
			return nil, fmt.Errorf("sfc.condition: no transition at %d:%d", g.At.Row, g.At.Column)
		}
		if err := oneExpression(g.Text); err != nil || strings.TrimSpace(g.Text) == "" {
			return nil, fmt.Errorf("sfc.condition: %q is not a condition", g.Text)
		}
		return []change{{tr.cstart, tr.cend, g.Text}}, nil
	case "sfc.initial":
		if err := needStep(); err != nil {
			return nil, err
		}
		if step.Initial {
			return nil, nil
		}
		cs := []change{{step.start, step.start + len("STEP"), "INITIAL_STEP"}}
		for _, s := range d.Steps {
			if s.Initial {
				cs = append(cs, change{s.start, s.start + len("INITIAL_STEP"), "STEP"})
			}
		}
		return cs, nil
	case "sfc.qualify", "sfc.unqualify":
		if err := needStep(); err != nil {
			return nil, err
		}
		if !isWord(g.Name) {
			return nil, fmt.Errorf("%s: %q is not an action", g.Op, g.Name)
		}
		a, b := ix.association(step, g.Name)
		if g.Op == "sfc.unqualify" {
			if a < 0 {
				return nil, fmt.Errorf("sfc.unqualify: %s has no action %s", step.Name, g.Name)
			}
			for b < step.end && (ix.src[b] == ' ' || ix.src[b] == '\t') {
				b++
			}
			return []change{{a, b, ""}}, nil
		}
		q := strings.ToUpper(g.Mode)
		if q == "" {
			q = "N"
		}
		if !slices.Contains([]string{"N", "R", "S", "P", "P0", "P1", "L", "D", "SD", "DS", "SL"}, q) {
			return nil, fmt.Errorf("sfc.qualify: %q is not a qualifier (N, R, S, P, P0, P1, L, D, SD, DS, SL)", g.Mode)
		}
		timed := slices.Contains([]string{"L", "D", "SD", "DS", "SL"}, q)
		if timed != (strings.TrimSpace(g.Text) != "") {
			if timed {
				return nil, fmt.Errorf("sfc.qualify: %s needs a time, e.g. T#5S", q)
			}
			return nil, fmt.Errorf("sfc.qualify: %s takes no time", q)
		}
		text := g.Name + "(" + q
		if timed {
			if err := oneExpression(g.Text); err != nil {
				return nil, fmt.Errorf("sfc.qualify: %w", err)
			}
			text += ", " + strings.TrimSpace(g.Text)
		}
		text += ");"
		if a >= 0 {
			return []change{{a, b, text}}, nil
		}
		end := step.end - len("END_STEP")
		return []change{{end, end, text + " "}}, nil
	case "sfc.delete":
		switch {
		case step != nil:
			cs := []change{{ix.lineStart(step.start), ix.lineAfter(step.end), ""}}
			for _, t := range d.Transits {
				if containsFold(t.From, step.Name) || containsFold(t.To, step.Name) {
					cs = append(cs, change{ix.lineStart(t.start), ix.lineAfter(t.end), ""})
				}
			}
			return cs, nil
		case tr != nil:
			return []change{{ix.lineStart(tr.start), ix.lineAfter(tr.end), ""}}, nil
		case act != nil:
			return []change{{ix.lineStart(act.start), ix.lineAfter(act.end), ""}}, nil
		}
		return nil, fmt.Errorf("sfc.delete: nothing at %d:%d", g.At.Row, g.At.Column)
	}
	return nil, fmt.Errorf("unknown gesture %q", g.Op)
}

// association is the text of step's Name(Q...); in its step, [a, b), or
// -1.
func (ix *Index) association(step *SFCStep, name string) (int, int) {
	for i := ix.first(step.start); i+1 < len(ix.toks) && ix.toks[i].Pos < step.end; i++ {
		t := ix.toks[i]
		if !strings.EqualFold(t.Literal, name) || ix.toks[i+1].Literal != "(" || ix.tok(i-1).Literal == "STEP" || ix.tok(i-1).Literal == "INITIAL_STEP" {
			continue
		}
		j := i + 1
		for j < len(ix.toks) && ix.toks[j].Literal != ")" {
			j++
		}
		end := ix.toks[j].Pos + 1
		if j+1 < len(ix.toks) && ix.toks[j+1].Literal == ";" {
			end = ix.toks[j+1].Pos + 1
		}
		return t.Pos, end
	}
	return -1, -1
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// ---- text helpers ----

// newline is the source's line ending.
func (ix *Index) newline() string {
	if strings.Contains(ix.src, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// indentOf is the white space starting off's line.
func (ix *Index) indentOf(off int) string {
	s := ix.lineStart(off)
	e := s
	for e < len(ix.src) && (ix.src[e] == ' ' || ix.src[e] == '\t') {
		e++
	}
	return ix.src[s:e]
}

// lineStart is the offset off's line starts at.
func (ix *Index) lineStart(off int) int { return strings.LastIndex(ix.src[:off], "\n") + 1 }

// lineEnd is the offset off's line ends at, before its line ending.
func (ix *Index) lineEnd(off int) int {
	i := strings.IndexByte(ix.src[off:], '\n')
	if i < 0 {
		return len(ix.src)
	}
	end := off + i
	if end > 0 && ix.src[end-1] == '\r' {
		end--
	}
	return end
}

// lineAfter is the offset after off's line and its line ending.
func (ix *Index) lineAfter(off int) int {
	i := strings.IndexByte(ix.src[off:], '\n')
	if i < 0 {
		return len(ix.src)
	}
	return off + i + 1
}

// swapLines swaps the whole lines of [a0, a1) and [b0, b1), a before b.
func (ix *Index) swapLines(a0, a1, b0, b1 int) []change {
	as, ae := ix.lineStart(a0), ix.lineEnd(a1)
	bs, be := ix.lineStart(b0), ix.lineEnd(b1)
	return []change{{as, be, ix.src[bs:be] + ix.src[ae:bs] + ix.src[as:ae]}}
}

// isDotted says whether s is a name or a dotted name, a.b.
func isDotted(s string) bool {
	for _, part := range strings.Split(s, ".") {
		if !isWord(part) {
			return false
		}
	}
	return true
}

// oneLine refuses text that would end a statement, start a comment or a
// new line: a gesture's text is one piece of one line.
func oneLine(text string) error {
	quote := byte(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			if c == '$' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
		case c == '\n' || c == '\r':
			return fmt.Errorf("%q has a line break", text)
		case strings.HasPrefix(text[i:], "(*") || strings.HasPrefix(text[i:], "//"):
			return fmt.Errorf("%q has a comment", text)
		}
	}
	if quote != 0 {
		return fmt.Errorf("%q has a string not closed", text)
	}
	return nil
}

// oneExpression is oneLine, without ; either.
func oneExpression(text string) error {
	if err := oneLine(text); err != nil {
		return err
	}
	quote := byte(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			if c == '$' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		} else if c == ';' {
			return fmt.Errorf("%q has a ;", text)
		}
	}
	return nil
}
