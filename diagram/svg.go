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
	"maps"
	"slices"
	"strings"

	"github.com/apiarytech/beedance/token"
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
	l := drawingLayout(elems, nil)
	return l.draw()
}

// SVGWires draws as SVG does, and names on each wire the value it
// carries (Lowered.Wires) as data-expr, so a page can draw the power flow
// from the program's live variables: a wire from the left rail carries
// TRUE.
func SVGWires(elems []*Elem, wires map[Conn]string) string {
	return drawingLayout(elems, drawnWires(elems, wires)).draw()
}

// drawnWires keys the lowering's wires as forDrawing draws them: a
// function in a rung is drawn with EN and ENO, and the AND the lowering
// joins it to the rung with is not drawn, so its value is the ENO's.
func drawnWires(elems []*Elem, wires map[Conn]string) map[Conn]string {
	out := maps.Clone(wires)
	for _, e := range elems {
		if e.Kind() == "block" && e.Name == contactAnd && len(e.Inputs) == 2 && e.Inputs[1].In != nil && len(e.Inputs[1].In.Conns) == 1 {
			if x, ok := wires[Conn{Ref: e.ID}]; ok {
				out[Conn{Ref: e.Inputs[1].In.Conns[0].Ref, Param: "ENO"}] = x
			}
		}
	}
	for _, e := range elems {
		eno := Conn{Ref: e.ID, Param: "ENO"}
		if _, set := out[eno]; !set && e.Kind() == "block" && e.Name == contactFunction {
			if x, ok := wires[Conn{Ref: e.ID}]; ok {
				out[eno] = x
			}
		}
	}
	return out
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
	elems map[int]*Elem
	// inline are the values drawn inside a block, by block and upper-case
	// pin: a variable or literal read by that pin alone (forDrawing).
	inline  map[int]map[string]*Elem
	order   []*Elem
	nodes   map[int]*node
	srcs    map[int][]Conn // element -> wires into it (all inputs)
	readers map[int][]Conn // element -> params read from it (Ref = reader)
	nets    [][]*node
	netRail map[int]*Elem   // network index -> its rail, if any
	wires   map[Conn]string // what each wire carries (SVGWires)
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
				if o.Kind() == "connector" && token.SameIdent(o.Name, e.Name) {
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
		// A block without EN and ENO has a title row, its name alone, above its
		// pins, so a value written inside beside IN1 never meets the name.
		if n.e.Kind() == "block" && !(len(n.ins) > 0 && isPower(n.ins[0])) && len(n.ins)+len(n.outs) > 0 {
			n.head = 1
		}
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
		below := 0
		if n.e.Kind() == "block" {
			below = 1 // the row under a block kept free: a contact there names its variable above it
		}
		for r := row - above(); r < row+n.rows+below; r++ {
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
			return slices.IndexFunc(reader.ins, func(s string) bool { return token.SameIdent(s, p.Param) })
		}
	}
	return -1
}

// outIndex is the row of output param in a block, 0 for any other element.
func outIndex(n *node, param string) int {
	return slices.IndexFunc(n.outs, func(o string) bool { return token.SameIdent(o, param) })
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
		if r.Param != "" && !slices.ContainsFunc(outs, func(o string) bool { return token.SameIdent(o, r.Param) }) {
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
			w := text(p)
			if v := l.inline[n.e.ID][strings.ToUpper(p)]; v != nil {
				w += 6 + text(valueOf(v)) // the value inside, beside the pin
				if name, _ := liveVar(v); name != "" {
					w += liveW // and its value at run time
				}
			}
			if n.e.InstanceName != "" && !isPower(p) {
				w += liveW // the pin's value at run time
			} else if l.dataWire(n.e, p) != "" {
				w += liveW // the value wired in (L-39)
			}
			in = max(in, w)
		}
		for _, p := range n.outs {
			w := text(p)
			if n.e.InstanceName != "" && p != "" && !isPower(p) {
				w += liveW
			} else if n.e.InstanceName == "" && l.wires != nil && p != "" && !isPower(p) {
				w += liveW // a function's result (L-40)
			}
			out = max(out, w)
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

// geo is an element's top left corner.
type geo struct{ x, y float64 }

// railLine is a network's left power rail, as drawn.
type railLine struct {
	net       int
	x, y1, y2 float64
	name      string
}

// geometry places the networks top to bottom, each column as wide as its
// widest element: every element's corner, the rails, and the drawing's
// size. The SVG drawing and the PLCopen XML writer share it.
func (l *layout) geometry() (at map[int]geo, rails []railLine, width, height float64) {
	at = map[int]geo{}
	top := float64(margin)
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
			rails = append(rails, railLine{net: i, x: float64(margin), y1: top - 4, y2: top + float64(rows)*rowH - 8, name: rail.Name})
		}
		top += float64(rows)*rowH + 20
	}
	return at, rails, width, top
}

func (l *layout) draw() string {
	var p pen
	at, rails, width, height := l.geometry()

	p.f(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="ui-monospace, Consolas, monospace" font-size="12">`, width+margin, height, width+margin, height)
	p.f(`<style>.w{stroke:var(--vscode-editor-foreground,#333);fill:none;stroke-width:1.2}.t{fill:var(--vscode-editor-foreground,#333)}` +
		`.d{fill:var(--vscode-descriptionForeground,#777)}.pw{fill:var(--vscode-charts-blue,#1a6fb5);font-weight:bold}.sep{stroke:var(--vscode-descriptionForeground,#999);stroke-dasharray:3 3}.j{fill:var(--vscode-editor-foreground,#333)}.b{fill:var(--vscode-editor-background,#fff);stroke:var(--vscode-editor-foreground,#333);stroke-width:1.2}</style>`)
	for _, r := range rails {
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke-width="3"/>`, r.x, r.y1, r.x, r.y2)
		if r.name != "" {
			p.f(`<text class="d" x="%.1f" y="%.1f">RUNG %s</text>`, r.x, r.y1-6, esc(r.name))
		}
	}

	rt := l.newRouter(at)
	mid := func(id int) float64 { return at[id].y + rowH/2 - 4 }
	for _, e := range l.order {
		n := l.nodes[e.ID]
		if n == nil {
			continue
		}
		g := at[e.ID]
		// Wires into it, around the elements in their way (route.go).
		for _, w := range l.wiresInto(e, at, rt) {
			if x, ok := l.wires[w.conn]; ok {
				p.f(`<polyline class="w" data-expr="%s" points="`, esc(x))
			} else {
				p.f(`<polyline class="w" points="`)
			}
			for i, q := range w.ps {
				if i > 0 {
					p.f(" ")
				}
				p.f("%.1f,%.1f", q.x, q.y)
			}
			p.f(`"/>`)
			if w.split != nil {
				p.f(`<circle class="j" cx="%.1f" cy="%.1f" r="2.5"/>`, w.split.x, w.split.y)
			}
			if w.join != nil {
				p.f(`<circle class="j" cx="%.1f" cy="%.1f" r="2.5"/>`, w.join.x, w.join.y)
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
	// A contact, coil or variable names its variable, so a page can show
	// it live (drone L-4: what is on drawn green): data-var, and data-neg
	// for a negated one.
	if v, kind := liveVar(e); v != "" {
		p.f(`<g class="el %s" data-var="%s" data-neg="%t">`, kind, esc(v), e.Negated || e.NegatedIn || e.NegatedOut)
		defer p.f(`</g>`)
	}
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
		// The box holds its title row (head) and its pins; the name is in the
		// first row, its instance above the box.
		my += float64(n.head) * rowH
		p.f(`<rect class="b" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="2"/>`, x, y+4, w, float64(n.rows)*rowH-8)
		// A timer's or counter's progress along its bottom edge: ET of PT,
		// CV of PV, which the page fills in (L-40).
		if cur, limit := progressOf(e); cur != "" {
			by := y + 4 + float64(n.rows)*rowH - 8 - 5
			p.f(`<g class="prog" data-prog="%s %s"><rect class="pbg" x="%.1f" y="%.1f" width="%.1f" height="3"/><rect class="pf" x="%.1f" y="%.1f" width="0" height="3" data-w="%.1f"/></g>`,
				esc(cur), esc(limit), x+3, by, w-6, x+3, by, w-6)
		}
		p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle" font-weight="bold">%s</text>`, cx, y+17, esc(e.TypeName))
		if n.head > 0 {
			p.f(`<line class="sep" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, x, y+rowH-4, x+w, y+rowH-4)
		}
		if e.InstanceName != "" {
			p.f(`<text class="d" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, y-1, esc(e.InstanceName))
		}
		// EN and ENO are the block's power flow, not its data (IEC 61131-3
		// 6.6.1.2): their row is set apart by a dashed line, and they are
		// labelled in the power's colour.
		if n.rows-n.head > 1 && ((len(n.ins) > 0 && isPower(n.ins[0])) || (len(n.outs) > 0 && isPower(n.outs[0]))) {
			p.f(`<line class="sep" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, x, my+rowH/2, x+w, my+rowH/2)
		}
		for i, in := range n.ins {
			py := my + float64(i)*rowH
			label := in // every input named, a function's IN1 and IN2 too
			if pin := pinOf(e, in); pin != nil && pin.Negated {
				p.f(`<circle class="b" cx="%.1f" cy="%.1f" r="3"/>`, x-3, py)
			}
			if label != "" {
				p.f(`<text class="%s" x="%.1f" y="%.1f" font-size="10">%s</text>`, pinClass(in), x+4, py+4, esc(label))
			}
			// Its value inside, beside the pin (foldValues), live where it is a
			// variable (data-var, as the variable's own element would be).
			cursor := x + 4 + float64(len(label))*charW*10/12 + 6
			if v := l.inline[e.ID][strings.ToUpper(in)]; v != nil {
				vx := cursor
				cursor += float64(len(valueOf(v)))*charW + 6
				if name, kind := liveVar(v); name != "" {
					p.f(`<g class="el %s" data-var="%s" data-neg="%t">`, kind, esc(name), v.Negated || v.NegatedIn || v.NegatedOut)
					p.f(`<text class="t iv" x="%.1f" y="%.1f">%s</text></g>`, vx, py+4, esc(valueOf(v)))
					slot(p, cursor, py+4, "start", name) // its value at run time (L-38)
				} else {
					p.f(`<text class="t iv" x="%.1f" y="%.1f">%s</text>`, vx, py+4, esc(valueOf(v)))
				}
			} else if e.InstanceName != "" && !isPower(in) {
				slot(p, cursor, py+4, "start", e.InstanceName+"."+in) // the pin's value at run time
			} else if x := l.dataWire(e, in); x != "" {
				exprSlot(p, cursor, py+4, "start", x) // the value wired in from another block (L-39)
			}
		}
		for i, out := range n.outs {
			py := my + float64(i)*rowH
			if i < len(e.Outputs) && e.Outputs[i].Negated {
				p.f(`<circle class="b" cx="%.1f" cy="%.1f" r="3"/>`, x+w+3, py)
			}
			switch {
			case out != "" && !(out == "OUT" && e.InstanceName == ""):
				if e.InstanceName != "" && !isPower(out) {
					// An instance's output: green while a BOOL one is TRUE, its
					// value beside it (L-38, L-40).
					p.f(`<g class="el pin" data-var="%s" data-neg="false">`, esc(e.InstanceName+"."+out))
					p.f(`<text class="%s" x="%.1f" y="%.1f" font-size="10" text-anchor="end">%s</text></g>`, pinClass(out), x+w-4, py+4, esc(out))
					slot(p, x+w-4-float64(len(out))*charW*10/12-6, py+4, "end", e.InstanceName+"."+out)
				} else {
					p.f(`<text class="%s" x="%.1f" y="%.1f" font-size="10" text-anchor="end">%s</text>`, pinClass(out), x+w-4, py+4, esc(out))
				}
			case out != "":
				// A function's result, held by no variable: worked out by the
				// page from the expression it is (L-40).
				if v := l.resultOf(e, i); v != "" {
					exprSlot(p, x+w-4, py+4, "end", v)
				}
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
		// An output variable: its value beside the box (L-40).
		if (e.Kind() == "outVariable" || e.Kind() == "inOutVariable") && isSimpleOperand(strings.TrimSpace(e.Expression)) {
			slot(p, x+w+4, my+4, "start", strings.TrimSpace(e.Expression))
		}
	}
}

func pinOf(e *Elem, param string) *Pin {
	for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
		for i := range pins {
			if token.SameIdent(pins[i].Param, param) {
				return &pins[i]
			}
		}
	}
	return nil
}

// forDrawing returns the elements as a drawing shows them: a function
// contact stands in its rung, powered through EN and continuing from ENO,
// without the AND lowering puts after it.
func forDrawing(elems []*Elem) ([]*Elem, map[int]map[string]*Elem) {
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
	return foldValues(out)
}

// foldValues takes a block's input values into the block (drone review: a
// block lists all its inputs inside it): a variable or literal read by one
// pin of a block, and by nothing else, is drawn beside the pin's name, not
// as an element of its own wired to it. The rest are as they were.
func foldValues(elems []*Elem) ([]*Elem, map[int]map[string]*Elem) {
	byID := map[int]*Elem{}
	readers := map[int]int{}
	for _, e := range elems {
		byID[e.ID] = e
		for _, in := range e.Ins {
			for _, c := range in.Conns {
				readers[c.Ref]++
			}
		}
		for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
			for _, p := range pins {
				if p.In != nil {
					for _, c := range p.In.Conns {
						readers[c.Ref]++
					}
				}
			}
		}
	}
	inline := map[int]map[string]*Elem{}
	folded := map[int]bool{}
	for _, e := range elems {
		if e.Kind() != "block" {
			continue
		}
		for i, p := range e.Inputs {
			if p.In == nil || len(p.In.Conns) != 1 || isPower(p.Param) {
				continue
			}
			src := byID[p.In.Conns[0].Ref]
			if src == nil || src.Kind() != "inVariable" || readers[src.ID] != 1 || strings.TrimSpace(src.Expression) == "" {
				continue
			}
			if inline[e.ID] == nil {
				inline[e.ID] = map[string]*Elem{}
			}
			inline[e.ID][strings.ToUpper(p.Param)] = src
			folded[src.ID] = true
			e.Inputs[i].In = nil // no wire: the value is inside
		}
	}
	out := elems[:0:0]
	for _, e := range elems {
		if !folded[e.ID] {
			out = append(out, e)
		}
	}
	return out, inline
}

// liveVar returns the variable a contact, coil or variable element shows,
// and its kind; "" for an expression that is not one variable.
func liveVar(e *Elem) (string, string) {
	switch e.Kind() {
	case "contact", "coil":
		return e.Variable, e.Kind()
	case "block", "connector", "continuation":
		return "", ""
	}
	if isName(e.Expression) {
		return e.Expression, "var"
	}
	return "", ""
}

// isName says whether s is a variable, possibly dotted (Tank.Level).
func isName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		letter := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		if !(letter || (i > 0 && (r == '.' || (r >= '0' && r <= '9')))) {
			return false
		}
	}
	return true
}

// isPower reports whether a pin is EN or ENO, a block's power flow.
func isPower(pin string) bool { return token.SameIdent(pin, "EN") || token.SameIdent(pin, "ENO") }

// pinClass is a pin label's class: pw for EN and ENO, d for data.
func pinClass(pin string) string {
	if isPower(pin) {
		return "pw"
	}
	return "d"
}

// drawingLayout lays out a body's elements as drawn: with forDrawing's
// changes, and each block wide enough for the values inside it.
// drawingLayout lays out a body to draw; wires (SVGWires) also leave room
// for the values the page shows on data inputs (dataWire).
func drawingLayout(elems []*Elem, wires map[Conn]string) *layout {
	drawn, inline := forDrawing(elems)
	l := newLayout(drawn)
	l.inline = inline
	l.wires = wires
	for _, net := range l.nets {
		for _, n := range net {
			n.w = l.width(n)
		}
	}
	return l
}

// valueOf is a variable or literal element's text, as its own element
// would show it.
func valueOf(e *Elem) string {
	if e.Negated || e.NegatedIn || e.NegatedOut {
		return "NOT " + e.Expression
	}
	return e.Expression
}

// liveW is the room a pin's value at run time takes inside a block: about
// nine characters at the pins' size ("T#1m30s", "-123.456").
const liveW = 6 + 9*charW*10/12

// slot is an empty text a page fills with a value at run time (drone
// L-38): data-vals "=<name>" asks for the value alone.
func slot(p *pen, x, y float64, anchor, name string) {
	p.f(`<text class="lv" x="%.1f" y="%.1f" font-size="10" text-anchor="%s" data-vals="=%s"></text>`, x, y, anchor, esc(name))
}

// exprSlot is a value the page works out from an expression over the
// program's variables (data-val-expr): a block's result, which no variable
// holds at run time (drone L-39).
func exprSlot(p *pen, x, y float64, anchor, expr string) {
	p.f(`<text class="lv" x="%.1f" y="%.1f" font-size="10" text-anchor="%s" data-val-expr="%s"></text>`, x, y, anchor, esc(expr))
}

// dataWire is the value wired into a function's data input, as the
// expression the wires carry: ADD's result into MUL's IN1, a comparison's
// BOOL into AND, a timer's Q, a variable read by several pins, wires
// joined at the pin (their OR). "" where the page has nothing to add: no
// wires named, an instance's input (its own value shows), EN, a pin not
// wired (a value inside the block shows itself) (L-39, L-41).
func (l *layout) dataWire(e *Elem, in string) string {
	if l.wires == nil || e.InstanceName != "" || isPower(in) || l.inline[e.ID][strings.ToUpper(in)] != nil {
		return ""
	}
	pin := pinOf(e, in)
	if pin == nil || pin.In == nil || len(pin.In.Conns) == 0 {
		return ""
	}
	var parts []string
	for _, c := range pin.In.Conns {
		x, ok := l.wires[c]
		if !ok {
			return ""
		}
		parts = append(parts, x)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// resultOf is a function's i-th drawn output as the expression the
// lowering made of it, from a wire that carries it; "" when no wire does.
func (l *layout) resultOf(e *Elem, i int) string {
	if l.wires == nil {
		return ""
	}
	n := l.nodes[e.ID]
	if n == nil || i >= len(n.outs) {
		return ""
	}
	for _, c := range []Conn{{Ref: e.ID, Param: n.outs[i]}, {Ref: e.ID}} {
		if x, ok := l.wires[c]; ok && (c.Param != "" || i == firstData(n.outs)) {
			return x
		}
	}
	return ""
}

// firstData is the index of the first output that is not ENO.
func firstData(outs []string) int {
	for i, o := range outs {
		if !isPower(o) {
			return i
		}
	}
	return -1
}

// progressOf names what a standard timer's or counter's progress is: its
// elapsed time of its preset (ET, PT), or its count of its preset (CV,
// PV); "" for other blocks.
func progressOf(e *Elem) (cur, limit string) {
	if e.InstanceName == "" {
		return "", ""
	}
	switch strings.ToUpper(e.TypeName) {
	case "TON", "TOF", "TP":
		return e.InstanceName + ".ET", e.InstanceName + ".PT"
	case "CTU", "CTD", "CTUD":
		return e.InstanceName + ".CV", e.InstanceName + ".PV"
	}
	return "", ""
}
