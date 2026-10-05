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

	"github.com/apiarytech/beedance/diagram"
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
