/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast

import (
	"math/rand"
	"regexp"
	"testing"
)

// directSize accepts what the regexp it replaces accepted, on every input.
func TestDirectSizeMatchesRegexp(t *testing.T) {
	re := regexp.MustCompile(`^%([IQM])([XBWDL]?)(\d+(?:\.\d+)*)$`)
	check := func(s string) {
		t.Helper()
		m := re.FindStringSubmatch(s)
		size, ok := directSize(s)
		if ok != (m != nil) {
			t.Fatalf("%q: ok %v, regexp %v", s, ok, m)
		}
		if ok && string(rune(size)) != m[2] && !(size == 0 && m[2] == "") {
			t.Fatalf("%q: size %q, regexp %q", s, size, m[2])
		}
	}
	for _, s := range []string{"%IX0.0", "%IW1", "%QX0.1", "%MW5", "%I0", "%IX", "%I", "%", "%IX0.", "%IX.0",
		"%IX0..1", "%ZX0", "IX0", "%IXX0", "%IL12.3.4", "%IB", "%QD7"} {
		check(s)
	}
	r := rand.New(rand.NewSource(1))
	const alphabet = "%IQMXBWDLZ0123.."
	for i := 0; i < 200000; i++ {
		b := make([]byte, r.Intn(8))
		for j := range b {
			b[j] = alphabet[r.Intn(len(alphabet))]
		}
		check(string(b))
	}
}
