/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command all runs both workloads on all three engines, one after another:
// one flash measures them all. Build it with -stack-size=16KB, which the
// evaluator needs.
package main

import (
	"github.com/apiarytech/beedance/embedded/bench/eng/eval"
	"github.com/apiarytech/beedance/embedded/bench/eng/goeng"
	"github.com/apiarytech/beedance/embedded/bench/eng/vm"
	"github.com/apiarytech/beedance/embedded/bench/report"
)

func main() {
	report.Begin()
	goeng.Bench()
	vm.Bench()
	eval.Bench()
	report.End()
	select {}
}
