/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package ide answers an editor's questions about an IEC 61131-3 source:
// where a name is declared, where it is used, what it is (hover), what may
// be written at a place (completion), its outline, and the edits that
// rename it. It is the language half of a language server; the protocol
// half (JSON-RPC, documents, UTF-16 columns) is the host's, beehive-lsp.
//
// Analyze reads a source once: the parser gives the declarations, with
// their positions, and the lexer gives every name used; each name is
// resolved through the scopes it stands in (a method, its function block
// and those it extends, the globals, the POUs and types, the standard
// library). A source with errors is analyzed as far as it parses, so an
// editor keeps its answers while the user types.
//
// Positions are 1-based rows and 1-based byte columns, as the lexer and
// the parser's diagnostics count them.
package ide

import (
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/diagram"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib" // the standard functions
	"github.com/apiarytech/beedance/token"
)

// Pos is a place in a source: a 1-based row and a 1-based byte column.
type Pos struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// Span is a stretch of one line of a source, End exclusive.
type Span struct {
	Start Pos `json:"start"`
	End   Pos `json:"end"`
}

// Kind is what a symbol is.
type Kind string

// The kinds of symbols.
const (
	Program       Kind = "program"
	FunctionBlock Kind = "function_block"
	Function      Kind = "function"
	Interface     Kind = "interface"
	Type          Kind = "type"
	Method        Kind = "method"
	Property      Kind = "property"
	Variable      Kind = "variable"
	Step          Kind = "step"
	Action        Kind = "action"
	Configuration Kind = "configuration"
	EnumValue     Kind = "enum_value"
	Builtin       Kind = "builtin" // a standard function
)

// Symbol is a declared name.
type Symbol struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	// Section is a variable's VAR section (VAR_INPUT, VAR_OUTPUT,
	// VAR_IN_OUT, VAR, VAR_TEMP, VAR_EXTERNAL, VAR_GLOBAL, VAR CONSTANT) or
	// a member's (STRUCT); empty for others.
	Section string `json:"section,omitempty"`
	// Type is a variable's, property's or function's declared type as
	// written, e.g. "TON" or "ARRAY[1..3] OF INT".
	Type string `json:"type,omitempty"`
	// Container is the POU, type or method it is declared in; empty for a
	// POU, a global or a standard one.
	Container string `json:"container,omitempty"`
	// Decl is its name where it is declared; zero for one with no place in
	// the source (the standard library, a diagram's generated variables).
	Decl Span `json:"decl"`
	// Range is all of a POU's, method's or type's text, for an outline;
	// zero for a variable.
	Range Span `json:"range"`
	// Detail is the declaration's line as written (a variable's), or the
	// POU's header line: hover shows it.
	Detail string `json:"detail,omitempty"`
	// Standard is true for the standard library's functions and function
	// blocks and their members.
	Standard bool `json:"standard,omitempty"`
	// Children are a POU's or type's members, in order, for an outline.
	Children []*Symbol `json:"children,omitempty"`

	scope *scope // a POU's or method's own scope
	base  string // a function block's EXTENDS
}

// Index is a source analyzed: Analyze makes it, and its methods answer.
type Index struct {
	src    string
	lines  []string
	toks   []token.Token
	pous   []*scope             // in source order, methods after their block
	named  map[string]*Symbol   // POUs, interfaces, types, by upper-case name
	glob   map[string]*Symbol   // global variables, by upper-case name
	enums  map[string]*Symbol   // enumerated values, by upper-case name
	top    []*Symbol            // the outline
	refs   map[int]*Symbol      // token index: the symbol it names
	std    map[string]*scope    // standard function blocks read so far
	stdFn  map[string]*Symbol   // standard functions
	mem    map[string]*scope    // members of struct types, by type name
	diags  []parser.Diagnostic  // the parse's
	bodies []parser.DiagramBody // the LD and FBD bodies
	charts []chartOf            // the SFC bodies
}

// scope is a POU's or method's names, and where its text is.
type scope struct {
	sym        *Symbol
	start, end int // byte offsets
	vars       map[string]*Symbol
	order      []*Symbol
	parent     *scope // a method's function block
}

func (s *scope) add(sym *Symbol) {
	key := strings.ToUpper(sym.Name)
	if _, dup := s.vars[key]; dup {
		return
	}
	s.vars[key] = sym
	s.order = append(s.order, sym)
}

// Analyze reads source.
func Analyze(source string) *Index {
	ix := &Index{
		src: source, lines: strings.Split(source, "\n"),
		named: map[string]*Symbol{}, glob: map[string]*Symbol{}, enums: map[string]*Symbol{},
		refs: map[int]*Symbol{}, std: map[string]*scope{}, mem: map[string]*scope{},
	}
	l := lexer.New(source)
	for {
		t := l.NextToken()
		if t.Type == token.EOF {
			break
		}
		if t.Type == token.COMMENT || t.Type == token.UNTERMINATED_COMMENT {
			continue
		}
		ix.toks = append(ix.toks, t)
	}
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	ix.diags = p.Diagnostics()
	diagrams := p.Diagrams()
	ix.bodies = diagrams
	ix.declare(program.Statements, len(source), diagrams)
	ix.resolveAll()
	return ix
}

// Diagnostics are the parse's errors, as the parser gives them.
func (ix *Index) Diagnostics() []parser.Diagnostic { return ix.diags }

// Outline is the source's POUs, types, globals and configurations with
// their members, in source order.
func (ix *Index) Outline() []*Symbol { return ix.top }

// declare reads the declarations of stmts, which end at offset end.
func (ix *Index) declare(stmts []ast.Statement, end int, diagrams []parser.DiagramBody) {
	starts := make([]int, len(stmts))
	for i, st := range stmts {
		starts[i] = stmtPos(st)
	}
	for i, st := range stmts {
		stop := end
		for j := i + 1; j < len(stmts); j++ {
			if starts[j] > starts[i] {
				stop = starts[j]
				break
			}
		}
		switch d := st.(type) {
		case *ast.ProgramDeclaration:
			s := ix.pou(d.Token, d.Name, Program, stop, "")
			ix.vars(s, "VAR_INPUT", d.VarInputs)
			ix.vars(s, "VAR_OUTPUT", d.VarOutputs)
			ix.vars(s, "VAR_IN_OUT", d.VarInOuts)
			for _, e := range d.VarExternal {
				ix.vars(s, "VAR_EXTERNAL", e.Vars)
			}
			for _, g := range d.VarGlobal {
				ix.vars(s, "VAR_GLOBAL", g.Vars)
			}
			for _, t := range d.VarTemp {
				ix.vars(s, "VAR_TEMP", t.Vars)
			}
			ix.vars(s, "VAR", d.Vars)
			ix.sfc(s, d.Body)
			ix.diagramVars(s, diagrams)
		case *ast.FunctionBlockDeclaration:
			s := ix.pou(d.Token, d.Name, FunctionBlock, stop, exprText(d.Extends))
			ix.vars(s, "VAR_INPUT", d.VarInputs)
			ix.vars(s, "VAR_OUTPUT", d.VarOutputs)
			ix.vars(s, "VAR_IN_OUT", d.VarInOuts)
			for _, e := range d.VarExternal {
				ix.vars(s, "VAR_EXTERNAL", e.Vars)
			}
			for _, t := range d.VarTemp {
				ix.vars(s, "VAR_TEMP", t.Vars)
			}
			ix.vars(s, "VAR", d.Vars)
			for _, pr := range d.Properties {
				ix.property(s, pr)
			}
			ix.diagramVars(s, diagrams)
			if body, ok := d.Body.(*ast.BlockStatement); ok && body != nil {
				ix.methods(s, body.Statements, stop)
			}
			ix.sfc(s, d.Body)
		case *ast.FunctionDeclaration:
			s := ix.pou(d.Token, d.Name, Function, stop, "")
			if d.ReturnType != nil {
				s.sym.Type = d.ReturnType.String()
			}
			ix.vars(s, "VAR_INPUT", d.VarInputs)
			ix.vars(s, "VAR_OUTPUT", d.VarOutputs)
			ix.vars(s, "VAR_IN_OUT", d.VarInOuts)
			ix.vars(s, "VAR", d.Vars)
			ix.diagramVars(s, diagrams)
		case *ast.InterfaceDeclaration:
			s := ix.pou(d.Token, d.Name, Interface, stop, "")
			for _, m := range d.Methods {
				if m == nil || m.Name == nil {
					continue
				}
				sym := &Symbol{Name: m.Name.Value, Kind: Method, Container: s.sym.Name, Decl: ix.span(m.Name.Token), Detail: ix.line(m.Token.Row)}
				if m.ReturnType != nil {
					sym.Type = m.ReturnType.String()
				}
				s.add(sym)
				s.sym.Children = append(s.sym.Children, sym)
			}
			for _, pr := range d.Properties {
				ix.property(s, pr)
			}
		case *ast.TypeBlockDeclaration:
			for _, td := range d.Declarations {
				ix.typeDecl(td, stop)
			}
		case *ast.TypeDeclaration:
			ix.typeDecl(d, stop)
		case *ast.GlobalVarDeclaration:
			ix.globals("VAR_GLOBAL", d.Vars, "")
		case *ast.ConfigurationDeclaration:
			if d.Name == nil {
				continue
			}
			sym := &Symbol{Name: d.Name.Value, Kind: Configuration, Decl: ix.span(d.Name.Token), Range: ix.rangeOf(d.Token.Pos, stop), Detail: ix.line(d.Token.Row)}
			ix.top = append(ix.top, sym)
			for _, g := range d.GlobalVars {
				sym.Children = append(sym.Children, ix.globals("VAR_GLOBAL", g.Vars, d.Name.Value)...)
			}
			for _, r := range d.Resources {
				for _, g := range r.GlobalVars {
					sym.Children = append(sym.Children, ix.globals("VAR_GLOBAL", g.Vars, d.Name.Value)...)
				}
			}
		case *ast.NamespaceDeclaration:
			ix.declare(d.Statements, stop, diagrams)
		}
	}
}

// stmtPos is the byte offset a top-level statement starts at; -1 if it
// has none.
func stmtPos(st ast.Statement) int {
	switch d := st.(type) {
	case *ast.ProgramDeclaration:
		return d.Token.Pos
	case *ast.FunctionBlockDeclaration:
		return d.Token.Pos
	case *ast.FunctionDeclaration:
		return d.Token.Pos
	case *ast.InterfaceDeclaration:
		return d.Token.Pos
	case *ast.TypeBlockDeclaration:
		return d.Token.Pos
	case *ast.TypeDeclaration:
		return d.Token.Pos
	case *ast.GlobalVarDeclaration:
		return d.Token.Pos
	case *ast.ConfigurationDeclaration:
		return d.Token.Pos
	case *ast.NamespaceDeclaration:
		return d.Token.Pos
	case *ast.MethodImplementation:
		return d.Token.Pos
	}
	return -1
}

func (ix *Index) pou(tok token.Token, name *ast.Identifier, kind Kind, stop int, base string) *scope {
	sym := &Symbol{Kind: kind, Range: ix.rangeOf(tok.Pos, stop), Detail: ix.line(tok.Row), base: base}
	if name != nil {
		sym.Name = name.Value
		sym.Decl = ix.span(name.Token)
	}
	s := &scope{sym: sym, start: tok.Pos, end: stop, vars: map[string]*Symbol{}}
	sym.scope = s
	ix.pous = append(ix.pous, s)
	ix.top = append(ix.top, sym)
	if sym.Name != "" {
		if _, dup := ix.named[strings.ToUpper(sym.Name)]; !dup {
			ix.named[strings.ToUpper(sym.Name)] = sym
		}
	}
	return s
}

// methods reads a function block's methods; the block's text ends at stop.
func (ix *Index) methods(fb *scope, stmts []ast.Statement, stop int) {
	var ms []*ast.MethodImplementation
	for _, st := range stmts {
		if m, ok := st.(*ast.MethodImplementation); ok && m.Name != nil {
			ms = append(ms, m)
		}
	}
	for i, m := range ms {
		end := stop
		if i+1 < len(ms) {
			end = ms[i+1].Token.Pos
		}
		sym := &Symbol{Name: m.Name.Value, Kind: Method, Container: fb.sym.Name, Decl: ix.span(m.Name.Token),
			Range: ix.rangeOf(m.Token.Pos, end), Detail: ix.line(m.Token.Row)}
		if m.ReturnType != nil {
			sym.Type = m.ReturnType.String()
		}
		s := &scope{sym: sym, start: m.Token.Pos, end: end, vars: map[string]*Symbol{}, parent: fb}
		sym.scope = s
		ix.vars(s, "VAR_INPUT", m.VarInputs)
		ix.vars(s, "VAR_OUTPUT", m.VarOutputs)
		ix.vars(s, "VAR_IN_OUT", m.VarInOuts)
		ix.vars(s, "VAR", m.Vars)
		ix.pous = append(ix.pous, s)
		fb.add(sym)
		fb.sym.Children = append(fb.sym.Children, sym)
	}
}

func (ix *Index) property(s *scope, pr *ast.PropertyDeclaration) {
	if pr == nil || pr.Name == nil {
		return
	}
	sym := &Symbol{Name: pr.Name.Value, Kind: Property, Container: s.sym.Name, Decl: ix.span(pr.Name.Token), Detail: ix.line(pr.Token.Row)}
	if pr.DataType != nil {
		sym.Type = pr.DataType.String()
	}
	s.add(sym)
	s.sym.Children = append(s.sym.Children, sym)
}

// vars declares a section's variables in s.
func (ix *Index) vars(s *scope, section string, decls []*ast.VarDeclStatement) {
	for _, v := range decls {
		if v == nil || v.Name == nil {
			continue
		}
		sec := section
		if v.IsConstant && section == "VAR" {
			sec = "VAR CONSTANT"
		}
		sym := &Symbol{Name: v.Name.Value, Kind: Variable, Section: sec, Type: typeText(v), Container: s.sym.Name}
		if ix.at(v.Name.Token) {
			sym.Decl = ix.span(v.Name.Token)
			sym.Detail = ix.line(v.Name.Token.Row)
		}
		s.add(sym)
		if sym.Decl != (Span{}) {
			s.sym.Children = append(s.sym.Children, sym)
		}
	}
}

// globals declares global variables; container is their configuration.
func (ix *Index) globals(section string, decls []*ast.VarDeclStatement, container string) []*Symbol {
	var out []*Symbol
	for _, v := range decls {
		if v == nil || v.Name == nil {
			continue
		}
		sym := &Symbol{Name: v.Name.Value, Kind: Variable, Section: section, Type: typeText(v), Container: container,
			Decl: ix.span(v.Name.Token), Detail: ix.line(v.Name.Token.Row)}
		key := strings.ToUpper(sym.Name)
		if _, dup := ix.glob[key]; !dup {
			ix.glob[key] = sym
		}
		out = append(out, sym)
		if container == "" {
			ix.top = append(ix.top, sym)
		}
	}
	return out
}

func (ix *Index) typeDecl(td *ast.TypeDeclaration, stop int) {
	if td == nil || td.Name == nil {
		return
	}
	sym := &Symbol{Name: td.Name.Value, Kind: Type, Decl: ix.span(td.Name.Token), Range: ix.rangeOf(td.Token.Pos, stop), Detail: ix.line(td.Name.Token.Row)}
	switch dt := td.DataType.(type) {
	case *ast.StructDefinition:
		sym.Type = "STRUCT"
		s := &scope{sym: sym, vars: map[string]*Symbol{}}
		for _, m := range dt.Members {
			if m == nil || m.Name == nil {
				continue
			}
			ms := &Symbol{Name: m.Name.Value, Kind: Variable, Section: "STRUCT", Type: typeText(m), Container: sym.Name,
				Decl: ix.span(m.Name.Token), Detail: ix.line(m.Name.Token.Row)}
			s.add(ms)
			sym.Children = append(sym.Children, ms)
		}
		ix.mem[strings.ToUpper(sym.Name)] = s
	case *ast.EnumDefinition:
		sym.Type = "ENUM"
		for _, v := range dt.Values {
			if v == nil {
				continue
			}
			ev := &Symbol{Name: v.Value, Kind: EnumValue, Type: sym.Name, Container: sym.Name, Decl: ix.span(v.Token), Detail: ix.line(v.Token.Row)}
			sym.Children = append(sym.Children, ev)
			if _, dup := ix.enums[strings.ToUpper(ev.Name)]; !dup {
				ix.enums[strings.ToUpper(ev.Name)] = ev
			}
		}
	default:
		if td.DataType != nil {
			sym.Type = td.DataType.String()
		}
	}
	if _, dup := ix.named[strings.ToUpper(sym.Name)]; !dup {
		ix.named[strings.ToUpper(sym.Name)] = sym
	}
	ix.top = append(ix.top, sym)
}

// sfc declares a chart's steps and actions in s.
func (ix *Index) sfc(s *scope, body ast.Statement) {
	chart, ok := body.(*ast.SFCProgram)
	if !ok || chart == nil {
		return
	}
	ix.charts = append(ix.charts, chartOf{s, chart})
	for _, el := range chart.Elements {
		switch e := el.(type) {
		case *ast.StepStatement:
			if e.Name == nil {
				continue
			}
			sym := &Symbol{Name: e.Name.Value, Kind: Step, Container: s.sym.Name, Decl: ix.span(e.Name.Token), Detail: ix.line(e.Token.Row)}
			if e.IsInitial {
				sym.Section = "INITIAL_STEP"
			}
			s.add(sym)
			s.sym.Children = append(s.sym.Children, sym)
		case *ast.ActionStatement:
			if e.Name == nil {
				continue
			}
			sym := &Symbol{Name: e.Name.Value, Kind: Action, Container: s.sym.Name, Decl: ix.span(e.Name.Token), Detail: ix.line(e.Token.Row)}
			s.add(sym)
			s.sym.Children = append(s.sym.Children, sym)
		}
	}
}

// diagramVars places the instances an LD or FBD body of s declares
// (name : TYPE(...), name:TYPE(...)): the parser declares them from text it
// generates, so each is found in the body itself.
func (ix *Index) diagramVars(s *scope, diagrams []parser.DiagramBody) {
	for _, d := range diagrams {
		if !token.SameIdent(d.POU, s.sym.Name) {
			continue
		}
		t, err := diagram.ParseText(d.Lang, d.Text, d.Row)
		if err != nil {
			continue
		}
		for _, dec := range t.Declares {
			sym := s.vars[strings.ToUpper(dec[0])]
			if sym == nil {
				sym = &Symbol{Name: dec[0], Kind: Variable, Section: "VAR", Type: dec[1], Container: s.sym.Name}
				s.add(sym)
			}
			if sym.Decl != (Span{}) {
				continue
			}
			for i := ix.first(s.start); i+1 < len(ix.toks) && ix.toks[i].Pos < s.end; i++ {
				t := ix.toks[i]
				if t.Row >= d.Row && token.SameIdent(t.Literal, dec[0]) && ix.toks[i+1].Type == token.COLON {
					sym.Decl, sym.Detail = ix.span(t), ix.line(t.Row)
					s.sym.Children = append(s.sym.Children, sym)
					break
				}
			}
		}
	}
}

// at says whether t's literal stands in the source where t says: a name
// the parser made from generated text does not.
func (ix *Index) at(t token.Token) bool {
	return t.Pos >= 0 && t.Pos+len(t.Literal) <= len(ix.src) && t.Literal != "" &&
		token.SameIdent(ix.src[t.Pos:t.Pos+len(t.Literal)], t.Literal) && t.Row > 0 && ix.line(t.Row) != ""
}

func (ix *Index) span(t token.Token) Span {
	return Span{Start: Pos{t.Row, t.Column}, End: Pos{t.Row, t.Column + len(t.Literal)}}
}

// rangeOf is the span from byte offset from to stop, trailing blank lines
// and comments left out.
func (ix *Index) rangeOf(from, stop int) Span {
	text := strings.TrimRight(ix.src[min(from, len(ix.src)):min(stop, len(ix.src))], " \t\r\n")
	return Span{Start: ix.posOf(from), End: ix.posOf(from + len(text))}
}

// posOf is the position of byte offset off.
func (ix *Index) posOf(off int) Pos {
	off = max(0, min(off, len(ix.src)))
	row := 1 + strings.Count(ix.src[:off], "\n")
	col := off - strings.LastIndex(ix.src[:off], "\n")
	return Pos{row, col}
}

// offset is the byte offset of p, clamped to its line.
func (ix *Index) offset(p Pos) int {
	if p.Row < 1 || p.Row > len(ix.lines) {
		return -1
	}
	off := 0
	for i := 0; i < p.Row-1; i++ {
		off += len(ix.lines[i]) + 1
	}
	return off + max(0, min(p.Column-1, len(ix.lines[p.Row-1])))
}

// line is row's text, trimmed.
func (ix *Index) line(row int) string {
	if row < 1 || row > len(ix.lines) {
		return ""
	}
	return strings.TrimSpace(ix.lines[row-1])
}

// first is the index of the first token at or after byte offset off.
func (ix *Index) first(off int) int {
	return sort.Search(len(ix.toks), func(i int) bool { return ix.toks[i].Pos >= off })
}

func typeText(v *ast.VarDeclStatement) string {
	t := exprText(v.DataType)
	if v.StringLength != nil {
		t += "(" + v.StringLength.String() + ")"
	}
	return t
}

func exprText(e ast.Expression) string {
	if e == nil {
		return ""
	}
	defer func() { recover() }() // a node half-built by a parse error
	return e.String()
}

// standardFB is the standard function block name, read on first use.
func (ix *Index) standardFB(name string) *scope {
	key := strings.ToUpper(name)
	if s, ok := ix.std[key]; ok {
		return s
	}
	var s *scope
	if fb := compiler.StandardFunctionBlock(key); fb != nil {
		sym := &Symbol{Name: key, Kind: FunctionBlock, Standard: true, Detail: "FUNCTION_BLOCK " + key + " (standard)"}
		s = &scope{sym: sym, vars: map[string]*Symbol{}}
		sym.scope = s
		add := func(section string, decls []*ast.VarDeclStatement) {
			for _, v := range decls {
				if v == nil || v.Name == nil {
					continue
				}
				m := &Symbol{Name: v.Name.Value, Kind: Variable, Section: section, Type: typeText(v), Container: key, Standard: true,
					Detail: v.Name.Value + " : " + typeText(v) + ";"}
				s.add(m)
				sym.Children = append(sym.Children, m)
			}
		}
		add("VAR_INPUT", fb.VarInputs)
		add("VAR_OUTPUT", fb.VarOutputs)
	}
	ix.std[key] = s
	return s
}

// standardFunction is the standard function name, or nil.
func (ix *Index) standardFunction(name string) *Symbol {
	if ix.stdFn == nil {
		ix.stdFn = map[string]*Symbol{}
		for _, b := range builtinNames() {
			ix.stdFn[strings.ToUpper(b)] = &Symbol{Name: b, Kind: Builtin, Standard: true, Detail: b + " (standard function)"}
		}
	}
	return ix.stdFn[strings.ToUpper(name)]
}

// builtinNames are the standard functions, without the engines' own
// (__CLOCK and the like, which no identifier can name).
func builtinNames() []string {
	var out []string
	for _, n := range object.BuiltinNames() {
		if n != "" && !strings.HasPrefix(n, "_") {
			out = append(out, n)
		}
	}
	return out
}

type parserDiagnostic = parser.Diagnostic
