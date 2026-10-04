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
	"testing"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// Program instances of a configuration run IL, SFC and ST bodies with
// VAR_TEMP, and map their outputs.
func TestProgramInstanceBodies(t *testing.T) {
	const config = `
PROGRAM IlProg
	VAR_OUTPUT n : INT; END_VAR
	LD n
	ADD 1
	ST n
END_PROGRAM
PROGRAM TempProg
	VAR_TEMP t1 : INT := 10; END_VAR
	VAR_OUTPUT o : INT; END_VAR
	t1 := t1 + 1;
	o := t1;
END_PROGRAM
PROGRAM SfcProg
	VAR go1 : BOOL := TRUE; steps : INT; END_VAR
	INITIAL_STEP S1: A1(N); END_STEP
	TRANSITION FROM S1 TO S2 := go1; END_TRANSITION
	STEP S2: END_STEP
	ACTION A1: steps := steps + 1; END_ACTION
END_PROGRAM
CONFIGURATION Cell
	RESOURCE R1 ON CPU
		TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
		PROGRAM I1 WITH Fast : IlProg;
		PROGRAM Tp WITH Fast : TempProg (o => seen);
		PROGRAM Sq WITH Fast : SfcProg;
	END_RESOURCE
END_CONFIGURATION
`
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, config, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}
	s, err := NewScheduler(env)
	if err != nil {
		t.Fatalf("NewScheduler failed: %s", err.Inspect())
	}
	start := time.Now()
	runSchedulerCycle(s, env, start)
	runSchedulerCycle(s, env, start.Add(20*time.Millisecond))

	if n, _ := programInstanceEnv(t, env, "R1", "I1").Get("n"); n.Inspect() != "2" {
		t.Errorf("IL program instance: n = %s, want 2", n.Inspect())
	}
	// VAR_TEMP starts afresh on every run.
	if o, _ := programInstanceEnv(t, env, "R1", "Tp").Get("o"); o.Inspect() != "11" {
		t.Errorf("VAR_TEMP: o = %s, want 11", o.Inspect())
	}
	if seen, _ := env.Get("seen"); seen == nil || seen.Inspect() != "11" {
		t.Errorf("output mapping: seen = %v, want 11", seen)
	}
	if steps, _ := programInstanceEnv(t, env, "R1", "Sq").Get("steps"); steps.Inspect() == "0" {
		t.Errorf("the SFC program instance did not run its initial step's action")
	}
}

// Errors in a configuration's program instances are reported.
func TestProgramConfigurationErrors(t *testing.T) {
	checkEval(t, []evalCase{
		{"CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Nope; END_RESOURCE END_CONFIGURATION", "ERROR: program type 'Nope' not defined"},
		{"VAR F : INT; END_VAR CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : F; END_RESOURCE END_CONFIGURATION", "ERROR: 'F' is not a PROGRAM"},
		{"PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := 5, PRIORITY := 1); END_RESOURCE END_CONFIGURATION", "ERROR: task INTERVAL must be of type TIME"},
		{"PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := T#1s, PRIORITY := TRUE); END_RESOURCE END_CONFIGURATION", "ERROR: task PRIORITY must be of type INTEGER"},
		{"PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := missing, PRIORITY := 1); END_RESOURCE END_CONFIGURATION", "ERROR: identifier not found: missing"},
		{"PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := T#1s, PRIORITY := missing); END_RESOURCE END_CONFIGURATION", "ERROR: identifier not found: missing"},
	})
}

// A program instance binds its VAR_EXTERNALs to the resource's globals.
func TestProgramInstanceExternals(t *testing.T) {
	const config = `
PROGRAM P
	VAR_EXTERNAL shared : INT; END_VAR
	shared := shared + 1;
END_PROGRAM
CONFIGURATION C
	RESOURCE Res ON CPU
		VAR_GLOBAL shared : INT := 5; END_VAR
		TASK T1(INTERVAL := T#10ms, PRIORITY := 1);
		PROGRAM P1 WITH T1 : P;
	END_RESOURCE
END_CONFIGURATION
`
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, config, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}
	s, err := NewScheduler(env)
	if err != nil {
		t.Fatalf("NewScheduler failed: %s", err.Inspect())
	}
	runSchedulerCycle(s, env, time.Now())
	resObj, _ := env.Get("Res")
	if shared, _ := resObj.(*object.FunctionBlockInstance).Env.Get("shared"); shared.Inspect() != "6" {
		t.Errorf("shared = %s, want 6", shared.Inspect())
	}
}

// VAR_CONFIG reaches variables of function block instances in a program.
func TestVarConfigIntoFunctionBlocks(t *testing.T) {
	const config = `
FUNCTION_BLOCK Fb VAR limit : INT; END_VAR END_FUNCTION_BLOCK
PROGRAM P VAR f : Fb; x : INT; END_VAR END_PROGRAM
CONFIGURATION C
	RESOURCE Res ON CPU PROGRAM P1 : P; END_RESOURCE
	VAR_CONFIG
		Res.P1.f.limit : INT := 7;
	END_VAR
END_CONFIGURATION
`
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, config, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}
	f, _ := programInstanceEnv(t, env, "Res", "P1").Get("f")
	if limit, _ := f.(*object.FunctionBlockInstance).Env.Get("limit"); limit.Inspect() != "7" {
		t.Errorf("f.limit = %s, want 7", limit.Inspect())
	}
	checkEval(t, []evalCase{
		{`PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : P; END_RESOURCE
			VAR_CONFIG Res.P1.x.y : INT := 7; END_VAR END_CONFIGURATION`, "ERROR: 'x' in VAR_CONFIG path"},
	})
}

// IL conditions treat numbers and other values as truthy.
func TestIlTruthiness(t *testing.T) {
	checkIlEval(t, []evalCase{
		{ilProgram("LD 5\nJMPC yes\nLD 0\nRET\nyes: LD 1"), "1"},
		{ilProgram("LD 0\nJMPC yes\nLD 0\nRET\nyes: LD 1"), "0"},
		{ilProgram("LD 2.5\nJMPC yes\nLD 0\nRET\nyes: LD 1"), "1"},
		{ilProgram("LD 'txt'\nJMPC yes\nLD 0\nRET\nyes: LD 1"), "1"},
		{ilProgram("LD UINT#3\nJMPC yes\nLD 0\nRET\nyes: LD 1"), "1"},
		{ilProgram("LD REAL#0.0\nJMPC yes\nLD 0\nRET\nyes: LD 1"), "0"},
	})
}

// SFC steps and actions have members: a step's X and T, an action's Q.
func TestSfcMembers(t *testing.T) {
	const sfc = `
PROGRAM Seq
	VAR active : BOOL; elapsed : TIME; running : BOOL; END_VAR
	INITIAL_STEP S1: A1(N); END_STEP
	TRANSITION FROM S1 TO S2 := FALSE; END_TRANSITION
	STEP S2: END_STEP
	ACTION A1: active := S1.X; elapsed := S1.T; running := A1.Q; END_ACTION
END_PROGRAM
Seq();
Seq.active;`
	testBooleanObject(t, testEval(t, sfc), "S1.X", true)
	checkEval(t, []evalCase{
		{`PROGRAM Seq VAR v : BOOL; END_VAR INITIAL_STEP S1: A1(N); END_STEP ACTION A1: v := S1.Nope; END_ACTION END_PROGRAM Seq();`, "ERROR: member 'Nope' not found for type STEP"},
		{`PROGRAM Seq VAR v : BOOL; END_VAR INITIAL_STEP S1: A1(N); END_STEP ACTION A1: v := A1.Nope; END_ACTION END_PROGRAM Seq();`, "ERROR: member 'Nope' not found for type ACTION"},
		// A step that is not active has no elapsed time.
		{`PROGRAM Seq VAR t1 : TIME := T#5s; END_VAR INITIAL_STEP S1: A1(N); END_STEP STEP S2: END_STEP TRANSITION FROM S1 TO S2 := FALSE; END_TRANSITION ACTION A1: t1 := S2.T; END_ACTION END_PROGRAM Seq(); Seq.t1;`, "T#0s"},
	})
}

// Identifiers that name conversions and typed literals.
func TestSpecialIdentifiers(t *testing.T) {
	checkEval(t, []evalCase{
		{"DINT_TO_LINT(5);", "5"},
		{"VAR x : INT; END_VAR x := -INT#3; x;", "-3"},
		{"VAR b : BOOL; END_VAR b := -TRUE;", "ERROR: unknown operator: - BOOLEAN"},
		{"VAR t1 : TIME; END_VAR t1 := T#5q;", `ERROR: unknown duration unit "q"`},
	})
}

// Standard function blocks report inputs of the wrong type.
func TestStandardFunctionBlockInputErrors(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR t1 : TON; END_VAR t1(IN := 5, PT := T#1s);", "ERROR: TON requires IN (BOOL) and PT (TIME) inputs"},
		{"VAR t1 : TOF; END_VAR t1(IN := TRUE, PT := 5);", "ERROR: TOF requires IN (BOOL) and PT (TIME) inputs"},
		{"VAR t1 : TP; END_VAR t1(IN := TRUE, PT := 5);", "ERROR: TP requires IN (BOOL) and PT (TIME) inputs"},
		{"VAR c : CTU; END_VAR c(CU := 1, R := FALSE, PV := 3);", "ERROR: CTU requires CU (BOOL), R (BOOL), and PV (any INT type) inputs"},
		{"VAR c : CTD; END_VAR c(CD := TRUE, LD := FALSE, PV := TRUE);", "ERROR: CTD requires a PV (Preset Value) input of an integer type"},
		{"VAR c : CTUD; END_VAR c(CU := 1, CD := FALSE, R := FALSE, LD := FALSE, PV := 3);", "ERROR: CTUD requires CU, CD, R, LD (BOOL) and PV (INT) inputs"},
		{"VAR f : SR; END_VAR f(S1 := 1, R := FALSE);", "ERROR: SR requires S1 (BOOL) and R (BOOL) inputs"},
		{"VAR f : RS; END_VAR f(S := 1, R1 := FALSE);", "ERROR: RS requires S (BOOL) and R1 (BOOL) inputs"},
	})
}

// Stored timed actions keep timing after their step deactivates: SD becomes
// active when its delay ends, and SL becomes inactive when its limit ends.
func TestStoredTimedActionsAfterTheirStep(t *testing.T) {
	mockTime = time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	originalNowFunc := nowFunc
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, `
		PROGRAM Seq
			VAR Delayed, Limited, Go2 : BOOL; END_VAR
			INITIAL_STEP S1: Delayed(SD, T#2s); Limited(SL, T#2s); END_STEP
			TRANSITION FROM S1 TO S2 := Go2; END_TRANSITION
			STEP S2: END_STEP
		END_PROGRAM`, env)
	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("expected an SFC object, got %T", sfcObj)
	}
	progObj, _ := env.Get("Seq")
	progEnv := progObj.(*object.Program).Env
	initializeActionVars(t, sfc, progEnv)

	evalSFCCycle(sfc, progEnv) // S1 active: the SL action runs, the SD action waits.
	progEnv.Set("Go2", TRUE)
	evalSFCCycle(sfc, progEnv) // S1 deactivates.
	advanceMockTime(3 * time.Second)
	evalSFCCycle(sfc, progEnv)
	if !sfc.Actions["Delayed"].IsActive {
		t.Error("the SD action did not become active after its delay")
	}
	if sfc.Actions["Limited"].IsActive {
		t.Error("the SL action is still active after its limit")
	}
}

// Assorted branches of evaluation: literals, SFC and located variables,
// function block inheritance, durations, CASE and conversions.
func TestMoreEvaluationBranches(t *testing.T) {
	checkEval(t, []evalCase{
		{"16#FF;", "255"},
		{"VAR a : ARRAY[0..1] OF INT; END_VAR a[missing];", "ERROR: identifier not found: missing"},
		{"missing[1];", "ERROR: identifier not found: missing"},
		{"PROGRAM Seq STEP S1: END_STEP END_PROGRAM", "ERROR: SFC program has no initial step"},
		{"PROGRAM Seq VAR x : INT; END_VAR INITIAL_STEP S1: A1(N); END_STEP ACTION A1: x := missing; END_ACTION END_PROGRAM Seq();", "ERROR: identifier not found: missing"},
		// A structure's member may be assigned by its name as an index.
		{"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR p : Pt; END_VAR p['px'] := 5; p.px;", "5"},
		// A located output keeps its initial value; an input the host has not
		// set starts with its type's default.
		{"VAR q AT %QX0.0 : BOOL := TRUE; END_VAR q;", "true"},
		{"VAR x AT %IX0.3 : BOOL; END_VAR x;", "false"},
		{"VAR t1 : TIME; END_VAR t1 := T#5;", `ERROR: invalid duration format in "5"`},
		{"VAR x : INT; END_VAR CASE x OF missing: x := 1; END_CASE", "ERROR: identifier not found: missing"},
		{"VAR x : INT := 2; y : INT; END_VAR CASE x OF 1..3: y := 1; END_CASE y;", "1"},
		{"FOO_TO_BAR(1);", "ERROR: type mismatch for FOO_TO_BAR"},
		{"TON();", "FUNCTION_BLOCK_INSTANCE(standard)"},
		{"VAR x : INT; END_VAR 5 := x;", "ERROR: invalid assignment target: *ast.IntegerLiteral"},
		// A step of a function block's SFC can be read from outside it.
		{"FUNCTION_BLOCK Fb INITIAL_STEP S1: END_STEP END_FUNCTION_BLOCK VAR f : Fb; END_VAR S1.X;", "true"},
		// A function block extending an unknown or non-function-block parent
		// cannot be instantiated.
		{"FUNCTION_BLOCK B EXTENDS Nope END_FUNCTION_BLOCK VAR b1 : B; END_VAR", "ERROR: parent function block 'Nope' not found"},
		{"FUNCTION F : INT F := 1; END_FUNCTION FUNCTION_BLOCK B EXTENDS F END_FUNCTION_BLOCK VAR b1 : B; END_VAR", "ERROR: parent function block 'F' not found"},
	})
}

// An anonymous function, `fn(...) { ... }`, takes its parameters as inputs
// and gives the value of its last statement.
func TestFunctionLiterals(t *testing.T) {
	checkEval(t, []evalCase{
		{"fn(x : INT) { x * 2; }(3);", "6"},
		{"fn(a : INT, b : INT) { a - b; }(7, 2);", "5"},
		{"fn(x : INT) { RETURN x + 1; }(3);", "4"},
		{"fn() { }();", "null"},
		{"VAR d : INT; END_VAR d := fn(x : INT) { x * 3; }(2); d;", "6"},
	})
}

// VAR_CONFIG sets variables and structure members of program instances.
func TestVarConfigForms(t *testing.T) {
	const prog = "TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE PROGRAM Prog VAR_OUTPUT COUNT : INT; END_VAR VAR x : INT; p : Pt; END_VAR COUNT := COUNT + 1; END_PROGRAM "
	run := func(config string) (*object.Environment, object.Object) {
		env := object.NewEnvironment()
		return env, testEvalWithEnv(t, prog+config, env)
	}
	// The program-scoped form, and an entry without a value, which changes nothing.
	env, result := run("CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE VAR_CONFIG P1 x := 3; END_VAR END_CONFIGURATION")
	if isError(result) {
		t.Fatal(result.Inspect())
	}
	if x, _ := programInstanceEnv(t, env, "Res", "P1").Get("x"); x.Inspect() != "3" {
		t.Errorf("program-scoped VAR_CONFIG: x = %s, want 3", x.Inspect())
	}
	if _, result = run("CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE VAR_CONFIG Res.P1.x : INT; END_VAR END_CONFIGURATION"); isError(result) {
		t.Errorf("an entry without a value: %s", result.Inspect())
	}
	// A structure's members.
	env, result = run("CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE VAR_CONFIG Res.P1.p : Pt := (px := 4); END_VAR END_CONFIGURATION")
	if isError(result) {
		t.Fatal(result.Inspect())
	}
	p, _ := programInstanceEnv(t, env, "Res", "P1").Get("p")
	if key, ok := hashMemberKey(p.(*object.Hash), "px"); !ok || p.(*object.Hash).Pairs[key].Value.Inspect() != "4" {
		t.Errorf("p = %s, want px = 4", p.Inspect())
	}
	checkEval(t, []evalCase{
		{prog + "CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE VAR_CONFIG Res.P1.p : Pt := (py := 4); END_VAR END_CONFIGURATION", "ERROR: structure 'p' has no member 'py'"},
		{prog + "CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE VAR_CONFIG Res.P1.x : INT := (px := 4); END_VAR END_CONFIGURATION", "ERROR: requires a function block instance or structure, but 'x' is"},
		{prog + "VAR a : ARRAY[0..1] OF INT; END_VAR CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := T#1s, PRIORITY := 1); PROGRAM P1 WITH T1 : Prog (COUNT => a[1]); END_RESOURCE END_CONFIGURATION", "ERROR: target of an output mapping '=>' must be a variable identifier"},
		{"VAR x : INT; END_VAR x := missing^;", "ERROR: identifier not found: missing"},
	})
}

// IL conditions: numbers of every type are TRUE unless zero; NULL is FALSE.
func TestIlTruthinessOfEveryType(t *testing.T) {
	jump := func(value string) string { return ilProgram("LD " + value + "\nJMPC yes\nLD 0\nRET\nyes: LD 1") }
	cases := []evalCase{}
	for _, v := range []string{"SINT#1", "INT#1", "DINT#1", "USINT#1", "UDINT#1", "ULINT#1", "LREAL#1.5", "T#1s"} {
		cases = append(cases, evalCase{jump(v), "1"})
	}
	for _, v := range []string{"SINT#0", "INT#0", "DINT#0", "USINT#0", "UDINT#0", "ULINT#0", "LREAL#0.0"} {
		cases = append(cases, evalCase{jump(v), "0"})
	}
	checkIlEval(t, cases)
	if isTruthy(nil) || isTruthy(NULL) {
		t.Error("nil and NULL must be FALSE")
	}
}

// An edge trigger given a clock that is not a BOOL gives FALSE.
func TestEdgeTriggersWithoutABoolClock(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR e : R_TRIG; END_VAR e(CLK := 5); e.Q;", "false"},
		{"VAR e : F_TRIG; END_VAR e(CLK := 5); e.Q;", "false"},
	})
}

// Literal nodes and identifiers that the parser does not produce from source
// today, but other tools building an AST may.
func TestLiteralAndIdentifierNodes(t *testing.T) {
	env := object.NewEnvironment()
	for _, tt := range []struct {
		node ast.Node
		want string
	}{
		{&ast.UnsignedIntegerLiteral{Value: 18446744073709551615}, "18446744073709551615"},
		{&ast.RealLiteral{Value: 1.5, Precision: 32}, "1.500000"},
		{&ast.BitStringLiteral{Value: 255, Width: 8}, "BYTE#16#FF"},
		{&ast.EnumeratedValueLiteral{TypeName: &ast.Identifier{Value: "Color"}, Value: &ast.Identifier{Value: "Red"}}, "Color#Red"},
		// An identifier holding a typed literal or naming a conversion.
		{&ast.Identifier{Value: "T#5s"}, "T#5s"},
		{&ast.Identifier{Value: "BYTE#16#0F"}, "BYTE#16#F"},
		{&ast.Identifier{Value: "XYZ#1"}, "ERROR: ERROR (0:0): identifier not found: XYZ#1"},
	} {
		got := Eval(tt.node, env)
		if got == nil || got.Inspect() != tt.want {
			t.Errorf("%T: got %v, want %s", tt.node, got, tt.want)
		}
	}
	if fn := Eval(&ast.Identifier{Value: "FOO_TO_BAR"}, env); fn == nil || isError(fn) {
		t.Errorf("a conversion name gives its builtin, got %v", fn)
	}
}

// A bit of an integer or bit string is read and written as a BOOL; the
// variable keeps its type. The VM's TestBitAccess checks the same programs.
func TestBitAccess(t *testing.T) {
	checkEval(t, []evalCase{
		{"VAR x : BYTE := 16#0A; END_VAR x.3;", "true"},
		{"VAR x : BYTE := 16#0A; END_VAR x.0;", "false"},
		{"VAR x : INT := -1; END_VAR x.15 := FALSE; x;", "32767"},
		{"VAR x : INT := 5; END_VAR x.15 := TRUE; x;", "-32763"},
		{"VAR a : ARRAY[0..1] OF INT; END_VAR a[1].2 := TRUE; a[1];", "4"},
		{"FUNCTION_BLOCK Fb VAR_OUTPUT f : INT; END_VAR f.1 := TRUE; END_FUNCTION_BLOCK VAR fb : Fb; END_VAR fb(); fb.f;", "2"},
		{"VAR x : BYTE := 16#0A; b : BOOL; END_VAR IF x.1 AND NOT x.0 THEN b := TRUE; END_IF b;", "true"},
		{"VAR x : INT := 0; END_VAR x.16;", "ERROR: bit 16 is outside the 16 bits of INT"},
		{"VAR x : INT := 0; END_VAR x.20 := TRUE;", "ERROR: bit 20 is outside the 16 bits of INT"},
		{"VAR x : REAL; END_VAR x.1;", "ERROR: bit access needs an integer or bit string"},
		{"VAR x : BYTE; END_VAR missing.1;", "ERROR: identifier not found: missing"},
		{"VAR x : BYTE; END_VAR missing.1 := TRUE;", "ERROR: identifier not found: missing"},
	})
}
