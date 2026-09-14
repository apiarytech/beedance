package evaluator

import (
	"beedance/object"
	"testing"
)

// TestConfigurationAndResourceEvaluation verifies that a full CONFIGURATION block,
// including nested RESOURCEs, TASKs, and PROGRAM instances, is evaluated correctly.
// It checks that variables are correctly scoped and that program instances are created.
func TestConfigurationAndResourceEvaluation(t *testing.T) {
	input := `
		PROGRAM P1
			VAR_INPUT
				ResourceFlag : BOOL; 
			END_VAR
			VAR Sensor : BOOL; END_VAR
			VAR 
				x : INT; 
			END_VAR

			IF ResourceFlag OR Sensor THEN
				x := 1;
			ELSE
				x := 0;
			END_IF
		END_PROGRAM

		CONFIGURATION MyConfig
			VAR_GLOBAL
				GlobalFlag : BOOL := TRUE;
			END_VAR

			VAR_CONFIG
				Res1.P1_inst3.Sensor AT %IX0.5 : BOOL;
			END_VAR

			RESOURCE Res1 ON PLC
				VAR_GLOBAL
					ResourceFlag : BOOL := TRUE;
				END_VAR

				TASK T1 (INTERVAL := T#100ms, PRIORITY := 1);
				TASK T2 (PRIORITY := 2);

				PROGRAM P1_inst1 WITH T1: P1 (ResourceFlag := ResourceFlag);
				PROGRAM P1_inst2 WITH T2: P1 (ResourceFlag := ResourceFlag);
				PROGRAM P1_inst3 WITH T3: P1;

			END_RESOURCE
		END_CONFIGURATION`
	// Simulate hardware input being ON for the test
	ioMap = make(map[string]object.Object) // Reset I/O map
	ioMap["%IX0.5"] = TRUE

	env := object.NewEnvironment()
	result := testEvalWithEnv(t, input, env)

	if !isError(result) && result != NULL {
		t.Fatalf("Expected configuration evaluation to return NULL or an error, got %T", result)
	}

	// Check that the program was defined
	_, ok := env.Get("P1")
	if !ok {
		t.Fatalf("Program 'P1' was not defined in the environment")
	}

	// After evaluation, the environment should contain the configured resource.
	resObj, ok := env.Get("Res1")
	if !ok {
		t.Fatalf("Resource 'Res1' not found in environment after configuration evaluation")
	}

	resInstance, ok := resObj.(*object.FunctionBlockInstance)
	if !ok {
		t.Fatalf("Resource object is not a FunctionBlockInstance, got %T", resObj)
	}
	resEnv := resInstance.Env

	// Check resource-level global variable
	testBooleanObjectInEnv(t, resEnv, "ResourceFlag", true)

	// Check that the task was created correctly
	task1Obj, ok := resEnv.Get("T1")
	if !ok {
		t.Fatalf("Task 'T1' not found in resource environment")
	}
	if _, ok := task1Obj.(*object.Task); !ok {
		t.Fatalf("Task object T1 is not an object.Task, got %T", task1Obj)
	}

	// Check that task T2 was created correctly
	task2Obj, ok := resEnv.Get("T2")
	if !ok {
		t.Fatalf("Task 'T2' not found in resource environment")
	}
	if _, ok := task2Obj.(*object.Task); !ok {
		t.Fatalf("Task object T2 is not an object.Task, got %T", task2Obj)
	}

	// Check program instance 1
	progInstObj1, ok := resEnv.Get("P1_inst1")
	if !ok {
		t.Fatalf("Program instance 'P1_inst1' not found in resource environment")
	}
	progInstance1, ok := progInstObj1.(*object.ProgramInstance)
	if !ok {
		t.Fatalf("Program instance object is not an object.ProgramInstance, got %T", progInstObj1)
	}
	testEvalWithEnv(t, "P1_inst1();", resEnv)
	testIntegerObjectInEnv(t, progInstance1.Env, "x", 1)

	// Check program instance 2, which should also see the resource-level flag
	progInstObj2, ok := resEnv.Get("P1_inst2")
	if !ok {
		t.Fatalf("Program instance 'P1_inst2' not found in resource environment")
	}
	progInstance2, ok := progInstObj2.(*object.ProgramInstance)
	if !ok {
		t.Fatalf("Program instance object is not an object.ProgramInstance, got %T", progInstObj2)
	}
	testEvalWithEnv(t, "P1_inst2();", resEnv)
	testIntegerObjectInEnv(t, progInstance2.Env, "x", 1)

	// Check program instance 3
	// This instance's Sensor is linked to a direct hardware address
	progInstObj3, ok := resEnv.Get("P1_inst3")
	if !ok {
		t.Fatalf("Program instance 'P1_inst3' not found in resource environment")
	}
	progInstance3, ok := progInstObj3.(*object.ProgramInstance)
	if !ok {
		t.Fatalf("Program instance object is not an object.ProgramInstance, got %T", progInstObj3)
	}
	instEnv3 := progInstance3.Env
	sensorObj, _ := instEnv3.Get("Sensor")
	sensorPtr, ok := sensorObj.(*object.Pointer)
	if !ok {
		t.Fatalf("Sensor should be a pointer after VAR_CONFIG AT, got %T", sensorObj)
	}
	if sensorPtr.Env != nil || sensorPtr.Name != "%IX0.5" {
		t.Errorf("Sensor pointer is incorrect. Got Name=%s, Env=%v", sensorPtr.Name, sensorPtr.Env)
	}

	// Now, execute the program instance to see if the mapping works
	testEvalWithEnv(t, "P1_inst3();", resEnv) // Execute in resource env

	// Check the result inside the program instance
	xVar, _ := instEnv3.Get("x")
	if !testIntegerObject(t, xVar, "x", 1) {
		t.Errorf("Expected x to be 1 because Sensor was TRUE via VAR_CONFIG")
	}
}

func TestConfigurationWithVarAccess(t *testing.T) {
	input := `
		PROGRAM P1
			VAR_INPUT
				InVar : INT;
			END_VAR
			VAR
				InternalVar : INT;
			END_VAR
			InternalVar := InVar * 2;
		END_PROGRAM

		CONFIGURATION MyConfig
			VAR_GLOBAL
				GlobalInput : INT := 10;
			END_VAR

			RESOURCE Res1 ON PLC
				TASK T1 (INTERVAL := T#100ms, PRIORITY := 1);
				PROGRAM P1_inst WITH T1 : P1(InVar := GlobalInput);
			END_RESOURCE

			VAR_ACCESS
				AliasForInternal : Res1.P1_inst.InternalVar : INT READ_ONLY;
			END_VAR
		END_CONFIGURATION
	`
	env := object.NewEnvironment()
	// 1. Evaluate the configuration. This should set up everything, including the VAR_ACCESS alias.
	result := testEvalWithEnv(t, input, env)
	if isError(result) {
		t.Fatalf("Configuration evaluation failed: %s", result.Inspect())
	}

	// 2. Check that the alias exists in the config environment and is a pointer.
	aliasObj, ok := env.Get("AliasForInternal")
	if !ok {
		t.Fatalf("VAR_ACCESS alias 'AliasForInternal' not found in environment")
	}
	if _, ok := aliasObj.(*object.Pointer); !ok {
		t.Fatalf("Alias is not a pointer, got %T", aliasObj)
	}

	// 3. Get the resource environment to execute the program instance.
	resObj, _ := env.Get("Res1")
	resEnv := resObj.(*object.FunctionBlockInstance).Env

	// 4. Execute the program instance. This should run `InternalVar := 10 * 2`.
	testEvalWithEnv(t, "P1_inst();", resEnv)

	// 5. Read the value through the alias from the top-level environment.
	// The evalIdentifier logic should dereference the pointer.
	finalValue := testEvalWithEnv(t, "AliasForInternal;", env)
	testIntegerObject(t, finalValue, "AliasForInternal", 20)
}

func TestConfigurationProgramWithoutTask(t *testing.T) {
	input := `
		PROGRAM P1
			VAR x : INT := 1; END_VAR
		END_PROGRAM

		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC
				PROGRAM P1_inst : P1;
			END_RESOURCE
		END_CONFIGURATION
	`
	env := object.NewEnvironment()
	result := testEvalWithEnv(t, input, env)
	if isError(result) {
		t.Fatalf("Configuration evaluation failed: %s", result.Inspect())
	}

	// Get the resource environment
	resObj, ok := env.Get("Res1")
	if !ok {
		t.Fatalf("Resource 'Res1' not found in environment")
	}
	resEnv := resObj.(*object.FunctionBlockInstance).Env

	// Check that the program instance exists
	progInstObj, ok := resEnv.Get("P1_inst")
	if !ok {
		t.Fatalf("Program instance 'P1_inst' not found in resource environment")
	}

	// Check that the task name is empty
	progInstance, _ := progInstObj.(*object.ProgramInstance)
	if progInstance.TaskName != "" {
		t.Errorf("Expected program instance to have no task association, but TaskName is %q", progInstance.TaskName)
	}
}

func TestMultiResourceConfiguration(t *testing.T) {
	input := `
		PROGRAM P_A
			VAR x : INT; END_VAR
			x := 1;
		END_PROGRAM

		PROGRAM P_B
			VAR y : BOOL; END_VAR
			y := TRUE;
		END_PROGRAM

		CONFIGURATION MultiResConfig
			RESOURCE Res_A ON CPU1
				TASK Task_A (INTERVAL := T#10ms);
				PROGRAM PA_inst WITH Task_A : P_A;
			END_RESOURCE

			RESOURCE Res_B ON CPU2
				TASK Task_B (PRIORITY := 1);
				PROGRAM PB_inst WITH Task_B : P_B;
			END_RESOURCE
		END_CONFIGURATION
	`
	env := object.NewEnvironment()
	result := testEvalWithEnv(t, input, env)
	if isError(result) {
		t.Fatalf("Configuration evaluation failed: %s", result.Inspect())
	}

	// --- Check Resource A ---
	resA_Obj, ok := env.Get("Res_A")
	if !ok {
		t.Fatalf("Resource 'Res_A' not found in environment")
	}
	resA_Instance, ok := resA_Obj.(*object.FunctionBlockInstance)
	if !ok {
		t.Fatalf("Resource object Res_A is not a FunctionBlockInstance, got %T", resA_Obj)
	}
	resA_Env := resA_Instance.Env

	if _, ok := resA_Env.Get("Task_A"); !ok {
		t.Errorf("Task 'Task_A' not found in environment for Res_A")
	}
	if _, ok := resA_Env.Get("PA_inst"); !ok {
		t.Errorf("Program instance 'PA_inst' not found in environment for Res_A")
	}

	// --- Check Resource B ---
	resB_Obj, ok := env.Get("Res_B")
	if !ok {
		t.Fatalf("Resource 'Res_B' not found in environment")
	}
	resB_Instance, ok := resB_Obj.(*object.FunctionBlockInstance)
	if !ok {
		t.Fatalf("Resource object Res_B is not a FunctionBlockInstance, got %T", resB_Obj)
	}
	resB_Env := resB_Instance.Env

	if _, ok := resB_Env.Get("Task_B"); !ok {
		t.Errorf("Task 'Task_B' not found in environment for Res_B")
	}
	if _, ok := resB_Env.Get("PB_inst"); !ok {
		t.Errorf("Program instance 'PB_inst' not found in environment for Res_B")
	}
}

func TestConfigurationWithVarExternal(t *testing.T) {
	input := `
		PROGRAM MonitorProgram
			VAR_EXTERNAL
				g_SystemRunning : BOOL;
			END_VAR
			VAR
				LocalCycleCount : DINT := 0;
			END_VAR

			IF g_SystemRunning THEN
				LocalCycleCount := LocalCycleCount + 1;
			END_IF;
		END_PROGRAM

		CONFIGURATION MyPLCSystem
			VAR_GLOBAL
				g_SystemRunning : BOOL := FALSE;
			END_VAR

			RESOURCE CPU_Slot1 ON EmbeddedProcessor
				TASK PeriodicTask(INTERVAL := T#20ms, PRIORITY := 1);
				PROGRAM MainApp WITH PeriodicTask : MonitorProgram;
			END_RESOURCE
		END_CONFIGURATION
	`

	// 1. Evaluate the configuration. This sets up all environments and instances.
	env := object.NewEnvironment()
	result := testEvalWithEnv(t, input, env)
	if isError(result) {
		t.Fatalf("Configuration evaluation failed: %s", result.Inspect())
	}

	// 2. Get the program instance to test against.
	resObj, ok := env.Get("CPU_Slot1")
	if !ok {
		t.Fatal("Resource 'CPU_Slot1' not found in environment")
	}
	resEnv := resObj.(*object.FunctionBlockInstance).Env

	progInstObj, ok := resEnv.Get("MainApp")
	if !ok {
		t.Fatal("Program instance 'MainApp' not found in resource environment")
	}
	progInstance := progInstObj.(*object.ProgramInstance)
	progEnv := progInstance.Env

	// 3. Run the first scan cycle. g_SystemRunning is FALSE, so count should be 0.
	testEvalWithEnv(t, "MainApp();", resEnv)
	testIntegerObjectInEnv(t, progEnv, "LocalCycleCount", 0)

	// 4. Set the global flag to TRUE from the top-level environment.
	env.Set("g_SystemRunning", TRUE)

	// 5. Run the second scan. The program instance should see the change via its VAR_EXTERNAL link.
	testEvalWithEnv(t, "MainApp();", resEnv)
	testIntegerObjectInEnv(t, progEnv, "LocalCycleCount", 1)

	// 6. Run a third scan to confirm the counter increments.
	testEvalWithEnv(t, "MainApp();", resEnv)
	testIntegerObjectInEnv(t, progEnv, "LocalCycleCount", 2)
}
