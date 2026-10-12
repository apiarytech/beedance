/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram

import (
	"slices"
	"strings"
	"testing"
)

// A transition names the variables its condition reads, and an action that
// is a variable names it, so a page can show their values (drone L-34).
func TestSFCValueSlots(t *testing.T) {
	for cond, want := range map[string][]string{
		"WellLevel > 70.0":             {"WellLevel"},
		"rest.Q AND NOT Stop":          {"rest.Q", "Stop"},
		"t.ET >= T#5s OR x = INT#3":    {"t.ET", "x"},
		"ABS(a - b) > 1.0 AND a > 0.0": {"a", "b"},
		"TRUE":                         nil,
	} {
		if got := condVars(cond); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", cond, got, want)
		}
	}
	c := &Chart{
		Steps:       []Step{{Name: "A", Initial: true, Actions: []Association{{Action: "Pump", Qualifier: "N"}}}, {Name: "B"}},
		Transitions: []Transition{{From: []string{"A"}, To: []string{"B"}, Condition: "Level > 70.0"}, {From: []string{"B"}, To: []string{"A"}, Condition: "TRUE"}},
	}
	svg := SFCSVG(c)
	for _, want := range []string{`data-vals="Level"`, `data-vals="Pump"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s in %s", want, svg)
		}
	}
}
