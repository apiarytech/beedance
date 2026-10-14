/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package goeng runs the workloads as transpiled Go.
package goeng

import (
	"fmt"
	"time"

	"github.com/apiarytech/beedance/embedded/bench/gocalc"
	"github.com/apiarytech/beedance/embedded/bench/goplc"
	"github.com/apiarytech/beedance/embedded/bench/report"
)

// Bench runs and reports both workloads.
func Bench() {
	p := goplc.NewMain()
	vars := p.Variables()
	t := report.Start()
	pulses, on := 0, false
	for n := 0; n < report.Scans; n++ {
		b := time.Now()
		p.Logic(b)
		t.Add(time.Since(b))
		for _, v := range vars {
			if v.Name == "pulse" {
				q := fmt.Sprint(v.Get()) == "true"
				if q && !on {
					pulses++
				}
				on = q
			}
		}
	}
	t.Print("go", "plc", fmt.Sprintf("pulses:%d", pulses))

	c := gocalc.NewMain()
	t = report.Start()
	for n := 0; n < report.Scans; n++ {
		b := time.Now()
		c.Logic(b)
		t.Add(time.Since(b))
	}
	acc := ""
	for _, v := range c.Variables() {
		if v.Name == "acc" {
			acc = fmt.Sprint(v.Get())
		}
	}
	t.Print("go", "calc", "acc:"+acc)
}
