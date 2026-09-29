// 1. Define an Interface for a generic motor
INTERFACE IMotor
    METHOD Start : BOOL
    END_METHOD

    METHOD Stop : BOOL
    END_METHOD

    PROPERTY Speed : LREAL
    END_PROPERTY
END_INTERFACE

// 2. Create an Abstract Base Function Block
// This FB implements the interface but leaves some parts abstract.
FUNCTION_BLOCK ABSTRACT AbstractMotor IMPLEMENTS IMotor
    VAR_OUTPUT
        IsRunning : BOOL;
    END_VAR
    VAR
        internalSpeed : LREAL;
    END_VAR

    // Provide a concrete implementation for Stop, which can be called by derived classes
    METHOD Stop : BOOL
        IsRunning := FALSE;
        internalSpeed := 0.0;
        Stop := TRUE;
    END_METHOD

    // A concrete method that can be overridden or called via SUPER
    METHOD ResetDiagnostics : BOOL
        // In a real scenario, this would reset internal error codes.
        ResetDiagnostics := TRUE;
    END_METHOD

    // Leave Start and Speed as abstract, forcing derived FBs to implement them
    METHOD ABSTRACT Start : BOOL
    END_METHOD

    PROPERTY ABSTRACT Speed : LREAL
    END_PROPERTY
END_FUNCTION_BLOCK

// 3. Create a Concrete Function Block that inherits from the abstract one
FUNCTION_BLOCK DCMotor EXTENDS AbstractMotor
    VAR_INPUT
        Voltage : LREAL;
    END_VAR
    VAR
        lastVoltage : LREAL;
    END_VAR

    // Provide the concrete implementation for the abstract Start method
    METHOD Start : BOOL
        IF Voltage > 0.0 THEN
            THIS^.IsRunning := TRUE; // Use THIS^ to access own members
            Start := TRUE;
        ELSE
            Start := FALSE;
        END_IF
    END_METHOD

    // Override the Stop method to add more specific logic
    METHOD Stop : BOOL
        // Call the parent's Stop method using SUPER^
        SUPER^.Stop();
        lastVoltage := 0.0;
        Stop := TRUE;
    END_METHOD

    // Provide the concrete implementation for the abstract Speed property
    PROPERTY Speed : LREAL
        GET
            // The implicit 'Speed' variable holds the value being read
            Speed := THIS^.internalSpeed;
        END_GET
        SET
            // The implicit 'Speed' variable holds the value being written
            IF THIS^.IsRunning THEN
                THIS^.internalSpeed := Speed;
            END_IF
        END_SET
    END_PROPERTY

    // Override the default FB logic cycle
    // This is not a method, but the main body of the FB
    IF IsRunning THEN
        internalSpeed := Voltage * 100.0; // Simple logic: speed is proportional to voltage
    ELSE
        internalSpeed := 0.0;
    END_IF;
    lastVoltage := Voltage;

END_FUNCTION_BLOCK


// 4. Create a Program to test the implementation
PROGRAM OopTestProgram
    VAR
        Motor1 : DCMotor;
        TestSpeed : LREAL;
        IsMotor1Running : BOOL;
        IsMotor2Running : BOOL;
        Motor2Speed : LREAL;
    END_VAR

    Motor1(Voltage := 12.0);
    Motor1.Start();
    // Run the FB body again, as the next PLC scan would: now that the motor is
    // running, its logic sets the speed from the voltage (12.0 * 100.0).
    Motor1(Voltage := 12.0);
    TestSpeed := Motor1.Speed;
    IsMotor1Running := Motor1.IsRunning;

END_PROGRAM