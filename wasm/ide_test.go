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
	"strings"
	"testing"
)

const ldSource = `FUNCTION_BLOCK Motor
VAR_INPUT Start, Stop : BOOL; END_VAR
VAR_OUTPUT Run : BOOL; END_VAR
VAR spare : INT; END_VAR
LD
  RUNG sealin
    [ Start | Run ] /Stop ( Run )
END_LD
END_FUNCTION_BLOCK

PROGRAM Main
VAR m : Motor; go : BOOL; halt : BOOL; lamp : BOOL; END_VAR
m(Start := go, Stop := halt);
lamp := m.Run;
END_PROGRAM
`

// TestIDEOps: an editor's questions, answered by the module.
func TestIDEOps(t *testing.T) {
	h := New()
	at := func(op string, line, col int, more ...any) map[string]any {
		req := map[string]any{"op": op, "source": ldSource, "line": line, "column": col}
		for i := 0; i+1 < len(more); i += 2 {
			req[more[i].(string)] = more[i+1]
		}
		r := call(t, h, req)
		noError(t, r)
		return r
	}

	lint := call(t, h, map[string]any{"op": "lint", "source": ldSource})
	found := false
	for _, f := range lint["findings"].([]any) {
		f := f.(map[string]any)
		if f["rule"] == "unused-variable" && strings.Contains(f["message"].(string), "spare") && f["line"] == 4.0 && f["severity"] == "warning" {
			found = true
		}
	}
	if !found {
		t.Errorf("lint %v: no unused spare on line 4", lint)
	}

	use := strings.Index(strings.Split(ldSource, "\n")[13], "m.Run") + 1 // lamp := m.Run;
	if r := at("hover", 14, use); !strings.Contains(r["text"].(string), "Motor") {
		t.Errorf("hover on m: %v", r)
	}
	items := at("complete", 14, use+2)["items"].([]any)
	if len(items) == 0 || items[0].(map[string]any)["label"] == "" {
		t.Errorf("completion after m.: %v", items)
	}
	def := at("definition", 14, use)["span"].(map[string]any)
	if def["start"].(map[string]any)["row"] != 12.0 {
		t.Errorf("definition of m: %v", def)
	}
	if refs := at("references", 14, use)["spans"].([]any); len(refs) != 3 {
		t.Errorf("references to m: %v, want its declaration and two uses", refs)
	}
	if spans := at("rename", 14, use, "newName", "drive")["spans"].([]any); len(spans) != 3 {
		t.Errorf("rename m: %v", spans)
	}
	if r := call(t, h, map[string]any{"op": "hover", "source": ldSource}); r["error"] == nil {
		t.Error("hover with no place")
	}

	ds := call(t, h, map[string]any{"op": "diagrams", "source": ldSource})["diagrams"].([]any)
	if len(ds) != 1 {
		t.Fatalf("%d diagrams, want the LD body of Motor", len(ds))
	}
	d := ds[0].(map[string]any)
	if d["pou"] != "Motor" || d["lang"] != "LD" || !strings.HasPrefix(d["svg"].(string), "<svg") || len(d["watches"].([]any)) == 0 {
		t.Errorf("diagram %v", d)
	}
}

// TestOutline: the POUs with their members, and a configuration's
// resources, tasks and program instances, for an editor's tree.
func TestOutline(t *testing.T) {
	src := ldSource + `
CONFIGURATION Cell
RESOURCE CPU ON PLC
TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
PROGRAM P1 WITH Fast : Main;
END_RESOURCE
END_CONFIGURATION
`
	r := call(t, New(), map[string]any{"op": "outline", "source": src})
	noError(t, r)
	var kinds []string
	for _, s := range r["symbols"].([]any) {
		s := s.(map[string]any)
		kinds = append(kinds, s["kind"].(string)+" "+s["name"].(string))
	}
	if got := strings.Join(kinds, ", "); got != "function_block Motor, program Main, configuration Cell" {
		t.Errorf("outline %s", got)
	}
	cfg := r["configurations"].([]any)[0].(map[string]any)
	res := cfg["resources"].([]any)[0].(map[string]any)
	task := res["tasks"].([]any)[0].(map[string]any)
	inst := res["programs"].([]any)[0].(map[string]any)
	if cfg["name"] != "Cell" || res["name"] != "CPU" || task["name"] != "Fast" || task["interval"] != "T#10ms" ||
		inst["name"] != "P1" || inst["type"] != "Main" || inst["task"] != "Fast" {
		t.Errorf("configuration %v", cfg)
	}
}
