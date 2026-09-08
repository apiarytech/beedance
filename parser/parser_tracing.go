/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package parser

import (
	"fmt"
	"strings"
)

var (
	// traceLevel tracks the current indentation level for nested function calls.
	traceLevel int = 0
	// enableTracing is a global flag that turns parser tracing on or off.
	enableTracing bool = false
)

// traceIdentPlaceholder is the character used for a single level of indentation.
const traceIdentPlaceholder string = "\t"

// identLevel returns a string of indentation characters corresponding to the current traceLevel.
func identLevel() string {
	if !enableTracing {
		return ""
	}
	return strings.Repeat(traceIdentPlaceholder, traceLevel)
}

// tracePrint prints a formatted string with the correct indentation level if tracing is enabled.
func tracePrint(fs string) {
	if !enableTracing {
		return
	}
	fmt.Printf("%s%s\n", identLevel(), fs)
}

// incIdent increments the indentation level for tracing.
func incIdent() {
	if enableTracing {
		traceLevel = traceLevel + 1
	}
}

// decIdent decrements the indentation level for tracing.
func decIdent() {
	if enableTracing {
		traceLevel = traceLevel - 1
	}
}

// trace is called at the beginning of a function to print a "BEGIN" message
// and increase the indentation level. It's designed to be used with `defer untrace()`.
func trace(msg string) string {
	tracePrint("BEGIN " + msg)
	incIdent()
	return msg
}

// untrace is called via `defer` at the end of a traced function to print an "END"
// message and decrease the indentation level.
func untrace(msg string) {
	decIdent()
	tracePrint("END " + msg)
}

// SetTracing enables or disables the parser tracing.
func SetTracing(enabled bool) {
	enableTracing = enabled
}
