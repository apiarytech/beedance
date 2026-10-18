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

// The walk over each POU's body: what it reads and writes, what it calls,
// and the checks made on the way (unreachable code, narrowing, loops,
// equality of reals and times, the PLCopen rules on statements).

import (
	"fmt"
	"math"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// ctx is where the walk is: the POU, and the loops around the statement.
type ctx struct {
	p     *pou
	loops []int64  // each enclosing loop's iterations a scan, -1 if not bounded
	fors  []string // the counters of the enclosing FOR loops, upper case
}

func (a *analysis) run() {
	for _, p := range a.pous {
		a.stmt(&ctx{p: p}, p.body)
	}
	a.afterWalk()
}

func pos(n ast.Node) (int, int) {
	if n == nil {
		return 0, 0
	}
	return n.Pos()
}

// touch counts a read or a write of v, and the checks a write makes.
func (a *analysis) touch(c *ctx, v *variable, write bool, row, col int) {
	if write {
		v.writes++
	} else {
		v.reads++
	}
	g := v
	if v.section == "VAR_EXTERNAL" {
		if gv := a.globals[up(v.name)]; gv != nil {
			g = gv
			if write {
				gv.writes++
			} else {
				gv.reads++
			}
		}
	}
	if !write {
		return
	}
	loc := v.located
	if loc == "" {
		loc = g.located
	}
	if strings.HasPrefix(loc, "Q") && !strings.Contains(loc, "*") {
		a.outputWrites[loc] = append(a.outputWrites[loc], site{row, col, c.p.name})
	}
	if g.section == "VAR_GLOBAL" && c.p.kind == "PROGRAM" {
		if a.globalWriters[g] == nil {
			a.globalWriters[g] = map[string]site{}
		}
		if _, seen := a.globalWriters[g][c.p.name]; !seen {
			a.globalWriters[g][c.p.name] = site{row, col, c.p.name}
		}
	}
	if v.section == "VAR_INPUT" {
		a.report("input-write", Warning, row, col, c.p.name,
			fmt.Sprintf("%s is an input: a POU should not write its own inputs", v.name))
	}
	for _, f := range c.fors {
		if f == up(v.name) && v.pou != nil {
			a.report("for-counter-modified", Warning, row, col, c.p.name,
				fmt.Sprintf("the FOR counter %s is changed inside its loop", v.name))
		}
	}
}

// use walks an expression that is read, or written when write is set (the
// left of an assignment, an output's target).
func (a *analysis) use(c *ctx, e ast.Expression, write bool) {
	switch x := e.(type) {
	case nil:
	case *ast.Identifier:
		if v := a.lookup(c.p, x.Value); v != nil {
			a.touch(c, v, write, x.Token.Row, x.Token.Column)
		}
	case *ast.DirectVariable:
		a.report("direct-address", Info, x.Token.Row, x.Token.Column, c.p.name,
			fmt.Sprintf("physical address %%%s in code: declare a variable AT it", x.Address))
		if write && strings.HasPrefix(up(x.Address), "Q") {
			a.outputWrites[up(x.Address)] = append(a.outputWrites[up(x.Address)], site{x.Token.Row, x.Token.Column, c.p.name})
		}
	case *ast.MemberAccessExpression:
		a.use(c, x.Struct, write)
	case *ast.IndexExpression:
		a.use(c, x.Left, write)
		a.use(c, x.Index, false)
	case *ast.BitAccessExpression:
		a.use(c, x.Target, write)
	case *ast.DereferenceExpression:
		a.use(c, x.Pointer, false)
	case *ast.PrefixExpression:
		a.use(c, x.Right, false)
	case *ast.InfixExpression:
		a.use(c, x.Left, false)
		a.use(c, x.Right, false)
		a.equality(c, x)
	case *ast.CallExpression:
		a.call(c, x)
	case *ast.NamedArgument:
		a.use(c, x.Value, false)
	case *ast.OutputArgument:
		a.use(c, x.Target, true)
	case *ast.IntegerLiteral, *ast.UnsignedIntegerLiteral, *ast.RealLiteral, *ast.LRealLiteral, *ast.BitStringLiteral,
		*ast.Boolean, *ast.StringLiteral, *ast.WStringLiteral, *ast.TimeLiteral, *ast.DateLiteral, *ast.TimeOfDayLiteral,
		*ast.DateAndTimeLiteral, *ast.TypedLiteral, *ast.EnumeratedValueLiteral, *ast.ThisExpression, *ast.SuperExpression:
	default:
		a.fallback(c, e)
	}
}

// fallback counts every name in a node it does not know as read, so that
// what it holds is not reported unused.
func (a *analysis) fallback(c *ctx, n ast.Node) {
	ast.Modify(n, func(m ast.Node) ast.Node {
		if id, ok := m.(*ast.Identifier); ok {
			if v := a.lookup(c.p, id.Value); v != nil {
				a.touch(c, v, false, id.Token.Row, id.Token.Column)
			}
		}
		return m
	})
}

// equality checks = and <> on reals (CP8) and times (CP28).
func (a *analysis) equality(c *ctx, x *ast.InfixExpression) {
	if x.Operator != "=" && x.Operator != "<>" {
		return
	}
	l, r := a.typeOf(c.p, x.Left), a.typeOf(c.p, x.Right)
	switch {
	case (isReal(l) || isReal(r)) && !(l == anyReal && r == anyReal):
		a.report("float-equality", Warning, x.Token.Row, x.Token.Column, c.p.name,
			fmt.Sprintf("%s on floating point values: compare with a tolerance, ABS(a - b) < eps", x.Operator))
	case familyOf(l) == famTime || familyOf(r) == famTime: // a DATE is a whole day, which = can compare
		a.report("time-equality", Warning, x.Token.Row, x.Token.Column, c.p.name,
			fmt.Sprintf("%s on times: a time may step past the value, compare with >= or <=", x.Operator))
	}
}

// call walks a call: the instance or function called, its arguments, and
// the types of the arguments against the parameters.
func (a *analysis) call(c *ctx, x *ast.CallExpression) {
	var callee *pou
	stdFB := ""
	switch f := x.Function.(type) {
	case *ast.Identifier:
		if v := a.lookup(c.p, f.Value); v != nil {
			a.touch(c, v, false, f.Token.Row, f.Token.Column)
			t := a.resolve(v.typ)
			if fb, ok := a.fbs[t]; ok {
				callee = fb
			} else if a.isFBType(t) {
				stdFB = t
			}
			if callee != nil || stdFB != "" {
				c.p.instanceCalls[v] = append(c.p.instanceCalls[v], [2]int{f.Token.Row, f.Token.Column})
			}
		} else if fn, ok := a.funcs[up(f.Value)]; ok {
			callee = fn
		}
	case *ast.MemberAccessExpression:
		a.use(c, f.Struct, false)
		if f.Member != nil {
			if fb, ok := a.fbs[a.typeOf(c.p, f.Struct)]; ok {
				callee = a.method(fb, f.Member.Value)
			}
		}
	default:
		a.use(c, x.Function, false)
	}
	if callee != nil {
		if _, seen := c.p.calls[callee.key]; !seen {
			r, col := pos(x.Function)
			c.p.calls[callee.key] = [2]int{r, col}
		}
	}
	var positional []*variable // a function's inputs, for positional arguments
	if callee != nil && callee.kind == "FUNCTION" {
		for _, v := range callee.order {
			if v.section == "VAR_INPUT" {
				positional = append(positional, v)
			}
			if v.section == "VAR_IN_OUT" {
				positional = nil
				break
			}
		}
	}
	n := 0
	for _, arg := range x.Arguments {
		switch y := arg.(type) {
		case *ast.NamedArgument:
			a.use(c, y.Value, false)
			if y.Name == nil {
				continue
			}
			if callee != nil {
				if pv, ok := callee.vars[up(y.Name.Value)]; ok && pv.section == "VAR_IN_OUT" {
					a.use(c, y.Value, true)
					continue
				}
			}
			if pt := a.param(callee, stdFB, y.Name.Value); pt != "" {
				a.narrow(c, pt, y.Value, "parameter "+y.Name.Value)
			}
		case *ast.OutputArgument:
			a.use(c, y.Target, true)
			if y.Source != nil {
				if st := a.param(callee, stdFB, y.Source.Value); st != "" {
					a.narrowTypes(c, a.typeOf(c.p, y.Target), st, y.Target, "output "+y.Source.Value)
				}
			}
		default:
			a.use(c, arg, false)
			if callee != nil && callee.kind != "FUNCTION" {
				a.use(c, arg, true) // a method's or block's in-out, maybe
			}
			if n < len(positional) {
				a.narrow(c, a.resolve(positional[n].typ), arg, "parameter "+positional[n].name)
			}
			n++
		}
	}
}

// narrow checks a value given to a target of type target: an assignment, a
// parameter. A literal must fit; another value must convert implicitly.
func (a *analysis) narrow(c *ctx, target string, val ast.Expression, what string) {
	t := a.resolve(target)
	if !isElementary(t) || val == nil {
		return
	}
	s := a.typeOf(c.p, val)
	if s == anyInt {
		k, ok := a.constant(c.p, val)
		if !ok || k.isBool {
			return
		}
		row, col := pos(val)
		if lo, hi, ok := intRange(t); ok {
			if float64(k.i) < lo || float64(k.i) > hi {
				a.report("literal-range", Error, row, col, c.p.name,
					fmt.Sprintf("%s: %d is out of the range of %s (%.0f..%.0f)", what, k.i, t, lo, hi))
			}
			return
		}
		if !isReal(t) {
			a.report("narrowing", Warning, row, col, c.p.name,
				fmt.Sprintf("%s: an integer given to a %s", what, t))
		}
		return
	}
	a.narrowTypes(c, t, s, val, what)
}

// narrowTypes checks that a value of type s goes to a target of type t
// without an implicit conversion that loses something.
func (a *analysis) narrowTypes(c *ctx, t, s string, at ast.Node, what string) {
	t = a.resolve(t)
	if s == "" || t == "" || s == t || !isElementary(t) {
		return
	}
	row, col := pos(at)
	if s == anyReal {
		if !isReal(t) && numeric(t) {
			a.report("narrowing", Warning, row, col, c.p.name,
				fmt.Sprintf("%s: a real given to a %s loses its fraction; write REAL_TO_%s", what, t, t))
		}
		return
	}
	if !isElementary(s) || implicit(s, t) {
		return
	}
	a.report("narrowing", Warning, row, col, c.p.name,
		fmt.Sprintf("%s: %s to %s %s; write %s_TO_%s", what, s, t, loss(s, t), s, t))
}

// loss says what a conversion from s to t may lose.
func loss(s, t string) string {
	fs, ft := familyOf(s), familyOf(t)
	switch {
	case fs == famReal && (ft == famSigned || ft == famUnsigned || ft == famBits):
		return "drops the fraction and may overflow"
	case fs == famReal && ft == famReal:
		return "loses precision"
	case fs == famSigned && ft == famUnsigned:
		return "loses the sign"
	case (fs == famSigned || fs == famUnsigned) && ft == famReal:
		return "may lose precision"
	case numeric(s) && numeric(t):
		if _, hs, ok := intRange(s); ok {
			if _, ht, ok := intRange(t); ok && ht < hs {
				return "may overflow"
			}
		}
		return "is not an implicit conversion"
	}
	return "is not an implicit conversion"
}

func first(b *ast.BlockStatement) ast.Statement {
	if b == nil || len(b.Statements) == 0 {
		return nil
	}
	return b.Statements[0]
}

func (a *analysis) unreachable(c *ctx, s ast.Node, why string) {
	if s == nil {
		return
	}
	if b, ok := s.(*ast.BlockStatement); ok {
		if s = first(b); s == nil {
			return
		}
	}
	row, col := s.Pos()
	a.report("unreachable-code", Warning, row, col, c.p.name, "unreachable: "+why)
}

// terminator names the statement after which a block goes no further.
func terminator(s ast.Statement) string {
	switch x := s.(type) {
	case *ast.ReturnStatement:
		return "RETURN"
	case *ast.ExitStatement:
		return "EXIT"
	case *ast.ContinueStatement:
		return "CONTINUE"
	case *ast.JumpStatement:
		if x.Label != nil {
			return "JMP " + x.Label.Value
		}
		return "JMP"
	case *ast.IlInstructionStatement:
		switch up(x.Operator) {
		case "RET", "JMP":
			return up(x.Operator)
		}
	}
	return ""
}

func isLabel(s ast.Statement) bool {
	switch x := s.(type) {
	case *ast.LabelStatement:
		return true
	case *ast.IlInstructionStatement:
		return x.Label != nil
	}
	return false
}

func (a *analysis) block(c *ctx, stmts []ast.Statement) {
	dead, reported := "", false
	for _, st := range stmts {
		if isLabel(st) {
			dead, reported = "", false
		}
		if dead != "" && !reported {
			a.unreachable(c, st, "after "+dead)
			reported = true
		}
		a.stmt(c, st)
		if t := terminator(st); t != "" && dead == "" {
			dead = t
		}
	}
}

func (a *analysis) stmt(c *ctx, s ast.Statement) {
	switch x := s.(type) {
	case nil:
	case *ast.BlockStatement:
		if x != nil {
			a.block(c, x.Statements)
		}
	case *ast.AssignmentStatement:
		a.assignment(c, x)
	case *ast.ExpressionStatement:
		a.use(c, x.Expression, false)
	case *ast.IfStatement:
		a.ifStatement(c, x, true)
	case *ast.CaseStatement:
		a.use(c, x.Expression, false)
		for _, br := range x.Cases {
			if br == nil {
				continue
			}
			for _, v := range br.Values {
				a.use(c, v, false)
			}
			a.stmt(c, br.Consequence)
		}
		if x.Alternative != nil {
			a.stmt(c, x.Alternative)
		}
	case *ast.ForLoopStatement:
		a.forLoop(c, x)
	case *ast.WhileStatement:
		a.conditionLoop(c, x, x.Condition, x.Body, false)
	case *ast.RepeatStatement:
		a.conditionLoop(c, x, x.Condition, x.Body, true)
	case *ast.ReturnStatement:
		a.use(c, x.ReturnValue, false)
		a.report("jump-return", Info, x.Token.Row, x.Token.Column, c.p.name, "RETURN: a POU should have a single exit at its end")
	case *ast.ExitStatement:
		a.report("continue-exit", Info, x.Token.Row, x.Token.Column, c.p.name, "EXIT leaves the loop from its middle")
	case *ast.ContinueStatement:
		a.report("continue-exit", Info, x.Token.Row, x.Token.Column, c.p.name, "CONTINUE skips the rest of the loop's body")
	case *ast.JumpStatement:
		a.report("jump-return", Info, x.Token.Row, x.Token.Column, c.p.name, "JMP: structured statements are easier to follow")
	case *ast.LabelStatement:
	case *ast.IlInstructionStatement:
		a.il(c, x)
	case *ast.SFCProgram:
		for _, el := range x.Elements {
			a.stmt(c, el)
		}
	case *ast.ActionStatement:
		a.stmt(c, x.Body)
	case *ast.StepStatement:
		a.stmt(c, x.Body)
		for _, ab := range x.Actions {
			a.actionBlock(c, ab)
		}
		for _, tr := range x.Transitions {
			a.stmt(c, tr)
		}
	case *ast.TransitionStatement:
		a.use(c, x.Condition, false)
		a.use(c, x.Priority, false)
	case *ast.ActionBlockStatement:
		a.actionBlock(c, x)
	default:
		a.fallback(c, s)
	}
}

func (a *analysis) actionBlock(c *ctx, ab *ast.ActionBlockStatement) {
	if ab == nil {
		return
	}
	if ab.ActionName != nil {
		if v := a.lookup(c.p, ab.ActionName.Value); v != nil {
			a.touch(c, v, true, ab.ActionName.Token.Row, ab.ActionName.Token.Column) // an action's BOOL
		}
	}
	a.use(c, ab.Duration, false)
	if ab.Body != nil {
		a.stmt(c, ab.Body)
	}
}

func lastPart(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (a *analysis) assignment(c *ctx, x *ast.AssignmentStatement) {
	a.use(c, x.Value, false)
	if id, ok := x.Left.(*ast.Identifier); ok && (c.p.kind == "FUNCTION" || c.p.kind == "METHOD") &&
		up(id.Value) == up(lastPart(c.p.name)) && a.lookup(c.p, id.Value) == nil {
		c.p.set = true
		if c.p.result != "" {
			a.narrow(c, c.p.result, x.Value, "result")
		}
		return
	}
	a.use(c, x.Left, true)
	if x.Token.Literal == "?=" {
		return
	}
	if t := a.typeOf(c.p, x.Left); t != "" {
		a.narrow(c, t, x.Value, "assignment")
	}
}

func hasElse(x *ast.IfStatement) bool {
	switch alt := x.Alternative.(type) {
	case nil:
		return false
	case *ast.IfStatement:
		return hasElse(alt)
	case *ast.BlockStatement:
		return alt != nil
	}
	return true
}

func (a *analysis) ifStatement(c *ctx, x *ast.IfStatement, top bool) {
	a.use(c, x.Condition, false)
	k, isConst := a.constant(c.p, x.Condition)
	isConst = isConst && k.isBool
	if isConst && !k.b {
		a.unreachable(c, x.Consequence, "the condition is always FALSE")
	}
	a.stmt(c, x.Consequence)
	if x.Alternative != nil {
		if isConst && k.b {
			a.unreachable(c, x.Alternative, "an earlier condition is always TRUE")
		}
		if alt, ok := x.Alternative.(*ast.IfStatement); ok {
			a.ifStatement(c, alt, false)
		} else {
			a.stmt(c, x.Alternative)
		}
	}
	if top && !hasElse(x) {
		a.report("if-without-else", Info, x.Token.Row, x.Token.Column, c.p.name, "IF without ELSE")
	}
}

func (a *analysis) il(c *ctx, x *ast.IlInstructionStatement) {
	op := up(x.Operator)
	switch {
	case strings.HasPrefix(op, "JMP"), strings.HasPrefix(op, "RET"):
		a.report("jump-return", Info, x.Token.Row, x.Token.Column, c.p.name, op+": structured statements are easier to follow")
		if strings.HasPrefix(op, "RET") {
			a.use(c, x.Operand, false)
		}
	case op == "ST" || op == "STN" || op == "S" || op == "R":
		a.use(c, x.Operand, true)
	default:
		a.use(c, x.Operand, false)
	}
}

// mul multiplies iterations, saturating at the largest int64.
func mul(x, y int64) int64 {
	if x == 0 || y == 0 {
		return 0
	}
	if x > math.MaxInt64/y {
		return math.MaxInt64
	}
	return x * y
}

// iterations is the product of the enclosing loops' iterations, -1 when one
// is not bounded.
func (c *ctx) iterations() int64 {
	n := int64(1)
	for _, l := range c.loops {
		if l < 0 {
			return -1
		}
		n = mul(n, l)
	}
	return n
}

func (a *analysis) forLoop(c *ctx, x *ast.ForLoopStatement) {
	counter := ""
	var cv *variable
	ctype := ""
	var start ast.Expression
	if x.ControlVar != nil {
		start = x.ControlVar.Value
		a.use(c, start, false)
		if id, ok := x.ControlVar.Left.(*ast.Identifier); ok {
			counter = up(id.Value)
			if cv = a.lookup(c.p, id.Value); cv != nil {
				cv.reads++
				cv.writes++
				ctype = a.resolve(cv.typ)
				a.narrow(c, ctype, start, "FOR start")
			}
		} else {
			a.use(c, x.ControlVar.Left, true)
		}
	}
	a.use(c, x.EndValue, false)
	a.use(c, x.StepValue, false)
	if cv != nil {
		a.narrow(c, ctype, x.EndValue, "FOR end")
		if x.StepValue != nil {
			a.narrow(c, ctype, x.StepValue, "FOR step")
		}
	}

	iterations := int64(-1)
	step, stepConst := int64(1), true
	if x.StepValue != nil {
		if k, ok := a.constant(c.p, x.StepValue); ok && !k.isBool {
			step = k.i
		} else {
			stepConst = false
		}
	}
	row, col := x.Token.Row, x.Token.Column
	modified := counter != "" && assigns(x.Body, counter)
	s, okS := a.constant(c.p, start)
	e, okE := a.constant(c.p, x.EndValue)
	lo, hi, ranged := intRange(ctype)
	switch {
	case stepConst && step == 0:
		a.report("unbounded-loop", Error, row, col, c.p.name, "FOR with a step of 0 never ends")
	case modified:
	case okE && !e.isBool && ranged && ((step > 0 && float64(e.i) >= hi) || (step < 0 && float64(e.i) <= lo)):
		a.report("unbounded-loop", Error, row, col, c.p.name,
			fmt.Sprintf("FOR ... TO %d never ends: the %s counter %s cannot pass it, so it wraps", e.i, ctype, cv.name))
	case okS && okE && !s.isBool && !e.isBool && stepConst:
		n := (e.i-s.i)/step + 1
		if (step > 0 && e.i < s.i) || (step < 0 && e.i > s.i) {
			n = 0
		}
		iterations = n
		if n == 0 {
			a.unreachable(c, x.Body, fmt.Sprintf("the FOR loop from %d to %d runs no time", s.i, e.i))
		}
	case stepConst && ranged:
		// Bounded only by the counter's type.
		span := hi - lo + 1
		if n := span / math.Abs(float64(step)); n < math.MaxInt64 {
			iterations = int64(n)
		} else {
			iterations = math.MaxInt64
		}
	}
	a.loopBudget(c, row, col, iterations, "FOR")

	c.loops = append(c.loops, iterations)
	if counter != "" {
		c.fors = append(c.fors, counter)
	}
	a.stmt(c, x.Body)
	if counter != "" {
		c.fors = c.fors[:len(c.fors)-1]
	}
	c.loops = c.loops[:len(c.loops)-1]
}

// loopBudget reports a loop whose iterations, with the loops around it,
// may pass the limit, once: not again inside a loop already over it.
func (a *analysis) loopBudget(c *ctx, row, col int, n int64, kind string) {
	outer := c.iterations()
	if n <= 0 || outer < 0 || outer > a.opts.MaxIterations {
		return
	}
	if total := mul(outer, n); total > a.opts.MaxIterations {
		how := fmt.Sprintf("up to %d iterations", total)
		if total == math.MaxInt64 {
			how = "more iterations than an int64 holds"
		}
		a.report("loop-iterations", Warning, row, col, c.p.name,
			fmt.Sprintf("%s: %s a scan with the loops around it, over the limit of %d: the scan may overrun its cycle time", kind, how, a.opts.MaxIterations))
	}
}

// conditionLoop checks a WHILE or REPEAT loop: one that never ends, and
// one whose iterations nothing bounds.
func (a *analysis) conditionLoop(c *ctx, at ast.Statement, cond ast.Expression, body *ast.BlockStatement, repeat bool) {
	a.use(c, cond, false)
	kind := "WHILE"
	if repeat {
		kind = "REPEAT"
	}
	row, col := at.Pos()
	k, isConst := a.constant(c.p, cond)
	isConst = isConst && k.isBool
	leaves := exits(body, true)
	runsForever := isConst && ((!repeat && k.b) || (repeat && !k.b))
	switch {
	case isConst && !repeat && !k.b:
		a.unreachable(c, body, "the WHILE condition is always FALSE")
	case runsForever && !leaves:
		a.report("unbounded-loop", Error, row, col, c.p.name,
			fmt.Sprintf("%s never ends: its condition never changes and nothing leaves it", kind))
	case !isConst && !leaves && a.invariant(c, cond, body):
		a.report("unbounded-loop", Error, row, col, c.p.name,
			fmt.Sprintf("%s never ends once it starts: nothing in it changes its condition", kind))
	default:
		a.report("unbounded-loop", Warning, row, col, c.p.name,
			fmt.Sprintf("%s: nothing bounds how often it runs a scan, which may overrun the cycle time; prefer a FOR with a bound", kind))
	}
	c.loops = append(c.loops, -1)
	a.stmt(c, body)
	c.loops = c.loops[:len(c.loops)-1]
}

// exits reports whether a loop's body can leave it: RETURN or JMP anywhere,
// EXIT outside an inner loop (here is false inside one).
func exits(s ast.Statement, here bool) bool {
	switch x := s.(type) {
	case *ast.ReturnStatement, *ast.JumpStatement:
		return true
	case *ast.ExitStatement:
		return here
	case *ast.BlockStatement:
		if x == nil {
			return false
		}
		for _, st := range x.Statements {
			if exits(st, here) {
				return true
			}
		}
	case *ast.IfStatement:
		if exits(x.Consequence, here) {
			return true
		}
		if x.Alternative != nil {
			return exits(x.Alternative, here)
		}
	case *ast.CaseStatement:
		for _, br := range x.Cases {
			if br != nil && exits(br.Consequence, here) {
				return true
			}
		}
		if x.Alternative != nil {
			return exits(x.Alternative, here)
		}
	case *ast.ForLoopStatement:
		return exits(x.Body, false)
	case *ast.WhileStatement:
		return exits(x.Body, false)
	case *ast.RepeatStatement:
		return exits(x.Body, false)
	case *ast.IlInstructionStatement:
		op := up(x.Operator)
		return strings.HasPrefix(op, "JMP") || strings.HasPrefix(op, "RET")
	}
	return false
}

// assigns reports whether a statement assigns name (upper case) by name.
func assigns(s ast.Node, name string) bool {
	found := false
	ast.Modify(s, func(n ast.Node) ast.Node {
		if as, ok := n.(*ast.AssignmentStatement); ok {
			if id, ok := as.Left.(*ast.Identifier); ok && up(id.Value) == name {
				found = true
			}
		}
		return n
	})
	return found
}

// invariant reports whether nothing in a loop's body can change its
// condition: the condition calls nothing, and no variable it reads is
// written in the body, passed to a call, or (when the body calls a POU) a
// global, an external or an in-out.
func (a *analysis) invariant(c *ctx, cond ast.Expression, body *ast.BlockStatement) bool {
	names := map[string]bool{}
	ok := true
	ast.Modify(cond, func(n ast.Node) ast.Node {
		switch x := n.(type) {
		case *ast.CallExpression, *ast.DereferenceExpression:
			ok = false
		case *ast.Identifier:
			names[up(x.Value)] = true
		case *ast.DirectVariable:
			names["%"+up(x.Address)] = true
		}
		return n
	})
	if !ok || len(names) == 0 {
		return false
	}
	// Member names (t.Q) are among names; they are harmless, as the base is
	// there too.
	written, callsPOU := a.changes(c, body)
	vars := 0
	for name := range names {
		if written[name] {
			return false
		}
		if strings.HasPrefix(name, "%") {
			vars++
			continue
		}
		v := a.lookup(c.p, name)
		if v == nil {
			continue
		}
		vars++
		if callsPOU && (v.section == "VAR_GLOBAL" || v.section == "VAR_EXTERNAL" || v.section == "VAR_IN_OUT") {
			return false
		}
	}
	return vars > 0
}

// changes are the names a body may change: assigned, given to an output or
// to any call, instances it calls; callsPOU when it calls one of the
// program's POUs, which may change globals.
func (a *analysis) changes(c *ctx, body ast.Node) (map[string]bool, bool) {
	written := map[string]bool{}
	callsPOU := false
	base := func(e ast.Expression) {
		for {
			switch x := e.(type) {
			case *ast.Identifier:
				written[up(x.Value)] = true
				return
			case *ast.DirectVariable:
				written["%"+up(x.Address)] = true
				return
			case *ast.MemberAccessExpression:
				e = x.Struct
			case *ast.IndexExpression:
				e = x.Left
			case *ast.BitAccessExpression:
				e = x.Target
			default:
				return
			}
		}
	}
	ast.Modify(body, func(n ast.Node) ast.Node {
		switch x := n.(type) {
		case *ast.AssignmentStatement:
			base(x.Left)
		case *ast.OutputArgument:
			base(x.Target)
		case *ast.ForLoopStatement:
			if x.ControlVar != nil {
				base(x.ControlVar.Left)
			}
		case *ast.IlInstructionStatement:
			switch up(x.Operator) {
			case "ST", "STN", "S", "R":
				base(x.Operand)
			}
		case *ast.CallExpression:
			base(x.Function)
			if id, ok := x.Function.(*ast.Identifier); ok {
				if _, fn := a.funcs[up(id.Value)]; fn {
					callsPOU = true
				} else if v := a.lookup(c.p, id.Value); v != nil {
					if _, fb := a.fbs[a.resolve(v.typ)]; fb {
						callsPOU = true
					}
				}
			} else if _, ok := x.Function.(*ast.MemberAccessExpression); ok {
				callsPOU = true
			}
			for _, arg := range argValues(x.Arguments) {
				base(arg)
			}
		}
		return n
	})
	return written, callsPOU
}
