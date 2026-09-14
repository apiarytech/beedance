package evaluator

import (
	"beedance/object"
	"testing"
	"time"
)

func TestTrafficLightProgram(t *testing.T) {
	// This test simulates the PLC scan cycle for the traffic light program to verify
	// the state machine's behavior and analyze the multiple calls to the StateTimer
	// function block within a single scan.
	input := `
		PROGRAM TrafficLight
			VAR
				State : INT := 0;
				StateTimer : TON;
				Green_Light : BOOL;
				Yellow_Light : BOOL;
				Red_Light : BOOL;
			END_VAR

			(* Call the timer instance on every scan *)
			StateTimer(IN := TRUE, PT := T#5s);

			CASE State OF
				0: (* Green State *)
					Green_Light := TRUE;
					Yellow_Light := FALSE;
					Red_Light := FALSE;
					IF StateTimer.Q THEN
						State := 1;
						StateTimer(IN := FALSE);
					END_IF

				1: (* Yellow State *)
					Green_Light := FALSE;
					Yellow_Light := TRUE;
					Red_Light := FALSE;
					IF StateTimer.Q THEN
						State := 2;
						StateTimer(IN := FALSE);
					END_IF

				2: (* Red State *)
					Green_Light := FALSE;
					Yellow_Light := FALSE;
					Red_Light := TRUE;
					IF StateTimer.Q THEN
						State := 0;
						StateTimer(IN := FALSE);
					END_IF
			END_CASE
		END_PROGRAM
	`

	// Setup mock time
	originalNowFunc := nowFunc
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	advanceTime := func(d time.Duration) {
		mockTime = mockTime.Add(d)
	}

	// Setup environment and parse the program
	env := object.NewEnvironment()
	// Evaluating the program will declare the POU and its variables.
	testEvalWithEnv(t, input, env)

	// Get the program object and its internal environment
	progObj, ok := env.Get("TrafficLight")
	if !ok {
		t.Fatalf("Program 'TrafficLight' not found in environment")
	}
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TrafficLight();`, env)
	}

	// --- Cycle 1: Initial State (t=0s) ---
	runScan()
	testIntegerObjectInEnv(t, progEnv, "State", 0)
	testBooleanObjectInEnv(t, progEnv, "Green_Light", true)
	testBooleanObjectInEnv(t, progEnv, "Yellow_Light", false)
	testBooleanObjectInEnv(t, progEnv, "Red_Light", false)
	stateTimer, _ := progEnv.Get("StateTimer")
	timerEnv := stateTimer.(*object.FunctionBlockInstance).Env
	testBooleanObjectInEnv(t, timerEnv, "Q", false)

	// --- Cycle 2: During Green State (t=4s) ---
	advanceTime(4 * time.Second)
	runScan()
	testIntegerObjectInEnv(t, progEnv, "State", 0) // Still in state 0
	testBooleanObjectInEnv(t, progEnv, "Green_Light", true)
	testTimeObjectInEnv(t, timerEnv, "ET", 4*time.Second)
	testBooleanObjectInEnv(t, timerEnv, "Q", false)

	// --- Cycle 3: Transition to Yellow State (t=5s) ---
	advanceTime(1 * time.Second) // Total time is 5s
	runScan()
	testIntegerObjectInEnv(t, progEnv, "State", 1)
	testBooleanObjectInEnv(t, progEnv, "Green_Light", true) // Light changes on next scan
	testBooleanObjectInEnv(t, timerEnv, "Q", false)         // Timer was reset
	testTimeObjectInEnv(t, timerEnv, "ET", 0)

	// --- Cycle 4: Yellow State (t=5s + 1 scan) ---
	runScan()
	testIntegerObjectInEnv(t, progEnv, "State", 1)
	testBooleanObjectInEnv(t, progEnv, "Green_Light", false)
	testBooleanObjectInEnv(t, progEnv, "Yellow_Light", true)
	testBooleanObjectInEnv(t, progEnv, "Red_Light", false)
}

func TestStandardFunctionBlocks(t *testing.T) {
	// Mock time for timer tests
	originalNowFunc := nowFunc
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	// Helper to advance mock time
	advanceTime := func(d time.Duration) {
		mockTime = mockTime.Add(d)
	}

	t.Run("TON - Timer On-Delay", func(t *testing.T) {
		input := `
			PROGRAM TestTON
				VAR
					MyTimer : TON;
					Start : BOOL;
					TimerDone : BOOL;
					ET : TIME;
				END_VAR

				MyTimer(IN := Start, PT := T#5s, Q => TimerDone, ET => ET);
			END_PROGRAM
		`
		env := object.NewEnvironment()
		// First, evaluate the whole program to set up the environment
		testEvalWithEnv(t, input, env)

		progObj, _ := env.Get("TestTON")
		progEnv := progObj.(*object.Program).Env

		// Helper to run one "scan"
		runScan := func() {
			// In a real app, you'd re-evaluate the program body.
			// For this test, we just need to evaluate the FB call.
			testEvalWithEnv(t, `TestTON();`, env)
		}

		// --- Cycle 1: Initial state ---
		progEnv.Set("Start", FALSE)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerDone", false)
		testTimeObjectInEnv(t, progEnv, "ET", 0)

		// --- Cycle 2: Rising edge on IN ---
		progEnv.Set("Start", TRUE)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerDone", false) // Q is still false
		testTimeObjectInEnv(t, progEnv, "ET", 0)               // ET is still 0 on the first scan

		// --- Cycle 3: Time advances (3s) ---
		advanceTime(3 * time.Second)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerDone", false) // Q is still false
		testTimeObjectInEnv(t, progEnv, "ET", 3*time.Second)

		// --- Cycle 4: Time reaches PT (5s) ---
		advanceTime(2 * time.Second)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerDone", true) // Q is now true
		testTimeObjectInEnv(t, progEnv, "ET", 5*time.Second)  // ET is capped at PT

		// --- Cycle 5: IN is still true, time advances further ---
		advanceTime(2 * time.Second)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerDone", true) // Q remains true
		testTimeObjectInEnv(t, progEnv, "ET", 5*time.Second)  // ET remains capped at PT

		// --- Cycle 6: Falling edge on IN ---
		progEnv.Set("Start", FALSE)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerDone", false) // Q resets to false
		testTimeObjectInEnv(t, progEnv, "ET", 0)               // ET resets to 0
	})

	t.Run("CTU - Counter Up", func(t *testing.T) {
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

		// --- Cycle 7: Fourth rising edge (CV does not exceed PV in this implementation) ---
		progEnv.Set("CountUp", FALSE)
		runScan()
		progEnv.Set("CountUp", TRUE)
		runScan()
		testIntegerObjectInEnv(t, progEnv, "CurrentValue", 3) // CV is capped
		testBooleanObjectInEnv(t, progEnv, "IsDone", true)

		// --- Cycle 8: Reset ---
		progEnv.Set("CountUp", FALSE)
		progEnv.Set("Reset", TRUE)
		runScan()
		testIntegerObjectInEnv(t, progEnv, "CurrentValue", 0)
		testBooleanObjectInEnv(t, progEnv, "IsDone", false)
	})

	t.Run("TOF - Timer Off-Delay", func(t *testing.T) {
		input := `
			PROGRAM TestTOF
				VAR
					MyTimer : TOF;
					Input : BOOL;
					TimerActive : BOOL;
					ET : TIME;
				END_VAR

				MyTimer(IN := Input, PT := T#5s, Q => TimerActive, ET => ET);
			END_PROGRAM
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env)
		progObj, _ := env.Get("TestTOF")
		progEnv := progObj.(*object.Program).Env

		runScan := func() {
			testEvalWithEnv(t, `TestTOF();`, env)
		}

		// --- Cycle 1: IN is high ---
		progEnv.Set("Input", TRUE)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerActive", true)
		testTimeObjectInEnv(t, progEnv, "ET", 0)

		// --- Cycle 2: Falling edge on IN ---
		progEnv.Set("Input", FALSE)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerActive", true) // Q remains true
		testTimeObjectInEnv(t, progEnv, "ET", 0)

		// --- Cycle 3: Time advances (3s) ---
		advanceTime(3 * time.Second)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerActive", true) // Q still true
		testTimeObjectInEnv(t, progEnv, "ET", 3*time.Second)

		// --- Cycle 4: Time reaches PT (5s) ---
		advanceTime(2 * time.Second)
		runScan()
		testBooleanObjectInEnv(t, progEnv, "TimerActive", false) // Q is now false
		testTimeObjectInEnv(t, progEnv, "ET", 5*time.Second)     // ET is capped
	})

	t.Run("CTD - Counter Down", func(t *testing.T) {
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
		progEnv.Set("CountDown", FALSE)
		runScan()

		// --- Cycle 3: Count down to 0 ---
		progEnv.Set("CountDown", TRUE)
		runScan() // CV = 1
		progEnv.Set("CountDown", FALSE)
		runScan()
		progEnv.Set("CountDown", TRUE)
		runScan() // CV = 0
		testIntegerObjectInEnv(t, progEnv, "CurrentValue", 0)
		testBooleanObjectInEnv(t, progEnv, "IsDone", true) // Q is now true
	})
}

func TestTP_PulseTimer(t *testing.T) {
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
		PROGRAM TestTP
			VAR
				MyPulse : TP;
				Trigger : BOOL;
				PulseOut : BOOL;
				ET : TIME;
			END_VAR

			MyPulse(IN := Trigger, PT := T#5s, Q => PulseOut, ET => ET);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(t, input, env)
	progObj, _ := env.Get("TestTP")
	progEnv := progObj.(*object.Program).Env

	runScan := func() {
		testEvalWithEnv(t, `TestTP();`, env)
	}

	// --- Cycle 1: Initial state ---
	progEnv.Set("Trigger", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "PulseOut", false)
	testTimeObjectInEnv(t, progEnv, "ET", 0)

	// --- Cycle 2: Rising edge on IN, pulse starts ---
	progEnv.Set("Trigger", TRUE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "PulseOut", true)
	testTimeObjectInEnv(t, progEnv, "ET", 0)

	// --- Cycle 3: IN goes low, but pulse continues ---
	advanceTime(2 * time.Second)
	progEnv.Set("Trigger", FALSE)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "PulseOut", true)
	testTimeObjectInEnv(t, progEnv, "ET", 2*time.Second)

	// --- Cycle 4: Time reaches PT, pulse ends ---
	advanceTime(3 * time.Second) // Total elapsed time is now 5s
	runScan()
	testBooleanObjectInEnv(t, progEnv, "PulseOut", false)
	testTimeObjectInEnv(t, progEnv, "ET", 5*time.Second)

	// --- Cycle 5: State after pulse completion ---
	advanceTime(1 * time.Second)
	runScan()
	testBooleanObjectInEnv(t, progEnv, "PulseOut", false)
	testTimeObjectInEnv(t, progEnv, "ET", 0)
}
