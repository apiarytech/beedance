/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/diagram"
	"github.com/apiarytech/beedance/parser"
)

// Lowered is a graphical body as Structured Text.
type Lowered = diagram.Lowered

// StandardFunctionBlocks are the IEC 61131-3 function blocks: a block of
// one of these types without an instance name gets one.
var StandardFunctionBlocks = diagram.StandardFunctionBlocks

// LowerGraphical lowers an FBD or LD body, given as the inner XML of its
// <FBD> or <LD> element, to Structured Text (package diagram). fbTypes
// names the function block types (the standard ones and the project's,
// any case); declared names the POU's variables, so generated names avoid
// them.
func LowerGraphical(pou, innerXML string, fbTypes, declared []string) (*Lowered, error) {
	elems, err := diagram.ParseXML(innerXML)
	if err != nil {
		return nil, fmt.Errorf("pou '%s': graphical body: %w", pou, err)
	}
	return diagram.Lower(pou, elems, diagram.Options{FunctionBlocks: fbTypes, Declared: declared})
}

// projectFunctionBlocks names the project's function block types.
func projectFunctionBlocks(proj *Project) []string {
	var out []string
	for _, p := range proj.Types.Pous.Pous {
		if p.PouType == "functionBlock" {
			out = append(out, p.Name)
		}
	}
	return out
}

// declaredNames names a POU's variables.
func declaredNames(pou POU) []string {
	out := []string{pou.Name}
	if pou.Interface == nil {
		return out
	}
	i := pou.Interface
	for _, lists := range [][]VarList{i.InputVars, i.OutputVars, i.InOutVars, i.LocalVars, i.TempVars, i.ExternalVars, i.GlobalVars, i.AccessVars} {
		for _, l := range lists {
			for _, v := range l.Variables {
				out = append(out, v.Name)
			}
		}
	}
	return out
}

// graphicalBodies writes the POUs whose bodies are in the LD or FBD text
// form as graphical bodies (diagram.XML), so other IEC tools draw them as
// diagrams. The interface keeps the instances the text declares, not the
// variables beedance's lowering adds for itself (edge detectors and the
// like), which another tool makes its own.
func graphicalBodies(proj *Project, diagrams []parser.DiagramBody) error {
	byPOU := map[string]parser.DiagramBody{}
	for _, d := range diagrams {
		byPOU[strings.ToUpper(d.POU)] = d
	}
	for i := range proj.Types.Pous.Pous {
		pou := &proj.Types.Pous.Pous[i]
		d, ok := byPOU[strings.ToUpper(pou.Name)]
		if !ok {
			continue
		}
		t, err := diagram.ParseText(d.Lang, d.Text, d.Row)
		if err != nil {
			return fmt.Errorf("pou '%s': %w", pou.Name, err)
		}
		x, err := diagram.XML(t.Elems)
		if err != nil {
			return fmt.Errorf("pou '%s': %w", pou.Name, err)
		}
		raw := &RawXML{Inner: x}
		pou.Body.ST, pou.Body.IL = nil, nil
		if strings.EqualFold(d.Lang, "LD") {
			pou.Body.LD = raw
		} else {
			pou.Body.FBD = raw
		}
		low, err := diagram.LowerText(pou.Name, d.Lang, d.Text, d.Row, diagram.Options{Declared: declaredExcept(*pou, t)})
		if err != nil || pou.Interface == nil {
			continue
		}
		generated := map[string]bool{}
		for _, decl := range low.Decls {
			name, _, _ := strings.Cut(decl, ":")
			generated[strings.ToUpper(strings.TrimSpace(name))] = true
		}
		for _, inst := range t.Declares {
			delete(generated, strings.ToUpper(inst[0]))
		}
		for j := range pou.Interface.LocalVars {
			vs := pou.Interface.LocalVars[j].Variables[:0]
			for _, v := range pou.Interface.LocalVars[j].Variables {
				if !generated[strings.ToUpper(v.Name)] {
					vs = append(vs, v)
				}
			}
			pou.Interface.LocalVars[j].Variables = vs
		}
		lists := pou.Interface.LocalVars[:0]
		for _, l := range pou.Interface.LocalVars {
			if len(l.Variables) > 0 {
				lists = append(lists, l)
			}
		}
		pou.Interface.LocalVars = lists
	}
	return nil
}

// declaredExcept names a POU's variables but those the lowering of t adds,
// so lowering t again names its helpers as the parser did.
func declaredExcept(pou POU, t *diagram.Text) []string {
	own := map[string]bool{}
	for _, inst := range t.Declares {
		own[strings.ToUpper(inst[0])] = true
	}
	var out []string
	for _, n := range declaredNames(pou) {
		if !strings.HasPrefix(n, "_") || own[strings.ToUpper(n)] {
			out = append(out, n)
		}
	}
	return out
}

// diagramText writes a graphical body in the text form (diagram.Format),
// checked by lowering it as the parser will. It returns the text between LD
// (FBD) and END_LD (END_FBD), or why the body cannot be written so.
func diagramText(pou POU, lang, innerXML string, fbTypes []string) (string, error) {
	elems, err := diagram.ParseXML(innerXML)
	if err != nil {
		return "", err
	}
	text, err := diagram.Format(lang, elems, fbTypes)
	if err != nil {
		return "", err
	}
	if _, err := diagram.LowerText(pou.Name, lang, text, 1, diagram.Options{FunctionBlocks: fbTypes, Declared: declaredNames(pou)}); err != nil {
		return "", fmt.Errorf("the text form does not read back: %w", err)
	}
	return text, nil
}
