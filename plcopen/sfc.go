/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

// This file reads a graphical SFC body into a diagram.Chart, which writes
// it as beedance's text form of SFC.
//
// A graphical chart is a graph: each element names the elements wired into
// it. A transition's source steps are found by walking up through
// selection divergences and simultaneous convergences to steps; its
// target steps by walking down through selection convergences,
// simultaneous divergences and jumps. Steps and transitions are all the
// text form needs, the branches being implied by them.
//
// A condition is an inline ST expression, a named transition of the POU
// (ST, or an LD or FBD body assigning the transition's name), or a network
// of the body wired into the transition. Transitions leaving the same
// steps are a selection: the first, by priority and then left to right,
// wins, so each condition is ANDed with NOT of those before it.

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/apiarytech/beedance/diagram"
)

// sfcElem is any element of a graphical SFC body; the fields used depend
// on its kind (XMLName.Local).
type sfcElem struct {
	XMLName  xml.Name
	ID       int               `xml:"localId,attr"`
	Pos      diagram.Pos       `xml:"position"`
	Name     string            `xml:"name,attr"`
	Initial  bool              `xml:"initialStep,attr"`
	Target   string            `xml:"targetName,attr"`
	Priority string            `xml:"priority,attr"`
	Ins      []diagram.PointIn `xml:"connectionPointIn"`
	Cond     *sfcCondition     `xml:"condition"`
	Actions  []sfcAction       `xml:"action"`
}

type sfcCondition struct {
	Negated   bool             `xml:"negated,attr"`
	Reference *sfcReference    `xml:"reference"`
	Inline    *POUBody         `xml:"inline"`
	In        *diagram.PointIn `xml:"connectionPointIn"`
}

type sfcReference struct {
	Name string `xml:"name,attr"`
}

type sfcAction struct {
	Qualifier string        `xml:"qualifier,attr"`
	Duration  string        `xml:"duration,attr"`
	Reference *sfcReference `xml:"reference"`
	Inline    *POUBody      `xml:"inline"`
}

func (e *sfcElem) kind() string { return e.XMLName.Local }

// SFCChart is a POU's graphical SFC body read as a chart, and the
// variables its lowered LD and FBD actions need besides the POU's own.
type SFCChart struct {
	Chart *diagram.Chart
	Decls []string
}

// sfcReader reads one POU's chart.
type sfcReader struct {
	pou      POU
	fbTypes  []string
	declared []string // the POU's names, and those generated so far
	elems    map[int]*sfcElem
	order    []*sfcElem
	next     map[int][]int // element -> the elements wired from it
	graphics []*diagram.Elem
	chart    *diagram.Chart
	decls    []string
}

// ReadSFC reads a POU's graphical SFC body into a chart. fbTypes names the
// project's function block types, for actions and conditions drawn in LD
// or FBD.
func ReadSFC(pou POU, fbTypes []string) (*SFCChart, error) {
	if pou.Body.SFC == nil {
		return nil, fmt.Errorf("pou '%s' has no SFC body", pou.Name)
	}
	r := &sfcReader{pou: pou, fbTypes: fbTypes, declared: declaredNames(pou), elems: map[int]*sfcElem{},
		next: map[int][]int{}, chart: &diagram.Chart{}}
	if err := r.read(pou.Body.SFC.Inner); err != nil {
		return nil, fmt.Errorf("pou '%s': SFC body: %w", pou.Name, err)
	}
	return &SFCChart{Chart: r.chart, Decls: r.decls}, nil
}

func (r *sfcReader) read(inner string) error {
	var body struct {
		Elems []*sfcElem `xml:",any"`
	}
	if err := xml.Unmarshal([]byte("<body>"+inner+"</body>"), &body); err != nil {
		return err
	}
	graphics, err := diagram.ParseXML(inner)
	if err != nil {
		return err
	}
	for _, g := range graphics {
		switch g.Kind() {
		case "step", "transition", "selectionDivergence", "selectionConvergence", "simultaneousDivergence",
			"simultaneousConvergence", "jumpStep", "macroStep", "actionBlock", "comment", "addData", "documentation":
		default:
			r.graphics = append(r.graphics, g)
		}
	}
	for _, e := range body.Elems {
		switch e.kind() {
		case "macroStep":
			return fmt.Errorf("element %d: macro steps are not supported yet", e.ID)
		case "step", "transition", "selectionDivergence", "selectionConvergence", "simultaneousDivergence",
			"simultaneousConvergence", "jumpStep", "actionBlock":
			r.elems[e.ID] = e
			r.order = append(r.order, e)
			for _, in := range e.Ins {
				for _, c := range in.Conns {
					r.next[c.Ref] = append(r.next[c.Ref], e.ID)
				}
			}
		}
	}
	// Top to bottom, then left to right: the order the text lists them.
	slices.SortStableFunc(r.order, func(a, b *sfcElem) int {
		if a.Pos.Y != b.Pos.Y {
			return cmpFloat(a.Pos.Y, b.Pos.Y)
		}
		return cmpFloat(a.Pos.X, b.Pos.X)
	})

	if err := r.namedActions(); err != nil {
		return err
	}
	stepAt := map[int]int{} // step element -> its index in the chart
	for _, e := range r.order {
		if e.kind() == "step" {
			if e.Name == "" {
				return fmt.Errorf("step %d has no name", e.ID)
			}
			stepAt[e.ID] = len(r.chart.Steps)
			r.chart.Steps = append(r.chart.Steps, diagram.Step{Name: e.Name, Initial: e.Initial})
		}
	}
	for _, e := range r.order {
		if e.kind() != "actionBlock" {
			continue
		}
		if err := r.actionBlock(e, stepAt); err != nil {
			return err
		}
	}
	type pending struct {
		e   *sfcElem
		t   diagram.Transition
		key string
	}
	var ts []pending
	for _, e := range r.order {
		if e.kind() != "transition" {
			continue
		}
		t := diagram.Transition{}
		var err error
		if t.From, err = r.sources(e.ID); err != nil {
			return err
		}
		if t.To, err = r.targets(e.ID); err != nil {
			return err
		}
		if len(t.From) == 0 {
			return fmt.Errorf("transition %d follows no step", e.ID)
		}
		if len(t.To) == 0 {
			return fmt.Errorf("transition %d leads to no step", e.ID)
		}
		if t.Name, t.Condition, err = r.condition(e); err != nil {
			return err
		}
		from := slices.Clone(t.From)
		slices.Sort(from)
		ts = append(ts, pending{e: e, t: t, key: strings.ToUpper(strings.Join(from, ","))})
	}

	// A selection: transitions leaving the same steps clear one at a time,
	// by priority, then left to right.
	groups := map[string][]int{}
	for i, p := range ts {
		groups[p.key] = append(groups[p.key], i)
	}
	for _, idx := range groups {
		if len(idx) < 2 {
			continue
		}
		slices.SortStableFunc(idx, func(a, b int) int {
			pa, pb := priority(ts[a].e), priority(ts[b].e)
			if pa != pb {
				return pa - pb
			}
			return cmpFloat(ts[a].e.Pos.X, ts[b].e.Pos.X)
		})
		var before []string
		for _, i := range idx {
			c := operand(ts[i].t.Condition)
			if len(before) == 1 {
				ts[i].t.Condition = c + " AND NOT " + before[0]
			} else if len(before) > 1 {
				ts[i].t.Condition = c + " AND NOT (" + strings.Join(before, " OR ") + ")"
			}
			before = append(before, c)
		}
	}
	for _, p := range ts {
		r.chart.Transitions = append(r.chart.Transitions, p.t)
	}
	return nil
}

// priority is a transition's priority, lower first; one without comes last.
func priority(e *sfcElem) int {
	if n, err := strconv.Atoi(strings.TrimSpace(e.Priority)); err == nil {
		return n
	}
	return 1 << 30
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// sources are the steps a transition leaves.
func (r *sfcReader) sources(id int) ([]string, error) {
	var out []string
	seen := map[int]bool{}
	var up func(ref int) error
	up = func(ref int) error {
		if seen[ref] {
			return nil
		}
		seen[ref] = true
		e := r.elems[ref]
		if e == nil {
			return fmt.Errorf("transition %d: element %d before it is not part of the chart", id, ref)
		}
		switch e.kind() {
		case "step":
			out = appendName(out, e.Name)
			return nil
		case "selectionDivergence", "simultaneousConvergence":
			for _, in := range e.Ins {
				for _, c := range in.Conns {
					if err := up(c.Ref); err != nil {
						return err
					}
				}
			}
			return nil
		}
		return fmt.Errorf("transition %d: a %s cannot come before a transition", id, e.kind())
	}
	for _, in := range r.elems[id].Ins {
		for _, c := range in.Conns {
			if err := up(c.Ref); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// targets are the steps a transition leads to.
func (r *sfcReader) targets(id int) ([]string, error) {
	var out []string
	seen := map[int]bool{}
	var down func(ref int) error
	down = func(ref int) error {
		if seen[ref] {
			return nil
		}
		seen[ref] = true
		e := r.elems[ref]
		if e == nil {
			return fmt.Errorf("transition %d: element %d after it is not part of the chart", id, ref)
		}
		switch e.kind() {
		case "step":
			out = appendName(out, e.Name)
			return nil
		case "jumpStep":
			if e.Target == "" {
				return fmt.Errorf("jump %d has no target step", e.ID)
			}
			out = appendName(out, e.Target)
			return nil
		case "selectionConvergence", "simultaneousDivergence":
			for _, n := range r.next[ref] {
				if err := down(n); err != nil {
					return err
				}
			}
			return nil
		}
		return fmt.Errorf("transition %d: a %s cannot follow a transition", id, e.kind())
	}
	for _, n := range r.next[id] {
		if err := down(n); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func appendName(names []string, n string) []string {
	if slices.ContainsFunc(names, func(s string) bool { return strings.EqualFold(s, n) }) {
		return names
	}
	return append(names, n)
}

// condition is a transition's name, if it has one, and its condition as
// an expression.
func (r *sfcReader) condition(e *sfcElem) (string, string, error) {
	c := e.Cond
	if c == nil {
		return "", "", fmt.Errorf("transition %d has no condition", e.ID)
	}
	var name, expr string
	var err error
	switch {
	case c.Reference != nil:
		name = c.Reference.Name
		expr, err = r.namedTransition(name)
	case c.Inline != nil:
		if c.Inline.ST == nil {
			return "", "", fmt.Errorf("transition %d: an inline condition must be ST", e.ID)
		}
		expr, err = stExpression(c.Inline.ST, "")
	case c.In != nil:
		expr, err = r.wired(*c.In)
	default:
		return "", "", fmt.Errorf("transition %d has an empty condition", e.ID)
	}
	if err != nil {
		return "", "", fmt.Errorf("transition %d: %w", e.ID, err)
	}
	if c.Negated {
		expr = "NOT (" + expr + ")"
	}
	return name, expr, nil
}

// namedTransition is the condition of the POU's transition name.
func (r *sfcReader) namedTransition(name string) (string, error) {
	if r.pou.Transitions != nil {
		for _, t := range r.pou.Transitions.Transitions {
			if !strings.EqualFold(t.Name, name) {
				continue
			}
			b := t.Body
			switch {
			case b.ST != nil:
				return stExpression(b.ST, t.Name)
			case b.FBD != nil, b.LD != nil:
				raw := b.FBD
				if raw == nil {
					raw = b.LD
				}
				low, err := LowerGraphical(r.pou.Name, raw.Inner, r.fbTypes, append(slices.Clone(r.declared), t.Name))
				if err != nil {
					return "", fmt.Errorf("transition %s: %w", t.Name, err)
				}
				return assignedValue(low, t.Name)
			}
			return "", fmt.Errorf("transition %s: only ST, LD and FBD conditions are supported", t.Name)
		}
	}
	return "", fmt.Errorf("no transition named %s", name)
}

var assignment = regexp.MustCompile(`(?is)^\s*([A-Za-z_][A-Za-z0-9_.]*)\s*:=\s*(.*?)\s*;?\s*$`)

// stExpression reads a condition written in ST: an expression, ":= expr;"
// or "name := expr;", as tools write them.
func stExpression(ft *FormattedText, name string) (string, error) {
	text, err := FormattedTextContent(ft.Text)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if rest, ok := strings.CutPrefix(text, ":="); ok {
		text = rest
	} else if m := assignment.FindStringSubmatch(text); m != nil && (name == "" || strings.EqualFold(m[1], name)) {
		text = m[2]
	}
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ";"))
	if text == "" {
		return "", fmt.Errorf("the condition is empty")
	}
	if strings.Contains(text, ";") {
		return "", fmt.Errorf("the condition %q is statements, not an expression", text)
	}
	return text, nil
}

// assignedValue is the expression a lowered LD or FBD body assigns to
// name, which must be all it does.
func assignedValue(low *diagram.Lowered, name string) (string, error) {
	body := strings.TrimSpace(low.Body)
	m := assignment.FindStringSubmatch(body)
	if len(low.Decls) > 0 || m == nil || !strings.EqualFold(m[1], name) || strings.Contains(m[2], ";") {
		return "", fmt.Errorf("the condition needs statements (%s), not one expression", strings.ReplaceAll(body, "\n", " "))
	}
	return m[2], nil
}

// wired is the condition a network of the body feeds into a transition:
// its elements are lowered as if they fed an output variable.
func (r *sfcReader) wired(in diagram.PointIn) (string, error) {
	byID := map[int]*diagram.Elem{}
	for _, g := range r.graphics {
		byID[g.ID] = g
	}
	keep := map[int]bool{}
	var walk func(conns []diagram.Conn)
	walk = func(conns []diagram.Conn) {
		for _, c := range conns {
			g := byID[c.Ref]
			if g == nil || keep[g.ID] {
				continue
			}
			keep[g.ID] = true
			for _, p := range g.Ins {
				walk(p.Conns)
			}
			for _, ps := range [][]diagram.Pin{g.Inputs, g.InOuts} {
				for _, p := range ps {
					if p.In != nil {
						walk(p.In.Conns)
					}
				}
			}
		}
	}
	walk(in.Conns)
	if len(keep) == 0 {
		return "", fmt.Errorf("nothing of the body is wired into its condition")
	}
	next := 0
	var elems []*diagram.Elem
	for _, g := range r.graphics {
		next = max(next, g.ID)
		if keep[g.ID] {
			elems = append(elems, g)
		}
	}
	for _, e := range r.elems {
		next = max(next, e.ID)
	}
	const sink = "_TRANSITION_CONDITION"
	out := &diagram.Elem{ID: next + 1, Expression: sink, Ins: []diagram.PointIn{in}}
	out.XMLName.Local = "outVariable"
	low, err := diagram.Lower(r.pou.Name, append(elems, out), diagram.Options{FunctionBlocks: r.fbTypes, Declared: r.declared})
	if err != nil {
		return "", err
	}
	return assignedValue(low, sink)
}

// namedActions adds the POU's actions to the chart, LD and FBD lowered to
// ST.
func (r *sfcReader) namedActions() error {
	if r.pou.Actions == nil {
		return nil
	}
	for _, a := range r.pou.Actions.Actions {
		body, err := r.actionBody(a.Name, a.Body)
		if err != nil {
			return err
		}
		r.addAction(a.Name, body)
	}
	return nil
}

func (r *sfcReader) addAction(name, body string) {
	r.chart.Actions = append(r.chart.Actions, diagram.Action{Name: name, Body: body})
	r.declared = append(r.declared, name)
}

// actionBody is an action's body as Structured Text.
func (r *sfcReader) actionBody(name string, b POUBody) (string, error) {
	switch {
	case b.ST != nil:
		text, err := FormattedTextContent(b.ST.Text)
		if err != nil {
			return "", fmt.Errorf("action %s: %w", name, err)
		}
		return text, nil
	case b.FBD != nil, b.LD != nil:
		raw := b.FBD
		if raw == nil {
			raw = b.LD
		}
		low, err := LowerGraphical(r.pou.Name, raw.Inner, r.fbTypes, r.declared)
		if err != nil {
			return "", fmt.Errorf("action %s: %w", name, err)
		}
		for _, d := range low.Decls {
			n, _, _ := strings.Cut(d, ":")
			r.declared = append(r.declared, strings.TrimSpace(n))
		}
		r.decls = append(r.decls, low.Decls...)
		return low.Body, nil
	}
	return "", fmt.Errorf("action %s: only ST, LD and FBD actions are supported", name)
}

// actionBlock adds a block's actions to the step it is wired to. An
// inline action becomes an action of its own, named after the step.
func (r *sfcReader) actionBlock(e *sfcElem, stepAt map[int]int) error {
	step := -1
	for _, in := range e.Ins {
		for _, c := range in.Conns {
			if i, ok := stepAt[c.Ref]; ok {
				step = i
			}
		}
	}
	if step < 0 {
		return fmt.Errorf("action block %d is not wired to a step", e.ID)
	}
	s := &r.chart.Steps[step]
	for _, a := range e.Actions {
		as := diagram.Association{Qualifier: strings.ToUpper(strings.TrimSpace(a.Qualifier))}
		if a.Duration != "" {
			as.Duration = durationText(strings.TrimSpace(a.Duration))
		}
		switch {
		case a.Reference != nil:
			as.Action = a.Reference.Name
		case a.Inline != nil:
			body, err := r.actionBody(s.Name, *a.Inline)
			if err != nil {
				return fmt.Errorf("step %s: inline %w", s.Name, err)
			}
			as.Action = r.fresh("_" + s.Name)
			r.addAction(as.Action, body)
		default:
			return fmt.Errorf("action block %d: an action names no action", e.ID)
		}
		s.Actions = append(s.Actions, as)
	}
	return nil
}

// fresh returns base with a number, unused in the POU.
func (r *sfcReader) fresh(base string) string {
	used := map[string]bool{}
	for _, d := range r.declared {
		used[strings.ToUpper(d)] = true
	}
	for _, s := range r.chart.Steps {
		used[strings.ToUpper(s.Name)] = true
	}
	for i := 1; ; i++ {
		n := fmt.Sprintf("%s_%d", base, i)
		if !used[strings.ToUpper(n)] {
			return n
		}
	}
}

var plainName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$|^\(.*\)$`)

// operand is an expression as an operand of AND, OR and NOT: in
// parentheses unless it is a name, or already in them.
func operand(expr string) string {
	if m := plainName.FindString(expr); m != "" && (m[0] != '(' || balanced(m[1:len(m)-1])) {
		return expr
	}
	return "(" + expr + ")"
}

// balanced reports whether s's parentheses pair up from left to right.
func balanced(s string) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}
