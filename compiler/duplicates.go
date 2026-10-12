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
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/token"
)

// checkDuplicates refuses a POU that declares one name twice. Names are not
// case-sensitive in IEC 61131-3, so `Pump : BOOL` and `pump : ACTUATOR_PUMP`
// are the same variable: compiled, one would overwrite the other at run
// time. The same declaration met twice (a function block's body VAR blocks
// are merged into its Vars) is not a duplicate.
func checkDuplicates(pou string, groups ...[]*ast.VarDeclStatement) error {
	seen := map[string]*ast.VarDeclStatement{}
	for _, group := range groups {
		for _, v := range group {
			if v == nil || v.Name == nil {
				continue
			}
			key := strings.ToUpper(v.Name.Value)
			if first, ok := seen[key]; ok && first != v {
				if first.Name.Value == v.Name.Value {
					return fmt.Errorf("%s: '%s' is declared twice", pou, v.Name.Value)
				}
				return fmt.Errorf("%s: '%s' and '%s' are one name (IEC 61131-3 names are not case-sensitive)", pou, first.Name.Value, v.Name.Value)
			}
			seen[key] = v
		}
	}
	return nil
}

// externalVars and tempVars flatten their blocks for checkDuplicates.
func externalVars(blocks []*ast.ExternalVarDeclaration) []*ast.VarDeclStatement {
	var out []*ast.VarDeclStatement
	for _, b := range blocks {
		out = append(out, b.Vars...)
	}
	return out
}

func tempVars(blocks []*ast.TempVarDeclaration) []*ast.VarDeclStatement {
	var out []*ast.VarDeclStatement
	for _, b := range blocks {
		out = append(out, b.Vars...)
	}
	return out
}

// assignedTypeName is the declared type of an assigned variable: from its
// declaration, or the symbol table (a program's variable).
func (c *Compiler) assignedTypeName(target *ast.Identifier) string {
	if t := c.declaredTypeOf(target); t != nil {
		return c.flattenExpressionToString(t)
	}
	if sym, ok := c.symbolTable.Resolve(target.Value); ok {
		return sym.TypeName
	}
	return ""
}

// bitStringLiteral is value as a bit-string literal when typeName is BYTE,
// WORD, DWORD or LWORD and value a non-negative integer literal that fits;
// otherwise value itself.
func (c *Compiler) bitStringLiteral(typeName string, value ast.Expression) ast.Expression {
	width, ok := object.GetBitStringWidth(strings.ToUpper(typeName))
	if !ok {
		return value
	}
	var v uint64
	var tok token.Token
	switch lit := value.(type) {
	case *ast.IntegerLiteral:
		if lit.Value < 0 {
			return value
		}
		v, tok = uint64(lit.Value), lit.Token
	case *ast.UnsignedIntegerLiteral:
		v, tok = lit.Value, lit.Token
	default:
		return value
	}
	if width < 64 && v >= 1<<uint(width) {
		return value
	}
	return &ast.BitStringLiteral{Token: tok, Value: v, Width: width}
}
