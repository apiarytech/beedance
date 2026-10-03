package transpiler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// transpileAndCheck is a helper function to parse, transpile, and compare the output.
func transpileAndCheck(t *testing.T, name, input, expected string) {
	t.Helper()

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Errorf("[%s] parser has %d errors:", name, len(p.Errors()))
		for _, msg := range p.Errors() {
			t.Errorf("  - %s", msg)
		}
		t.FailNow()
	}

	var buf bytes.Buffer
	// We need to mock the config package for the generated code to be valid.
	// This is a simplified approach for testing the transpiler's output.
	header := `// Beedance iec to go transpiler converter
package main

import (
	"fmt"
	"time"

	.	"github.com/apiarytech/royaljelly/iec"
	.	"github.com/apiarytech/royaljelly"
)

`
	buf.WriteString(header)

	transpiler := New(&buf)
	err := transpiler.Transpile(program)
	if err != nil {
		t.Fatalf("[%s] transpilation failed: %v", name, err)
	}

	fullExpected := header + expected

	// Normalize whitespace by removing all newlines and tabs, and collapsing multiple spaces.
	// This makes the comparison robust against insignificant formatting changes.
	normalize := func(s string) string {
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.ReplaceAll(s, "\t", " ")
		return strings.Join(strings.Fields(s), " ")
	}

	actualNormalized := normalize(buf.String())
	expectedNormalized := normalize(fullExpected)

	if diff := cmp.Diff(expectedNormalized, actualNormalized); diff != "" {
		t.Errorf("[%s] normalized transpiled output does not match expected. Diff (-want +got):\n%s", name, diff)
		t.Logf("\n--- EXPECTED (raw) ---\n%s\n\n--- ACTUAL (raw) ---\n%s", fullExpected, buf.String())
	}
}

func TestSimpleProgramTranspilation(t *testing.T) {
	input := `
PROGRAM MySimpleProgram
	VAR
		myVar : INT;
		anotherVar : REAL := 3.14;
		isReady : BOOL;
	END_VAR

	myVar := 10 + 5;
	isReady := TRUE;
END_PROGRAM
`
	expected := `
type MySimpleProgram struct {
	myVar iec.INT
	anotherVar iec.REAL
	isReady iec.BOOL
}

// NewMySimpleProgramFactory creates a new instance of the MySimpleProgram program.
func NewMySimpleProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &MySimpleProgram{}
	instance.anotherVar = 3.14
	return instance.Logic, nil
}

func (p *MySimpleProgram) Logic(now time.Time) {
	p.myVar = (10 + 5)
	p.isReady = true
}
`
	transpileAndCheck(t, "TestSimpleProgramTranspilation", input, expected)
}

func TestFunctionBlockTranspilation(t *testing.T) {
	input := `
FUNCTION_BLOCK MyFB
    VAR_INPUT
        In1 : BOOL;
    END_VAR
    VAR_OUTPUT
        Out1 : INT;
    END_VAR

    IF In1 THEN
        Out1 := 10;
    ELSE
        Out1 := 20;
    END_IF
END_FUNCTION_BLOCK
`
	expected := `
// MyFB is the transpiled struct for the FUNCTION_BLOCK of the same name.
type MyFB struct {
	EN  iec.BOOL
	ENO iec.BOOL
	In1 iec.BOOL
	Out1 iec.INT
}

// Logic executes the logic for the MyFB FUNCTION_BLOCK.
func (m *MyFB) Logic(now time.Time) {
	if !m.EN {
		m.ENO = false
		return
	}
	m.ENO = true

	if m.In1 {
		m.Out1 = 10
	} else {
		m.Out1 = 20
	}
}

`
	transpileAndCheck(t, "TestFunctionBlockTranspilation", input, expected)
}

func TestFunctionTranspilation(t *testing.T) {
	input := `
FUNCTION MyFunc : INT
    VAR_INPUT
        A : INT;
    END_VAR
    VAR_IN_OUT
        C : REAL;
    END_VAR
	VAR
		Local : INT;
	END_VAR

    Local := A * 2;
	C := C + 1.0;
    MyFunc := Local;
END_FUNCTION
`
	expected := `
func MyFunc(A iec.INT, C *iec.REAL) (MyFunc iec.INT) {
	var Local iec.INT

	Local = (A * 2)
	(*C) = ((*C) + 1.0)
	MyFunc = Local
	return
}
`
	transpileAndCheck(t, "TestFunctionTranspilation", input, expected)
}

func TestTypeDeclarationTranspilation(t *testing.T) {
	input := `// COLOR is an enumerated type
TYPE 
    COLOR : (RED, GREEN, BLUE);
    POINT : STRUCT
        X : INT;
        Y : INT;
    END_STRUCT;
    SMALL_INT : INT(-100..100);
END_TYPE
`
	expected := `
// COLOR is an enumerated type
type COLOR int

const (
	COLOR_RED COLOR = iota
	COLOR_GREEN
	COLOR_BLUE
)

// POINT is the transpiled struct for the user-defined type.
type POINT struct {
	X iec.INT
	Y iec.INT
}

// SMALL_INT is a subrange of iec.INT.
type SMALL_INT iec.INT
`
	transpileAndCheck(t, "TestTypeDeclarationTranspilation", input, expected)
}

func TestControlFlowTranspilation(t *testing.T) {
	input := `
PROGRAM ControlFlow
    VAR
        x : INT := 0;
        y : INT := 10;
        z : INT;
        color : INT;
    END_VAR

    IF x < y THEN
        x := x + 1;
    ELSIF x > y THEN
        x := x - 1;
    ELSE
        x := 0;
    END_IF

    CASE color OF
        1: z := 10;
        2, 3: z := 20;
        4..7: z := 30;
    ELSE
        z := -1;
    END_CASE

    FOR z := 1 TO 5 BY 1 DO
        x := x + z;
        EXIT;
    END_FOR

    WHILE x < 100 DO
        x := x * 2;
    END_WHILE

    REPEAT
        y := y - 1;
    UNTIL y <= 0
    END_REPEAT
END_PROGRAM
`
	expected := `type ControlFlow struct {
	x iec.INT
	y iec.INT
	z iec.INT
	color iec.INT
}

// NewControlFlowFactory creates a new instance of the ControlFlow program.
func NewControlFlowFactory(params map[string]string) (func(time.Time), error) {
	instance := &ControlFlow{}
	instance.x = 0
	instance.y = 10
	return instance.Logic, nil
}

func (p *ControlFlow) Logic(now time.Time) {
	if (p.x < p.y) {
		p.x = (p.x + 1)
	} else if (p.x > p.y) {
		p.x = (p.x - 1)
	} else {
		p.x = 0
	}

	caseSelector := p.color
	if (caseSelector == 1) {
		p.z = 10
	} else if (caseSelector == 2) || (caseSelector == 3) {
		p.z = 20
	} else if (caseSelector >= 4 && caseSelector <= 7) {
		p.z = 30
	} else {
		p.z = (-1)
	}

	for p.z = 1; p.z <= 5; p.z += 1 {
		p.x = (p.x + p.z)
		break
	}

	for (p.x < 100) {
		p.x = (p.x * 2)
	}

	for {
		p.y = (p.y - 1)
		if (p.y <= 0) { break }
	}
}
`
	transpileAndCheck(t, "TestControlFlowTranspilation", input, expected)
}

func TestSubrangeAssignmentTranspilation(t *testing.T) {
	input := `
TYPE
    SMALL_INT : INT(-100..100);
END_TYPE

PROGRAM SubrangeTest
    VAR
        mySmallInt : SMALL_INT;
        inputVal : INT := 200;
    END_VAR

    mySmallInt := inputVal;
END_PROGRAM
`
	expected := `
// SMALL_INT is a subrange of iec.INT.
type SMALL_INT iec.INT

type SubrangeTest struct {
    mySmallInt SMALL_INT
    inputVal iec.INT
}

// NewSubrangeTestFactory creates a new instance of the SubrangeTest program.
func NewSubrangeTestFactory(params map[string]string) (func(time.Time), error) {
    instance := &SubrangeTest{}
    instance.mySmallInt = -100
    instance.inputVal = 200
    return instance.Logic, nil
}

func (p *SubrangeTest) Logic(now time.Time) {
    p.mySmallInt = SMALL_INT(min(max(p.inputVal, (-100)), 100))
}
`
	transpileAndCheck(t, "TestSubrangeAssignmentTranspilation", input, expected)
}

func TestFunctionBlockInstanceTranspilation(t *testing.T) {
	input := `FUNCTION_BLOCK CounterFB
	VAR_INPUT
		INC : BOOL;
	END_VAR
	VAR_OUTPUT
		CV : INT;
	END_VAR
	VAR
		hasIncremented : BOOL;
	END_VAR

	IF INC AND NOT hasIncremented THEN
		CV := CV + 1;
	END_IF;
	hasIncremented := INC;
END_FUNCTION_BLOCK

PROGRAM TestFBProgram
	VAR
		MyCounter : CounterFB;
		Trigger : BOOL := TRUE;
		Result : INT;
	END_VAR

	MyCounter(INC := Trigger, CV => Result);
END_PROGRAM
`
	expected := `
// CounterFB is the transpiled struct for the FUNCTION_BLOCK of the same name.
type CounterFB struct {
	EN             iec.BOOL
	ENO            iec.BOOL
	INC            iec.BOOL
	CV             iec.INT
	hasIncremented iec.BOOL
}

// Logic executes the logic for the CounterFB FUNCTION_BLOCK.
func (c *CounterFB) Logic(now time.Time) {
	if !c.EN {
		c.ENO = false
		return
	}
	c.ENO = true

	if (c.INC && (!c.hasIncremented)) {
		c.CV = (c.CV + 1)
	}
	c.hasIncremented = c.INC
}

type TestFBProgram struct {
	MyCounter CounterFB
	Trigger   iec.BOOL
	Result    iec.INT
}

// NewTestFBProgramFactory creates a new instance of the TestFBProgram program.
func NewTestFBProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &TestFBProgram{}
	instance.MyCounter.EN = true
	instance.Trigger = true
	return instance.Logic, nil
}

func (p *TestFBProgram) Logic(now time.Time) {
	p.MyCounter.INC = p.Trigger
	p.MyCounter.Logic(now)
	p.Result = p.MyCounter.CV
}
`
	transpileAndCheck(t, "TestFunctionBlockInstanceTranspilation", input, expected)
}

func TestSFCProgramTranspilation(t *testing.T) {
	input := `
PROGRAM MySFC
	VAR
		Go : BOOL;
		x : INT;
		y : INT;
	END_VAR

	ACTION Action1:
		x := 1;
	END_ACTION

	ACTION Action2:
		y := 2;
	END_ACTION

	INITIAL_STEP S1:
		Action1(N);
	END_STEP

	TRANSITION FROM S1 TO S2 := Go; END_TRANSITION

	STEP S2:
		Action2(S);
	END_STEP
END_PROGRAM
`
	// The expected output is very verbose and reflects the current (possibly incomplete) SFC transpilation logic.
	expected := `
type MySFC struct {
	Go iec.BOOL
	x iec.INT
	y iec.INT
	sfcActiveSteps map[string]bool
	S1_X bool
	S1_X_prev bool
	S1_T time.Time
	S2_X bool
	S2_X_prev bool
	S2_T time.Time
	Action1_Q iec.BOOL
	Action1_Timer time.Time
	Action1_ActivationCount int
	Action1_Qualifier string
	Action1_Duration time.Duration
	Action2_Q iec.BOOL
	Action2_Timer time.Time
	Action2_ActivationCount int
	Action2_Qualifier string
	Action2_Duration time.Duration
}

// NewMySFCFactory creates a new instance of the MySFC program.
func NewMySFCFactory(params map[string]string) (func(time.Time), error) {
	instance := &MySFC{}
	instance.sfcActiveSteps = make(map[string]bool)
	instance.sfcActiveSteps["S1"] = true
	return instance.Logic, nil
}

func (p *MySFC) Logic(now time.Time) {
	// --- SFC Phase 0: Store previous step state ---
	p.S1_X_prev = p.S1_X
	p.S2_X_prev = p.S2_X

	// --- SFC Phase 1: Evaluate Transitions and collect fired transitions ---
	firedTransitions := make(map[string]bool)

	// Check transition t3 from [S1] to [S2]
	if p.sfcActiveSteps["S1"] {
		if p.Go {
			firedTransitions["t3"] = true
		}
	}

	// --- SFC Phase 2: Update Step States based on fired transitions ---
	nextActiveSteps := make(map[string]bool)
	// Copy current active steps; they will be deactivated if they are a source of a fired transition.
	for step, active := range p.sfcActiveSteps {
		if active { nextActiveSteps[step] = true }
	}

	if firedTransitions["t3"] {
		delete(nextActiveSteps, "S1")
		nextActiveSteps["S2"] = true
	}

	// --- SFC Phase 3: Update Step and Action States ---
	p.sfcActiveSteps = nextActiveSteps
	// Reset all step active flags
	p.S1_X = p.sfcActiveSteps["S1"]
	p.S2_X = p.sfcActiveSteps["S2"]

	// Process actions for active steps
	// Actions for step S1
	if p.sfcActiveSteps["S1"] {
		p.Action1_Q = true
	} else {
		p.Action1_Q = false
		p.Action1_Timer = time.Time{}
	}
	// Actions for step S2
	if p.sfcActiveSteps["S2"] {
		p.Action2_Q = true
	} else {
	}

	// --- SFC Phase 4: Execute Action Bodies ---
	if p.Action1_Q {
		p.x = 1
	}
	if p.Action2_Q {
		p.y = 2
	}
}
`
	transpileAndCheck(t, "TestSFCProgramTranspilation", input, expected)
}

func TestILProgramTranspilation(t *testing.T) {
	input := `
PROGRAM MyILProgram
	VAR
		A : INT := 10;
		B : INT := 5;
		C : INT;
		D : BOOL;
	END_VAR

	LD 		A
	ADD 	B
	ST 		C

	LD 		C
	GT 		12
	ST 		D
END_PROGRAM
`
	expected := `
type MyILProgram struct {
	A iec.INT
	B iec.INT
	C iec.INT
	D iec.BOOL
}

// NewMyILProgramFactory creates a new instance of the MyILProgram program.
func NewMyILProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &MyILProgram{}
	instance.A = 10
	instance.B = 5
	return instance.Logic, nil
}

func (p *MyILProgram) Logic(now time.Time) {
	// Typed accumulators for IL Current Result (CR)
	var cr_BOOL iec.BOOL
	var cr_LINT iec.LINT
	var cr_LREAL iec.LREAL
	var cr_TIME iec.TIME
	var cr_STRING iec.STRING
	_ = cr_BOOL; _ = cr_LINT; _ = cr_LREAL; _ = cr_TIME; _ = cr_STRING // Avoid unused var errors

	cr_LINT = iec.LINT(p.A)
	cr_LINT = cr_LINT + iec.LINT(p.B)
	p.C = iec.INT(cr_LINT)
	cr_LINT = iec.LINT(p.C)
	cr_BOOL = cr_LINT > iec.LINT(12)
	p.D = iec.BOOL(cr_BOOL)
}
`
	transpileAndCheck(t, "TestILProgramTranspilation", input, expected)
}

func TestSFCSimultaneousBranchTranspilation(t *testing.T) {
	input := `
PROGRAM MySFCForkJoin
	VAR
		ForkCond : BOOL;
		JoinCond : BOOL;
		PathA_Active : BOOL;
		PathB_Active : BOOL;
		Merged_Active : BOOL;
	END_VAR

	ACTION ActionA: PathA_Active := TRUE; END_ACTION
	ACTION ActionB: PathB_Active := TRUE; END_ACTION
	ACTION ActionC: Merged_Active := TRUE; END_ACTION

	INITIAL_STEP S1:
	END_STEP

	TRANSITION FROM S1 TO (S2, S3) := ForkCond; END_TRANSITION

	STEP S2: ActionA(N); END_STEP
	STEP S3: ActionB(N); END_STEP

	TRANSITION FROM S2 TO S4 := TRUE; END_TRANSITION
	TRANSITION FROM S3 TO S5 := TRUE; END_TRANSITION

	STEP S4:
	END_STEP

	STEP S5:
	END_STEP

	TRANSITION FROM (S4, S5) TO S6 := JoinCond; END_TRANSITION

	STEP S6: ActionC(N); END_STEP
END_PROGRAM
`
	expected := `
type MySFCForkJoin struct {
	ForkCond      iec.BOOL
	JoinCond      iec.BOOL
	PathA_Active  iec.BOOL
	PathB_Active  iec.BOOL
	Merged_Active iec.BOOL
	sfcActiveSteps map[string]bool
	S1_X          bool
	S1_X_prev     bool
	S1_T          time.Time
	S2_X          bool
	S2_X_prev     bool
	S2_T          time.Time
	S3_X          bool
	S3_X_prev     bool
	S3_T          time.Time
	S4_X          bool
	S4_X_prev     bool
	S4_T          time.Time
	S5_X          bool
	S5_X_prev     bool
	S5_T          time.Time
	S6_X          bool
	S6_X_prev     bool
	S6_T          time.Time
	ActionA_Q       iec.BOOL
	ActionA_Timer   time.Time
	ActionA_ActivationCount int
	ActionA_Qualifier string
	ActionA_Duration  time.Duration
	ActionB_Q       iec.BOOL
	ActionB_Timer   time.Time
	ActionB_ActivationCount int
	ActionB_Qualifier string
	ActionB_Duration  time.Duration
	ActionC_Q       iec.BOOL
	ActionC_Timer   time.Time
	ActionC_ActivationCount int
	ActionC_Qualifier string
	ActionC_Duration  time.Duration
}

// NewMySFCForkJoinFactory creates a new instance of the MySFCForkJoin program.
func NewMySFCForkJoinFactory(params map[string]string) (func(time.Time), error) {
	instance := &MySFCForkJoin{}
	instance.sfcActiveSteps = make(map[string]bool)
	instance.sfcActiveSteps["S1"] = true
	return instance.Logic, nil
}

func (p *MySFCForkJoin) Logic(now time.Time) {
	// --- SFC Phase 0: Store previous step state ---
	p.S1_X_prev = p.S1_X
	p.S2_X_prev = p.S2_X
	p.S3_X_prev = p.S3_X
	p.S4_X_prev = p.S4_X
	p.S5_X_prev = p.S5_X
	p.S6_X_prev = p.S6_X

	// --- SFC Phase 1: Evaluate Transitions and collect fired transitions ---
	firedTransitions := make(map[string]bool)

	// Check transition t4 from [S1] to [S2 S3]
	if p.sfcActiveSteps["S1"] {
		if p.ForkCond {
			firedTransitions["t4"] = true
		}
	}
	// Check transition t7 from [S2] to [S4]
	if p.sfcActiveSteps["S2"] {
		if true {
			firedTransitions["t7"] = true
		}
	}
	// Check transition t8 from [S3] to [S5]
	if p.sfcActiveSteps["S3"] {
		if true {
			firedTransitions["t8"] = true
		}
	}
	// Check transition t11 from [S4 S5] to [S6]
	if p.sfcActiveSteps["S4"] && p.sfcActiveSteps["S5"] {
		if p.JoinCond {
			firedTransitions["t11"] = true
		}
	}

	// --- SFC Phase 2: Update Step States based on fired transitions ---
	nextActiveSteps := make(map[string]bool)
	// Copy current active steps; they will be deactivated if they are a source of a fired transition.
	for step, active := range p.sfcActiveSteps {
		if active { nextActiveSteps[step] = true }
	}

	if firedTransitions["t4"] {
		delete(nextActiveSteps, "S1")
		nextActiveSteps["S2"] = true
		nextActiveSteps["S3"] = true
	}
	if firedTransitions["t7"] {
		delete(nextActiveSteps, "S2")
		nextActiveSteps["S4"] = true
	}
	if firedTransitions["t8"] {
		delete(nextActiveSteps, "S3")
		nextActiveSteps["S5"] = true
	}
	if firedTransitions["t11"] {
		delete(nextActiveSteps, "S4")
		delete(nextActiveSteps, "S5")
		nextActiveSteps["S6"] = true
	}

	// --- SFC Phase 3: Update Step and Action States ---
	p.sfcActiveSteps = nextActiveSteps
	// Reset all step active flags
	p.S1_X = p.sfcActiveSteps["S1"]
	p.S2_X = p.sfcActiveSteps["S2"]
	p.S3_X = p.sfcActiveSteps["S3"]
	p.S4_X = p.sfcActiveSteps["S4"]
	p.S5_X = p.sfcActiveSteps["S5"]
	p.S6_X = p.sfcActiveSteps["S6"]

	// Process actions for active steps
	// Actions for step S1
	if p.sfcActiveSteps["S1"] {
	} else {
	}
	// Actions for step S2
	if p.sfcActiveSteps["S2"] {
		p.ActionA_Q = true
	} else {
		p.ActionA_Q = false
		p.ActionA_Timer = time.Time{}
	}
	// Actions for step S3
	if p.sfcActiveSteps["S3"] {
		p.ActionB_Q = true
	} else {
		p.ActionB_Q = false
		p.ActionB_Timer = time.Time{}
	}
	// Actions for step S4
	if p.sfcActiveSteps["S4"] {
	} else {
	}
	// Actions for step S5
	if p.sfcActiveSteps["S5"] {
	} else {
	}
	// Actions for step S6
	if p.sfcActiveSteps["S6"] {
		p.ActionC_Q = true
	} else {
		p.ActionC_Q = false
		p.ActionC_Timer = time.Time{}
	}

	// --- SFC Phase 4: Execute Action Bodies ---
	if p.ActionA_Q {
		p.PathA_Active = true
	}
	if p.ActionB_Q {
		p.PathB_Active = true
	}
	if p.ActionC_Q {
		p.Merged_Active = true
	}
}
`
	transpileAndCheck(t, "TestSFCSimultaneousBranchTranspilation", input, expected)
}

func TestSFCTimedActionTranspilation(t *testing.T) {
	input := `
PROGRAM MySFC_Timed
	VAR
		Go1 : BOOL;
		Go2 : BOOL;
		DelayedActionActive : BOOL;
		LimitedActionActive : BOOL;
	END_VAR

	ACTION DelayedAction:
		DelayedActionActive := TRUE;
	END_ACTION

	ACTION LimitedAction:
		LimitedActionActive := TRUE;
	END_ACTION

	INITIAL_STEP S1:
	END_STEP

	TRANSITION FROM S1 TO S2 := Go1; END_TRANSITION

	STEP S2:
		DelayedAction(D, T#2s);
		LimitedAction(L, T#3s);
	END_STEP

	TRANSITION FROM S2 TO S3 := Go2; END_TRANSITION

	STEP S3:
	END_STEP
END_PROGRAM
`
	expected := `
type MySFC_Timed struct {
	Go1                 iec.BOOL
	Go2                 iec.BOOL
	DelayedActionActive iec.BOOL
	LimitedActionActive iec.BOOL
	sfcActiveSteps      map[string]bool
	S1_X                bool
	S1_X_prev           bool
	S1_T                time.Time
	S2_X                bool
	S2_X_prev           bool
	S2_T                time.Time
	S3_X                bool
	S3_X_prev           bool
	S3_T                time.Time
	DelayedAction_Q       iec.BOOL
	DelayedAction_Timer   time.Time
	DelayedAction_ActivationCount int
	DelayedAction_Qualifier string
	DelayedAction_Duration  time.Duration
	LimitedAction_Q       iec.BOOL
	LimitedAction_Timer   time.Time
	LimitedAction_ActivationCount int
	LimitedAction_Qualifier string
	LimitedAction_Duration  time.Duration
}

// NewMySFC_TimedFactory creates a new instance of the MySFC_Timed program.
func NewMySFC_TimedFactory(params map[string]string) (func(time.Time), error) {
	instance := &MySFC_Timed{}
	instance.sfcActiveSteps = make(map[string]bool)
	instance.sfcActiveSteps["S1"] = true
	return instance.Logic, nil
}

func (p *MySFC_Timed) Logic(now time.Time) {
	// --- SFC Phase 0: Store previous step state ---
	p.S1_X_prev = p.S1_X
	p.S2_X_prev = p.S2_X
	p.S3_X_prev = p.S3_X

	// --- SFC Phase 1: Evaluate Transitions and collect fired transitions ---
	firedTransitions := make(map[string]bool)

	// Check transition t3 from [S1] to [S2]
	if p.sfcActiveSteps["S1"] {
		if p.Go1 {
			firedTransitions["t3"] = true
		}
	}
	// Check transition t5 from [S2] to [S3]
	if p.sfcActiveSteps["S2"] {
		if p.Go2 {
			firedTransitions["t5"] = true
		}
	}

	// --- SFC Phase 2: Update Step States based on fired transitions ---
	nextActiveSteps := make(map[string]bool)
	// Copy current active steps; they will be deactivated if they are a source of a fired transition.
	for step, active := range p.sfcActiveSteps {
		if active { nextActiveSteps[step] = true }
	}

	if firedTransitions["t3"] {
		delete(nextActiveSteps, "S1")
		nextActiveSteps["S2"] = true
	}
	if firedTransitions["t5"] {
		delete(nextActiveSteps, "S2")
		nextActiveSteps["S3"] = true
	}

	// --- SFC Phase 3: Update Step and Action States ---
	p.sfcActiveSteps = nextActiveSteps
	// Reset all step active flags
	p.S1_X = p.sfcActiveSteps["S1"]
	p.S2_X = p.sfcActiveSteps["S2"]
	p.S3_X = p.sfcActiveSteps["S3"]

	// Process actions for active steps
	// Actions for step S1
	if p.sfcActiveSteps["S1"] {
	} else {
	}
	// Actions for step S2
	if p.sfcActiveSteps["S2"] {
		if p.DelayedAction_Timer.IsZero() { p.DelayedAction_Timer = now; }
		p.DelayedAction_Q = iec.TIME(now.Sub(p.DelayedAction_Timer)) >= iec.TIME(2000000000)
		if p.LimitedAction_Timer.IsZero() { p.LimitedAction_Timer = now; }
		p.LimitedAction_Q = iec.TIME(now.Sub(p.LimitedAction_Timer)) < iec.TIME(3000000000)
	} else {
		p.DelayedAction_Q = false
		p.DelayedAction_Timer = time.Time{}
		p.LimitedAction_Q = false
		p.LimitedAction_Timer = time.Time{}
	}
	// Actions for step S3
	if p.sfcActiveSteps["S3"] {
	} else {
	}

	// --- SFC Phase 4: Execute Action Bodies ---
	if p.DelayedAction_Q {
		p.DelayedActionActive = true
	}
	if p.LimitedAction_Q {
		p.LimitedActionActive = true
	}
}
`
	transpileAndCheck(t, "TestSFCTimedActionTranspilation", input, expected)
}

func TestSFCActionPriorityTranspilation(t *testing.T) {
	input := `
PROGRAM MySFC_Priority
	VAR
		Go : BOOL;
		ResetIsActive : BOOL;
		SetIsActive : BOOL;
	END_VAR

	ACTION ResetAction:
		ResetIsActive := TRUE;
	END_ACTION

	ACTION SetAction:
		SetIsActive := TRUE;
	END_ACTION

	INITIAL_STEP S1:
		ResetAction(R);
		ResetAction(S); // R should override S
		SetAction(S);
		SetAction(N); // S should override N
	END_STEP

	TRANSITION FROM S1 TO S2 := Go; END_TRANSITION

	STEP S2:
	END_STEP
END_PROGRAM
`
	expected := `
type MySFC_Priority struct {
	Go            iec.BOOL
	ResetIsActive iec.BOOL
	SetIsActive   iec.BOOL
	sfcActiveSteps map[string]bool
	S1_X          bool
	S1_X_prev     bool
	S1_T          time.Time
	S2_X          bool
	S2_X_prev     bool
	S2_T          time.Time
	ResetAction_Q       iec.BOOL
	ResetAction_Timer   time.Time
	ResetAction_ActivationCount int
	ResetAction_Qualifier string
	ResetAction_Duration  time.Duration
	SetAction_Q       iec.BOOL
	SetAction_Timer   time.Time
	SetAction_ActivationCount int
	SetAction_Qualifier string
	SetAction_Duration  time.Duration
}

// NewMySFC_PriorityFactory creates a new instance of the MySFC_Priority program.
func NewMySFC_PriorityFactory(params map[string]string) (func(time.Time), error) {
	instance := &MySFC_Priority{}
	instance.sfcActiveSteps = make(map[string]bool)
	instance.sfcActiveSteps["S1"] = true
	return instance.Logic, nil
}

func (p *MySFC_Priority) Logic(now time.Time) {
	// --- SFC Phase 0: Store previous step state ---
	p.S1_X_prev = p.S1_X
	p.S2_X_prev = p.S2_X

	// --- SFC Phase 1: Evaluate Transitions and collect fired transitions ---
	firedTransitions := make(map[string]bool)

	// Check transition t3 from [S1] to [S2]
	if p.sfcActiveSteps["S1"] {
		if p.Go {
			firedTransitions["t3"] = true
		}
	}

	// --- SFC Phase 2: Update Step States based on fired transitions ---
	nextActiveSteps := make(map[string]bool)
	// Copy current active steps; they will be deactivated if they are a source of a fired transition.
	for step, active := range p.sfcActiveSteps {
		if active { nextActiveSteps[step] = true }
	}

	if firedTransitions["t3"] {
		delete(nextActiveSteps, "S1")
		nextActiveSteps["S2"] = true
	}

	// --- SFC Phase 3: Update Step and Action States ---
	p.sfcActiveSteps = nextActiveSteps
	// Reset all step active flags
	p.S1_X = p.sfcActiveSteps["S1"]
	p.S2_X = p.sfcActiveSteps["S2"]

	// Process actions for active steps
	// Actions for step S1
	if p.sfcActiveSteps["S1"] {
		p.ResetAction_Q = false
		p.SetAction_Q = true
	} else {
		p.ResetAction_Q = false
		p.ResetAction_Timer = time.Time{}
	}
	// Actions for step S2
	if p.sfcActiveSteps["S2"] {
	} else {
	}

	// --- SFC Phase 4: Execute Action Bodies ---
	if p.ResetAction_Q {
		p.ResetIsActive = true
	}
	if p.SetAction_Q {
		p.SetIsActive = true
	}
}
`
	transpileAndCheck(t, "TestSFCActionPriorityTranspilation", input, expected)
}

func TestILProgramWithFBAndJmpTranspilation(t *testing.T) {
	input := `
FUNCTION_BLOCK R_TRIG
    VAR_INPUT
        CLK: BOOL;
    END_VAR
    VAR_OUTPUT
        Q: BOOL;
    END_VAR
    VAR
        M: BOOL;
    END_VAR

    Q := CLK AND NOT M;
    M := CLK;
END_FUNCTION_BLOCK

PROGRAM MyIlCalJmpProgram
    VAR
        DoTrigger : BOOL := FALSE;
        MyTrigger : R_TRIG;
        RisingEdgeDetected : BOOL;
        Counter : INT := 0;
    END_VAR

Loop:
    LD      Counter
    ADD     1
    ST      Counter

    CAL     MyTrigger(CLK := DoTrigger)
    JMPCN   EndLoop
    LD      TRUE
    ST      RisingEdgeDetected

EndLoop:
    LD      Counter
    GT      10
    JMPCN   Loop
END_PROGRAM
`
	expected := `
// R_TRIG is the transpiled struct for the FUNCTION_BLOCK of the same name.
type R_TRIG struct {
	EN  iec.BOOL
	ENO iec.BOOL
	CLK iec.BOOL
	Q   iec.BOOL
	M   iec.BOOL
}

// Logic executes the logic for the R_TRIG FUNCTION_BLOCK.
func (r *R_TRIG) Logic(now time.Time) {
	if !r.EN {
		r.ENO = false
		return
	}
	r.ENO = true

	r.Q = (r.CLK && (!r.M))
	r.M = r.CLK
}

type MyIlCalJmpProgram struct {
	DoTrigger          iec.BOOL
	MyTrigger          R_TRIG
	RisingEdgeDetected iec.BOOL
	Counter            iec.INT
}

// NewMyIlCalJmpProgramFactory creates a new instance of the MyIlCalJmpProgram program.
func NewMyIlCalJmpProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &MyIlCalJmpProgram{}
	instance.DoTrigger = false
	instance.MyTrigger.EN = true
	instance.Counter = 0
	return instance.Logic, nil
}

func (p *MyIlCalJmpProgram) Logic(now time.Time) {
	// Typed accumulators for IL Current Result (CR)
	var cr_BOOL iec.BOOL
	var cr_LINT iec.LINT
	var cr_LREAL iec.LREAL
	var cr_TIME iec.TIME
	var cr_STRING iec.STRING
	_ = cr_BOOL; _ = cr_LINT; _ = cr_LREAL; _ = cr_TIME; _ = cr_STRING // Avoid unused var errors

Loop:
	cr_LINT = iec.LINT(p.Counter)
	cr_LINT = cr_LINT + iec.LINT(1)
	p.Counter = iec.INT(cr_LINT)
	p.MyTrigger.CLK = p.DoTrigger
	p.MyTrigger.Logic(now)
	cr_BOOL = p.MyTrigger.Q
	if !cr_BOOL { goto EndLoop; }
	cr_BOOL = iec.BOOL(true)
	p.RisingEdgeDetected = iec.BOOL(cr_BOOL)
EndLoop:
	cr_LINT = iec.LINT(p.Counter)
	cr_BOOL = cr_LINT > iec.LINT(10)
	if !cr_BOOL { goto Loop; }
}
`
	transpileAndCheck(t, "TestILProgramWithFBAndJmpTranspilation", input, expected)
}

func TestLocatedVariablesTranspilation(t *testing.T) {
	input := `
PROGRAM LocatedVarsProgram
    VAR
        myInput : BOOL AT %IX0.0;
        myOutput : INT AT %QW1;
        memo : REAL AT %MD2;
        internalVar : REAL := 1.23;
    END_VAR

    myOutput := myOutput + 1;
    memo := memo * 2.0;
    myInput := NOT myInput;
END_PROGRAM
`
	// Located variables are copied from royaljelly's process image as the
	// scan starts, and to it as the scan ends.
	expected := `
type LocatedVarsProgram struct {
	myInput iec.BOOL // AT %IX0.0
	myOutput iec.INT // AT %QW1
	memo iec.REAL // AT %MD2
	internalVar iec.REAL
}

// NewLocatedVarsProgramFactory creates a new instance of the LocatedVarsProgram program.
func NewLocatedVarsProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &LocatedVarsProgram{}
	instance.internalVar = 1.23
	return instance.Logic, nil
}

func (p *LocatedVarsProgram) Logic(now time.Time) {
	processImage.Read(func(img *vars.Image) {
		p.myInput = iec.BOOL(img.I.B[0])
		p.memo = iec.REAL(img.M.R[2])
	})
	defer processImage.Write(func(img *vars.Image) {
		img.Q.W[1] = iec.WORD(p.myOutput)
		img.M.R[2] = iec.REAL(p.memo)
	})
	p.myOutput = (p.myOutput + 1)
	p.memo = (p.memo * 2.0)
	p.myInput = (!p.myInput)
}

// processImage holds the located variables (AT %I, %Q, %M) of the programs
// and function blocks in this file. I/O drivers read the outputs from it and
// write the inputs to it.
var processImage vars.ProcessImage
`
	transpileAndCheck(t, "TestLocatedVariablesTranspilation", input, expected)
}

func TestVarAccessTranspilation(t *testing.T) {
	input := `
PROGRAM AccessTest
    VAR
        SourceVar : INT := 123;
        DestVar : INT;
    END_VAR
    VAR_ACCESS
        LocalSource : SourceVar READ_ONLY;
    END_VAR

    DestVar := LocalSource;
END_PROGRAM
`
	expected := `
type AccessTest struct {
	SourceVar   iec.INT
	DestVar     iec.INT
	LocalSource *iec.INT // VAR_ACCESS SourceVar
}

// NewAccessTestFactory creates a new instance of the AccessTest program.
func NewAccessTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &AccessTest{}
	instance.SourceVar = 123
	return instance.Logic, nil
}

// LinkAccess points the program's VAR_ACCESS variables at the variables
// their access paths name; resolve returns a pointer to the variable a path names.
func (p *AccessTest) LinkAccess(resolve func(path string) any) error {
	if v, ok := resolve("SourceVar").(*iec.INT); ok {
		p.LocalSource = v
	} else {
		return fmt.Errorf("VAR_ACCESS LocalSource: SourceVar does not name a variable of type INT")
	}
	return nil
}

func (p *AccessTest) Logic(now time.Time) {
	p.DestVar = (*p.LocalSource)
}
`
	transpileAndCheck(t, "TestVarAccessTranspilation", input, expected)
}

func TestVarTempAndExternalTranspilation(t *testing.T) {
	input := `VAR_GLOBAL
	Global_Var : INT := 100; END_VAR

	PROGRAM TempAndExternalTest
		VAR_EXTERNAL
			Global_Var : INT;
		END_VAR
		VAR_TEMP
			Temp_Var : INT := 5;
		END_VAR
		VAR
			Result : INT;
		END_VAR

		Temp_Var := Temp_Var + 1;
		Result := Global_Var + Temp_Var;
	END_PROGRAM
	`
	expected := `// --- VAR_GLOBAL ---
var Global_Var iec.INT = 100

type TempAndExternalTest struct {
	Result iec.INT
}

// NewTempAndExternalTestFactory creates a new instance of the TempAndExternalTest program.
func NewTempAndExternalTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &TempAndExternalTest{}
	return instance.Logic, nil
}

func (p *TempAndExternalTest) Logic(now time.Time) {
	var Temp_Var iec.INT = 5
	Temp_Var = (Temp_Var + 1)
	p.Result = (Global_Var + Temp_Var)
}
`
	transpileAndCheck(t, "TestVarTempAndExternalTranspilation", input, expected)
}

func TestArrayRepetitionTranspilation(t *testing.T) {
	input := `
	PROGRAM ArrayRepTest
		VAR
			myArray : ARRAY[1..5] OF INT := [2(10), 3(20)];
			anotherArray : ARRAY[1..7] OF INT := [1, 2, 3(0), 4, 5];
		END_VAR

		myArray[1] := 0; // The first element of ARRAY[1..5].

	END_PROGRAM
	`
	expected := `
type ArrayRepTest struct {
	myArray      [5]iec.INT
	anotherArray [7]iec.INT
}

// NewArrayRepTestFactory creates a new instance of the ArrayRepTest program.
func NewArrayRepTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &ArrayRepTest{}
	instance.myArray = [5]iec.INT{10, 10, 20, 20, 20}
	instance.anotherArray = [7]iec.INT{1, 2, 0, 0, 0, 4, 5}
	return instance.Logic, nil
}

func (p *ArrayRepTest) Logic(now time.Time) {
	p.myArray[0] = 0
}
`
	transpileAndCheck(t, "TestArrayRepetitionTranspilation", input, expected)
}

func TestMacroTranspilation(t *testing.T) {
	input := `
		PROGRAM MacroTestProgram
			VAR
				twice : MACRO := macro(a) { EXPR(EVAL(a) * 2); };
				result : INT;
			END_VAR

			result := twice(10 + 5);
		END_PROGRAM
		`
	// The macro `twice(10 + 5)` should be expanded to `((10 + 5) * 2)`
	// before being transpiled.
	expected := `type MacroTestProgram struct {
	result iec.INT
}

// NewMacroTestProgramFactory creates a new instance of the MacroTestProgram program.
func NewMacroTestProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &MacroTestProgram{}
	return instance.Logic, nil
}

func (p *MacroTestProgram) Logic(now time.Time) {
	p.result = ((10 + 5) * 2)
}
`
	transpileAndCheck(t, "TestMacroTranspilation", input, expected)
}

func TestConfigurationTranspilation(t *testing.T) {
	input := `
PROGRAM MyProgram
	VAR_INPUT
		ConfigInput : INT;
	END_VAR
	VAR
		myVar : INT;
	END_VAR
	myVar := ConfigInput;
END_PROGRAM

CONFIGURATION MyConfig
	RESOURCE Res1 ON PLC
		TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
		PROGRAM P1 WITH Task1 : MyProgram;
	END_RESOURCE

	VAR_CONFIG P1
		ConfigInput := 42;
	END_VAR
END_CONFIGURATION
`
	expected := `
type MyProgram struct {
	ConfigInput iec.INT
	myVar iec.INT
}

// NewMyProgramFactory creates a new instance of the MyProgram program.
func NewMyProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &MyProgram{}
	if _, ok := params["ConfigInput"]; ok {
		var v iec.LINT
		if err := config.ParseLINT(params, "ConfigInput", &v); err != nil {
			return nil, err
		}
		instance.ConfigInput = iec.INT(v)
	}
	return instance.Logic, nil
}

func (p *MyProgram) Logic(now time.Time) {
	p.myVar = p.ConfigInput
}

// main runs configuration MyConfig until the process is interrupted.
func main() {
	config.RegisterProgramFactory("MyProgram", NewMyProgramFactory)
	cfg := &core.Configuration{Name: "MyConfig"}

	// RESOURCE Res1
	res1 := &core.Resource{Name: "Res1"}
	res1_task1 := core.NewTask("Task1", core.CyclicTask, 0, time.Duration(iec.TIME(100000000)))
	res1.AddTask(res1_task1)
	res1_prog1, err := NewMyProgramFactory(map[string]string{"ConfigInput": "42"})
	if err != nil {
		log.Fatalf("program P1: %v", err)
	}
	res1_task1.AddProgram(&core.Program{Name: "P1", Logic: res1_prog1})
	cfg.AddResource(res1)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cfg.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
`
	transpileAndCheck(t, "TestConfigurationTranspilation", input, expected)
}

func TestExpressionStatementAssignmentWorkaround(t *testing.T) {
	input := `
PROGRAM AssignmentWorkaround
	VAR
		myArray : ARRAY[0..4] OF INT;
		i : INT := 0;
	END_VAR

	myArray[i] := 5;
END_PROGRAM
`
	expected := `
type AssignmentWorkaround struct {
	myArray [5]iec.INT
	i       iec.INT
}

// NewAssignmentWorkaroundFactory creates a new instance of the AssignmentWorkaround program.
func NewAssignmentWorkaroundFactory(params map[string]string) (func(time.Time), error) {
	instance := &AssignmentWorkaround{}
	instance.myArray = [5]iec.INT{}
	instance.i = 0
	return instance.Logic, nil
}

func (p *AssignmentWorkaround) Logic(now time.Time) {
	p.myArray[p.i] = 5
}
`
	transpileAndCheck(t, "TestExpressionStatementAssignmentWorkaround", input, expected)
}

func TestIlBuiltinFunctionAndStandardFbCall(t *testing.T) {
	input := `
PROGRAM MyIlBuiltins
	VAR
		neg_val : INT := -10;
		abs_val : INT;
		my_timer : TON;
		timer_in : BOOL := TRUE;
		timer_q : BOOL;
	END_VAR

	LD 		neg_val
	ABS
	ST 		abs_val

	CAL my_timer(IN := timer_in, PT := T#2s)
	ST timer_q
END_PROGRAM
`
	expected := `
type MyIlBuiltins struct {
	neg_val  iec.INT
	abs_val  iec.INT
	my_timer timers.TON
	timer_in iec.BOOL
	timer_q  iec.BOOL
}

// NewMyIlBuiltinsFactory creates a new instance of the MyIlBuiltins program.
func NewMyIlBuiltinsFactory(params map[string]string) (func(time.Time), error) {
	instance := &MyIlBuiltins{}
	instance.neg_val = (-10)
	instance.timer_in = true
	return instance.Logic, nil
}

func (p *MyIlBuiltins) Logic(now time.Time) {
	// Typed accumulators for IL Current Result (CR)
	var cr_BOOL iec.BOOL
	var cr_LINT iec.LINT
	var cr_LREAL iec.LREAL
	var cr_TIME iec.TIME
	var cr_STRING iec.STRING
	_ = cr_BOOL; _ = cr_LINT; _ = cr_LREAL; _ = cr_TIME; _ = cr_STRING // Avoid unused var errors

	cr_LINT = iec.LINT(p.neg_val)
	cr_LREAL = iec.LREAL(numerical.ABS(cr_LINT))
	p.abs_val = iec.INT(cr_LREAL)

	p.my_timer.IN = p.timer_in
	p.my_timer.PT = iec.TIME(2000000000)
	p.my_timer.Execute(now)
	cr_BOOL = p.my_timer.Q
	p.timer_q = iec.BOOL(cr_BOOL)
}
`
	transpileAndCheck(t, "TestIlBuiltinFunctionAndStandardFbCall", input, expected)
}

func TestSfcStoredAndPulseActions(t *testing.T) {
	input := `
PROGRAM SfcStoredPulse
	VAR
		Go : BOOL;
		PulseActionActive : BOOL;
		StoredDelayedActive : BOOL;
		StoredLimitedActive : BOOL;
	END_VAR

	ACTION PulseAction: PulseActionActive := TRUE; END_ACTION
	ACTION StoredDelayedAction: StoredDelayedActive := TRUE; END_ACTION
	ACTION StoredLimitedAction: StoredLimitedActive := TRUE; END_ACTION

	INITIAL_STEP S1:
		PulseAction(P);
		StoredDelayedAction(SD, T#1s);
		StoredLimitedAction(SL, T#2s);
	END_STEP

	TRANSITION FROM S1 TO S2 := Go; END_TRANSITION

	STEP S2:
	END_STEP
END_PROGRAM
`
	expected := `
type SfcStoredPulse struct {
	Go                  iec.BOOL
	PulseActionActive   iec.BOOL
	StoredDelayedActive iec.BOOL
	StoredLimitedActive iec.BOOL
	sfcActiveSteps      map[string]bool
	S1_X                bool
	S1_X_prev           bool
	S1_T                time.Time
	S2_X                bool
	S2_X_prev           bool
	S2_T                time.Time
	PulseAction_Q       iec.BOOL
	PulseAction_Timer   time.Time
	PulseAction_ActivationCount int
	PulseAction_Qualifier string
	PulseAction_Duration  time.Duration
	StoredDelayedAction_Q       iec.BOOL
	StoredDelayedAction_Timer   time.Time
	StoredDelayedAction_ActivationCount int
	StoredDelayedAction_Qualifier string
	StoredDelayedAction_Duration  time.Duration
	StoredLimitedAction_Q       iec.BOOL
	StoredLimitedAction_Timer   time.Time
	StoredLimitedAction_ActivationCount int
	StoredLimitedAction_Qualifier string
	StoredLimitedAction_Duration  time.Duration
}

// NewSfcStoredPulseFactory creates a new instance of the SfcStoredPulse program.
func NewSfcStoredPulseFactory(params map[string]string) (func(time.Time), error) {
	instance := &SfcStoredPulse{}
	instance.sfcActiveSteps = make(map[string]bool)
	instance.sfcActiveSteps["S1"] = true
	return instance.Logic, nil
}

func (p *SfcStoredPulse) Logic(now time.Time) {
	// --- SFC Phase 0: Store previous step state ---
	p.S1_X_prev = p.S1_X
	p.S2_X_prev = p.S2_X

	// --- SFC Phase 1: Evaluate Transitions and collect fired transitions ---
	firedTransitions := make(map[string]bool)

	// Check transition t4 from [S1] to [S2]
	if p.sfcActiveSteps["S1"] {
		if p.Go {
			firedTransitions["t4"] = true
		}
	}

	// --- SFC Phase 2: Update Step States based on fired transitions ---
	nextActiveSteps := make(map[string]bool)
	// Copy current active steps; they will be deactivated if they are a source of a fired transition.
	for step, active := range p.sfcActiveSteps {
		if active { nextActiveSteps[step] = true }
	}

	if firedTransitions["t4"] {
		delete(nextActiveSteps, "S1")
		nextActiveSteps["S2"] = true
	}

	// --- SFC Phase 3: Update Step and Action States ---
	p.sfcActiveSteps = nextActiveSteps
	// Reset all step active flags
	p.S1_X = p.sfcActiveSteps["S1"]
	p.S2_X = p.sfcActiveSteps["S2"]

	// Process actions for active steps
	// Actions for step S1
	if p.sfcActiveSteps["S1"] {
		p.PulseAction_Q = iec.BOOL(p.S1_X && !p.S1_X_prev)
		if p.StoredDelayedAction_Timer.IsZero() { p.StoredDelayedAction_Timer = now }
		if !p.StoredDelayedAction_Q && iec.TIME(now.Sub(p.StoredDelayedAction_Timer)) >= iec.TIME(1000000000) {
			p.StoredDelayedAction_Q = true
		}
		if p.StoredLimitedAction_Timer.IsZero() { p.StoredLimitedAction_Timer = now p.StoredLimitedAction_Q = true }
		if iec.TIME(now.Sub(p.StoredLimitedAction_Timer)) >= iec.TIME(2000000000) {
			p.StoredLimitedAction_Q = false
		}
	} else {
		p.PulseAction_Q = false
		p.PulseAction_Timer = time.Time{}
		if !p.StoredDelayedAction_Timer.IsZero() && !bool(p.StoredDelayedAction_Q) && iec.TIME(now.Sub(p.StoredDelayedAction_Timer)) >= iec.TIME(1000000000) {
			p.StoredDelayedAction_Q = true
		}
		if !p.StoredLimitedAction_Timer.IsZero() && bool(p.StoredLimitedAction_Q) && iec.TIME(now.Sub(p.StoredLimitedAction_Timer)) >= iec.TIME(2000000000) {
			p.StoredLimitedAction_Q = false
		}
	}
	// Actions for step S2
	if p.sfcActiveSteps["S2"] {
	} else {
	}

	// --- SFC Phase 4: Execute Action Bodies ---
	if p.PulseAction_Q {
		p.PulseActionActive = true
	}
	if p.StoredDelayedAction_Q {
		p.StoredDelayedActive = true
	}
	if p.StoredLimitedAction_Q {
		p.StoredLimitedActive = true
	}
}
`
	transpileAndCheck(t, "TestSfcStoredAndPulseActions", input, expected)
}

func TestMiscellaneousExpressions(t *testing.T) {
	input := `
TYPE MyColor : (RED, GREEN); END_TYPE
FUNCTION MyFunc : INT
	RETURN 123;
END_FUNCTION

PROGRAM MiscTest
	VAR
		c : MyColor := MyColor#RED;
		ws : WSTRING := "wide";
		t : TIME := T#5s;
		d : DATE := D#2026-01-02;
		tod : TIME_OF_DAY := TOD#14:21:00;
		dt : DATE_AND_TIME := DT#2026-01-02-14:21:00;
	END_VAR
	h := {'key': 'value'};
	f := fn() {};
	MyFunc();
END_PROGRAM
`
	expected := `
type MyColor int
const (
	MyColor_RED MyColor = iota
	MyColor_GREEN
)

func MyFunc() (MyFunc iec.INT) {
	return 123
}

type MiscTest struct {
	c   MyColor
	ws  iec.WSTRING
	t   iec.TIME
	d   iec.DATE
	tod iec.TIME_OF_DAY
	dt  iec.DT
	f any // Inferred
	h any // Inferred
}

// NewMiscTestFactory creates a new instance of the MiscTest program.
func NewMiscTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &MiscTest{}
	instance.c = MyColor_RED
	instance.ws = "wide"
	instance.t = iec.TIME(5000000000)
	instance.d = iec.DATE(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	instance.tod = iec.TOD(time.Date(0, 1, 1, 14, 21, 0, 0, time.UTC))
	instance.dt = iec.DT(time.Date(2026, 1, 2, 14, 21, 0, 0, time.UTC))
	return instance.Logic, nil
}

func (p *MiscTest) Logic(now time.Time) {
	p.h = map[any]any {
		"key": "value",
	}
	p.f = func() {}
	MyFunc()
}
`
	transpileAndCheck(t, "TestMiscellaneousExpressions", input, expected)
}
