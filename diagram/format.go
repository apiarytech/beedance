/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram

// This file writes a graphical body's elements (from ParseXML) in
// beedance's text form, so a diagram imported from another IEC tool stays a
// diagram: it can be drawn, edited as text and exported again (XML).
//
// FBD becomes a netlist, which can hold any FBD. LD becomes rungs of
// contacts in series and parallel branches; a rung wired in a way the text
// form cannot write (a bridge between branches, a function powered through
// EN) is refused with the reason, and the caller can lower the body to
// Structured Text instead (Lower).

import (
	"fmt"
	"slices"
	"strings"
)

// Format returns elems, a body of language lang ("LD" or "FBD"), in the text
// form: the lines between LD (FBD) and END_LD (END_FBD), indented by two
// spaces. fbTypes names the project's function block types, which, with the
// standard ones, are called as instances. ParseText reads the result.
func Format(lang string, elems []*Elem, fbTypes []string) (string, error) {
	f := &formatter{elems: map[int]*Elem{}, readers: map[int][]Conn{}, fbTypes: map[string]bool{}, uses: map[int]int{},
		names: map[int]string{}, taken: map[string]bool{}, lang: strings.ToUpper(lang), absorbed: map[int]bool{}}
	for _, t := range append(slices.Clone(StandardFunctionBlocks), fbTypes...) {
		f.fbTypes[strings.ToUpper(t)] = true
	}
	for _, e := range elems {
		switch e.Kind() {
		case "comment", "addData", "documentation", "rightPowerRail":
			continue
		case "jump", "label", "return":
			return "", fmt.Errorf("element %d: %s is not supported yet in the text form", e.ID, e.Kind())
		}
		if _, dup := f.elems[e.ID]; dup {
			return "", fmt.Errorf("localId %d is used twice", e.ID)
		}
		f.elems[e.ID] = e
		f.order = append(f.order, e)
		if e.Kind() == "block" && e.InstanceName != "" {
			f.taken[strings.ToUpper(e.InstanceName)] = true
		}
		if e.Kind() == "connector" {
			f.taken[strings.ToUpper(e.Name)] = true
		}
	}
	slices.SortStableFunc(f.order, func(a, b *Elem) int {
		if before(a.Pos, b.Pos) {
			return -1
		}
		if before(b.Pos, a.Pos) {
			return 1
		}
		return 0
	})
	for _, e := range f.order {
		for _, c := range f.wiresInto(e) {
			f.readers[c.Ref] = append(f.readers[c.Ref], Conn{Ref: e.ID, Param: c.Param})
		}
	}
	switch strings.ToUpper(lang) {
	case "FBD":
		return f.netlist()
	case "LD":
		return f.ladder()
	}
	return "", fmt.Errorf("unknown diagram language %q", lang)
}

type formatter struct {
	elems   map[int]*Elem
	order   []*Elem // by position, top to bottom
	readers map[int][]Conn
	fbTypes map[string]bool
	uses    map[int]int    // LD: times an element is written
	names   map[int]string // generated names: unnamed instances, wires
	taken   map[string]bool
	wires   []string // FBD: wire statements, before the rest
	n       int
	lang    string
	doms    map[int]map[int]bool
	// absorbed are the variables an LD block writes with => arguments.
	absorbed map[int]bool
}

// wiresInto lists every connection into e.
func (f *formatter) wiresInto(e *Elem) []Conn {
	var conns []Conn
	for _, in := range e.Ins {
		conns = append(conns, in.Conns...)
	}
	for _, ps := range [][]Pin{e.Inputs, e.InOuts} {
		for _, p := range ps {
			if p.In != nil {
				conns = append(conns, p.In.Conns...)
			}
		}
	}
	return conns
}

func (f *formatter) isFB(e *Elem) bool {
	return e.Kind() == "block" && (e.InstanceName != "" || f.fbTypes[strings.ToUpper(e.TypeName)])
}

// fresh returns an unused name built from base.
func (f *formatter) fresh(base string) string {
	for {
		f.n++
		name := fmt.Sprintf("%s%d", base, f.n)
		if !f.taken[strings.ToUpper(name)] {
			f.taken[strings.ToUpper(name)] = true
			return name
		}
	}
}

// instance is the name a function block is called by.
func (f *formatter) instance(e *Elem) string {
	if e.InstanceName != "" {
		return e.InstanceName
	}
	if n, ok := f.names[e.ID]; ok {
		return n
	}
	n := f.fresh("_" + strings.ToUpper(e.TypeName))
	f.names[e.ID] = n
	return n
}

// one returns the single connection of an input; an input with several
// (FBD has no OR of wires) is refused.
func one(conns []Conn, what string) (Conn, error) {
	if len(conns) != 1 {
		return Conn{}, fmt.Errorf("%s has %d connections; the text form takes one", what, len(conns))
	}
	return conns[0], nil
}

func not(s string) string { return "NOT(" + s + ")" }

// value writes the value carried by wire c, for an FBD statement or a block
// input in a rung.
func (f *formatter) value(c Conn) (string, error) {
	e, ok := f.elems[c.Ref]
	if !ok {
		return "", fmt.Errorf("a wire comes from element %d, which does not exist", c.Ref)
	}
	switch e.Kind() {
	case "inVariable":
		s := strings.TrimSpace(e.Expression)
		if e.Negated {
			s = not(s)
		}
		return s, nil
	case "inOutVariable":
		s := strings.TrimSpace(e.Expression)
		if e.NegatedOut {
			s = not(s)
		}
		return s, nil
	case "continuation":
		return e.Name, nil
	case "block":
		out := c.Param
		var s string
		if f.isFB(e) {
			if out == "" {
				return "", fmt.Errorf("a wire from block %d names no output", e.ID)
			}
			s = f.instance(e) + "." + out
		} else {
			if out != "" && !strings.EqualFold(out, "OUT") {
				return "", fmt.Errorf("function %s: output %s cannot be read in the text form (only its result)", e.TypeName, out)
			}
			call, err := f.function(e)
			if err != nil {
				return "", err
			}
			// A result read twice is named once, as a wire (in FBD; a rung has
			// no wires, and a function has no state, so there it is written twice).
			if f.lang == "FBD" && len(f.readers[e.ID]) > 1 {
				name, ok := f.names[e.ID]
				if !ok {
					name = f.fresh("_w")
					f.names[e.ID] = name
					f.wires = append(f.wires, name+" = "+call)
				}
				call = name
			}
			s = call
		}
		if p := pinOf2(e.Outputs, out); p != nil && p.Negated {
			s = not(s)
		}
		return s, nil
	}
	return "", fmt.Errorf("a value cannot come from a %s (element %d)", e.Kind(), e.ID)
}

func pinOf2(ps []Pin, param string) *Pin {
	for i := range ps {
		if strings.EqualFold(ps[i].Param, param) {
			return &ps[i]
		}
	}
	return nil
}

// input writes the value of a pin.
func (f *formatter) input(p Pin, owner string) (string, error) {
	if p.In == nil {
		return "", fmt.Errorf("%s: input %s is not connected", owner, p.Param)
	}
	if p.Edge != "" {
		return "", fmt.Errorf("%s: input %s detects an edge, which the text form has no way to write", owner, p.Param)
	}
	c, err := one(p.In.Conns, owner+" input "+p.Param)
	if err != nil {
		return "", err
	}
	s, err := f.value(c)
	if err != nil {
		return "", err
	}
	if p.Negated {
		s = not(s)
	}
	return s, nil
}

// function writes a function call, inputs positional (EN named, first).
func (f *formatter) function(e *Elem) (string, error) {
	var args []string
	for _, p := range e.Inputs {
		s, err := f.input(p, "function "+e.TypeName)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(p.Param, "EN") {
			if len(args) > 0 {
				return "", fmt.Errorf("function %s: EN after other inputs", e.TypeName)
			}
			s = "EN := " + s
		}
		args = append(args, s)
	}
	if len(args) == 0 || (len(args) == 1 && strings.HasPrefix(args[0], "EN := ")) {
		return "", fmt.Errorf("function %s has no inputs", e.TypeName)
	}
	if len(e.InOuts) > 0 {
		return "", fmt.Errorf("function %s has in-out variables, which the text form cannot write", e.TypeName)
	}
	return e.TypeName + "(" + strings.Join(args, ", ") + ")", nil
}

// call writes a function block call's arguments: inputs by name except
// skip (the rung's), outputs to the variables that only they feed.
func (f *formatter) call(e *Elem, skip string, outs map[string]string) (string, error) {
	var args []string
	for _, p := range e.Inputs {
		if skip != "" && strings.EqualFold(p.Param, skip) {
			continue
		}
		s, err := f.input(p, "block "+f.instance(e))
		if err != nil {
			return "", err
		}
		args = append(args, p.Param+" := "+s)
	}
	if len(e.InOuts) > 0 {
		return "", fmt.Errorf("block %s has in-out variables, which the text form cannot write", f.instance(e))
	}
	for _, o := range e.Outputs {
		if v, ok := outs[strings.ToUpper(o.Param)]; ok {
			args = append(args, o.Param+" => "+v)
		}
	}
	return "(" + strings.Join(args, ", ") + ")", nil
}

// ---- FBD ----

// netlist writes an FBD: named wires first, then a statement per function
// block, connector and variable written, in the drawing's order. Order is
// left to the data flow, as for any text body.
func (f *formatter) netlist() (string, error) {
	var stmts []string
	for _, e := range f.order {
		switch e.Kind() {
		case "block":
			if !f.isFB(e) {
				continue // written where it is read
			}
			for _, o := range e.Outputs {
				if o.Negated {
					return "", fmt.Errorf("block %s: output %s is negated where it leaves the block; the text form negates where a value is read", f.instance(e), o.Param)
				}
			}
			args, err := f.call(e, "", nil)
			if err != nil {
				return "", err
			}
			head := f.instance(e)
			if e.TypeName != "" {
				head += " : " + e.TypeName
			}
			stmts = append(stmts, head+args)
		case "connector":
			c, err := one(insOf(e), "connector "+e.Name)
			if err != nil {
				return "", err
			}
			v, err := f.value(c)
			if err != nil {
				return "", err
			}
			stmts = append(stmts, e.Name+" = "+v)
		case "outVariable", "inOutVariable":
			conns := insOf(e)
			if len(conns) == 0 {
				continue // an inOutVariable only read
			}
			c, err := one(conns, e.Kind()+" "+e.Expression)
			if err != nil {
				return "", err
			}
			v, err := f.value(c)
			if err != nil {
				return "", err
			}
			if e.Negated || e.NegatedIn {
				v = not(v)
			}
			stmts = append(stmts, strings.TrimSpace(e.Expression)+" := "+v)
		case "inVariable", "continuation":
		default:
			return "", fmt.Errorf("element %d: a %s does not belong in FBD", e.ID, e.Kind())
		}
	}
	var b strings.Builder
	for _, s := range append(f.wires, stmts...) {
		b.WriteString("  " + s + "\n")
	}
	return b.String(), nil
}

func insOf(e *Elem) []Conn {
	var conns []Conn
	for _, in := range e.Ins {
		conns = append(conns, in.Conns...)
	}
	return conns
}

// ---- LD ----

// ladder writes an LD: a rung for each set of coils powered alike, or for
// a function block that ends a rung, top to bottom.
func (f *formatter) ladder() (string, error) {
	type rung struct {
		power []Conn
		coils []*Elem
		end   *Elem // a function block ending the rung without coils
		y     float64
	}
	var rungs []*rung
	key := func(cs []Conn) string {
		parts := make([]string, len(cs))
		for i, c := range cs {
			parts[i] = fmt.Sprintf("%d.%s", c.Ref, strings.ToUpper(c.Param))
		}
		slices.Sort(parts)
		return strings.Join(parts, ",")
	}
	byKey := map[string]*rung{}
	for _, e := range f.order {
		switch e.Kind() {
		case "coil":
			power, err := f.coilPower(e, map[int]bool{})
			if err != nil {
				return "", err
			}
			k := key(power)
			r := byKey[k]
			if r == nil {
				r = &rung{power: power, y: e.Pos.Y}
				byKey[k] = r
				rungs = append(rungs, r)
			}
			r.coils = append(r.coils, e)
		case "block":
			if !f.isFB(e) || f.andContact(e) != nil {
				continue
			}
			pins := f.powerPins(e)
			if pinOf2(e.Inputs, pins[0]) == nil || !f.powered(e, pins[0]) {
				continue // a block on the side of a rung, feeding a pin
			}
			// A block nothing continues from ends its rung.
			if !slices.ContainsFunc(f.readers[e.ID], func(r Conn) bool { return f.inRung(f.elems[r.Ref]) }) {
				rungs = append(rungs, &rung{power: []Conn{{Ref: e.ID, Param: pins[1]}}, end: e, y: e.Pos.Y})
			}
		case "outVariable", "inOutVariable":
			for _, c := range insOf(e) {
				if f.inRung(f.elems[c.Ref]) && !f.blockOutput(c) {
					return "", fmt.Errorf("variable %s is written from the rung's power; the text form writes it with a coil", e.Expression)
				}
			}
		}
	}
	slices.SortStableFunc(rungs, func(a, b *rung) int {
		switch {
		case a.y < b.y:
			return -1
		case a.y > b.y:
			return 1
		}
		return 0
	})
	f.doms = map[int]map[int]bool{}
	var b strings.Builder
	for _, r := range rungs {
		var text string
		var err error
		if r.end != nil {
			text, err = f.seg(r.end.ID, 0)
		} else {
			text, err = f.join(r.power, 0)
		}
		if err != nil {
			return "", err
		}
		for _, c := range r.coils {
			text += " " + coilText(c)
		}
		b.WriteString("  RUNG\n    " + strings.TrimSpace(text) + "\n")
	}
	// Every element of a rung is written once; only plain contacts and
	// function contacts, which have no state, may be written twice.
	for _, e := range f.order {
		if !f.inRung(e) || e.Kind() == "leftPowerRail" || e.Kind() == "coil" {
			continue
		}
		switch n := f.uses[e.ID]; {
		case n == 0:
			return "", fmt.Errorf("element %d (%s) is not in a rung that ends in a coil or a function block", e.ID, label(e))
		case n > 1 && (e.Kind() == "block" || e.Edge != ""):
			return "", fmt.Errorf("%s would be written in %d places: the rungs are wired in a way the text form cannot write (branches that cross)", label(e), n)
		}
	}
	// Nothing may be left out: a variable a rung's block writes is a =>
	// argument, and a function block stands in a rung.
	for _, e := range f.order {
		switch {
		case (e.Kind() == "outVariable" || e.Kind() == "inOutVariable") && len(insOf(e)) > 0 && !f.absorbed[e.ID]:
			return "", fmt.Errorf("variable %s is written by a wire the LD text form cannot write (only a block's output, as =>)", strings.TrimSpace(e.Expression))
		case f.isFB(e) && !f.inRung(e):
			return "", fmt.Errorf("%s is not powered by a rung; the LD text form calls a function block only in a rung", label(e))
		}
	}
	return b.String(), nil
}

func label(e *Elem) string {
	switch e.Kind() {
	case "contact", "coil":
		return e.Kind() + " " + e.Variable
	case "block":
		if e.InstanceName != "" {
			return "block " + e.InstanceName
		}
		return "block " + e.TypeName
	}
	return e.Kind()
}

// powerPins are the input a rung drives and the output it continues from.
func (f *formatter) powerPins(e *Elem) [2]string {
	if p, ok := powerPins[strings.ToUpper(e.TypeName)]; ok {
		return p
	}
	return [2]string{"EN", "ENO"}
}

// inRung reports whether e carries the rung's power.
func (f *formatter) inRung(e *Elem) bool {
	if e == nil {
		return false
	}
	switch e.Kind() {
	case "leftPowerRail", "contact", "coil":
		return true
	case "block":
		if f.andContact(e) != nil {
			return true
		}
		if f.isFB(e) {
			pins := f.powerPins(e)
			return pinOf2(e.Inputs, pins[0]) != nil && f.powered(e, pins[0])
		}
	}
	return false
}

// powered reports whether input pin of e is wired from the rung's power.
func (f *formatter) powered(e *Elem, pin string) bool {
	p := pinOf2(e.Inputs, pin)
	if p == nil || p.In == nil || len(p.In.Conns) == 0 {
		return false
	}
	for _, c := range p.In.Conns {
		src := f.elems[c.Ref]
		if src == nil {
			return false
		}
		switch src.Kind() {
		case "leftPowerRail", "contact", "coil":
		case "block":
			if f.andContact(src) == nil && !(f.isFB(src) && strings.EqualFold(c.Param, f.powerPins(src)[1])) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// andContact recognizes a function contact: an AND of the rung's power
// (IN1) and a function's result (IN2), as ParseText and XML write it. It
// returns the function, or nil.
func (f *formatter) andContact(e *Elem) *Elem {
	if e.Kind() != "block" || !strings.EqualFold(e.TypeName, "AND") || e.InstanceName != "" || len(e.Inputs) != 2 {
		return nil
	}
	in1, in2 := e.Inputs[0], e.Inputs[1]
	if in1.In == nil || in2.In == nil || len(in2.In.Conns) != 1 || in1.Negated || in2.Negated {
		return nil
	}
	fn := f.elems[in2.In.Conns[0].Ref]
	if fn == nil || fn.Kind() != "block" || f.isFB(fn) || pinOf2(fn.Inputs, "EN") != nil || len(f.readers[fn.ID]) != 1 {
		return nil
	}
	for _, c := range in1.In.Conns {
		src := f.elems[c.Ref]
		if src == nil {
			return nil
		}
		switch src.Kind() {
		case "leftPowerRail", "contact", "coil":
		case "block":
			if f.andContact(src) == nil && !f.isFB(src) {
				return nil
			}
		default:
			return nil
		}
	}
	return fn
}

// blockOutput reports whether c reads a function block's output other than
// the one the rung continues from: a value, not power.
func (f *formatter) blockOutput(c Conn) bool {
	e := f.elems[c.Ref]
	return e != nil && f.isFB(e) && !strings.EqualFold(c.Param, f.powerPins(e)[1])
}

// coilPower is the power into a coil: a coil passes its power on, so a
// coil fed by another is powered as that one.
func (f *formatter) coilPower(e *Elem, seen map[int]bool) ([]Conn, error) {
	if seen[e.ID] {
		return nil, fmt.Errorf("coils %s are wired in a loop", e.Variable)
	}
	seen[e.ID] = true
	var out []Conn
	for _, c := range insOf(e) {
		src := f.elems[c.Ref]
		if src != nil && src.Kind() == "coil" {
			more, err := f.coilPower(src, seen)
			if err != nil {
				return nil, err
			}
			out = append(out, more...)
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("coil %s is not connected", e.Variable)
	}
	return out, nil
}

// preds are the elements whose power feeds e, as connections.
func (f *formatter) preds(e *Elem) []Conn {
	switch e.Kind() {
	case "contact":
		return insOf(e)
	case "coil":
		cs, _ := f.coilPower(e, map[int]bool{})
		return cs
	case "block":
		if f.andContact(e) != nil {
			return slices.Clone(e.Inputs[0].In.Conns)
		}
		if p := pinOf2(e.Inputs, f.powerPins(e)[0]); p != nil && p.In != nil {
			return slices.Clone(p.In.Conns)
		}
	}
	return nil
}

// dom returns the elements every path of power to id passes through, id
// included (its dominators, the rail at the root).
func (f *formatter) dom(id int) map[int]bool {
	if d, ok := f.doms[id]; ok {
		return d
	}
	f.doms[id] = map[int]bool{id: true} // a loop is cut here
	var d map[int]bool
	for _, c := range f.preds(f.elems[id]) {
		pd := f.dom(c.Ref)
		if d == nil {
			d = map[int]bool{}
			for k := range pd {
				d[k] = true
			}
			continue
		}
		for k := range d {
			if !pd[k] {
				delete(d, k)
			}
		}
	}
	if d == nil {
		d = map[int]bool{}
	}
	d[id] = true
	f.doms[id] = d
	return d
}

// common is the nearest element every connection's power passes through.
func (f *formatter) common(conns []Conn) int {
	var both map[int]bool
	for _, c := range conns {
		d := f.dom(c.Ref)
		if both == nil {
			both = map[int]bool{}
			for k := range d {
				both[k] = true
			}
			continue
		}
		for k := range both {
			if !d[k] {
				delete(both, k)
			}
		}
	}
	best, depth := 0, -1
	for k := range both {
		if n := len(f.dom(k)); n > depth { // the deepest dominator
			best, depth = k, n
		}
	}
	return best
}

// join writes the power of conns from element from (0: the rail): one
// series, or the series to where they part and then a branch of each.
func (f *formatter) join(conns []Conn, from int) (string, error) {
	if len(conns) == 1 {
		return f.seg(conns[0].Ref, from)
	}
	at := f.common(conns)
	head, err := f.seg(at, from)
	if err != nil {
		return "", err
	}
	// Legs that part again further on are one leg with a branch inside,
	// as drawn: A [ B [ C | D ] | E ], not A [ B C | B D | E ].
	var keys []int
	groups := map[int][]Conn{}
	for _, c := range conns {
		k := f.after(c.Ref, at)
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], c)
	}
	var legs []string
	for _, k := range keys {
		var leg string
		if g := groups[k]; len(g) == 1 {
			leg, err = f.seg(g[0].Ref, at)
		} else {
			leg, err = f.join(g, at)
		}
		if err != nil {
			return "", err
		}
		legs = append(legs, leg)
	}
	return strings.TrimSpace(head + " [ " + strings.Join(legs, " | ") + " ]"), nil
}

// after is the first element after at that every path of power to id
// passes through: the start of id's leg from at. It is id itself when
// nothing comes between, and at when id is at.
func (f *formatter) after(id, at int) int {
	if id == at {
		return at
	}
	best, depth := id, len(f.dom(id))
	for k := range f.dom(id) {
		if k != at && f.dom(k)[at] && len(f.dom(k)) < depth {
			best, depth = k, len(f.dom(k))
		}
	}
	return best
}

// seg writes the elements from after from up to and including id.
func (f *formatter) seg(id, from int) (string, error) {
	e := f.elems[id]
	if id == from || e == nil || e.Kind() == "leftPowerRail" {
		return "", nil
	}
	if e.Kind() == "coil" { // power passes through: written with its rung
		cs, err := f.coilPower(e, map[int]bool{})
		if err != nil {
			return "", err
		}
		return f.join(cs, from)
	}
	cs := f.preds(e)
	if len(cs) == 0 {
		return "", fmt.Errorf("%s is not connected on its left", label(e))
	}
	if from != 0 && !f.dom(id)[from] {
		return "", fmt.Errorf("%s is reached around a branch: the rung is wired in a way the text form cannot write", label(e))
	}
	head, err := f.join(cs, from)
	if err != nil {
		return "", err
	}
	text, err := f.element(e)
	if err != nil {
		return "", err
	}
	f.uses[e.ID]++
	return strings.TrimSpace(head + " " + text), nil
}

// element writes one element of a rung.
func (f *formatter) element(e *Elem) (string, error) {
	switch e.Kind() {
	case "contact":
		v := strings.TrimSpace(e.Variable)
		switch {
		case e.Edge == "rising":
			return "+" + v, nil
		case e.Edge == "falling":
			return "-" + v, nil
		case e.Negated:
			return "/" + v, nil
		}
		return v, nil
	case "block":
		if fn := f.andContact(e); fn != nil {
			call, err := f.function(fn)
			if err != nil {
				return "", err
			}
			if p := pinOf2(fn.Outputs, "OUT"); p != nil && p.Negated {
				call = "/" + call
			}
			return call, nil
		}
		pins := f.powerPins(e)
		// Outputs written to variables they alone feed are => arguments.
		outs := map[string]string{}
		for _, r := range f.readers[e.ID] {
			reader := f.elems[r.Ref]
			if reader.Kind() == "outVariable" && !reader.Negated && len(insOf(reader)) == 1 && !strings.EqualFold(r.Param, pins[1]) {
				outs[strings.ToUpper(r.Param)] = strings.TrimSpace(reader.Expression)
			}
		}
		for p := range outs {
			for _, r := range f.readers[e.ID] {
				reader := f.elems[r.Ref]
				if strings.EqualFold(r.Param, p) && reader.Kind() != "outVariable" {
					delete(outs, p) // read elsewhere too: by its instance name
				}
			}
		}
		for _, r := range f.readers[e.ID] {
			if _, ok := outs[strings.ToUpper(r.Param)]; ok && f.elems[r.Ref].Kind() == "outVariable" {
				if f.absorbed[r.Ref] {
					delete(outs, strings.ToUpper(r.Param)) // two variables from one output: one => only
					continue
				}
				f.absorbed[r.Ref] = true
			}
		}
		args, err := f.call(e, pins[0], outs)
		if err != nil {
			return "", err
		}
		if e.TypeName == "" {
			return "", fmt.Errorf("block %s in a rung has no type", e.InstanceName)
		}
		if args == "()" {
			args = ""
		}
		return f.instance(e) + ":" + e.TypeName + args, nil
	}
	return "", fmt.Errorf("a %s cannot stand in a rung", e.Kind())
}

func coilText(e *Elem) string {
	v := strings.TrimSpace(e.Variable)
	switch {
	case e.Storage == "set":
		return "( S " + v + " )"
	case e.Storage == "reset":
		return "( R " + v + " )"
	case e.Edge == "rising":
		return "( P " + v + " )"
	case e.Edge == "falling":
		return "( N " + v + " )"
	case e.Negated:
		return "( /" + v + " )"
	}
	return "( " + v + " )"
}
