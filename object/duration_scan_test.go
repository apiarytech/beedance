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
	"math/rand"
	"regexp"
	"testing"
)

// durationParts finds what the regexp it replaces found, on every input.
func TestDurationPartsMatchesRegexp(t *testing.T) {
	re := regexp.MustCompile(`(\d*\.?\d+)([a-z]+)`)
	check := func(s string) {
		t.Helper()
		var want [][2]string
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			want = append(want, [2]string{m[1], m[2]})
		}
		got := durationParts(s)
		if len(got) != len(want) {
			t.Fatalf("%q: got %q, want %q", s, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%q: got %q, want %q", s, got, want)
			}
		}
	}
	for _, s := range []string{"", "1s", "1.5s", ".5s", "1d12h30m5s10ms", "15ms", "1.s", "12.x3s", "1..5s",
		"5", "s", "1m30", "1.2.3s", "x1s", "1s.", "0.25h", "1us2ns", "1.5.5s", "99"} {
		check(s)
	}
	r := rand.New(rand.NewSource(1))
	const alphabet = "0123456789..dhmsunx_-"
	for i := 0; i < 200000; i++ {
		b := make([]byte, r.Intn(12))
		for j := range b {
			b[j] = alphabet[r.Intn(len(alphabet))]
		}
		check(string(b))
	}
}
