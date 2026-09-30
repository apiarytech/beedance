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

func init() {
	// __CLOCK() is the clock the compiled standard timers (TON, TOF, TP)
	// read. Its name is not an IEC identifier, so no program can clash with it.
	object.RegisterBuiltin(BuiltinClock, "__CLOCK", func(args ...object.Object) object.Object {
		if len(args) != 0 {
			return object.NewBuiltinError("__CLOCK takes no arguments, got %d", len(args))
		}
		return &object.Time{Value: Clock()}
	})
}
