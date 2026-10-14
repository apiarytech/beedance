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
	"fmt"
	"slices"
	"strings"

	"github.com/apiarytech/beedance/token"
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
			if slices.ContainsFunc(t.From, func(s string) bool { return token.SameIdent(s, name) }) {
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
				r += l.actionValuesW(nd.index) // the values beside the actions, in full (L-34, L-43)
			}
			right[nd.col] = max(right[nd.col], r)
			g.rowH[nd.rank] = max(g.rowH[nd.rank], max(sfcStepH, float64(len(l.c.Steps[nd.index].Actions))*sfcActH))
		case sfcTransNode:
			t := l.c.Transitions[nd.index]
			r := sfcBar + 8 + textW(l.condText(nd.index))
			vals := 0.0 // the values line under the condition: "name = value" each
			for _, n := range condVars(t.Condition) {
				vals += float64(len(n)+12) * charW * 10 / 12
			}
			for _, x := range condExprs(t.Condition) { // "expr = value" each (L-39)
				vals += float64(len(x)+14) * charW * 10 / 12
			}
			r = max(r, sfcBar+8+vals)
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
	// Left of the spine: half the step, or a step's time under it, right
	// aligned to the link, in full (longestTime, L-44).
	left := make([]float64, l.cols)
	for c := range left {
		left[c] = max(g.colStep[c]/2, stepTimeW)
	}
	x := float64(margin)
	for c := range g.spine {
		g.spine[c] = x + left[c]
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
		`.d{fill:var(--vscode-descriptionForeground,#777)}.lv{fill:var(--vscode-charts-blue,#0b5c73);font-weight:600}.j{fill:var(--vscode-editor-foreground,#333)}.b{fill:var(--vscode-editor-background,#fff);stroke:var(--vscode-editor-foreground,#333);stroke-width:1.2}</style>`)

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
			// The step names its flag, as package sfc lowers it (_sfc_<step>_X),
			// so a page can show the active steps live (drone L-4).
			p.f(`<g class="el var" data-var="_sfc_%s_X" data-neg="false">`, esc(s.Name))
			p.f(`<rect class="b" x="%.1f" y="%.1f" width="%.1f" height="%d"/>`, cx-w/2, y, w, sfcStepH)
			if s.Initial {
				p.f(`<rect class="w" x="%.1f" y="%.1f" width="%.1f" height="%d"/>`, cx-w/2+3, y+3, w-6, sfcStepH-6)
			}
			p.f(`<text class="t" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, cx, y+sfcStepH/2+4, esc(s.Name))
			p.f(`</g>`)
			// How long it has been active (IEC <step>.T, package sfc's
			// _sfc_<step>_E), under it beside the link (L-40).
			slot(&p, cx-6, y+sfcStepH+11, "end", "_sfc_"+s.Name+"_E.ET")
			if len(s.Actions) == 0 {
				continue
			}
			qw, nw := l.actionCols(nd.index)
			ax := cx + w/2 + 16
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, cx+w/2, y+sfcActH/2, ax, y+sfcActH/2)
			for k, a := range s.Actions {
				ay := y + float64(k)*sfcActH
				// The action's state, live: its row green while it is active (its
				// _sfc_a<n>_Q, drone L-35).
				n, known := l.c.ActionIndex[strings.ToUpper(a.Action)]
				if known {
					// Its step and qualifier too: a P action is active for one
					// scan only, so a page holds it green a moment when its step
					// becomes active (drone L-42).
					p.f(`<g class="el var" data-var="_sfc_a%d_Q" data-neg="false" data-step="%s" data-qual="%s">`, n, esc(s.Name), esc(strings.ToUpper(a.Qualifier)))
				}
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
				if known {
					p.f(`</g>`)
				}
				// Beside it, live: a stored timed qualifier's time run (SD, DS, SL;
				// a D or L action's is its step's time, under the step, L-43), and
				// the action's variable's value if it is one (a BOOL set by N...).
				var vals []string
				if known && storedTimed(q) {
					vals = append(vals, fmt.Sprintf("elapsed=_sfc_a%d_t.ET", n))
				}
				if l.isVariable(a.Action) {
					vals = append(vals, a.Action)
				}
				if len(vals) > 0 {
					p.f(`<text class="lv" x="%.1f" y="%.1f" font-size="10" data-vals="%s"></text>`, ax+qw+nw+6, ay+14, esc(strings.Join(vals, " ")))
				}
			}
		case sfcTransNode:
			t := l.c.Transitions[nd.index]
			ty := y + sfcTransH/2
			// The bar names its condition, so a page draws it green while it
			// is TRUE (L-40).
			p.f(`<line class="w" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke-width="2.4" data-expr="%s"/>`, cx-sfcBar, ty, cx+sfcBar, ty, esc(t.Condition))
			tx := cx + sfcBar + 8
			if t.Name != "" {
				p.f(`<text class="d" x="%.1f" y="%.1f">%s</text>`, tx, ty+4, esc(t.Name))
				tx += textW(t.Name) + 8
			}
			p.f(`<text class="t" x="%.1f" y="%.1f">%s</text>`, tx, ty+4, esc(l.condText(nd.index)))
			// Its variables' values, which a page fills in live (drone L-34).
			// Its computed parts (WellLevel + 5.0) after them, as expressions the
			// page works out (data-val-exprs, "|" between them; L-39).
			names, exprs := condVars(t.Condition), condExprs(t.Condition)
			switch {
			case len(exprs) > 0:
				p.f(`<text class="lv" x="%.1f" y="%.1f" font-size="10" data-vals="%s" data-val-exprs="%s"></text>`, cx+sfcBar+8, ty+16, esc(strings.Join(names, " ")), esc(strings.Join(exprs, "|")))
			case len(names) > 0:
				p.f(`<text class="lv" x="%.1f" y="%.1f" font-size="10" data-vals="%s"></text>`, cx+sfcBar+8, ty+16, esc(strings.Join(names, " ")))
			}
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

// condVars are the variables a transition's condition reads, in order,
// each once: names and members (rest.Q), not keywords, literals or calls.
func condVars(cond string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range condName.get().FindAllStringSubmatchIndex(cond, -1) {
		name := cond[m[2]:m[3]]
		rest := strings.TrimLeft(cond[m[1]:], " ")
		up := strings.ToUpper(name)
		if condWords[up] || strings.HasPrefix(rest, "(") || (m[2] > 0 && cond[m[2]-1] == '#') || seen[up] {
			continue // a keyword, a call, a typed literal's value
		}
		if strings.HasPrefix(rest, "#") {
			continue // T#5s, INT#3: a literal's type
		}
		seen[up] = true
		out = append(out, name)
	}
	return out
}

// condName compiles on first use (lazyRegexp): drawing a chart is not part
// of lowering one, so firmware that parses charts does not carry regexp.
var condName = &lazyRegexp{src: `(?:^|[^A-Za-z0-9_#.])([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`}

var condWords = map[string]bool{"AND": true, "OR": true, "XOR": true, "NOT": true, "MOD": true, "TRUE": true, "FALSE": true, "AND_THEN": true, "OR_ELSE": true}

// storedTimed reports whether a qualifier times an action that outlives its
// step: SD, DS and SL. D and L time it while the step is active, so their
// time is the step's own (L-43).
func storedTimed(q string) bool {
	switch strings.ToUpper(q) {
	case "SD", "DS", "SL":
		return true
	}
	return false
}

// actionValuesW is the room the values beside a step's actions take: the
// widest of "elapsed = T#1h23m45s678ms" and "Name = -12345.678", in the
// values' smaller font; 8 for a step whose actions show none.
func (l *sfcLayout) actionValuesW(step int) float64 {
	w := 0.0
	for _, a := range l.c.Steps[step].Actions {
		n := 0
		if _, known := l.c.ActionIndex[strings.ToUpper(a.Action)]; known && storedTimed(a.Qualifier) {
			n += len("elapsed = "+longestTime) + 3
		}
		if l.isVariable(a.Action) {
			n += len(a.Action) + len(" = -12345.678") + 3
		}
		w = max(w, float64(n)*charW*10/12)
	}
	return w + 8
}

// maxCondExprs bounds the computed parts shown under one condition.
const maxCondExprs = 6

// condExprs are the computed parts of a transition's condition, which no
// variable holds at run time: each side of a comparison, and each input of
// a comparison function, that is not a plain name or literal
// (WellLevel + 5.0, ADD(a, b)); at most maxCondExprs, in order, each once.
// A page shows their values beside the variables' (drone L-39).
func condExprs(cond string) []string {
	var out []string
	seen := map[string]bool{}
	// add takes a computed part, then the computed inputs it has: a
	// function's arguments, an operator's operands (L-41).
	var add func(s string, depth int)
	add = func(s string, depth int) {
		s = strings.TrimSpace(s)
		if s == "" || depth > 8 || isSimpleOperand(s) || len(out) >= maxCondExprs {
			return
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
		inner := stripParens(s)
		if _, args, ok := callOf(inner); ok {
			for _, a := range args {
				add(a, depth+1)
			}
			return
		}
		for _, ops := range [][]string{{"+", "-"}, {"*", "/", "MOD"}} {
			if parts := splitArith(inner, ops); len(parts) > 1 {
				for _, p := range parts {
					add(p, depth+1)
				}
				return
			}
		}
	}
	var walk func(s string, depth int)
	walk = func(s string, depth int) {
		s = stripParens(strings.TrimSpace(s))
		if s == "" || depth > 8 {
			return
		}
		if parts := splitTop(s, isBoolWord); len(parts) > 1 {
			for _, p := range parts {
				walk(p, depth+1)
			}
			return
		}
		if rest, ok := cutWord(s, "NOT"); ok {
			walk(rest, depth+1)
			return
		}
		if a, b, ok := cutCompare(s); ok {
			add(a, depth)
			add(b, depth)
			return
		}
		if name, args, ok := callOf(s); ok {
			switch strings.ToUpper(name) {
			case "AND", "OR", "XOR", "NOT":
				for _, a := range args {
					walk(a, depth+1)
				}
			default: // a comparison, or a function of the program's: its inputs
				for _, a := range args {
					add(a, depth+1)
				}
			}
		}
	}
	walk(cond, 0)
	return out
}

// scanTop calls at for each index of s outside parentheses and quotes.
func scanTop(s string, at func(i int) bool) {
	depth, quote := 0, byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0:
			if at(i) {
				return
			}
		}
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '#' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// wordAt is the word starting at s[i] when one starts there.
func wordAt(s string, i int) string {
	if i > 0 && isWordByte(s[i-1]) {
		return ""
	}
	j := i
	for j < len(s) && isWordByte(s[j]) {
		j++
	}
	return s[i:j]
}

func isBoolWord(w string) bool {
	switch strings.ToUpper(w) {
	case "AND", "OR", "XOR", "AND_THEN", "OR_ELSE":
		return true
	}
	return false
}

// splitTop splits s at its top-level words that sep accepts, and at "&".
func splitTop(s string, sep func(string) bool) []string {
	var parts []string
	from := 0
	scanTop(s, func(i int) bool {
		if s[i] == '&' {
			parts = append(parts, s[from:i])
			from = i + 1
			return false
		}
		if w := wordAt(s, i); w != "" && sep(w) {
			parts = append(parts, s[from:i])
			from = i + len(w)
		}
		return false
	})
	return append(parts, s[from:])
}

// cutWord cuts a leading keyword: "NOT x" is x.
func cutWord(s, word string) (string, bool) {
	if w := wordAt(s, 0); token.SameIdent(w, word) {
		return s[len(w):], true
	}
	return "", false
}

// cutCompare splits s at its first top-level comparison.
func cutCompare(s string) (a, b string, ok bool) {
	scanTop(s, func(i int) bool {
		for _, op := range []string{"<>", "<=", ">=", "=", "<", ">"} {
			if strings.HasPrefix(s[i:], op) {
				a, b, ok = s[:i], s[i+len(op):], true
				return true
			}
		}
		return false
	})
	return a, b, ok
}

// stripParens removes parentheses around the whole of s.
func stripParens(s string) string {
	for len(s) > 1 && s[0] == '(' {
		closes := -1
		depth := 0
		for i := 0; i < len(s); i++ {
			if s[i] == '(' {
				depth++
			} else if s[i] == ')' {
				depth--
				if depth == 0 {
					closes = i
					break
				}
			}
		}
		if closes != len(s)-1 {
			return s
		}
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// callOf reads s as one call, NAME(args), with its top-level arguments.
func callOf(s string) (name string, args []string, ok bool) {
	name = wordAt(s, 0)
	rest := strings.TrimSpace(s[len(name):])
	if name == "" || !strings.HasPrefix(rest, "(") || stripParens(rest) == rest {
		return "", nil, false // not one call over the whole of s
	}
	inner := rest[1 : len(rest)-1]
	from := 0
	arg := func(a string) {
		if _, v, formal := strings.Cut(a, ":="); formal {
			a = v // IN1 := x is x
		}
		args = append(args, a)
	}
	scanTop(inner, func(i int) bool {
		if inner[i] == ',' {
			arg(inner[from:i])
			from = i + 1
		}
		return false
	})
	arg(inner[from:])
	return name, args, true
}

// splitArith splits s at its top-level binary operators ops ("+", "-";
// "*", "/", "MOD"): a "+" or "-" that starts s or follows an operator or
// "(" is a sign, and an exponent's (1.5E-3) is part of its number.
func splitArith(s string, ops []string) []string {
	var parts []string
	from := 0
	scanTop(s, func(i int) bool {
		c := s[i]
		if c == ' ' || c == '\t' {
			return false
		}
		for _, op := range ops {
			word := op == "MOD"
			if word && token.SameIdent(wordAt(s, i), "MOD") || !word && strings.HasPrefix(s[i:], op) {
				exponent := (op == "+" || op == "-") && i > 1 && (s[i-1] == 'e' || s[i-1] == 'E') && s[i-2] >= '0' && s[i-2] <= '9'
				if endsOperand(s, i) && !exponent && !(op == "*" && strings.HasPrefix(s[i:], "**")) {
					parts = append(parts, s[from:i])
					from = i + len(op)
					return false
				}
			}
		}
		return false
	})
	return append(parts, s[from:])
}

// endsOperand says whether what comes before s[i] ends an operand (a name,
// a number, a literal, a closing parenthesis), so an operator at i is
// binary, not a sign.
func endsOperand(s string, i int) bool {
	j := i - 1
	for j >= 0 && (s[j] == ' ' || s[j] == '\t') {
		j--
	}
	return j >= 0 && (isWordByte(s[j]) || s[j] == ')' || s[j] == '\'' || s[j] == '"')
}

// isVariable says whether an action may be a variable of the POU (a BOOL
// that is TRUE while it is active): a name the chart has no ACTION body for.
func (l *sfcLayout) isVariable(action string) bool {
	if !isName(action) {
		return false
	}
	for _, a := range l.c.Actions {
		if token.SameIdent(a.Name, action) {
			return false
		}
	}
	return true
}

// stepTimeW is the room a step's time takes left of its link, with the gap
// to it: as long as T#1h23m45s678ms in the values' smaller font.
var stepTimeW = 6 + float64(len(longestTime))*charW*10/12 + 2

// longestTime is the longest time a step or action shows.
const longestTime = "T#99d23h59m59s999ms"
