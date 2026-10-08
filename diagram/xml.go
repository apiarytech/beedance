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

// This file writes a body's elements (from ParseText) as the content of a
// PLCopen TC6 <LD> or <FBD> element, so a body written in the text form
// exports as a diagram other IEC tools draw. Positions come from the same
// layout as the SVG drawing. ParseXML reads the result back into the same
// elements, and Format writes them as the text form again.

import (
	"fmt"
	"slices"
	"strings"
)

// XML returns elems as the inner XML of a TC6 <LD> or <FBD> body: every
// element at its laid-out position, wires as connections, and in a ladder a
// right power rail after each rung's coils.
func XML(elems []*Elem) (string, error) {
	elems = forXML(elems)
	l := newLayout(elems)
	at, rails, width, _ := l.geometry()

	railAt := map[int]geo{} // a left rail's corner, from its network's line
	for _, r := range rails {
		if e := l.netRail[r.net]; e != nil {
			if _, ok := railAt[e.ID]; !ok {
				railAt[e.ID] = geo{x: r.x, y: r.y1}
			}
		}
	}
	next := 0
	for _, e := range elems {
		next = max(next, e.ID)
	}

	var b strings.Builder
	for _, e := range l.order {
		switch e.Kind() {
		case "leftPowerRail":
			g := railAt[e.ID]
			fmt.Fprintf(&b, `<leftPowerRail localId="%d" height="%d" width="3">%s<connectionPointOut formalParameter=""/></leftPowerRail>`,
				e.ID, rowH, position(g))
			continue
		}
		n := l.nodes[e.ID]
		if n == nil {
			continue
		}
		g := at[e.ID]
		size := fmt.Sprintf(` height="%d" width="%.0f"`, n.rows*rowH, n.w)
		switch e.Kind() {
		case "contact", "coil":
			var attrs strings.Builder
			if e.Negated {
				attrs.WriteString(` negated="true"`)
			}
			if e.Edge != "" {
				fmt.Fprintf(&attrs, ` edge="%s"`, e.Edge)
			}
			if e.Storage != "" {
				fmt.Fprintf(&attrs, ` storage="%s"`, e.Storage)
			}
			fmt.Fprintf(&b, `<%s localId="%d"%s%s>%s%s<connectionPointOut/><variable>%s</variable></%s>`,
				e.Kind(), e.ID, size, attrs.String(), position(g), pointIn(e.Ins), esc(e.Variable), e.Kind())
		case "block":
			fmt.Fprintf(&b, `<block localId="%d"%s typeName="%s"`, e.ID, size, esc(e.TypeName))
			if e.InstanceName != "" {
				fmt.Fprintf(&b, ` instanceName="%s"`, esc(e.InstanceName))
			}
			b.WriteString(">" + position(g))
			pins := func(tag string, ps []Pin) {
				if len(ps) == 0 {
					fmt.Fprintf(&b, "<%s/>", tag)
					return
				}
				b.WriteString("<" + tag + ">")
				for _, p := range ps {
					fmt.Fprintf(&b, `<variable formalParameter="%s"%s>`, esc(p.Param), pinAttrs(p))
					if p.In != nil {
						b.WriteString(pointIn([]PointIn{*p.In}))
					} else {
						b.WriteString("<connectionPointIn/>")
					}
					b.WriteString("</variable>")
				}
				b.WriteString("</" + tag + ">")
			}
			pins("inputVariables", e.Inputs)
			pins("inOutVariables", e.InOuts)
			b.WriteString("<outputVariables>")
			for _, o := range n.outs {
				if o == "" {
					continue
				}
				attrs := ""
				if i := slices.IndexFunc(e.Outputs, func(p Pin) bool { return strings.EqualFold(p.Param, o) }); i >= 0 {
					attrs = pinAttrs(e.Outputs[i])
				}
				fmt.Fprintf(&b, `<variable formalParameter="%s"%s><connectionPointOut/></variable>`, esc(o), attrs)
			}
			b.WriteString("</outputVariables></block>")
		case "inVariable":
			fmt.Fprintf(&b, `<inVariable localId="%d"%s%s>%s<connectionPointOut/><expression>%s</expression></inVariable>`,
				e.ID, size, negated(e.Negated), position(g), esc(e.Expression))
		case "outVariable":
			fmt.Fprintf(&b, `<outVariable localId="%d"%s%s>%s%s<expression>%s</expression></outVariable>`,
				e.ID, size, negated(e.Negated), position(g), pointIn(e.Ins), esc(e.Expression))
		case "inOutVariable":
			fmt.Fprintf(&b, `<inOutVariable localId="%d"%s>%s%s<connectionPointOut/><expression>%s</expression></inOutVariable>`,
				e.ID, size, position(g), pointIn(e.Ins), esc(e.Expression))
		case "connector":
			fmt.Fprintf(&b, `<connector name="%s" localId="%d"%s>%s%s</connector>`, esc(e.Name), e.ID, size, position(g), pointIn(e.Ins))
		case "continuation":
			fmt.Fprintf(&b, `<continuation name="%s" localId="%d"%s>%s<connectionPointOut/></continuation>`, esc(e.Name), e.ID, size, position(g))
		default:
			return "", fmt.Errorf("element %d: a %s cannot be written as PLCopen XML", e.ID, e.Kind())
		}
	}

	// A right power rail closes each rung, after the coils no other coil
	// continues from.
	for i, net := range l.nets {
		if l.netRail[i] == nil {
			continue
		}
		var ends []Conn
		top := -1.0
		for _, n := range net {
			if n.e.Kind() != "coil" {
				continue
			}
			if top < 0 || at[n.e.ID].y < top {
				top = at[n.e.ID].y
			}
			continued := slices.ContainsFunc(l.readers[n.e.ID], func(r Conn) bool { return l.elems[r.Ref].Kind() == "coil" })
			if !continued {
				ends = append(ends, Conn{Ref: n.e.ID})
			}
		}
		if len(ends) == 0 {
			continue
		}
		next++
		fmt.Fprintf(&b, `<rightPowerRail localId="%d" height="%d" width="3">%s%s</rightPowerRail>`,
			next, rowH, position(geo{x: width, y: top}), pointIn([]PointIn{{Conns: ends}}))
	}
	return b.String(), nil
}

// forXML returns the elements as PLCopen XML holds them. A function
// contact's power, kept on the function for a drawing, is not XML: the
// function's result is ANDed with the rung's power by a block. ParseText
// adds that AND unless the function stands first in its rung, where its
// result alone is the power; here it gets one too, so every function
// contact is the same pattern, which Format reads back as a contact.
func forXML(elems []*Elem) []*Elem {
	out := make([]*Elem, 0, len(elems))
	next := 0
	for _, e := range elems {
		next = max(next, e.ID)
	}
	anded := map[int]bool{}
	for _, e := range elems {
		if e.Kind() == "block" && e.Name == contactAnd && len(e.Inputs) == 2 && e.Inputs[1].In != nil {
			for _, c := range e.Inputs[1].In.Conns {
				anded[c.Ref] = true
			}
		}
	}
	redirect := map[int]int{} // a function first in its rung -> its new AND
	for _, e := range elems {
		c := *e
		if e.Kind() == "block" && e.Name == contactFunction {
			c.Ins = nil
			if !anded[e.ID] && len(e.Ins) > 0 {
				next++
				and := &Elem{ID: next, Pos: Pos{X: e.Pos.X + 0.5, Y: e.Pos.Y}, TypeName: "AND", Name: contactAnd,
					Inputs:  []Pin{{Param: "IN1", In: &PointIn{Conns: slices.Clone(e.Ins[0].Conns)}}, {Param: "IN2", In: &PointIn{Conns: []Conn{{Ref: e.ID}}}}},
					Outputs: []Pin{{Param: "OUT"}}}
				and.XMLName.Local = "block"
				redirect[e.ID] = and.ID
				out = append(out, &c, and)
				continue
			}
		}
		out = append(out, &c)
	}
	if len(redirect) == 0 {
		return out
	}
	fix := func(conns []Conn) []Conn {
		cs := slices.Clone(conns)
		for i, cn := range cs {
			if a, ok := redirect[cn.Ref]; ok {
				cs[i] = Conn{Ref: a}
			}
		}
		return cs
	}
	for _, e := range out {
		if e.Name == contactAnd && len(e.Inputs) == 2 {
			continue // its IN2 is the function itself
		}
		ins := make([]PointIn, len(e.Ins))
		for i := range e.Ins {
			ins[i] = PointIn{Conns: fix(e.Ins[i].Conns)}
		}
		e.Ins = ins
		ps := slices.Clone(e.Inputs)
		for i := range ps {
			if ps[i].In != nil {
				ps[i].In = &PointIn{Conns: fix(ps[i].In.Conns)}
			}
		}
		e.Inputs = ps
	}
	return out
}

func position(g geo) string { return fmt.Sprintf(`<position x="%.0f" y="%.0f"/>`, g.x, g.y) }

func negated(on bool) string {
	if on {
		return ` negated="true"`
	}
	return ""
}

func pinAttrs(p Pin) string {
	s := negated(p.Negated)
	if p.Edge != "" {
		s += fmt.Sprintf(` edge="%s"`, p.Edge)
	}
	return s
}

// pointIn writes the connections into an element, all of its points in one
// connectionPointIn, as TC6 joins wires at an input (a ladder's OR).
func pointIn(ins []PointIn) string {
	var b strings.Builder
	b.WriteString("<connectionPointIn>")
	for _, in := range ins {
		for _, c := range in.Conns {
			if c.Param != "" {
				fmt.Fprintf(&b, `<connection refLocalId="%d" formalParameter="%s"/>`, c.Ref, esc(c.Param))
			} else {
				fmt.Fprintf(&b, `<connection refLocalId="%d"/>`, c.Ref)
			}
		}
	}
	b.WriteString("</connectionPointIn>")
	return b.String()
}
