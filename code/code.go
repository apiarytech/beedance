/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Instructions represents a slice of bytes that make up the bytecode of a program.
type Instructions []byte

// String provides a human-readable representation of the bytecode instructions.
func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))

		i += 1 + read
	}

	return out.String()
}

// fmtInstruction formats a single instruction, including its name and operands, into a string.
func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)

	if len(operands) != operandCount {
		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n",
			len(operands), operandCount)
	}

	switch operandCount {
	// Handles instructions with no operands.
	case 0:
		return def.Name
	// Handles instructions with one operand.
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	// Handles instructions with two operands.
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}

	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n", def.Name)
}

// Opcode is a single byte that represents a virtual machine instruction.
type Opcode byte

// The full set of opcodes supported by the virtual machine.
const (
	OpConstant Opcode = iota

	OpAdd

	OpPop

	OpSub
	OpMul
	OpDiv
	OpMod
	OpExponent

	OpTrue
	OpFalse

	OpEqual
	OpNotEqual
	OpGreaterThan
	OpLessThan
	OpGreaterThanOrEqual
	OpLessThanOrEqual

	OpAnd
	OpOr
	OpXor
	OpNand
	OpNor

	OpMinus
	OpBang

	OpJumpNotTruthy
	OpJump

	OpNull

	OpGetGlobal
	OpSetGlobal

	OpArray
	OpHash
	OpIndex

	OpSetIndex

	OpCall

	OpReturnValue
	OpReturn

	OpGetLocal
	OpSetLocal

	OpGetBuiltin

	OpClosure

	OpGetFree

	OpSetFree

	OpCurrentClosure

	OpGetExternal
	OpSetExternal

	OpDup
	OpSwap

	OpMakeNamedArg
	OpReturnValueMulti

	OpSuperIndex

	// OpArrayBounds sets the declared lower bound of each dimension of the
	// array on top of the stack, from the constant array of bounds its
	// operand names, so that ARRAY[1..3] is indexed from 1.
	OpArrayBounds

	// OpCopy replaces the array or structure on top of the stack with a copy,
	// as they are assigned and passed by value.
	OpCopy
)

// Definition describes an opcode, including its name and the width (in bytes) of its operands.
type Definition struct {
	Name          string
	OperandWidths []int
}

// definitions maps each Opcode to its corresponding Definition.
var definitions = map[Opcode]*Definition{
	// OpConstant pushes a constant from the constant pool onto the stack. Operand: constant index (2 bytes).
	OpConstant: {"OpConstant", []int{2}},

	OpAdd: {"OpAdd", []int{}},

	OpPop: {"OpPop", []int{}},

	OpSub:      {"OpSub", []int{}},
	OpMul:      {"OpMul", []int{}},
	OpDiv:      {"OpDiv", []int{}},
	OpMod:      {"OpMod", []int{}},
	OpExponent: {"OpExponent", []int{}},

	OpTrue:  {"OpTrue", []int{}},
	OpFalse: {"OpFalse", []int{}},

	OpEqual:              {"OpEqual", []int{}},
	OpNotEqual:           {"OpNotEqual", []int{}},
	OpGreaterThan:        {"OpGreaterThan", []int{}},
	OpLessThan:           {"OpLessThan", []int{}},
	OpGreaterThanOrEqual: {"OpGreaterThanOrEqual", []int{}},
	OpLessThanOrEqual:    {"OpLessThanOrEqual", []int{}},

	OpAnd:  {"OpAnd", []int{}},
	OpOr:   {"OpOr", []int{}},
	OpXor:  {"OpXor", []int{}},
	OpNand: {"OpNand", []int{}},
	OpNor:  {"OpNor", []int{}},

	OpMinus: {"OpMinus", []int{}},
	OpBang:  {"OpBang", []int{}},

	OpJumpNotTruthy: {"OpJumpNotTruthy", []int{2}},
	OpJump:          {"OpJump", []int{2}},

	OpNull: {"OpNull", []int{}},

	OpGetGlobal: {"OpGetGlobal", []int{2}},
	OpSetGlobal: {"OpSetGlobal", []int{2}},

	OpArray: {"OpArray", []int{2}},
	OpHash:  {"OpHash", []int{2}},
	OpIndex: {"OpIndex", []int{}},

	OpSetIndex: {"OpSetIndex", []int{}},

	OpCall: {"OpCall", []int{1}},

	OpReturnValue: {"OpReturnValue", []int{}},
	OpReturn:      {"OpReturn", []int{}},

	OpGetLocal: {"OpGetLocal", []int{1}},
	OpSetLocal: {"OpSetLocal", []int{1}},

	OpGetBuiltin: {"OpGetBuiltin", []int{2}},

	OpClosure: {"OpClosure", []int{2, 1}},

	OpGetFree: {"OpGetFree", []int{1}},

	OpSetFree: {"OpSetFree", []int{1}},

	OpCurrentClosure: {"OpCurrentClosure", []int{}},

	OpGetExternal: {"OpGetExternal", []int{2}},
	OpSetExternal: {"OpSetExternal", []int{2}},

	OpDup:  {"OpDup", []int{}},
	OpSwap: {"OpSwap", []int{}},

	OpMakeNamedArg:     {"OpMakeNamedArg", []int{2}},
	OpReturnValueMulti: {"OpReturnValueMulti", []int{}},

	// OpSuperIndex retrieves a method from a parent function block. It expects the instance and the method name (as a string constant) on the stack.
	OpSuperIndex: {"OpSuperIndex", []int{}},

	OpArrayBounds: {"OpArrayBounds", []int{2}},
	OpCopy:        {"OpCopy", []int{}},
}

// Lookup retrieves the Definition for a given opcode byte.
func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}

	return def, nil
}

// Make creates a bytecode instruction from an opcode and its operands.
func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		// Handles 2-byte operands (e.g., for OpConstant).
		case 2:
			binary.LittleEndian.PutUint16(instruction[offset:], uint16(o))
		// Handles 1-byte operands (e.g., for OpGetLocal).
		case 1:
			instruction[offset] = byte(o)
		}
		offset += width
	}
	return instruction
}

// ReadOperands decodes the operands from a bytecode instruction stream based on an opcode's definition.
func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			// Reads a 2-byte operand.
			operands[i] = int(ReadUint16(ins[offset:]))
		case 1:
			// Reads a 1-byte operand.
			operands[i] = int(ReadUint8(ins[offset:]))
		}

		offset += width
	}

	return operands, offset
}

// ReadUint8 reads a single byte from an instruction stream as a uint8.
func ReadUint8(ins Instructions) uint8 { return uint8(ins[0]) }

// ReadUint16 reads two bytes from an instruction stream as a uint16 in little-endian format.
func ReadUint16(ins Instructions) uint16 {
	return binary.LittleEndian.Uint16(ins)
}
