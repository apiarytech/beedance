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
	"encoding/csv"
	"io"
	"sort"
	"strconv"
)

// WriteCSV writes an engine's run of a test as CSV: a header of "scan" and
// the variable names in alphabetical order, then a row per scan.
func WriteCSV(w io.Writer, vars []Variable, r EngineResult) error {
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		names = append(names, v.Name)
	}
	sort.Strings(names)
	cw := csv.NewWriter(w)
	if err := cw.Write(append([]string{"scan"}, names...)); err != nil {
		return err
	}
	for i, s := range r.Scans {
		row := []string{strconv.Itoa(i + 1)}
		for _, n := range names {
			row = append(row, s[n])
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
