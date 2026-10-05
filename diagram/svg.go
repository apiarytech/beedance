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

// This file draws a body as SVG, for previews: the text form has no layout,
// so one is computed, and PLCopen XML's own coordinates are used only to
// order the networks and their elements, so both forms of a body look
// alike.
//
// The layout is layered: an element's column is how far it is from the
// power rail or from the values it reads; an element shares the row of
// what feeds it, so contacts in series line up, and parallel legs take the
// next free row; coils are right-aligned; a block spans a row per pin.
// Wires are orthogonal and join just before the element they feed, where
// a ladder's branches close.

import (
	"fmt"
	"slices"
	"strings"
)

// Layout sizes, in SVG user units (pixels at 100 %).
const (
	rowH    = 32 // a row
	gapX    = 36 // between columns
	margin  = 16
	charW   = 7.2 // monospace at 12px, roughly
	railGap = 24  // from the rail to the first column
)

// SVG draws a body's elements (from ParseText or ParseXML) as an SVG
// document. Colours follow the editor's theme where the page sets VS
// Code's CSS variables, with plain fallbacks elsewhere.
func SVG(elems []*Elem) string {
	l := newLayout(forDrawing(elems))
	return l.draw()
}

type node struct {
	e          *Elem
	layer, row int
	rows       int      // rows spanned
	head       int      // rows above the pins (none yet)
	label      bool     // an instance name above it, in the row above
	ins, outs  []string // pins drawn, in order (a block's)
	w          float64
}

type layout struct {
	elems   map[int]*Elem
	order   []*Elem
	nodes   map[int]*node
	srcs    map[int][]Conn // element -> wires into it (all inputs)
	readers map[int][]Conn // element -> params read from it (Ref = reader)
	nets    [][]*node
	netRail map[int]*Elem // network index -> its rail, if any
}

func newLayout(elems []*Elem) *layout {
	l := &layout{elems: map[int]*Elem{}, nodes: map[int]*node{}, srcs: map[int][]Conn{}, readers: map[int][]Conn{}, netRail: map[int]*Elem{}}
	for _, e := range elems {
		switch e.Kind() {
		case "comment", "addData", "documentation", "rightPowerRail":
			continue
		}
		l.elems[e.ID] = e
		l.order = append(l.order, e)
	}
	slices.SortStableFunc(l.order, func(a, b *Elem) int {
		if before(a.Pos, b.Pos) {
			return -1
		}
		if before(b.Pos, a.Pos) {
			return 1
		}
		return 0
	})
	for _, e := range l.order {
		var conns []Conn
		for _, in := range e.Ins {
			conns = append(conns, in.Conns...)
		}
		for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
			for _, p := range pins {
				if p.In != nil {
					conns = append(conns, p.In.Conns...)
				}
			}
		}
		for _, c := range conns {
			if _, ok := l.elems[c.Ref]; ok {
				l.srcs[e.ID] = append(l.srcs[e.ID], c)
				l.readers[c.Ref] = append(l.readers[c.Ref], Conn{Ref: e.ID, Param: c.Param})
			}
		}
	}
	l.networks()
	for i, net := range l.nets {
		l.place(i, net)
	}
	return l
}

// networks groups elements joined by wires, rails apart (each LD rung is
// one), in the order of their first element.
func (l *layout) networks() {
	parent := map[int]int{}
	var find func(int) int
	find = func(x int) int {
		if p, ok := parent[x]; ok && p != x {
			r := find(p)
			parent[x] = r
			return r
		}
		parent[x] = x
		return x
	}
	rail := func(id int) bool { return l.elems[id].Kind() == "leftPowerRail" }
	for _, e := range l.order {
		find(e.ID)
		for _, c := range l.srcs[e.ID] {
			if !rail(c.Ref) {
				parent[find(e.ID)] = find(c.Ref)
			}
		}
		if e.Kind() == "continuation" {
			for _, o := range l.order {
				if o.Kind() == "connector" && strings.EqualFold(o.Name, e.Name) {
					parent[find(e.ID)] = find(o.ID)
				}
			}
		}
	}
	index := map[int]int{}
	for _, e := range l.order {
		if rail(e.ID) {
			continue
		}
		r := find(e.ID)
		i, ok := index[r]
		if !ok {
			i = len(l.nets)
			index[r] = i
			l.nets = append(l.nets, nil)
		}
		n := &node{e: e}
		l.nodes[e.ID] = n
		l.nets[i] = append(l.nets[i], n)
		for _, c := range l.srcs[e.ID] {
			if rail(c.Ref) {
				l.netRail[i] = l.elems[c.Ref]
			}
		}
	}
}

// place sets the layers and rows of one network.
func (l *layout) place(net int, nodes []*node) {
	// Layers: the longest path from the rail or the network's inputs.
	layer := map[int]int{}
	var depth func(id int, seen map[int]bool) int
	depth = func(id int, seen map[int]bool) int {
		if d, ok := layer[id]; ok {
			return d
		}
		if seen[id] { // a loop: cut it here
			return 0
		}
		seen[id] = true
		d := 0
		for _, c := range l.srcs[id] {
			if n := l.nodes[c.Ref]; n != nil {
				d = max(d, depth(c.Ref, seen)+1)
			}
		}
		delete(seen, id)
		layer[id] = d
		return d
	}
	last := 0
	for _, n := range nodes {
		n.layer = depth(n.e.ID, map[int]bool{})
		last = max(last, n.layer)
	}
	for _, n := range nodes {
		if n.e.Kind() == "coil" {
			n.layer = last // coils on the right, as a rung ends
		}
		n.ins, n.outs = l.pins(n.e)
		n.label = n.e.Kind() == "block" && n.e.InstanceName != ""
		n.rows = n.head + max(1, len(n.ins), len(n.outs))
		n.w = l.width(n)
	}

	// Rows: the row of what feeds it, or the next free one.
	byLayer := slices.Clone(nodes)
	slices.SortStableFunc(byLayer, func(a, b *node) int { return a.layer - b.layer })
	taken := map[[2]int]bool{}
	free := func(layer, row, rows int) bool {
		for r := row; r < row+rows; r++ {
			if taken[[2]int{layer, r}] {
				return false
			}
		}
		return true
	}
	for _, n := range byLayer {
		row := -1
		consider := func(conns []Conn, pin int) {
			for _, c := range conns {
				s := l.nodes[c.Ref]
				if s == nil {
					continue
				}
				r := max(0, s.row+s.head+max(0, outIndex(s, c.Param))-pin-n.head)
				if row < 0 || r < row {
					row = r
				}
			}
		}
		for _, in := range n.e.Ins {
			consider(in.Conns, 0)
		}
		for i, pin := range slices.Concat(n.e.Inputs, n.e.InOuts) {
			if pin.In != nil {
				consider(pin.In.Conns, i)
			}
		}
		row = max(row, 0)
		above := func() int { // the row of its name, kept free
			if n.label && row > 0 {
				return 1
			}
			return 0
		}
		for !free(n.layer, row-above(), n.rows+above()) {
			row++
		}
		n.row = row
		for r := row - above(); r < row+n.rows; r++ {
			taken[[2]int{n.layer, r}] = true
		}
	}

	// A value read by one pin moves to that pin's row when it is free there.
	for _, n := range nodes {
		if len(l.srcs[n.e.ID]) > 0 || len(l.readers[n.e.ID]) != 1 || n.rows != 1 {
			continue
		}
		r := l.readers[n.e.ID][0]
		reader := l.nodes[r.Ref]
		if reader == nil {
			continue
		}
		want := reader.row + max(0, l.inIndex(reader, n.e.ID))
		if want == n.row || !free(n.layer, want, 1) {
			continue
		}
		delete(taken, [2]int{n.layer, n.row})
		n.row = want
		taken[[2]int{n.layer, want}] = true
	}
}

// inIndex is the pin of reader that src feeds; -1 if none of its pins.
func (l *layout) inIndex(reader *node, src int) int {
	for _, p := range slices.Concat(reader.e.Inputs, reader.e.InOuts) {
		if p.In != nil && slices.ContainsFunc(p.In.Conns, func(c Conn) bool { return c.Ref == src }) {
			return slices.IndexFunc(reader.ins, func(s string) bool { return strings.EqualFold(s, p.Param) })
		}
	}
	return -1
}

// outIndex is the row of output param in a block, 0 for any other element.
func outIndex(n *node, param string) int {
	return slices.IndexFunc(n.outs, func(o string) bool { return strings.EqualFold(o, param) })
}

// pins are the inputs and outputs a block shows: those it has and those
// wires read.
func (l *layout) pins(e *Elem) (ins, outs []string) {
	if e.Kind() != "block" {
		return nil, nil
	}
	for _, p := range e.Inputs {
		ins = append(ins, p.Param)
	}
	for _, p := range e.InOuts {
		ins = append(ins, p.Param)
	}
	for _, p := range e.Outputs {
		if !slices.Contains(outs, p.Param) {
			outs = append(outs, p.Param)
		}
	}
	for _, r := range l.readers[e.ID] {
		if r.Param != "" && !slices.ContainsFunc(outs, func(o string) bool { return strings.EqualFold(o, r.Param) }) {
			outs = append(outs, r.Param)
		}
	}
	if len(outs) == 0 {
		outs = []string{""}
	}
	return ins, outs
}

func (l *layout) width(n *node) float64 {
	text := func(s string) float64 { return float64(len(s)) * charW }
	switch n.e.Kind() {
	case "contact", "coil":
		return max(44, text(n.e.Variable)+8)
	case "block":
		in, out := 0.0, 0.0
		for _, p := range n.ins {
			in = max(in, text(p))
		}
		for _, p := range n.outs {
			out = max(out, text(p))
		}
		return max(64, in+out+text(n.e.TypeName)+28, text(n.e.InstanceName)+8)
	case "connector", "continuation":
		return text(n.e.Name) + 24
	}
	return max(36, text(n.e.Expression)+16)
}

// ---- drawing ----

type pen struct {
	b strings.Builder
}

func (p *pen) f(format string, args ...any) { fmt.Fprintf(&p.b, format, args...) }

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")

func esc(s string) string { return escaper.Replace(s) }

func (l *layout) draw() string {
	var p pen
	// Column x positions, per network, from the widest element of each layer.
	type geo struct{ x, y float64 }
	at := map[int]geo{}
	top := float64(margin)
	width := 0.0
	type railLine struct {
		x, y1, y2 float64
		name      string
	}
	var rails []railLine
	for i, net := range l.nets {
		layers := 0
		rows := 0
		for _, n := range net {
			layers = max(layers, n.layer+1)
			rows = max(rows, n.row+n.rows)
		}
		colW := make([]float64, layers)
		for _, n := range net {
			colW[n.layer] = max(colW[n.layer], n.w)
		}
		x0 := float64(margin)
		rail, hasRail := l.netRail[i]
		if hasRail {
			x0 += railGap
		}
		if hasRail && rail.Name != "" {
			top += 16 // the rung's name
		}
		if slices.ContainsFunc(net, func(n *node) bool { return n.label && n.row == 0 }) {
			top += 14 // an instance's name on the first row
		}
		xs := make([]float64, layers)
		x := x0
		for k := range layers {
			xs[k] = x
			x += colW[k] + gapX
		}
		width = max(width, x)
		for _, n := range net {
			at[n.e.ID] = geo{x: xs[n.layer], y: top + float64(n.row)*rowH}
		}
		if hasRail {
			rails = append(rails, railLine{x: float64(margin), y1: top - 4, y2: top + float64(rows)*rowH - 8, name: rail.Name})
		}
		top += float64(rows)*rowH + 20
	}
	height := top

	p.f(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="ui-monospace, Consolas, monospace" font-size="12">`, width+margin, height, width+margin, height)
	p.f(`<style>.w{stroke:var(--vscode-editor-foreground,#333);fill:none;stroke-width:1.2}.t{fill:var(--vscode-editor-foreground,#333)}` +
		`.d{fill:var(--vscode-descriptionForeground,#777)}.j{fill:var(--vscode-editor-foreground,#333)}.b{fill:var(--vscode-editor-background,#fff);stroke:var(--vscode-editor-foreground,#333);stroke-width:1.2}</style>`)
	for _, r := range rails {
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke-width="3"/>`, r.x, r.y1, r.x, r.y2)
		if r.name != "" {
			p.f(`<text class="d" x="%.1f" y="%.1f">RUNG %s</text>`, r.x, r.y1-6, esc(r.name))
		}
	}

	mid := func(id int) float64 { return at[id].y + rowH/2 - 4 }
	// The point a wire leaves an element from, and where it enters one.
	outPoint := func(c Conn) (float64, float64, bool) {
		src, ok := l.elems[c.Ref]
		if !ok {
			return 0, 0, false
		}
		if src.Kind() == "leftPowerRail" {
			return float64(margin), 0, true // y is the reader's
		}
		n := l.nodes[c.Ref]
		g := at[c.Ref]
		y := mid(c.Ref) + float64(n.head+max(0, outIndex(n, c.Param)))*rowH
		return g.x + n.w, y, true
	}
	for _, e := range l.order {
		n := l.nodes[e.ID]
		if n == nil {
			continue
		}
		g := at[e.ID]
		// Wires into it.
		inY := func(param string) float64 {
			i := slices.IndexFunc(n.ins, func(s string) bool { return strings.EqualFold(s, param) })
			return mid(e.ID) + float64(n.head+max(0, i))*rowH
		}
		wiresIn := func(conns []Conn, y float64) {
			for _, c := range conns {
				x1, y1, ok := outPoint(c)
				if !ok {
					continue
				}
				if l.elems[c.Ref].Kind() == "leftPowerRail" {
					y1 = y
				}
				xm := g.x - gapX/3
				p.f(`<polyline class="w" points="%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f"/>`, x1, y1, xm, y1, xm, y, g.x, y)
				if y1 != y && len(l.readers[c.Ref]) > 1 { // a split
					p.f(`<circle class="j" cx="%.1f" cy="%.1f" r="2.5"/>`, xm, y1)
				}
				if y1 != y && len(conns) > 1 { // a join
					p.f(`<circle class="j" cx="%.1f" cy="%.1f" r="2.5"/>`, xm, y)
				}
			}
		}
		for _, in := range e.Ins {
			wiresIn(in.Conns, mid(e.ID))
		}
		for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
			for _, pin := range pins {
				if pin.In != nil {
					wiresIn(pin.In.Conns, inY(pin.Param))
				}
			}
		}
		l.drawElem(&p, n, g.x, g.y, mid(e.ID))
	}
	p.f(`</svg>`)
	return p.b.String()
}

func (l *layout) drawElem(p *pen, n *node, x, y, my float64) {
	e := n.e
	w := n.w
	cx := x + w/2
	switch e.Kind() {
	case "contact":
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, x, my, cx-6, my)
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, cx+6, my, x+w, my)
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/><line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`,
			cx-6, my-9, cx-6, my+9, cx+6, my-9, cx+6, my+9)
		switch {
		case e.Negated:
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, cx-5, my+8, cx+5, my-8)
		case e.Edge == "rising":
			p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle" font-size="10">P</text>`, cx, my+4)
		case e.Edge == "falling":
			p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle" font-size="10">N</text>`, cx, my+4)
		}
		p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, my-12, esc(e.Variable))
	case "coil":
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, x, my, cx-9, my)
		p.f(`<path class="w" d="M%.1f %.1f A12 12 0 0 1 %.1f %.1f M%.1f %.1f A12 12 0 0 0 %.1f %.1f"/>`,
			cx-5, my-9, cx-5, my+9, cx+5, my-9, cx+5, my+9)
		mark := ""
		switch {
		case e.Negated:
			mark = "/"
		case e.Storage == "set":
			mark = "S"
		case e.Storage == "reset":
			mark = "R"
		case e.Edge == "rising":
			mark = "P"
		case e.Edge == "falling":
			mark = "N"
		}
		if mark != "" {
			p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle" font-size="10">%s</text>`, cx, my+4, mark)
		}
		p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, my-12, esc(e.Variable))
	case "block":
		head := float64(n.head) * rowH
		my += head
		p.f(`<rect class="b" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="2"/>`, x, y+head+4, w, float64(n.rows-n.head)*rowH-8)
		p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle" font-weight="bold">%s</text>`, cx, y+head+17, esc(e.TypeName))
		if e.InstanceName != "" {
			p.f(`<text class="d" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, y+head-1, esc(e.InstanceName))
		}
		for i, in := range n.ins {
			py := my + float64(i)*rowH
			label := in
			if strings.HasPrefix(strings.ToUpper(in), "IN") && len(in) > 2 && in[2] >= '0' && in[2] <= '9' && e.InstanceName == "" {
				label = "" // a function's positional inputs need no names
			}
			if pin := pinOf(e, in); pin != nil && pin.Negated {
				p.f(`<circle class="b" cx="%.1f" cy="%.1f" r="3"/>`, x-3, py)
			}
			if label != "" {
				p.f(`<text class="d" x="%.1f" y="%.1f" font-size="10">%s</text>`, x+4, py+4, esc(label))
			}
		}
		for i, out := range n.outs {
			py := my + float64(i)*rowH
			if i < len(e.Outputs) && e.Outputs[i].Negated {
				p.f(`<circle class="b" cx="%.1f" cy="%.1f" r="3"/>`, x+w+3, py)
			}
			if out != "" && !(out == "OUT" && e.InstanceName == "") {
				p.f(`<text class="d" x="%.1f" y="%.1f" font-size="10" text-anchor="end">%s</text>`, x+w-4, py+4, esc(out))
			}
		}
	case "connector", "continuation":
		p.f(`<path class="w" d="M%.1f %.1f h%.1f l8 9 l-8 9 h%.1f z"/>`, x, my-9, w-8, -(w - 8))
		p.f(`<text class="t" x="%.1f" y="%.1f">%s</text>`, x+6, my+4, esc(e.Name))
	default: // variables
		p.f(`<rect class="b" x="%.1f" y="%.1f" width="%.1f" height="18" rx="9"/>`, x, my-9, w)
		text := e.Expression
		if e.Negated || e.NegatedIn || e.NegatedOut {
			text = "NOT " + text
		}
		p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, my+4, esc(text))
	}
}

func pinOf(e *Elem, param string) *Pin {
	for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
		for i := range pins {
			if strings.EqualFold(pins[i].Param, param) {
				return &pins[i]
			}
		}
	}
	return nil
}

// forDrawing returns the elements as a drawing shows them: a function
// contact stands in its rung, powered through EN and continuing from ENO,
// without the AND lowering puts after it.
func forDrawing(elems []*Elem) []*Elem {
	redirect := map[int]int{} // lowering's AND -> its function
	for _, e := range elems {
		if e.Kind() == "block" && e.Name == contactAnd && len(e.Inputs) == 2 && e.Inputs[1].In != nil && len(e.Inputs[1].In.Conns) == 1 {
			redirect[e.ID] = e.Inputs[1].In.Conns[0].Ref
		}
	}
	inRung := map[int]bool{}
	var out []*Elem
	for _, e := range elems {
		if _, ok := redirect[e.ID]; ok {
			continue
		}
		c := *e
		if e.Kind() == "block" && e.Name == contactFunction {
			inRung[e.ID] = true
			var power *PointIn
			if len(e.Ins) > 0 {
				power = &PointIn{Conns: e.Ins[0].Conns}
			}
			c.Ins = nil
			c.Inputs = append([]Pin{{Param: "EN", In: power}}, e.Inputs...)
			c.Outputs = []Pin{{Param: "ENO", Negated: len(e.Outputs) > 0 && e.Outputs[0].Negated}}
		}
		out = append(out, &c)
	}
	fix := func(conns []Conn) []Conn {
		cs := slices.Clone(conns)
		for i, cn := range cs {
			if f, ok := redirect[cn.Ref]; ok {
				cs[i] = Conn{Ref: f, Param: "ENO"}
			} else if inRung[cn.Ref] {
				cs[i].Param = "ENO"
			}
		}
		return cs
	}
	for _, e := range out {
		ins := make([]PointIn, len(e.Ins))
		for i := range e.Ins {
			ins[i] = PointIn{Conns: fix(e.Ins[i].Conns)}
		}
		e.Ins = ins
		for _, pins := range []*[]Pin{&e.Inputs, &e.InOuts} {
			ps := slices.Clone(*pins)
			for i := range ps {
				if ps[i].In != nil {
					ps[i].In = &PointIn{Conns: fix(ps[i].In.Conns)}
				}
			}
			*pins = ps
		}
	}
	return out
}
