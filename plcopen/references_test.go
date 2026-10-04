/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

import (
	"strings"
	"testing"
)

// REF_TO, POINTER TO and REFERENCE TO are all TC6 <pointer>, and come back
// as they were written: POINTER TO from the <pointer> itself, the others
// from the type beedance keeps in additional data.
func TestReferenceTypesRoundTrip(t *testing.T) {
	src := `TYPE
	PInt : POINTER TO INT;
	RInt : REF_TO INT;
	Pair : STRUCT a : REF_TO REAL; b : POINTER TO REAL; END_STRUCT;
END_TYPE
FUNCTION_BLOCK Fb
VAR_INPUT
	pt : POINTER TO ARRAY[0..9] OF REAL;
	r : REF_TO INT;
	ref : REFERENCE TO INT;
	plain : INT;
END_VAR
END_FUNCTION_BLOCK`
	proj, err := ExportSource(src, "Refs")
	if err != nil {
		t.Fatal(err)
	}
	out, err := marshalProject(proj)
	if err != nil {
		t.Fatal(err)
	}
	xmlText := string(out)
	if n := strings.Count(xmlText, "<pointer>"); n != 7 {
		t.Errorf("%d <pointer> types, want 7:\n%s", n, xmlText)
	}
	// Only the types <pointer> does not say exactly carry beedance's data.
	if n := strings.Count(xmlText, STTypeData+`" handleUnknown="discard"`); n != 4 {
		t.Errorf("%d st-type entries, want 4 (RInt, Pair.a, r, ref)", n)
	}
	if !strings.Contains(xmlText, `<info name="`+STTypeData+`" version="1.0" vendor="https://apiarytech.io">`) {
		t.Error("beedance's additional data is not declared in addDataInfo")
	}

	text, err := ImportToIECText(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PInt : POINTER TO INT;", "RInt : REF_TO INT;", "a : REF_TO REAL;", "b : POINTER TO REAL;",
		"pt : POINTER TO ARRAY [0..9] OF REAL;", "r : REF_TO INT;", "ref : REFERENCE TO INT;", "plain : INT;",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if _, err := ImportToAST(out); err != nil {
		t.Errorf("the imported text does not parse: %v", err)
	}
}

// A <pointer> from another tool, without beedance's data, is POINTER TO.
func TestForeignPointerIsPointerTo(t *testing.T) {
	foreign := strings.Replace(toolProject,
		`<variable name="Run"><type><BOOL/></type></variable>`,
		`<variable name="Run"><type><BOOL/></type></variable><variable name="p"><type><pointer><baseType><INT/></baseType></pointer></type></variable>`, 1)
	text, err := ImportToIECText([]byte(foreign))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "p : POINTER TO INT;") {
		t.Errorf("got:\n%s", text)
	}
	// Kept data that does not decode falls back to the <pointer>.
	bad, _ := (*AddData)(nil).Set(STTypeData, HandleDiscard, struct {
		X string `xml:"other"`
	}{})
	if got := typeText(DataType{Pointer: &PointerType{BaseType: &DataType{INT: &EmptyTag{}}}}, bad); got != "POINTER TO INT" {
		t.Errorf("got %q", got)
	}
}
