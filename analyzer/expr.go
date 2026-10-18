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

// The types of expressions, and the values of constant ones.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// typeOf is the type of an expression inside POU p, or "" when it cannot
// be worked out.
func (a *analysis) typeOf(p *pou, e ast.Expression) string {
	switch e := e.(type) {
	case *ast.Identifier:
		if v := a.lookup(p, e.Value); v != nil {
			return a.resolve(v.typ)
		}
		if t, ok := a.enumVals[up(e.Value)]; ok {
			return t
		}
	case *ast.DirectVariable:
		return directType(e.Address)
	case *ast.IntegerLiteral, *ast.UnsignedIntegerLiteral:
		return anyInt
	case *ast.RealLiteral:
		return anyReal
	case *ast.LRealLiteral:
		return "LREAL"
	case *ast.BitStringLiteral:
		switch e.Width {
		case 8:
			return "BYTE"
		case 16:
			return "WORD"
		case 32:
			return "DWORD"
		case 64:
			return "LWORD"
		}
	case *ast.Boolean:
		return "BOOL"
	case *ast.StringLiteral:
		return "STRING"
	case *ast.WStringLiteral:
		return "WSTRING"
	case *ast.TimeLiteral:
		return "TIME"
	case *ast.DateLiteral:
		return "DATE"
	case *ast.TimeOfDayLiteral:
		return "TOD"
	case *ast.DateAndTimeLiteral:
		return "DT"
	case *ast.TypedLiteral:
		return a.resolve(normalize(e.TypeName))
	case *ast.EnumeratedValueLiteral:
		if e.TypeName != nil {
			return up(e.TypeName.Value)
		}
	case *ast.PrefixExpression:
		t := a.typeOf(p, e.Right)
		if up(e.Operator) == "NOT" || e.Operator == "-" || e.Operator == "+" {
			return t
		}
	case *ast.InfixExpression:
		return a.infixType(p, e)
	case *ast.CallExpression:
		return a.callType(p, e)
	case *ast.MemberAccessExpression:
		if e.Member == nil {
			return ""
		}
		return a.member(a.typeOf(p, e.Struct), e.Member.Value)
	case *ast.IndexExpression:
		if t := a.typeOf(p, e.Left); strings.HasPrefix(t, "ARRAY OF ") {
			return a.resolve(strings.TrimPrefix(t, "ARRAY OF "))
		}
	case *ast.BitAccessExpression:
		return "BOOL"
	case *ast.DereferenceExpression:
		if t := a.typeOf(p, e.Pointer); strings.HasPrefix(t, "REF_TO ") {
			return a.resolve(strings.TrimPrefix(t, "REF_TO "))
		}
	}
	return ""
}

func isComparison(op string) bool {
	switch op {
	case "=", "<>", "<", ">", "<=", ">=":
		return true
	}
	return false
}

func (a *analysis) infixType(p *pou, e *ast.InfixExpression) string {
	op := up(e.Operator)
	if isComparison(op) {
		return "BOOL"
	}
	l, r := a.typeOf(p, e.Left), a.typeOf(p, e.Right)
	switch op {
	case "AND", "OR", "XOR", "&", "AND_THEN", "OR_ELSE":
		if l == "BOOL" && r == "BOOL" {
			return "BOOL"
		}
		return wider(l, r)
	case "+", "-", "*", "/":
		if t, ok := timeArithmetic(op, l, r); ok {
			return t
		}
		return wider(l, r)
	case "MOD":
		return wider(l, r)
	case "**":
		if w := wider(l, r); isReal(w) {
			return w
		}
	}
	return ""
}

// timeArithmetic is the type of +, -, * and / when an operand is a time,
// date or duration; ok is false when neither is.
func timeArithmetic(op, l, r string) (string, bool) {
	lt, rt := familyOf(l), familyOf(r)
	if lt != famTime && lt != famDate && rt != famTime && rt != famDate {
		return "", false
	}
	switch {
	case op == "+" && lt == famTime && rt == famTime:
		return l, true
	case op == "+" && lt == famDate && rt == famTime:
		return l, true
	case op == "-" && lt == famTime && rt == famTime:
		return l, true
	case op == "-" && lt == famDate && rt == famTime:
		return l, true
	case op == "-" && lt == famDate && rt == famDate:
		return "TIME", true
	case (op == "*" || op == "/") && lt == famTime && numeric(r):
		return l, true
	case op == "*" && numeric(l) && rt == famTime:
		return r, true
	}
	return "", true
}

// The standard functions whose result has the type of their arguments.
var argTyped = map[string]int{ // the first argument that counts
	"ADD": 0, "SUB": 0, "MUL": 0, "DIV": 0, "MOD": 0, "MAX": 0, "MIN": 0, "MOVE": 0, "ABS": 0,
	"LIMIT": 0, "SEL": 1, "MUX": 1, "AND": 0, "OR": 0, "XOR": 0, "NOT": 0,
}

// The standard functions of one argument whose result has its real type.
var realTyped = map[string]bool{
	"SQRT": true, "LN": true, "LOG": true, "EXP": true, "SIN": true, "COS": true, "TAN": true,
	"ASIN": true, "ACOS": true, "ATAN": true, "EXPT": true,
}

func (a *analysis) callType(p *pou, e *ast.CallExpression) string {
	switch f := e.Function.(type) {
	case *ast.Identifier:
		name := up(f.Value)
		if v := a.lookup(p, f.Value); v != nil {
			return "" // a function block instance's call
		}
		if fn, ok := a.funcs[name]; ok {
			return a.resolve(fn.result)
		}
		args := argValues(e.Arguments)
		if i := strings.LastIndex(name, "_TO_"); i >= 0 {
			return normalize(name[i+4:])
		}
		if strings.HasPrefix(name, "TO_") {
			return normalize(name[3:])
		}
		if first, ok := argTyped[name]; ok {
			t := ""
			for i := first; i < len(args); i++ {
				at := a.typeOf(p, args[i])
				if i == first {
					t = at
					continue
				}
				t = wider(t, at)
			}
			return t
		}
		if realTyped[name] && len(args) > 0 {
			if t := a.typeOf(p, args[0]); isReal(t) {
				return t
			}
			return ""
		}
		switch name {
		case "SHL", "SHR", "ROL", "ROR":
			if len(args) > 0 {
				return a.typeOf(p, args[0])
			}
		case "GT", "GE", "EQ", "LE", "LT", "NE":
			return "BOOL"
		case "LEN", "FIND":
			return "INT"
		case "LEFT", "RIGHT", "MID", "CONCAT", "INSERT", "DELETE", "REPLACE":
			if len(args) > 0 && a.typeOf(p, args[0]) == "WSTRING" {
				return "WSTRING"
			}
			return "STRING"
		}
	case *ast.MemberAccessExpression:
		if f.Member == nil {
			return ""
		}
		if fb, ok := a.fbs[a.typeOf(p, f.Struct)]; ok {
			if m := a.method(fb, f.Member.Value); m != nil {
				return a.resolve(m.result)
			}
		}
	}
	return ""
}

// method finds a method of a function block or of one it extends.
func (a *analysis) method(fb *pou, name string) *pou {
	seen := map[*pou]bool{}
	for cur := fb; cur != nil && !seen[cur]; cur = a.fbs[cur.base] {
		seen[cur] = true
		if m, ok := cur.methods[up(name)]; ok {
			return m
		}
	}
	return nil
}

// argValues are a call's argument values, named or not, without outputs.
func argValues(args []ast.Expression) []ast.Expression {
	var out []ast.Expression
	for _, arg := range args {
		switch x := arg.(type) {
		case *ast.NamedArgument:
			out = append(out, x.Value)
		case *ast.OutputArgument:
		default:
			out = append(out, x)
		}
	}
	return out
}

// value is a constant's value: a BOOL or an integer.
type value struct {
	isBool bool
	b      bool
	i      int64
}

// constant is the value of a constant expression: literals, VAR CONSTANT
// variables initialized with one, and operators on them.
func (a *analysis) constant(p *pou, e ast.Expression) (value, bool) {
	switch e := e.(type) {
	case *ast.Boolean:
		return value{isBool: true, b: e.Value}, true
	case *ast.IntegerLiteral:
		return value{i: e.Value}, true
	case *ast.TypedLiteral:
		if isInteger(a.resolve(normalize(e.TypeName))) || up(e.TypeName) == "BOOL" {
			return a.constant(p, e.Value)
		}
	case *ast.Identifier:
		// VAR CONSTANT and VAR_GLOBAL CONSTANT; an input declared CONSTANT
		// only may not be written inside its POU, a caller gives its value.
		if v := a.lookup(p, e.Value); v != nil && v.constant && v.decl != nil && v.decl.Value != nil &&
			(v.section == "VAR" || v.section == "VAR_GLOBAL" || v.section == "VAR_EXTERNAL") {
			return a.constant(nil, v.decl.Value)
		}
	case *ast.PrefixExpression:
		v, ok := a.constant(p, e.Right)
		if !ok {
			return v, false
		}
		switch {
		case up(e.Operator) == "NOT" && v.isBool:
			return value{isBool: true, b: !v.b}, true
		case e.Operator == "-" && !v.isBool:
			return value{i: -v.i}, true
		case e.Operator == "+" && !v.isBool:
			return v, true
		}
	case *ast.InfixExpression:
		l, ok1 := a.constant(p, e.Left)
		r, ok2 := a.constant(p, e.Right)
		if !ok1 || !ok2 {
			return value{}, false
		}
		return foldInfix(up(e.Operator), l, r)
	}
	return value{}, false
}

func foldInfix(op string, l, r value) (value, bool) {
	if l.isBool && r.isBool {
		switch op {
		case "AND", "&", "AND_THEN":
			return value{isBool: true, b: l.b && r.b}, true
		case "OR", "OR_ELSE":
			return value{isBool: true, b: l.b || r.b}, true
		case "XOR":
			return value{isBool: true, b: l.b != r.b}, true
		case "=":
			return value{isBool: true, b: l.b == r.b}, true
		case "<>":
			return value{isBool: true, b: l.b != r.b}, true
		}
		return value{}, false
	}
	if l.isBool || r.isBool {
		return value{}, false
	}
	b := func(x bool) (value, bool) { return value{isBool: true, b: x}, true }
	switch op {
	case "+":
		return value{i: l.i + r.i}, true
	case "-":
		return value{i: l.i - r.i}, true
	case "*":
		return value{i: l.i * r.i}, true
	case "/":
		if r.i != 0 {
			return value{i: l.i / r.i}, true
		}
	case "MOD":
		if r.i != 0 {
			return value{i: l.i % r.i}, true
		}
	case "=":
		return b(l.i == r.i)
	case "<>":
		return b(l.i != r.i)
	case "<":
		return b(l.i < r.i)
	case ">":
		return b(l.i > r.i)
	case "<=":
		return b(l.i <= r.i)
	case ">=":
		return b(l.i >= r.i)
	}
	return value{}, false
}
