package evaluator

import (
	"beedance/object"
	"testing"
	"time"
)

func TestFunctionBlock_EN_ENO(t *testing.T) {
	// Mock time for timer tests to have deterministic results.
	originalNowFunc := nowFunc
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	// Helper to advance mock time
	advanceTime := func(d time.Duration) {
		mockTime = mockTime.Add(d)
	}

	input := `
		PROGRAM TestTON_EN
			VAR
				MyTimer : TON;
				EnableExecution : BOOL := TRUE;
				Start : BOOL;
				TimerDone : BOOL;
				EnableOut : BOOL;
				ElapsedTime : TIME;
			END_VAR

			MyTimer(
				EN := EnableExecution, 
				ENO => EnableOut, 
				IN := Start, 
				PT := T#5s, 
				Q => TimerDone, 
				ET => ElapsedTime
			);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	// First, evaluate the whole program to set up the environment and FB instance.
	testEvalWithEnv(t, input, env)

	// Helper to run one "scan" by re-evaluating the FB call.
	runScan := func() {
		testEvalWithEnv(t, `MyTimer(EN := EnableExecution, ENO => EnableOut, IN := Start, PT := T#5s, Q => TimerDone, ET => ElapsedTime);`, env)
	}

	// --- Cycle 1: Start the timer while it is enabled. ---
	env.Set("EnableExecution", TRUE)
	env.Set("Start", TRUE)
	runScan()
	advanceTime(2 * time.Second) // Let 2 seconds pass.
	runScan()

	// Check that the timer is running and ENO is TRUE.
	testBooleanObjectInEnv(t, env, "TimerDone", false)
	testTimeObjectInEnv(t, env, "ElapsedTime", 2*time.Second)
	testBooleanObjectInEnv(t, env, "EnableOut", true)

	// --- Cycle 2: Disable the function block. ---
	// The timer should "freeze". Its internal state and outputs should not change,
	// even though time advances.
	env.Set("EnableExecution", FALSE)
	advanceTime(10 * time.Second) // Advance time well past the PT.
	runScan()

	// Check that the outputs are held from the last enabled state.
	// The timer should NOT have completed. ElapsedTime should still be 2s.
	testBooleanObjectInEnv(t, env, "TimerDone", false)
	testTimeObjectInEnv(t, env, "ElapsedTime", 2*time.Second)
	// ENO must be FALSE because EN is FALSE.
	testBooleanObjectInEnv(t, env, "EnableOut", false)

	// --- Cycle 3: Re-enable the function block. ---
	// The timer should now resume from where it was frozen.
	env.Set("EnableExecution", TRUE)
	runScan()

	// After re-enabling, the timer runs for 3 more seconds to reach its preset time of 5s.
	advanceTime(3 * time.Second)
	runScan()

	// Check that the timer has now completed.
	testBooleanObjectInEnv(t, env, "TimerDone", true)
	testTimeObjectInEnv(t, env, "ElapsedTime", 5*time.Second)
	testBooleanObjectInEnv(t, env, "EnableOut", true)
}
