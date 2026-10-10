/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// This file renames IEC 61131-3 identifiers that Go cannot use as written: a
// Go keyword, such as a variable named `go` or `map`, and a name the
// generated code relies on, such as `len`, `min` or the package `iec`, which
// a declaration of the same name would hide. Such a name gets a trailing
// underscore, `go_`, in every place it is declared and used. Within a
// program or function block, a local variable named like the Go receiver
// (`p`, or a function block's initial) or the scan time `now` is renamed the
// same way.

import (
	"unicode"

	"github.com/apiarytech/beedance/ast"
)

// reservedGoNames are the names an IEC declaration cannot keep in Go.
var reservedGoNames = map[string]bool{
	// Go keywords.
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true, "var": true,
	// Predeclared Go names the generated code uses.
	"any": true, "append": true, "bool": true, "copy": true, "error": true, "false": true, "int": true,
	"iota": true, "len": true, "make": true, "max": true, "min": true, "new": true, "nil": true,
	"string": true, "true": true, "uint": true,
	// Packages and functions of the generated file.
	"iec": true, "core": true, "config": true, "vars": true, "timers": true, "counters": true,
	"triggers": true, "numerical": true, "iecstrings": true, "selection": true, "bitwise": true,
	"arithmetic": true, "comparison": true, "conversion": true, "iectime": true, "iecmath": true,
	"fmt": true, "time": true, "strings": true, "context": true, "os": true, "signal": true, "log": true,
	"stdValue": true, "stringCut": true, "plcTime": true, "plcStarted": true, "plcClock": true, "processImage": true, "main": true, "init": true,
}

// GoName returns the Go name of a declared variable, structure member or
// function block member as the transpiler generates it: name itself, or
// name_ when Go cannot use it (a Go keyword, or a name the generated code
// uses). A host addressing a transpiled program's fields uses it.
func GoName(name string) string {
	if reservedGoNames[name] {
		return goSafeName(name)
	}
	return name
}

// goSafeName returns the name an identifier takes in Go.
func goSafeName(name string) string {
	return name + "_"
}

// renameForGo returns the program with every declared name that Go cannot
// use renamed, with the uses of those names.
func renameForGo(program *ast.Program) *ast.Program {
	declared := map[string]bool{}
	for _, stmt := range program.Statements {
		collectDeclaredNames(stmt, declared)
	}
	global := map[string]bool{}
	for name := range declared {
		if reservedGoNames[name] {
			global[name] = true
		}
	}

	for i, stmt := range program.Statements {
		rename := global
		// A local named like the receiver or the scan time, within its POU.
		if locals, receiver := pouLocals(stmt); receiver != "" {
			scoped := map[string]bool{}
			for name := range global {
				scoped[name] = true
			}
			for _, name := range locals {
				if name == receiver || name == "now" {
					scoped[name] = true
				}
			}
			rename = scoped
		}
		if len(rename) == 0 {
			continue
		}
		program.Statements[i] = renameNames(stmt, rename)
	}
	return program
}

// collectDeclaredNames adds the names stmt declares: POUs, types, methods,
// properties, variables, parameters and structure members.
func collectDeclaredNames(stmt ast.Statement, declared map[string]bool) {
	ast.Modify(stmt, func(n ast.Node) ast.Node {
		switch v := n.(type) {
		case *ast.VarDeclStatement:
			if v.Name != nil {
				declared[v.Name.Value] = true
			}
		case *ast.FunctionDeclaration:
			declared[v.Name.Value] = true
		case *ast.FunctionBlockDeclaration:
			declared[v.Name.Value] = true
		case *ast.ProgramDeclaration:
			declared[v.Name.Value] = true
		case *ast.InterfaceDeclaration:
			declared[v.Name.Value] = true
		case *ast.MethodImplementation:
			declared[v.Name.Value] = true
		case *ast.PropertyDeclaration:
			declared[v.Name.Value] = true
		case *ast.TypeDeclaration:
			declared[v.Name.Value] = true
		}
		return n
	})
}

// pouLocals returns the names that are Go locals in a program's or function
// block's methods (its VAR_TEMP, and its methods' parameters and
// variables), with the Go receiver's name; the receiver is "" for any other
// statement.
func pouLocals(stmt ast.Statement) ([]string, string) {
	names := []string{}
	addTemps := func(blocks []*ast.TempVarDeclaration) {
		for _, b := range blocks {
			for _, d := range b.Vars {
				names = append(names, d.Name.Value)
			}
		}
	}
	switch s := stmt.(type) {
	case *ast.ProgramDeclaration:
		addTemps(s.VarTemp)
		return names, "p"
	case *ast.FunctionBlockDeclaration:
		addTemps(s.VarTemp)
		ast.Modify(s, func(n ast.Node) ast.Node {
			if m, ok := n.(*ast.MethodImplementation); ok {
				for _, block := range [][]*ast.VarDeclStatement{m.VarInputs, m.VarOutputs, m.VarInOuts, m.Vars} {
					for _, d := range block {
						names = append(names, d.Name.Value)
					}
				}
			}
			return n
		})
		return names, receiverOf(s.Name.Value)
	}
	return nil, ""
}

// renameNames renames, in stmt, every identifier, declaration and type name
// whose text is in rename.
func renameNames(stmt ast.Statement, rename map[string]bool) ast.Statement {
	renameIdent := func(id *ast.Identifier) {
		if id != nil && rename[id.Value] {
			id.Value = goSafeName(id.Value)
		}
	}
	seen := map[*ast.Identifier]bool{}
	return ast.Modify(stmt, func(n ast.Node) ast.Node {
		switch v := n.(type) {
		case *ast.Identifier:
			// Rename each identifier once; Modify may visit it twice.
			if !seen[v] {
				seen[v] = true
				renameIdent(v)
			}
		case *ast.VarDeclStatement:
			if !seen[v.Name] {
				seen[v.Name] = true
				renameIdent(v.Name)
			}
		case *ast.TypeDeclaration, *ast.ProgramDeclaration, *ast.FunctionDeclaration, *ast.FunctionBlockDeclaration,
			*ast.MethodImplementation, *ast.InterfaceDeclaration:
			// A declaration's own name, which Modify may not visit.
			if name := declarationName(v); name != nil && !seen[name] {
				seen[name] = true
				renameIdent(name)
			}
		case *ast.TypeSpecifier:
			if rename[v.Token.Literal] {
				spec := *v
				spec.Token.Literal = goSafeName(v.Token.Literal)
				return &spec
			}
		case *ast.TypedLiteral:
			if rename[v.TypeName] {
				v.TypeName = goSafeName(v.TypeName)
			}
		}
		return n
	}).(ast.Statement)
}

// declarationName returns the name a declaration declares.
func declarationName(n ast.Node) *ast.Identifier {
	switch v := n.(type) {
	case *ast.TypeDeclaration:
		return v.Name
	case *ast.ProgramDeclaration:
		return v.Name
	case *ast.FunctionDeclaration:
		return v.Name
	case *ast.FunctionBlockDeclaration:
		return v.Name
	case *ast.MethodImplementation:
		return v.Name
	case *ast.InterfaceDeclaration:
		return v.Name
	}
	return nil
}

// receiverOf returns the receiver name of a function block's methods: the
// first letter of its name in lower case, as Go code usually has, so _RMP_B
// gets r. A name without letters gets f.
func receiverOf(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) {
			return string(unicode.ToLower(r))
		}
	}
	return "f"
}
