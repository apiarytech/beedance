/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// To compile this example for a Raspberry Pi (e.g., Pi Zero, 3, 4):
// 1. Install TinyGo: https://tinygo.org/
// 2. Run the following command from the project root:
//    tinygo build -target=pico-w -o program-pico-w.elf ./embedded/example/raspberrypi/main.go
//
// To compile for a Raspberry Pi Pico (RP2040 microcontroller):
//    tinygo build -target=pico -o program-pico.uf2 ./embedded/example/raspberrypi/main.go
//
// This creates a standalone executable that runs the beedance VM with the
// embedded bytecode.

package main

import (
	"fmt"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/vm"
	"time"
)

// This bytecode matches what the compiler generates for the IEC 61131-3
// expression statement `10 + 20;`. code.Make encodes each instruction, so the
// operand widths always match the opcode definitions.

var generatedInstructions = concatInstructions(
	code.Make(code.OpConstant, 0), // loads constant 10
	code.Make(code.OpConstant, 1), // loads constant 20
	code.Make(code.OpAdd),
	code.Make(code.OpPop), // pops the result, which LastPoppedStackElem returns
)

// concatInstructions joins individually encoded instructions into one stream.
func concatInstructions(parts ...[]byte) code.Instructions {
	out := code.Instructions{}
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

var generatedConstants []object.Object

func init() {
	// The constant pool for the bytecode.
	generatedConstants = make([]object.Object, 2)
	generatedConstants[0] = &object.LInt{Value: 10}
	generatedConstants[1] = &object.LInt{Value: 20}
}

func main() {
	fmt.Println("--- Embedded Bytecode Disassembly ---")
	fmt.Println(generatedInstructions.String())
	fmt.Println("------------------------------------")
	fmt.Println("")

	fmt.Println("Starting embedded beedance VM...")

	bytecode := &compiler.Bytecode{
		Instructions: generatedInstructions,
		Constants:    generatedConstants,
	}

	machine := vm.New(bytecode)
	err := machine.Run()
	if err != nil {
		// On a microcontroller, you might flash an LED or send a serial message.
		fmt.Printf("VM execution failed: %s\n", err)
		return
	}

	// The result of the bytecode execution (30) is left on the stack.
	lastPopped := machine.LastPoppedStackElem()
	fmt.Println("VM execution finished.")
	fmt.Println("Result:", lastPopped.Inspect()) // Should print "30"

	// On a real microcontroller, the main function would likely enter an
	// infinite loop to keep the device running.
	for {
		time.Sleep(time.Second)
	}
}
