package evaluator

import (
	"beedance/object"
	"testing"
)

func TestR_TRIG(t *testing.T) {
	input := `
		PROGRAM TestR_TRIG
			VAR
				MyTrigger : R_TRIG;
				InputSignal : BOOL;
				RisingEdgeDetected : BOOL;
			END_VAR

			MyTrigger(CLK := InputSignal, Q => RisingEdgeDetected);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	// Evaluate the program to set up the environment and FB instance
	testEvalWithEnv(t, input, env)

	// Helper to run one "scan" by re-evaluating the FB call
	runScan := func() {
		testEvalWithEnv(t, `MyTrigger(CLK := InputSignal, Q => RisingEdgeDetected);`, env)
	}

	// --- Cycle 1: Initial state, Input is FALSE ---
	env.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", false)

	// --- Cycle 2: Input is still FALSE ---
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", false)

	// --- Cycle 3: Rising edge on Input (FALSE -> TRUE) ---
	env.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", true) // Q should be TRUE for one scan

	// --- Cycle 4: Input is still TRUE ---
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", false) // Q should be FALSE now

	// --- Cycle 5: Input is still TRUE ---
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", false)

	// --- Cycle 6: Falling edge on Input (TRUE -> FALSE) ---
	env.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", false)

	// --- Cycle 7: Input is FALSE again ---
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", false)

	// --- Cycle 8: Second rising edge ---
	env.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, env, "RisingEdgeDetected", true)
}

func TestF_TRIG(t *testing.T) {
	input := `
		PROGRAM TestF_TRIG
			VAR
				MyTrigger : F_TRIG;
				InputSignal : BOOL;
				FallingEdgeDetected : BOOL;
			END_VAR

			MyTrigger(CLK := InputSignal, Q => FallingEdgeDetected);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	// Evaluate the program to set up the environment and FB instance
	testEvalWithEnv(t, input, env)

	// Helper to run one "scan" by re-evaluating the FB call
	runScan := func() {
		testEvalWithEnv(t, `MyTrigger(CLK := InputSignal, Q => FallingEdgeDetected);`, env)
	}

	// --- Cycle 1: Initial state, Input is TRUE ---
	env.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", false)

	// --- Cycle 2: Input is still TRUE ---
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", false)

	// --- Cycle 3: Falling edge on Input (TRUE -> FALSE) ---
	env.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", true) // Q should be TRUE for one scan

	// --- Cycle 4: Input is still FALSE ---
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", false) // Q should be FALSE now

	// --- Cycle 5: Input is still FALSE ---
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", false)

	// --- Cycle 6: Rising edge on Input (FALSE -> TRUE) ---
	env.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", false)

	// --- Cycle 7: Input is TRUE again ---
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", false)

	// --- Cycle 8: Second falling edge ---
	env.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, env, "FallingEdgeDetected", true)
}

func TestCTU(t *testing.T) {
	input := `
		PROGRAM TestCTU
			VAR
				MyCounter : CTU;
				CountUp : BOOL;
				Reset : BOOL;
				IsDone : BOOL;
				CurrentValue : INT;
			END_VAR

			MyCounter(CU := CountUp, R := Reset, PV := 3, Q => IsDone, CV => CurrentValue);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(t, input, env)

	runScan := func() {
		testEvalWithEnv(t, `MyCounter(CU := CountUp, R := Reset, PV := 3, Q => IsDone, CV => CurrentValue);`, env)
	}

	// --- Cycle 1: Initial state ---
	env.Set("CountUp", FALSE)
	env.Set("Reset", FALSE)
	runScan()
	testBooleanObjectInEnv(t, env, "IsDone", false)
	testIntegerObjectInEnv(t, env, "CurrentValue", 0)

	// --- Cycle 2: First rising edge on CU ---
	env.Set("CountUp", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 1)
	testBooleanObjectInEnv(t, env, "IsDone", false)

	// --- Cycle 3: CU is still high (no change) ---
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 1)

	// --- Cycle 4: Falling edge on CU ---
	env.Set("CountUp", FALSE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 1)

	// --- Cycle 5: Second rising edge ---
	env.Set("CountUp", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 2)
	env.Set("CountUp", FALSE)
	runScan()

	// --- Cycle 6: Third rising edge (reaches PV) ---
	env.Set("CountUp", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 3)
	testBooleanObjectInEnv(t, env, "IsDone", true) // Q is now true

	// --- Cycle 7: Reset ---
	env.Set("CountUp", FALSE)
	env.Set("Reset", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 0)
	testBooleanObjectInEnv(t, env, "IsDone", false)
}

func TestCTD(t *testing.T) {
	input := `
		PROGRAM TestCTD
			VAR
				MyCounter : CTD;
				CountDown : BOOL;
				Load : BOOL;
				IsDone : BOOL;
				CurrentValue : INT;
			END_VAR

			MyCounter(CD := CountDown, LD := Load, PV := 3, Q => IsDone, CV => CurrentValue);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(t, input, env)

	runScan := func() {
		testEvalWithEnv(t, `MyCounter(CD := CountDown, LD := Load, PV := 3, Q => IsDone, CV => CurrentValue);`, env)
	}

	// --- Cycle 1: Load the counter ---
	env.Set("Load", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 3)
	testBooleanObjectInEnv(t, env, "IsDone", false)

	// --- Cycle 2: First rising edge on CD ---
	env.Set("Load", FALSE)
	env.Set("CountDown", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 2)
}

func TestCTUD(t *testing.T) {
	input := `
		PROGRAM TestCTUD
			VAR
				MyCounter : CTUD;
				CountUp : BOOL;
				CountDown : BOOL;
				Reset : BOOL;
				Load : BOOL;
				IsFull : BOOL;
				IsEmpty : BOOL;
				CurrentValue : INT;
			END_VAR

			MyCounter(
				CU := CountUp,
				CD := CountDown,
				R := Reset,
				LD := Load,
				PV := 5,
				QU => IsFull,
				QD => IsEmpty,
				CV => CurrentValue
			);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(t, input, env)

	runScan := func() {
		testEvalWithEnv(t, `MyCounter(CU := CountUp, CD := CountDown, R := Reset, LD := Load, PV := 5, QU => IsFull, QD => IsEmpty, CV => CurrentValue);`, env)
	}

	// Helper for rising edge pulse
	pulse := func(varName string) {
		env.Set(varName, TRUE)
		runScan()
		env.Set(varName, FALSE)
		runScan()
	}

	env.Set("CountUp", FALSE)
	env.Set("CountDown", FALSE)
	env.Set("Reset", FALSE)
	env.Set("Load", FALSE)

	// --- Cycle 1: Initial state ---
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 0)
	testBooleanObjectInEnv(t, env, "IsFull", false)
	testBooleanObjectInEnv(t, env, "IsEmpty", true)

	// --- Cycle 2: Count up to 2 ---
	pulse("CountUp") // CV=1
	pulse("CountUp") // CV=2
	testIntegerObjectInEnv(t, env, "CurrentValue", 2)
	testBooleanObjectInEnv(t, env, "IsEmpty", false)
	//testBooleanObject(t, mustGet(env, "IsFull"), "IsFull", false)

	// --- Cycle 3: Count down to 1 ---
	pulse("CountDown") // CV=1
	testIntegerObjectInEnv(t, env, "CurrentValue", 1)

	// --- Cycle 4: Count up to PV (5) ---
	pulse("CountUp") // CV=2
	pulse("CountUp") // CV=3
	pulse("CountUp") // CV=4
	pulse("CountUp") // CV=5
	testIntegerObjectInEnv(t, env, "CurrentValue", 5)
	testBooleanObjectInEnv(t, env, "IsFull", true)
	testBooleanObjectInEnv(t, env, "IsEmpty", false)

	// --- Cycle 5: Try to count past PV ---
	pulse("CountUp") // CV should stay 5
	testIntegerObjectInEnv(t, env, "CurrentValue", 5)

	// --- Cycle 6: Load PV ---
	env.Set("Load", TRUE)
	runScan()
	testIntegerObjectInEnv(t, env, "CurrentValue", 5) // Already at PV, but confirms load
	env.Set("Load", FALSE)
	runScan()

	// --- Cycle 7: Count down to 0 ---
	pulse("CountDown") // 4
	pulse("CountDown") // 3
	pulse("CountDown") // 2
	pulse("CountDown") // 1
	pulse("CountDown") // 0
	testIntegerObjectInEnv(t, env, "CurrentValue", 0)
	testBooleanObjectInEnv(t, env, "IsEmpty", true)
	testBooleanObjectInEnv(t, env, "IsFull", false)
}
