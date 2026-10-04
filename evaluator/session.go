/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

import (
	"sync"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

var sessionMu sync.Mutex

// Session makes now the evaluator's clock, which the standard timers and
// SFC action qualifiers read, and gives the evaluator an empty I/O image for
// located variables, until end is called. A host that simulates programs on a
// clock of its own (an engineering tool stepping through scans) runs them in
// a session.
//
// The clock and the I/O image belong to the package, so sessions run one at
// a time: Session waits until the previous session has ended.
func Session(now func() time.Time) (end func()) {
	sessionMu.Lock()
	prevNow, prevIO, prevTypes := nowFunc, ioMap, ioTypes
	nowFunc, ioMap, ioTypes = now, make(map[string]object.Object), make(map[string]string)
	return func() {
		nowFunc, ioMap, ioTypes, traceFn = prevNow, prevIO, prevTypes, nil
		budget, remaining = 0, 0
		sessionMu.Unlock()
	}
}

// IO returns the I/O image of the current session: the values of located
// variables by address.
func IO() map[string]object.Object { return ioMap }

// IOTypes returns the declared type of each located variable of the current
// session by address, e.g. "INT" for `x AT %IW1 : INT`, as the declaration
// writes it. A host exchanging values with IO converts them to these types:
// %IW1 may hold an INT, not only a WORD.
func IOTypes() map[string]string { return ioTypes }

// traceFn, when set, is called before each statement of a statement list is
// evaluated (see Trace).
var traceFn func(stmt ast.Statement, env *object.Environment)

// Trace calls fn before each statement the evaluator executes, with the
// environment it executes in, until the session ends or Trace(nil) is
// called. An engineering tool records a scan statement by statement with it:
// what changes between one call and the next is the effect of the statement
// of the first call (for an IF or a loop, of its condition). Statements in
// called function blocks and functions are reported too, in their own
// environments. Call it only within a Session.
func Trace(fn func(stmt ast.Statement, env *object.Environment)) { traceFn = fn }

// budget bounds the statements and loop iterations the evaluator executes
// (SetBudget); remaining counts down. Session-wide, like the clock.
var budget, remaining int64

// SetBudget bounds what the evaluator executes from now on to n statements
// and loop iterations; 0 or less means no bound. Past it, evaluation stops
// with an "execution budget exceeded" error, so a program that loops for ever
// fails instead of holding the session. A host simulating scans sets it
// before each scan. It is reset when the session ends.
func SetBudget(n int64) { budget, remaining = n, n }

// overBudget counts one statement or loop iteration and reports whether the
// budget is spent.
func overBudget() bool {
	if budget <= 0 {
		return false
	}
	remaining--
	return remaining < 0
}
