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

// This file routes the LD and FBD drawing's wires around the elements. A
// wire takes the plain route (out along its row, down or up in the gap
// before the element it feeds, then in) when nothing is in its way. When
// an element is, as for a wire that skips columns or runs back to an
// earlier one, it leaves through the gap after its source's column, runs
// along a lane where no element is, and comes back in the gap before its
// target. Columns are apart by gapX, and nothing but wires is drawn there.

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// box is the area an element covers, its labels included.
type box struct{ x0, y0, x1, y1 float64 }

// pt is a point of a wire.
type pt struct{ x, y float64 }

// router knows where the elements are.
type router struct {
	boxes  map[int]box
	colEnd map[int]float64 // element -> right edge of its column
	netOf  map[int]int     // element -> its network
	bottom []float64       // network -> below its lowest element
	segs   []wseg          // the wires drawn so far
}

func (l *layout) newRouter(at map[int]geo) *router {
	r := &router{boxes: map[int]box{}, colEnd: map[int]float64{}, netOf: map[int]int{}, bottom: make([]float64, len(l.nets))}
	for i, net := range l.nets {
		ends := map[int]float64{}
		for _, n := range net {
			g := at[n.e.ID]
			ends[n.layer] = max(ends[n.layer], g.x+n.w)
			b := l.boxOf(n, g)
			r.boxes[n.e.ID] = b
			r.netOf[n.e.ID] = i
			r.bottom[i] = max(r.bottom[i], b.y1)
		}
		for _, n := range net {
			r.colEnd[n.e.ID] = ends[n.layer]
		}
		r.bottom[i] += 6
	}
	return r
}

// boxOf is what drawElem draws of n at g: its symbol and its labels.
func (l *layout) boxOf(n *node, g geo) box {
	my := g.y + rowH/2 - 4
	switch n.e.Kind() {
	case "contact", "coil":
		return box{g.x, my - 22, g.x + n.w, my + 10} // the variable above it
	case "block":
		y0 := g.y + 4 // the box, its title row in it
		if n.e.InstanceName != "" {
			y0 -= 14 // the instance's name above it
		}
		return box{g.x - 6, y0, g.x + n.w + 6, g.y + float64(n.rows)*rowH - 4}
	}
	return box{g.x, my - 10, g.x + n.w, my + 10}
}

// clear reports whether the segment a-b, horizontal or vertical, passes
// no element but those in skip.
func (r *router) clear(a, b pt, skip ...int) bool {
	x0, x1 := min(a.x, b.x), max(a.x, b.x)
	y0, y1 := min(a.y, b.y), max(a.y, b.y)
	for id, bx := range r.boxes {
		if slices.Contains(skip, id) {
			continue
		}
		if bx.x0 < x1+0.5 && bx.x1 > x0-0.5 && bx.y0 < y1+4 && bx.y1 > y0-4 {
			return false
		}
	}
	return true
}

func (r *router) clearPath(ps []pt, src, dst int) bool {
	for i := 1; i < len(ps); i++ {
		if !r.clear(ps[i-1], ps[i], src, dst) {
			return false
		}
	}
	return true
}

// routedWire is one drawn wire into an element: its points, and the dots of a
// split (where it leaves a value read elsewhere too) and of a join (where
// it meets the other wires into the same input).
type routedWire struct {
	src, dst    int  // src -1: a power rail
	conn        Conn // the wire's source, as the lowering keys it
	ps          []pt
	split, join *pt
}

// wiresInto are the wires into e, routed.
func (l *layout) wiresInto(e *Elem, at map[int]geo, rt *router) []routedWire {
	n := l.nodes[e.ID]
	g := at[e.ID]
	mid := func(id int) float64 { return at[id].y + rowH/2 - 4 }
	inY := func(param string) float64 {
		i := slices.IndexFunc(n.ins, func(s string) bool { return strings.EqualFold(s, param) })
		return mid(e.ID) + float64(n.head+max(0, i))*rowH
	}
	// Each input turns on its own vertical in the gap before e, the top one
	// nearest, so wires into different pins never run along one line.
	type input struct {
		conns []Conn
		y     float64
	}
	var inputs []input
	for _, in := range e.Ins {
		inputs = append(inputs, input{in.Conns, mid(e.ID)})
	}
	for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
		for _, pin := range pins {
			if pin.In != nil {
				inputs = append(inputs, input{pin.In.Conns, inY(pin.Param)})
			}
		}
	}
	slices.SortStableFunc(inputs, func(a, b input) int { return int(a.y - b.y) })
	step := 6.0
	if len(inputs) > 1 {
		step = min(6, (gapX*2/3-6)/float64(len(inputs)-1))
	}
	var out []routedWire
	into := func(conns []Conn, y, xm float64) {
		for _, c := range conns {
			s, ok := l.elems[c.Ref]
			if !ok {
				continue
			}
			src, x1, y1 := c.Ref, float64(margin), y
			if s.Kind() != "leftPowerRail" {
				sn := l.nodes[c.Ref]
				if sn == nil {
					continue
				}
				x1 = at[c.Ref].x + sn.w
				y1 = mid(c.Ref) + float64(sn.head+max(0, outIndex(sn, c.Param)))*rowH
			} else {
				src = -1
			}
			ps, split := rt.route(src, e.ID, x1, y1, g.x, y, xm)
			w := routedWire{src: src, dst: e.ID, ps: ps, conn: c}
			if y1 != y && len(l.readers[c.Ref]) > 1 {
				w.split = &split
			}
			if y1 != y && len(conns) > 1 {
				w.join = &pt{xm, y}
			}
			out = append(out, w)
		}
	}
	for k, in := range inputs {
		into(in.conns, in.y, g.x-6-float64(k)*step)
	}
	return out
}

// wseg is a drawn wire's segment: from (its source; rails are one per
// network) and into (its target pin) say which wires it may run along.
type wseg struct {
	a, b       pt
	from, into string
}

// alongside reports whether a-b runs along a wire already drawn, one of
// another source into another pin: on the same line, horizontal or
// vertical, over more than a point. Such wires would look like one.
func (r *router) alongside(a, b pt, from, into string) bool {
	const near = 3 // closer than this looks like one line
	for _, s := range r.segs {
		if s.from == from || s.into == into {
			continue // the same signal: a split or a join, drawn with a dot
		}
		switch {
		case a.y == b.y && s.a.y == s.b.y && math.Abs(a.y-s.a.y) < near:
			if overlap(a.x, b.x, s.a.x, s.b.x) {
				return true
			}
		case a.x == b.x && s.a.x == s.b.x && math.Abs(a.x-s.a.x) < near:
			if overlap(a.y, b.y, s.a.y, s.b.y) {
				return true
			}
		}
	}
	return false
}

// overlap: do [a0, a1] and [b0, b1], in any order, share more than a point?
func overlap(a0, a1, b0, b1 float64) bool {
	lo := max(min(a0, a1), min(b0, b1))
	hi := min(max(a0, a1), max(b0, b1))
	return hi-lo > 1
}

func (r *router) free(ps []pt, src, dst int, from, into string) bool {
	if !r.clearPath(ps, src, dst) {
		return false
	}
	for i := 1; i < len(ps); i++ {
		if ps[i-1] != ps[i] && r.alongside(ps[i-1], ps[i], from, into) {
			return false
		}
	}
	return true
}

// route is a wire from (x1, y1), the right of src (-1: a power rail), to
// (x2, y2), the left of dst, turning on xm if it can; split is the bend a
// split's dot goes on. It keeps off the elements and off the other wires:
// the plain route on another vertical of the gap, else round through the
// gap after the source's column and a lane, both moved until clear. The
// route is kept, so later wires keep off it.
func (r *router) route(src, dst int, x1, y1, x2, y2, xm float64) (ps []pt, split pt) {
	from := fmt.Sprintf("e%d", src)
	if src < 0 {
		from = fmt.Sprintf("rail%d", r.netOf[dst])
	}
	into := fmt.Sprintf("%d@%.1f", dst, y2)
	defer func() {
		for i := 1; i < len(ps); i++ {
			if ps[i-1] != ps[i] {
				r.segs = append(r.segs, wseg{ps[i-1], ps[i], from, into})
			}
		}
	}()
	// Verticals in the gap before dst, nearest xm first; never in a column.
	lo := x2 - gapX + 4
	xms := []float64{xm}
	for d := 3.0; d <= gapX; d += 3 {
		for _, x := range []float64{xm - d, xm + d} {
			if x >= lo && x <= x2-3 {
				xms = append(xms, x)
			}
		}
	}
	for _, m := range xms {
		plain := []pt{{x1, y1}, {m, y1}, {m, y2}, {x2, y2}}
		if x1 <= m && r.free(plain, src, dst, from, into) {
			return plain, plain[1]
		}
	}
	// Round: out through the gap after the source's column.
	xa0 := x1 + railGap/2 // from a rail, the gap before the first column
	hi := x1 + railGap - 4
	if src >= 0 {
		xa0 = r.colEnd[src] + gapX/3
		hi = r.colEnd[src] + gapX - 4
	}
	xas := []float64{xa0}
	for d := 3.0; d <= gapX; d += 3 {
		for _, x := range []float64{xa0 + d, xa0 - d} {
			if x > x1+2 && x <= hi {
				xas = append(xas, x)
			}
		}
	}
	// Lanes: the rows themselves and beside them, then just above and below
	// each element of the network, then under the whole network; nearest
	// first.
	net := r.netOf[dst]
	lanes := []float64{y1, y2}
	for d := 6.0; d <= 18; d += 6 {
		lanes = append(lanes, y1-d, y1+d, y2-d, y2+d)
	}
	for id, bx := range r.boxes {
		if r.netOf[id] == net {
			lanes = append(lanes, bx.y0-7, bx.y1+7, bx.y0-13, bx.y1+13)
		}
	}
	for d := 0.0; d <= 18; d += 6 {
		lanes = append(lanes, r.bottom[net]+d)
	}
	slices.SortStableFunc(lanes, func(a, b float64) int {
		ca, cb := math.Abs(a-y1)+math.Abs(a-y2), math.Abs(b-y1)+math.Abs(b-y2)
		switch {
		case ca < cb:
			return -1
		case ca > cb:
			return 1
		}
		return 0
	})
	var fallback []pt
	for _, yl := range lanes {
		for _, a := range xas {
			for _, m := range xms {
				ps := []pt{{x1, y1}, {a, y1}, {a, yl}, {m, yl}, {m, y2}, {x2, y2}}
				if r.free(ps, src, dst, from, into) {
					return ps, ps[1]
				}
				if fallback == nil && r.clearPath(ps, src, dst) {
					fallback = ps // off the elements, if not off every wire
				}
			}
		}
	}
	if fallback != nil {
		return fallback, fallback[1]
	}
	plain := []pt{{x1, y1}, {xm, y1}, {xm, y2}, {x2, y2}}
	return plain, plain[1] // nothing clear: as before
}
