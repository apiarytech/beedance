/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command plc runs program.st, compiled to bytecode, on the beedance VM the
// way a PLC does, for Scans scans 10 ms apart, and reports on its UART what
// embedded/esp32hw checks on a real board:
//
//	plc: start interval=10ms scans=600
//	pulse 1 at 1000 ms          each rising edge of %QX0.0, device time
//	...
//	stats scans=600 period_avg_us=... period_max_us=... scan_avg_us=... scan_max_us=... overruns=0
//	heap gcs=... mallocs=... total_alloc=... heap_inuse=... heap_sys=...
//	plc: done
//
// A scan whose start comes more than half an interval late is an overrun.
//
//	tinygo flash -target=esp32-coreboard-v2 -port COM6 ./embedded/esp32hw/plc
package main

//go:generate go run ./gen

import (
	"fmt"
	"runtime"
	"time"

	"github.com/apiarytech/beedance/embedded/esp32hw/plc/program"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/vm"
)

// Scans is how many scans the firmware runs: six seconds' worth.
const Scans = 600

func main() {
	fmt.Printf("plc: start interval=%v scans=%d\n", program.Interval, Scans)
	var (
		start, last          time.Time
		pulses, overruns, n  int
		periodSum, periodMax time.Duration
		scanSum, scanMax     time.Duration
		pulse                bool
	)
	err := program.Run(Scans, func(m *vm.VM) {
		// The scan began LastScan ago: time its start, as Run schedules it,
		// not its end, which a garbage collection in the scan delays.
		now := time.Now().Add(-program.LastScan)
		if n == 0 {
			start = now
		} else {
			p := now.Sub(last)
			periodSum += p
			periodMax = max(periodMax, p)
			if p > program.Interval+program.Interval/2 {
				overruns++
			}
		}
		last = now
		n++
		scanSum += program.LastScan
		scanMax = max(scanMax, program.LastScan)
		q, _ := m.IO()["%QX0.0"].(*object.Boolean)
		on := q != nil && q.Value
		if on && !pulse {
			pulses++
			fmt.Printf("pulse %d at %d ms\n", pulses, now.Sub(start).Milliseconds())
		}
		pulse = on
	})
	if err != nil {
		fmt.Printf("plc: error: %v\n", err)
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	periods := max(n-1, 1)
	fmt.Printf("stats scans=%d period_avg_us=%d period_max_us=%d scan_avg_us=%d scan_max_us=%d overruns=%d\n",
		n, (periodSum / time.Duration(periods)).Microseconds(), periodMax.Microseconds(),
		(scanSum / time.Duration(max(n, 1))).Microseconds(), scanMax.Microseconds(), overruns)
	fmt.Printf("heap gcs=%d mallocs=%d total_alloc=%d heap_inuse=%d heap_sys=%d\n",
		mem.NumGC, mem.Mallocs, mem.TotalAlloc, mem.HeapInuse, mem.HeapSys)
	fmt.Println("plc: done")
}
