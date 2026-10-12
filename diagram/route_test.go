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

import (
	"fmt"
	"testing"
)

// Wires pass no element but the two they join (drone review: wires drawn
// over blocks). The plain route, along the source's row, is kept where it
// is clear; a wire skipping a column, or running back to an earlier one,
// goes round.
func TestWiresAvoidElements(t *testing.T) {
	for _, c := range []struct {
		name, lang, src string
		rerouted        bool // some wire must leave the plain route
	}{
		{"ld sample", "LD", ldSample, false},
		{"fbd sample", "FBD", fbdSample, false},
		{"ld demo", "LD", `
  RUNG start_stop
    [ GT(WellLevel, 70.0) | Run ] /LT(WellLevel, 40.0) /Stop ( Run )
  RUNG high_delay
    GT(WellLevel, 85.0) t1:TON(PT := T#5S) ( HighAlarm )
  RUNG count_starts
    +Run c1:CTU(PV := 1000, CV => Starts)
  RUNG latch
    HighAlarm ( S Latched )
`, false},
		{"fbd demo", "FBD", `
  above = GE(WellLevel, SP)
  hold : TON(IN := above, PT := T#3S)
  High := OR(hold.Q, AND(High, NOT(LT(WellLevel, SUB(SP, 5.0)))))
  Speed := SEL(High, 0.0, LIMIT(0.0, MUL(SUB(WellLevel, 40.0), 2.0), 100.0))
`, false},
		// x feeds MUL beside it and SUB two columns on.
		{"skip a column", "FBD", `
  x = ADD(a, b)
  y = MUL(x, c)
  z := SUB(y, x)
`, false},
	} {
		body, err := ParseText(c.lang, c.src, 1)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		l := drawingLayout(body.Elems)
		at, _, _, _ := l.geometry()
		rt := l.newRouter(at)
		wires, rerouted := 0, false
		var all []routedWire // as drawn: one routing pass
		for _, e := range l.order {
			if l.nodes[e.ID] == nil {
				continue
			}
			for _, w := range l.wiresInto(e, at, rt) {
				all = append(all, w)
				wires++
				if !rt.clearPath(w.ps, w.src, w.dst) {
					t.Errorf("%s: a wire into %s %s crosses an element: %v", c.name, e.Kind(), e.TypeName+e.Variable+e.Expression, w.ps)
				}
				if len(w.ps) > 4 {
					rerouted = true
				}
			}
		}
		// No two wires, of other sources into other pins, run along one line.
		checker := &router{}
		{
			for _, w := range all {
				from, into := fmt.Sprintf("e%d", w.src), fmt.Sprintf("%d@%.1f", w.dst, w.ps[len(w.ps)-1].y)
				if w.src < 0 {
					from = fmt.Sprintf("rail%d", rt.netOf[w.dst])
				}
				for i := 1; i < len(w.ps); i++ {
					if w.ps[i-1] != w.ps[i] && checker.alongside(w.ps[i-1], w.ps[i], from, into) {
						t.Errorf("%s: a wire into %d runs along another at %v-%v", c.name, w.dst, w.ps[i-1], w.ps[i])
					}
				}
				for i := 1; i < len(w.ps); i++ {
					checker.segs = append(checker.segs, wseg{w.ps[i-1], w.ps[i], from, into})
				}
			}
		}
		if wires == 0 {
			t.Errorf("%s: no wires", c.name)
		}
		if c.rerouted && !rerouted {
			t.Errorf("%s: no wire went round", c.name)
		}
		texts(t, SVG(body.Elems)) // still well-formed
	}
}
