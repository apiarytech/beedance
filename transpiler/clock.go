/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// isClockCall reports whether a call is TIME(), the CODESYS function that
// reads the PLC clock, an extension to IEC 61131-3 that OSCAT's T_PLC_MS and
// T_PLC_US use.
func isClockCall(exp *ast.CallExpression) bool {
	ident, ok := exp.Function.(*ast.Identifier)
	return ok && len(exp.Arguments) == 0 && strings.EqualFold(ident.Value, "TIME")
}

// plcTimeHelper is the clock TIME() reads: the time since the program
// started, as in the evaluator and the VM.
const plcTimeHelper = `
// plcStarted is when the program started, for TIME().
var plcStarted = time.Now()

// plcTime returns the time since the program started, as TIME() does.
func plcTime() iec.TIME { return iec.TIME(time.Since(plcStarted)) }
`
