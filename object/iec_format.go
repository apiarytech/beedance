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

// This file writes values as their *_TO_STRING conversions give them, in
// IEC 61131-3's literal forms. The transpiler writes the same functions into
// the Go it generates (transpiler.iecFormatHelpers), so that every engine
// gives the same text.

import (
	"strconv"
	"strings"
	"time"
)

// FormatReal writes a REAL (bits 32) or LREAL (bits 64) as REAL_TO_STRING
// gives it: the shortest decimal that reads back as the same value, with a
// decimal point, and an exponent outside 1E-4 to 1E15: 1.5, 100.0,
// -0.000125, 1.0E+20.
func FormatReal(f float64, bits int) string {
	if f != f || f > 1e308 || f < -1e308 {
		return strconv.FormatFloat(f, 'g', -1, bits) // NaN, +Inf, -Inf
	}
	abs := f
	if abs < 0 {
		abs = -abs
	}
	var s string
	if abs == 0 || (abs >= 1e-4 && abs < 1e15) {
		s = strconv.FormatFloat(f, 'f', -1, bits)
	} else {
		s = strconv.FormatFloat(f, 'e', -1, bits)
	}
	mantissa, exponent, hasExponent := strings.Cut(s, "e")
	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	if hasExponent {
		return mantissa + "E" + exponent
	}
	return mantissa
}

// FormatTime writes a TIME as TIME_TO_STRING gives it, a TIME literal with
// the units that are not zero: T#1h2m3s4ms, T#-250ms, T#0s.
func FormatTime(d time.Duration) string {
	if d == 0 {
		return "T#0s"
	}
	var b strings.Builder
	b.WriteString("T#")
	u := uint64(d)
	if d < 0 {
		b.WriteByte('-')
		u = uint64(-d)
	}
	for _, unit := range []struct {
		name string
		size uint64
	}{
		{"d", uint64(24 * time.Hour)}, {"h", uint64(time.Hour)}, {"m", uint64(time.Minute)},
		{"s", uint64(time.Second)}, {"ms", uint64(time.Millisecond)}, {"us", uint64(time.Microsecond)}, {"ns", 1},
	} {
		if n := u / unit.size; n > 0 {
			b.WriteString(strconv.FormatUint(n, 10))
			b.WriteString(unit.name)
			u -= n * unit.size
		}
	}
	return b.String()
}
