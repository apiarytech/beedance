/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package transpiler

import (
	"testing"
)

func TestOOPFeaturesTranspilation(t *testing.T) {
	input := `
INTERFACE IMyInterface
	METHOD MyMethod : INT
	END_METHOD
END_INTERFACE

FUNCTION_BLOCK BaseFB
	VAR
		x : INT;
	END_VAR
	METHOD MyMethod : INT
		MyMethod := 10;
	END_METHOD
END_FUNCTION_BLOCK

FUNCTION_BLOCK DerivedFB EXTENDS BaseFB IMPLEMENTS IMyInterface
	VAR
		y : INT;
	END_VAR
	METHOD MyMethod : INT
		// Call parent's method and add to it
		MyMethod := SUPER^.MyMethod() + 5;
	END_METHOD
END_FUNCTION_BLOCK

PROGRAM OOP_Test
	VAR
		myInstance : DerivedFB;
		result : INT;
	END_VAR

	result := myInstance.MyMethod();
END_PROGRAM
`
	expected := `
// IMyInterface is the transpiled Go interface for the IEC 61131-3 INTERFACE of the same name.
type IMyInterface interface {
	MyMethod() iec.INT
}

// BaseFB is the transpiled struct for the FUNCTION_BLOCK of the same name.
type BaseFB struct {
	EN  iec.BOOL
	ENO iec.BOOL
	x   iec.INT
}

// Logic executes the logic for the BaseFB FUNCTION_BLOCK.
func (b *BaseFB) Logic(now time.Time) {
	if !b.EN {
		b.ENO = false
		return
	}
	b.ENO = true

}

// MyMethod is a method on the BaseFB FUNCTION_BLOCK.
func (b *BaseFB) MyMethod() iec.INT {
	return 10
}

// DerivedFB is the transpiled struct for the FUNCTION_BLOCK of the same name.
type DerivedFB struct {
	BaseFB
	y iec.INT
}

// Logic executes the logic for the DerivedFB FUNCTION_BLOCK.
func (d *DerivedFB) Logic(now time.Time) {
	if !d.EN {
		d.ENO = false
		return
	}
	d.ENO = true

	d.BaseFB.Logic(now)

}

// MyMethod is a method on the DerivedFB FUNCTION_BLOCK.
func (d *DerivedFB) MyMethod() iec.INT {
	return (d.BaseFB.MyMethod() + 5)
}

// Statically assert that DerivedFB implements IMyInterface.
var _ IMyInterface = (*DerivedFB)(nil)

type OOP_Test struct {
	myInstance DerivedFB
	result     iec.INT
}

// NewOOP_TestFactory creates a new instance of the OOP_Test program.
func NewOOP_TestFactory(params map[string]string) (func(time.Time), error) {
	instance := &OOP_Test{}
	instance.myInstance.EN = true
	return instance.Logic, nil
}

// Link connects the program's located variables to the runtime's I/O manager.
func (p *OOP_Test) Link(linker config.IOLinker) error {
	return nil
}

func (p *OOP_Test) Logic(now time.Time) {
	p.result = p.myInstance.MyMethod()
}
`
	transpileAndCheck(t, "TestOOPFeaturesTranspilation", input, expected)
}

func TestAdvancedOOPFeaturesTranspilation(t *testing.T) {
	input := `
INTERFACE IMotor
    METHOD Start : BOOL;
    METHOD Stop : BOOL;
    PROPERTY Speed : LREAL;
END_INTERFACE

FUNCTION_BLOCK ABSTRACT AbstractMotor IMPLEMENTS IMotor
    VAR_OUTPUT
        IsRunning : BOOL;
    END_VAR
    VAR
        internalSpeed : LREAL;
    END_VAR

    METHOD Stop : BOOL
        IsRunning := FALSE;
        internalSpeed := 0.0;
        Stop := TRUE;
    END_METHOD

    METHOD ABSTRACT Start : BOOL
    END_METHOD

    PROPERTY ABSTRACT Speed : LREAL
    END_PROPERTY
END_FUNCTION_BLOCK

FUNCTION_BLOCK DCMotor EXTENDS AbstractMotor
    VAR_INPUT
        Voltage : LREAL;
    END_VAR

    METHOD Start : BOOL
        IF Voltage > 0.0 THEN
            THIS^.IsRunning := TRUE;
            Start := TRUE;
        ELSE
            Start := FALSE;
        END_IF
    END_METHOD

    PROPERTY Speed : LREAL
        GET
            Speed := THIS^.internalSpeed;
        END_GET
        SET
            IF THIS^.IsRunning THEN
                THIS^.internalSpeed := Speed;
            END_IF
        END_SET
    END_PROPERTY

    IF IsRunning THEN
        internalSpeed := Voltage * 100.0;
    END_IF;
END_FUNCTION_BLOCK

PROGRAM OopTestProgram
    VAR
        Motor1 : DCMotor;
        TestSpeed : LREAL;
    END_VAR

    Motor1(Voltage := 12.0);
    Motor1.Start();
    TestSpeed := Motor1.Speed;
    Motor1.Speed := 1500.0;
END_PROGRAM
`
	expected := `
// IMotor is the transpiled Go interface for the IEC 61131-3 INTERFACE of the same name.
type IMotor interface {
	Start() iec.BOOL
	Stop() iec.BOOL
	GetSpeed() iec.LREAL
	SetSpeed(value iec.LREAL)
}

// AbstractMotor is the transpiled struct for the FUNCTION_BLOCK of the same name.
type AbstractMotor struct {
	EN            iec.BOOL
	ENO           iec.BOOL
	IsRunning     iec.BOOL
	internalSpeed iec.LREAL
}

// Logic executes the logic for the AbstractMotor FUNCTION_BLOCK.
func (a *AbstractMotor) Logic(now time.Time) {
	if !a.EN {
		a.ENO = false
		return
	}
	a.ENO = true
}

// Stop is a method on the AbstractMotor FUNCTION_BLOCK.
func (a *AbstractMotor) Stop() iec.BOOL {
	a.IsRunning = false
	a.internalSpeed = 0.000000
	return true
}

// DCMotor is the transpiled struct for the FUNCTION_BLOCK of the same name.
type DCMotor struct {
	AbstractMotor
	Voltage iec.LREAL
}

// Logic executes the logic for the DCMotor FUNCTION_BLOCK.
func (d *DCMotor) Logic(now time.Time) {
	if !d.EN {
		d.ENO = false
		return
	}
	d.ENO = true
	d.AbstractMotor.Logic(now)
	if d.IsRunning {
		d.internalSpeed = (d.Voltage * 100.000000)
	}
}

// Start is a method on the DCMotor FUNCTION_BLOCK.
func (d *DCMotor) Start() iec.BOOL {
	if (d.Voltage > 0.000000) {
		d.IsRunning = true
		return true
	} else {
		return false
	}
}

// GetSpeed is the getter for the Speed property.
func (d *DCMotor) GetSpeed() iec.LREAL {
	return d.internalSpeed
}

// SetSpeed is the setter for the Speed property.
func (d *DCMotor) SetSpeed(value iec.LREAL) {
	if d.IsRunning {
		d.internalSpeed = value
	}
}

// Statically assert that DCMotor implements IMotor.
var _ IMotor = (*DCMotor)(nil)

type OopTestProgram struct {
	Motor1    DCMotor
	TestSpeed iec.LREAL
}

// NewOopTestProgramFactory creates a new instance of the OopTestProgram program.
func NewOopTestProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &OopTestProgram{}
	instance.Motor1.EN = true
	return instance.Logic, nil
}

// Link connects the program's located variables to the runtime's I/O manager.
func (p *OopTestProgram) Link(linker config.IOLinker) error {
	return nil
}

func (p *OopTestProgram) Logic(now time.Time) {
	p.Motor1.Voltage = 12.000000
	p.Motor1.Logic(now)
	p.Motor1.Start()
	p.TestSpeed = p.Motor1.GetSpeed()
	p.Motor1.SetSpeed(1500.000000)
}
`
	transpileAndCheck(t, "TestAdvancedOOPFeaturesTranspilation", input, expected)
}

func TestInterfaceWithPropertyTranspilation(t *testing.T) {
	input := `
INTERFACE ICounter
	METHOD Increment;
	PROPERTY Value : INT;
END_INTERFACE

FUNCTION_BLOCK Counter IMPLEMENTS ICounter
	VAR
		currentValue : INT;
	END_VAR

	METHOD Increment
		currentValue := currentValue + 1;
	END_METHOD

	PROPERTY Value : INT
		GET
			Value := currentValue;
		END_GET
		SET
			currentValue := Value;
		END_SET
	END_PROPERTY
END_FUNCTION_BLOCK

PROGRAM TestCounterProgram
	VAR
		C1 : Counter;
		ReadValue : INT;
	END_VAR

	C1.Value := 10;
	C1.Increment();
	ReadValue := C1.Value;
END_PROGRAM
`
	expected := `
// ICounter is the transpiled Go interface for the IEC 61131-3 INTERFACE of the same name.
type ICounter interface {
	Increment()
	GetValue() iec.INT
	SetValue(value iec.INT)
}

// Counter is the transpiled struct for the FUNCTION_BLOCK of the same name.
type Counter struct {
	EN           iec.BOOL
	ENO          iec.BOOL
	currentValue iec.INT
}

// Logic executes the logic for the Counter FUNCTION_BLOCK.
func (c *Counter) Logic(now time.Time) {
	if !c.EN {
		c.ENO = false
		return
	}
	c.ENO = true

}

// Increment is a method on the Counter FUNCTION_BLOCK.
func (c *Counter) Increment() {
	c.currentValue = (c.currentValue + 1)
}

// GetValue is the getter for the Value property.
func (c *Counter) GetValue() iec.INT {
	return c.currentValue
}

// SetValue is the setter for the Value property.
func (c *Counter) SetValue(value iec.INT) {
	c.currentValue = value
}

// Statically assert that Counter implements ICounter.
var _ ICounter = (*Counter)(nil)

type TestCounterProgram struct {
	C1        Counter
	ReadValue iec.INT
}

// NewTestCounterProgramFactory creates a new instance of the TestCounterProgram program.
func NewTestCounterProgramFactory(params map[string]string) (func(time.Time), error) {
	instance := &TestCounterProgram{}
	instance.C1.EN = true
	return instance.Logic, nil
}

// Link connects the program's located variables to the runtime's I/O manager.
func (p *TestCounterProgram) Link(linker config.IOLinker) error {
	return nil
}

func (p *TestCounterProgram) Logic(now time.Time) {
	p.C1.SetValue(10)
	p.C1.Increment()
	p.ReadValue = p.C1.GetValue()
}
`
	transpileAndCheck(t, "TestInterfaceWithPropertyTranspilation", input, expected)
}
