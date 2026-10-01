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
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	for input, want := range map[string]time.Duration{
		"1d":              24 * time.Hour,
		"1d_12h":          36 * time.Hour,
		"1h30m":           90 * time.Minute,
		"1.5s":            1500 * time.Millisecond,
		"10ms":            10 * time.Millisecond,
		"5us":             5 * time.Microsecond,
		"7ns":             7,
		"-2s":             -2 * time.Second,
		"1D_2H_3M_4S_5MS": 26*time.Hour + 3*time.Minute + 4*time.Second + 5*time.Millisecond,
	} {
		got, err := ParseDuration(input)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	for input, want := range map[string]string{
		"":       "empty",
		"-":      "empty",
		"bad":    "invalid duration format",
		"1w":     `unknown duration unit "w"`,
		"1s2":    "unparsed characters",
		"1.2.3s": "unparsed characters",
	} {
		if _, err := ParseDuration(input); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseDuration(%q) error = %v, want %q", input, err, want)
		}
	}
}

func TestLowerBounds(t *testing.T) {
	row := func() *Array { return &Array{Elements: []Object{&LInt{}, &LInt{}}} }
	matrix := &Array{Elements: []Object{row(), row()}}
	SetLowerBounds(matrix, []int64{1, -1})
	if got := LowerBounds(matrix); len(got) != 2 || got[0] != 1 || got[1] != -1 {
		t.Errorf("LowerBounds = %v, want [1 -1]", got)
	}
	if got := LowerBounds(&Array{LowerBound: 3}); len(got) != 1 || got[0] != 3 {
		t.Errorf("LowerBounds of an empty array = %v, want [3]", got)
	}
	if got := LowerBounds(&LInt{}); len(got) != 0 {
		t.Errorf("LowerBounds of a number = %v", got)
	}
	SetLowerBounds(&LInt{}, []int64{1}) // Not an array: left alone.
}
