/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

// The simulator's projects as PLCopen XML (IEC 61131-10, TC6 XML): the
// source as POUs any tool reads, and what only the simulator knows (the
// canvas, breakpoints, watches) as additional data under its own name,
// which other tools keep or ignore.

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"strings"

	"github.com/apiarytech/beedance/plcopen"
)

// SimAddData names the simulator's additional data in a PLCopen project.
const SimAddData = "https://apiarytech.io/beedance/sim"

// simData is the simulator's additional data: its project settings as JSON,
// kept as text so the XML need not know their shape.
type simData struct {
	XMLName xml.Name `xml:"https://apiarytech.io/beedance/sim simProject"`
	Version int      `xml:"version,attr"`
	JSON    string   `xml:",chardata"`
}

type xmlReply struct {
	XML string `json:"xml"`
}

type importReply struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	// Sim is the simulator's settings, as JSON, when the project has them.
	Sim string `json:"sim,omitempty"`
}

// exportXML writes req.Source as a PLCopen project named req.Name, with
// req.Sim (JSON) as the simulator's additional data. With req.From, the
// XML the source was imported from, the other tools' additional data is
// kept.
func exportXML(req Request) (any, error) {
	var from *plcopen.Project
	if req.From != "" {
		var err error
		if from, err = plcopen.Import([]byte(req.From)); err != nil {
			from = nil // not to keep: export what there is
		}
	}
	proj, err := plcopen.ExportSourceKeeping(req.Source, req.Name, from)
	if err != nil {
		return nil, err
	}
	if req.Sim != "" {
		if proj.AddData, err = proj.AddData.Set(SimAddData, plcopen.HandlePreserve, simData{Version: 1, JSON: req.Sim}); err != nil {
			return nil, err
		}
		info := proj.ContentHeader.AddDataInfo
		if info == nil {
			info = &plcopen.AddDataInfo{}
		}
		known := false
		for _, e := range info.Info {
			known = known || e.Name == SimAddData
		}
		if !known {
			info.Info = append(info.Info, plcopen.AddDataInfoEntry{Name: SimAddData, Version: "1", Vendor: "ApiaryTech.io"})
		}
		proj.ContentHeader.AddDataInfo = info
	}
	data, err := xml.MarshalIndent(proj, "", "  ")
	if err != nil {
		return nil, err
	}
	return xmlReply{XML: xml.Header + string(data) + "\n"}, nil
}

// importXML reads a PLCopen project: its POUs as source, its name, and
// the simulator's settings if it has them.
func importXML(req Request) (any, error) {
	if strings.TrimSpace(req.XML) == "" {
		return nil, errors.New(`importXML: give "xml"`)
	}
	source, err := plcopen.ImportToIECText([]byte(req.XML))
	if err != nil {
		return nil, err
	}
	r := importReply{Source: source}
	if proj, err := plcopen.Import([]byte(req.XML)); err == nil {
		r.Name = proj.ContentHeader.Name
		if e := proj.AddData.Get(SimAddData); e != nil {
			var d simData
			if e.Decode(&d) == nil {
				r.Sim = strings.TrimSpace(d.JSON)
			}
		}
	}
	// PLCopen keeps the POUs, not every comment around them: the simulator
	// keeps its text in its settings ("source"). If the POUs are still what
	// that text makes, no other tool has changed them: the text comes back
	// as written.
	var kept struct {
		Source string `json:"source"`
	}
	if r.Sim != "" && json.Unmarshal([]byte(r.Sim), &kept) == nil && kept.Source != "" {
		if again, err := plcopen.ExportSourceToXML(kept.Source, r.Name); err == nil {
			if canon, err := plcopen.ImportToIECText(again); err == nil && canon == source {
				r.Source = kept.Source
			}
		}
	}
	return r, nil
}
