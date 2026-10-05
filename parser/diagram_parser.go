/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package parser

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/diagram"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/token"
)

// diagramBody reports whether a POU body starting here is a Ladder Diagram
// (LD, then RUNG or END_LD) or a Function Block Diagram (FBD, then a
// statement or END_FBD) in beedance's text form: "LD", "FBD" or "".
func (p *Parser) diagramBody() string {
	next := p.afterComments()
	switch {
	case p.curTokenIs(token.LD):
		if next.Type == token.IDENT && (strings.EqualFold(next.Literal, "RUNG") || strings.EqualFold(next.Literal, "END_LD")) {
			return "LD"
		}
	case p.curTokenIs(token.IDENT) && strings.EqualFold(p.curToken.Literal, "FBD"):
		if next.Type == token.IDENT { // an ST statement never starts with a name and a name
			return "FBD"
		}
	}
	return ""
}

// afterComments is the first token after the current one that is not a
// comment.
func (p *Parser) afterComments() token.Token {
	for _, t := range []token.Token{p.peekToken, p.peek2Token} {
		if t.Type != token.COMMENT {
			return t
		}
	}
	saved := *p.l
	defer func() { *p.l = saved }()
	for {
		if t := p.l.NextToken(); t.Type != token.COMMENT {
			return t
		}
	}
}

// DiagramBody is an LD or FBD body in the text form, as the parser found it.
type DiagramBody struct {
	POU  string // the PROGRAM, FUNCTION_BLOCK or FUNCTION
	Lang string // LD or FBD
	// Text is the body between the keyword and END_LD (END_FBD), starting
	// on source line Row.
	Text string
	Row  int
}

// Diagrams lists the LD and FBD bodies read in the text form.
func (p *Parser) Diagrams() []DiagramBody { return p.diagrams }

// parseDiagramBody reads an LD or FBD body in the text form and returns it
// lowered to Structured Text (package diagram), its statements on the
// source lines they came from. The variables the body needs besides the
// POU's own (edge detectors, instances it declares) are added to vars.
func (p *Parser) parseDiagramBody(lang string, pou *ast.Identifier, vars *[]*ast.VarDeclStatement, declared []string) *ast.BlockStatement {
	body := &ast.BlockStatement{Token: p.curToken}
	input := p.l.Input()
	start := p.curToken.Pos + len(p.curToken.Literal)
	row := p.curToken.Row
	end := diagram.End(input, start, "END_"+lang)
	if end < 0 {
		p.currentError("%s body without END_%s", lang, lang)
		end = len(input)
	}
	name := ""
	if pou != nil {
		name = pou.Value
	}
	p.diagrams = append(p.diagrams, DiagramBody{POU: name, Lang: lang, Text: input[start:end], Row: row})
	// Continue after END_LD (END_FBD).
	defer func() {
		p.l.Seek(min(end+len("END_")+len(lang), len(input)))
		p.peekToken = p.l.NextToken()
		p.peek2Token = p.l.NextToken()
		p.nextToken()
	}()

	low, err := diagram.LowerText(name, lang, input[start:end], row, diagram.Options{Declared: declared})
	if err != nil {
		p.errors = append(p.errors, err.Error())
		return body
	}

	// The lowered ST, wrapped in a program so it parses with its variables,
	// each statement on its source line: errors from here on point at the
	// diagram.
	var b strings.Builder
	b.WriteString(strings.Repeat("\n", row-1))
	b.WriteString("PROGRAM _diagram ")
	if len(low.Decls) > 0 {
		b.WriteString("VAR " + strings.Join(low.Decls, " ") + " END_VAR ")
	}
	at := row
	for i, line := range strings.Split(strings.TrimSuffix(low.Body, "\n"), "\n") {
		if line == "" {
			continue
		}
		for ; at < low.Rows[i]; at++ {
			b.WriteString("\n")
		}
		b.WriteString(line + " ")
	}
	b.WriteString("\nEND_PROGRAM\n")

	sub := New(lexer.New(b.String()))
	prog := sub.ParseProgram()
	if len(sub.Errors()) > 0 {
		p.errors = append(p.errors, sub.Errors()...)
		return body
	}
	if len(prog.Statements) != 1 {
		p.currentError("%s body did not lower to one block of statements", lang)
		return body
	}
	pd, ok := prog.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		p.currentError("%s body did not lower to statements", lang)
		return body
	}
	*vars = append(*vars, pd.Vars...)
	if blk, ok := pd.Body.(*ast.BlockStatement); ok {
		body.Statements = blk.Statements
	}
	return body
}

// declaredNames names the variables of declaration lists.
func declaredNames(name *ast.Identifier, lists ...[]*ast.VarDeclStatement) []string {
	var out []string
	if name != nil {
		out = append(out, name.Value)
	}
	for _, l := range lists {
		for _, v := range l {
			if v != nil && v.Name != nil {
				out = append(out, v.Name.Value)
			}
		}
	}
	return out
}

func externalVars(ext []*ast.ExternalVarDeclaration) []*ast.VarDeclStatement {
	var out []*ast.VarDeclStatement
	for _, e := range ext {
		out = append(out, e.Vars...)
	}
	return out
}

func tempVars(temp []*ast.TempVarDeclaration) []*ast.VarDeclStatement {
	var out []*ast.VarDeclStatement
	for _, t := range temp {
		out = append(out, t.Vars...)
	}
	return out
}
