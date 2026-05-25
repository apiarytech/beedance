package evaluator

import (
	"beedance/object"
	"testing"
)

func TestSR_FunctionBlock(t *testing.T) {
	input := `
		PROGRAM TestSR
			VAR
				MyLatch : SR;
				SetInput : BOOL;
				ResetInput : BOOL;
				OutputQ1 : BOOL;
			END_VAR

			MyLatch(S1 := SetInput, R := ResetInput, Q1 => OutputQ1);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(input, env)

	runScan := func() {
		testEvalWithEnv(`MyLatch(S1 := SetInput, R := ResetInput, Q1 => OutputQ1);`, env)
	}

	// --- Cycle 1: Initial state (S1=F, R=F) -> Q1=F ---
	env.Set("SetInput", FALSE)
	env.Set("ResetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)

	// --- Cycle 2: Set (S1=T, R=F) -> Q1=T ---
	env.Set("SetInput", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)

	// --- Cycle 3: Hold state (S1=F, R=F) -> Q1=T ---
	env.Set("SetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)

	// --- Cycle 4: Reset (S1=F, R=T) -> Q1=F ---
	env.Set("ResetInput", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)

	// --- Cycle 5: Hold state (S1=F, R=F) -> Q1=F ---
	env.Set("ResetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)

	// --- Cycle 6: Priority check (S1=T, R=T) -> Q1=F (Reset dominates) ---
	env.Set("SetInput", TRUE)
	env.Set("ResetInput", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)

	// --- Cycle 7: Release Reset first (S1=T, R=F) -> Q1=T ---
	env.Set("ResetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)
}

func TestRS_FunctionBlock(t *testing.T) {
	input := `
		PROGRAM TestRS
			VAR
				MyLatch : RS;
				SetInput : BOOL;
				ResetInput : BOOL;
				OutputQ1 : BOOL;
			END_VAR

			MyLatch(S := SetInput, R1 := ResetInput, Q1 => OutputQ1);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(input, env)

	runScan := func() {
		testEvalWithEnv(`MyLatch(S := SetInput, R1 := ResetInput, Q1 => OutputQ1);`, env)
	}

	// --- Cycle 1: Initial state (S=F, R1=F) -> Q1=F ---
	env.Set("SetInput", FALSE)
	env.Set("ResetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)

	// --- Cycle 2: Set (S=T, R1=F) -> Q1=T ---
	env.Set("SetInput", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)

	// --- Cycle 3: Hold state (S=F, R1=F) -> Q1=T ---
	env.Set("SetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)

	// --- Cycle 4: Reset (S=F, R1=T) -> Q1=F ---
	env.Set("ResetInput", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)

	// --- Cycle 5: Priority check (S=T, R1=T) -> Q1=T (Set dominates) ---
	env.Set("SetInput", TRUE)
	env.Set("ResetInput", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)

	// --- Cycle 6: Release Set first (S=F, R1=T) -> Q1=F ---
	env.Set("SetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false)
}

func TestSR_FunctionBlock_EN_ENO(t *testing.T) {
	input := `
		PROGRAM TestSR_EN
			VAR
				MyLatch : SR;
				EnableExecution : BOOL := TRUE;
				SetInput : BOOL;
				ResetInput : BOOL;
				OutputQ1 : BOOL;
				EnableOut : BOOL;
			END_VAR

			MyLatch(EN := EnableExecution, ENO => EnableOut, S1 := SetInput, R := ResetInput, Q1 => OutputQ1);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(input, env)

	runScan := func() {
		testEvalWithEnv(`MyLatch(EN := EnableExecution, ENO => EnableOut, S1 := SetInput, R := ResetInput, Q1 => OutputQ1);`, env)
	}

	// --- Cycle 1: Enabled, Set the latch ---
	env.Set("EnableExecution", TRUE)
	env.Set("SetInput", TRUE)
	env.Set("ResetInput", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)
	testBooleanObject(t, mustGet(env, "EnableOut"), true)

	// --- Cycle 2: Disable execution ---
	// The inputs change, but the block should not execute, and outputs should hold their values.
	env.Set("EnableExecution", FALSE)
	env.Set("SetInput", FALSE)
	env.Set("ResetInput", TRUE) // This would normally reset the latch
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)   // Output is held from previous state
	testBooleanObject(t, mustGet(env, "EnableOut"), false) // ENO should be FALSE

	// --- Cycle 3: Still disabled ---
	// Run again to ensure state is held.
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), true)
	testBooleanObject(t, mustGet(env, "EnableOut"), false)

	// --- Cycle 4: Re-enable execution ---
	// The block should now execute with the inputs from Cycle 2 (R=TRUE).
	env.Set("EnableExecution", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "OutputQ1"), false) // Latch is now reset
	testBooleanObject(t, mustGet(env, "EnableOut"), true) // ENO is TRUE again
}
