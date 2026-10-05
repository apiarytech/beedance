/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram

// This file reads beedance's text form of LD and FBD bodies into the same
// elements as PLCopen XML, so both forms lower alike. The body of a POU is
// written between LD and END_LD, or FBD and END_FBD; doc/language.md has
// the grammar. In short:
//
//	LD
//	  RUNG sealin (* comments anywhere *)
//	    [ Start | Run ] /Stop ( Run )
//	  RUNG delay
//	    Run t1:TON(PT := T#5S) ( Done )
//	END_LD
//
//	FBD
//	  hot = GT(Temp, Limit)            // a named wire
//	  t1 : TON(IN := hot, PT := T#5S)  // an instance, called
//	  Alarm := OR(t1.Q, Alarm)         // a variable written
//	END_FBD
//
// The layout is not stored: an element's position is its line and column.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Text is a body read from the text form: its elements, positioned at
// their line and column, and the instances it declares.
type Text struct {
	Lang  string // LD or FBD
	Elems []*Elem
	// Rows gives each element's source line.
	Rows map[int]int
	// Declares are the instances declared by name : TYPE, as name and type.
	Declares [][2]string
}

// ParseText reads a body in the text form, lang "LD" or "FBD". src is the
// text between the LD (or FBD) keyword and END_LD (END_FBD), and line is
// the source line src starts on.
func ParseText(lang, src string, line int) (*Text, error) {
	b := &builder{lang: strings.ToUpper(lang), rows: map[int]int{}, instances: map[string]*Elem{}, called: map[string]int{},
		wires: map[string]bool{}, pending: map[int]*Elem{}}
	toks, err := scan(src, line)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.lang, err)
	}
	b.toks = toks
	switch b.lang {
	case "LD":
		err = b.ladder()
	case "FBD":
		err = b.netlist()
	default:
		err = fmt.Errorf("unknown diagram language %q", lang)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.lang, err)
	}
	return &Text{Lang: b.lang, Elems: b.elems, Rows: b.rows, Declares: b.declares}, nil
}

// LowerText lowers a body in the text form (ParseText) to Structured Text.
// The result's Rows give the source line of each line of Body, and its
// Decls include the instances the body declares (name : TYPE(...)) that
// opt.Declared lacks.
func LowerText(pou, lang, src string, line int, opt Options) (*Lowered, error) {
	t, err := ParseText(lang, src, line)
	if err != nil {
		return nil, fmt.Errorf("pou '%s': %w", pou, err)
	}
	if t.Lang == "FBD" {
		opt.OneNetwork = true
	}
	declared := map[string]bool{}
	for _, d := range opt.Declared {
		declared[strings.ToUpper(d)] = true
	}
	var decls []string
	for _, d := range t.Declares {
		if !declared[strings.ToUpper(d[0])] {
			declared[strings.ToUpper(d[0])] = true
			decls = append(decls, d[0]+" : "+d[1]+";")
			opt.Declared = append(opt.Declared, d[0])
		}
	}
	low, err := Lower(pou, t.Elems, opt)
	if err != nil {
		// Elements have no number in the text: say where they are.
		msg := elementRef.ReplaceAllStringFunc(err.Error(), func(m string) string {
			id, _ := strconv.Atoi(elementRef.FindStringSubmatch(m)[1])
			if r, ok := t.Rows[id]; ok {
				return fmt.Sprintf("line %d", r)
			}
			return m
		})
		return nil, fmt.Errorf("%s", msg)
	}
	low.Decls = append(decls, low.Decls...)
	for _, id := range low.Lines {
		low.Rows = append(low.Rows, t.Rows[id])
	}
	return low, nil
}

var elementRef = regexp.MustCompile(`element (\d+)`)

// ---- scanning ----

type tokKind int

const (
	tEOF tokKind = iota
	tNewline
	tIdent   // a name
	tLiteral // a number, string or typed literal (T#5S, 16#FF, INT#3)
	tPunct   // ( ) [ ] | , ; . : := => = / + -
)

type tok struct {
	kind      tokKind
	text      string
	line, col int
}

func (t tok) String() string {
	switch t.kind {
	case tEOF:
		return "the end of the body"
	case tNewline:
		return "the end of the line"
	}
	return fmt.Sprintf("%q", t.text)
}

// scan splits src into tokens; comments are dropped, ends of lines kept.
func scan(src string, line int) ([]tok, error) {
	var out []tok
	col := 1
	for i := 0; i < len(src); {
		c := src[i]
		start := tok{line: line, col: col}
		advance := func(n int) {
			for k := 0; k < n && i < len(src); k++ {
				if src[i] == '\n' {
					line++
					col = 1
				} else {
					col++
				}
				i++
			}
		}
		switch {
		case c == '\n':
			start.kind = tNewline
			out = append(out, start)
			advance(1)
		case c == ' ' || c == '\t' || c == '\r':
			advance(1)
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				advance(1)
			}
		case strings.HasPrefix(src[i:], "(*"):
			depth := 0
			for {
				if i >= len(src) {
					return nil, fmt.Errorf("line %d: comment not closed", start.line)
				}
				if strings.HasPrefix(src[i:], "(*") {
					depth++
					advance(2)
					continue
				}
				if strings.HasPrefix(src[i:], "*)") {
					depth--
					advance(2)
					if depth == 0 {
						break
					}
					continue
				}
				advance(1)
			}
		case c == '\'' || c == '"':
			j := i + 1
			for ; j < len(src); j++ {
				if src[j] == c {
					if j+1 < len(src) && src[j+1] == c {
						j++
						continue
					}
					break
				}
				if src[j] == '\n' {
					return nil, fmt.Errorf("line %d: string not closed", start.line)
				}
			}
			if j >= len(src) {
				return nil, fmt.Errorf("line %d: string not closed", start.line)
			}
			start.kind, start.text = tLiteral, src[i:j+1]
			out = append(out, start)
			advance(j + 1 - i)
		case isLetter(c) || isDigit(c):
			j := i
			for j < len(src) && (isLetter(src[j]) || isDigit(src[j])) {
				j++
			}
			kind := tIdent
			if isDigit(c) {
				kind = tLiteral
				j = number(src, i)
			}
			if j < len(src) && src[j] == '#' { // T#5S, INT#-3, 16#FF
				kind = tLiteral
				j++
				for j < len(src) && (isLetter(src[j]) || isDigit(src[j]) || strings.IndexByte(".:#", src[j]) >= 0 ||
					((src[j] == '-' || src[j] == '+') && strings.IndexByte("#eE", src[j-1]) >= 0)) {
					j++
				}
			}
			start.kind, start.text = kind, src[i:j]
			out = append(out, start)
			advance(j - i)
		default:
			n := 1
			if strings.HasPrefix(src[i:], ":=") || strings.HasPrefix(src[i:], "=>") {
				n = 2
			} else if strings.IndexByte("()[]|,;.:=/+-", c) < 0 {
				return nil, fmt.Errorf("line %d, column %d: unexpected %q", start.line, start.col, string(c))
			}
			start.kind, start.text = tPunct, src[i:i+n]
			out = append(out, start)
			advance(n)
		}
	}
	return append(out, tok{kind: tEOF, line: line, col: col}), nil
}

// number returns the end of the number starting at i: digits, '_', a
// fraction and an exponent.
func number(src string, i int) int {
	j := i
	digits := func() {
		for j < len(src) && (isDigit(src[j]) || src[j] == '_') {
			j++
		}
	}
	digits()
	if j+1 < len(src) && src[j] == '.' && isDigit(src[j+1]) {
		j++
		digits()
	}
	if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
		k := j + 1
		if k < len(src) && (src[k] == '+' || src[k] == '-') {
			k++
		}
		if k < len(src) && isDigit(src[k]) {
			j = k
			digits()
		}
	}
	return j
}

func isLetter(c byte) bool { return c == '_' || (c|0x20 >= 'a' && c|0x20 <= 'z') }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

// ---- building the elements ----

type builder struct {
	lang      string
	toks      []tok
	pos       int
	elems     []*Elem
	rows      map[int]int      // element -> source line
	instances map[string]*Elem // upper-case instance name -> its one call
	called    map[string]int   // upper-case instance name -> calls
	wires     map[string]bool  // FBD: upper-case wire names
	declares  [][2]string      // instances declared by name : TYPE
	pending   map[int]*Elem    // FBD: token index of a call -> its block
	wireReads bool             // FBD: an instance output read is wired from its call
}

func (b *builder) peek() tok { return b.toks[b.pos] }

func (b *builder) next() tok {
	t := b.toks[b.pos]
	if t.kind != tEOF {
		b.pos++
	}
	return t
}

func (b *builder) is(text string) bool {
	t := b.peek()
	return (t.kind == tPunct || t.kind == tIdent) && strings.EqualFold(t.text, text)
}

func (b *builder) expect(text string) (tok, error) {
	t := b.next()
	if (t.kind != tPunct && t.kind != tIdent) || !strings.EqualFold(t.text, text) {
		return t, b.errorf(t, "expected %q, found %s", text, t)
	}
	return t, nil
}

func (b *builder) errorf(t tok, format string, args ...any) error {
	return fmt.Errorf("line %d, column %d: %s", t.line, t.col, fmt.Sprintf(format, args...))
}

// skipLines skips ends of lines; inside parentheses and brackets they mean
// nothing, and in LD only RUNG starts a rung.
func (b *builder) skipLines() {
	for b.peek().kind == tNewline {
		b.next()
	}
}

func (b *builder) add(kind string, at tok) *Elem {
	e := &Elem{ID: len(b.elems) + 1, Pos: Pos{X: float64(at.col), Y: float64(at.line) * 20}}
	e.XMLName.Local = kind
	b.elems = append(b.elems, e)
	b.rows[e.ID] = at.line
	return e
}

func wire(conns ...Conn) *PointIn { return &PointIn{Conns: conns} }

// reference reads a variable reference: a name with members and indices,
// as one text (a.b[i, 2].c).
func (b *builder) reference() (string, tok, error) {
	t := b.next()
	if t.kind != tIdent {
		return "", t, b.errorf(t, "expected a variable, found %s", t)
	}
	text := t.text
	for {
		switch {
		case b.is("."):
			b.next()
			m := b.next()
			if m.kind != tIdent && m.kind != tLiteral { // a.b or a word's bit a.3
				return "", m, b.errorf(m, "expected a member after '.', found %s", m)
			}
			text += "." + m.text
		case b.is("[") && b.touching(): // a[i]; in a rung, A [ B | C ] is a contact and a branch
			b.next()
			depth := 1
			var idx []string
			for depth > 0 {
				x := b.next()
				switch {
				case x.kind == tEOF:
					return "", x, b.errorf(t, "'[' not closed")
				case x.kind == tNewline:
					continue
				case x.text == "[":
					depth++
				case x.text == "]":
					depth--
					if depth == 0 {
						continue
					}
				}
				idx = append(idx, x.text)
			}
			text += "[" + strings.Join(idx, " ") + "]"
		default:
			return text, t, nil
		}
	}
}

// expression reads an input's value and returns the wire carrying it.
func (b *builder) expression() (Conn, error) {
	b.skipLines()
	t := b.peek()
	switch {
	case t.kind == tLiteral:
		b.next()
		return b.variable(t.text, t), nil
	case t.kind == tPunct && t.text == "(":
		b.next()
		in, err := b.expression()
		if err != nil {
			return Conn{}, err
		}
		b.skipLines()
		_, err = b.expect(")")
		return in, err
	case t.kind == tPunct && (t.text == "-" || t.text == "+"):
		b.next()
		n := b.next()
		if n.kind != tLiteral {
			return Conn{}, b.errorf(n, "expected a number after %q", t.text)
		}
		return b.variable(t.text+n.text, t), nil
	case t.kind == tIdent && strings.EqualFold(t.text, "NOT"):
		b.next()
		in, err := b.expression()
		if err != nil {
			return Conn{}, err
		}
		e := b.add("block", t)
		e.TypeName = "NOT"
		e.Inputs = []Pin{{Param: "IN", In: wire(in)}}
		e.Outputs = []Pin{{Param: "OUT"}}
		return Conn{Ref: e.ID}, nil
	case t.kind == tIdent && b.toks[b.pos+1].text == "(":
		return b.function(false)
	case t.kind == tIdent:
		if b.wires[strings.ToUpper(t.text)] {
			b.next()
			e := b.add("continuation", t)
			e.Name = t.text
			return Conn{Ref: e.ID}, nil
		}
		ref, at, err := b.reference()
		if err != nil {
			return Conn{}, err
		}
		// An output of an instance this body calls (once) is wired from it.
		if head, pin, ok := strings.Cut(ref, "."); ok && !strings.ContainsAny(pin, ".[") {
			if blk := b.instances[strings.ToUpper(head)]; b.wireReads && blk != nil && b.called[strings.ToUpper(head)] == 1 {
				return Conn{Ref: blk.ID, Param: pin}, nil
			}
		}
		return b.variable(ref, at), nil
	}
	return Conn{}, b.errorf(t, "expected a value, found %s", t)
}

func (b *builder) variable(text string, at tok) Conn {
	e := b.add("inVariable", at)
	e.Expression = text
	return Conn{Ref: e.ID}
}

// function reads a function call, its inputs positional (EN := x may come
// first); negated marks its result negated.
func (b *builder) function(negated bool) (Conn, error) {
	name := b.next()
	if isStandardFB(name.text) {
		return Conn{}, b.errorf(name, "%s is a function block: give it an instance, name:%s(...)", name.text, strings.ToUpper(name.text))
	}
	b.next() // (
	e := b.add("block", name)
	e.TypeName = name.text
	e.Outputs = []Pin{{Param: "OUT", Negated: negated}}
	n := 0
	for {
		b.skipLines()
		if b.is(")") {
			b.next()
			break
		}
		if len(e.Inputs) > 0 {
			if _, err := b.expect(","); err != nil {
				return Conn{}, err
			}
			b.skipLines()
		}
		param := ""
		if t := b.peek(); t.kind == tIdent && b.toks[b.pos+1].text == ":=" {
			if !strings.EqualFold(t.text, "EN") || len(e.Inputs) > 0 {
				return Conn{}, b.errorf(t, "the inputs of function %s are positional (only EN := may be named, first)", name.text)
			}
			b.next()
			b.next()
			param = "EN"
		}
		in, err := b.expression()
		if err != nil {
			return Conn{}, err
		}
		if param == "" {
			n++
			param = fmt.Sprintf("IN%d", n)
		}
		e.Inputs = append(e.Inputs, Pin{Param: param, In: wire(in)})
	}
	if n == 0 {
		return Conn{}, b.errorf(name, "function %s without inputs", name.text)
	}
	return Conn{Ref: e.ID}, nil
}

// call reads a function block call's arguments, after the instance's name
// (and type): IN := value inputs, OUT => variable outputs. power, when not
// "", is the input the rung drives, which may not be given.
func (b *builder) call(e *Elem, power string) error {
	if !b.is("(") {
		return nil
	}
	b.next()
	first := true
	for {
		b.skipLines()
		if b.is(")") {
			b.next()
			return nil
		}
		if !first {
			if _, err := b.expect(","); err != nil {
				return err
			}
			b.skipLines()
		}
		first = false
		pin := b.next()
		if pin.kind != tIdent {
			return b.errorf(pin, "expected an input or output of %s, found %s", e.InstanceName, pin)
		}
		op := b.next()
		switch op.text {
		case ":=":
			if strings.EqualFold(pin.text, power) {
				return b.errorf(pin, "%s of %s is driven by the rung; it may not be given", pin.text, e.InstanceName)
			}
			in, err := b.expression()
			if err != nil {
				return err
			}
			e.Inputs = append(e.Inputs, Pin{Param: pin.text, In: wire(in)})
		case "=>":
			target, at, err := b.reference()
			if err != nil {
				return err
			}
			out := b.add("outVariable", at)
			out.Expression = target
			out.Ins = []PointIn{{Conns: []Conn{{Ref: e.ID, Param: pin.text}}}}
			e.Outputs = append(e.Outputs, Pin{Param: pin.text})
		default:
			return b.errorf(op, "expected := or => after %s, found %s (the inputs of a function block are named)", pin.text, op)
		}
	}
}

// instance makes the block of a function block call at name.
func (b *builder) instance(name tok) *Elem {
	e := b.add("block", name)
	e.InstanceName = name.text
	key := strings.ToUpper(name.text)
	b.called[key]++
	b.instances[key] = e
	return e
}

// callHead reads the instance's name and, after a colon, its type, which
// declares it.
func (b *builder) callHead(e *Elem) error {
	b.next() // the name
	if !b.is(":") {
		return nil
	}
	b.next()
	typ := b.next()
	if typ.kind != tIdent {
		return b.errorf(typ, "expected the type of %s, found %s", e.InstanceName, typ)
	}
	e.TypeName = typ.text
	b.declares = append(b.declares, [2]string{e.InstanceName, typ.text})
	return nil
}

// callStatement reads an FBD statement calling a function block.
func (b *builder) callStatement(e *Elem) error {
	if err := b.callHead(e); err != nil {
		return err
	}
	if !b.is("(") && e.TypeName == "" {
		return b.errorf(b.peek(), "expected the inputs of %s, found %s", e.InstanceName, b.peek())
	}
	return b.call(e, "")
}

func isStandardFB(name string) bool {
	for _, s := range StandardFunctionBlocks {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

// ---- FBD ----

// netlist reads an FBD body: one statement per line (or ;).
func (b *builder) netlist() error {
	b.wireReads = true
	// Wires are named before they are read: find them first.
	depth := 0
	for i, t := range b.toks {
		switch t.text {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
		case "=":
			if depth == 0 && t.kind == tPunct && i > 0 && b.toks[i-1].kind == tIdent && (i == 1 || b.toks[i-2].kind == tNewline || b.toks[i-2].text == ";") {
				b.wires[strings.ToUpper(b.toks[i-1].text)] = true
			}
		}
	}
	// Instances are called before their outputs are read: make each call's
	// block first, so a read wires to it wherever the call is.
	depth = 0
	for i, t := range b.toks {
		switch t.text {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
		}
		start := i == 0 || b.toks[i-1].kind == tNewline || b.toks[i-1].text == ";"
		if depth == 0 && start && t.kind == tIdent && (b.toks[i+1].text == ":" || b.toks[i+1].text == "(") {
			b.pending[i] = b.instance(t)
		}
	}
	for {
		for b.peek().kind == tNewline || b.is(";") {
			b.next()
		}
		t := b.peek()
		if t.kind == tEOF {
			return nil
		}
		if t.kind != tIdent {
			return b.errorf(t, "expected a statement, found %s", t)
		}
		after := b.toks[b.pos+1].text
		switch {
		case after == "=" && b.wires[strings.ToUpper(t.text)]:
			b.next()
			b.next()
			in, err := b.expression()
			if err != nil {
				return err
			}
			e := b.add("connector", t)
			e.Name = t.text
			e.Ins = []PointIn{{Conns: []Conn{in}}}
		case after == ":" || after == "(":
			if err := b.callStatement(b.pending[b.pos]); err != nil {
				return err
			}
		default:
			target, at, err := b.reference()
			if err != nil {
				return err
			}
			if _, err := b.expect(":="); err != nil {
				return err
			}
			in, err := b.expression()
			if err != nil {
				return err
			}
			e := b.add("outVariable", at)
			e.Expression = target
			e.Ins = []PointIn{{Conns: []Conn{in}}}
		}
		if t := b.peek(); t.kind != tNewline && t.kind != tEOF && !b.is(";") {
			return b.errorf(t, "expected the end of the statement, found %s", t)
		}
	}
}

// ---- LD ----

// powerPins are the input a rung drives and the output it continues from,
// for the standard function blocks; any other block's are EN and ENO.
var powerPins = map[string][2]string{
	"TON": {"IN", "Q"}, "TOF": {"IN", "Q"}, "TP": {"IN", "Q"},
	"CTU": {"CU", "Q"}, "CTD": {"CD", "Q"}, "CTUD": {"CU", "QU"},
	"R_TRIG": {"CLK", "Q"}, "F_TRIG": {"CLK", "Q"},
	"SR": {"S1", "Q1"}, "RS": {"S", "Q1"},
}

// ladder reads an LD body: rungs, each RUNG [name] then its elements left
// to right, coils last.
func (b *builder) ladder() error {
	for {
		b.skipLines()
		t := b.peek()
		if t.kind == tEOF {
			return nil
		}
		if !b.is("RUNG") {
			return b.errorf(t, "expected RUNG, found %s", t)
		}
		b.next()
		name := ""
		if b.peek().kind == tIdent && !b.is("RUNG") {
			name = b.next().text
		}
		if err := b.rung(t, name); err != nil {
			return err
		}
	}
}

func (b *builder) rung(at tok, name string) error {
	rail := b.add("leftPowerRail", at)
	rail.Name = name // the rung's, for a drawing
	power, block, err := b.series([]Conn{{Ref: rail.ID}}, false)
	if err != nil {
		return err
	}
	coils := 0
	for b.skipLines(); b.is("("); b.skipLines() {
		if err := b.coil(power); err != nil {
			return err
		}
		coils++
	}
	if t := b.peek(); t.kind != tEOF && !b.is("RUNG") {
		if coils > 0 {
			return b.errorf(t, "coils end a rung; found %s after them", t)
		}
		return b.errorf(t, "expected a contact, block or coil, found %s", t)
	}
	if coils == 0 && !block {
		return b.errorf(at, "the rung ends in neither a coil nor a function block")
	}
	return nil
}

// series reads elements in series from power until a coil, the end of a
// rung or, in a branch, '|' or ']'; it returns the power after them and
// whether the last element was a function block.
func (b *builder) series(power []Conn, inBranch bool) ([]Conn, bool, error) {
	block := false
	for {
		b.skipLines()
		t := b.peek()
		switch {
		case t.kind == tEOF, b.is("RUNG"), b.is("("):
			if inBranch {
				return nil, false, b.errorf(t, "'[' not closed by ']'")
			}
			return power, block, nil
		case b.is("|"), b.is("]"):
			if !inBranch {
				return nil, false, b.errorf(t, "%s outside a branch", t)
			}
			return power, block, nil
		case b.is("["):
			b.next()
			var outs []Conn
			for {
				leg, _, err := b.series(power, true)
				if err != nil {
					return nil, false, err
				}
				outs = append(outs, leg...)
				if b.next().text == "]" {
					break
				}
			}
			power, block = outs, false
		default:
			next, isBlock, err := b.element(power)
			if err != nil {
				return nil, false, err
			}
			power, block = next, isBlock
		}
	}
}

// element reads a contact, a function contact or a function block.
func (b *builder) element(power []Conn) ([]Conn, bool, error) {
	t := b.peek()
	prefix := ""
	if t.kind == tPunct && (t.text == "/" || t.text == "+" || t.text == "-") {
		prefix = b.next().text
	}
	name := b.peek()
	if name.kind != tIdent {
		return nil, false, b.errorf(name, "expected a contact, block or coil, found %s", name)
	}
	after := b.toks[b.pos+1].text
	switch {
	case after == ":" && prefix == "":
		e := b.instance(name)
		if err := b.callHead(e); err != nil {
			return nil, false, err
		}
		if e.TypeName == "" {
			return nil, false, b.errorf(name, "a block in a rung is written name:TYPE(...)")
		}
		pins, ok := powerPins[strings.ToUpper(e.TypeName)]
		if !ok {
			pins = [2]string{"EN", "ENO"}
		}
		e.Inputs = append(e.Inputs, Pin{Param: pins[0], In: wire(power...)})
		if err := b.call(e, pins[0]); err != nil {
			return nil, false, err
		}
		return []Conn{{Ref: e.ID, Param: pins[1]}}, true, nil
	case after == "(" && b.adjacent():
		if prefix == "+" || prefix == "-" {
			return nil, false, b.errorf(t, "an edge contact reads a variable, not a function")
		}
		fc, err := b.function(prefix == "/")
		if err != nil {
			return nil, false, err
		}
		// For a drawing: the function stands in the rung, powered through its
		// EN (Ins, which lowering does not read). The AND below is lowering's.
		f := b.elems[fc.Ref-1]
		f.Name = contactFunction
		f.Ins = []PointIn{{Conns: power}}
		if len(power) == 1 && b.elems[power[0].Ref-1].Kind() == "leftPowerRail" {
			return []Conn{fc}, false, nil
		}
		and := b.add("block", name)
		and.TypeName = "AND"
		and.Name = contactAnd
		and.Inputs = []Pin{{Param: "IN1", In: wire(power...)}, {Param: "IN2", In: wire(fc)}}
		and.Outputs = []Pin{{Param: "OUT"}}
		return []Conn{{Ref: and.ID}}, false, nil
	}
	ref, at, err := b.reference()
	if err != nil {
		return nil, false, err
	}
	e := b.add("contact", at)
	e.Variable = ref
	e.Ins = []PointIn{{Conns: power}}
	switch prefix {
	case "/":
		e.Negated = true
	case "+":
		e.Edge = "rising"
	case "-":
		e.Edge = "falling"
	}
	return []Conn{{Ref: e.ID}}, false, nil
}

// coil reads ( [/ | S | R | P | N] variable ).
func (b *builder) coil(power []Conn) error {
	open := b.next()
	e := b.add("coil", open)
	e.Ins = []PointIn{{Conns: power}}
	if b.is("/") {
		b.next()
		e.Negated = true
	} else if t := b.peek(); t.kind == tIdent && b.toks[b.pos+1].kind == tIdent {
		switch strings.ToUpper(t.text) {
		case "S":
			e.Storage = "set"
		case "R":
			e.Storage = "reset"
		case "P":
			e.Edge = "rising"
		case "N":
			e.Edge = "falling"
		default:
			return b.errorf(t, "unknown coil %s (S, R, P, N or /)", t.text)
		}
		b.next()
	}
	ref, _, err := b.reference()
	if err != nil {
		return err
	}
	e.Variable = ref
	_, err = b.expect(")")
	return err
}

// adjacent reports whether the next token (a name) is followed by '('
// directly, as in EQ(A, B): in a rung, A ( B ) is a contact and a coil.
func (b *builder) adjacent() bool {
	n, p := b.toks[b.pos], b.toks[b.pos+1]
	return p.text == "(" && p.line == n.line && p.col == n.col+len(n.text)
}

// End returns the offset of word (END_LD or END_FBD), as a whole word
// outside comments and strings, in src from offset start; -1 if it is
// missing.
func End(src string, start int, word string) int {
	for i := start; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "(*"):
			depth := 0
			for i < len(src) {
				if strings.HasPrefix(src[i:], "(*") {
					depth++
					i += 2
				} else if strings.HasPrefix(src[i:], "*)") {
					depth--
					i += 2
					if depth == 0 {
						break
					}
				} else {
					i++
				}
			}
		case src[i] == '\'' || src[i] == '"':
			q := src[i]
			for i++; i < len(src) && src[i] != q && src[i] != '\n'; i++ {
			}
			i++
		case isLetter(src[i]):
			j := i
			for j < len(src) && (isLetter(src[j]) || isDigit(src[j])) {
				j++
			}
			if strings.EqualFold(src[i:j], word) {
				return i
			}
			i = j
		default:
			i++
		}
	}
	return -1
}

// touching reports whether the next token follows the last one read with
// no space between.
func (b *builder) touching() bool {
	if b.pos == 0 {
		return false
	}
	p, n := b.toks[b.pos-1], b.toks[b.pos]
	return p.line == n.line && p.col+len(p.text) == n.col
}

// Marks, in Elem.Name, of the blocks a function contact makes in a rung:
// the function, and the AND of its result with the rung's power.
const (
	contactFunction = "_contact"
	contactAnd      = "_contactAnd"
)
