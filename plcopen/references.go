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

// This file maps references to TC6, which has one reference type, <pointer>.
// REF_TO, POINTER TO and REFERENCE TO are all written as <pointer>, and
// <pointer> is read as POINTER TO, which is what it means in the files of
// CODESYS and other tools. A REFERENCE TO is read without ^ and a REF_TO is
// spelled the IEC 61131-3 way, which <pointer> cannot say, so beedance also
// writes such a variable's type as it is in additional data of its own, and
// reads it back from there. That data is marked "discard": a tool that
// changes the variable drops it, and the variable is then read from its
// <pointer>.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// STTypeData names beedance's additional data holding a variable's type as
// written in Structured Text.
const STTypeData = "https://apiarytech.io/beedance/plcopen/st-type"

// stType is the content of STTypeData.
type stType struct {
	XMLName struct{} `xml:"https://apiarytech.io/beedance/plcopen stType"`
	Text    string   `xml:",chardata"`
}

// stTypeData returns the additional data holding a type's Structured Text,
// for a type <pointer> does not say exactly, or nil.
func stTypeData(t ast.Expression) *AddData {
	if t == nil {
		return nil
	}
	if _, isStruct := t.(*ast.StructDefinition); isStruct {
		return nil // its members carry their own
	}
	text := t.String()
	if !strings.Contains(text, "REF_TO") && !strings.Contains(text, "REFERENCE TO") {
		return nil
	}
	a, _ := (*AddData)(nil).Set(STTypeData, HandleDiscard, stType{Text: text})
	return a
}

// typeText returns the Structured Text of a variable's type: the text
// beedance kept, or the text of its TC6 type.
func typeText(dt DataType, a *AddData) string {
	if e := a.Get(STTypeData); e != nil {
		var t stType
		if err := e.Decode(&t); err == nil && strings.TrimSpace(t.Text) != "" {
			return strings.TrimSpace(t.Text)
		}
	}
	return dt.String()
}

// declareBeedanceData lists beedance's additional data in the project's
// addDataInfo when the project uses it.
func declareBeedanceData(proj *Project) {
	used := false
	mark := func(a *AddData) {
		used = used || a.Get(STTypeData) != nil
	}
	for _, dt := range proj.Types.DataTypes.DataTypes {
		mark(dt.AddData)
		if dt.BaseType.Struct != nil {
			for _, v := range dt.BaseType.Struct.Variables {
				mark(v.AddData)
			}
		}
	}
	lists := func(ls []VarList) {
		for _, l := range ls {
			for _, v := range l.Variables {
				mark(v.AddData)
			}
		}
	}
	for _, pou := range proj.Types.Pous.Pous {
		if in := pou.Interface; in != nil {
			for _, ls := range [][]VarList{in.InputVars, in.OutputVars, in.InOutVars, in.LocalVars, in.TempVars, in.ExternalVars, in.GlobalVars, in.AccessVars} {
				lists(ls)
			}
		}
	}
	for _, cfg := range proj.Instances.Configurations.Configurations {
		lists(cfg.GlobalVars)
		for _, res := range cfg.Resources {
			lists(res.GlobalVars)
		}
	}
	if !used {
		return
	}
	if proj.ContentHeader.AddDataInfo == nil {
		proj.ContentHeader.AddDataInfo = &AddDataInfo{}
	}
	for _, i := range proj.ContentHeader.AddDataInfo.Info {
		if i.Name == STTypeData {
			return
		}
	}
	proj.ContentHeader.AddDataInfo.Info = append(proj.ContentHeader.AddDataInfo.Info, AddDataInfoEntry{
		Name:        STTypeData,
		Version:     "1.0",
		Vendor:      "https://apiarytech.io",
		Description: &FormattedText{Text: `<xhtml:p xmlns:xhtml="http://www.w3.org/1999/xhtml">A variable's type as written in IEC 61131-3 Structured Text, where TC6 &lt;pointer&gt; does not say it exactly (REF_TO, REFERENCE TO).</xhtml:p>`},
	})
}
