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

	// Compile the instructions. A jump to a label seen already is set at once;
	// a jump forward is patched when its label is reached.
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
			if _, duplicate := labelPositions[labelName]; duplicate {
				return fmt.Errorf("duplicate label defined: %s", labelName)
			}
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

// ilResultName names the hidden variable that holds the IL current result.
// It is not a valid IEC 61131-3 identifier, so it never clashes with one.
const ilResultName = "__CR__"

// ilResult returns the current result's variable in the scope being
// compiled, defining it on first use. Keeping the current result in a
// variable, rather than on the stack, lets ST, S, R, conditional jumps and
// calls read it without consuming it, and keeps the stack balanced across
// jumps and labels.
func (c *Compiler) ilResult() Symbol {
	if symbol, ok := c.symbolTable.store[ilResultName]; ok {
		return symbol
	}
	return c.symbolTable.Define(ilResultName, false)
}

// loadIlResult pushes the current result.
func (c *Compiler) loadIlResult() {
	c.loadSymbol(c.ilResult())
}

// storeIlResult makes the value on top of the stack the current result.
func (c *Compiler) storeIlResult() error {
	return c.setSymbol(c.ilResult())
}

// compileIlInstruction compiles a single IL instruction.
func (c *Compiler) compileIlInstruction(node *ast.IlInstructionStatement, jumpsToPatch map[string][]int, labelPositions map[string]int) error {
	op := strings.ToUpper(node.Operator)
	modifier := strings.ToUpper(node.Modifier)
	isNegatedOperand := strings.Contains(modifier, "N") && !strings.Contains(modifier, "C")
	isConditional := strings.Contains(modifier, "C")

	// CALC/CALCN and RETC/RETCN run only when the current result is TRUE
	// (FALSE for N). JMP handles its own condition.
	if isConditional && (op == "CAL" || op == "RET") {
		c.loadIlResult()
		if strings.Contains(modifier, "N") {
			c.emit(code.OpBang)
		}
		jumpPos := c.emit(code.OpJumpNotTruthy, 9999)
		defer func() {
			// Skip to the instruction after this one.
			c.changeOperand(jumpPos, len(c.currentInstructions()))
		}()
	}

	switch op {
	case "LD":
		if err := c.compileIlOperand(op, node.Operand, isNegatedOperand); err != nil {
			return err
		}
		return c.storeIlResult()
	case "ST":
		ident, ok := node.Operand.(*ast.Identifier)
		if !ok {
			return fmt.Errorf("operand for ST must be a variable identifier")
		}
		c.loadIlResult()
		if isNegatedOperand {
			c.emit(code.OpBang)
		}
		return c.storeTopInto(ident.Value)
	case "S", "R":
		// Set (or reset) the operand when the current result is TRUE.
		ident, ok := node.Operand.(*ast.Identifier)
		if !ok {
			return fmt.Errorf("operand for %s must be a variable identifier", op)
		}
		c.loadIlResult()
		jumpPos := c.emit(code.OpJumpNotTruthy, 9999)
		if op == "S" {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
		if err := c.storeTopInto(ident.Value); err != nil {
			return err
		}
		c.changeOperand(jumpPos, len(c.currentInstructions()))
	case "ADD", "SUB", "MUL", "DIV", "MOD", "AND", "OR", "XOR", "GT", "LT", "EQ", "GE", "LE", "NE":
		// current result := current result <op> operand
		c.loadIlResult()
		if err := c.compileIlOperand(op, node.Operand, isNegatedOperand); err != nil {
			return err
		}
		if opcode, ok := ilArithmeticOpcodes[op]; ok {
			c.emit(opcode)
		} else {
			c.emit(ilComparisonOpcodes[op])
		}
		return c.storeIlResult()
	case "JMP":
		ident, ok := node.Operand.(*ast.Identifier)
		if !ok {
			return fmt.Errorf("operand for JMP must be a label identifier")
		}
		var pos int
		switch {
		case !isConditional:
			pos = c.emit(code.OpJump, 9999)
		case strings.Contains(modifier, "N"): // JMPCN: jump when the current result is FALSE.
			c.loadIlResult()
			pos = c.emit(code.OpJumpNotTruthy, 9999)
		default: // JMPC: jump when the current result is TRUE.
			c.loadIlResult()
			c.emit(code.OpBang)
			pos = c.emit(code.OpJumpNotTruthy, 9999)
		}
		if target, seen := labelPositions[ident.Value]; seen {
			c.changeOperand(pos, target) // A jump back.
		} else {
			jumpsToPatch[ident.Value] = append(jumpsToPatch[ident.Value], pos)
		}
	case "CAL":
		// A function's result becomes the current result; a function block
		// or program has none, so the current result stays.
		if node.Operand != nil {
			if err := c.Compile(node.Operand); err != nil {
				return err
			}
			if call, ok := node.Operand.(*ast.CallExpression); ok && c.calleeFunctionBlock(call.Function) == nil {
				return c.storeIlResult()
			}
			c.emit(code.OpPop)
		}
	case "RET":
		// RET returns from the POU: a function returns its result.
		c.emitFunctionReturn()
	case "NOT":
		c.loadIlResult()
		c.emit(code.OpBang)
		return c.storeIlResult()
	default:
		return fmt.Errorf("IL operator not yet supported by compiler: %s", op)
	}
	return nil
}

// compileIlOperand pushes the operand of an IL instruction: an expression,
// or the result of a parenthesized (deferred) block of IL instructions,
// which runs with its own current result.
func (c *Compiler) compileIlOperand(op string, operand ast.Expression, isNegated bool) error {
	if operand == nil {
		return fmt.Errorf("%s instruction requires an operand", op)
	}
	if block, ok := operand.(*ast.BlockStatement); ok {
		if err := c.compileIlProgram(block); err != nil {
			return err
		}
		c.loadIlResult()
	} else if err := c.Compile(operand); err != nil {
		return err
	}
	if isNegated {
		c.emit(code.OpBang)
	}
	return nil
}
