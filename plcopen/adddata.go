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

// This file keeps PLCopen's additional data (addData): application data in
// schemas of other vendors, such as a programming tool's own settings or an
// HMI's tag properties, which TC6 lets almost every element carry. beedance
// reads it, writes it back, and gives it to programs by name (Get, Set).

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// How a tool treats additional data it does not know, when it changes the
// element that carries it (the handleUnknown attribute).
const (
	// HandlePreserve keeps the data.
	HandlePreserve = "preserve"
	// HandleDiscard drops it, as it may be wrong for the changed element.
	HandleDiscard = "discard"
	// HandleImplementation leaves it to the tool; beedance keeps it.
	HandleImplementation = "implementation"
)

// AddData is an element's additional data (addData).
type AddData struct {
	Data []AddDataEntry `xml:"data"`
}

// AddDataEntry is one item of additional data, named by a URI. Its content
// is XML of any schema, kept as tokens with their namespaces resolved, so
// that it is written back valid whatever prefixes the document declared.
type AddDataEntry struct {
	Name          string
	HandleUnknown string
	Content       []xml.Token
}

// AddDataInfo lists the additional data a document uses (addDataInfo).
type AddDataInfo struct {
	Info []AddDataInfoEntry `xml:"info"`
}

// AddDataInfoEntry describes one kind of additional data: its name, the
// version of its schema and its vendor.
type AddDataInfoEntry struct {
	Name        string         `xml:"name,attr"`
	Version     string         `xml:"version,attr,omitempty"`
	Vendor      string         `xml:"vendor,attr"`
	Description *FormattedText `xml:"description,omitempty"`
}

// UnmarshalXML reads a <data> element, keeping its content.
func (e *AddDataEntry) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "name":
			e.Name = a.Value
		case "handleUnknown":
			e.HandleUnknown = a.Value
		}
	}
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			if depth == 0 {
				return nil
			}
			depth--
		}
		e.Content = append(e.Content, xml.CopyToken(tok))
	}
}

// MarshalXML writes a <data> element with its content.
func (e AddDataEntry) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "data"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "name"}, Value: e.Name},
		{Name: xml.Name{Local: "handleUnknown"}, Value: e.handleUnknown()},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	for _, tok := range e.Content {
		if err := enc.EncodeToken(tok); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}

// handleUnknown returns the entry's handleUnknown, which TC6 requires:
// "implementation" when it was not given.
func (e AddDataEntry) handleUnknown() string {
	if e.HandleUnknown == "" {
		return HandleImplementation
	}
	return e.HandleUnknown
}

// Decode unmarshals the entry's content into v, as xml.Unmarshal does.
func (e AddDataEntry) Decode(v any) error {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	for _, tok := range e.Content {
		if err := enc.EncodeToken(tok); err != nil {
			return err
		}
	}
	if err := enc.Flush(); err != nil {
		return err
	}
	return xml.Unmarshal(buf.Bytes(), v)
}

// Get returns the entry named name, or nil. It may be called on nil.
func (a *AddData) Get(name string) *AddDataEntry {
	if a == nil {
		return nil
	}
	for i := range a.Data {
		if a.Data[i].Name == name {
			return &a.Data[i]
		}
	}
	return nil
}

// Set stores v, marshalled as XML, as the entry named name, replacing an
// entry of that name, and returns a, or a new AddData when a is nil.
func (a *AddData) Set(name, handleUnknown string, v any) (*AddData, error) {
	data, err := xml.Marshal(v)
	if err != nil {
		return a, fmt.Errorf("addData %s: %w", name, err)
	}
	var content []xml.Token
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return a, fmt.Errorf("addData %s: %w", name, err)
		}
		content = append(content, xml.CopyToken(tok))
	}
	entry := AddDataEntry{Name: name, HandleUnknown: handleUnknown, Content: content}
	if a == nil {
		a = &AddData{}
	}
	if old := a.Get(name); old != nil {
		*old = entry
	} else {
		a.Data = append(a.Data, entry)
	}
	return a, nil
}

// kept returns the entries of from to carry onto an element beedance has
// written anew, without those to discard when an element changes and those
// the new element has itself; nil when none is left.
func kept(from, to *AddData) *AddData {
	if from == nil {
		return to
	}
	out := to
	for _, e := range from.Data {
		if e.HandleUnknown == HandleDiscard || to.Get(e.Name) != nil {
			continue
		}
		if out == nil {
			out = &AddData{}
		} else if out == to {
			out = &AddData{Data: append([]AddDataEntry{}, to.Data...)}
		}
		out.Data = append(out.Data, e)
	}
	return out
}

// KeepAddData carries the additional data of a project beedance read onto
// one it wrote, such as the export of the read project's code after an
// edit, so that a tool's own data survives a round trip through beedance.
// Elements are matched by name: the project, data types, POUs and their
// variables, configurations, resources and tasks. Entries marked
// "discard" are dropped, as beedance writes each element anew.
func KeepAddData(from, to *Project) {
	if from == nil || to == nil {
		return
	}
	to.AddData = kept(from.AddData, to.AddData)
	to.ContentHeader.AddData = kept(from.ContentHeader.AddData, to.ContentHeader.AddData)
	if from.ContentHeader.AddDataInfo != nil {
		to.ContentHeader.AddDataInfo = keptInfo(from.ContentHeader.AddDataInfo, to.ContentHeader.AddDataInfo)
	}

	types := map[string]*DataTypeDecl{}
	for i := range from.Types.DataTypes.DataTypes {
		types[from.Types.DataTypes.DataTypes[i].Name] = &from.Types.DataTypes.DataTypes[i]
	}
	for i := range to.Types.DataTypes.DataTypes {
		dt := &to.Types.DataTypes.DataTypes[i]
		if old := types[dt.Name]; old != nil {
			dt.AddData = kept(old.AddData, dt.AddData)
			if old.BaseType.Struct != nil && dt.BaseType.Struct != nil {
				keepVariables(old.BaseType.Struct.Variables, dt.BaseType.Struct.Variables)
			}
		}
	}

	pous := map[string]*POU{}
	for i := range from.Types.Pous.Pous {
		pous[from.Types.Pous.Pous[i].Name] = &from.Types.Pous.Pous[i]
	}
	for i := range to.Types.Pous.Pous {
		pou := &to.Types.Pous.Pous[i]
		old := pous[pou.Name]
		if old == nil {
			continue
		}
		pou.AddData = kept(old.AddData, pou.AddData)
		pou.Body.AddData = kept(old.Body.AddData, pou.Body.AddData)
		if o, n := old.Interface, pou.Interface; o != nil && n != nil {
			n.AddData = kept(o.AddData, n.AddData)
			keepVarLists(o.InputVars, n.InputVars)
			keepVarLists(o.OutputVars, n.OutputVars)
			keepVarLists(o.InOutVars, n.InOutVars)
			keepVarLists(o.LocalVars, n.LocalVars)
			keepVarLists(o.TempVars, n.TempVars)
			keepVarLists(o.ExternalVars, n.ExternalVars)
			keepVarLists(o.GlobalVars, n.GlobalVars)
			keepVarLists(o.AccessVars, n.AccessVars)
		}
	}

	configs := map[string]*Configuration{}
	for i := range from.Instances.Configurations.Configurations {
		configs[from.Instances.Configurations.Configurations[i].Name] = &from.Instances.Configurations.Configurations[i]
	}
	for i := range to.Instances.Configurations.Configurations {
		cfg := &to.Instances.Configurations.Configurations[i]
		old := configs[cfg.Name]
		if old == nil {
			continue
		}
		cfg.AddData = kept(old.AddData, cfg.AddData)
		keepVarLists(old.GlobalVars, cfg.GlobalVars)
		resources := map[string]*Resource{}
		for j := range old.Resources {
			resources[old.Resources[j].Name] = &old.Resources[j]
		}
		for j := range cfg.Resources {
			res := &cfg.Resources[j]
			oldRes := resources[res.Name]
			if oldRes == nil {
				continue
			}
			res.AddData = kept(oldRes.AddData, res.AddData)
			keepVarLists(oldRes.GlobalVars, res.GlobalVars)
			tasks := map[string]*Task{}
			for k := range oldRes.Tasks {
				tasks[oldRes.Tasks[k].Name] = &oldRes.Tasks[k]
			}
			for k := range res.Tasks {
				if oldTask := tasks[res.Tasks[k].Name]; oldTask != nil {
					res.Tasks[k].AddData = kept(oldTask.AddData, res.Tasks[k].AddData)
				}
			}
		}
	}
}

// keepVarLists carries the additional data of variables, matched by name,
// and of a list onto the list that holds its first variable.
func keepVarLists(from, to []VarList) {
	byVar := map[string]*VarList{}
	var vars []Variable
	for i := range from {
		for _, v := range from[i].Variables {
			byVar[v.Name] = &from[i]
		}
		vars = append(vars, from[i].Variables...)
	}
	var toVars []*Variable
	for i := range to {
		for j := range to[i].Variables {
			toVars = append(toVars, &to[i].Variables[j])
		}
		if len(to[i].Variables) > 0 {
			if old := byVar[to[i].Variables[0].Name]; old != nil {
				to[i].AddData = kept(old.AddData, to[i].AddData)
			}
		}
	}
	keepVariablePointers(vars, toVars)
}

// keepVariables carries the additional data of variables matched by name.
func keepVariables(from, to []Variable) {
	ptrs := make([]*Variable, len(to))
	for i := range to {
		ptrs[i] = &to[i]
	}
	keepVariablePointers(from, ptrs)
}

func keepVariablePointers(from []Variable, to []*Variable) {
	byName := map[string]*Variable{}
	for i := range from {
		byName[from[i].Name] = &from[i]
	}
	for _, v := range to {
		if old := byName[v.Name]; old != nil {
			v.AddData = kept(old.AddData, v.AddData)
		}
	}
}

// keptInfo merges the descriptions of additional data, by name.
func keptInfo(from, to *AddDataInfo) *AddDataInfo {
	if to == nil {
		return from
	}
	out := &AddDataInfo{Info: append([]AddDataInfoEntry{}, to.Info...)}
	have := map[string]bool{}
	for _, i := range to.Info {
		have[i.Name] = true
	}
	for _, i := range from.Info {
		if !have[i.Name] {
			out.Info = append(out.Info, i)
		}
	}
	return out
}
