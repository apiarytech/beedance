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

import (
	"strings"

	"github.com/apiarytech/beedance/token"
)

// resolveAll resolves every name the source uses, left to right, so a
// member's left side is resolved before it.
func (ix *Index) resolveAll() {
	for i, t := range ix.toks {
		if !isWord(t.Literal) {
			continue
		}
		keyword := t.Type != token.IDENT
		if sym := ix.resolve(i, keyword); sym != nil {
			ix.refs[i] = sym
		}
	}
}

// resolve names the symbol token i stands for, or nil. A keyword that may
// also be a name (S, R, EN, IN) is only a declared variable.
func (ix *Index) resolve(i int, keyword bool) *Symbol {
	t := ix.toks[i]
	prev := ix.tok(i - 1)
	next := ix.tok(i + 1)
	switch {
	case prev.Type == token.DOT:
		if m := ix.members(ix.leftOf(i - 1)); m != nil {
			return m.lookup(ix, t.Literal)
		}
		return nil
	case prev.Type == token.HASH: // Color#Red, INT#5
		if keyword {
			return nil
		}
		return ix.enums[strings.ToUpper(t.Literal)]
	case next.Type == token.HASH: // the type of a typed literal
		if keyword {
			return nil
		}
		return ix.named[strings.ToUpper(t.Literal)]
	}
	// A named argument, IN := x or Q => y: the callee's input or output.
	if next.Type == token.ASSIGN || next.Type == token.ARROW {
		if callee := ix.callee(i); callee != nil {
			if sym := callee.lookup(ix, t.Literal); sym != nil {
				return sym
			}
		}
	}
	s := ix.scopeAt(t.Pos)
	for c := s; c != nil; c = c.parent {
		if sym := c.lookup(ix, t.Literal); sym != nil {
			return sym
		}
	}
	if keyword {
		return nil
	}
	key := strings.ToUpper(t.Literal)
	if sym := ix.glob[key]; sym != nil {
		return sym
	}
	if sym := ix.named[key]; sym != nil {
		return sym
	}
	if sym := ix.enums[key]; sym != nil {
		return sym
	}
	if s := ix.standardFB(key); s != nil {
		return s.sym
	}
	return ix.standardFunction(key)
}

// lookup finds name among s's own names and, for a function block, those
// of the blocks it extends.
func (s *scope) lookup(ix *Index, name string) *Symbol {
	seen := map[*scope]bool{}
	for c := s; c != nil && !seen[c]; {
		seen[c] = true
		if sym := c.vars[strings.ToUpper(name)]; sym != nil {
			return sym
		}
		if c.sym.base == "" {
			break
		}
		base := ix.named[strings.ToUpper(c.sym.base)]
		if base == nil {
			break
		}
		c = base.scope
	}
	return nil
}

// scopeAt is the innermost POU or method whose text holds offset off.
func (ix *Index) scopeAt(off int) *scope {
	var best *scope
	for _, s := range ix.pous {
		if s.start <= off && off < s.end && (best == nil || s.start >= best.start) {
			best = s
		}
	}
	return best
}

// leftOf is the symbol of what stands left of the dot at token d: a name,
// an array's element (a[i].x), a dereference (p^.x).
func (ix *Index) leftOf(d int) (*Symbol, bool) {
	j := d - 1
	element := false
	if ix.tok(j).Type == token.CARET {
		j--
	}
	if ix.tok(j).Type == token.RBRACKET {
		depth := 0
		for ; j >= 0; j-- {
			switch ix.toks[j].Type {
			case token.RBRACKET:
				depth++
			case token.LBRACKET:
				depth--
			}
			if depth == 0 {
				break
			}
		}
		j--
		element = true
	}
	if j < 0 {
		return nil, false
	}
	return ix.refs[j], element
}

// members is the scope of the members of sym's type; element: of its
// array's element type.
func (ix *Index) members(sym *Symbol, element bool) *scope {
	if sym == nil {
		return nil
	}
	switch sym.Kind {
	case Program, FunctionBlock, Interface:
		return sym.scope
	case Step:
		return stepMembers
	case Variable, Property:
		t := sym.Type
		if element {
			t = elementType(t)
		}
		return ix.typeScope(t, 0)
	}
	return nil
}

// typeScope is the scope of the members of type t.
func (ix *Index) typeScope(t string, depth int) *scope {
	t = strings.TrimSpace(t)
	for _, p := range []string{"POINTER TO ", "REFERENCE TO "} {
		if strings.HasPrefix(strings.ToUpper(t), p) {
			t = strings.TrimSpace(t[len(p):])
		}
	}
	key := strings.ToUpper(t)
	if s := ix.mem[key]; s != nil {
		return s
	}
	if sym := ix.named[key]; sym != nil {
		if sym.scope != nil {
			return sym.scope
		}
		if sym.Kind == Type && depth < 8 && sym.Type != "" && !strings.EqualFold(sym.Type, t) {
			return ix.typeScope(sym.Type, depth+1) // an alias
		}
		return nil
	}
	return ix.standardFB(key)
}

// elementType is the element type of an ARRAY type, written as declared.
func elementType(t string) string {
	u := strings.ToUpper(t)
	if i := strings.LastIndex(u, " OF "); i >= 0 && strings.HasPrefix(strings.TrimSpace(u), "ARRAY") {
		return strings.TrimSpace(t[i+4:])
	}
	return t
}

// stepMembers are an SFC step's flags, Step.X and Step.T.
var stepMembers = func() *scope {
	s := &scope{sym: &Symbol{Name: "STEP", Kind: Type, Standard: true}, vars: map[string]*Symbol{}}
	s.add(&Symbol{Name: "X", Kind: Variable, Type: "BOOL", Standard: true, Detail: "X : BOOL; (* the step is active *)"})
	s.add(&Symbol{Name: "T", Kind: Variable, Type: "TIME", Standard: true, Detail: "T : TIME; (* how long the step has been active *)"})
	return s
}()

// callee is the scope of the function or block whose call's parentheses
// hold token i: t1(IN := x), TON(PT := T#5S) in a diagram, F(a := 1).
func (ix *Index) callee(i int) *scope {
	depth := 0
	for j := i - 1; j >= 0 && i-j < 400; j-- {
		t := ix.toks[j]
		switch {
		case t.Type == token.RPAREN:
			depth++
		case t.Type == token.LPAREN:
			if depth > 0 {
				depth--
				continue
			}
			name := ix.tok(j - 1)
			if !isWord(name.Literal) {
				return nil
			}
			sym := ix.refs[j-1]
			if sym == nil {
				return nil
			}
			switch sym.Kind {
			case FunctionBlock, Function, Program, Method:
				return sym.scope
			case Variable:
				return ix.typeScope(sym.Type, 0)
			}
			return nil
		case t.Type == token.SEMICOLON || strings.HasPrefix(string(t.Type), "END_"):
			return nil
		}
	}
	return nil
}

func (ix *Index) tok(i int) token.Token {
	if i < 0 || i >= len(ix.toks) {
		return token.Token{}
	}
	return ix.toks[i]
}

// isWord says whether s is written like an identifier.
func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '_' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
