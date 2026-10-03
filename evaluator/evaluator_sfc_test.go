package evaluator

import (
	"github.com/apiarytech/beedance/object"
	"testing"
	"time"
)

func TestSFCExecution(t *testing.T) {
	input := `
		PROGRAM TestSFC
			VAR
				x : INT := 0;
				cond1 : BOOL := FALSE;
				cond2 : BOOL := FALSE;
			END_VAR

			ACTION Step1Action: x := 1; END_ACTION
			ACTION Step2Action: x := x + 10; END_ACTION
			ACTION Step3Action: x := x + 100; END_ACTION

			INITIAL_STEP S1: Step1Action(); END_STEP

			TRANSITION FROM S1 TO S2 := cond1; END_TRANSITION

			STEP S2: Step2Action(); END_STEP

			TRANSITION FROM S2 TO S3 := cond2; END_TRANSITION

			STEP S3: Step3Action(); END_STEP

		END_PROGRAM
	`

	// We need to manage the environment manually for this test to check variables across cycles.
	env := object.NewEnvironment()
	// This evaluates the PROGRAM declaration, which now returns the created SFC object.
	sfcObj := testEvalWithEnv(t, input, env)

	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("TestSFC is not an SFC object, got %T", sfcObj)
	}

	// Get the program's internal environment where its variables live.
	progObj, _ := env.Get("TestSFC")
	progEnv := progObj.(*object.Program).Env

	// Ensure transition conditions are false initially to control the test flow.
	progEnv.Set("cond1", FALSE)
	progEnv.Set("cond2", FALSE)

	// --- Cycle 1: Initial state ---
	// S1 is active. The action "x := 1" should execute.
	evalSFCCycle(sfc, progEnv)
	if !sfc.Steps["S1"].IsActive {
		t.Fatal("S1 should be active initially")
	}
	testIntegerObjectInEnv(t, progEnv, "x", 1)

	// --- Cycle 2: Transition to S2 ---
	// Set the condition and run the cycle. The transition should clear,
	// S1 should deactivate, and S2 should activate and execute its action.
	progEnv.Set("cond1", TRUE)
	evalSFCCycle(sfc, progEnv)
	if sfc.Steps["S1"].IsActive || !sfc.Steps["S2"].IsActive {
		t.Fatalf("Should have transitioned to S2. Active steps: %v", sfc.ActiveSteps)
	}
	testIntegerObjectInEnv(t, progEnv, "x", 11)

	// --- Cycle 3: Still in S2 ---
	// The transition condition `cond2` is FALSE. S2 remains active.
	// The action "x := x + 10" executes again.
	progEnv.Set("cond1", FALSE) // Reset condition
	evalSFCCycle(sfc, progEnv)
	testIntegerObjectInEnv(t, progEnv, "x", 21)
}

func TestSFCActionQualifiers(t *testing.T) {
	input := `
		PROGRAM TestSFCQualifiers
		VAR
			// Action variables
			ActionN, ActionS, ActionP : BOOL;
			// Conditions
			GoToS2, GoToS3, GoToS4, Reset : BOOL;
		END_VAR

		INITIAL_STEP S1:
			ActionN(N);
			ActionS(S);
		END_STEP

		TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION

		STEP S2:
			ActionP(P);
		END_STEP

		TRANSITION FROM S2 TO S3 := GoToS3; END_TRANSITION

		STEP S3:
			ActionS(R); // Reset the 'ActionS' variable
		END_STEP

		TRANSITION FROM S3 TO S1 := Reset; END_TRANSITION
		END_PROGRAM
	`

	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfcObj)
	}

	progObj, _ := env.Get("TestSFCQualifiers")
	progEnv := progObj.(*object.Program).Env

	// Initialize all action variables to FALSE before starting the test cycles.
	initializeActionVars(t, sfc, progEnv)

	// --- Cycle 1: Initial state ---
	// S1 is active. ActionN and ActionS should be TRUE. ActionP and ActionR_S are FALSE.
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionN", true)
	testBooleanObjectInEnv(t, progEnv, "ActionS", true)
	// testBooleanObjectInEnv(t, env, "ActionR_S", false) // This variable does not exist in the program.
	testBooleanObjectInEnv(t, progEnv, "ActionP", false)

	// --- Cycle 2: Transition from S1 to S2 ---
	// Set condition and cycle. S1 becomes inactive, S2 becomes active.
	progEnv.Set("GoToS2", TRUE)
	evalSFCCycle(sfc, progEnv)
	// ActionN (Non-stored) becomes FALSE as S1 is no longer active.
	// ActionS (Set) remains TRUE.
	// ActionP (Pulse) becomes TRUE for this one cycle.
	testBooleanObjectInEnv(t, progEnv, "ActionN", false) // N action deactivates with step
	testBooleanObjectInEnv(t, progEnv, "ActionS", true)
	testBooleanObjectInEnv(t, progEnv, "ActionP", true)

	// --- Cycle 3: S2 is active ---
	// Reset condition. Cycle again.
	progEnv.Set("GoToS2", FALSE)
	evalSFCCycle(sfc, progEnv)
	// ActionP (Pulse) should now be FALSE again.
	// ActionS remains TRUE.
	testBooleanObjectInEnv(t, progEnv, "ActionP", false) // P action is only active for one cycle
	testBooleanObjectInEnv(t, progEnv, "ActionS", true)

	// --- Cycle 4: Transition from S2 to S3 ---
	// Set condition and cycle. S2 becomes inactive, S3 becomes active.
	progEnv.Set("GoToS3", TRUE)
	evalSFCCycle(sfc, progEnv)
	// S3 is now active. It has an 'R' qualifier for 'ActionS'.
	// This should force 'ActionS' to become FALSE immediately in this cycle.
	testBooleanObjectInEnv(t, progEnv, "ActionS", false)
}

func TestSFCTimedQualifier_SD(t *testing.T) {
	// Setup mock time
	mockTime = time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	originalNowFunc := nowFunc
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	input := `
		PROGRAM TestSD
			VAR ActionSD : BOOL; GoToS2 : BOOL; END_VAR
			INITIAL_STEP S1: ActionSD(SD, T#2s); END_STEP
			TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION
			STEP S2: END_STEP
		END_PROGRAM`
	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, _ := sfcObj.(*object.SFC)
	progObj, _ := env.Get("TestSD")
	progEnv := progObj.(*object.Program).Env
	initializeActionVars(t, sfc, progEnv)

	// Cycle 1 (t=1s): S1 active, timer starts, output is FALSE
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSD", false)

	// Cycle 2 (t=2s): 1s elapsed, output is still FALSE
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSD", false)

	// Cycle 3 (t=3s): 2s elapsed, timer is met, output becomes TRUE
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSD", true)

	// Cycle 4 (t=4s): Transition to S2, S1 becomes inactive
	progEnv.Set("GoToS2", TRUE)
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	// Action is "Stored", so it should remain TRUE even though S1 is inactive
	testBooleanObjectInEnv(t, progEnv, "ActionSD", true)
}

func TestSFCTimedQualifier_DS(t *testing.T) {
	// Setup mock time
	mockTime = time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	originalNowFunc := nowFunc
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	input := `
		PROGRAM TestDS
			VAR ActionDS : BOOL; GoToS2 : BOOL; END_VAR
			INITIAL_STEP S1: ActionDS(DS, T#3s); END_STEP
			TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION
			STEP S2: END_STEP
		END_PROGRAM`
	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, _ := sfcObj.(*object.SFC)
	progObj, _ := env.Get("TestDS")
	progEnv := progObj.(*object.Program).Env
	initializeActionVars(t, sfc, progEnv)

	cycle := 1

	// Cycle 1 (t=1s): S1 active, timer starts, output is FALSE
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionDS", false)

	// Cycle 2 (t=2s): 1s elapsed, output is still FALSE
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionDS", false)

	// Cycle 3 (t=3s): 2s elapsed, output is still FALSE
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(1001 * time.Millisecond) // Total elapsed since timer start: 2.002s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionDS", false) // 2.002s is less than 3s, so it must be false.

	// Cycle 4 (t=4s): 3s elapsed, timer is met, output becomes TRUE
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(1001 * time.Millisecond) // Total elapsed since timer start: 3.003s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionDS", true)

	// Cycle 5 (t=5s): Transition to S2, S1 becomes inactive
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	progEnv.Set("GoToS2", TRUE)
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	// Action is "Stored", so it should remain TRUE even though S1 is inactive
	testBooleanObjectInEnv(t, progEnv, "ActionDS", true)
}

func TestSFCTimedQualifier_SL(t *testing.T) {
	// Setup mock time
	mockTime = time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	originalNowFunc := nowFunc
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	input := `
		PROGRAM TestSL
			VAR ActionSL : BOOL; GoToS2 : BOOL; END_VAR
			INITIAL_STEP S1: ActionSL(SL, T#4s); END_STEP
			TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION
			STEP S2: END_STEP
		END_PROGRAM`
	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, _ := sfcObj.(*object.SFC)
	progObj, _ := env.Get("TestSL")
	progEnv := progObj.(*object.Program).Env
	initializeActionVars(t, sfc, progEnv)

	// Cycle 1 (t=1s): S1 active, output becomes TRUE immediately, timer starts
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSL", true)

	// Cycle 2 (t=3s): 2s elapsed, output is still TRUE
	advanceMockTime(2002 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSL", true)

	// Cycle 3 (t=4.004s): 3.003s elapsed, output is still TRUE
	advanceMockTime(1001 * time.Millisecond) // Total elapsed since timer start: 3.003s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSL", true)

	// Cycle 4 (t=5.005s): 4.004s elapsed, time limit is met, output becomes FALSE
	advanceMockTime(1001 * time.Millisecond) // Total elapsed since timer start: 4.004s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSL", false)

	// Cycle 5 (t=6s): Transition to S2, S1 becomes inactive
	progEnv.Set("GoToS2", TRUE)
	advanceMockTime(1001 * time.Millisecond)
	evalSFCCycle(sfc, progEnv)
	// Action is "Stored", its timer logic continues. It should remain FALSE.
	testBooleanObjectInEnv(t, progEnv, "ActionSL", false)
}

// initializeActionVars sets all action-related boolean variables in the environment to FALSE.
// This ensures a clean state before starting SFC cycle tests.
func initializeActionVars(t *testing.T, sfc *object.SFC, env *object.Environment) {
	if sfc != nil {
		for _, action := range sfc.Actions {
			if action == nil || action.Name == nil {
				continue // Skip nil actions or actions with nil names to prevent panic
			}
			env.Set(action.Name.Value, FALSE)
		}
	} else {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfc)
	}
}

// Mockable time for testing
var mockTime time.Time

func advanceMockTime(d time.Duration) {
	mockTime = mockTime.Add(d)
}

func TestSFCDivergenceConvergence(t *testing.T) {
	input := `
		PROGRAM TestSFCBranching
			VAR
				// Action variables
				PathA_Active, PathB_Active, PathC_Active, Merged_Active : BOOL;
				// Conditions
				SelectA, SelectB, Fork, Join : BOOL;
				// No Operations
				NOP : BOOL := FALSE;
			END_VAR

			INITIAL_STEP S1: NOP; END_STEP

			// Selection Divergence
			TRANSITION FROM S1 TO S2 := SelectA; END_TRANSITION
			TRANSITION FROM S1 TO S3 := SelectB; END_TRANSITION

			STEP S2: PathA_Active(N); END_STEP
			STEP S3: PathB_Active(N); END_STEP

			// Selection Convergence
			TRANSITION FROM S2 TO S4 := TRUE; END_TRANSITION
			TRANSITION FROM S3 TO S4 := TRUE; END_TRANSITION

			STEP S4: Merged_Active(N); END_STEP

			// Simultaneous Divergence (Fork)
			TRANSITION FROM S4 TO (S5, S6) := Fork; END_TRANSITION

			STEP S5: PathA_Active(S); END_STEP // Use Set to see state over cycles
			STEP S6: PathB_Active(S); END_STEP

			// Some intermediate steps
			TRANSITION FROM S5 TO S7 := TRUE; END_TRANSITION
			STEP S7: NOP; END_STEP
			TRANSITION FROM S6 TO S8 := TRUE; END_TRANSITION
			STEP S8: NOP; END_STEP

			// Simultaneous Convergence (Join)
			TRANSITION FROM (S7, S8) TO S9 := Join; END_TRANSITION

			STEP S9: PathC_Active(N); END_STEP

		END_PROGRAM
	`

	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfcObj)
	}

	progObj, _ := env.Get("TestSFCBranching")
	progEnv := progObj.(*object.Program).Env

	// --- Cycle 1: Initial state ---
	evalSFCCycle(sfc, progEnv)
	if !sfc.Steps["S1"].IsActive {
		t.Fatal("S1 should be active initially")
	}

	// --- Cycle 2: Test Selection Divergence ---
	progEnv.Set("SelectA", TRUE) // Choose path A
	evalSFCCycle(sfc, progEnv)
	if sfc.Steps["S1"].IsActive || !sfc.Steps["S2"].IsActive || sfc.Steps["S3"].IsActive {
		t.Fatal("Selection divergence failed: S1 should be inactive, S2 active, S3 inactive")
	}

	// --- Cycle 3: Test Selection Convergence ---
	evalSFCCycle(sfc, progEnv)
	if sfc.Steps["S2"].IsActive || !sfc.Steps["S4"].IsActive {
		t.Fatal("Selection convergence failed: S2 should be inactive, S4 active")
	}

	// --- Cycle 4: Test Simultaneous Divergence (Fork) ---
	progEnv.Set("Fork", TRUE)
	evalSFCCycle(sfc, progEnv)
	if sfc.Steps["S4"].IsActive || !sfc.Steps["S5"].IsActive || !sfc.Steps["S6"].IsActive {
		t.Fatal("Simultaneous divergence failed: S4 should be inactive, S5 and S6 should be active")
	}

	// --- Cycle 5 & 6: Let parallel paths advance ---
	evalSFCCycle(sfc, progEnv) // S5->S7, S6->S8
	if !sfc.Steps["S7"].IsActive || !sfc.Steps["S8"].IsActive {
		t.Fatal("Parallel paths did not advance correctly")
	}

	// --- Cycle 7: Test Simultaneous Convergence (Join) ---
	progEnv.Set("Join", TRUE)
	evalSFCCycle(sfc, progEnv)
	if sfc.Steps["S7"].IsActive || sfc.Steps["S8"].IsActive || !sfc.Steps["S9"].IsActive {
		t.Fatal("Simultaneous convergence failed: S7/S8 should be inactive, S9 active")
	}
}

func TestSFCActionQualifiersTimed(t *testing.T) {
	// Initialize mock time for this test
	mockTime = time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	// Override the global nowFunc in evaluator package for testing
	originalNowFunc := nowFunc
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }() // Restore original nowFunc after test

	input := `
		PROGRAM TestSFCTimedQualifiers
			VAR
				// Action variables
				ActionD_Q, ActionL_Q, ActionSD_Q, ActionDS_Q, ActionSL_Q : BOOL;
				// Conditions
				GoToS2, GoToS3, GoToS4, GoToS5 : BOOL;
			END_VAR

			INITIAL_STEP S1:
				ActionD_Q(D, T#5s);
				ActionL_Q(L, T#3s);
			END_STEP

			TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION

			STEP S2:
				ActionSD_Q(SD, T#2s);
				ActionDS_Q(DS, T#4s);
			END_STEP

			TRANSITION FROM S2 TO S3 := GoToS3; END_TRANSITION

			STEP S3:
				ActionSL_Q(SL, T#6s);
			END_STEP

			TRANSITION FROM S3 TO S4 := GoToS4; END_TRANSITION

			STEP S4:
				(* Final step *)
			END_STEP
		END_PROGRAM
	`

	env := object.NewEnvironment()
	// This call parses the program and returns the SFC object for convenience.
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfcObj)
	}

	// Get the program's persistent environment where its variables live.
	progObj, ok := env.Get("TestSFCTimedQualifiers")
	if !ok {
		t.Fatalf("Program 'TestSFCTimedQualifiers' not found in environment")
	}
	progEnv := progObj.(*object.Program).Env

	// Initialize all action variables to FALSE before starting the test cycles.
	initializeActionVars(t, sfc, progEnv)

	cycle := 1

	// --- Cycle 1: Initial state (S1 active) ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionD_Q", false)
	testBooleanObjectInEnv(t, progEnv, "ActionL_Q", true)

	// --- Cycle 2: Advance time by 2s ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(2 * time.Second)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionD_Q", false)
	testBooleanObjectInEnv(t, progEnv, "ActionL_Q", true)

	// --- Cycle 3: Advance time by another 2s (total 4s) ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(2 * time.Second)
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionD_Q", false)
	testBooleanObjectInEnv(t, progEnv, "ActionL_Q", false)

	// --- Cycle 4: Transition S1 -> S2 (GoToS2 = TRUE) ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	progEnv.Set("GoToS2", TRUE)
	advanceMockTime(1 * time.Second) // Total elapsed: 5s
	evalSFCCycle(sfc, progEnv)
	// S1 is inactive, S2 is active.
	// ActionD_Q and ActionL_Q (non-stored) are reset to FALSE because S1 deactivated.
	testBooleanObjectInEnv(t, progEnv, "ActionD_Q", false)
	testBooleanObjectInEnv(t, progEnv, "ActionL_Q", false)
	testBooleanObjectInEnv(t, progEnv, "ActionSD_Q", false)
	testBooleanObjectInEnv(t, progEnv, "ActionDS_Q", false)

	// --- Cycle 5: In S2, advance time by 2s ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(2 * time.Second) // Total elapsed since S2 activation: 2s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSD_Q", true)
	testBooleanObjectInEnv(t, progEnv, "ActionDS_Q", false)

	// --- Cycle 6: In S2, advance time by another 2s ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(2 * time.Second) // Total elapsed since S2 activation: 4s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSD_Q", true)
	testBooleanObjectInEnv(t, progEnv, "ActionDS_Q", true)
	testBooleanObjectInEnv(t, progEnv, "ActionSL_Q", false)

	// --- Cycle 7: Transition S2 -> S3 (GoToS3 = TRUE) ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	progEnv.Set("GoToS3", TRUE)
	advanceMockTime(1 * time.Second)
	evalSFCCycle(sfc, progEnv)
	// S2 inactive, S3 active. ActionSD_Q and ActionDS_Q remain TRUE (stored).
	testBooleanObjectInEnv(t, progEnv, "ActionSD_Q", true)
	testBooleanObjectInEnv(t, progEnv, "ActionDS_Q", true)
	testBooleanObjectInEnv(t, progEnv, "ActionSL_Q", true)

	// --- Cycle 8: Advance time by 6s (total 16s) ---
	t.Logf("--- Cycle %d ---", cycle)
	cycle++
	advanceMockTime(6 * time.Second) // Total elapsed since S3 activation: 6s
	evalSFCCycle(sfc, progEnv)
	testBooleanObjectInEnv(t, progEnv, "ActionSL_Q", false)
}

func TestSFCActionWithSTBody(t *testing.T) {
	input := `
		PROGRAM TestSFC_ST_Action
			VAR
				Counter : INT := 0;
				GoToStep2 : BOOL := FALSE;
				GoToStep1 : BOOL := FALSE;
			END_VAR

			ACTION IncrementCounter:
				Counter := Counter + 1;
			END_ACTION

			INITIAL_STEP S1:
				(* Do nothing *)
			END_STEP

			TRANSITION FROM S1 TO S2 := GoToStep2;
			END_TRANSITION

			STEP S2:
				IncrementCounter(N); (* Non-stored action *)
			END_STEP

			TRANSITION FROM S2 TO S1 := GoToStep1;
			END_TRANSITION

		END_PROGRAM
	`

	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)
	if _, ok := sfcObj.(*object.SFC); !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfcObj)
	}
	progObj, _ := env.Get("TestSFC_ST_Action")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestSFC_ST_Action();`, env)
	}

	// --- Cycle 1: Initial state (S1 active) ---
	runScan()
	testIntegerObjectInEnv(t, progEnv, "Counter", 0) // Action body should not have run

	// --- Cycle 2: Transition to S2 ---
	progEnv.Set("GoToStep2", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "Counter", 1) // Action body runs for the first time

	// --- Cycle 3: Still in S2 ---
	progEnv.Set("GoToStep2", FALSE) // Prevent immediate re-transition
	runScan()
	testIntegerObjectInEnv(t, progEnv, "Counter", 2) // Action body runs again
}

func TestFunctionBlockWithSFCBody_EdgeCases(t *testing.T) {
	// This test verifies edge cases, like a transition condition remaining true
	// for multiple cycles.
	// Mock time for timer tests
	originalNowFunc := nowFunc
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	// Helper to advance mock time
	advanceTime := func(d time.Duration) {
		mockTime = mockTime.Add(d)
	}

	input := `
		FUNCTION_BLOCK MySFC_FB
			VAR_INPUT
				EnableTransitionToS2 : BOOL;
				EnableTransitionToS1 : BOOL;
			END_VAR
			VAR_OUTPUT
				ActiveStepOut : INT;
			END_VAR

			ACTION S1_Action: 
				ActiveStepOut := 1; 
			END_ACTION
			ACTION S2_Action: 
				ActiveStepOut := 2; 
			END_ACTION

			INITIAL_STEP S1:
				S1_Action(N); 
			END_STEP

			TRANSITION 
				FROM S1 TO S2 := EnableTransitionToS2; 
			END_TRANSITION

			STEP S2: 
				S2_Action(N); 
			END_STEP

			TRANSITION 
				FROM S2 TO S1 := EnableTransitionToS1; 
			END_TRANSITION
		END_FUNCTION_BLOCK

		PROGRAM TestSFCinFB_Edges
			VAR
				myFb : MySFC_FB;
				doTransitionToS2 : BOOL;
				doTransitionToS1 : BOOL;
				currentActiveStep : INT;
			END_VAR

			myFb(
				EnableTransitionToS2 := doTransitionToS2,
				EnableTransitionToS1 := doTransitionToS1,
				ActiveStepOut => currentActiveStep
			);
		END_PROGRAM
	`

	env := object.NewEnvironment()
	// First, evaluate the whole program to set up the environment and FB instance.
	testEvalWithEnv(t, input, env)

	progObj, _ := env.Get("TestSFCinFB_Edges")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestSFCinFB_Edges();`, env)
	}

	// --- Cycle 1: Initial State ---
	// The initial Eval() call already executed the first scan.
	runScan()
	testIntegerObjectInEnv(t, progEnv, "currentActiveStep", 1)

	// --- Cycle 2: Set transition condition to TRUE ---
	progEnv.Set("doTransitionToS2", TRUE)
	runScan()
	advanceTime(1 * time.Second)
	// The transition should have occurred. We are now in S2.
	testIntegerObjectInEnv(t, progEnv, "currentActiveStep", 2)

	// --- Cycle 3: Transition condition remains TRUE ---
	// The SFC should remain in S2. A cleared transition should not re-fire
	// just because the condition is still true.
	runScan()
	advanceTime(1 * time.Second)
	testIntegerObjectInEnv(t, progEnv, "currentActiveStep", 2)

	// --- Cycle 4: Reset condition and transition back to S1 ---
	progEnv.Set("doTransitionToS2", FALSE)
	progEnv.Set("doTransitionToS1", TRUE)
	runScan()
	advanceTime(1 * time.Second)
	testIntegerObjectInEnv(t, progEnv, "currentActiveStep", 1)

	// --- Cycle 5: Both transition conditions are TRUE ---
	// Since the active step is S1, only the S1->S2 transition should be evaluated.
	progEnv.Set("doTransitionToS2", TRUE)
	progEnv.Set("doTransitionToS1", TRUE)
	runScan()
	advanceTime(1 * time.Second)
	testIntegerObjectInEnv(t, progEnv, "currentActiveStep", 2)
}

func TestPumpControlSFC(t *testing.T) {
	input := `
		PROGRAM PumpControlProgram
			VAR
				StartButton: BOOL;
				TankHighSensor: BOOL;
				PumpMotor: BOOL;
				TimerDone: BOOL;
			END_VAR

			ACTION IdleAction:
				PumpMotor := FALSE;
			END_ACTION

			ACTION RunningAction:
				PumpMotor := TRUE;
			END_ACTION

			INITIAL_STEP Idle:
				IdleAction();
			END_STEP

			TRANSITION FROM Idle TO Running := StartButton AND NOT TankHighSensor;
			END_TRANSITION

			STEP Running:
				RunningAction();
			END_STEP

			TRANSITION FROM Running TO Idle := TankHighSensor OR TimerDone;
			END_TRANSITION
		END_PROGRAM
	`

	env := object.NewEnvironment()
	// Evaluate the program to declare the POU and its variables.
	// testEvalWithEnv will parse and evaluate the program, returning the SFC object.
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfcObj)
	}

	// Helper to run a scan cycle
	runScan := func() {
		evalSFCCycle(sfc, env)
	}

	// --- Cycle 1: Initial State ---
	// System is idle, pump should be off.
	env.Set("StartButton", FALSE)
	env.Set("TankHighSensor", FALSE)
	env.Set("TimerDone", FALSE)
	runScan()

	if !sfc.Steps["Idle"].IsActive {
		t.Fatal("SFC should be in 'Idle' step initially.")
	}
	testBooleanObjectInEnv(t, env, "PumpMotor", false)

	// --- Cycle 2: Transition to Running ---
	// Press the start button. Tank is not high. Pump should turn on.
	env.Set("StartButton", TRUE)
	runScan()

	if !sfc.Steps["Running"].IsActive {
		t.Fatal("SFC should have transitioned to 'Running' step.")
	}
	testBooleanObjectInEnv(t, env, "PumpMotor", true)

	// --- Cycle 3: Transition back to Idle via TankHighSensor ---
	// Release start button, sensor indicates tank is full. Pump should turn off.
	env.Set("StartButton", FALSE)
	env.Set("TankHighSensor", TRUE)
	runScan()

	if !sfc.Steps["Idle"].IsActive {
		t.Fatal("SFC should have transitioned back to 'Idle' step.")
	}
	testBooleanObjectInEnv(t, env, "PumpMotor", false)

	// --- Cycle 4 & 5: Transition to Running, then stop via TimerDone ---
	env.Set("TankHighSensor", FALSE)
	env.Set("StartButton", TRUE)
	runScan() // Go to Running
	env.Set("StartButton", FALSE)
	env.Set("TimerDone", TRUE)
	runScan() // Go to Idle
	testBooleanObjectInEnv(t, env, "PumpMotor", false)
}

func TestEvalSFCProgram(t *testing.T) {
	input := `
		PROGRAM TestSFC
			VAR
				x : INT;
			END_VAR
			ACTION A1: x := 1; END_ACTION
			INITIAL_STEP S1: A1(); END_STEP
			TRANSITION FROM S1 TO S1 := TRUE; END_TRANSITION
		END_PROGRAM
	`
	evaluated := testEval(t, input)
	sfc, ok := evaluated.(*object.SFC)
	if !ok {
		t.Fatalf("Expected *object.SFC, got %T", evaluated)
	}

	if sfc.InitialStepName != "S1" {
		t.Errorf("Expected initial step 'S1', got %q", sfc.InitialStepName)
	}
	if _, ok := sfc.Steps["S1"]; !ok {
		t.Errorf("Expected step 'S1' to be defined")
	}
	if _, ok := sfc.Actions["A1"]; !ok {
		t.Errorf("Expected action 'A1' to be defined")
	}
	if len(sfc.Transitions) != 1 {
		t.Errorf("Expected 1 transition, got %d", len(sfc.Transitions))
	}
}
