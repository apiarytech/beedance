/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

// AsBitString turns the fixed-width bit-string values a host puts in the
// I/O image (Byte, Word, DWord and LWord, as the honeycomb connector gives
// a tag's WORD) into a BitString of that width, the form both engines
// compute with, so %IW0 OR 16#0100 and lvl AND WORD#16#00FF work as they
// do for declared bit strings. Any other value is returned as it is.
func AsBitString(o Object) Object {
	switch v := o.(type) {
	case *Byte:
		return &BitString{Value: uint64(v.Value), Width: 8}
	case *Word:
		return &BitString{Value: uint64(v.Value), Width: 16}
	case *DWord:
		return &BitString{Value: uint64(v.Value), Width: 32}
	case *LWord:
		return &BitString{Value: v.Value, Width: 64}
	}
	return o
}
