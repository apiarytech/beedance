/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ide

// This file shows a diagram live, whatever runs the program: the
// evaluator (the simulator), the VM, transpiled Go on a target, a SIL or
// HIL rig, or a controller beehive reaches. Each is a Values: it reads
// named values. A diagram says what it needs (Watches, read from its
// drawing, so the two always agree), and Live reads them and works out
// what no variable holds: the value on each wire, a block's result.

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/token"
	"github.com/apiarytech/beedance/watch"
)

// Values reads named values of a running program: dotted names, as
// package watch names them (p.MV, t1.Q). A name it cannot read is left
// out of the map.
type Values interface {
	Read(names []string) (map[string]object.Object, error)
}

// ValuesFunc is a function as a Values.
type ValuesFunc func(names []string) (map[string]object.Object, error)

// Read calls f.
func (f ValuesFunc) Read(names []string) (map[string]object.Object, error) { return f(names) }

// FromEvaluator reads a program's values on the evaluator, from the
// program's environment.
func FromEvaluator(env *object.Environment) Values {
	return ValuesFunc(func(names []string) (map[string]object.Object, error) {
		out := map[string]object.Object{}
		for _, n := range names {
			if o, ok := watch.Evaluator(env, watch.Variable{Name: n, Path: strings.Split(n, ".")}); ok {
				out[n] = o
			}
		}
		return out, nil
	})
}

// FromVM reads a program's values on the VM, from its globals and the
// slot of each program variable by upper-case name.
func FromVM(globals []object.Object, slots map[string]int) Values {
	return ValuesFunc(func(names []string) (map[string]object.Object, error) {
		out := map[string]object.Object{}
		for _, n := range names {
			if o, ok := watch.VM(globals, slots, watch.Variable{Name: n, Path: strings.Split(n, ".")}); ok {
				out[n] = o
			}
		}
		return out, nil
	})
}

// FromMap reads values already read, names matched in any case: what a
// SIL rig sent, or a controller answered.
func FromMap(m map[string]object.Object) Values {
	upper := map[string]object.Object{}
	for k, v := range m {
		upper[strings.ToUpper(k)] = v
	}
	return ValuesFunc(func(names []string) (map[string]object.Object, error) {
		out := map[string]object.Object{}
		for _, n := range names {
			if o, ok := upper[strings.ToUpper(n)]; ok {
				out[n] = o
			}
		}
		return out, nil
	})
}

// FromJSON reads values given as JSON, as a controller's API gives them
// (beehive's LogicService.ReadVariables: valueJson): true and false are
// BOOLs, whole numbers LINTs, others LREALs, strings STRINGs.
func FromJSON(m map[string]string) Values {
	objs := map[string]object.Object{}
	for k, v := range m {
		if o := jsonObject(v); o != nil {
			objs[k] = o
		}
	}
	return FromMap(objs)
}

func jsonObject(v string) object.Object {
	var x any
	if json.Unmarshal([]byte(v), &x) != nil {
		return nil
	}
	switch t := x.(type) {
	case bool:
		return &object.Boolean{Value: t}
	case float64:
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return &object.LInt{Value: n}
		}
		return &object.LReal{Value: t}
	case string:
		return &object.String{Value: t}
	}
	return nil
}

// Live is a diagram's state from its program's values.
type Live struct {
	// Values are the watched names' values, as text (TRUE, 12, T#5S).
	Values map[string]string `json:"values"`
	// Exprs are the drawing's expressions' values: each wire's (data-expr),
	// a block's result (data-val-expr, data-val-exprs).
	Exprs map[string]string `json:"exprs"`
	// Steps are an SFC body's active steps.
	Steps []string `json:"steps,omitempty"`
}

// Watches are the names d's live view reads: the variables its drawing
// shows, those its wires' expressions read, and an SFC body's step flags.
func (d *Diagram) Watches() []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if n != "" && !seen[strings.ToUpper(n)] {
			seen[strings.ToUpper(n)] = true
			out = append(out, n)
		}
	}
	for _, n := range attrValues(d.SVG, "data-var") {
		add(n)
	}
	for _, list := range append(attrValues(d.SVG, "data-vals"), attrValues(d.SVG, "data-prog")...) {
		for _, n := range strings.Fields(list) {
			add(strings.TrimPrefix(n, "="))
		}
	}
	for _, x := range d.exprs() {
		for _, n := range exprNames(x) {
			add(n)
		}
	}
	for _, s := range d.Steps {
		add("_sfc_" + s.Name + "_X")
	}
	return out
}

// exprs are the drawing's expressions.
func (d *Diagram) exprs() []string {
	var out []string
	out = append(out, attrValues(d.SVG, "data-expr")...)
	out = append(out, attrValues(d.SVG, "data-val-expr")...)
	for _, list := range attrValues(d.SVG, "data-val-exprs") {
		out = append(out, strings.Split(list, "|")...)
	}
	return out
}

// Live reads d's watches from v (each name read as prefix+name: "m." for
// a function block's body seen through its instance m) and works out its
// expressions.
func (d *Diagram) Live(v Values, prefix string) (*Live, error) {
	names := d.Watches()
	full := make([]string, len(names))
	for i, n := range names {
		full[i] = prefix + n
	}
	got, err := v.Read(full)
	if err != nil {
		return nil, err
	}
	vals := map[string]object.Object{}
	live := &Live{Values: map[string]string{}, Exprs: map[string]string{}}
	for i, n := range names {
		if o, ok := got[full[i]]; ok && o != nil {
			vals[strings.ToUpper(n)] = o
			live.Values[n] = text(o)
		}
	}
	for _, x := range d.exprs() {
		if _, done := live.Exprs[x]; done {
			continue
		}
		if o := evalExpr(x, vals); o != nil {
			live.Exprs[x] = text(o)
		}
	}
	for _, s := range d.Steps {
		if b, ok := vals[strings.ToUpper("_sfc_"+s.Name+"_X")].(*object.Boolean); ok && b.Value {
			live.Steps = append(live.Steps, s.Name)
		}
	}
	return live, nil
}

// exprNames are the variables expression x reads: names and dotted names,
// not functions called nor keywords.
func exprNames(x string) []string {
	var toks []token.Token
	l := lexer.New(x)
	for t := l.NextToken(); t.Type != token.EOF; t = l.NextToken() {
		toks = append(toks, t)
	}
	var out []string
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.Type != token.IDENT || (i > 0 && (toks[i-1].Type == token.HASH || toks[i-1].Type == token.DOT)) {
			continue
		}
		if i+1 < len(toks) && (toks[i+1].Type == token.LPAREN || toks[i+1].Type == token.HASH) {
			continue
		}
		name := t.Literal
		for i+2 < len(toks) && toks[i+1].Type == token.DOT && isWord(toks[i+2].Literal) {
			name += "." + toks[i+2].Literal
			i += 2
		}
		out = append(out, name)
	}
	return out
}

// evalExpr works out x from vals (by upper-case name); nil if it cannot.
// Each variable is put in a fresh environment under a name of its own, so
// a dotted name needs no instance around it.
func evalExpr(x string, vals map[string]object.Object) (out object.Object) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	names := exprNames(x)
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	env := object.NewEnvironment()
	text := x
	for i, n := range names {
		o, ok := vals[strings.ToUpper(n)]
		if !ok {
			return nil
		}
		slot := "__v" + strconv.Itoa(i)
		env.Set(slot, o)
		text = replaceName(text, n, slot)
	}
	p := parser.New(lexer.New("__r := " + text + ";"))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || len(prog.Statements) != 1 {
		return nil
	}
	a, ok := prog.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		return nil
	}
	o := evaluator.Eval(a.Value, env)
	if o == nil || o.Type() == object.ERROR_OBJ {
		return nil
	}
	return o
}

// replaceName replaces name, whole, in x by with.
func replaceName(x, name, with string) string {
	var b strings.Builder
	for {
		i := indexFold(x, name)
		if i < 0 {
			b.WriteString(x)
			return b.String()
		}
		end := i + len(name)
		whole := (i == 0 || !wordByte(x[i-1]) && x[i-1] != '.') && (end == len(x) || !wordByte(x[end]) && x[end] != '.')
		b.WriteString(x[:i])
		if whole {
			b.WriteString(with)
		} else {
			b.WriteString(x[i:end])
		}
		x = x[end:]
	}
}

func indexFold(s, sub string) int { return strings.Index(strings.ToUpper(s), strings.ToUpper(sub)) }

func wordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// attrValues are the values of attribute name in svg, unescaped, in
// order, without repeats.
func attrValues(svg, name string) []string {
	var out []string
	seen := map[string]bool{}
	key := " " + name + `="`
	for {
		i := strings.Index(svg, key)
		if i < 0 {
			return out
		}
		svg = svg[i+len(key):]
		j := strings.IndexByte(svg, '"')
		if j < 0 {
			return out
		}
		v := unescape(svg[:j])
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
		svg = svg[j:]
	}
}

var unescaper = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&amp;", "&")

func unescape(s string) string { return unescaper.Replace(s) }

// text is o as a value is written: TRUE and FALSE upper case.
func text(o object.Object) string {
	if b, ok := o.(*object.Boolean); ok {
		if b.Value {
			return "TRUE"
		}
		return "FALSE"
	}
	return o.Inspect()
}
