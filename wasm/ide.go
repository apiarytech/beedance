/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

// An editor's questions, for a page with no language server: the static
// analyzer's findings (package analyzer), and hover, completion, the
// definition, the references and a rename (package ide), and the
// graphical bodies drawn (LD, FBD and SFC as SVG). Positions are 1-based
// rows and byte columns, as the parser counts them.

import (
	"errors"
	"strings"

	"github.com/apiarytech/beedance/analyzer"
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/ide"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// finding is one of lint's findings.
type finding struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // info, warning, error
	Rule     string `json:"rule"`
	PLCopen  string `json:"plcopen,omitempty"`
	POU      string `json:"pou,omitempty"`
	Message  string `json:"message"`
}

type lintReply struct {
	Findings []finding `json:"findings"`
	Errors   []string  `json:"errors,omitempty"` // the parser's, when it could not read all
}

func lint(req Request) lintReply {
	fs, errs := analyzer.CheckSource(req.Source, analyzer.Options{})
	r := lintReply{Findings: []finding{}, Errors: errs}
	for _, f := range fs {
		r.Findings = append(r.Findings, finding{Line: f.Row, Column: f.Column, Severity: f.Severity.String(),
			Rule: f.Rule, PLCopen: f.PLCopen, POU: f.POU, Message: f.Message})
	}
	return r
}

type hoverReply struct {
	Text string    `json:"text"` // empty: nothing to say
	Span *ide.Span `json:"span,omitempty"`
	// Symbol is what the name is, when it resolves.
	Symbol *ide.Symbol `json:"symbol,omitempty"`
}

type completeReply struct {
	Items []ide.Completion `json:"items"`
}

type spanReply struct {
	Span *ide.Span `json:"span"` // nil: none
}

type spansReply struct {
	Spans []ide.Span `json:"spans"`
}

// diagramInfo is a graphical body: ide.Diagram, and the names its live
// view reads (Watches).
type diagramInfo struct {
	*ide.Diagram
	Watches []string `json:"watches"`
}

// outlineReply is the source's outline: its POUs, types, globals and
// configurations, each with its members (ide.Symbol.Children).
type outlineReply struct {
	Symbols        []*ide.Symbol `json:"symbols"`
	Configurations []configInfo  `json:"configurations"`
}

// configInfo is a CONFIGURATION: its resources, their tasks and the
// program instances that run in them.
type configInfo struct {
	Name      string         `json:"name"`
	Line      int            `json:"line"`
	Resources []resourceInfo `json:"resources"`
}

type resourceInfo struct {
	Name     string         `json:"name"`
	Type     string         `json:"type,omitempty"` // ON TYPE
	Line     int            `json:"line"`
	Tasks    []taskInfo     `json:"tasks"`
	Programs []instanceInfo `json:"programs"`
}

type taskInfo struct {
	Name     string `json:"name"`
	Interval string `json:"interval,omitempty"`
	Priority string `json:"priority,omitempty"`
	Single   string `json:"single,omitempty"`
	Line     int    `json:"line"`
}

// instanceInfo is PROGRAM Instance WITH Task : Type.
type instanceInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Task string `json:"task,omitempty"`
	Line int    `json:"line"`
}

// configurations reads the source's CONFIGURATIONs, as far as it parses.
func configurations(source string) []configInfo {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	out := []configInfo{}
	name := func(id *ast.Identifier) string {
		if id == nil {
			return ""
		}
		return id.Value
	}
	text := func(e ast.Expression) string {
		if e == nil {
			return ""
		}
		return e.String()
	}
	for _, st := range program.Statements {
		cd, ok := st.(*ast.ConfigurationDeclaration)
		if !ok {
			continue
		}
		ci := configInfo{Name: name(cd.Name), Line: cd.Token.Row, Resources: []resourceInfo{}}
		for _, rd := range cd.Resources {
			if rd == nil {
				continue
			}
			ri := resourceInfo{Name: name(rd.Name), Type: name(rd.ResourceType), Line: rd.Token.Row, Tasks: []taskInfo{}, Programs: []instanceInfo{}}
			for _, td := range rd.Tasks {
				if td != nil {
					ri.Tasks = append(ri.Tasks, taskInfo{Name: name(td.Name), Interval: text(td.Interval), Priority: text(td.Priority), Single: text(td.Single), Line: td.Token.Row})
				}
			}
			for _, pc := range rd.Programs {
				if pc != nil {
					ri.Programs = append(ri.Programs, instanceInfo{Name: name(pc.InstanceName), Type: name(pc.TypeName), Task: name(pc.TaskName), Line: pc.Token.Row})
				}
			}
			ci.Resources = append(ci.Resources, ri)
		}
		out = append(out, ci)
	}
	return out
}

type diagramsReply struct {
	Diagrams []diagramInfo `json:"diagrams"`
}

// ideOp answers the ide ops; ok is false if op is not one.
func ideOp(req Request) (reply any, ok bool, err error) {
	op := strings.ToLower(req.Op)
	switch op {
	case "lint":
		return lint(req), true, nil
	case "outline":
		out := ide.Analyze(req.Source).Outline()
		if out == nil {
			out = []*ide.Symbol{}
		}
		return outlineReply{Symbols: out, Configurations: configurations(req.Source)}, true, nil
	case "diagrams":
		ix := ide.Analyze(req.Source)
		r := diagramsReply{Diagrams: []diagramInfo{}}
		for _, d := range ix.Diagrams() {
			w := d.Watches()
			if w == nil {
				w = []string{}
			}
			r.Diagrams = append(r.Diagrams, diagramInfo{Diagram: d, Watches: w})
		}
		return r, true, nil
	case "hover", "complete", "definition", "references", "rename":
	default:
		return nil, false, nil
	}
	if req.Line < 1 || req.Column < 1 {
		return nil, true, errors.New(req.Op + `: give "line" and "column", from 1`)
	}
	ix := ide.Analyze(req.Source)
	p := ide.Pos{Row: req.Line, Column: req.Column}
	switch op {
	case "hover":
		r := hoverReply{}
		if text, span, found := ix.Hover(p); found {
			r.Text, r.Span = text, &span
		}
		if sym, _, found := ix.SymbolAt(p); found {
			r.Symbol = sym
		}
		return r, true, nil
	case "complete":
		items := ix.Complete(p)
		if items == nil {
			items = []ide.Completion{}
		}
		return completeReply{Items: items}, true, nil
	case "definition":
		if span, found := ix.Definition(p); found {
			return spanReply{Span: &span}, true, nil
		}
		return spanReply{}, true, nil
	case "references":
		spans := ix.References(p, true)
		if spans == nil {
			spans = []ide.Span{}
		}
		return spansReply{Spans: spans}, true, nil
	default: // rename
		spans, err := ix.Rename(p, req.NewName)
		if err != nil {
			return nil, true, err
		}
		return spansReply{Spans: spans}, true, nil
	}
}
