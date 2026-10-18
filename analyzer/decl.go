/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package analyzer

// The declarations: POUs and their variables, globals and types, read
// before the bodies are walked.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
)

// variable is a declared variable and how the bodies use it.
type variable struct {
	name     string
	section  string // VAR, VAR_TEMP, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_EXTERNAL, VAR_GLOBAL
	typ      string // as resolve gives it
	decl     *ast.VarDeclStatement
	pou      *pou // nil for a global
	located  string
	constant bool
	reads    int
	writes   int
}

// pou is a program, function block, function, method or property accessor.
type pou struct {
	name   string
	kind   string // PROGRAM, FUNCTION_BLOCK, FUNCTION, METHOD, PROPERTY
	key    string // unique: "FUNCTION F", "METHOD FB.M"
	vars   map[string]*variable
	order  []*variable
	parent *pou   // a method's or accessor's function block
	base   string // a function block's EXTENDS, upper case
	result string // a function's or method's result type
	set    bool   // a function's or method's result is assigned
	row    int
	col    int
	body   ast.Statement

	methods map[string]*pou
	// calls are the POUs it calls by key, with the first call's position.
	calls map[string][2]int
	// instanceCalls are the positions each instance is called at.
	instanceCalls map[*variable][][2]int
}

// typeDef is a TYPE: an alias, a structure or an enumeration.
type typeDef struct {
	alias   string
	members map[string]string
	enum    bool
}

// site is a place in the source.
type site struct {
	row, col int
	pou      string
}

type analysis struct {
	program  *ast.Program
	opts     Options
	findings []Finding

	pous     []*pou
	byKey    map[string]*pou
	fbs      map[string]*pou
	funcs    map[string]*pou
	globals  map[string]*variable
	gorder   []*variable
	types    map[string]*typeDef
	enumVals map[string]string // a value (upper) to its type, when only one has it

	outputWrites  map[string][]site             // %Q address to its writes
	globalWriters map[*variable]map[string]site // a global to the PROGRAMs writing it
}

func newAnalysis(program *ast.Program, opts Options) *analysis {
	a := &analysis{
		program:       program,
		opts:          opts,
		byKey:         map[string]*pou{},
		fbs:           map[string]*pou{},
		funcs:         map[string]*pou{},
		globals:       map[string]*variable{},
		types:         map[string]*typeDef{},
		enumVals:      map[string]string{},
		outputWrites:  map[string][]site{},
		globalWriters: map[*variable]map[string]site{},
	}
	a.declare(program.Statements)
	return a
}

func (a *analysis) report(rule string, sev Severity, row, col int, pou, msg string) {
	if a.opts.Disable[rule] {
		return
	}
	a.findings = append(a.findings, Finding{Rule: rule, PLCopen: plcopenOf[rule], Severity: sev, Row: row, Column: col, POU: pou, Message: msg})
}

func up(s string) string { return strings.ToUpper(s) }

func (a *analysis) declare(stmts []ast.Statement) {
	dupEnum := map[string]bool{}
	for _, s := range stmts {
		switch d := s.(type) {
		case *ast.NamespaceDeclaration:
			a.declare(d.Statements)
		case *ast.TypeBlockDeclaration:
			for _, td := range d.Declarations {
				a.typeDecl(td, dupEnum)
			}
		case *ast.TypeDeclaration:
			a.typeDecl(d, dupEnum)
		case *ast.GlobalVarDeclaration:
			a.globalVars(d.Vars)
		case *ast.ConfigurationDeclaration:
			for _, g := range d.GlobalVars {
				a.globalVars(g.Vars)
			}
			for _, r := range d.Resources {
				for _, g := range r.GlobalVars {
					a.globalVars(g.Vars)
				}
			}
		case *ast.ProgramDeclaration:
			if d.Name == nil {
				continue
			}
			p := a.newPOU("PROGRAM", d.Name.Value, d.Token.Row, d.Token.Column, d.Body)
			p.addVars("VAR_INPUT", d.VarInputs)
			p.addVars("VAR_OUTPUT", d.VarOutputs)
			p.addVars("VAR_IN_OUT", d.VarInOuts)
			p.addVars("VAR", d.Vars)
			for _, e := range d.VarExternal {
				p.addVars("VAR_EXTERNAL", e.Vars)
			}
			for _, t := range d.VarTemp {
				p.addVars("VAR_TEMP", t.Vars)
			}
			for _, g := range d.VarGlobal {
				a.globalVars(g.Vars)
			}
		case *ast.FunctionDeclaration:
			if d.Name == nil {
				continue
			}
			p := a.newPOU("FUNCTION", d.Name.Value, d.Token.Row, d.Token.Column, d.Body)
			if d.ReturnType != nil {
				p.result = normalize(d.ReturnType.Token.Literal)
			}
			p.addVars("VAR_INPUT", d.VarInputs)
			p.addVars("VAR_OUTPUT", d.VarOutputs)
			p.addVars("VAR_IN_OUT", d.VarInOuts)
			p.addVars("VAR", d.Vars)
			a.funcs[up(d.Name.Value)] = p
		case *ast.FunctionBlockDeclaration:
			if d.Name != nil {
				a.functionBlock(d)
			}
		}
	}
	for v := range dupEnum {
		delete(a.enumVals, v)
	}
}

func (a *analysis) newPOU(kind, name string, row, col int, body ast.Statement) *pou {
	p := &pou{name: name, kind: kind, key: kind + " " + up(name), vars: map[string]*variable{}, row: row, col: col, body: body,
		methods: map[string]*pou{}, calls: map[string][2]int{}, instanceCalls: map[*variable][][2]int{}}
	a.pous = append(a.pous, p)
	a.byKey[p.key] = p
	return p
}

func (a *analysis) functionBlock(d *ast.FunctionBlockDeclaration) {
	fb := a.newPOU("FUNCTION_BLOCK", d.Name.Value, d.Token.Row, d.Token.Column, nil)
	a.fbs[up(d.Name.Value)] = fb
	if d.Extends != nil {
		name := d.Extends.String()
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		fb.base = up(name)
	}
	fb.addVars("VAR_INPUT", d.VarInputs)
	fb.addVars("VAR_OUTPUT", d.VarOutputs)
	fb.addVars("VAR_IN_OUT", d.VarInOuts)
	fb.addVars("VAR", d.Vars)
	for _, e := range d.VarExternal {
		fb.addVars("VAR_EXTERNAL", e.Vars)
	}
	for _, t := range d.VarTemp {
		fb.addVars("VAR_TEMP", t.Vars)
	}
	for _, pr := range d.Properties {
		a.property(fb, pr)
	}
	blk, ok := d.Body.(*ast.BlockStatement)
	if !ok {
		fb.body = d.Body
		return
	}
	// The body proper, without the methods and properties written in it.
	own := &ast.BlockStatement{Token: blk.Token}
	for _, st := range blk.Statements {
		switch m := st.(type) {
		case *ast.MethodImplementation:
			if m.Name == nil {
				continue
			}
			mp := a.newPOU("METHOD", d.Name.Value+"."+m.Name.Value, m.Token.Row, m.Token.Column, m.Body)
			mp.parent = fb
			if m.ReturnType != nil {
				mp.result = normalize(m.ReturnType.Token.Literal)
			}
			mp.addVars("VAR_INPUT", m.VarInputs)
			mp.addVars("VAR_OUTPUT", m.VarOutputs)
			mp.addVars("VAR_IN_OUT", m.VarInOuts)
			mp.addVars("VAR", m.Vars)
			fb.methods[up(m.Name.Value)] = mp
		case *ast.PropertyDeclaration:
			a.property(fb, m)
		default:
			own.Statements = append(own.Statements, st)
		}
	}
	fb.body = own
}

// property makes its getter and setter POUs of their own, in which the
// property's name is its value.
func (a *analysis) property(fb *pou, pr *ast.PropertyDeclaration) {
	if pr == nil || pr.Name == nil {
		return
	}
	typ := ""
	if pr.DataType != nil {
		typ = normalize(pr.DataType.Token.Literal)
	}
	add := func(kind string, body *ast.BlockStatement, row, col int) {
		if body == nil {
			return
		}
		p := a.newPOU("PROPERTY", fb.name+"."+pr.Name.Value+"."+kind, row, col, body)
		p.parent = fb
		v := &variable{name: pr.Name.Value, section: "PROPERTY", typ: typ, pou: p}
		p.vars[up(pr.Name.Value)] = v
	}
	if pr.Getter != nil {
		add("GET", pr.Getter.Body, pr.Getter.Token.Row, pr.Getter.Token.Column)
	}
	if pr.Setter != nil {
		add("SET", pr.Setter.Body, pr.Setter.Token.Row, pr.Setter.Token.Column)
	}
}

func (p *pou) addVars(section string, decls []*ast.VarDeclStatement) {
	for _, d := range decls {
		if d == nil || d.Name == nil {
			continue
		}
		v := newVariable(d, section)
		v.pou = p
		p.vars[up(d.Name.Value)] = v
		p.order = append(p.order, v)
	}
}

func newVariable(d *ast.VarDeclStatement, section string) *variable {
	v := &variable{name: d.Name.Value, section: section, typ: declType(d.DataType), decl: d, constant: d.IsConstant}
	if d.Location != nil {
		v.located = up(strings.TrimPrefix(d.Location.String(), "AT "))
		v.located = strings.TrimPrefix(strings.TrimSpace(v.located), "%")
	}
	return v
}

func (a *analysis) globalVars(decls []*ast.VarDeclStatement) {
	for _, d := range decls {
		if d == nil || d.Name == nil {
			continue
		}
		v := newVariable(d, "VAR_GLOBAL")
		a.globals[up(d.Name.Value)] = v
		a.gorder = append(a.gorder, v)
	}
}

func (a *analysis) typeDecl(td *ast.TypeDeclaration, dup map[string]bool) {
	if td == nil || td.Name == nil {
		return
	}
	name := up(td.Name.Value)
	def := &typeDef{}
	switch t := td.DataType.(type) {
	case *ast.StructDefinition:
		def.members = map[string]string{}
		for _, m := range t.Members {
			if m != nil && m.Name != nil {
				def.members[up(m.Name.Value)] = declType(m.DataType)
			}
		}
	case *ast.EnumDefinition:
		def.enum = true
		for _, v := range t.Values {
			if v == nil {
				continue
			}
			k := up(v.Value)
			if _, seen := a.enumVals[k]; seen {
				dup[k] = true
			}
			a.enumVals[k] = name
		}
	default:
		def.alias = declType(td.DataType)
	}
	a.types[name] = def
}

// declType is a declared type as a name: "INT", "TON", "ARRAY OF INT",
// "REF_TO REAL"; "" for one written in place (a structure).
func declType(e ast.Expression) string {
	switch t := e.(type) {
	case nil:
		return ""
	case *ast.Identifier:
		return normalize(t.Value)
	case *ast.TypeSpecifier:
		return normalize(t.Token.Literal)
	case *ast.ArrayDefinition:
		if t.DataType == nil {
			return "ARRAY OF "
		}
		return "ARRAY OF " + normalize(t.DataType.Token.Literal)
	case *ast.RefToType:
		return "REF_TO " + declType(t.BaseType)
	case *ast.StructDefinition, *ast.EnumDefinition:
		return ""
	}
	return normalize(e.String())
}

// resolve follows aliases to the type a name stands for.
func (a *analysis) resolve(t string) string {
	for i := 0; i < 16; i++ {
		def, ok := a.types[t]
		if !ok || def.alias == "" {
			return t
		}
		t = def.alias
	}
	return t
}

// isFBType reports whether t names a function block, the program's or a
// standard one.
func (a *analysis) isFBType(t string) bool {
	if _, ok := a.fbs[t]; ok {
		return true
	}
	return t != "" && !isElementary(t) && compiler.StandardFunctionBlock(t) != nil
}

// lookup finds a name from inside a POU: its own variables, its function
// block's (and those it extends), then the globals.
func (a *analysis) lookup(p *pou, name string) *variable {
	k := up(name)
	seen := map[*pou]bool{}
	for cur := p; cur != nil && !seen[cur]; {
		seen[cur] = true
		if v, ok := cur.vars[k]; ok {
			return v
		}
		switch {
		case cur.parent != nil:
			cur = cur.parent
		case cur.base != "":
			cur = a.fbs[cur.base]
		default:
			cur = nil
		}
	}
	return a.globals[k]
}

// member is the type of a member of a value of type t: a function block's
// variable, a standard block's input or output, a structure's member.
func (a *analysis) member(t, name string) string {
	t = a.resolve(t)
	k := up(name)
	seen := map[*pou]bool{}
	for fb := a.fbs[t]; fb != nil && !seen[fb]; fb = a.fbs[fb.base] {
		seen[fb] = true
		if v, ok := fb.vars[k]; ok {
			return a.resolve(v.typ)
		}
	}
	if def, ok := a.types[t]; ok && def.members != nil {
		return a.resolve(def.members[k])
	}
	if !isElementary(t) && t != "" {
		if fb := compiler.StandardFunctionBlock(t); fb != nil {
			for _, list := range [][]*ast.VarDeclStatement{fb.VarInputs, fb.VarOutputs} {
				for _, d := range list {
					if d != nil && d.Name != nil && up(d.Name.Value) == k {
						return declType(d.DataType)
					}
				}
			}
		}
	}
	return ""
}

// param is the declared type of a callee's parameter, or "".
func (a *analysis) param(callee *pou, stdFB, name string) string {
	if callee != nil {
		if v, ok := callee.vars[up(name)]; ok {
			return a.resolve(v.typ)
		}
		if callee.kind == "FUNCTION_BLOCK" {
			return a.member(up(callee.name), name)
		}
		return ""
	}
	if stdFB != "" {
		return a.member(stdFB, name)
	}
	return ""
}
