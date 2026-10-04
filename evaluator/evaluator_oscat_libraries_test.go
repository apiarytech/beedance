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
	"math"
	"os"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// TestOscatBuildingAndNetworkRun loads OSCAT BASIC with the BUILDING and
// NETWORK libraries and calls some of their functions. The expected values
// are those of beebread's Go ports (building/hlk, network/encoding), so the
// evaluator and the ports agree.
func TestOscatBuildingAndNetworkRun(t *testing.T) {
	var src strings.Builder
	for _, path := range []string{
		"../reference/beedance_oscat_basic.st",
		"../reference/beedance_building_100.st",
		"../reference/beedance_network_135.st",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("library not present: %v", err)
		}
		src.Write(data)
		src.WriteString("\n")
	}
	p := parser.New(lexer.New(src.String()))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("the libraries do not parse: %v", p.Errors()[0])
	}
	env := object.NewEnvironment()
	if result := Eval(program, env); isError(result) {
		t.Fatalf("the libraries do not evaluate: %s", result.Inspect())
	}
	eval := func(call string) object.Object {
		p := parser.New(lexer.New(call + ";"))
		return Eval(p.ParseProgram(), env)
	}

	// IP4_DECODE writes its own input (CODESYS allows it).
	if got := eval("IP4_DECODE('192.168.1.20')"); got.Inspect() != "DWORD#16#C0A80114" {
		t.Errorf("IP4_DECODE('192.168.1.20') = %s, want DWORD#16#C0A80114", got.Inspect())
	}
	if got := eval("IP4_TO_STRING(DWORD#16#C0A80114)"); got.Inspect() != "192.168.1.20" {
		t.Errorf("IP4_TO_STRING = %s, want 192.168.1.20", got.Inspect())
	}
	for call, want := range map[string][2]float64{
		"DEW_TEMP(60.0, 25.0)":   {16.7, 0.1},
		"HEAT_INDEX(30.0, 70.0)": {35.0, 0.5}, // writes its input T
		"HEAT_INDEX(15.0, 70.0)": {15, 0},
	} {
		v := eval(call)
		got, ok := object.GetFloat64Value(v)
		if !ok || math.Abs(got-want[0]) > want[1] {
			t.Errorf("%s = %s, want %v", call, v.Inspect(), want[0])
		}
	}
	if b, ok := eval("NETWORK_VERSION(FALSE)").(*object.BitString); !ok || b.Value != 135 {
		t.Errorf("NETWORK_VERSION(FALSE) = %v, want DWORD 135", b)
	}
}
