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
	// Evaluate the program to define it.
	testEvalWithEnv(t, input, env)

	// Get the program object and its internal environment where variables live.
	progObj, _ := env.Get("TestSR")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		// A "scan" now means calling the program POU.
		testEvalWithEnv(t, `TestSR();`, env)
	}

	// --- Cycle 1: Initial state (S1=F, R=F) -> Q1=F ---
	progEnv.Set("SetInput", FALSE)
	progEnv.Set("ResetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)

	// --- Cycle 2: Set (S1=T, R=F) -> Q1=T ---
	progEnv.Set("SetInput", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)

	// --- Cycle 3: Hold state (S1=F, R=F) -> Q1=T ---
	progEnv.Set("SetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)

	// --- Cycle 4: Reset (S1=F, R=T) -> Q1=F ---
	progEnv.Set("ResetInput", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)

	// --- Cycle 5: Hold state (S1=F, R=F) -> Q1=F ---
	progEnv.Set("ResetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)

	// --- Cycle 6: Priority check (S1=T, R=T) -> Q1=F (Reset dominates) ---
	progEnv.Set("SetInput", TRUE)
	progEnv.Set("ResetInput", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)

	// --- Cycle 7: Release Reset first (S1=T, R=F) -> Q1=T ---
	progEnv.Set("ResetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)
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
	testEvalWithEnv(t, input, env)

	progObj, _ := env.Get("TestRS")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestRS();`, env)
	}

	// --- Cycle 1: Initial state (S=F, R1=F) -> Q1=F ---
	progEnv.Set("SetInput", FALSE)
	progEnv.Set("ResetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)

	// --- Cycle 2: Set (S=T, R1=F) -> Q1=T ---
	progEnv.Set("SetInput", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)

	// --- Cycle 3: Hold state (S=F, R1=F) -> Q1=T ---
	progEnv.Set("SetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)

	// --- Cycle 4: Reset (S=F, R1=T) -> Q1=F ---
	progEnv.Set("ResetInput", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)

	// --- Cycle 5: Priority check (S=T, R1=T) -> Q1=T (Set dominates) ---
	progEnv.Set("SetInput", TRUE)
	progEnv.Set("ResetInput", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)

	// --- Cycle 6: Release Set first (S=F, R1=T) -> Q1=F ---
	progEnv.Set("SetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false)
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
	testEvalWithEnv(t, input, env)

	progObj, _ := env.Get("TestSR_EN")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestSR_EN();`, env)
	}

	// --- Cycle 1: Enabled, Set the latch ---
	progEnv.Set("EnableExecution", TRUE)
	progEnv.Set("SetInput", TRUE)
	progEnv.Set("ResetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)
	testBooleanObjectInEnv(t, progEnv, "EnableOut", true)

	// --- Cycle 2: Disable execution ---
	// The inputs change, but the block should not execute, and outputs should hold their values.
	progEnv.Set("EnableExecution", FALSE)
	progEnv.Set("SetInput", FALSE)
	progEnv.Set("ResetInput", TRUE) // This would normally reset the latch
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)   // Output is held from previous state
	testBooleanObjectInEnv(t, progEnv, "EnableOut", false) // ENO should be FALSE

	// --- Cycle 3: Still disabled ---
	// Run again to ensure state is held.
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)
	testBooleanObjectInEnv(t, progEnv, "EnableOut", false)

	// --- Cycle 4: Re-enable execution ---
	// The block should now execute with the inputs from Cycle 2 (R=TRUE).
	progEnv.Set("EnableExecution", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false) // Latch is now reset
	testBooleanObjectInEnv(t, progEnv, "EnableOut", true) // ENO is TRUE again
}

func TestRS_FunctionBlock_EN_ENO(t *testing.T) {
	input := `
		PROGRAM TestRS_EN
			VAR
				MyLatch : RS;
				EnableExecution : BOOL := TRUE;
				SetInput : BOOL;
				ResetInput : BOOL;
				OutputQ1 : BOOL;
				EnableOut : BOOL;
			END_VAR

			MyLatch(EN := EnableExecution, ENO => EnableOut, S := SetInput, R1 := ResetInput, Q1 => OutputQ1);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(t, input, env)

	progObj, _ := env.Get("TestRS_EN")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestRS_EN();`, env)
	}

	// --- Cycle 1: Enabled, Set the latch ---
	progEnv.Set("EnableExecution", TRUE)
	progEnv.Set("SetInput", TRUE)
	progEnv.Set("ResetInput", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)
	testBooleanObjectInEnv(t, progEnv, "EnableOut", true)

	// --- Cycle 2: Disable execution ---
	// The inputs change, but the block should not execute, and outputs should hold their values.
	progEnv.Set("EnableExecution", FALSE)
	progEnv.Set("SetInput", FALSE)
	progEnv.Set("ResetInput", TRUE) // This would normally reset the latch
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", true)   // Output is held from previous state
	testBooleanObjectInEnv(t, progEnv, "EnableOut", false) // ENO should be FALSE

	// --- Cycle 3: Re-enable execution ---
	// The block should now execute with the inputs from Cycle 2 (R1=TRUE).
	// Since Set has priority in RS, if SetInput were TRUE, it would set. But it's FALSE.
	progEnv.Set("EnableExecution", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "OutputQ1", false) // Latch is now reset
	testBooleanObjectInEnv(t, progEnv, "EnableOut", true) // ENO is TRUE again
}
