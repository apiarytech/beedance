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

// This file writes a chart as the content of a PLCopen TC6 <SFC> body, so
// a chart written in the text form exports as one other IEC tools draw.
// Positions come from the drawing's layout (sfc_svg.go), and its graph is
// written as TC6 has it: where a step leads to several transitions, a
// selection divergence; where several transitions lead to a step, a
// selection convergence; a transition's several steps meet in a
// simultaneous divergence or convergence; a loop is a jump. A selection's
// transitions get priorities in the chart's order. Package plcopen reads
// the result back (ReadSFC).

import (
	"fmt"
	"strings"
)

// SFCXML returns a chart as the inner XML of a TC6 <SFC> body. Conditions
// are written as inline ST; the actions' bodies are not part of the body
// (a TC6 POU holds them in its <actions>).
func SFCXML(c *Chart) (string, error) {
	if err := c.check(); err != nil {
		return "", err
	}
	l := newSFCLayout(c)
	g := l.geometry()

	id := func(n int) int { return n + 1 }
	next := len(l.nodes)
	fresh := func() int { next++; return next }
	divider := map[int]int{} // node -> its divergence
	joiner := map[int]int{}  // node -> its convergence
	for n, nd := range l.nodes {
		if len(nd.outs) > 1 {
			divider[n] = fresh()
		}
		if len(nd.ins) > 1 {
			joiner[n] = fresh()
		}
	}
	// The element a wire from n to one of its followers starts at.
	from := func(n int) int {
		if d, ok := divider[n]; ok {
			return d
		}
		return id(n)
	}
	// The element a node's input is wired from.
	into := func(n int) string {
		nd := l.nodes[n]
		switch {
		case len(nd.ins) == 0:
			return ""
		case len(nd.ins) > 1:
			return sfcPointIn(joiner[n])
		}
		return sfcPointIn(from(nd.ins[0]))
	}
	priority := map[int]int{} // transition node -> its place in a selection
	for _, nd := range l.nodes {
		if nd.kind == sfcStepNode && len(nd.outs) > 1 {
			for k, m := range nd.outs {
				priority[m] = k + 1
			}
		}
	}

	var b strings.Builder
	at := func(x, y float64) string { return fmt.Sprintf(`<position x="%.0f" y="%.0f"/>`, x, y) }
	for n, nd := range l.nodes {
		x := l.xOf(g, n)
		switch nd.kind {
		case sfcStepNode:
			s := c.Steps[nd.index]
			w := l.stepW(nd.index)
			fmt.Fprintf(&b, `<step localId="%d" height="%d" width="%.0f" name="%s"`, id(n), sfcStepH, w, esc(s.Name))
			if s.Initial {
				b.WriteString(` initialStep="true"`)
			}
			b.WriteString(">" + at(x-w/2, g.rowTop[nd.rank]) + into(n))
			fmt.Fprintf(&b, `<connectionPointOut formalParameter=""><relPosition x="%.0f" y="%d"/></connectionPointOut>`, w/2, sfcStepH)
			if len(s.Actions) > 0 {
				fmt.Fprintf(&b, `<connectionPointOutAction formalParameter=""><relPosition x="%.0f" y="%d"/></connectionPointOutAction>`, w, sfcActH/2)
			}
			b.WriteString("</step>")
		case sfcTransNode:
			t := c.Transitions[nd.index]
			fmt.Fprintf(&b, `<transition localId="%d" height="2" width="%d"`, id(n), 2*sfcBar)
			if p := priority[n]; p > 0 {
				fmt.Fprintf(&b, ` priority="%d"`, p)
			}
			b.WriteString(">" + at(x-sfcBar, l.top(g, n)) + into(n) + "<connectionPointOut/>")
			fmt.Fprintf(&b, `<condition><inline name=""><ST>%s</ST></inline></condition></transition>`, xhtmlST(t.Condition))
		case sfcJumpNode:
			fmt.Fprintf(&b, `<jumpStep localId="%d" height="10" width="12" targetName="%s">%s%s</jumpStep>`,
				id(n), esc(nd.target), at(x-6, g.rowTop[nd.rank]+sfcJumpH-10), into(n))
		}
	}
	// The bars.
	for n, nd := range l.nodes {
		kind := map[sfcKind][2]string{sfcStepNode: {"selectionDivergence", "selectionConvergence"},
			sfcTransNode: {"simultaneousDivergence", "simultaneousConvergence"}}[nd.kind]
		span := func(others []int) (float64, float64) {
			lo, hi := l.xOf(g, n), l.xOf(g, n)
			for _, m := range others {
				lo, hi = min(lo, l.xOf(g, m)), max(hi, l.xOf(g, m))
			}
			return lo, hi
		}
		if d, ok := divider[n]; ok {
			lo, hi := span(nd.outs)
			fmt.Fprintf(&b, `<%s localId="%d" height="1" width="%.0f">%s%s`, kind[0], d, hi-lo, at(lo, l.divideY(g, n)), sfcPointIn(id(n)))
			for k, m := range nd.outs {
				fmt.Fprintf(&b, `<connectionPointOut formalParameter="%d"><relPosition x="%.0f" y="0"/></connectionPointOut>`, k, l.xOf(g, m)-lo)
			}
			fmt.Fprintf(&b, `</%s>`, kind[0])
		}
		if j, ok := joiner[n]; ok {
			lo, hi := span(nd.ins)
			fmt.Fprintf(&b, `<%s localId="%d" height="1" width="%.0f">%s`, kind[1], j, hi-lo, at(lo, l.joinY(g, n)))
			for _, m := range nd.ins {
				b.WriteString(sfcPointIn(from(m)))
			}
			fmt.Fprintf(&b, `<connectionPointOut><relPosition x="%.0f" y="0"/></connectionPointOut></%s>`, l.xOf(g, n)-lo, kind[1])
		}
	}
	// The action blocks, right of their steps.
	for n, nd := range l.nodes {
		if nd.kind != sfcStepNode || len(c.Steps[nd.index].Actions) == 0 {
			continue
		}
		s := c.Steps[nd.index]
		qw, nw := l.actionCols(nd.index)
		block := fresh()
		fmt.Fprintf(&b, `<actionBlock localId="%d" height="%d" width="%.0f">%s%s`, block, len(s.Actions)*sfcActH, qw+nw,
			at(l.xOf(g, n)+l.stepW(nd.index)/2+16, g.rowTop[nd.rank]), sfcPointIn(id(n)))
		for k, a := range s.Actions {
			q := a.Qualifier
			if q == "" {
				q = "N"
			}
			fmt.Fprintf(&b, `<action localId="%d" qualifier="%s"`, fresh(), esc(strings.ToUpper(q)))
			if a.Duration != "" {
				fmt.Fprintf(&b, ` duration="%s"`, esc(a.Duration))
			}
			fmt.Fprintf(&b, `><relPosition x="0" y="%d"/><reference name="%s"/></action>`, k*sfcActH, esc(a.Action))
		}
		b.WriteString("</actionBlock>")
	}
	return b.String(), nil
}

func sfcPointIn(ref int) string {
	return fmt.Sprintf(`<connectionPointIn><connection refLocalId="%d"/></connectionPointIn>`, ref)
}

// xhtmlST is code as the XHTML of a TC6 formattedText, in CDATA.
func xhtmlST(code string) string {
	code = strings.ReplaceAll(code, "]]>", "]]]]><![CDATA[>")
	return `<xhtml:p xmlns:xhtml="http://www.w3.org/1999/xhtml"><![CDATA[` + code + `]]></xhtml:p>`
}
