/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"beedance/compiler"
	"beedance/object"
	"testing"
)

// runConfiguration compiles and runs input, then returns the hash the compiler
// built for the configuration named configName.
func runConfiguration(t *testing.T, input, configName string) *object.Hash {
	t.Helper()
	symbols := compiler.NewSymbolTable()
	comp := compiler.NewWithState(symbols, []object.Object{}, nil, nil)
	if err := comp.Compile(parse(t, input)); err != nil {
		t.Fatalf("compiler error: %s", err)
	}
	machine := New(comp.Bytecode())
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	symbol, ok := symbols.Resolve(configName)
	if !ok {
		t.Fatalf("configuration %q not defined", configName)
	}
	cfg, ok := machine.Globals()[symbol.Index].(*object.Hash)
	if !ok {
		t.Fatalf("configuration %q is not a hash: %T", configName, machine.Globals()[symbol.Index])
	}
	return cfg
}

// field returns hash[key], failing the test if it is missing.
func field(t *testing.T, hash *object.Hash, key string) object.Object {
	t.Helper()
	pair, ok := hash.Pairs[(&object.String{Value: key}).HashKey()]
	if !ok {
		t.Fatalf("key %q missing from %s", key, hash.Inspect())
	}
	return pair.Value
}

// programsByResource returns "resource/instance" -> program hash.
func programsByResource(t *testing.T, cfg *object.Hash) map[string]*object.Hash {
	t.Helper()
	out := map[string]*object.Hash{}
	for _, r := range field(t, cfg, "resources").(*object.Array).Elements {
		res := r.(*object.Hash)
		resName := field(t, res, "name").(*object.String).Value
		for _, p := range field(t, res, "programs").(*object.Array).Elements {
			prog := p.(*object.Hash)
			out[resName+"/"+field(t, prog, "instance").(*object.String).Value] = prog
		}
	}
	return out
}

const configPrograms = `
	PROGRAM Prog
		VAR COUNT : INT; END_VAR
	END_PROGRAM
`

func TestConfigurationWithStandardVarConfig(t *testing.T) {
	cfg := runConfiguration(t, configPrograms+`
	CONFIGURATION Cell
		RESOURCE Station_1 ON CPU
			TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
			PROGRAM P1 WITH Fast : Prog;
		END_RESOURCE
		RESOURCE Station_2 ON CPU
			PROGRAM P1 : Prog;
		END_RESOURCE
		VAR_CONFIG
			STATION_1.P1.COUNT : INT := 1;
			Station_2.P1.COUNT : INT := 100;
			Station_2.P1.COUNT AT %MW4 : INT;
		END_VAR
	END_CONFIGURATION`, "Cell")

	programs := programsByResource(t, cfg)
	for key, want := range map[string]int64{"Station_1/P1": 1, "Station_2/P1": 100} {
		prog, ok := programs[key]
		if !ok {
			t.Fatalf("program %s missing; have %v", key, programs)
		}
		params := field(t, prog, "params").(*object.Hash)
		if len(params.Pairs) != 1 {
			t.Fatalf("%s: expected 1 param (location-only entries are skipped), got %s", key, params.Inspect())
		}
		value, _, ok := object.GetIntegerObjectValue(field(t, params, "COUNT"))
		if !ok || value != want {
			t.Fatalf("%s: expected COUNT = %d, got %s", key, want, params.Inspect())
		}
	}
	// A program without WITH has an empty task name.
	if task := field(t, programs["Station_2/P1"], "task").(*object.String).Value; task != "" {
		t.Fatalf("expected no task for Station_2/P1, got %q", task)
	}
}

func TestSingleResourceConfigurationCompilation(t *testing.T) {
	cfg := runConfiguration(t, configPrograms+`
	CONFIGURATION Cell
		TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
		PROGRAM P1 WITH Fast : Prog;
		VAR_CONFIG P1.COUNT : INT := 7; END_VAR
	END_CONFIGURATION`, "Cell")

	resources := field(t, cfg, "resources").(*object.Array).Elements
	if len(resources) != 1 {
		t.Fatalf("expected 1 implicit resource, got %d", len(resources))
	}
	res := resources[0].(*object.Hash)
	if name := field(t, res, "name").(*object.String).Value; name != "Cell" {
		t.Fatalf("implicit resource should take the configuration's name, got %q", name)
	}
	if typ := field(t, res, "type").(*object.String).Value; typ != "" {
		t.Fatalf("implicit resource should have no type, got %q", typ)
	}
	prog := programsByResource(t, cfg)["Cell/P1"]
	if prog == nil {
		t.Fatalf("program P1 missing")
	}
	value, _, ok := object.GetIntegerObjectValue(field(t, field(t, prog, "params").(*object.Hash), "COUNT"))
	if !ok || value != 7 {
		t.Fatalf("expected COUNT = 7, got %s", prog.Inspect())
	}
}

func TestConfigurationUnmatchedVarConfig(t *testing.T) {
	comp := compiler.New()
	err := comp.Compile(parse(t, configPrograms+`
	CONFIGURATION Cell
		RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE
		VAR_CONFIG Res.Nope.COUNT : INT := 1; END_VAR
	END_CONFIGURATION`))
	want := "VAR_CONFIG path 'Res.Nope.COUNT' does not name a variable of a program instance in configuration 'Cell'"
	if err == nil || err.Error() != want {
		t.Fatalf("expected error %q, got %v", want, err)
	}
}
