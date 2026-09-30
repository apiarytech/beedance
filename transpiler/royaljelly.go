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

// This file ties the generated Go to royaljelly, the runtime it targets:
// the standard functions of its std packages, the standard function blocks
// of its fb packages, and the imports a generated file needs.

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/format"
	goparser "go/parser"
	"go/token"
	"sort"
	"strings"

	"beedance/ast"
)

// stdFunction describes a royaljelly standard function; see
// royaljelly_functions.go.
type stdFunction struct {
	pkg      string   // The std package, e.g. "numerical".
	params   []string // Each parameter's Go type, or "" for a type parameter.
	variadic bool     // The last parameter takes any number of arguments.
	result   string   // The result's Go type, or "" for a type parameter.
	err      bool     // An error follows the result.
}

const royaljellyModule = "github.com/apiarytech/royaljelly"

// generatedPackages maps the name the generated code uses for each package
// to its import path. royaljelly's strings, time and math packages are
// renamed so they do not hide Go's packages of the same name.
var generatedPackages = map[string]string{
	"iec":        royaljellyModule + "/iec",
	"core":       royaljellyModule + "/core",
	"config":     royaljellyModule + "/config",
	"vars":       royaljellyModule + "/vars",
	"timers":     royaljellyModule + "/fb/timers",
	"counters":   royaljellyModule + "/fb/counters",
	"triggers":   royaljellyModule + "/fb/triggers",
	"numerical":  royaljellyModule + "/std/numerical",
	"iecstrings": royaljellyModule + "/std/strings",
	"selection":  royaljellyModule + "/std/selection",
	"bitwise":    royaljellyModule + "/std/bitwise",
	"arithmetic": royaljellyModule + "/std/arithmetic",
	"comparison": royaljellyModule + "/std/comparison",
	"conversion": royaljellyModule + "/std/conversion",
	"iectime":    royaljellyModule + "/std/time",
	"iecmath":    royaljellyModule + "/std/math",
	"fmt":        "fmt",
	"time":       "time",
	"strings":    "strings",
	"context":    "context",
	"os":         "os",
	"signal":     "os/signal",
	"log":        "log",
}

// goPackageAlias is the name the generated code uses for a royaljelly std
// package.
func goPackageAlias(pkg string) string {
	switch pkg {
	case "strings", "time", "math":
		return "iec" + pkg
	}
	return pkg
}

// standardFunctionBlock is a royaljelly standard function block: its Go type
// the call that runs it once, and whether it has EN and ENO.
type standardFunctionBlock struct {
	goType string
	run    string
	hasEN  bool // It has EN and ENO.
}

// standardFunctionBlocks maps the IEC 61131-3 standard function blocks to
// royaljelly's.
var standardFunctionBlocks = map[string]standardFunctionBlock{
	"TON":    {"timers.TON", "Execute(now)", false},
	"TOF":    {"timers.TOF", "Execute(now)", false},
	"TP":     {"timers.TP", "Execute(now)", false},
	"CTU":    {"counters.CTU", "Execute()", true},
	"CTD":    {"counters.CTD", "Execute()", true},
	"CTUD":   {"counters.CTUD", "Execute()", true},
	"R_TRIG": {"triggers.R_TRIG", "R_TRIG()", false},
	"F_TRIG": {"triggers.F_TRIG", "F_TRIG()", false},
	"SR":     {"triggers.SR_FB", "SR()", true},
	"RS":     {"triggers.RS_FB", "RS()", true},
}

// standardFunctionBlockOf returns the royaljelly standard function block a
// data type names, unless the program declares a type of the same name.
func (t *Transpiler) standardFunctionBlockOf(dataType ast.Expression) (standardFunctionBlock, bool) {
	if dataType == nil {
		return standardFunctionBlock{}, false
	}
	name := dataType.String()
	if _, declared := t.typeInfo[name]; declared {
		return standardFunctionBlock{}, false
	}
	fb, ok := standardFunctionBlocks[strings.ToUpper(name)]
	return fb, ok
}

// functionBlockRun returns the call that runs the function block instance
// fbExpr once: its Logic method, or a standard function block's own call.
func (t *Transpiler) functionBlockRun(fbExpr ast.Expression) string {
	if td := t.resolveAssignmentTargetType(fbExpr); td != nil {
		if fb, ok := t.standardFunctionBlockOf(td.DataType); ok {
			return fb.run
		}
	}
	return "Logic(now)"
}

// realOnlyFunctions take and give only REAL or LREAL values, so an untyped
// literal argument is an LREAL.
var realOnlyFunctions = map[string]bool{
	"SQRT": true, "LN": true, "LOG": true, "EXP": true, "SIN": true, "COS": true, "TAN": true,
	"ASIN": true, "ACOS": true, "ATAN": true, "EXPT": true, "TRUNC": true,
}

// transpileRoyaljellyCall transpiles a call to a royaljelly standard
// function. Arguments are converted to the parameters' types; untyped
// literals given to a type parameter take the type the call's value is
// expected to have, or a default; and the value is converted to the type
// expected of it.
func (t *Transpiler) transpileRoyaljellyCall(name string, fn stdFunction, exp *ast.CallExpression) error {
	expected := t.expectedGoType
	t.expectedGoType = ""
	defer func() { t.expectedGoType = expected }()

	literalType := "iec.LINT"
	if realOnlyFunctions[name] {
		literalType = "iec.LREAL"
	}
	allLiterals := true
	for i, arg := range exp.Arguments {
		if _, named := arg.(*ast.NamedArgument); named {
			return fmt.Errorf("the standard function %s takes its arguments in order, got %s", name, arg.String())
		}
		if paramType(fn, i) != "" {
			continue
		}
		switch untypedLiteral(arg) {
		case "":
			allLiterals = false
		case "real":
			literalType = "iec.LREAL"
		}
	}
	if fn.result == "" && isNumericGoType(expected) && (!realOnlyFunctions[name] || isRealGoType(expected)) {
		literalType = expected
	}

	convert := expected != "" && fn.result != expected
	if convert {
		t.write("%s(", expected)
	}
	if fn.err {
		t.usesStdValue = true
		t.write("stdValue(")
	}
	t.write("%s.%s(", goPackageAlias(fn.pkg), name)
	for i, arg := range exp.Arguments {
		if i > 0 {
			t.write(", ")
		}
		wrap := paramType(fn, i)
		if wrap == "" && allLiterals && untypedLiteral(arg) != "" {
			wrap = literalType
		}
		if wrap != "" && !strings.HasPrefix(wrap, "[") && !strings.HasPrefix(wrap, "*") && wrap != "any" && wrap != "func" {
			t.write("%s(", wrap)
		} else {
			wrap = ""
		}
		if err := t.transpileExpression(arg); err != nil {
			return err
		}
		if wrap != "" {
			t.write(")")
		}
	}
	t.write(")")
	if fn.err {
		t.write(")")
	}
	if convert {
		t.write(")")
	}
	return nil
}

// paramType returns the Go type of a function's i-th parameter, or "" for a
// type parameter.
func paramType(fn stdFunction, i int) string {
	if len(fn.params) == 0 {
		return ""
	}
	if i >= len(fn.params) {
		if !fn.variadic {
			return ""
		}
		i = len(fn.params) - 1
	}
	return fn.params[i]
}

// untypedLiteral returns "int" or "real" for a numeric literal that Go
// treats as an untyped constant, or "".
func untypedLiteral(e ast.Expression) string {
	switch v := e.(type) {
	case *ast.IntegerLiteral:
		return "int"
	case *ast.RealLiteral:
		return "real"
	case *ast.PrefixExpression:
		if v.Operator == "-" || v.Operator == "+" {
			return untypedLiteral(v.Right)
		}
	}
	return ""
}

func isRealGoType(goType string) bool {
	return goType == "iec.REAL" || goType == "iec.LREAL"
}

func isNumericGoType(goType string) bool {
	switch goType {
	case "iec.SINT", "iec.INT", "iec.DINT", "iec.LINT", "iec.USINT", "iec.UINT", "iec.UDINT", "iec.ULINT",
		"iec.REAL", "iec.LREAL":
		return true
	}
	return false
}

// elementaryGoType returns the Go type of an assignment target when it is
// an elementary IEC type, such as iec.INT, or "".
func (t *Transpiler) elementaryGoType(target ast.Expression) string {
	td := t.resolveAssignmentTargetType(target)
	if td == nil || td.DataType == nil {
		return ""
	}
	dataType := td.DataType
	if _, isIndex := target.(*ast.IndexExpression); isIndex {
		def := t.arrayDefinitionOf(dataType)
		if def == nil || def.DataType == nil {
			return ""
		}
		dataType = def.DataType
	}
	switch dataType.(type) {
	case *ast.EnumDefinition, *ast.StructDefinition:
		return "" // A user-defined type.
	}
	if goType := t.mapIecTypeToGo(dataType); strings.HasPrefix(goType, "iec.") {
		return goType
	}
	return ""
}

// stdValueHelper returns the value of a royaljelly function that also
// returns an error. The error only reports what IEC 61131-3 leaves the value
// to show, such as MID past the end of a string giving a shorter string.
const stdValueHelper = `
// stdValue returns the value of a standard function that also returns an error.
func stdValue[T any](value T, _ error) T { return value }
`

// GoFile transpiles program into a complete, formatted Go source file of
// package main, importing each package the generated code uses.
func GoFile(program *ast.Program) ([]byte, error) {
	var body bytes.Buffer
	t := New(&body)
	if err := t.Transpile(program); err != nil {
		return nil, err
	}
	return goFileWithImports(body.Bytes())
}

// goFileWithImports adds the package clause and the imports that body
// refers to, and formats the file.
func goFileWithImports(body []byte) ([]byte, error) {
	src := append([]byte("package main\n\n"), body...)
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "main.go", src, goparser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("the generated Go does not parse: %w", err)
	}

	// Names declared at the top level of the file are not packages.
	declared := map[string]bool{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *goast.FuncDecl:
			if d.Recv == nil {
				declared[d.Name.Name] = true
			}
		case *goast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *goast.TypeSpec:
					declared[s.Name.Name] = true
				case *goast.ValueSpec:
					for _, n := range s.Names {
						declared[n.Name] = true
					}
				}
			}
		}
	}
	used := map[string]bool{}
	goast.Inspect(file, func(n goast.Node) bool {
		if sel, ok := n.(*goast.SelectorExpr); ok {
			if id, ok := sel.X.(*goast.Ident); ok && !declared[id.Name] {
				if _, known := generatedPackages[id.Name]; known {
					used[id.Name] = true
				}
			}
		}
		return true
	})
	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	sort.Strings(names)

	var out bytes.Buffer
	out.WriteString("package main\n\n")
	if len(names) > 0 {
		out.WriteString("import (\n")
		for _, name := range names {
			path := generatedPackages[name]
			if path[strings.LastIndex(path, "/")+1:] == name {
				fmt.Fprintf(&out, "\t%q\n", path)
			} else {
				fmt.Fprintf(&out, "\t%s %q\n", name, path)
			}
		}
		out.WriteString(")\n\n")
	}
	out.Write(body)
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return out.Bytes(), fmt.Errorf("the generated Go does not format: %w", err)
	}
	return formatted, nil
}
