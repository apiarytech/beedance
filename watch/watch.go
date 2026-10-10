/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package watch lists what a host can watch of a program while it runs,
// scan by scan, on either engine: its variables of elementary types, and
// the members of its function block instances and structures under dotted
// names, e.g. p.MV or p.tSample.ET. A simulator, a test runner or a trace
// view records the same names whichever engine runs the program.
package watch

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
)

// Members chooses which members of a function block instance are listed.
type Members int

const (
	// Outputs lists an instance's VAR_OUTPUT members: what it gives its
	// caller.
	Outputs Members = iota
	// All lists its VAR_INPUT, VAR_OUTPUT and VAR members too. A standard
	// function block's internal state (a timer's start time) is never
	// listed: the engines keep it differently.
	All
)

// Options control what Variables lists.
type Options struct {
	Members Members
	// Depth is how many levels of members are listed below a program
	// variable: 1 lists p.MV, 2 also p.tSample.ET. Default 2.
	Depth int
}

// Variable is a watched value.
type Variable struct {
	// Name is the dotted name, spelled as declared, e.g. "p.tSample.ET".
	Name string
	// Path is Name split at the dots.
	Path []string
	// Type is the declared elementary type, upper case, e.g. "REAL".
	Type string
}

// elementary are the types whose values are watched.
var elementary = map[string]bool{
	"BOOL": true, "SINT": true, "INT": true, "DINT": true, "LINT": true,
	"USINT": true, "UINT": true, "UDINT": true, "ULINT": true,
	"REAL": true, "LREAL": true, "BYTE": true, "WORD": true, "DWORD": true, "LWORD": true,
	"STRING": true, "TIME": true,
}

// Variables lists the watched values of decl, a program of program: its
// variables of elementary types in declaration order (VAR_INPUT,
// VAR_OUTPUT, VAR_IN_OUT, VAR), each instance or structure followed by its
// members, as o chooses.
func Variables(program *ast.Program, decl *ast.ProgramDeclaration, o Options) []Variable {
	if o.Depth <= 0 {
		o.Depth = 2
	}
	l := lister{o: o, fbs: map[string]*ast.FunctionBlockDeclaration{}, structs: map[string]*ast.StructDefinition{}}
	l.collect(program.Statements)
	for _, group := range [][]*ast.VarDeclStatement{decl.VarInputs, decl.VarOutputs, decl.VarInOuts, decl.Vars} {
		for _, d := range group {
			l.add(nil, d, 0)
		}
	}
	return l.out
}

type lister struct {
	o       Options
	fbs     map[string]*ast.FunctionBlockDeclaration // by upper-case name
	structs map[string]*ast.StructDefinition
	out     []Variable
}

// collect records the function blocks and structures declared in stmts,
// also inside namespaces, by their own names.
func (l *lister) collect(stmts []ast.Statement) {
	for _, s := range stmts {
		switch s := s.(type) {
		case *ast.FunctionBlockDeclaration:
			if s.Name != nil {
				l.fbs[strings.ToUpper(s.Name.Value)] = s
			}
		case *ast.TypeBlockDeclaration:
			for _, td := range s.Declarations {
				if sd, ok := td.DataType.(*ast.StructDefinition); ok && td.Name != nil {
					l.structs[strings.ToUpper(td.Name.Value)] = sd
				}
			}
		case *ast.NamespaceDeclaration:
			l.collect(s.Statements)
		}
	}
}

// add lists the variable d, under the path prefix, depth levels below a
// program variable.
func (l *lister) add(prefix []string, d *ast.VarDeclStatement, depth int) {
	if d == nil || d.Name == nil || d.DataType == nil {
		return
	}
	path := append(append([]string{}, prefix...), d.Name.Value)
	typ := strings.ToUpper(d.DataType.String())
	if elementary[typ] {
		l.out = append(l.out, Variable{Name: strings.Join(path, "."), Path: path, Type: typ})
		return
	}
	if depth >= l.o.Depth {
		return
	}
	// A type named in a namespace, Lib.PID, is looked up as PID.
	if i := strings.LastIndex(typ, "."); i >= 0 {
		typ = typ[i+1:]
	}
	if sd := l.structs[typ]; sd != nil {
		for _, m := range sd.Members {
			l.add(path, m, depth+1)
		}
		return
	}
	if fb := l.fbs[typ]; fb != nil {
		for _, m := range l.members(fb, map[string]bool{}) {
			l.add(path, m, depth+1)
		}
		return
	}
	if fb := compiler.StandardFunctionBlock(typ); fb != nil {
		groups := [][]*ast.VarDeclStatement{fb.VarOutputs}
		if l.o.Members == All {
			groups = [][]*ast.VarDeclStatement{fb.VarInputs, fb.VarOutputs}
		}
		for _, g := range groups {
			for _, m := range g {
				l.add(path, m, depth+1)
			}
		}
	}
}

// members are the listed members of a user function block, its parent's
// (EXTENDS) first.
func (l *lister) members(fb *ast.FunctionBlockDeclaration, seen map[string]bool) []*ast.VarDeclStatement {
	name := strings.ToUpper(fb.Name.Value)
	if seen[name] {
		return nil
	}
	seen[name] = true
	var out []*ast.VarDeclStatement
	if fb.Extends != nil {
		parent := strings.ToUpper(fb.Extends.String())
		if i := strings.LastIndex(parent, "."); i >= 0 {
			parent = parent[i+1:]
		}
		if p := l.fbs[parent]; p != nil {
			out = l.members(p, seen)
		}
	}
	if l.o.Members == All {
		out = append(out, fb.VarInputs...)
		out = append(out, fb.VarOutputs...)
		return append(out, fb.Vars...)
	}
	return append(out, fb.VarOutputs...)
}

// Evaluator reads v from env, the environment of a program instance on the
// tree-walking evaluator (object.Program.Env).
func Evaluator(env *object.Environment, v Variable) (object.Object, bool) {
	root, ok := env.Get(v.Path[0])
	if !ok {
		return nil, false
	}
	return Member(root, v.Path[1:])
}

// VM reads v from a VM's globals, given the global slot of each program
// variable by upper-case name (from compiler.CompiledProgram.Variables).
func VM(globals []object.Object, slots map[string]int, v Variable) (object.Object, bool) {
	g, ok := slots[strings.ToUpper(v.Path[0])]
	if !ok || g < 0 || g >= len(globals) || globals[g] == nil {
		return nil, false
	}
	return Member(globals[g], v.Path[1:])
}

// Member follows path through the members of o: of a function block
// instance on the evaluator, or of an instance or structure held as an
// *object.Hash. Names match in any case.
func Member(o object.Object, path []string) (object.Object, bool) {
	for _, name := range path {
		switch v := o.(type) {
		case *object.FunctionBlockInstance:
			m, ok := v.Env.GetRaw(name)
			if !ok {
				return nil, false
			}
			o = m
		case *object.Hash:
			var found object.Object
			for _, pair := range v.Pairs {
				if strings.EqualFold(pair.Key.Inspect(), name) {
					found = pair.Value
					break
				}
			}
			if found == nil {
				return nil, false
			}
			o = found
		default:
			return nil, false
		}
	}
	return o, o != nil
}
