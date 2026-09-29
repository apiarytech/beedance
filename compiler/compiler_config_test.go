package compiler

import (
	"beedance/code"
	"testing"
	"time"
)

func TestFullConfigurationCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			PROGRAM MotorProgram
				VAR_INPUT
					Speed : INT;
				END_VAR
				VAR_OUTPUT
					Status: INT;
				END_VAR
			END_PROGRAM

			CONFIGURATION MySystem
				RESOURCE PLC1 ON MyDevice
					TASK FastTask (INTERVAL := T#20ms, PRIORITY := 1);
					TASK SlowTask (INTERVAL := T#100ms, PRIORITY := 10);

					PROGRAM Motor1 WITH FastTask : MotorProgram;
					PROGRAM Motor2 WITH SlowTask : MotorProgram;
				END_RESOURCE

				VAR_CONFIG Motor1
					Speed : INT := 500;
				END_VAR
			END_CONFIGURATION
			`,
			// The goal is to ensure a complex configuration block compiles without errors
			// and produces a non-empty bytecode, validating the compiler's ability
			// to handle the full system definition syntax.
			expectedConstants: []interface{}{
				"name", "MySystem", "resources", "PLC1", "type", "MyDevice", "tasks", "FastTask", // 0-7
				"interval", 20 * time.Millisecond, "priority", int64(1), "SlowTask", 100 * time.Millisecond, // 8-13
				int64(10), "programs", "instance", "Motor1", "task", "MotorProgram", "params", // 14-20
				"Speed", int64(500), "Motor2", // 21-23
			},
			expectedInstructions: []code.Instructions{
				// PROGRAM MotorProgram is just a definition, no instructions emitted.
				// CONFIGURATION MySystem
				code.Make(code.OpConstant, 0), // "name"
				code.Make(code.OpConstant, 1), // "MySystem"
				code.Make(code.OpConstant, 2), // "resources"
				// RESOURCE PLC1
				code.Make(code.OpConstant, 0), // "name"
				code.Make(code.OpConstant, 3), // "PLC1"
				code.Make(code.OpConstant, 4), // "type"
				code.Make(code.OpConstant, 5), // "MyDevice"
				code.Make(code.OpConstant, 6), // "tasks"
				// TASK FastTask
				code.Make(code.OpConstant, 0),  // "name"
				code.Make(code.OpConstant, 7),  // "FastTask"
				code.Make(code.OpConstant, 8),  // "interval"
				code.Make(code.OpConstant, 9),  // 20ms
				code.Make(code.OpConstant, 10), // "priority"
				code.Make(code.OpConstant, 11), // 1
				code.Make(code.OpHash, 6),
				// TASK SlowTask
				code.Make(code.OpConstant, 0),  // "name"
				code.Make(code.OpConstant, 12), // "SlowTask"
				code.Make(code.OpConstant, 8),  // "interval"
				code.Make(code.OpConstant, 13), // 100ms
				code.Make(code.OpConstant, 10), // "priority"
				code.Make(code.OpConstant, 14), // 10
				code.Make(code.OpHash, 6),
				code.Make(code.OpArray, 2),     // tasks array
				code.Make(code.OpConstant, 15), // "programs"
				// PROGRAM Motor1
				code.Make(code.OpConstant, 16), // "instance"
				code.Make(code.OpConstant, 17), // "Motor1"
				code.Make(code.OpConstant, 18), // "task"
				code.Make(code.OpConstant, 7),  // "FastTask"
				code.Make(code.OpConstant, 4),  // "type"
				code.Make(code.OpConstant, 19), // "MotorProgram"
				code.Make(code.OpConstant, 20), // "params"
				code.Make(code.OpConstant, 21), // "Speed"
				code.Make(code.OpConstant, 22), // 500
				code.Make(code.OpHash, 2),      // params hash
				code.Make(code.OpHash, 8),      // program hash
				// PROGRAM Motor2
				code.Make(code.OpConstant, 16), // "instance"
				code.Make(code.OpConstant, 23), // "Motor2"
				code.Make(code.OpConstant, 18), // "task"
				code.Make(code.OpConstant, 12), // "SlowTask"
				code.Make(code.OpConstant, 4),  // "type"
				code.Make(code.OpConstant, 19), // "MotorProgram"
				code.Make(code.OpConstant, 20), // "params"
				code.Make(code.OpHash, 0),      // empty params hash
				code.Make(code.OpHash, 8),      // program hash
				code.Make(code.OpArray, 2),     // programs array
				code.Make(code.OpHash, 8),      // resource hash
				code.Make(code.OpArray, 1),     // resources array
				code.Make(code.OpHash, 4),      // config hash
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}

	runCompilerTests(t, tests)
}
