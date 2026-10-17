/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package oscatdiff

import (
	"flag"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var strict = flag.Bool("strict", false, "fail on the evaluator's errors and on differences of precision too")

// maxMismatches is how many mismatches of one POU are reported before its
// comparison stops.
const maxMismatches = 3

// report collects the comparisons that are not failures by themselves: the
// evaluator's errors, by message, the differences of precision and the
// known mismatches, by POU, and the evaluator's unwrapped integer results.
type report struct {
	evalErrors map[string]map[string]string // message pattern, POU, example
	precision  map[string]string            // POU, example
	bothFail   map[string]string            // POU, example
	known      map[string]string            // POU, example
	wide       map[string]bool
	mismatches map[string]int
	tested     map[string]bool // the POUs compared
}

func newReport() *report {
	return &report{
		evalErrors: map[string]map[string]string{},
		precision:  map[string]string{},
		bothFail:   map[string]string{},
		known:      map[string]string{},
		wide:       map[string]bool{},
		mismatches: map[string]int{},
		tested:     map[string]bool{},
	}
}

var (
	reNumber   = regexp.MustCompile(`-?\d+(\.\d+)?`)
	rePosition = regexp.MustCompile(`^(ERROR: )?ERROR \(\d+:\d+\): `)
)

// add records a comparison of the POU name, failing t on a mismatch that is
// not known. It returns false when the POU's comparison should stop.
func (r *report) add(t *testing.T, name string, v verdict, text string) bool {
	t.Helper()
	switch v {
	case wide:
		r.wide[name] = true
	case precision:
		if *strict {
			t.Error("precision: " + text)
		}
		if _, ok := r.precision[name]; !ok {
			r.precision[name] = text
		}
	case bothFail:
		if _, ok := r.bothFail[name]; !ok {
			r.bothFail[name] = text
		}
	case evalError:
		if *strict {
			t.Error("evaluator error: " + text)
		}
		_, msg, _ := strings.Cut(text, ": ")
		msg = reNumber.ReplaceAllString(rePosition.ReplaceAllString(msg, ""), "N")
		if r.evalErrors[msg] == nil {
			r.evalErrors[msg] = map[string]string{}
		}
		if _, ok := r.evalErrors[msg][name]; !ok {
			r.evalErrors[msg][name] = text
		}
	case mismatch:
		if why, ok := known[name]; ok {
			if _, seen := r.known[name]; !seen {
				r.known[name] = text + " (" + why + ")"
			}
			return true
		}
		t.Error(text)
		r.mismatches[name]++
		if r.mismatches[name] == maxMismatches {
			t.Errorf("stopping after %d mismatches", maxMismatches)
			return false
		}
	}
	return true
}

// log writes what was collected.
func (r *report) log(t *testing.T) {
	t.Helper()
	if len(r.mismatches) > 0 {
		t.Logf("MISMATCHES in %d POUs: %v", len(r.mismatches), slices.Sorted(maps.Keys(r.mismatches)))
	}
	var open []string
	for _, name := range slices.Sorted(maps.Keys(r.known)) {
		if strings.Contains(known[name], untriaged) {
			open = append(open, name)
		}
	}
	if len(open) > 0 {
		t.Logf("%s mismatches in %d POUs: %v", untriaged, len(open), open)
	}
	// A known mismatch that is gone, with a fix of the port or of the
	// evaluator, comes off the list.
	var gone []string
	for name := range known {
		if _, seen := r.known[name]; r.tested[name] && !seen {
			gone = append(gone, name)
		}
	}
	if len(gone) > 0 {
		slices.Sort(gone)
		t.Errorf("known mismatches that agree now, to take off the list: %v", gone)
	}
	for _, name := range slices.Sorted(maps.Keys(r.known)) {
		t.Logf("known: %s: %s", name, r.known[name])
	}
	for _, name := range slices.Sorted(maps.Keys(r.bothFail)) {
		t.Logf("both fail: %s", r.bothFail[name])
	}
	for _, name := range slices.Sorted(maps.Keys(r.precision)) {
		t.Logf("precision: %s", r.precision[name])
	}
	for _, msg := range slices.Sorted(maps.Keys(r.evalErrors)) {
		names := slices.Sorted(maps.Keys(r.evalErrors[msg]))
		t.Logf("evaluator error %q in %d POUs %v, e.g. %s", msg, len(names), names, r.evalErrors[msg][names[0]])
	}
	if len(r.wide) > 0 {
		t.Logf("the evaluator does not wrap the integer results of %v", slices.Sorted(maps.Keys(r.wide)))
	}
}
