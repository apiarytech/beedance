/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"testing"

	"github.com/apiarytech/beedance/sil"
)

// TestGoEngine runs the session's tests on the go engine, which builds the
// program with the Go toolchain (and fetches royaljelly): without go in
// PATH it is skipped; elsewhere it fails as package sil's go engine does.
func TestGoEngine(t *testing.T) {
	if err := sil.GoEngineAvailable(); err != nil {
		t.Skip("the go engine cannot build here:", err)
	}
	runOn(t, EngineGo)
}
