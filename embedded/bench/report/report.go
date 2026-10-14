/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package report times an engine's scans and prints one line per engine and
// workload on the console (a microcontroller's UART):
//
//	bench engine=vm workload=calc scans=500 avg_us=... min_us=... max_us=... wall_ms=... gcs=... mallocs=... alloc_bytes=... heap_inuse=... result=...
//
// gcs, mallocs and alloc_bytes are what the run added; result is what the
// engine computed, so the engines can be compared.
package report

import (
	"fmt"
	"runtime"
	"strconv"
	"time"
)

// Scans is how many scans each engine runs of each workload, back to back.
// Building with -ldflags="-X github.com/apiarytech/beedance/embedded/bench/report.scans=N"
// changes it: an emulator is slower than a board.
var Scans = 500

var scans string

func init() {
	if n, err := strconv.Atoi(scans); err == nil && n > 0 {
		Scans = n
	}
}

// Timer sums and bounds the scan times of one run.
type Timer struct {
	n             int
	sum, min, max time.Duration
	mem           runtime.MemStats
	began         time.Time
}

// Start begins a run.
func Start() *Timer {
	t := &Timer{min: time.Hour}
	runtime.ReadMemStats(&t.mem)
	t.began = time.Now()
	return t
}

// Add records one scan's time.
func (t *Timer) Add(d time.Duration) {
	t.n++
	t.sum += d
	t.min = min(t.min, d)
	t.max = max(t.max, d)
}

// Print reports the run.
func (t *Timer) Print(engine, workload, result string) {
	wall := time.Since(t.began)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	avg := t.sum / time.Duration(max(t.n, 1))
	fmt.Printf("bench engine=%s workload=%s scans=%d avg_us=%d min_us=%d max_us=%d wall_ms=%d gcs=%d mallocs=%d alloc_bytes=%d heap_inuse=%d result=%s\n",
		engine, workload, t.n, avg.Microseconds(), t.min.Microseconds(), t.max.Microseconds(), wall.Milliseconds(),
		m.NumGC-t.mem.NumGC, m.Mallocs-t.mem.Mallocs, m.TotalAlloc-t.mem.TotalAlloc, m.HeapInuse, result)
}

// Begin starts the output, after a pause that lets a host that has just
// reset the board open its serial port.
func Begin() {
	time.Sleep(500 * time.Millisecond)
	fmt.Println("bench: start")
}

// End ends the output.
func End() { fmt.Println("bench: done") }
