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
	"testing"
)

func TestReferenceTypeTranspilation(t *testing.T) {
	input := `
PROGRAM ReferenceTest
	VAR
		myInt : INT := 10;
		myRef : REFERENCE TO INT;
		anotherInt : INT;
	END_VAR

	myRef := myInt;
	anotherInt := myRef;
	myRef^ := 20;
END_PROGRAM
`
	expected := `
type ReferenceTest struct {
	myInt      iec.INT
	myRef      *iec.INT
	anotherInt iec.INT
}

// NewReferenceTestFactory creates a new instance of the ReferenceTest program.
func NewReferenceTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &ReferenceTest{}
	instance.myInt = 10
	return instance.Logic, nil
}

func (p *ReferenceTest) Logic(now time.Time) {
	p.myRef = &p.myInt
	p.anotherInt = (*p.myRef)
	(*p.myRef) = 20
}
`
	transpileAndCheck(t, "TestReferenceTypeTranspilation", input, expected)
}
