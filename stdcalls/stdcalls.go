/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package stdcalls lets a program call a standard function with its inputs
// named (IEC 61131-3, the formal call):
//
//	a := LIMIT(MN := 0, IN := v, MX := 20);
//	b := SEL(G := cond, IN0 := x, IN1 := y);
//	c := MAX(IN1 := p, IN2 := q, IN3 := r);
//
// Each engine calls the standard functions with their inputs in order, so
// the call is rewritten to LIMIT(0, v, 20) for every engine alike. A call
// of a function the program declares, or with an input the standard does
// not name (EN), is left as it is.
package stdcalls

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// inputs are the inputs of the standard functions that are not IN, or IN1,
// IN2, ... in order, by function.
var inputs = map[string][]string{
	"LIMIT": {"MN", "IN", "MX"},
	"SEL":   {"G", "IN0", "IN1"},
	"SHL":   {"IN", "N"}, "SHR": {"IN", "N"}, "ROL": {"IN", "N"}, "ROR": {"IN", "N"},
	"LEFT": {"IN", "L"}, "RIGHT": {"IN", "L"}, "MID": {"IN", "L", "P"},
	"INSERT": {"IN1", "IN2", "P"}, "DELETE": {"IN", "L", "P"},
	"REPLACE": {"IN1", "IN2", "L", "P"}, "FIND": {"IN1", "IN2"},
	"ATAN2":       {"Y", "X"},
	"CONCAT_DATE": {"YEAR", "MONTH", "DAY"},
	"CONCAT_TOD":  {"HOUR", "MINUTE", "SECOND", "MILLISECOND"},
	"CONCAT_LTOD": {"HOUR", "MINUTE", "SECOND", "MILLISECOND"},
	"CONCAT_DT":   {"YEAR", "MONTH", "DAY", "HOUR", "MINUTE", "SECOND", "MILLISECOND"},
	"CONCAT_LDT":  {"YEAR", "MONTH", "DAY", "HOUR", "MINUTE", "SECOND", "MILLISECOND"},
}

// numbered matches an input of an extensible function: IN1, IN2, ... (and
// MUX's IN0, IN1, ... after K).
var numbered = regexp.MustCompile(`^IN(\d+)$`)

// Positional rewrites the formal calls of standard functions in the
// program in place.
func Positional(program *ast.Program) {
	if program == nil {
		return
	}
	declared := map[string]bool{}
	for _, s := range program.Statements {
		switch d := s.(type) {
		case *ast.FunctionDeclaration:
			if d.Name != nil {
				declared[strings.ToUpper(d.Name.Value)] = true
			}
		case *ast.FunctionBlockDeclaration:
			if d.Name != nil {
				declared[strings.ToUpper(d.Name.Value)] = true
			}
		}
	}
	// A variable shadows a function of its name: gt(IN := TRUE) of a TON gt.
	ast.Modify(program, func(n ast.Node) ast.Node {
		if d, ok := n.(*ast.VarDeclStatement); ok && d.Name != nil {
			declared[strings.ToUpper(d.Name.Value)] = true
		}
		return n
	})
	// SPLIT_DT(IN := dt, YEAR => y, ...); see splits.go.
	splits(program, declared)
	ast.Modify(program, func(n ast.Node) ast.Node {
		call, ok := n.(*ast.CallExpression)
		if !ok || len(call.Arguments) == 0 {
			return n
		}
		ident, ok := call.Function.(*ast.Identifier)
		if !ok || declared[strings.ToUpper(ident.Value)] {
			return n
		}
		// Only a standard function: t(IN := x) of a TON instance keeps its names.
		if !isStandard(strings.ToUpper(ident.Value)) {
			return n
		}
		if args, ok := inOrder(strings.ToUpper(ident.Value), call.Arguments); ok {
			call.Arguments = args
		}
		return n
	})
}

// inOrder returns the inputs of a formal call of the standard function name
// in order, or false if the call is not one.
func inOrder(name string, args []ast.Expression) ([]ast.Expression, bool) {
	byName := map[string]ast.Expression{}
	for _, a := range args {
		named, ok := a.(*ast.NamedArgument)
		if !ok || named.Name == nil {
			return nil, false
		}
		key := strings.ToUpper(named.Name.Value)
		if _, twice := byName[key]; twice {
			return nil, false
		}
		byName[key] = named.Value
	}
	var order []string
	switch {
	case inputs[name] != nil:
		order = inputs[name]
	case len(byName) == 1 && byName["IN"] != nil:
		order = []string{"IN"} // ABS(IN := x), INT_TO_REAL(IN := i), ...
	default:
		// An extensible function: MAX(IN1 := a, IN2 := b), MUX(K := k, IN0 := a, ...).
		type input struct {
			n    int
			name string
		}
		var ins []input
		for key := range byName {
			if key == "K" && name == "MUX" {
				continue
			}
			m := numbered.FindStringSubmatch(key)
			if m == nil {
				return nil, false
			}
			n, _ := strconv.Atoi(m[1])
			ins = append(ins, input{n, key})
		}
		sort.Slice(ins, func(i, j int) bool { return ins[i].n < ins[j].n })
		if name == "MUX" {
			if byName["K"] == nil {
				return nil, false
			}
			order = append(order, "K")
		}
		for _, in := range ins {
			order = append(order, in.name)
		}
	}
	if len(order) != len(byName) {
		return nil, false
	}
	out := make([]ast.Expression, 0, len(order))
	for _, key := range order {
		v, ok := byName[key]
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}

// standard are the standard functions besides the conversions and the
// functions of time types, which object names.
var standard = map[string]bool{
	"ABS": true, "SQRT": true, "LN": true, "LOG": true, "EXP": true, "SIN": true, "COS": true, "TAN": true,
	"ASIN": true, "ACOS": true, "ATAN": true, "ATAN2": true, "EXPT": true, "MOVE": true, "TRUNC": true,
	"ADD": true, "MUL": true, "SUB": true, "DIV": true, "MOD": true,
	"AND": true, "OR": true, "XOR": true, "NOT": true, "SHL": true, "SHR": true, "ROL": true, "ROR": true,
	"SEL": true, "MAX": true, "MIN": true, "LIMIT": true, "MUX": true,
	"GT": true, "GE": true, "EQ": true, "LE": true, "LT": true, "NE": true,
	"LEN": true, "LEFT": true, "RIGHT": true, "MID": true, "CONCAT": true, "INSERT": true, "DELETE": true,
	"REPLACE": true, "FIND": true,
	"TO_BIG_ENDIAN": true, "TO_LITTLE_ENDIAN": true, "FROM_BIG_ENDIAN": true, "FROM_LITTLE_ENDIAN": true,
	"IS_VALID": true, "IS_VALID_BCD": true, "DAY_OF_WEEK": true,
}

// isStandard reports whether name (upper case) is a standard function, or
// a built-in function the host registers.
func isStandard(name string) bool {
	if standard[name] || object.IsTimeFunction(name) {
		return true
	}
	if _, _, isConversion := object.ParseConversion(name); isConversion {
		return true
	}
	_, registered := object.GetBuiltinByName(name)
	return registered
}
