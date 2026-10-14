/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package vm runs the workloads on the bytecode VM, as embedded firmware
// does (embedded.GenerateEmbeddedProgram).
package vm

import (
	"fmt"

	"github.com/apiarytech/beedance/embedded/bench/report"
	"github.com/apiarytech/beedance/embedded/bench/vmcalc"
	"github.com/apiarytech/beedance/embedded/bench/vmplc"

	"github.com/apiarytech/beedance/object"
	bvm "github.com/apiarytech/beedance/vm"
)

// Bench runs and reports both workloads.
func Bench() {
	t := report.Start()
	pulses, on := 0, false
	err := vmplc.Run(report.Scans, func(m *bvm.VM) {
		t.Add(vmplc.LastScan)
		q, _ := m.IO()["%QX0.0"].(*object.Boolean)
		if q != nil && q.Value && !on {
			pulses++
		}
		on = q != nil && q.Value
	})
	t.Print("vm", "plc", fmt.Sprintf("pulses:%d,err:%v", pulses, err))

	t = report.Start()
	err = vmcalc.Run(report.Scans, func(*bvm.VM) { t.Add(vmcalc.LastScan) })
	t.Print("vm", "calc", fmt.Sprintf("err:%v", err))
}

// Calc runs only the calc workload, for n scans, reporting nothing: for a
// profiler (gdbprof).
func Calc(n int) { vmcalc.Run(n, nil) }

// PLC is Calc for the PLC workload.
func PLC(n int) { vmplc.Run(n, nil) }
