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
	"sync"
	"testing"
)

// TestHostsConcurrently loads, scans and tests programs on several Hosts
// at once, as a server embedding the toolchain does: they share the
// built-in table, which loading must not rewrite under another engine.
// Run with -race.
func TestHostsConcurrently(t *testing.T) {
	do := func(h *Host, req map[string]any) (map[string]any, bool) {
		in, _ := json.Marshal(req)
		var reply map[string]any
		if err := json.Unmarshal(h.Handle(in), &reply); err != nil || reply["error"] != nil {
			t.Errorf("%v: %v %v", req["op"], err, reply)
			return nil, false
		}
		return reply, true
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h := New()
			if i%2 == 0 {
				if _, ok := do(h, map[string]any{"op": "test", "source": source, "filter": "Add"}); !ok {
					return
				}
			}
			for j := 0; j < 5; j++ {
				reply, ok := do(h, map[string]any{"op": "load", "source": source, "intervalMs": 250})
				if !ok {
					return
				}
				sid := reply["session"]
				reply, ok = do(h, map[string]any{"op": "scan", "session": sid, "count": 5, "values": map[string]any{"start": true}})
				if !ok {
					return
				}
				if v := reply["values"].(map[string]any); v["count"] != 3.0 || v["lamp"] != true {
					t.Errorf("after five scans: %v", v)
					return
				}
				do(h, map[string]any{"op": "unload", "session": sid})
			}
		}(i)
	}
	wg.Wait()
}
