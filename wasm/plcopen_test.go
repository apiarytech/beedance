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

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestXMLRoundTrip: beedance's examples go out as PLCopen XML and come back
// as written, comments and all, with the settings a host keeps with them.
func TestXMLRoundTrip(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "examples", "*", "*.st"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	h := New()
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if r := call(t, h, map[string]any{"op": "check", "source": string(src)}); r["ok"] != true {
			continue // an example of errors
		}
		sim, _ := json.Marshal(map[string]any{"source": string(src), "watches": []string{"x"}})
		out := call(t, h, map[string]any{"op": "exportXML", "source": string(src), "name": filepath.Base(f), "sim": string(sim)})
		if out["error"] != nil {
			continue // a source PLCopen cannot hold (see package plcopen)
		}
		xmlText := out["xml"].(string)
		if !strings.Contains(xmlText, `<project xmlns="http://www.plcopen.org/xml/tc6_0201"`) {
			t.Fatalf("%s: not a TC6 project: %.200s", f, xmlText)
		}
		in := call(t, h, map[string]any{"op": "importXML", "xml": xmlText})
		noError(t, in)
		if in["source"] != string(src) {
			t.Errorf("%s: the source came back changed:\n%s", f, in["source"])
		}
		if in["name"] != filepath.Base(f) || !strings.Contains(in["sim"].(string), `"watches":["x"]`) {
			t.Errorf("%s: name %v, sim %v", f, in["name"], in["sim"])
		}
	}
}

// TestXMLFromOtherTools: XML another tool changed is read as it is now, and
// another tool's additional data survives an export.
func TestXMLFromOtherTools(t *testing.T) {
	h := New()
	src := "(* a comment *)\nPROGRAM Main\nVAR n : INT; END_VAR\nn := n + 1;\nEND_PROGRAM\n"
	sim, _ := json.Marshal(map[string]any{"source": src})
	xmlText := call(t, h, map[string]any{"op": "exportXML", "source": src, "name": "P", "sim": string(sim)})["xml"].(string)

	// Another tool edits the body: its edit wins over the kept text.
	edited := strings.Replace(xmlText, "n := n + 1;", "n := n + 2;", 1)
	in := call(t, h, map[string]any{"op": "importXML", "xml": edited})
	noError(t, in)
	if s := in["source"].(string); !strings.Contains(s, "n := n + 2;") || strings.Contains(s, "a comment") {
		t.Errorf("edited elsewhere, the source is %q", s)
	}

	// Another tool's data, at the project's level, is kept on export.
	foreign := strings.Replace(xmlText, "<addData>", `<addData>
    <data name="https://example.com/tool" handleUnknown="preserve"><setting xmlns="https://example.com/tool">42</setting></data>`, 1)
	out := call(t, h, map[string]any{"op": "exportXML", "source": src, "name": "P", "sim": string(sim), "from": foreign})
	noError(t, out)
	if x := out["xml"].(string); !strings.Contains(x, "https://example.com/tool") || strings.Count(x, `name="`+SimAddData+`"`) != 2 {
		t.Errorf("the other tool's data, or ours, is lost or doubled:\n%s", x)
	}

	if r := call(t, h, map[string]any{"op": "importXML", "xml": "<nope/>"}); r["error"] == nil {
		t.Errorf("importing what is no PLCopen project: %v", r)
	}
	if r := call(t, h, map[string]any{"op": "exportXML", "source": "PROGRAM P VAR x : INT END_VAR"}); r["error"] == nil {
		t.Errorf("exporting a source with errors: %v", r)
	}
}
