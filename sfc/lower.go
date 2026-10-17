/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package sfc lowers a Sequential Function Chart to Structured Text, so a
// chart runs wherever ST runs (the bytecode compiler and VM), as package
// diagram does for LD and FBD bodies.
//
// The lowered body follows the evaluator's cycle, scan by scan:
//
//  1. Each action's control: the qualifier of highest precedence among its
//     active steps (R, then S, then the last other one), with the timers
//     and counters of N, S, R, P (and P1), D, L, SD, DS and SL; then P0, a
//     pulse when its step has just become inactive.
//  2. Each action's output (a BOOL of the action's name, when the POU has
//     one) and, when active, its body.
//  3. Each transition whose steps are all active, its condition.
//  4. The fired transitions move their steps.
//  5. The actions of the steps that moved are evaluated again, so a new
//     step acts in the scan it became active.
//
// Each step is a BOOL _sfc_<step>_X, TRUE for the initial step, and a TON
// _sfc_<step>_E whose ET is its elapsed time; each action
// keeps its state in _sfc_a<n>_Q, _qual, _cnt, _active, _t and _d (IEC names
// are case-insensitive, so no two differ by case alone); a timed qualifier measures
// time with a TON. Actions run in the order they are first named, steps
// and transitions in the order they are written.
package sfc

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// Qualifier codes, ordered so that a code below qS has no precedence.
const (
	qNone = iota
	qN
	qP
	qD
	qL
	qSD
	qDS
	qSL
	qS
	qR
	// qP0 pulses when its step becomes inactive: outside the precedence,
	// worked out apart (control).
	qP0
)

// qualifiers by name; P1 (IEC 61131-3 3rd edition) is P, a pulse when the
// step becomes active.
var qualifiers = map[string]int{"N": qN, "P": qP, "P1": qP, "P0": qP0, "D": qD, "L": qL, "SD": qSD, "DS": qDS, "SL": qSL, "S": qS, "R": qR}

// forever is the preset of the TON that measures an action's time: longer
// than any qualifier's duration.
const forever = "T#24D"

type assoc struct {
	step     string
	code     int
	duration ast.Expression // a timed qualifier's time
}

type action struct {
	name   string
	index  int
	body   *ast.BlockStatement // nil: an action without a body
	assocs []assoc
	output bool // the POU has a BOOL of the action's name
}

type transition struct {
	from, to  []string
	condition ast.Expression
	priority  int64 // (PRIORITY := n), lower first; the largest when not given
}

// Program returns program with every PROGRAM and FUNCTION_BLOCK whose body
// is a chart lowered to ST. program itself is not changed; a POU without a
// chart is shared with it.
func Program(program *ast.Program) (*ast.Program, error) {
	out := *program
	out.Statements = make([]ast.Statement, len(program.Statements))
	needFlags := false
	for i, s := range program.Statements {
		out.Statements[i] = s
		switch d := s.(type) {
		case *ast.ProgramDeclaration:
			chart, ok := d.Body.(*ast.SFCProgram)
			if !ok {
				continue
			}
			vars, body, err := Lower(d.Name.Value, chart, declared(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars))
			if err != nil {
				return nil, err
			}
			low := *d
			low.Vars = append(append([]*ast.VarDeclStatement{}, d.Vars...), vars...)
			low.Body = body
			out.Statements[i] = &low
			needFlags = needFlags || declaresFlags(vars)
		case *ast.FunctionBlockDeclaration:
			chart, ok := d.Body.(*ast.SFCProgram)
			if !ok {
				continue
			}
			vars, body, err := Lower(d.Name.Value, chart, declared(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars))
			if err != nil {
				return nil, err
			}
			low := *d
			low.Vars = append(append([]*ast.VarDeclStatement{}, d.Vars...), vars...)
			low.Body = body
			out.Statements[i] = &low
			needFlags = needFlags || declaresFlags(vars)
		}
	}
	if needFlags {
		p := parser.New(lexer.New(stepFlagsDecl))
		types := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			return nil, fmt.Errorf("the step flags type does not parse (a beedance defect): %s", errs[0])
		}
		out.Statements = append(types.Statements, out.Statements...)
	}
	return &out, nil
}

// declared returns the BOOL variables of a POU, upper case, by name.
func declared(blocks ...[]*ast.VarDeclStatement) map[string]bool {
	out := map[string]bool{}
	for _, b := range blocks {
		for _, d := range b {
			if d.Name != nil && d.DataType != nil && strings.EqualFold(d.DataType.String(), "BOOL") {
				out[strings.ToUpper(d.Name.Value)] = true
			}
		}
	}
	return out
}

// Lower returns a chart as the variables it needs and an ST body. bools are
// the POU's BOOL variables (upper case): an action of one of those names
// sets it to whether the action is active.
func Lower(pou string, chart *ast.SFCProgram, bools map[string]bool) ([]*ast.VarDeclStatement, *ast.BlockStatement, error) {
	var (
		actions []*action
		byName  = map[string]*action{}
		steps   []string
		initial string
		trans   []transition
	)
	get := func(name string) *action {
		key := strings.ToUpper(name)
		if a, ok := byName[key]; ok {
			return a
		}
		a := &action{name: name, index: len(actions), output: bools[key]}
		byName[key] = a
		actions = append(actions, a)
		return a
	}
	stepSet := map[string]bool{}
	for _, el := range chart.Elements {
		switch e := el.(type) {
		case *ast.ActionStatement:
			a := get(e.Name.Value)
			switch b := e.Body.(type) {
			case *ast.BlockStatement:
				a.body = b
			case nil:
			default:
				a.body = &ast.BlockStatement{Statements: []ast.Statement{b}}
			}
		case *ast.StepStatement:
			if e == nil || e.Name == nil {
				return nil, nil, fmt.Errorf("%s: a step without a name", pou)
			}
			name := e.Name.Value
			if stepSet[strings.ToUpper(name)] {
				return nil, nil, fmt.Errorf("%s: step %s defined twice", pou, name)
			}
			stepSet[strings.ToUpper(name)] = true
			steps = append(steps, name)
			if e.IsInitial {
				if initial != "" {
					return nil, nil, fmt.Errorf("%s: two initial steps, %s and %s", pou, initial, name)
				}
				initial = name
			}
			if e.Body != nil && len(e.Body.Statements) > 0 {
				return nil, nil, fmt.Errorf("%s: step %s has statements of its own; put them in an action", pou, name)
			}
			for _, ab := range e.Actions {
				code := qN
				if ab.Qualifier != nil {
					c, ok := qualifiers[strings.ToUpper(ab.Qualifier.Value)]
					if !ok {
						return nil, nil, fmt.Errorf("%s: step %s: qualifier %s", pou, name, ab.Qualifier.Value)
					}
					code = c
				}
				timed := code == qD || code == qL || code == qSD || code == qDS || code == qSL
				if timed && ab.Duration == nil {
					return nil, nil, fmt.Errorf("%s: step %s: action %s: qualifier %s needs a time", pou, name, ab.ActionName.Value, ab.Qualifier.Value)
				}
				a := get(ab.ActionName.Value)
				var d ast.Expression
				if timed {
					d = ab.Duration
				}
				a.assocs = append(a.assocs, assoc{step: name, code: code, duration: d})
			}
		case *ast.TransitionStatement:
			t := transition{condition: e.Condition, priority: math.MaxInt64}
			if p, ok := e.PriorityValue(); ok {
				t.priority = p
			}
			for _, s := range e.From {
				t.from = append(t.from, s.Value)
			}
			for _, s := range e.To {
				t.to = append(t.to, s.Value)
			}
			trans = append(trans, t)
		}
	}
	// Transitions with a priority come first, lower first; the others keep
	// their written order.
	sort.SliceStable(trans, func(i, j int) bool { return trans[i].priority < trans[j].priority })
	if initial == "" {
		return nil, nil, fmt.Errorf("%s: SFC program has no initial step", pou)
	}
	for _, t := range trans {
		for _, s := range append(append([]string{}, t.from...), t.to...) {
			if !stepSet[strings.ToUpper(s)] {
				return nil, nil, fmt.Errorf("%s: a transition names step %s, which is not defined", pou, s)
			}
		}
	}

	g := &gen{}
	// Placeholders stand for the chart's own expressions and bodies, which
	// are put back as they were parsed, never printed and parsed again.
	var decls []string
	for _, s := range steps {
		decls = append(decls, fmt.Sprintf("%s : BOOL := %s;", stepVar(s), map[bool]string{true: "TRUE", false: "FALSE"}[strings.EqualFold(s, initial)]))
		decls = append(decls, stepTimer(s)+" : TON;")
	}
	decls = append(decls, "_sfc_any : BOOL;", "_sfc_qq : INT;")
	for _, a := range actions {
		p := a.prefix()
		decls = append(decls, p+"Q : BOOL;", p+"qual : INT;", p+"cnt : INT;", p+"active : BOOL;", p+"t : TON;", p+"d : TIME;")
		if a.hasP0() {
			decls = append(decls, p+"w0 : BOOL;", p+"p0 : BOOL;")
		}
	}
	for i := range trans {
		decls = append(decls, fmt.Sprintf("_sfc_f%d : BOOL;", i))
	}
	// A step whose flags the chart reads, S1.X or S1.T, is a variable of
	// that name holding them (stepFlagsType).
	flagged := flaggedSteps(chart, steps)
	for _, s := range flagged {
		decls = append(decls, fmt.Sprintf("%s : %s := (X := %s);", s, stepFlagsType, map[bool]string{true: "TRUE", false: "FALSE"}[strings.EqualFold(s, initial)]))
	}
	// So is an action whose flags it reads, A.Q or A.A (actionFlagsType),
	// unless the POU has a BOOL of the action's name.
	g.actionFlags = map[*action]bool{}
	for _, a := range flaggedActions(chart, actions) {
		if !a.output {
			g.actionFlags[a] = true
			decls = append(decls, fmt.Sprintf("%s : %s;", a.name, actionFlagsType))
		}
	}

	// A step's time, as its actions and transitions read it this scan: the
	// time since it became active (the evaluator's now - activation).
	for _, s := range flagged {
		g.line("%s(IN := %s, PT := %s);", stepTimer(s), stepVar(s), forever)
		g.line("%s.T := %s.ET;", s, stepTimer(s))
	}
	// 1, 2: control, then output and body.
	for _, a := range actions {
		g.control(a)
		g.syncAction(a)
	}
	for _, a := range actions {
		g.effect(a)
	}
	// 3: the transitions that fire, on the steps as they were.
	for i, t := range trans {
		g.line("_sfc_f%d := FALSE;", i)
		g.line("IF %s THEN", allActive(t.from))
		g.line("_sfc_f%d := %s;", i, g.expr(t.condition))
		g.line("END_IF;")
	}
	// 4: they move their steps.
	for i, t := range trans {
		g.line("IF _sfc_f%d THEN", i)
		for _, s := range t.from {
			g.line("%s := FALSE;", stepVar(s))
		}
		for _, s := range t.to {
			g.line("%s := TRUE;", stepVar(s))
		}
		g.line("END_IF;")
	}
	// The flags of the steps that moved, for the actions run again.
	for _, s := range flagged {
		g.line("%s.X := %s;", s, stepVar(s))
	}
	// 5: the actions of the steps that moved, again.
	for _, a := range actions {
		var fired []string
		for i, t := range trans {
			if a.touches(t) {
				fired = append(fired, fmt.Sprintf("_sfc_f%d", i))
			}
		}
		if len(fired) == 0 {
			continue
		}
		g.line("IF %s THEN", strings.Join(fired, " OR "))
		g.control(a)
		g.syncAction(a)
		g.effect(a)
		g.line("END_IF;")
	}

	// 6: each step's elapsed time (IEC 61131-3's <step>.T), counted while
	// it is active and back to zero when it is not, for pages to show.
	for _, s := range steps {
		g.line("%s(IN := %s, PT := %s);", stepTimer(s), stepVar(s), forever)
	}

	src := "PROGRAM _sfc VAR " + strings.Join(decls, " ") + " END_VAR\n" + g.b.String() + "END_PROGRAM\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, nil, fmt.Errorf("%s: the chart lowered to ST that does not parse (a beedance defect): %s", pou, errs[0])
	}
	pd, ok := prog.Statements[0].(*ast.ProgramDeclaration)
	if !ok || len(prog.Statements) != 1 {
		return nil, nil, fmt.Errorf("%s: the chart did not lower to one block (a beedance defect)", pou)
	}
	body, ok := pd.Body.(*ast.BlockStatement)
	if !ok {
		return nil, nil, fmt.Errorf("%s: the chart did not lower to statements (a beedance defect)", pou)
	}
	if err := g.fill(body); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", pou, err)
	}
	return pd.Vars, body, nil
}

func stepVar(step string) string { return "_sfc_" + step + "_X" }

// stepFlagsType is the structure of a step's flags, IEC 61131-3's
// <step>.X (active) and <step>.T (time active), declared by Program for a
// chart that reads them.
const stepFlagsType = "_SFC_STEP_FLAGS"

// actionFlagsType is the structure of an action's flags, <action>.Q (active)
// and <action>.A (executing).
const actionFlagsType = "_SFC_ACTION_FLAGS"

// stepFlagsDecl declares stepFlagsType and actionFlagsType.
const stepFlagsDecl = "TYPE " + stepFlagsType + " : STRUCT X : BOOL; T : TIME; END_STRUCT; END_TYPE\n" +
	"TYPE " + actionFlagsType + " : STRUCT Q : BOOL; A : BOOL; END_STRUCT; END_TYPE"

// flaggedSteps returns the steps whose flags, <step>.X or <step>.T, the
// chart's actions or transitions read.
func flaggedSteps(chart *ast.SFCProgram, steps []string) []string {
	byName := map[string]string{}
	for _, s := range steps {
		byName[strings.ToUpper(s)] = s
	}
	used := map[string]bool{}
	visit := func(n ast.Node) {
		if n == nil {
			return
		}
		ast.Modify(n, func(x ast.Node) ast.Node {
			if m, ok := x.(*ast.MemberAccessExpression); ok && m.Member != nil {
				if id, ok := m.Struct.(*ast.Identifier); ok {
					member := strings.ToUpper(m.Member.Value)
					if s, isStep := byName[strings.ToUpper(id.Value)]; isStep && (member == "X" || member == "T") {
						used[s] = true
					}
				}
			}
			return x
		})
	}
	for _, el := range chart.Elements {
		switch e := el.(type) {
		case *ast.ActionStatement:
			if e.Body != nil {
				visit(e.Body)
			}
		case *ast.TransitionStatement:
			if e.Condition != nil {
				visit(e.Condition)
			}
		}
	}
	var out []string
	for _, s := range steps {
		if used[s] {
			out = append(out, s)
		}
	}
	return out
}

// stepTimer is the TON whose ET is a step's elapsed time.
func stepTimer(step string) string { return "_sfc_" + step + "_E" }

func (a *action) prefix() string { return fmt.Sprintf("_sfc_a%d_", a.index) }

// touches says whether t moves a step a is associated with.
func (a *action) touches(t transition) bool {
	for _, as := range a.assocs {
		for _, s := range append(append([]string{}, t.from...), t.to...) {
			if strings.EqualFold(as.step, s) {
				return true
			}
		}
	}
	return false
}

func allActive(steps []string) string {
	vars := make([]string, len(steps))
	for i, s := range steps {
		vars[i] = stepVar(s)
	}
	return strings.Join(vars, " AND ")
}

// gen writes the lowered body and keeps the placeholders it used.
type gen struct {
	b           strings.Builder
	exprs       []ast.Expression
	bodies      []*ast.BlockStatement
	actionFlags map[*action]bool // actions whose flags are a variable (actionFlagsType)
}

// syncAction sets the flags of an action whose flags the chart reads to its
// control, once worked out: Q, and A, which is Q as an action has no final
// scan after it ends.
func (g *gen) syncAction(a *action) {
	if g.actionFlags[a] {
		g.line("%s.Q := %sQ; %s.A := %sQ;", a.name, a.prefix(), a.name, a.prefix())
	}
}

func (g *gen) line(format string, args ...any) {
	fmt.Fprintf(&g.b, format, args...)
	g.b.WriteByte('\n')
}

// expr returns a placeholder for e: a name no program may use.
func (g *gen) expr(e ast.Expression) string {
	g.exprs = append(g.exprs, e)
	return fmt.Sprintf("_SFC_EXPR_%d", len(g.exprs)-1)
}

// control writes an action's control logic: the evaluator's evaluateAction.
func (g *gen) control(a *action) {
	p := a.prefix()
	start := fmt.Sprintf("%st(IN := TRUE, PT := %s);", p, forever)
	reset := fmt.Sprintf("%sactive := FALSE; %st(IN := FALSE);", p, p)
	g.line("_sfc_any := FALSE; _sfc_qq := %d;", qNone)
	for _, as := range a.assocs {
		if as.code == qP0 {
			continue // worked out below, on its step going inactive
		}
		g.line("IF %s THEN", stepVar(as.step))
		g.line("_sfc_any := TRUE;")
		if as.duration != nil {
			g.line("%sd := %s;", p, g.expr(as.duration))
		}
		switch {
		case as.code == qR:
			g.line("_sfc_qq := %d;", qR)
		default: // S over the others, the last of the others over the earlier
			g.line("IF _sfc_qq < %d THEN _sfc_qq := %d; END_IF;", qS, as.code)
		}
		g.line("END_IF;")
	}
	g.line("IF _sfc_any THEN")
	g.line("IF _sfc_qq <> %squal THEN %s %scnt := 0; END_IF;", p, reset, p)
	g.line("%squal := _sfc_qq;", p)
	g.line("IF _sfc_qq = %d OR _sfc_qq = %d THEN %sQ := TRUE;", qN, qS, p)
	g.line("ELSIF _sfc_qq = %d THEN %sQ := FALSE; %s", qR, p, reset)
	g.line("ELSIF _sfc_qq = %d THEN %sQ := %scnt = 0; %scnt := %scnt + 1;", qP, p, p, p, p)
	g.line("ELSIF _sfc_qq = %d OR _sfc_qq = %d THEN", qD, qL)
	g.line("IF NOT %sactive THEN %sactive := TRUE; %st(IN := FALSE); END_IF;", p, p, p)
	g.line("%s", start)
	g.line("IF _sfc_qq = %d THEN %sQ := %st.ET >= %sd; ELSE %sQ := %st.ET < %sd; END_IF;", qD, p, p, p, p, p, p)
	g.line("ELSIF _sfc_qq = %d OR _sfc_qq = %d THEN", qSD, qDS)
	g.line("IF NOT %sactive THEN %sactive := TRUE; %st(IN := FALSE); %s %sQ := FALSE;", p, p, p, start, p)
	g.line("ELSIF NOT %sQ THEN %s IF %st.ET >= %sd THEN %sQ := TRUE; END_IF;", p, start, p, p, p)
	g.line("END_IF;")
	g.line("ELSIF _sfc_qq = %d THEN", qSL)
	g.line("IF NOT %sactive THEN %sactive := TRUE; %st(IN := FALSE); %s %sQ := TRUE;", p, p, p, start, p)
	g.line("ELSE %s IF %st.ET >= %sd THEN %sQ := FALSE; END_IF;", start, p, p, p)
	g.line("END_IF;")
	g.line("END_IF;")
	g.line("ELSE") // no active step: what the stored qualifiers keep
	g.line("IF %squal = %d THEN", p, qS)
	g.line("%sQ := %sQ;", p, p)
	g.line("ELSIF %squal = %d THEN", p, qSD) // stored at once: comes on when the time is up
	g.line("IF NOT %sQ AND %sactive THEN %s IF %st.ET >= %sd THEN %sQ := TRUE; END_IF; END_IF;", p, p, start, p, p, p)
	g.line("ELSIF %squal = %d THEN", p, qDS) // stored only once the time was up: cancelled if not yet
	g.line("IF NOT %sQ THEN %s END_IF;", p, reset)
	g.line("ELSIF %squal = %d THEN", p, qSL)
	g.line("IF %sQ AND %sactive THEN %s IF %st.ET >= %sd THEN %sQ := FALSE; END_IF; END_IF;", p, p, start, p, p, p)
	g.line("ELSE")
	g.line("%sQ := FALSE;", p)
	g.line("%scnt := 0;", p) // a P action pulses again on its step's next activation
	g.line("IF %squal = %d OR %squal = %d THEN %s END_IF;", p, qD, p, qL, reset)
	g.line("END_IF;")
	g.line("END_IF;")
	// P0: active for one scan when its step has just become inactive (the
	// steps it is P0 on were active when last seen, and none is now).
	if a.hasP0() {
		var now []string
		for _, as := range a.assocs {
			if as.code == qP0 {
				now = append(now, stepVar(as.step))
			}
		}
		g.line("_sfc_any := %s;", strings.Join(now, " OR "))
		g.line("%sp0 := %sw0 AND NOT _sfc_any; %sw0 := _sfc_any;", p, p, p)
		g.line("IF %sp0 THEN %sQ := TRUE; END_IF;", p, p)
	}
}

// hasP0 says whether a is P0 on any step.
func (a *action) hasP0() bool {
	for _, as := range a.assocs {
		if as.code == qP0 {
			return true
		}
	}
	return false
}

// effect writes an action's output and, while it is active, its body.
func (g *gen) effect(a *action) {
	p := a.prefix()
	if a.output {
		g.line("%s := %sQ;", a.name, p)
	}
	if a.body == nil || len(a.body.Statements) == 0 {
		return
	}
	g.bodies = append(g.bodies, a.body)
	g.line("IF %sQ THEN", p)
	g.line("_SFC_BODY_%d := TRUE;", len(g.bodies)-1)
	g.line("END_IF;")
}

// fill puts the chart's expressions and bodies in place of the
// placeholders.
func (g *gen) fill(b *ast.BlockStatement) error {
	var out []ast.Statement
	for _, s := range b.Statements {
		switch st := s.(type) {
		case *ast.AssignmentStatement:
			if id, ok := st.Left.(*ast.Identifier); ok && strings.HasPrefix(id.Value, "_SFC_BODY_") {
				var n int
				fmt.Sscanf(id.Value, "_SFC_BODY_%d", &n)
				out = append(out, g.bodies[n].Statements...)
				continue
			}
			if id, ok := st.Value.(*ast.Identifier); ok && strings.HasPrefix(id.Value, "_SFC_EXPR_") {
				var n int
				fmt.Sscanf(id.Value, "_SFC_EXPR_%d", &n)
				st.Value = g.exprs[n]
			}
		case *ast.IfStatement:
			if err := g.fillIf(st); err != nil {
				return err
			}
		}
		out = append(out, s)
	}
	b.Statements = out
	return nil
}

func (g *gen) fillIf(st *ast.IfStatement) error {
	if st.Consequence != nil {
		if err := g.fill(st.Consequence); err != nil {
			return err
		}
	}
	switch alt := st.Alternative.(type) {
	case *ast.IfStatement:
		return g.fillIf(alt)
	case *ast.BlockStatement:
		return g.fill(alt)
	}
	return nil
}

// declaresFlags reports whether a lowered chart's variables include a step's
// flags, so that the program needs stepFlagsType.
func declaresFlags(vars []*ast.VarDeclStatement) bool {
	for _, v := range vars {
		if v.DataType != nil && (strings.EqualFold(v.DataType.String(), stepFlagsType) || strings.EqualFold(v.DataType.String(), actionFlagsType)) {
			return true
		}
	}
	return false
}

// flaggedActions returns the actions whose flags, <action>.Q or <action>.A,
// the chart's actions or transitions read.
func flaggedActions(chart *ast.SFCProgram, actions []*action) []*action {
	byName := map[string]*action{}
	for _, a := range actions {
		byName[strings.ToUpper(a.name)] = a
	}
	used := map[*action]bool{}
	visit := func(n ast.Node) {
		ast.Modify(n, func(x ast.Node) ast.Node {
			if m, ok := x.(*ast.MemberAccessExpression); ok && m.Member != nil {
				if id, ok := m.Struct.(*ast.Identifier); ok {
					member := strings.ToUpper(m.Member.Value)
					if a, isAction := byName[strings.ToUpper(id.Value)]; isAction && (member == "Q" || member == "A") {
						used[a] = true
					}
				}
			}
			return x
		})
	}
	for _, el := range chart.Elements {
		switch e := el.(type) {
		case *ast.ActionStatement:
			if e.Body != nil {
				visit(e.Body)
			}
		case *ast.TransitionStatement:
			if e.Condition != nil {
				visit(e.Condition)
			}
		}
	}
	var out []*action
	for _, a := range actions {
		if used[a] {
			out = append(out, a)
		}
	}
	return out
}
