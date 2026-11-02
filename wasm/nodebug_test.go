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
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNodebugLeavesOutTheDebugger: the module built with -tags nodebug
// links neither package dap nor package vmdebug, so a page that only runs
// programs never receives the debugger; the default build has both.
func TestNodebugLeavesOutTheDebugger(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	deps := func(tags string) string {
		cmd := exec.Command("go", "list", "-deps", "-tags", tags, "./cmd/beedance-wasm")
		cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list -tags %q: %v", tags, err)
		}
		return string(out)
	}
	for _, tags := range []string{"nodebug", "lite,nodebug"} {
		got := deps(tags)
		for _, pkg := range []string{"beedance/dap\n", "beedance/vmdebug\n"} {
			if strings.Contains(got, pkg) {
				t.Errorf("-tags %s links %s", tags, strings.TrimSpace(pkg))
			}
		}
	}
	if got := deps("lite"); !strings.Contains(got, "beedance/dap\n") || !strings.Contains(got, "beedance/vmdebug\n") {
		t.Error("-tags lite has no debugger")
	}
}
