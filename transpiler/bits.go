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

// This file transpiles bit access, `flags.3` (IEC 61131-3 `flags.%X3`): one
// bit of an integer or bit string variable, bit 0 the least significant.

import (
	"fmt"

	"github.com/apiarytech/beedance/ast"
)

// transpileBitRead writes a bit read as a BOOL.
func (t *Transpiler) transpileBitRead(bit *ast.BitAccessExpression) error {
	if _, err := t.bitTargetType(bit); err != nil {
		return err
	}
	target, err := t.expressionString(bit.Target)
	if err != nil {
		return err
	}
	t.write("iec.BOOL(uint64(%s)>>%d&1 == 1)", target, bit.Bit)
	return nil
}

// transpileBitWrite writes a bit assignment: the bit is set or cleared, and
// the rest of the variable kept.
func (t *Transpiler) transpileBitWrite(bit *ast.BitAccessExpression, value ast.Expression) error {
	goType, err := t.bitTargetType(bit)
	if err != nil {
		return err
	}
	target, err := t.expressionString(bit.Target)
	if err != nil {
		return err
	}
	v, err := t.expressionString(value)
	if err != nil {
		return err
	}
	t.write("\tif %s {\n\t\t%s = %s(uint64(%s) | 1<<%d)\n\t} else {\n\t\t%s = %s(uint64(%s) &^ (1 << %d))\n\t}\n",
		v, target, goType, target, bit.Bit, target, goType, target, bit.Bit)
	return nil
}

// bitTargetType returns the Go type of a bit access's target, which must be
// an integer or bit string of more bits than the bit's number.
func (t *Transpiler) bitTargetType(bit *ast.BitAccessExpression) (string, error) {
	goType := t.elementaryGoType(bit.Target)
	widths := map[string]int64{
		"iec.SINT": 8, "iec.USINT": 8, "iec.BYTE": 8,
		"iec.INT": 16, "iec.UINT": 16, "iec.WORD": 16,
		"iec.DINT": 32, "iec.UDINT": 32, "iec.DWORD": 32,
		"iec.LINT": 64, "iec.ULINT": 64, "iec.LWORD": 64,
	}
	width, ok := widths[goType]
	if !ok {
		return "", fmt.Errorf("bit access %s needs an integer or bit string variable", bit.String())
	}
	if bit.Bit < 0 || bit.Bit >= width {
		return "", fmt.Errorf("bit %d is outside the %d bits of %s", bit.Bit, width, bit.Target.String())
	}
	return goType, nil
}
