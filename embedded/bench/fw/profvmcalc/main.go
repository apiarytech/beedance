/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command profvmcalc runs the calc workload on the VM for ever, for gdbprof to
// sample. It prints "profile: start" when it begins.
package main

import "github.com/apiarytech/beedance/embedded/bench/eng/vm"

func main() {
	println("profile: start")
	vm.Calc(1 << 30)
}
