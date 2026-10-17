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

// This file gives an editor the source's graphical bodies as models: an
// LD body's rungs, an FBD body's statements, an SFC body's steps,
// transitions and actions, each element with the text it is (Span) and
// the place it starts (At), by which a gesture names it (edit.go). The
// text stays the only truth: a model is read from it, never stored.

import (
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/diagram"
	"github.com/apiarytech/beedance/token"
)

// Diagram is one graphical body of a source.
type Diagram struct {
	POU  string `json:"pou"`
	Lang string `json:"lang"` // LD, FBD or SFC
	// Body is the body's text: from after LD (FBD) to END_LD (END_FBD); an
	// SFC POU's steps, transitions and actions.
	Body       Span             `json:"body"`
	Rungs      []*Rung          `json:"rungs,omitempty"`
	Statements []*Statement     `json:"statements,omitempty"`
	Steps      []*SFCStep       `json:"steps,omitempty"`
	Transits   []*SFCTransition `json:"transitions,omitempty"`
	Actions    []*SFCAction     `json:"actions,omitempty"`
	// SVG is the body drawn; an LD or FBD element's group names its place,
	// data-at="row:col" (diagram.SVGText), an SFC step its name.
	SVG string `json:"svg,omitempty"`
	// Error is why the body does not read, when it does not; the model is
	// then as far as it reads.
	Error string `json:"error,omitempty"`

	start, end int // byte offsets of Body
}

// Rung is one rung of an LD body.
type Rung struct {
	Name string `json:"name,omitempty"`
	At   Pos    `json:"at"`   // its RUNG keyword
	Span Span   `json:"span"` // RUNG to its last element
	// Elements are the rung's series, left to right, coils last.
	Elements []*Element `json:"elements"`

	start, end int
}

// Element is a rung's element.
type Element struct {
	// Kind is contact, function (a function's result as a contact), block
	// (a function block called), branch or coil.
	Kind string `json:"kind"`
	At   Pos    `json:"at"`
	Span Span   `json:"span"`
	// Name is a contact's or coil's variable, a block's instance.
	Name string `json:"name,omitempty"`
	// Type is a block's type, a function's name.
	Type string `json:"type,omitempty"`
	// Mode is a contact's "", "/" (negated), "+" or "-" (an edge); a coil's
	// "", "/", "S", "R", "P" or "N".
	Mode string `json:"mode,omitempty"`
	// Args are a block's or function's inputs and outputs.
	Args []*Arg `json:"args,omitempty"`
	// Legs are a branch's legs, each a series.
	Legs [][]*Element `json:"legs,omitempty"`
	Text string       `json:"text"` // as written
	// Ref is the place the drawing names the element by, its name's
	// "row:col" (the SVG's data-at); empty for a branch.
	Ref string `json:"ref,omitempty"`

	start, end, ref int
}

// Arg is an input or output of a call: Pin := Value, Pin => Value, or a
// positional Value (Pin "1", "2", ...).
type Arg struct {
	Pin    string `json:"pin"`
	Output bool   `json:"output,omitempty"` // =>
	Value  string `json:"value"`
	Span   Span   `json:"span"`      // the whole argument
	Val    Span   `json:"valueSpan"` // its value

	start, end, vstart, vend int
}

// Statement is one statement of an FBD body.
type Statement struct {
	// Kind is assign (X := value), wire (w = value) or call (inst :
	// TYPE(...) or inst(...)).
	Kind   string `json:"kind"`
	At     Pos    `json:"at"`
	Span   Span   `json:"span"`
	Target string `json:"target"` // the variable, wire or instance
	Type   string `json:"type,omitempty"`
	// Value is what an assign or a wire takes, as written.
	Value string `json:"value,omitempty"`
	// Call is the function or block called at the top: an assign's or a
	// wire's function (its Args positional), or the call.
	Call string `json:"call,omitempty"`
	Args []*Arg `json:"args,omitempty"`
	Text string `json:"text"`

	start, end, vstart, vend int
}

// SFCStep is a step of a chart.
type SFCStep struct {
	Name    string        `json:"name"`
	NameAt  Pos           `json:"nameAt"` // the name, where a rename names it
	Initial bool          `json:"initial,omitempty"`
	At      Pos           `json:"at"`
	Span    Span          `json:"span"`
	Actions []*SFCQualify `json:"actions,omitempty"`

	start, end int
}

// SFCQualify is an action associated with a step: Action(Qualifier[, Time]).
type SFCQualify struct {
	Action    string `json:"action"`
	Qualifier string `json:"qualifier"`
	Time      string `json:"time,omitempty"`
}

// SFCTransition is a transition of a chart.
type SFCTransition struct {
	From      []string `json:"from"`
	To        []string `json:"to"`
	Condition string   `json:"condition"`
	At        Pos      `json:"at"`
	Span      Span     `json:"span"`
	Cond      Span     `json:"conditionSpan"`

	start, end, cstart, cend int
}

// SFCAction is an action declared for a chart.
type SFCAction struct {
	Name string `json:"name"`
	At   Pos    `json:"at"`
	Span Span   `json:"span"`

	start, end int
}

type chartOf struct {
	s     *scope
	chart *ast.SFCProgram
}

// Diagrams are the source's graphical bodies in source order.
func (ix *Index) Diagrams() []*Diagram {
	var out []*Diagram
	for _, b := range ix.bodies {
		out = append(out, ix.textDiagram(b.POU, b.Lang, b.Text, b.Row))
	}
	for _, c := range ix.charts {
		out = append(out, ix.chartDiagram(c))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// DiagramAt is the graphical body p is in.
func (ix *Index) DiagramAt(p Pos) *Diagram {
	off := ix.offset(p)
	for _, d := range ix.Diagrams() {
		if d.start <= off && off <= d.end {
			return d
		}
	}
	return nil
}

// textDiagram models an LD or FBD body.
func (ix *Index) textDiagram(pou, lang, text string, row int) *Diagram {
	d := &Diagram{POU: pou, Lang: strings.ToUpper(lang)}
	for _, t := range ix.toks {
		if t.Row == row && strings.EqualFold(t.Literal, lang) {
			d.start = t.Pos + len(t.Literal)
			break
		}
	}
	d.end = d.start + len(text)
	d.Body = ix.spanOf(d.start, d.end)
	if t, err := diagram.ParseTextAt(lang, text, row, ix.posOf(d.start).Column); err != nil {
		d.Error = err.Error()
	} else {
		var wires map[diagram.Conn]string
		if low, err := diagram.LowerText(pou, lang, text, row, diagram.Options{}); err == nil {
			wires = low.Wires
		}
		d.SVG = diagram.SVGText(t.Elems, wires)
	}
	a, b := ix.first(d.start), ix.first(d.end)
	if d.Lang == "LD" {
		d.Rungs = ix.rungs(a, b)
	} else {
		d.Statements = ix.statements(a, b)
	}
	return d
}

// ---- LD ----

// rungs reads tokens [a, b) as rungs.
func (ix *Index) rungs(a, b int) []*Rung {
	var out []*Rung
	for i := a; i < b; {
		if !strings.EqualFold(ix.toks[i].Literal, "RUNG") {
			i++
			continue
		}
		r := &Rung{At: ix.posOf(ix.toks[i].Pos), start: ix.toks[i].Pos}
		i++
		// A word right after RUNG, on its line, is the rung's name, as
		// package diagram reads it.
		if i < b && isWord(ix.toks[i].Literal) && ix.toks[i].Row == ix.toks[i-1].Row && !strings.EqualFold(ix.toks[i].Literal, "RUNG") {
			r.Name = ix.toks[i].Literal
			i++
		}
		r.end = ix.toks[i-1].Pos + len(ix.toks[i-1].Literal)
		r.Elements, i = ix.series(i, b)
		if len(r.Elements) > 0 {
			r.end = r.Elements[len(r.Elements)-1].end
		}
		for i < b && !strings.EqualFold(ix.toks[i].Literal, "RUNG") { // what does not read
			i++
		}
		r.Span = ix.spanOf(r.start, r.end)
		out = append(out, r)
	}
	return out
}

// series reads elements from token i until a branch's | or ], the next
// RUNG or b; it returns them and where it stopped.
func (ix *Index) series(i, b int) ([]*Element, int) {
	var out []*Element
	for i < b {
		t := ix.toks[i]
		if t.Literal == "|" || t.Type == token.RBRACKET || strings.EqualFold(t.Literal, "RUNG") {
			break
		}
		e, next := ix.element(i, b)
		if e == nil {
			break
		}
		out = append(out, e)
		i = next
	}
	return out, i
}

// element reads one element at token i.
func (ix *Index) element(i, b int) (*Element, int) {
	t := ix.toks[i]
	e := &Element{start: t.Pos, ref: -1}
	switch {
	case t.Type == token.LBRACKET:
		e.Kind = "branch"
		i++
		for i < b {
			leg, next := ix.series(i, b)
			e.Legs = append(e.Legs, leg)
			i = next
			if i < b && ix.toks[i].Literal == "|" {
				i++
				continue
			}
			break
		}
		if i < b && ix.toks[i].Type == token.RBRACKET {
			i++
		}
	case t.Type == token.LPAREN && !ix.adjacent(i-1, i): // a coil
		e.Kind = "coil"
		i++
		if i < b && ix.toks[i].Type == token.SLASH {
			e.Mode = "/"
			i++
		} else if i+1 < b && isWord(ix.toks[i].Literal) && isWord(ix.toks[i+1].Literal) && strings.Contains("SRPN", strings.ToUpper(ix.toks[i].Literal)) && len(ix.toks[i].Literal) == 1 {
			e.Mode = strings.ToUpper(ix.toks[i].Literal)
			i++
		}
		e.ref = e.start // a coil is drawn from its (
		if i < b && isWord(ix.toks[i].Literal) {
			e.Name = ix.dotted(&i, b)
		}
		for i < b && ix.toks[i].Type != token.RPAREN && !strings.EqualFold(ix.toks[i].Literal, "RUNG") {
			i++
		}
		if i < b && ix.toks[i].Type == token.RPAREN {
			i++
		}
	default:
		if t.Type == token.SLASH || t.Type == token.PLUS || t.Type == token.MINUS {
			e.Mode = t.Literal
			i++
			if i >= b {
				return nil, i
			}
		}
		if !isWord(ix.toks[i].Literal) {
			return nil, i
		}
		e.ref = ix.toks[i].Pos
		switch {
		case i+2 < b && ix.toks[i+1].Type == token.COLON && isWord(ix.toks[i+2].Literal): // inst:TYPE(...)
			e.Kind, e.Name, e.Type = "block", ix.toks[i].Literal, ix.toks[i+2].Literal
			i += 3
			if i < b && ix.toks[i].Type == token.LPAREN {
				e.Args, i = ix.args(i, b, false)
			}
		case i+1 < b && ix.toks[i+1].Type == token.LPAREN && ix.adjacent(i, i+1): // EQ(a, b)
			e.Kind, e.Type = "function", ix.toks[i].Literal
			e.Args, i = ix.args(i+1, b, true)
		default:
			e.Kind = "contact"
			e.Name = ix.dotted(&i, b)
		}
	}
	e.end = ix.toks[i-1].Pos + len(ix.toks[i-1].Literal)
	e.At, e.Span, e.Text = ix.posOf(e.start), ix.spanOf(e.start, e.end), ix.src[e.start:e.end]
	if e.ref >= 0 {
		p := ix.posOf(e.ref)
		e.Ref = itoa(p.Row) + ":" + itoa(p.Column)
	}
	return e, i
}

// dotted reads a name, a.b.c, at token *i.
func (ix *Index) dotted(i *int, b int) string {
	start := ix.toks[*i].Pos
	*i++
	for *i+1 < b && ix.toks[*i].Type == token.DOT && ix.adjacent(*i-1, *i) && isWord(ix.toks[*i+1].Literal) {
		*i += 2
	}
	return ix.src[start : ix.toks[*i-1].Pos+len(ix.toks[*i-1].Literal)]
}

// adjacent says whether token j starts where token i ends.
func (ix *Index) adjacent(i, j int) bool {
	if i < 0 || j >= len(ix.toks) {
		return false
	}
	return ix.toks[i].Pos+len(ix.toks[i].Literal) == ix.toks[j].Pos
}

// args reads a call's arguments, token i its (; positional numbers the
// unnamed ones 1, 2, ... It returns them and the token after the ).
func (ix *Index) args(i, b int, positional bool) ([]*Arg, int) {
	var out []*Arg
	i++ // (
	n := 0
	for i < b && ix.toks[i].Type != token.RPAREN {
		a := &Arg{start: ix.toks[i].Pos}
		if i+1 < b && isWord(ix.toks[i].Literal) && (ix.toks[i+1].Type == token.ASSIGN || ix.toks[i+1].Type == token.ARROW) {
			a.Pin, a.Output = ix.toks[i].Literal, ix.toks[i+1].Type == token.ARROW
			i += 2
		} else {
			n++
			a.Pin = itoa(n)
		}
		a.vstart = a.start
		if i < b {
			a.vstart = ix.toks[i].Pos
		}
		depth := 0
		for i < b {
			t := ix.toks[i]
			if depth == 0 && (t.Type == token.COMMA || t.Type == token.RPAREN) {
				break
			}
			switch t.Type {
			case token.LPAREN, token.LBRACKET:
				depth++
			case token.RPAREN, token.RBRACKET:
				depth--
			}
			i++
		}
		a.end = ix.toks[i-1].Pos + len(ix.toks[i-1].Literal)
		a.vend = a.end
		if a.vend < a.vstart {
			a.vend = a.vstart
		}
		a.Value = ix.src[a.vstart:a.vend]
		a.Span, a.Val = ix.spanOf(a.start, a.end), ix.spanOf(a.vstart, a.vend)
		out = append(out, a)
		if i < b && ix.toks[i].Type == token.COMMA {
			i++
		}
	}
	if i < b {
		i++ // )
	}
	if !positional {
		for _, a := range out {
			if a.Pin != "" && a.Pin[0] >= '0' && a.Pin[0] <= '9' {
				a.Pin = "" // a block takes named inputs only
			}
		}
	}
	return out, i
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// ---- FBD ----

// statements reads tokens [a, b) as FBD statements: one a line, or ended
// by ;, a line going on while a parenthesis is open.
func (ix *Index) statements(a, b int) []*Statement {
	var out []*Statement
	for i := a; i < b; {
		if ix.toks[i].Type == token.SEMICOLON {
			i++
			continue
		}
		j, depth := i, 0
		for ; j < b; j++ {
			t := ix.toks[j]
			if depth == 0 && j > i && (t.Row != ix.toks[j-1].Row || ix.toks[j-1].Type == token.SEMICOLON) {
				break
			}
			switch t.Type {
			case token.LPAREN:
				depth++
			case token.RPAREN:
				depth--
			}
		}
		last := j - 1
		if ix.toks[last].Type == token.SEMICOLON && last > i {
			last--
		}
		st := &Statement{start: ix.toks[i].Pos, end: ix.toks[last].Pos + len(ix.toks[last].Literal)}
		st.At, st.Span, st.Text = ix.posOf(st.start), ix.spanOf(st.start, st.end), ix.src[st.start:st.end]
		ix.statement(st, i, last+1)
		out = append(out, st)
		i = j
	}
	return out
}

func (ix *Index) statement(st *Statement, i, b int) {
	t := ix.toks[i]
	next := ix.tok(i + 1)
	value := -1
	switch {
	case next.Type == token.ASSIGN:
		st.Kind, st.Target, value = "assign", ix.toks[i].Literal, i+2
	case next.Type == token.EQ:
		st.Kind, st.Target, value = "wire", t.Literal, i+2
	case next.Type == token.COLON && i+2 < b: // inst : TYPE(...)
		st.Kind, st.Target, st.Type, st.Call = "call", t.Literal, ix.toks[i+2].Literal, ix.toks[i+2].Literal
		if i+3 < b && ix.toks[i+3].Type == token.LPAREN {
			st.Args, _ = ix.args(i+3, b, false)
		}
		return
	case next.Type == token.LPAREN: // inst(...)
		st.Kind, st.Target, st.Call = "call", t.Literal, t.Literal
		st.Args, _ = ix.args(i+1, b, false)
		return
	default:
		st.Kind = "unknown"
		return
	}
	if value < b {
		st.vstart, st.vend = ix.toks[value].Pos, st.end
		st.Value = ix.src[st.vstart:st.vend]
		if value+1 < b && isWord(ix.toks[value].Literal) && ix.toks[value+1].Type == token.LPAREN && ix.adjacent(value, value+1) {
			st.Call = ix.toks[value].Literal
			st.Args, _ = ix.args(value+1, b, true)
		}
	}
}

// ---- SFC ----

// chartDiagram models an SFC body.
func (ix *Index) chartDiagram(c chartOf) *Diagram {
	d := &Diagram{POU: c.s.sym.Name, Lang: "SFC", start: -1}
	extend := func(start, end int) {
		if d.start < 0 || start < d.start {
			d.start = start
		}
		d.end = max(d.end, end)
	}
	for _, el := range c.chart.Elements {
		switch e := el.(type) {
		case *ast.StepStatement:
			if e.Name == nil {
				continue
			}
			s := &SFCStep{Name: e.Name.Value, Initial: e.IsInitial, NameAt: Pos{e.Name.Token.Row, e.Name.Token.Column}, start: e.Token.Pos, end: ix.endOf(e.Token.Pos, "END_STEP")}
			for _, a := range e.Actions {
				if a == nil || a.ActionName == nil {
					continue
				}
				q := &SFCQualify{Action: a.ActionName.Value, Qualifier: "N"}
				if a.Qualifier != nil {
					q.Qualifier = a.Qualifier.Value
				}
				if a.Duration != nil {
					q.Time = exprText(a.Duration)
				}
				s.Actions = append(s.Actions, q)
			}
			s.At, s.Span = ix.posOf(s.start), ix.spanOf(s.start, s.end)
			d.Steps = append(d.Steps, s)
			extend(s.start, s.end)
		case *ast.TransitionStatement:
			t := &SFCTransition{start: e.Token.Pos, end: ix.endOf(e.Token.Pos, "END_TRANSITION")}
			for _, f := range e.From {
				t.From = append(t.From, f.Value)
			}
			for _, to := range e.To {
				t.To = append(t.To, to.Value)
			}
			t.cstart, t.cend = ix.condition(t.start, t.end)
			t.Condition = ix.src[t.cstart:t.cend]
			t.At, t.Span, t.Cond = ix.posOf(t.start), ix.spanOf(t.start, t.end), ix.spanOf(t.cstart, t.cend)
			d.Transits = append(d.Transits, t)
			extend(t.start, t.end)
		case *ast.ActionStatement:
			if e.Name == nil {
				continue
			}
			a := &SFCAction{Name: e.Name.Value, start: e.Token.Pos, end: ix.endOf(e.Token.Pos, "END_ACTION")}
			a.At, a.Span = ix.posOf(a.start), ix.spanOf(a.start, a.end)
			d.Actions = append(d.Actions, a)
			extend(a.start, a.end)
		}
	}
	if d.start < 0 {
		d.start = c.s.start
	}
	d.Body = ix.spanOf(d.start, d.end)
	d.SVG = diagram.SFCSVG(diagram.ChartOf(c.chart))
	return d
}

// endOf is the offset after the first word token at or after off (an
// END_ keyword).
func (ix *Index) endOf(off int, word string) int {
	for i := ix.first(off); i < len(ix.toks); i++ {
		if strings.EqualFold(ix.toks[i].Literal, word) {
			return ix.toks[i].Pos + len(ix.toks[i].Literal)
		}
	}
	return len(ix.src)
}

// condition is a transition's condition, between := and the ; or
// END_TRANSITION, in the transition's text [start, end).
func (ix *Index) condition(start, end int) (int, int) {
	a := -1
	for i := ix.first(start); i < len(ix.toks) && ix.toks[i].Pos < end; i++ {
		t := ix.toks[i]
		if a < 0 {
			if t.Type == token.ASSIGN {
				a = i + 1
			}
			continue
		}
		if t.Type == token.SEMICOLON || strings.EqualFold(t.Literal, "END_TRANSITION") {
			if i == a {
				return t.Pos, t.Pos
			}
			return ix.toks[a].Pos, ix.toks[i-1].Pos + len(ix.toks[i-1].Literal)
		}
	}
	return end, end
}

// spanOf is the span of bytes [start, end).
func (ix *Index) spanOf(start, end int) Span { return Span{ix.posOf(start), ix.posOf(end)} }
