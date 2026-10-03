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

	"github.com/apiarytech/beedance/object"
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

// ClockBuiltins returns the clock built-ins (__CLOCK, which the standard
// timers read, and TIME) reading clock instead of the process clock, by
// built-in index. A host that runs programs on its own schedule (royaljelly's
// tasks) gives each VM its task's clock with vm.SetBuiltin, so the timers of
// one program follow its task, not the process.
func ClockBuiltins(clock func() time.Duration) map[int]*object.Builtin {
	fn := func(name string) *object.Builtin {
		return &object.Builtin{Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return object.NewBuiltinError("%s takes no arguments, got %d", name, len(args))
			}
			return &object.Time{Value: clock()}
		}}
	}
	return map[int]*object.Builtin{BuiltinClock: fn("__CLOCK"), BuiltinTime: fn("TIME")}
}

func init() {
	// __CLOCK() is the clock the compiled standard timers (TON, TOF, TP)
	// read. Its name is not an IEC identifier, so no program can clash with it.
	object.RegisterBuiltin(BuiltinClock, "__CLOCK", clockFn("__CLOCK"))
	// TIME() is the same clock under the name CODESYS gives it, which OSCAT
	// reads in T_PLC_MS and T_PLC_US. It is an extension to IEC 61131-3.
	object.RegisterBuiltin(BuiltinTime, "TIME", clockFn("TIME"))
}
