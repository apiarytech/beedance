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

import "testing"

// REF_TO and POINTER TO are Go pointers: REF and ADR are &, ^ is * and NULL
// is nil. The generated program was checked by compiling it against
// royaljelly and running it.
func TestRefToPointers(t *testing.T) {
	checkContains(t, "PROGRAM P VAR x : INT; r : REF_TO INT; q : POINTER TO INT; ok : BOOL; END_VAR r := REF(x); q := ADR(x); r^ := 3; ok := r <> NULL; r := NULL; END_PROGRAM",
		"r *iec.INT", "q *iec.INT", "p.r = &p.x", "p.q = &p.x", "(*p.r) = 3", "p.ok = (p.r != nil)", "p.r = nil")
}

// A reference to an array is a slice, so that POINTER TO ARRAY[0..32000]
// refers to arrays of any length: ADR(a) is a[:], and pt^[i] indexes from
// the lower bound of the array type pt refers to.
func TestRefToArraySlices(t *testing.T) {
	checkContains(t, `FUNCTION AVG3 : REAL VAR_INPUT pt : POINTER TO ARRAY[0..32000] OF REAL; END_VAR AVG3 := (pt^[0] + pt^[1] + pt^[2]) / 3.0; END_FUNCTION
FUNCTION FIRST1 : INT VAR_INPUT pt : REF_TO ARRAY[1..9] OF INT; END_VAR FIRST1 := pt^[1]; END_FUNCTION
PROGRAM P VAR a : ARRAY[1..3] OF REAL; y : REAL; END_VAR y := AVG3(ADR(a)); END_PROGRAM`,
		"func AVG3(pt []iec.REAL)", "pt[0] + pt[1]", "func FIRST1(pt []iec.INT)", "FIRST1 = pt[0]", "AVG3((p.a)[:])")
}
