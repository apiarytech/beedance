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
	"strings"
)

// directSize matches a directly represented variable (IEC 61131-3, 6.5.5),
// upper case, %([IQM])([XBWDL]?)(\d+(\.\d+)*): a location (I input, Q
// output, M memory), an optional size (X bit, B byte, W word, D double
// word, L long word; none means a bit) and a hierarchical address of
// numbers separated by dots. It returns the size, 0 for none. (Not a
// regexp: a microcontroller build would carry the regexp package and its
// Unicode tables in RAM.)
func directSize(a string) (size byte, ok bool) {
	if len(a) < 3 || a[0] != '%' || strings.IndexByte("IQM", a[1]) < 0 {
		return 0, false
	}
	rest := a[2:]
	if strings.IndexByte("XBWDL", rest[0]) >= 0 {
		size, rest = rest[0], rest[1:]
	}
	if rest == "" {
		return 0, false
	}
	for _, part := range strings.Split(rest, ".") {
		if part == "" {
			return 0, false
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return 0, false
			}
		}
	}
	return size, true
}

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
	size, ok := directSize(a)
	if !ok {
		return "", false
	}
	switch size {
	case 'B':
		return "BYTE", true
	case 'W':
		return "WORD", true
	case 'D':
		return "DWORD", true
	case 'L':
		return "LWORD", true
	}
	return "BOOL", true
}

// IsInputAddress reports whether address is in the input area (%I...),
// which a program reads but never writes.
func IsInputAddress(address string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(address)), "%I")
}
