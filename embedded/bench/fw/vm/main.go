/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command vm runs both workloads on one engine only, so its size can be
// measured on its own (tinygo build -size short).
package main

import (
	"github.com/apiarytech/beedance/embedded/bench/eng/vm"
	"github.com/apiarytech/beedance/embedded/bench/report"
)

func main() {
	report.Begin()
	vm.Bench()
	report.End()
	select {}
}
