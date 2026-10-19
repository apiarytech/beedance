/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/sil"
	"github.com/apiarytech/beedance/watch"
)

// BreakpointSpec asks for a breakpoint.
type BreakpointSpec struct {
	// Line is the source line, from 1.
	Line int
	// Condition, if not empty, is a Structured Text expression the
	// breakpoint stops only when TRUE, e.g. "n > 3 AND NOT b.q", on the
	// program's variables and the locals of the call it is in.
	Condition string
}

// Breakpoint is a breakpoint as set.
type Breakpoint struct {
	BreakpointSpec
	// Verified is set when the breakpoint can be hit: Actual has a
	// statement the program runs.
	Verified bool
	// Actual is the line it is set on: Line, or the next line with a
	// statement in the same POU.
	Actual int
	// Message says why it is not verified, or what is wrong with its
	// condition.
	Message string
	// Hits counts the times it stopped a run.
	Hits int

	cond ast.Expression
}

// SetBreakpoints replaces the session's breakpoints with specs.
// It may be called while a run is in progress.
func (s *Session) SetBreakpoints(specs []BreakpointSpec) []Breakpoint {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return s.setBreakpoints(specs)
}

func (s *Session) setBreakpoints(specs []BreakpointSpec) []Breakpoint {
	s.bps = map[int]*Breakpoint{}
	out := make([]Breakpoint, 0, len(specs))
	for _, spec := range specs {
		b := &Breakpoint{BreakpointSpec: spec}
		if spec.Condition != "" {
			cond, err := parseExpression(spec.Condition)
			if err != nil {
				b.Message = "condition: " + err.Error()
				out = append(out, *b)
				continue
			}
			b.cond = cond
		}
		// The breakpoints are kept by lines of the source; the client's
		// lines may be offset from them (attach.go).
		line := s.lineIn(spec.Line)
		b.Actual = s.img.statementLine(line)
		if b.Actual == 0 {
			b.Message = s.img.whyNotRun(line)
		} else {
			b.Verified = true
			s.bps[b.Actual] = b
		}
		out = append(out, s.breakpointOut(*b))
	}
	return out
}

// breakpointOut is b with its lines as the client sees them.
func (s *Session) breakpointOut(b Breakpoint) Breakpoint {
	b.Actual = s.lineOut(b.Actual)
	return b
}

// Breakpoints are the breakpoints set, by line.
func (s *Session) Breakpoints() []Breakpoint {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	out := make([]Breakpoint, 0, len(s.bps))
	for _, b := range s.bps {
		out = append(out, s.breakpointOut(*b))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Actual < out[j].Actual })
	return out
}

// whyNotRun says why no statement on line runs: it is in a PROGRAM other
// than the one debugged, or in a plant model, or there is none on it.
func (img *image) whyNotRun(line int) string {
	var in *ast.ProgramDeclaration
	for _, st := range img.program.Statements {
		row, _ := st.Pos()
		if row <= 0 || row > line {
			break
		}
		pd, _ := st.(*ast.ProgramDeclaration)
		in = pd // nil after a POU that is no PROGRAM
	}
	switch {
	case in == nil || in == img.decl || in.Name == nil:
	case IsPlant(in.Name.Value):
		return fmt.Sprintf("%s is a plant model: it runs after each scan, and no breakpoint stops it", in.Name.Value)
	default:
		return fmt.Sprintf("PROGRAM %s does not run in this session: it runs PROGRAM %s. Debug %s instead, or run both from a CONFIGURATION's tasks",
			in.Name.Value, img.name, in.Name.Value)
	}
	return fmt.Sprintf("line %d has no statement the program runs", line)
}

// statementLine is line if a statement the program runs is on it, else the
// next such line before the next POU starts; 0 if there is none.
func (img *image) statementLine(line int) int {
	if line <= 0 {
		return 0
	}
	end := 1 << 30
	for _, r := range img.pouRows {
		if r > line && r < end {
			end = r
		}
	}
	for l := line; l < end && l <= line+1000; l++ {
		if img.rows[l] {
			return l
		}
	}
	return 0
}

// holds reports whether b's condition holds where the run is; a condition
// that cannot be evaluated stops it, with why.
func (s *Session) holds(b *Breakpoint) (bool, string) {
	if b.cond == nil {
		return true, ""
	}
	out := evaluator.Eval(b.cond, s.environment(0))
	switch v := out.(type) {
	case *object.Boolean:
		return v.Value, ""
	case *object.Error:
		return true, "condition: " + v.Message
	}
	return true, fmt.Sprintf("condition: %s is not a BOOL", out.Inspect())
}

// Evaluate evaluates expr, a Structured Text expression, on the program's
// variables and the locals of frame (an index of Frames).
func (s *Session) Evaluate(expr string, frame int) (object.Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := parseExpression(expr)
	if err != nil {
		return nil, err
	}
	out := evaluator.Eval(e, s.environment(frame))
	if er, ok := out.(*object.Error); ok {
		return nil, errors.New(er.Message)
	}
	return out, nil
}

// environment holds the program's variables, then frame's locals and, in
// a function block's body, its instance's members, which hide them.
func (s *Session) environment(frame int) *object.Environment {
	env := object.NewEnvironment()
	for _, v := range s.globals() {
		if v.Object != nil {
			env.Set(v.Name, v.Object)
		}
	}
	for _, v := range s.locals(frame) {
		if v.Object != nil {
			env.Set(v.Name, v.Object)
		}
	}
	return env
}

// parseExpression parses one Structured Text expression.
func parseExpression(text string) (ast.Expression, error) {
	p := parser.New(lexer.New(text + ";"))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	if len(program.Statements) != 1 {
		return nil, errors.New("give one expression")
	}
	es, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok || es.Expression == nil {
		return nil, errors.New("give an expression, not a statement")
	}
	return es.Expression, nil
}

// Value is a variable as a debugger shows it.
type Value struct {
	// Name is the variable's name; a member's is its name in its parent,
	// an element's its index, "[1]".
	Name string
	// Type is the declared type, e.g. "INT" or "TON", where known; else
	// the value's.
	Type string
	// Address is a located variable's, e.g. "%IX0.0".
	Address string
	// Object is the value, in place: nil while the variable has none.
	Object object.Object
	// Forced is set when the variable, or its address, is forced.
	Forced bool
}

// Text is v's value as the source would write it.
func (v Value) Text() string { return Format(v.Object) }

// Globals are the program's variables, in declaration order.
func (s *Session) Globals() []Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.globals()
}

func (s *Session) globals() []Value {
	var out []Value
	for _, v := range s.img.compiled.Variables {
		val := Value{Name: v.Name, Type: v.Type, Address: v.Address}
		if v.Address != "" {
			val.Object = s.img.vm.IO()[v.Address]
		} else if v.Global >= 0 && v.Global < len(s.img.globals) {
			val.Object = s.img.globals[v.Global]
		}
		val.Forced = s.isForced(v.Name, v.Address)
		out = append(out, val)
	}
	return out
}

// Locals are the local variables of frame (an index of Frames): in a
// function block's body or method, its instance's members first. The
// program's body has none: its variables are Globals.
func (s *Session) Locals(frame int) []Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locals(frame)
}

func (s *Session) locals(frame int) []Value {
	fs := s.frames()
	if frame < 0 || frame >= len(fs) || fs[frame].vm.Fn == nil {
		return nil
	}
	f := fs[frame].vm
	var out []Value
	for i, o := range f.Locals {
		name := ""
		if i < len(f.Fn.LocalNames) {
			name = f.Fn.LocalNames[i]
		}
		if name == "" || strings.HasPrefix(name, "__") {
			continue
		}
		if strings.EqualFold(name, "THIS") {
			if h, ok := o.(*object.Hash); ok {
				out = append(Members(h), out...)
			}
			continue
		}
		out = append(out, Value{Name: name, Type: typeName(o), Object: o})
	}
	return out
}

// Watched are the values package watch lists for the program (Options.
// Watch): its elementary variables and, under dotted names, members.
func (s *Session) Watched() []Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watched()
}

func (s *Session) watched() []Value {
	var out []Value
	for _, v := range s.img.vars {
		val := Value{Name: v.Name, Type: v.Type}
		if o, ok := s.lookup(v.Path); ok {
			val.Object = o
		}
		if d, ok := s.img.decls[strings.ToUpper(v.Name)]; ok && d.Address != "" {
			val.Address = d.Address
		}
		val.Forced = s.isForced(v.Name, val.Address)
		out = append(out, val)
	}
	return out
}

// Members are the members of a structure or function block instance, or
// the elements of an array, for a debugger to show under it; nil for an
// elementary value.
func Members(o object.Object) []Value {
	var out []Value
	switch v := o.(type) {
	case *object.Hash:
		for _, p := range v.Pairs {
			name := p.Key.Inspect()
			if strings.HasPrefix(name, "__") {
				continue // the instance's class, and the like
			}
			out = append(out, Value{Name: name, Type: typeName(p.Value), Object: p.Value})
		}
		sort.Slice(out, func(i, j int) bool { return strings.ToUpper(out[i].Name) < strings.ToUpper(out[j].Name) })
	case *object.Array:
		for i, e := range v.Elements {
			out = append(out, Value{Name: "[" + strconv.FormatInt(v.LowerBound+int64(i), 10) + "]", Type: typeName(e), Object: e})
		}
	}
	return out
}

// typeName is the IEC type of o, as far as the value tells.
func typeName(o object.Object) string {
	switch v := o.(type) {
	case nil:
		return ""
	case *object.Boolean:
		return "BOOL"
	case *object.Hash:
		if c, ok := v.Pairs[(&object.String{Value: "__class__"}).HashKey()]; ok {
			if ch, ok := c.Value.(*object.Hash); ok {
				// The class's body is its "main", named after the block.
				if m, ok := ch.Pairs[(&object.String{Value: "main"}).HashKey()]; ok {
					if cl, ok := m.Value.(*object.Closure); ok && cl.Fn.Name != "" {
						return cl.Fn.Name
					}
				}
			}
			return "FUNCTION_BLOCK"
		}
		return "STRUCT"
	case *object.Array:
		return fmt.Sprintf("ARRAY[%d..%d]", v.LowerBound, v.LowerBound+int64(len(v.Elements))-1)
	}
	return string(o.Type())
}

// Format is o as the source would write it: TRUE, 42, 1.5, T#1s500ms,
// 'text'.
func Format(o object.Object) string {
	switch v := o.(type) {
	case nil:
		return "(none)"
	case *object.Boolean:
		if v.Value {
			return "TRUE"
		}
		return "FALSE"
	case *object.String:
		return "'" + v.Value + "'"
	case *object.WString:
		return `"` + v.Value + `"`
	case *object.Time:
		return "T#" + strings.TrimPrefix(strings.TrimPrefix(v.Inspect(), "T#"), "TIME#")
	case *object.Hash, *object.Array:
		return typeName(o)
	}
	return o.Inspect()
}

// lookup is the value at path (a variable's dotted name split), from the
// program's variables.
func (s *Session) lookup(path []string) (object.Object, bool) {
	if len(path) == 0 {
		return nil, false
	}
	d, ok := s.img.decls[strings.ToUpper(path[0])]
	if !ok {
		return nil, false
	}
	var top object.Object
	if d.Address != "" {
		top = s.img.vm.IO()[d.Address]
	} else if d.Global >= 0 && d.Global < len(s.img.globals) {
		top = s.img.globals[d.Global]
	}
	if top == nil {
		return nil, false
	}
	return watch.Member(top, path[1:])
}

// target is where a name or address points: a variable of the program, a
// member of one, or the I/O image.
type target struct {
	key  string   // upper-case name or address
	addr string   // I/O image address, if it is one
	path []string // else the variable's dotted name, split
	typ  string   // the elementary type values are converted to
}

// resolve finds what name, a variable's dotted name or an address
// ("%IX0.0"), points to.
//
// A member's type is its value's, so resolving one reads the program's
// variables; without values (a run is in progress), its type is left
// empty, to be resolved when the target is written.
func (s *Session) resolve(name string) (target, error) { return s.resolveWith(name, true) }

func (s *Session) resolveWith(name string, values bool) (target, error) {
	key := strings.ToUpper(strings.TrimSpace(name))
	if strings.HasPrefix(key, "%") {
		if d, ok := s.img.located[key]; ok {
			return target{key: key, addr: d.Address, typ: d.Type}, nil
		}
		if t, ok := ast.DirectType(key); ok {
			return target{key: key, addr: key, typ: t}, nil
		}
		return target{}, fmt.Errorf("%s is not an address", name)
	}
	path := strings.Split(strings.TrimSpace(name), ".")
	d, ok := s.img.decls[strings.ToUpper(path[0])]
	if !ok {
		return target{}, fmt.Errorf("%s has no variable %s", s.img.name, path[0])
	}
	if d.Constant {
		return target{}, fmt.Errorf("%s is a constant", path[0])
	}
	t := target{key: key, path: path, typ: d.Type}
	if len(path) == 1 && d.Address != "" {
		t.addr = d.Address
	}
	if len(path) > 1 && !values {
		t.typ = ""
	} else if len(path) > 1 {
		cur, ok := s.lookup(path)
		if !ok {
			return target{}, fmt.Errorf("no %s", name)
		}
		t.typ = typeName(cur)
	}
	if t.typ == "BOOLEAN" {
		t.typ = "BOOL"
	}
	return t, nil
}

// parseValue reads text, as the source would write a literal (TRUE, 42,
// 16#FF, 1.5, T#2s, 'text'), as a value of type typ.
func parseValue(typ, text string) (object.Object, error) {
	text = strings.TrimSpace(text)
	upper := strings.ToUpper(typ)
	if i := strings.Index(text, "#"); i > 0 && !strings.ContainsAny(text[:i], "0123456789") &&
		upper != "TIME" {
		text = text[i+1:] // INT#5, BOOL#TRUE
	}
	var v any
	switch upper {
	case "BOOL":
		switch strings.ToUpper(text) {
		case "TRUE", "1":
			v = true
		case "FALSE", "0":
			v = false
		default:
			return nil, fmt.Errorf("%s is not a BOOL", text)
		}
	case "STRING", "WSTRING":
		if len(text) >= 2 && (text[0] == '\'' || text[0] == '"') && text[len(text)-1] == text[0] {
			text = text[1 : len(text)-1]
		}
		v = text
	case "TIME":
		v = text
	case "REAL", "LREAL":
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is not a %s", text, upper)
		}
		v = f
	default:
		t := strings.ReplaceAll(text, "_", "")
		base := 10
		if b, digits, ok := strings.Cut(t, "#"); ok {
			n, err := strconv.Atoi(b)
			if err != nil {
				return nil, fmt.Errorf("%s is not a %s", text, upper)
			}
			base, t = n, digits
		}
		if i, err := strconv.ParseInt(t, base, 64); err == nil {
			v = i
		} else if u, err := strconv.ParseUint(t, base, 64); err == nil {
			v = u
		} else {
			return nil, fmt.Errorf("%s is not a %s", text, upper)
		}
	}
	return sil.ToObject(upper, v)
}

// write puts o where t points.
func (s *Session) write(t target, o object.Object) error {
	if t.addr != "" {
		s.img.vm.IO()[t.addr] = o
		return nil
	}
	d := s.img.decls[strings.ToUpper(t.path[0])]
	if len(t.path) == 1 {
		if d.Global < 0 || d.Global >= len(s.img.globals) {
			return fmt.Errorf("%s has no slot", t.path[0])
		}
		s.img.globals[d.Global] = o
		return nil
	}
	parent, ok := s.lookup(t.path[:len(t.path)-1])
	if !ok {
		return fmt.Errorf("no %s", strings.Join(t.path[:len(t.path)-1], "."))
	}
	h, ok := parent.(*object.Hash)
	if !ok {
		return fmt.Errorf("%s has no members", strings.Join(t.path[:len(t.path)-1], "."))
	}
	member := t.path[len(t.path)-1]
	for k, p := range h.Pairs {
		if strings.EqualFold(p.Key.Inspect(), member) {
			h.Pairs[k] = object.HashPair{Key: p.Key, Value: o}
			return nil
		}
	}
	return fmt.Errorf("no %s", strings.Join(t.path, "."))
}

// write is a value to put somewhere when the next scan starts.
type write struct{ name, value string }

// Set writes value (as the source would write a literal) to name, a
// variable, a member ("b.PT") or an address ("%IX0.0"), once: the program
// may change it again. Stopped, it is written at once; while a run is in
// progress, when the next scan starts (Options.Notify hears if it fails).
func (s *Session) Set(name, value string) error {
	if !s.mu.TryLock() {
		s.cmu.Lock()
		defer s.cmu.Unlock()
		if _, err := s.resolveWith(name, false); err != nil {
			return err
		}
		s.pending = append(s.pending, write{name, value})
		return nil
	}
	defer s.mu.Unlock()
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return s.set(name, value)
}

func (s *Session) set(name, value string) error {
	t, err := s.resolve(name)
	if err != nil {
		return err
	}
	o, err := parseValue(t.typ, value)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return s.write(t, o)
}

// applyPending makes the writes Set queued while a run was in progress.
func (s *Session) applyPending() {
	s.cmu.Lock()
	todo := s.pending
	s.pending = nil
	var failed []string
	for _, w := range todo {
		if err := s.set(w.name, w.value); err != nil {
			failed = append(failed, "set "+err.Error())
		}
	}
	s.cmu.Unlock()
	for _, msg := range failed {
		s.notify(msg)
	}
}

func (s *Session) notify(msg string) {
	if s.opts.Notify != nil {
		s.opts.Notify(msg)
	}
}

// force is a value kept on a variable or address.
type force struct {
	name  string // as given
	t     target // its type is empty until it is resolved with values
	value string
}

// Force keeps value on name (as Set takes them) until Unforce: it is
// written before every scan (so the program reads it, as a forced input)
// and after every scan (so the outputs and observers see it, as a forced
// output); stopped, also at once. While a run is in progress, a force of
// a member is checked when it is first applied (Options.Notify hears if it
// fails).
func (s *Session) Force(name, value string) error {
	if !s.mu.TryLock() {
		s.cmu.Lock()
		defer s.cmu.Unlock()
		t, err := s.resolveWith(name, false)
		if err != nil {
			return err
		}
		if t.typ != "" {
			if _, err := parseValue(t.typ, value); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		s.forces[t.key] = &force{name: strings.TrimSpace(name), t: t, value: value}
		return nil
	}
	defer s.mu.Unlock()
	s.cmu.Lock()
	defer s.cmu.Unlock()
	t, err := s.resolve(name)
	if err != nil {
		return err
	}
	o, err := parseValue(t.typ, value)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := s.write(t, o); err != nil {
		return err
	}
	s.forces[t.key] = &force{name: strings.TrimSpace(name), t: t, value: value}
	return nil
}

// Unforce ends the force on name; its value stays until the program
// changes it. It may be called while a run is in progress.
func (s *Session) Unforce(name string) error {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	key := strings.ToUpper(strings.TrimSpace(name))
	if s.forces[key] == nil {
		return fmt.Errorf("%s is not forced", name)
	}
	delete(s.forces, key)
	return nil
}

// Forced are the forces, name and value, by name.
func (s *Session) Forced() map[string]string {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	out := map[string]string{}
	for _, f := range s.forces {
		out[f.name] = f.value
	}
	return out
}

// isForced reports whether the variable name, or its address addr, is
// forced.
func (s *Session) isForced(name, addr string) bool {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return s.forces[strings.ToUpper(name)] != nil || addr != "" && s.forces[strings.ToUpper(addr)] != nil
}

// applyForces writes every force; each gets a value of its own, so the
// program cannot change one through another. A force made while a run was
// in progress is resolved now; one that fails is dropped.
func (s *Session) applyForces() {
	s.cmu.Lock()
	var failed []string
	for key, f := range s.forces {
		if f.t.typ == "" {
			t, err := s.resolve(f.name)
			if err == nil {
				_, err = parseValue(t.typ, f.value)
			}
			if err != nil {
				delete(s.forces, key)
				failed = append(failed, fmt.Sprintf("force %s dropped: %v", f.name, err))
				continue
			}
			f.t = t
		}
		if o, err := parseValue(f.t.typ, f.value); err == nil {
			_ = s.write(f.t, o)
		}
	}
	s.cmu.Unlock()
	for _, msg := range failed {
		s.notify(msg)
	}
}

// Sample is the watched values, and the I/O image, as a scan left them.
// As JSON, Elapsed is "elapsedMs", in milliseconds.
type Sample struct {
	Scan    int           `json:"scan"`
	Elapsed time.Duration `json:"-"`
	Values  []Sampled     `json:"values"`
	// IO is the I/O image (see Session.IO).
	IO map[string]any `json:"io"`
	// Extra are the values of the names SetSampled asked for, as text.
	Extra map[string]string `json:"extra,omitempty"`
}

// MarshalJSON writes s with its Elapsed in milliseconds ("elapsedMs").
func (s Sample) MarshalJSON() ([]byte, error) {
	type plain Sample
	return json.Marshal(struct {
		plain
		ElapsedMs float64 `json:"elapsedMs"`
	}{plain(s), float64(s.Elapsed) / float64(time.Millisecond)})
}

// Sampled is a watched value in a Sample.
type Sampled struct {
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Address string `json:"address,omitempty"`
	Text    string `json:"text"`
	Forced  bool   `json:"forced,omitempty"`
}

// ExtraEvery is how often a run reads the names SetSampled asked for: the
// Sample of a scan in between keeps the last values read.
const ExtraEvery = 25 * time.Millisecond

// Live is the watched values (Watched) at the end of the last scan, which
// may be read while a run is in progress: what a PLC's online view shows.
// The first call starts the sampling; it returns the values as they are
// if no run is in progress, else an empty Sample.
func (s *Session) Live() Sample {
	if !s.liveWanted.Swap(true) && s.mu.TryLock() {
		s.sample()
		s.mu.Unlock()
	}
	s.lmu.Lock()
	defer s.lmu.Unlock()
	return s.live
}

// sample keeps the watched values for Live, once it has been called.
func (s *Session) sample() {
	if !s.liveWanted.Load() {
		return
	}
	smp := Sample{Scan: s.scans, Elapsed: s.elapsed, IO: s.io()}
	if names := s.sampled.Load(); names != nil && len(*names) > 0 {
		// At most every ExtraEvery: a view of values (a monitor, a
		// diagram) does not need each of a fast run's scans, and reading
		// many names a scan would slow it.
		if s.extra == nil || names != s.extraFor || time.Since(s.extraAt) >= ExtraEvery {
			s.extra, s.extraFor, s.extraAt = s.texts(*names), names, time.Now()
		}
		smp.Extra = s.extra
	}
	for _, v := range s.watched() {
		smp.Values = append(smp.Values, Sampled{Name: v.Name, Type: v.Type, Address: v.Address, Text: v.Text(), Forced: v.Forced})
	}
	s.lmu.Lock()
	s.live = smp
	s.lmu.Unlock()
	if s.opts.Sampled != nil {
		s.opts.Sampled(smp)
	}
}

// ErrRunning is returned by what may not be done while a run is in
// progress.
var ErrRunning = errors.New("vmdebug: the program is running; pause it first")

// SetIn writes value into the member name of container, a structure or a
// function block instance, or into its element name ("[2]"), if it is an
// array: container is a Value's Object (Globals, Locals, Members). It
// returns the value written.
func (s *Session) SetIn(container object.Object, name, value string) (object.Object, error) {
	if !s.mu.TryLock() {
		return nil, ErrRunning
	}
	defer s.mu.Unlock()
	return setIn(container, name, value)
}

func setIn(container object.Object, name, value string) (object.Object, error) {
	switch c := container.(type) {
	case *object.Hash:
		for k, p := range c.Pairs {
			if strings.EqualFold(p.Key.Inspect(), name) {
				o, err := parseValue(valueType(p.Value), value)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", name, err)
				}
				c.Pairs[k] = object.HashPair{Key: p.Key, Value: o}
				return o, nil
			}
		}
		return nil, fmt.Errorf("no member %s", name)
	case *object.Array:
		i, err := strconv.ParseInt(strings.Trim(name, "[] "), 10, 64)
		if err != nil || i < c.LowerBound || i >= c.LowerBound+int64(len(c.Elements)) {
			return nil, fmt.Errorf("no element %s", name)
		}
		o, err := parseValue(valueType(c.Elements[i-c.LowerBound]), value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		c.Elements[i-c.LowerBound] = o
		return o, nil
	}
	return nil, fmt.Errorf("%s has no members", Format(container))
}

// SetLocal writes value to the local variable name of frame (an index of
// Frames), or to the member of the function block instance whose body it
// is; it returns the value written.
func (s *Session) SetLocal(frame int, name, value string) (object.Object, error) {
	if !s.mu.TryLock() {
		return nil, ErrRunning
	}
	defer s.mu.Unlock()
	fs := s.frames()
	if frame < 0 || frame >= len(fs) || fs[frame].vm.Fn == nil {
		return nil, fmt.Errorf("frame %d has no locals", frame)
	}
	f := fs[frame].vm
	for i, o := range f.Locals {
		local := ""
		if i < len(f.Fn.LocalNames) {
			local = f.Fn.LocalNames[i]
		}
		if strings.EqualFold(local, "THIS") {
			if h, ok := o.(*object.Hash); ok {
				if v, err := setIn(h, name, value); err == nil {
					return v, nil
				}
			}
			continue
		}
		if strings.EqualFold(local, name) {
			v, err := parseValue(valueType(o), value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			f.Locals[i] = v
			return v, nil
		}
	}
	return nil, fmt.Errorf("no local %s", name)
}

// valueType is the elementary type values written over o take.
func valueType(o object.Object) string {
	if t := typeName(o); t != "BOOLEAN" {
		return t
	}
	return "BOOL"
}
