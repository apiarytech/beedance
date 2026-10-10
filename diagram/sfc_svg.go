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

// This file draws a chart (sfc.go) as SVG, in the style of svg.go. The text
// form has no layout, so one is computed:
//
//   - the chart is walked depth first from the initial step; a transition
//     back to a step on the walk's path, a loop, is drawn as a jump: an
//     arrow naming the step, as IEC 61131-3 draws it, so no wire runs
//     upwards;
//   - what remains is acyclic, and an element's rank (its row) is the
//     longest path to it, steps on even ranks and transitions on odd ones,
//     so every branch of a convergence ends on the same row;
//   - an element shares the column of the element it follows, if it is
//     that element's first follower; other followers open columns to the
//     right, so branches never share a column;
//   - between two rows, the bar a step or transition divides on is drawn
//     a third of the way down, and the bar a step or transition joins on a
//     third of the way up, so the two never meet. A selection's bars are
//     single lines, a simultaneous branch's double ones.

import (
	"slices"
	"strings"
)

// Chart layout sizes, in SVG user units.
const (
	sfcStepH   = 28 // a step's box
	sfcTransH  = 24 // a transition's row
	sfcJumpH   = 24 // a jump's row
	sfcActH    = 20 // an action's row
	sfcGap     = 30 // between rows: room for bars
	sfcColGap  = 40 // between columns
	sfcBar     = 11 // half a transition's bar
	sfcMaxCond = 48 // characters of a condition shown
)

type sfcKind int

const (
	sfcStepNode sfcKind = iota
	sfcTransNode
	sfcJumpNode
)

type sfcNode struct {
	kind   sfcKind
	index  int    // into Chart.Steps or Chart.Transitions
	target string // a jump's step
	rank   int
	col    int
	parent int   // the node the walk first reached it from, or -1
	ins    []int // nodes wired into it, in the drawing's acyclic graph
	outs   []int // nodes it is wired to
}

type sfcLayout struct {
	c       *Chart
	nodes   []*sfcNode
	stepOf  map[string]int // upper-case step name -> node
	transOf []int          // transition -> node
	pre     []int          // nodes in the order the walk reached them
	cols    int
}

// SFCSVG draws a chart as an SVG document. Colours follow the editor's
// theme where the page sets VS Code's CSS variables, with plain fallbacks
// elsewhere.
func SFCSVG(c *Chart) string {
	l := newSFCLayout(c)
	return l.draw()
}

func newSFCLayout(c *Chart) *sfcLayout {
	l := &sfcLayout{c: c, stepOf: map[string]int{}}
	for i, s := range c.Steps {
		l.stepOf[strings.ToUpper(s.Name)] = len(l.nodes)
		l.nodes = append(l.nodes, &sfcNode{kind: sfcStepNode, index: i})
	}
	for i := range c.Transitions {
		l.transOf = append(l.transOf, len(l.nodes))
		l.nodes = append(l.nodes, &sfcNode{kind: sfcTransNode, index: i})
	}
	l.walk()
	l.rank()
	l.columns()
	return l
}

// next are a node's followers in the chart, in order: a step's
// transitions, a transition's steps.
func (l *sfcLayout) next(n int) []int {
	nd := l.nodes[n]
	var out []int
	switch nd.kind {
	case sfcStepNode:
		name := l.c.Steps[nd.index].Name
		for i, t := range l.c.Transitions {
			if slices.ContainsFunc(t.From, func(s string) bool { return strings.EqualFold(s, name) }) {
				out = append(out, l.transOf[i])
			}
		}
	case sfcTransNode:
		for _, s := range l.c.Transitions[nd.index].To {
			if m, ok := l.stepOf[strings.ToUpper(s)]; ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// walk builds the acyclic graph, loops turned into jumps. Each node but
// the walk's roots gets the node it was first reached from, its parent,
// and pre lists the nodes in the order the walk reached them.
func (l *sfcLayout) walk() {
	const (
		unseen = iota
		onPath
		done
	)
	state := make([]int, len(l.nodes))
	link := func(from, to int) {
		l.nodes[from].outs = append(l.nodes[from].outs, to)
		l.nodes[to].ins = append(l.nodes[to].ins, from)
	}
	reach := func(n, parent int) {
		l.nodes[n].parent = parent
		l.pre = append(l.pre, n)
	}
	var visit func(n int)
	visit = func(n int) {
		state[n] = onPath
		for _, m := range l.next(n) {
			if state[m] == onPath {
				if l.nodes[n].kind != sfcTransNode {
					continue // a step back into a transition on the path: nothing to draw
				}
				j := len(l.nodes)
				l.nodes = append(l.nodes, &sfcNode{kind: sfcJumpNode, target: l.c.Steps[l.nodes[m].index].Name})
				state = append(state, done)
				link(n, j)
				reach(j, n)
				continue
			}
			link(n, m)
			if state[m] == unseen {
				reach(m, n)
				visit(m)
			}
		}
		state[n] = done
	}
	var roots []int
	for _, s := range l.c.Steps {
		if s.Initial {
			roots = append(roots, l.stepOf[strings.ToUpper(s.Name)])
		}
	}
	for n := range l.nodes {
		roots = append(roots, n)
	}
	for _, r := range roots {
		if state[r] == unseen {
			reach(r, -1)
			visit(r)
		}
	}
}

// span is the rows a column is drawn on by one chain, in rank units.
type span struct{ lo, hi float64 }

// columns puts each chain of the walk (a node, its first child, that
// child's first child, ...) in one column: the main chain leftmost, every
// other chain in the leftmost column right of its parent's that is free
// over the rows it is drawn on, and free where the bar it hangs from
// crosses the columns between.
func (l *sfcLayout) columns() {
	var used [][]span
	free := func(c int, s span) bool {
		if c >= len(used) {
			return true
		}
		for _, u := range used[c] {
			if u.lo < s.hi && s.lo < u.hi {
				return false
			}
		}
		return true
	}
	firstChild := func(n int) int {
		for _, m := range l.nodes[n].outs {
			if l.nodes[m].parent == n {
				return m
			}
		}
		return -1
	}
	placed := make([]bool, len(l.nodes))
	for _, head := range l.pre {
		if placed[head] {
			continue
		}
		var chain []int
		for n := head; n >= 0; n = firstChild(n) {
			chain = append(chain, n)
		}
		// The rows it is drawn on: from the bar under its parent to its
		// last node, or to the bar it joins further down.
		s := span{lo: float64(l.nodes[head].rank) - 0.5, hi: float64(l.nodes[chain[len(chain)-1]].rank) + 0.5}
		parent := l.nodes[head].parent
		from := 0
		if parent >= 0 {
			s.lo = float64(l.nodes[parent].rank) + 0.2
			from = l.nodes[parent].col + 1
		}
		for _, n := range chain {
			for _, m := range l.nodes[n].outs {
				if l.nodes[m].parent != n || m != firstChild(n) {
					s.hi = max(s.hi, float64(l.nodes[m].rank)-0.2)
				}
			}
		}
		c := from
		for {
			crossing := span{lo: s.lo, hi: s.lo + 0.1}
			ok := free(c, s)
			for k := from; ok && k < c; k++ {
				ok = free(k, crossing)
			}
			if ok {
				break
			}
			c++
		}
		for len(used) <= c {
			used = append(used, nil)
		}
		used[c] = append(used[c], s)
		for _, n := range chain {
			l.nodes[n].col = c
			placed[n] = true
		}
		l.cols = max(l.cols, c+1)
	}
}

// rank gives each node the longest path to it: steps and jumps on even
// rows, transitions on odd ones.
func (l *sfcLayout) rank() {
	waiting := make([]int, len(l.nodes))
	var ready []int
	for n, nd := range l.nodes {
		waiting[n] = len(nd.ins)
		if waiting[n] == 0 {
			ready = append(ready, n)
			if nd.kind == sfcTransNode {
				nd.rank = 1
			}
		}
	}
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		for _, m := range l.nodes[n].outs {
			l.nodes[m].rank = max(l.nodes[m].rank, l.nodes[n].rank+1)
			if waiting[m]--; waiting[m] == 0 {
				ready = append(ready, m)
			}
		}
	}
}

func textW(s string) float64 { return float64(len(s)) * charW }

func (l *sfcLayout) stepW(i int) float64 { return max(56, textW(l.c.Steps[i].Name)+24) }

// condText is a transition's condition as shown, cut short if long.
func (l *sfcLayout) condText(i int) string {
	c := strings.Join(strings.Fields(l.c.Transitions[i].Condition), " ")
	if len(c) > sfcMaxCond {
		c = c[:sfcMaxCond-1] + "…"
	}
	return c
}

func qualText(a Association) string {
	q := a.Qualifier
	if q == "" {
		q = "N"
	}
	if a.Duration != "" {
		q += " " + a.Duration
	}
	return q
}

// actionCols are the widths of a step's action table: qualifiers, names.
func (l *sfcLayout) actionCols(i int) (float64, float64) {
	qw, nw := 28.0, 40.0
	for _, a := range l.c.Steps[i].Actions {
		qw = max(qw, textW(qualText(a))+12)
		nw = max(nw, textW(a.Action)+16)
	}
	return qw, nw
}

// sfcGeo is where the layout puts things.
type sfcGeo struct {
	spine         []float64 // a column's centre line
	colStep       []float64 // a column's step width
	rowTop, rowH  []float64
	width, height float64
}

func (l *sfcLayout) geometry() sfcGeo {
	ranks := 0
	for _, nd := range l.nodes {
		ranks = max(ranks, nd.rank+1)
	}
	g := sfcGeo{spine: make([]float64, l.cols), colStep: make([]float64, l.cols), rowTop: make([]float64, ranks), rowH: make([]float64, ranks)}
	right := make([]float64, l.cols) // what a column needs right of its spine
	for c := range g.colStep {
		g.colStep[c] = 56
		right[c] = 28
	}
	for _, nd := range l.nodes {
		switch nd.kind {
		case sfcStepNode:
			w := l.stepW(nd.index)
			g.colStep[nd.col] = max(g.colStep[nd.col], w)
			r := w / 2
			if len(l.c.Steps[nd.index].Actions) > 0 {
				qw, nw := l.actionCols(nd.index)
				r += 16 + qw + nw
			}
			right[nd.col] = max(right[nd.col], r)
			g.rowH[nd.rank] = max(g.rowH[nd.rank], max(sfcStepH, float64(len(l.c.Steps[nd.index].Actions))*sfcActH))
		case sfcTransNode:
			t := l.c.Transitions[nd.index]
			r := sfcBar + 8 + textW(l.condText(nd.index))
			if t.Name != "" {
				r += textW(t.Name) + 8
			}
			right[nd.col] = max(right[nd.col], r)
			g.rowH[nd.rank] = max(g.rowH[nd.rank], sfcTransH)
		case sfcJumpNode:
			right[nd.col] = max(right[nd.col], 14+textW(nd.target))
			g.rowH[nd.rank] = max(g.rowH[nd.rank], sfcJumpH)
		}
	}
	for c := range right {
		right[c] = max(right[c], g.colStep[c]/2)
	}
	x := float64(margin)
	for c := range g.spine {
		g.spine[c] = x + g.colStep[c]/2
		x = g.spine[c] + right[c] + sfcColGap
	}
	g.width = x - sfcColGap + margin
	y := float64(margin)
	for r := range g.rowTop {
		g.rowTop[r] = y
		y += g.rowH[r] + sfcGap
	}
	g.height = y - sfcGap + margin
	return g
}

func (l *sfcLayout) draw() string {
	var p pen
	g := l.geometry()
	p.f(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="ui-monospace, Consolas, monospace" font-size="12">`, g.width, g.height, g.width, g.height)
	p.f(`<style>.w{stroke:var(--vscode-editor-foreground,#333);fill:none;stroke-width:1.2}.t{fill:var(--vscode-editor-foreground,#333)}` +
		`.d{fill:var(--vscode-descriptionForeground,#777)}.j{fill:var(--vscode-editor-foreground,#333)}.b{fill:var(--vscode-editor-background,#fff);stroke:var(--vscode-editor-foreground,#333);stroke-width:1.2}</style>`)

	x := func(n int) float64 { return l.xOf(g, n) }
	top := func(n int) float64 { return l.top(g, n) }
	bottom := func(n int) float64 { return l.bottom(g, n) }
	divideY := func(n int) float64 { return l.divideY(g, n) }
	joinY := func(n int) float64 { return l.joinY(g, n) }

	bar := func(x1, x2, y float64, double bool) {
		if double {
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/><line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`,
				x1-6, y-1.5, x2+6, y-1.5, x1-6, y+1.5, x2+6, y+1.5)
			return
		}
		p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, x1, y, x2, y)
	}
	line := func(x1, y1, x2, y2 float64) {
		if y1 == y2 && x1 == x2 {
			return
		}
		if x1 == x2 {
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, x1, y1, x2, y2)
			return
		}
		ym := max(y1, y2-sfcGap/2+2)
		p.f(`<polyline class="w" points="%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f"/>`, x1, y1, x1, ym, x2, ym, x2, y2)
	}

	// Wires and bars.
	for n, nd := range l.nodes {
		if len(nd.outs) > 1 {
			xs := []float64{x(n)}
			for _, m := range nd.outs {
				xs = append(xs, x(m))
			}
			y := divideY(n)
			bar(slices.Min(xs), slices.Max(xs), y, nd.kind == sfcTransNode)
			line(x(n), bottom(n), x(n), y)
		}
		if len(nd.ins) > 1 {
			xs := []float64{x(n)}
			for _, m := range nd.ins {
				xs = append(xs, x(m))
			}
			y := joinY(n)
			bar(slices.Min(xs), slices.Max(xs), y, nd.kind == sfcTransNode)
			line(x(n), y, x(n), top(n))
		}
		for _, m := range nd.outs {
			x1, y1 := x(n), bottom(n)
			if len(nd.outs) > 1 {
				x1, y1 = x(m), divideY(n)
			}
			x2, y2 := x(m), top(m)
			if len(l.nodes[m].ins) > 1 {
				x2, y2 = x1, joinY(m)
			}
			line(x1, y1, x2, y2)
		}
	}

	// Elements, over the wires.
	for n, nd := range l.nodes {
		cx, y := x(n), g.rowTop[nd.rank]
		switch nd.kind {
		case sfcStepNode:
			s := l.c.Steps[nd.index]
			w := l.stepW(nd.index)
			p.f(`<rect class="b" x="%.1f" y="%.1f" width="%.1f" height="%d"/>`, cx-w/2, y, w, sfcStepH)
			if s.Initial {
				p.f(`<rect class="w" x="%.1f" y="%.1f" width="%.1f" height="%d"/>`, cx-w/2+3, y+3, w-6, sfcStepH-6)
			}
			p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, y+sfcStepH/2+4, esc(s.Name))
			if len(s.Actions) == 0 {
				continue
			}
			qw, nw := l.actionCols(nd.index)
			ax := cx + w/2 + 16
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, cx+w/2, y+sfcActH/2, ax, y+sfcActH/2)
			for k, a := range s.Actions {
				ay := y + float64(k)*sfcActH
				p.f(`<rect class="b" x="%.1f" y="%.1f" width="%.1f" height="%d"/><rect class="b" x="%.1f" y="%.1f" width="%.1f" height="%d"/>`,
					ax, ay, qw, sfcActH, ax+qw, ay, nw, sfcActH)
				q := a.Qualifier
				if q == "" {
					q = "N"
				}
				if a.Duration != "" {
					p.f(`<text class="t" x="%.1f" y="%.1f">%s <tspan class="d">%s</tspan></text>`, ax+6, ay+14, esc(q), esc(a.Duration))
				} else {
					p.f(`<text class="t" x="%.1f" y="%.1f">%s</text>`, ax+6, ay+14, esc(q))
				}
				p.f(`<text class="t" x="%.1f" y="%.1f">%s</text>`, ax+qw+8, ay+14, esc(a.Action))
			}
		case sfcTransNode:
			t := l.c.Transitions[nd.index]
			ty := y + sfcTransH/2
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke-width="2.4"/>`, cx-sfcBar, ty, cx+sfcBar, ty)
			tx := cx + sfcBar + 8
			if t.Name != "" {
				p.f(`<text class="d" x="%.1f" y="%.1f">%s</text>`, tx, ty+4, esc(t.Name))
				tx += textW(t.Name) + 8
			}
			p.f(`<text class="t" x="%.1f" y="%.1f">%s</text>`, tx, ty+4, esc(l.condText(nd.index)))
		case sfcJumpNode:
			ay := y + sfcJumpH - 10
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, cx, y, cx, ay)
			p.f(`<path class="j" d="M%.1f %.1f h12 l-6 10 z"/>`, cx-6, ay)
			p.f(`<text class="t" x="%.1f" y="%.1f">%s</text>`, cx+12, ay+9, esc(nd.target))
		}
	}
	p.f(`</svg>`)
	return p.b.String()
}

// xOf is a node's centre line.
func (l *sfcLayout) xOf(g sfcGeo, n int) float64 { return g.spine[l.nodes[n].col] }

// top is where wires enter a node: a step's or jump's top, a transition's
// bar.
func (l *sfcLayout) top(g sfcGeo, n int) float64 {
	nd := l.nodes[n]
	if nd.kind == sfcTransNode {
		return g.rowTop[nd.rank] + sfcTransH/2
	}
	return g.rowTop[nd.rank]
}

// bottom is where wires leave a node.
func (l *sfcLayout) bottom(g sfcGeo, n int) float64 {
	nd := l.nodes[n]
	if nd.kind == sfcTransNode {
		return g.rowTop[nd.rank] + sfcTransH/2
	}
	return g.rowTop[nd.rank] + sfcStepH
}

// divideY is the bar a node divides on, below its row; joinY the bar it
// joins on, above its row.
func (l *sfcLayout) divideY(g sfcGeo, n int) float64 {
	r := l.nodes[n].rank
	return g.rowTop[r] + g.rowH[r] + sfcGap/3
}

func (l *sfcLayout) joinY(g sfcGeo, n int) float64 { return g.rowTop[l.nodes[n].rank] - sfcGap/3 }
