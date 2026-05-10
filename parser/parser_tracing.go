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
	traceLevel    int  = 0
	enableTracing bool = false // New flag to control tracing
)

const traceIdentPlaceholder string = "\t"

func identLevel() string {
	if !enableTracing {
		return ""
	}
	return strings.Repeat(traceIdentPlaceholder, traceLevel-1)
}

func tracePrint(fs string) {
	if !enableTracing {
		return
	}
	fmt.Printf("%s%s\n", identLevel(), fs)
}

func incIdent() {
	if enableTracing {
		traceLevel = traceLevel + 1
	}
}
func decIdent() {
	if enableTracing {
		traceLevel = traceLevel - 1
	}
}

func trace(msg string) string {
	incIdent()
	tracePrint("BEGIN " + msg)
	return msg
}

func untrace(msg string) {
	tracePrint("END " + msg)
	decIdent()

}

// SetTracing enables or disables the parser tracing.
func SetTracing(enabled bool) {
	enableTracing = enabled
}
