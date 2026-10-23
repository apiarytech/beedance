/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoModDirective covers issue #19: the temporary module's go directive
// is royaljelly's, from a replaced royaljelly's own go.mod when there is one.
func TestGoModDirective(t *testing.T) {
	if got := goMod("", nil); !strings.Contains(got, "\ngo "+goDirective+"\n") {
		t.Errorf("go.mod without replace:\n%s\nwant go %s", got, goDirective)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/apiarytech/royaljelly\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := goMod("", map[string]string{"github.com/apiarytech/royaljelly": dir})
	if !strings.Contains(got, "\ngo 1.25\n") {
		t.Errorf("go.mod with a replaced royaljelly:\n%s\nwant go 1.25", got)
	}
}
