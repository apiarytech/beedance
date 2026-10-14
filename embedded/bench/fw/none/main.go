/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command none is the baseline of the size measurements: TinyGo's runtime,
// fmt and the reporter, with no engine.
package main

import "github.com/apiarytech/beedance/embedded/bench/report"

func main() {
	report.Begin()
	t := report.Start()
	t.Add(0)
	t.Print("none", "none", "-")
	report.End()
	select {}
}
