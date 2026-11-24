/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

import (
	"hash/fnv"
	"testing"
)

func TestFNV64aMatchesHashFNV(t *testing.T) {
	for _, s := range []string{"", "Q", "IN", "running", "__class__", "Ünïcode", "a#b"} {
		h := fnv.New64a()
		h.Write([]byte(s))
		if got, want := fnv64a(s), h.Sum64(); got != want {
			t.Errorf("fnv64a(%q) = %x, want %x", s, got, want)
		}
	}
}
