/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

import (
	"time"

	"beedance/object"
)

// started is when the clock starts, as the program loads.
var started = time.Now()

// Clock returns the time since the clock started. Tests replace it to
// control the standard timers.
var Clock = func() time.Duration { return time.Since(started) }

// clockFn returns the clock as a TIME.
func clockFn(name string) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 0 {
			return object.NewBuiltinError("%s takes no arguments, got %d", name, len(args))
		}
		return &object.Time{Value: Clock()}
	}
}

func init() {
	// __CLOCK() is the clock the compiled standard timers (TON, TOF, TP)
	// read. Its name is not an IEC identifier, so no program can clash with it.
	object.RegisterBuiltin(BuiltinClock, "__CLOCK", clockFn("__CLOCK"))
	// TIME() is the same clock under the name CODESYS gives it, which OSCAT
	// reads in T_PLC_MS and T_PLC_US. It is an extension to IEC 61131-3.
	object.RegisterBuiltin(BuiltinTime, "TIME", clockFn("TIME"))
}
