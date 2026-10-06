/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast

import (
	"regexp"
	"strings"
)

// directAddress is a directly represented variable (IEC 61131-3, 6.5.5):
// a location (I input, Q output, M memory), an optional size (X bit, B
// byte, W word, D double word, L long word; none means a bit) and a
// hierarchical address of numbers separated by dots.
var directAddress = regexp.MustCompile(`^%([IQM])([XBWDL]?)(\d+(?:\.\d+)*)$`)

// FullAddress returns the variable's address with its '%', upper-cased:
// "%IW0", "%QX0.1".
func (dv *DirectVariable) FullAddress() string {
	return "%" + strings.ToUpper(dv.Address)
}

// DirectType returns the elementary type a directly represented variable
// has by its size: BOOL for X or no size, BYTE, WORD, DWORD and LWORD for
// B, W, D and L. ok is false when address is not a directly represented
// variable. address may be written with or without its '%'.
func DirectType(address string) (typ string, ok bool) {
	a := strings.ToUpper(strings.TrimSpace(address))
	if !strings.HasPrefix(a, "%") {
		a = "%" + a
	}
	m := directAddress.FindStringSubmatch(a)
	if m == nil {
		return "", false
	}
	switch m[2] {
	case "B":
		return "BYTE", true
	case "W":
		return "WORD", true
	case "D":
		return "DWORD", true
	case "L":
		return "LWORD", true
	}
	return "BOOL", true
}

// IsInputAddress reports whether address is in the input area (%I...),
// which a program reads but never writes.
func IsInputAddress(address string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(address)), "%I")
}
