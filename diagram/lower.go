/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package diagram holds the model of IEC 61131-3's graphical languages,
// Function Block Diagram and Ladder Diagram, and lowers it to Structured
// Text statements, so they compile and run like any ST body. The model is
// PLCopen TC6 XML's: package plcopen reads it from XML, and this package
// reads it from beedance's text form too (text.go).
//
// Lowering:
//
//   - a function block is a call, its outputs read as inst.OUT;
//   - a function is an expression: an operator (ADD is +, GT is >, ...) or a
//     call with its inputs in order;
//   - an output, an in/out variable or a coil is an assignment;
//   - contacts in series are AND, wires joining at one input are OR, a
//     negated contact or pin is NOT, set and reset coils assign under IF,
//     edges use an R_TRIG or F_TRIG instance declared for them;
//   - a function block whose EN is wired reports ENO into a BOOL declared
//     for it, and the outputs it feeds are assigned only IF it is TRUE.
//
// Order: an element's executionOrderId when every statement has one;
// otherwise the networks (elements joined by wires; an LD rung) run top to
// bottom, and inside a network each statement runs after the statements it
// reads from, ties going top to bottom, left to right. Every generated line
// keeps the localId of its element, to place errors and live values.
package diagram

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Lowered is a graphical body as Structured Text.
type Lowered struct {
	// Body is the statements, one or more lines each.
	Body string
	// Lines gives the localId of the element each line of Body came from.
	Lines []int
	// Rows gives the source line each line of Body came from, for a body in
	// the text form (LowerText).
	Rows []int
	// Decls are the variables the statements need besides the POU's own:
	// edge detectors, ENO flags and unnamed function block instances, each
	// "name : TYPE;".
	Decls []string
	// Warnings say what was resolved by a rule the file did not settle,
	// such as a feedback loop.
	Warnings []string
}

// StandardFunctionBlocks are the IEC 61131-3 function blocks: a block of
// one of these types without an instance name gets one.
var StandardFunctionBlocks = []string{"TON", "TOF", "TP", "CTU", "CTD", "CTUD", "R_TRIG", "F_TRIG", "SR", "RS"}

// Conn is a wire into an input: from element Ref, at its output Param ("" is
// the element's only or main output).
type Conn struct {
	Ref   int    `xml:"refLocalId,attr"`
	Param string `xml:"formalParameter,attr"`
}

// PointIn is an input: the wires joining at it (more than one is an OR, in LD).
type PointIn struct {
	Conns []Conn `xml:"connection"`
}

// Pos is where an element is drawn; the text form uses its line and column.
type Pos struct {
	X float64 `xml:"x,attr"`
	Y float64 `xml:"y,attr"`
}

// Pin is a block's input, in/out or output variable.
type Pin struct {
	Param   string   `xml:"formalParameter,attr"`
	Negated bool     `xml:"negated,attr"`
	Edge    string   `xml:"edge,attr"`
	In      *PointIn `xml:"connectionPointIn"`
}

// Elem is any element of an FBD or LD body; the fields used depend on its
// kind, the name of its PLCopen element (XMLName.Local): inVariable,
// outVariable, inOutVariable, block, connector, continuation,
// leftPowerRail, rightPowerRail, contact or coil.
type Elem struct {
	XMLName      xml.Name
	ID           int       `xml:"localId,attr"`
	Order        int       `xml:"executionOrderId,attr"`
	Pos          Pos       `xml:"position"`
	Ins          []PointIn `xml:"connectionPointIn"`
	Expression   string    `xml:"expression"`
	Variable     string    `xml:"variable"`
	Negated      bool      `xml:"negated,attr"`
	NegatedIn    bool      `xml:"negatedIn,attr"`
	NegatedOut   bool      `xml:"negatedOut,attr"`
	Edge         string    `xml:"edge,attr"`
	Storage      string    `xml:"storage,attr"`
	Name         string    `xml:"name,attr"`
	TypeName     string    `xml:"typeName,attr"`
	InstanceName string    `xml:"instanceName,attr"`
	Inputs       []Pin     `xml:"inputVariables>variable"`
	InOuts       []Pin     `xml:"inOutVariables>variable"`
	Outputs      []Pin     `xml:"outputVariables>variable"`
}

// Kind is the element's kind (see Elem).
func (e *Elem) Kind() string { return e.XMLName.Local }

// expr is a lowered expression: its text, the statements that must run
// before it (edge detectors), and the statement elements it reads.
type expr struct {
	text string
	pre  []string
	deps []int
	// guard is the EN of a function whose result this is: it may only be
	// assigned, under IF guard (no temporary of the function's type).
	guard string
}

type stmt struct {
	id    int
	lines []string
	deps  []int
	elem  *Elem
}

type lowering struct {
	pou        string
	elems      map[int]*Elem
	inDoc      []*Elem
	connectors map[string]*Elem
	fbTypes    map[string]bool
	declared   map[string]bool // upper-case names
	decls      []string
	instance   map[int]string // function block element -> instance name
	enoFlag    map[int]string // function block element with EN wired -> its ENO flag
	edges      int
	edgeOf     map[int]*expr // edge contact -> its trigger, shared by every statement its rung feeds
	current    int           // the element whose statement is being made
	visiting   map[int]bool
	warnings   []string
	oneNetwork bool
}

// Options are what lowering needs to know about the POU.
type Options struct {
	// FunctionBlocks names the project's function block types (any case);
	// the standard ones are known. A block of one of these types is a call.
	FunctionBlocks []string
	// Declared names the POU's variables, so generated names avoid them.
	Declared []string
	// OneNetwork orders the whole body as one network, by dataflow with
	// ties in position order: the FBD text form, whose statements read
	// each other through variables rather than wires.
	OneNetwork bool
}

// ParseXML reads the elements of an FBD or LD body, given as the inner XML
// of its PLCopen <FBD> or <LD> element.
func ParseXML(innerXML string) ([]*Elem, error) {
	var body struct {
		Elems []*Elem `xml:",any"`
	}
	if err := xml.Unmarshal([]byte("<body>"+innerXML+"</body>"), &body); err != nil {
		return nil, err
	}
	return body.Elems, nil
}

// Lower lowers the elements of an FBD or LD body to Structured Text.
func Lower(pou string, elems []*Elem, opt Options) (*Lowered, error) {
	l := &lowering{pou: pou, elems: map[int]*Elem{}, connectors: map[string]*Elem{}, fbTypes: map[string]bool{},
		declared: map[string]bool{}, instance: map[int]string{}, enoFlag: map[int]string{}, visiting: map[int]bool{}, edgeOf: map[int]*expr{},
		oneNetwork: opt.OneNetwork}
	for _, t := range append(slices.Clone(StandardFunctionBlocks), opt.FunctionBlocks...) {
		l.fbTypes[strings.ToUpper(t)] = true
	}
	for _, d := range opt.Declared {
		l.declared[strings.ToUpper(d)] = true
	}
	for _, e := range elems {
		switch e.Kind() {
		case "comment", "addData", "documentation":
			continue
		case "jump", "label", "return":
			return nil, fmt.Errorf("pou '%s': element %d: %s is not supported yet in FBD and LD", pou, e.ID, e.Kind())
		case "step", "transition", "selectionDivergence", "selectionConvergence", "simultaneousDivergence",
			"simultaneousConvergence", "jumpStep", "macroStep", "actionBlock":
			return nil, fmt.Errorf("pou '%s': element %d: SFC element %s in an FBD or LD body", pou, e.ID, e.Kind())
		}
		if _, dup := l.elems[e.ID]; dup {
			return nil, fmt.Errorf("pou '%s': localId %d is used twice", pou, e.ID)
		}
		l.elems[e.ID] = e
		l.inDoc = append(l.inDoc, e)
		if e.Kind() == "connector" {
			l.connectors[strings.ToUpper(e.Name)] = e
		}
	}
	// Function block instances first: statements refer to them.
	for _, e := range l.inDoc {
		if e.Kind() == "block" && l.isFB(e) {
			name := e.InstanceName
			if name == "" {
				name = l.unique(fmt.Sprintf("_%s%d", strings.ToUpper(e.TypeName), e.ID))
				l.decls = append(l.decls, fmt.Sprintf("%s : %s;", name, e.TypeName))
			}
			l.instance[e.ID] = name
			if pin := findPin(e.Inputs, "EN"); pin != nil && pin.In != nil && len(pin.In.Conns) > 0 {
				flag := l.unique(fmt.Sprintf("_ENO%d", e.ID))
				l.decls = append(l.decls, flag+" : BOOL;")
				l.enoFlag[e.ID] = flag
			}
		}
	}

	var stmts []*stmt
	for _, e := range l.inDoc {
		l.current = e.ID
		s, err := l.statement(e)
		if err != nil {
			return nil, fmt.Errorf("pou '%s': element %d (%s): %w", pou, e.ID, e.Kind(), err)
		}
		if s != nil {
			stmts = append(stmts, s)
		}
	}
	ordered := l.order(stmts)

	out := &Lowered{Decls: l.decls, Warnings: l.warnings}
	var b strings.Builder
	for _, s := range ordered {
		for _, line := range s.lines {
			b.WriteString(line + "\n")
			out.Lines = append(out.Lines, s.id)
		}
	}
	out.Body = b.String()
	return out, nil
}

func (l *lowering) isFB(e *Elem) bool {
	return e.InstanceName != "" || l.fbTypes[strings.ToUpper(e.TypeName)]
}

// unique returns name, or name with a suffix, unused in the POU.
func (l *lowering) unique(name string) string {
	n := name
	for i := 2; l.declared[strings.ToUpper(n)]; i++ {
		n = fmt.Sprintf("%s_%d", name, i)
	}
	l.declared[strings.ToUpper(n)] = true
	return n
}

func findPin(pins []Pin, param string) *Pin {
	for i := range pins {
		if strings.EqualFold(pins[i].Param, param) {
			return &pins[i]
		}
	}
	return nil
}

// statement returns the statement an element makes, if it makes one.
func (l *lowering) statement(e *Elem) (*stmt, error) {
	switch e.Kind() {
	case "block":
		if !l.isFB(e) {
			return nil, nil // a function is an expression where it is read
		}
		return l.callStatement(e)
	case "outVariable", "inOutVariable":
		in, err := l.input(e)
		if err != nil || in == nil {
			return nil, err
		}
		target := strings.TrimSpace(e.Expression)
		if target == "" {
			return nil, fmt.Errorf("no variable")
		}
		value := in.text
		if e.Negated || e.NegatedIn {
			value = "NOT(" + value + ")"
		}
		return l.assignment(e, in, []string{target + " := " + value + ";"}), nil
	case "coil":
		in, err := l.input(e)
		if err != nil || in == nil {
			return nil, err
		}
		target := strings.TrimSpace(e.Variable)
		if target == "" {
			return nil, fmt.Errorf("coil without a variable")
		}
		var lines []string
		switch {
		case e.Storage == "set":
			lines = []string{"IF " + in.text + " THEN " + target + " := TRUE; END_IF;"}
		case e.Storage == "reset":
			lines = []string{"IF " + in.text + " THEN " + target + " := FALSE; END_IF;"}
		case e.Edge == "rising" || e.Edge == "falling":
			trig := l.edge(e.Edge, in.text)
			in.pre = append(in.pre, trig.pre...)
			lines = []string{target + " := " + trig.text + ";"}
		case e.Negated:
			lines = []string{target + " := NOT(" + in.text + ");"}
		default:
			lines = []string{target + " := " + in.text + ";"}
		}
		return l.assignment(e, in, lines), nil
	}
	return nil, nil
}

// assignment makes a statement of lines fed by in: after in's edge
// detectors, and only IF the ENO of a function block whose EN is wired, when
// in is read straight from it.
func (l *lowering) assignment(e *Elem, in *expr, lines []string) *stmt {
	if in.guard != "" { // a function with EN: assigned only IF EN
		for i := range lines {
			lines[i] = "IF " + in.guard + " THEN " + lines[i] + " END_IF;"
		}
	}
	if len(e.Ins) > 0 && len(e.Ins[0].Conns) == 1 {
		if flag, ok := l.enoFlag[e.Ins[0].Conns[0].Ref]; ok && !strings.EqualFold(e.Ins[0].Conns[0].Param, "ENO") {
			for i := range lines {
				lines[i] = "IF " + flag + " THEN " + lines[i] + " END_IF;"
			}
		}
	}
	return &stmt{id: e.ID, lines: append(slices.Clone(in.pre), lines...), deps: in.deps, elem: e}
}

// input is what an element's (single) input reads: the OR of its wires.
func (l *lowering) input(e *Elem) (*expr, error) {
	if len(e.Ins) == 0 || len(e.Ins[0].Conns) == 0 {
		return nil, nil // unconnected: nothing to do
	}
	return l.wires(e.Ins[0].Conns)
}

// wires is the value of one input: one wire's, or the OR of several (LD).
func (l *lowering) wires(conns []Conn) (*expr, error) {
	var parts []*expr
	for _, c := range conns {
		x, err := l.source(c)
		if err != nil {
			return nil, err
		}
		if len(conns) > 1 {
			if err := plain(x, "a wire joined with others"); err != nil {
				return nil, err
			}
		}
		parts = append(parts, x)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	out := &expr{}
	var texts []string
	for _, p := range parts {
		if p.text == "TRUE" {
			return &expr{text: "TRUE"}, nil // a wire from the rail powers the input on its own
		}
		text := p.text
		if strings.Contains(text, " AND ") && !strings.HasPrefix(text, "(") {
			text = "(" + text + ")" // readable: AND binds tighter anyway
		}
		texts = append(texts, text)
		out.pre = append(out.pre, p.pre...)
		out.deps = append(out.deps, p.deps...)
	}
	out.text = "(" + strings.Join(texts, " OR ") + ")"
	return out, nil
}

var simpleOperand = &lazyRegexp{src: `^(?:'(?:[^']|'')*'|"(?:[^"]|"")*"|[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*|\[[^\[\]]*\])*|[+-]?[0-9][0-9A-Za-z_.#:+-]*|[A-Za-z_]+#[0-9A-Za-z_.:+-]+)$`}

// lazyRegexp compiles on first use, so a program that never draws
// (firmware) does not carry the regexp package.
type lazyRegexp struct {
	once sync.Once
	src  string
	re   *regexp.Regexp
}

func (l *lazyRegexp) get() *regexp.Regexp {
	l.once.Do(func() { l.re = regexp.MustCompile(l.src) })
	return l.re
}

// operand returns text as it can stand inside an expression.
func operand(text string) string {
	text = strings.TrimSpace(text)
	if simpleOperand.get().MatchString(text) {
		return text
	}
	return "(" + text + ")"
}

// source is the value a wire carries from the element it starts at.
func (l *lowering) source(c Conn) (*expr, error) {
	e, ok := l.elems[c.Ref]
	if !ok {
		return nil, fmt.Errorf("a wire comes from element %d, which does not exist", c.Ref)
	}
	if l.visiting[e.ID] {
		return nil, fmt.Errorf("the wires through element %d form a loop without a variable to break it", e.ID)
	}
	l.visiting[e.ID] = true
	defer delete(l.visiting, e.ID)

	switch e.Kind() {
	case "leftPowerRail":
		return &expr{text: "TRUE"}, nil
	case "inVariable":
		text := operand(e.Expression)
		if e.Negated {
			text = "NOT(" + text + ")"
		}
		return &expr{text: text}, nil
	case "inOutVariable":
		text := operand(e.Expression) // the variable's value: its assignment runs first
		if e.NegatedOut {
			text = "NOT(" + text + ")"
		}
		return &expr{text: text, deps: []int{e.ID}}, nil
	case "continuation":
		conn, ok := l.connectors[strings.ToUpper(e.Name)]
		if !ok {
			return nil, fmt.Errorf("continuation %q has no connector", e.Name)
		}
		x, err := l.input(conn)
		if err != nil {
			return nil, err
		}
		if x == nil {
			return nil, fmt.Errorf("connector %q is not connected", e.Name)
		}
		return x, nil
	case "contact":
		v := strings.TrimSpace(e.Variable)
		if v == "" {
			return nil, fmt.Errorf("contact %d has no variable", e.ID)
		}
		term := &expr{text: operand(v)}
		switch {
		case e.Edge == "rising" || e.Edge == "falling":
			if done, ok := l.edgeOf[e.ID]; ok { // called by an earlier statement of the rung
				term = &expr{text: done.text, deps: done.deps}
			} else {
				term = l.edge(e.Edge, v)
				l.edgeOf[e.ID] = &expr{text: term.text, deps: []int{l.current}}
			}
		case e.Negated:
			term.text = "NOT(" + term.text + ")"
		}
		in, err := l.input(e)
		if err != nil {
			return nil, err
		}
		if in == nil {
			return nil, fmt.Errorf("contact %d (%s) is not connected on its left", e.ID, v)
		}
		if err := plain(in, fmt.Sprintf("contact %d", e.ID)); err != nil {
			return nil, err
		}
		if in.text == "TRUE" {
			term.pre = append(in.pre, term.pre...)
			return term, nil
		}
		return &expr{text: in.text + " AND " + term.text, pre: append(in.pre, term.pre...), deps: append(in.deps, term.deps...)}, nil
	case "coil":
		// Power continues to the right of a coil unchanged.
		in, err := l.input(e)
		if err != nil {
			return nil, err
		}
		if in == nil {
			return nil, fmt.Errorf("coil %d is not connected", e.ID)
		}
		return in, nil
	case "block":
		return l.blockOutput(e, c.Param)
	}
	return nil, fmt.Errorf("a wire cannot come from a %s", e.Kind())
}

// edge returns the output of an R_TRIG or F_TRIG declared for clk.
func (l *lowering) edge(kind, clk string) *expr {
	typ := "R_TRIG"
	if kind == "falling" {
		typ = "F_TRIG"
	}
	l.edges++
	name := l.unique(fmt.Sprintf("_%s%d", typ, l.edges))
	l.decls = append(l.decls, fmt.Sprintf("%s : %s;", name, typ))
	return &expr{text: name + ".Q", pre: []string{fmt.Sprintf("%s(CLK := %s);", name, clk)}}
}

// blockOutput is the value of a block's output: a function block's output
// variable, or a function's result.
func (l *lowering) blockOutput(e *Elem, param string) (*expr, error) {
	pin := findPin(e.Outputs, param)
	if pin == nil && param == "" && len(e.Outputs) > 0 {
		pin = &e.Outputs[0]
		for i := range e.Outputs { // a block's main output, not its ENO
			if !strings.EqualFold(e.Outputs[i].Param, "ENO") {
				pin = &e.Outputs[i]
				break
			}
		}
	}
	if l.isFB(e) {
		name := l.instance[e.ID]
		out := "OUT"
		if pin != nil {
			out = pin.Param
		} else if param != "" {
			out = param
		}
		text := name + "." + out
		if strings.EqualFold(out, "ENO") {
			flag, ok := l.enoFlag[e.ID]
			if !ok {
				return &expr{text: "TRUE", deps: []int{e.ID}}, nil // EN not wired: always enabled
			}
			text = flag
		}
		if pin != nil && pin.Negated {
			text = "NOT(" + text + ")"
		}
		return &expr{text: text, deps: []int{e.ID}}, nil
	}

	// A function: an expression of its inputs. With EN wired, its ENO is EN,
	// and its result may only be assigned, IF EN.
	var en *expr
	if p := findPin(e.Inputs, "EN"); p != nil && p.In != nil && len(p.In.Conns) > 0 {
		x, err := l.wires(p.In.Conns)
		if err != nil {
			return nil, err
		}
		if err := plain(x, "the EN of "+e.TypeName); err != nil {
			return nil, err
		}
		en = x
	}
	if strings.EqualFold(param, "ENO") {
		if en == nil {
			return &expr{text: "TRUE"}, nil
		}
		return en, nil
	}
	if param != "" && pin != nil && pin != &e.Outputs[0] && !strings.EqualFold(pin.Param, "OUT") {
		return nil, fmt.Errorf("output %s of function %s is not supported yet (only its result)", param, e.TypeName)
	}
	out := &expr{}
	var args []string
	for i := range e.Inputs {
		in := &e.Inputs[i]
		if strings.EqualFold(in.Param, "EN") {
			continue
		}
		if in.In == nil || len(in.In.Conns) == 0 {
			return nil, fmt.Errorf("input %s of %s is not connected", in.Param, e.TypeName)
		}
		x, err := l.wires(in.In.Conns)
		if err != nil {
			return nil, err
		}
		if x.guard != "" {
			// An EN/ENO chain: this function runs under the same enable,
			// so it may read the result inline, guarded as a whole.
			if en == nil || en.text != x.guard {
				if err := plain(x, "input "+in.Param+" of "+e.TypeName); err != nil {
					return nil, err
				}
			}
			x = &expr{text: x.text, pre: x.pre, deps: x.deps}
		}
		text := x.text
		if in.Edge == "rising" || in.Edge == "falling" {
			trig := l.edge(in.Edge, text)
			x.pre = append(x.pre, trig.pre...)
			text = trig.text
		}
		if in.Negated {
			text = "NOT(" + text + ")"
		}
		args = append(args, text)
		out.pre = append(out.pre, x.pre...)
		out.deps = append(out.deps, x.deps...)
	}
	text, err := functionText(e.TypeName, args)
	if err != nil {
		return nil, err
	}
	if pin != nil && pin.Negated {
		text = "NOT(" + text + ")"
	}
	out.text = text
	if en != nil {
		out.guard = en.text
		out.pre = append(en.pre, out.pre...)
		out.deps = append(out.deps, en.deps...)
	}
	return out, nil
}

// plain refuses the guarded result of a function with EN where it is not
// assigned: it would need a temporary of the function's type.
func plain(x *expr, where string) error {
	if x.guard != "" {
		return fmt.Errorf("%s reads the result of a function whose EN is wired; only a variable or coil may (not supported yet)", where)
	}
	return nil
}

// functionText writes a function of the given arguments: the standard
// arithmetic, logic and comparison functions as operators (extensible ones
// over all their inputs, comparisons chained with AND, as IEC 61131-3
// defines them), MOVE as its input, any other as a call.
func functionText(typ string, args []string) (string, error) {
	ops := map[string]string{"ADD": "+", "MUL": "*", "AND": "AND", "OR": "OR", "XOR": "XOR",
		"SUB": "-", "DIV": "/", "MOD": "MOD", "EXPT": "**"}
	cmps := map[string]string{"GT": ">", "GE": ">=", "EQ": "=", "NE": "<>", "LE": "<=", "LT": "<"}
	u := strings.ToUpper(typ)
	wrap := func(s string) string { return "(" + s + ")" }
	switch {
	case u == "MOVE":
		if len(args) != 1 {
			return "", fmt.Errorf("MOVE takes one input")
		}
		return args[0], nil
	case u == "NOT":
		if len(args) != 1 {
			return "", fmt.Errorf("NOT takes one input")
		}
		return "NOT(" + args[0] + ")", nil
	case u == "AND" && len(args) >= 2 && slices.Contains(args, "TRUE"):
		// TRUE AND x is x: a ladder's power rail ANDed with a function
		// contact's result, for one.
		kept := slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "TRUE" })
		switch len(kept) {
		case 0:
			return "TRUE", nil
		case 1:
			return kept[0], nil
		}
		return wrap(strings.Join(kept, " AND ")), nil
	case ops[u] != "":
		if len(args) < 2 || (len(args) > 2 && (u == "SUB" || u == "DIV" || u == "MOD" || u == "EXPT")) {
			return "", fmt.Errorf("%s with %d inputs", u, len(args))
		}
		return wrap(strings.Join(args, " "+ops[u]+" ")), nil
	case cmps[u] != "":
		if len(args) < 2 {
			return "", fmt.Errorf("%s with %d inputs", u, len(args))
		}
		var parts []string
		for i := 0; i+1 < len(args); i++ {
			parts = append(parts, wrap(args[i]+" "+cmps[u]+" "+args[i+1]))
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		return wrap(strings.Join(parts, " AND ")), nil
	}
	return typ + "(" + strings.Join(args, ", ") + ")", nil
}

// callStatement is a function block's call with its wired inputs.
func (l *lowering) callStatement(e *Elem) (*stmt, error) {
	name := l.instance[e.ID]
	s := &stmt{id: e.ID, elem: e}
	var args []string
	for i := range e.Inputs {
		in := &e.Inputs[i]
		if in.In == nil || len(in.In.Conns) == 0 {
			continue
		}
		x, err := l.wires(in.In.Conns)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", in.Param, err)
		}
		if err := plain(x, "input "+in.Param); err != nil {
			return nil, err
		}
		text := x.text
		if in.Edge == "rising" || in.Edge == "falling" {
			trig := l.edge(in.Edge, text)
			x.pre = append(x.pre, trig.pre...)
			text = trig.text
		}
		if in.Negated {
			text = "NOT(" + text + ")"
		}
		args = append(args, in.Param+" := "+text)
		s.lines = append(s.lines, x.pre...)
		s.deps = append(s.deps, x.deps...)
	}
	for i := range e.InOuts {
		io := &e.InOuts[i]
		if io.In == nil || len(io.In.Conns) == 0 {
			continue
		}
		if len(io.In.Conns) != 1 {
			return nil, fmt.Errorf("in/out %s takes one variable", io.Param)
		}
		src, ok := l.elems[io.In.Conns[0].Ref]
		if !ok || (src.Kind() != "inVariable" && src.Kind() != "inOutVariable") {
			return nil, fmt.Errorf("in/out %s must be wired to a variable", io.Param)
		}
		args = append(args, io.Param+" := "+strings.TrimSpace(src.Expression))
		if src.Kind() == "inOutVariable" {
			s.deps = append(s.deps, src.ID)
		}
	}
	if flag, ok := l.enoFlag[e.ID]; ok {
		args = append(args, "ENO => "+flag)
	}
	s.lines = append(s.lines, name+"("+strings.Join(args, ", ")+");")
	return s, nil
}

// order sorts the statements (see the file's comment).
func (l *lowering) order(stmts []*stmt) []*stmt {
	if len(stmts) == 0 {
		return nil
	}
	explicit := true
	for _, s := range stmts {
		if s.elem.Order <= 0 {
			explicit = false
			break
		}
	}
	if explicit {
		out := slices.Clone(stmts)
		slices.SortStableFunc(out, func(a, b *stmt) int { return a.elem.Order - b.elem.Order })
		return out
	}
	if l.oneNetwork {
		return l.dataflow(slices.Clone(stmts))
	}

	// Networks: elements joined by wires, power rails left out (they join
	// every rung of an LD body), connectors joined to their continuations.
	parent := map[int]int{}
	var find func(int) int
	find = func(x int) int {
		if p, ok := parent[x]; ok && p != x {
			r := find(p)
			parent[x] = r
			return r
		}
		parent[x] = x
		return x
	}
	union := func(a, b int) { parent[find(a)] = find(b) }
	rail := func(id int) bool {
		e := l.elems[id]
		return e != nil && (e.Kind() == "leftPowerRail" || e.Kind() == "rightPowerRail")
	}
	link := func(from int, conns []Conn) {
		for _, c := range conns {
			if !rail(from) && !rail(c.Ref) {
				union(from, c.Ref)
			}
		}
	}
	for _, e := range l.inDoc {
		find(e.ID)
		for _, in := range e.Ins {
			link(e.ID, in.Conns)
		}
		for _, pins := range [][]Pin{e.Inputs, e.InOuts} {
			for _, p := range pins {
				if p.In != nil {
					link(e.ID, p.In.Conns)
				}
			}
		}
		if e.Kind() == "continuation" {
			if c, ok := l.connectors[strings.ToUpper(e.Name)]; ok {
				union(e.ID, c.ID)
			}
		}
	}
	top := map[int]Pos{} // each network's top left element
	for _, e := range l.inDoc {
		r := find(e.ID)
		if p, ok := top[r]; !ok || before(e.Pos, p) {
			top[r] = e.Pos
		}
	}
	groups := map[int][]*stmt{}
	var roots []int
	for _, s := range stmts {
		r := find(s.id)
		if _, ok := groups[r]; !ok {
			roots = append(roots, r)
		}
		groups[r] = append(groups[r], s)
	}
	slices.SortStableFunc(roots, func(a, b int) int {
		if before(top[a], top[b]) {
			return -1
		}
		if before(top[b], top[a]) {
			return 1
		}
		return 0
	})
	var out []*stmt
	for _, r := range roots {
		out = append(out, l.dataflow(groups[r])...)
	}
	return out
}

// before orders positions top to bottom, then left to right, treating rows
// within 10 units as one, as drawing tools align them.
func before(a, b Pos) bool {
	if d := a.Y - b.Y; d < -10 || d > 10 {
		return a.Y < b.Y
	}
	return a.X < b.X
}

// dataflow orders one network's statements so each runs after those it
// reads, ties top to bottom; a loop (feedback through variables) is cut at
// its topmost statement, with a warning.
func (l *lowering) dataflow(group []*stmt) []*stmt {
	slices.SortStableFunc(group, func(a, b *stmt) int {
		if before(a.elem.Pos, b.elem.Pos) {
			return -1
		}
		if before(b.elem.Pos, a.elem.Pos) {
			return 1
		}
		return 0
	})
	in := map[int]*stmt{}
	for _, s := range group {
		in[s.id] = s
	}
	done := map[int]bool{}
	var out []*stmt
	for len(out) < len(group) {
		progressed := false
		for _, s := range group {
			if done[s.id] {
				continue
			}
			ready := true
			for _, d := range s.deps {
				if d != s.id && in[d] != nil && !done[d] {
					ready = false
					break
				}
			}
			if ready {
				done[s.id] = true
				out = append(out, s)
				progressed = true
				break // the first ready statement each time: position order where dataflow allows
			}
		}
		if !progressed {
			for _, s := range group { // a loop: cut it at its topmost statement
				if !done[s.id] {
					l.warnings = append(l.warnings, fmt.Sprintf("pou '%s': elements in a feedback loop; element %d runs first and reads the others' values from the previous scan", l.pou, s.id))
					done[s.id] = true
					out = append(out, s)
					break
				}
			}
		}
	}
	return out
}
