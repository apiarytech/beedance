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
	"github.com/apiarytech/beedance/object"
	"testing"
)

func TestAliasTypeInitialValues(t *testing.T) {
	input := `
	TYPE
		MyString : STRING := 'Default';
		Greeting : MyString;
		Percent : INT(0..100) := 50;
		Offset : INT(-10..10);
		Count : INT;
	END_TYPE

	PROGRAM P
		VAR
			s1 : MyString;
			s2 : Greeting;
			s3 : MyString := 'Override';
			p1 : Percent;
			o1 : Offset;
			c1 : Count;
		END_VAR
	END_PROGRAM
	`
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, input, env); isError(result) {
		t.Fatalf("evaluation failed: %s", result.Inspect())
	}
	progObj, _ := env.Get("P")
	progEnv := progObj.(*object.Program).Env

	get := func(name string) object.Object {
		obj, ok := progEnv.Get(name)
		if !ok {
			t.Fatalf("variable %s not declared", name)
		}
		return obj
	}
	testStringObject(t, get("s1"), "s1 (type initial value)", "Default")
	testStringObject(t, get("s2"), "s2 (alias of an alias)", "Default")
	testStringObject(t, get("s3"), "s3 (own initial value wins)", "Override")
	testIntegerObject(t, get("p1"), "p1 (subrange initial value)", 50)
	testIntegerObject(t, get("o1"), "o1 (subrange lower limit)", -10)
	testIntegerObject(t, get("c1"), "c1 (base type default)", 0)
}
