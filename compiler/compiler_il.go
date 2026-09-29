/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"beedance/ast"
	"beedance/code"
	"fmt"
	"strings"
)

var (
	ilArithmeticOpcodes = map[string]code.Opcode{
		"ADD": code.OpAdd, "SUB": code.OpSub, "MUL": code.OpMul, "DIV": code.OpDiv, "MOD": code.OpMod, "AND": code.OpAnd, "OR": code.OpOr, "XOR": code.OpXor,
	}
	ilComparisonOpcodes = map[string]code.Opcode{
		"GT": code.OpGreaterThan, "LT": code.OpLessThan, "EQ": code.OpEqual, "GE": code.OpGreaterThanOrEqual, "LE": code.OpLessThanOrEqual, "NE": code.OpNotEqual,
	}
)

// compileIlProgram compiles a block of IL statements. It performs a single pass,
// resolving jump labels via back-patching.
func (c *Compiler) compileIlProgram(block *ast.BlockStatement) error {
	// jumpsToPatch maps a label name to a slice of instruction positions that need patching.
	jumpsToPatch := make(map[string][]int)
	// labelPositions maps a label name to the instruction position where it is defined.
	labelPositions := make(map[string]int)

	// First pass: just to find label positions without compiling.
	// This is a simplified approach. A more advanced compiler might calculate
	// instruction sizes, but for now, we'll do a "dry run" to get positions.
	// We create a temporary compiler to avoid polluting the main one's instructions.
	posCounter := len(c.currentInstructions())
	for _, stmt := range block.Statements {
		ilStmt, ok := stmt.(*ast.IlInstructionStatement)
		if !ok {
			continue // Should not happen in a valid IL block
		}
		if ilStmt.Label != nil {
			labelPositions[ilStmt.Label.Value] = posCounter
		}
		// This is a placeholder for instruction size calculation.
		// For this implementation, we will assume fixed-size estimation is handled
		// during the actual compilation pass. The back-patching logic below
		// makes this pre-scan for positions less critical, but it's good for validation.
	}

	// Second pass: Compile instructions and handle jumps.
	for _, stmt := range block.Statements {
		ilStmt, ok := stmt.(*ast.IlInstructionStatement)
		if !ok {
			// This could be a FUNCTION declaration or other valid ST statement
			// that can appear before the IL body. We compile it using the main
			// ST compiler, which is correct for handling declarations.
			if err := c.Compile(stmt); err != nil {
				return err
			}
			continue
		}

		// If the instruction has a label, record its current position.
		if ilStmt.Label != nil {
			labelName := ilStmt.Label.Value
			pos := len(c.currentInstructions())
			labelPositions[labelName] = pos

			// Patch any jumps that were waiting for this label.
			if positions, ok := jumpsToPatch[labelName]; ok {
				for _, p := range positions {
					c.changeOperand(p, pos)
				}
				delete(jumpsToPatch, labelName) // Clean up patched jumps
			}
		}

		// Compile the instruction itself.
		if err := c.compileIlInstruction(ilStmt, jumpsToPatch, labelPositions); err != nil {
			return err
		}
	}

	// After the loop, check if any jumps are still unpatched.
	if len(jumpsToPatch) > 0 {
		for label := range jumpsToPatch {
			return fmt.Errorf("undefined jump label: %s", label)
		}
	}

	return nil
}

// compileIlInstruction compiles a single IL instruction.
func (c *Compiler) compileIlInstruction(node *ast.IlInstructionStatement, jumpsToPatch map[string][]int, labelPositions map[string]int) error {
	op := strings.ToUpper(node.Operator)
	modifier := strings.ToUpper(node.Modifier)
	isNegatedOperand := strings.Contains(modifier, "N") && !strings.Contains(modifier, "C")

	// Handle conditional execution for JMP, CAL, RET
	isConditional := strings.Contains(modifier, "C")
	if isConditional && (op == "CAL" || op == "RET") { // JMP is handled in its own case
		isNegated := strings.Contains(modifier, "N")
		// The CR is on top of the stack. We need to jump if it doesn't meet the condition.
		var jumpPos int
		if isNegated { // CALCN, RETCN -> execute if CR is FALSE
			// We want to skip the instruction if the CR is truthy.
			// To do this, we invert the CR and jump if it's now not truthy.
			c.emit(code.OpBang)
			jumpPos = c.emit(code.OpJumpNotTruthy, 9999)
		} else { // CALC, RETC -> execute if CR is TRUE
			// Skip if CR is not truthy.
			jumpPos = c.emit(code.OpJumpNotTruthy, 9999)
		}
		defer func() {
			// This defer will patch the jump to go to the instruction *after*
			// the one we are about to compile.
			afterPos := len(c.currentInstructions())
			c.changeOperand(jumpPos, afterPos)
		}()
	}

	switch op {
	case "LD":
		// Compile the operand, which will be loaded onto the stack.
		if err := c.compileIlOperand(op, node.Operand, isNegatedOperand); err != nil {
			return err
		}
		// Operand is already compiled and on the stack. This becomes the new CR.
		// break
	case "ST":
		if isNegatedOperand {
			c.emit(code.OpBang)
		}
		if ident, ok := node.Operand.(*ast.Identifier); ok {
			symbol, ok := c.symbolTable.Resolve(ident.Value)
			if !ok {
				return fmt.Errorf("undefined variable %s", ident.Value)
			}
			if err := c.setSymbol(symbol); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("operand for ST must be a variable identifier")
		}
	case "S", "R":
		// These are conditional on the CR.
		// The CR is consumed by the conditional check, so we DUP it first.
		c.emit(code.OpDup)
		jumpPos := c.emit(code.OpJumpNotTruthy, 9999)
		if ident, ok := node.Operand.(*ast.Identifier); ok {
			symbol, ok := c.symbolTable.Resolve(ident.Value)
			if !ok {
				return fmt.Errorf("undefined variable %s", ident.Value)
			}
			if op == "S" {
				c.emit(code.OpTrue)
			} else {
				c.emit(code.OpFalse)
			}
			c.setSymbol(symbol)
		}
		c.changeOperand(jumpPos, len(c.currentInstructions()))
	case "ADD", "SUB", "MUL", "DIV", "MOD", "AND", "OR", "XOR":
		// The VM will pop two values, operate, and push one result.
		if err := c.compileIlOperand(op, node.Operand, isNegatedOperand); err != nil {
			return err
		}
		c.emit(ilArithmeticOpcodes[op])

	case "GT", "LT", "EQ", "GE", "LE", "NE":
		// The VM will pop two values, operate, and push one result.
		if err := c.compileIlOperand(op, node.Operand, isNegatedOperand); err != nil {
			return err
		}
		c.emit(ilComparisonOpcodes[op])
	case "JMP":
		ident, ok := node.Operand.(*ast.Identifier)
		if !ok {
			return fmt.Errorf("operand for JMP must be a label identifier")
		}
		labelName := ident.Value
		var pos int

		if isConditional {
			isNegated := strings.Contains(modifier, "N")
			if isNegated { // JMPCN: Jump if condition is NOT true (FALSE)
				pos = c.emit(code.OpJumpNotTruthy, 9999)
			} else { // JMPC: Jump if condition is TRUE
				// We don't have OpJumpTruthy, so we invert the condition and use OpJumpNotTruthy
				c.emit(code.OpBang)
				pos = c.emit(code.OpJumpNotTruthy, 9999)
			}
		} else { // Unconditional JMP
			pos = c.emit(code.OpJump, 9999)
		}
		jumpsToPatch[labelName] = append(jumpsToPatch[labelName], pos)
	case "CAL":
		// Compile the CallExpression operand.
		if node.Operand != nil {
			if err := c.Compile(node.Operand); err != nil {
				return err
			}
		}
		// The CallExpression was already compiled, leaving the FB result on the stack.
		// Per the standard, CAL does not modify the CR. We pop the result to preserve the CR.
		c.emit(code.OpPop) // This assumes CAL always has an operand to pop.
	case "RET":
		c.emit(code.OpReturnValue)
	case "NOT":
		// NOT negates the current result. It has no operand.
		c.emit(code.OpBang)
	default:
		return fmt.Errorf("IL operator not yet supported by compiler: %s", op)
	}
	return nil
}

// compileIlOperand compiles the operand for an IL instruction. It handles standard
// ST expressions as well as deferred IL blocks enclosed in parentheses.
func (c *Compiler) compileIlOperand(op string, operand ast.Expression, isNegated bool) error {
	if operand == nil {
		return fmt.Errorf("%s instruction requires an operand", op)
	}
	if block, ok := operand.(*ast.BlockStatement); ok {
		// It's a deferred block. Compile it as a sub-program.
		if err := c.compileIlProgram(block); err != nil {
			return err
		}
	} else {
		// It's a regular ST expression.
		if err := c.Compile(operand); err != nil {
			return err
		}
	}
	if isNegated {
		c.emit(code.OpBang) // 'NOT' operator
	}
	return nil
}
