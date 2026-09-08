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
	testEvalWithEnv(t, input, env)

	progObj, _ := env.Get("TestR_TRIG")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestR_TRIG();`, env)
	}

	// --- Cycle 1: Initial state, Input is FALSE ---
	progEnv.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", false)

	// --- Cycle 2: Input is still FALSE ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", false)

	// --- Cycle 3: Rising edge on Input (FALSE -> TRUE) ---
	progEnv.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", true) // Q should be TRUE for one scan

	// --- Cycle 4: Input is still TRUE ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", false) // Q should be FALSE now

	// --- Cycle 5: Input is still TRUE ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", false)

	// --- Cycle 6: Falling edge on Input (TRUE -> FALSE) ---
	progEnv.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", false)

	// --- Cycle 7: Input is FALSE again ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", false)

	// --- Cycle 8: Second rising edge ---
	progEnv.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "RisingEdgeDetected", true)
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
	testEvalWithEnv(t, input, env)

	progObj, _ := env.Get("TestF_TRIG")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestF_TRIG();`, env)
	}

	// --- Cycle 1: Initial state, Input is TRUE ---
	progEnv.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", false)

	// --- Cycle 2: Input is still TRUE ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", false)

	// --- Cycle 3: Falling edge on Input (TRUE -> FALSE) ---
	progEnv.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", true) // Q should be TRUE for one scan

	// --- Cycle 4: Input is still FALSE ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", false) // Q should be FALSE now

	// --- Cycle 5: Input is still FALSE ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", false)

	// --- Cycle 6: Rising edge on Input (FALSE -> TRUE) ---
	progEnv.Set("InputSignal", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", false)

	// --- Cycle 7: Input is TRUE again ---
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", false)

	// --- Cycle 8: Second falling edge ---
	progEnv.Set("InputSignal", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "FallingEdgeDetected", true)
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
	progObj, _ := env.Get("TestCTU")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestCTU();`, env)
	}

	// --- Cycle 1: Initial state ---
	progEnv.Set("CountUp", FALSE)
	progEnv.Set("Reset", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "IsDone", false)
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 0)

	// --- Cycle 2: First rising edge on CU ---
	progEnv.Set("CountUp", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 1)
	testBooleanObjectInEnv(t, progEnv, "IsDone", false)

	// --- Cycle 3: CU is still high (no change) ---
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 1)

	// --- Cycle 4: Falling edge on CU ---
	progEnv.Set("CountUp", FALSE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 1)

	// --- Cycle 5: Second rising edge ---
	progEnv.Set("CountUp", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 2)
	progEnv.Set("CountUp", FALSE)
	runScan()

	// --- Cycle 6: Third rising edge (reaches PV) ---
	progEnv.Set("CountUp", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 3)
	testBooleanObjectInEnv(t, progEnv, "IsDone", true) // Q is now true

	// --- Cycle 7: Reset ---
	progEnv.Set("CountUp", FALSE)
	progEnv.Set("Reset", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 0)
	testBooleanObjectInEnv(t, progEnv, "IsDone", false)
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
	progObj, _ := env.Get("TestCTD")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestCTD();`, env)
	}

	// --- Cycle 1: Load the counter ---
	progEnv.Set("Load", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 3)
	testBooleanObjectInEnv(t, progEnv, "IsDone", false)

	// --- Cycle 2: First rising edge on CD ---
	progEnv.Set("Load", FALSE)
	progEnv.Set("CountDown", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 2)
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
	progObj, _ := env.Get("TestCTUD")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestCTUD();`, env)
	}

	// Helper for rising edge pulse
	pulse := func(varName string) {
		progEnv.Set(varName, TRUE)
		runScan()
		progEnv.Set(varName, FALSE)
		runScan()
	}

	progEnv.Set("CountUp", FALSE)
	progEnv.Set("CountDown", FALSE)
	progEnv.Set("Reset", FALSE)
	progEnv.Set("Load", FALSE)

	// --- Cycle 1: Initial state ---
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 0)
	testBooleanObjectInEnv(t, progEnv, "IsFull", false)
	testBooleanObjectInEnv(t, progEnv, "IsEmpty", true)

	// --- Cycle 2: Count up to 2 ---
	pulse("CountUp") // CV=1
	pulse("CountUp") // CV=2
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 2)
	testBooleanObjectInEnv(t, progEnv, "IsEmpty", false)
	//testBooleanObject(t, mustGet(env, "IsFull"), "IsFull", false)

	// --- Cycle 3: Count down to 1 ---
	pulse("CountDown") // CV=1
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 1)

	// --- Cycle 4: Count up to PV (5) ---
	pulse("CountUp") // CV=2
	pulse("CountUp") // CV=3
	pulse("CountUp") // CV=4
	pulse("CountUp") // CV=5
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 5)
	testBooleanObjectInEnv(t, progEnv, "IsFull", true)
	testBooleanObjectInEnv(t, progEnv, "IsEmpty", false)

	// --- Cycle 5: Try to count past PV ---
	pulse("CountUp") // CV should stay 5
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 5)

	// --- Cycle 6: Load PV ---
	progEnv.Set("Load", TRUE)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 5) // Already at PV, but confirms load
	progEnv.Set("Load", FALSE)
	runScan()

	// --- Cycle 7: Count down to 0 ---
	pulse("CountDown") // 4
	pulse("CountDown") // 3
	pulse("CountDown") // 2
	pulse("CountDown") // 1
	pulse("CountDown") // 0
	testIntegerObjectInEnv(t, progEnv, "CurrentValue", 0)
	testBooleanObjectInEnv(t, progEnv, "IsEmpty", true)
	testBooleanObjectInEnv(t, progEnv, "IsFull", false)
}
