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

// The operands of IL instructions: the accumulator (cr_BOOL, cr_LINT, ...)
// a value loads into, and a parenthesized expression, ADD( LD b MUL c ),
// which is a closure with accumulators of its own.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
)

// ilFamilyOfGoType is the accumulator of a value of a Go type, or "".
func ilFamilyOfGoType(goType string) string {
	switch goType {
	case "":
		return ""
	case "iec.BOOL":
		return "BOOL"
	case "iec.REAL", "iec.LREAL":
		return "LREAL"
	case "iec.TIME":
		return "TIME"
	case "iec.STRING", "iec.WSTRING":
		return "STRING"
	}
	if strings.HasPrefix(goType, "iec.") {
		return "LINT"
	}
	return ""
}

// ilOperandFamily is the accumulator an operand loads into.
func (t *Transpiler) ilOperandFamily(operand ast.Expression) string {
	if block, ok := operand.(*ast.BlockStatement); ok && isIlBlock(block) {
		return "LINT"
	}
	if m, ok := operand.(*ast.MemberAccessExpression); ok {
		if family := ilFamilyOfGoType(t.standardMemberGoType(m)); family != "" {
			return family
		}
	}
	if decl := t.resolveAssignmentTargetType(operand); decl != nil {
		return t.getBaseTypeFamily(decl.DataType)
	}
	switch operand.(type) {
	case *ast.Boolean:
		return "BOOL"
	case *ast.RealLiteral, *ast.LRealLiteral:
		return "LREAL"
	case *ast.StringLiteral:
		return "STRING"
	}
	if family := ilFamilyOfGoType(t.exprGoType(operand)); family != "" {
		return family
	}
	return "LINT" // other literals and ST expressions
}

// writeIlOperand writes an operand as a value of the accumulator family: a
// parenthesized IL expression as a closure computing it.
func (t *Transpiler) writeIlOperand(operand ast.Expression, family string) error {
	// The current result as a call's first input, LIMIT 0, 10, takes the type
	// of its other inputs, as a generic function's inputs are of one type.
	if call, isCall := operand.(*ast.CallExpression); isCall && len(call.Arguments) > 0 {
		if _, isCR := call.Arguments[0].(*ast.IlCurrentResult); isCR {
			saved := t.ilCRCast
			defer func() { t.ilCRCast = saved }()
			t.ilCRCast = ""
			for _, arg := range call.Arguments[1:] {
				if goType := t.exprGoType(arg); goType != "" {
					t.ilCRCast = goType
					break
				}
			}
		}
	}
	block, ok := operand.(*ast.BlockStatement)
	if !ok || !isIlBlock(block) {
		t.write("iec.%s(", family)
		if err := t.transpileExpression(operand); err != nil {
			return err
		}
		t.write(")")
		return nil
	}
	t.write("func() iec.%s {\n", family)
	if err := t.transpileIlProgram(block); err != nil {
		return err
	}
	inner := t.ilLastCRType
	if inner == "" {
		inner = family
	}
	t.write("\treturn iec.%s(cr_%s)\n\t}()", family, inner)
	return nil
}

// standardFBMembers are the Go types of the inputs and outputs of the
// standard function blocks, which are royaljelly's.
var standardFBMembers = map[string]map[string]string{
	"TON":    {"IN": "iec.BOOL", "PT": "iec.TIME", "Q": "iec.BOOL", "ET": "iec.TIME"},
	"TOF":    {"IN": "iec.BOOL", "PT": "iec.TIME", "Q": "iec.BOOL", "ET": "iec.TIME"},
	"TP":     {"IN": "iec.BOOL", "PT": "iec.TIME", "Q": "iec.BOOL", "ET": "iec.TIME"},
	"CTU":    {"CU": "iec.BOOL", "R": "iec.BOOL", "PV": "iec.INT", "Q": "iec.BOOL", "CV": "iec.INT"},
	"CTD":    {"CD": "iec.BOOL", "LD": "iec.BOOL", "PV": "iec.INT", "Q": "iec.BOOL", "CV": "iec.INT"},
	"CTUD":   {"CU": "iec.BOOL", "CD": "iec.BOOL", "R": "iec.BOOL", "LD": "iec.BOOL", "PV": "iec.INT", "QU": "iec.BOOL", "QD": "iec.BOOL", "CV": "iec.INT"},
	"R_TRIG": {"CLK": "iec.BOOL", "Q": "iec.BOOL"},
	"F_TRIG": {"CLK": "iec.BOOL", "Q": "iec.BOOL"},
	"SR":     {"S1": "iec.BOOL", "R": "iec.BOOL", "Q1": "iec.BOOL"},
	"RS":     {"S": "iec.BOOL", "R1": "iec.BOOL", "Q1": "iec.BOOL"},
}

// standardMemberGoType is the Go type of an input or output of an instance
// of a standard function block, t.Q, or "".
func (t *Transpiler) standardMemberGoType(e *ast.MemberAccessExpression) string {
	inst, ok := e.Struct.(*ast.Identifier)
	if !ok {
		return ""
	}
	decl := t.varInfo[inst.Value]
	if decl == nil || decl.DataType == nil {
		return ""
	}
	typeName := strings.ToUpper(decl.DataType.String())
	if _, declared := t.typeInfo[decl.DataType.String()]; declared {
		return "" // a function block of the program's own
	}
	return standardFBMembers[typeName][strings.ToUpper(e.Member.Value)]
}

// writeIlNegatedOperand is writeIlOperand of an operand with the N
// modifier (LDN, ANDN): its complement, logical for a BOOL.
func (t *Transpiler) writeIlNegatedOperand(operand ast.Expression, family string, negate bool) error {
	if !negate {
		return t.writeIlOperand(operand, family)
	}
	if family == "BOOL" {
		t.write("!")
	} else {
		t.write("^")
	}
	return t.writeIlOperand(operand, family)
}

// ilHoldsObject reports whether an operand is a function block instance, an
// interface or a reference, which no accumulator holds: LD of one is kept
// for an ST? that follows (t.ilLoaded).
func (t *Transpiler) ilHoldsObject(operand ast.Expression) bool {
	ident, ok := operand.(*ast.Identifier)
	if !ok {
		return false
	}
	decl := t.varInfo[ident.Value]
	if decl == nil || decl.DataType == nil {
		return false
	}
	if _, isRef := decl.DataType.(*ast.RefToType); isRef {
		return true
	}
	if t.isFunctionBlockType(decl.DataType) {
		return true
	}
	_, isInterface := t.typeInfo[decl.DataType.String()].(*ast.InterfaceDeclaration)
	return isInterface
}

// transpileIlAttempt writes ST? target, the assignment attempt of the
// instance, interface or reference the last LD loaded.
func (t *Transpiler) transpileIlAttempt(stmt *ast.IlInstructionStatement) error {
	if t.ilLoaded == nil {
		return fmt.Errorf("ST? %s needs an instance, an interface or a reference loaded by LD", stmt.Operand.String())
	}
	tok := token.Token{Type: token.ATTEMPT, Literal: "?=", Row: stmt.Token.Row, Column: stmt.Token.Column}
	return t.transpileAttempt(&ast.AssignmentStatement{Token: tok, Left: stmt.Operand, Value: t.ilLoaded})
}

// standardFBInputOrder are the inputs of each standard function block, in
// order, for a non-formal call, t(TRUE, T#5s).
var standardFBInputOrder = map[string][]string{
	"TON": {"IN", "PT"}, "TOF": {"IN", "PT"}, "TP": {"IN", "PT"},
	"CTU": {"CU", "R", "PV"}, "CTD": {"CD", "LD", "PV"}, "CTUD": {"CU", "CD", "R", "LD", "PV"},
	"R_TRIG": {"CLK"}, "F_TRIG": {"CLK"}, "SR": {"S1", "R"}, "RS": {"S", "R1"},
}

// namedFBArguments returns the arguments of a call of a function block
// instance with each positional one named by its input, as the VM orders
// them: the inputs of the blocks it extends first, then its own, then its
// VAR_IN_OUTs; or a standard block's (TON's IN, PT).
func (t *Transpiler) namedFBArguments(fn ast.Expression, args []ast.Expression) []ast.Expression {
	positional := false
	for _, a := range args {
		switch a.(type) {
		case *ast.NamedArgument, *ast.OutputArgument:
		default:
			positional = true
		}
	}
	if !positional {
		return args
	}
	var names []string
	if fb := t.lookupFunctionBlock(t.valueDataType(fn)); fb != nil {
		var chain []*ast.FunctionBlockDeclaration
		for cur, depth := fb, 0; cur != nil && depth < 16; depth++ {
			chain = append([]*ast.FunctionBlockDeclaration{cur}, chain...)
			if cur.Extends == nil {
				break
			}
			cur = t.getFunctionBlockDefinitionFromTypeInfo(cur.Extends.String())
		}
		var inOuts []string
		for _, b := range chain {
			for _, d := range b.VarInputs {
				names = append(names, d.Name.Value)
			}
			for _, d := range b.VarInOuts {
				inOuts = append(inOuts, d.Name.Value)
			}
		}
		names = append(names, inOuts...)
	} else if ident, ok := fn.(*ast.Identifier); ok {
		if decl := t.varInfo[ident.Value]; decl != nil && decl.DataType != nil {
			names = standardFBInputOrder[strings.ToUpper(decl.DataType.String())]
		}
	}
	out := make([]ast.Expression, len(args))
	position := 0
	for i, a := range args {
		switch a.(type) {
		case *ast.NamedArgument, *ast.OutputArgument:
			out[i] = a
		default:
			if position >= len(names) {
				return args // too many: Go reports it
			}
			out[i] = &ast.NamedArgument{Name: &ast.Identifier{Value: names[position]}, Value: a}
			position++
		}
	}
	return out
}

// notAClassInstance returns an error if fn, a callee, is an instance of a
// CLASS, which has no body to call: only its methods (Table 48).
func (t *Transpiler) notAClassInstance(fn ast.Expression) error {
	if fb := t.lookupFunctionBlock(t.valueDataType(fn)); fb != nil && fb.IsClass() {
		return fmt.Errorf("%s is an instance of CLASS %s, which has no body: call its methods", fn.String(), fb.Name.Value)
	}
	return nil
}
