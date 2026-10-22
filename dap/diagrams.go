/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

// The graphical bodies live: beedance/diagrams gives each LD, FBD and SFC
// body of the program being debugged its values, from the session (as
// package ide works them out: the variables shown, each wire's value, an
// SFC's active steps), for a page that draws the bodies (ide.Diagram's
// SVG) to paint power flow, values and steps. Stopped, the values are as
// they are; running, as the last scan left them.

import (
	"slices"
	"strconv"
	"strings"

	"github.com/apiarytech/beedance/ide"
	"github.com/apiarytech/beedance/object"
)

// diagramsArgs are beedance/diagrams': Prefixes names, by POU, the
// instance a function block's body is seen through ("m." for m : Motor);
// by default the first of the program's variables of its type.
type diagramsArgs struct {
	Prefixes map[string]string `json:"prefixes"`
}

// liveDiagram is a body's live view: Live is nil when the body has no
// instance to be seen through, or does not read (Error).
type liveDiagram struct {
	POU    string    `json:"pou"`
	Lang   string    `json:"lang"`
	Prefix string    `json:"prefix"`
	Live   *ide.Live `json:"live,omitempty"`
	Error  string    `json:"error,omitempty"`
}

func (s *Server) diagrams(a diagramsArgs) map[string]any {
	ix := s.analyzed()
	program := s.sess.Program()
	type body struct {
		d      *ide.Diagram
		prefix string
		names  []string
	}
	var bodies []body
	var all []string
	out := []liveDiagram{}
	for _, d := range ix.Diagrams() {
		prefix, ok := s.prefixFor(ix, program, d.POU, a.Prefixes)
		ld := liveDiagram{POU: d.POU, Lang: d.Lang, Prefix: prefix, Error: d.Error}
		out = append(out, ld)
		if !ok || d.Error != "" {
			bodies = append(bodies, body{})
			continue
		}
		var names []string
		for _, w := range d.Watches() {
			names = append(names, prefix+w)
		}
		bodies = append(bodies, body{d: d, prefix: prefix, names: names})
		all = append(all, names...)
	}
	slices.Sort(all)
	all = slices.Compact(all)
	var texts map[string]string
	if s.isRunning() {
		// Running: the values a recent scan's sample kept; ask for these
		// from the next on, if they are new.
		s.sampleFor("diagrams", all)
		texts = s.sess.Live().Extra
	} else {
		texts = s.sess.Texts(all)
	}
	values := ide.FromMap(objects(texts))
	for i, b := range bodies {
		if b.d == nil {
			continue
		}
		live, err := b.d.Live(values, b.prefix)
		if err != nil {
			out[i].Error = err.Error()
			continue
		}
		out[i].Live = live
	}
	return map[string]any{"program": program, "diagrams": out}
}

// prefixFor is the instance a body of pou is seen through: none for the
// program's own, the page's choice, or the program's first variable of
// pou's type; ok is false when there is none.
func (s *Server) prefixFor(ix *ide.Index, program, pou string, chosen map[string]string) (string, bool) {
	if strings.EqualFold(pou, program) {
		return "", true
	}
	for k, v := range chosen {
		if strings.EqualFold(k, pou) && v != "" {
			return strings.TrimSuffix(v, ".") + ".", true
		}
	}
	for _, sym := range ix.Outline() {
		if sym.Kind != ide.Program || !strings.EqualFold(sym.Name, program) {
			continue
		}
		for _, c := range sym.Children {
			if c.Kind == ide.Variable && strings.EqualFold(c.Type, pou) {
				return c.Name + ".", true
			}
		}
	}
	return "", false
}

// objects reads the session's texts back as values for ide's expressions:
// BOOLs and numbers; any other value (a TIME, a STRING) keeps its text, to
// be shown, not computed with.
func objects(texts map[string]string) map[string]object.Object {
	out := make(map[string]object.Object, len(texts))
	for k, v := range texts {
		switch {
		case strings.EqualFold(v, "TRUE"):
			out[k] = &object.Boolean{Value: true}
		case strings.EqualFold(v, "FALSE"):
			out[k] = &object.Boolean{Value: false}
		default:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out[k] = &object.LInt{Value: n}
			} else if f, err := strconv.ParseFloat(v, 64); err == nil {
				out[k] = &object.LReal{Value: f}
			} else {
				out[k] = &object.String{Value: v}
			}
		}
	}
	return out
}
