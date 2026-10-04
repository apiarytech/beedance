/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

// SizeOfType is SIZEOF of a declared type, in bytes, as CODESYS and TwinCAT
// give it, shared by the compiler and the evaluator. Sizes are packed: a
// structure is the sum of its members, without the alignment padding some
// targets add.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// elementarySizes are the sizes of the elementary types in bytes.
var elementarySizes = map[string]int64{
	"BOOL": 1, "BYTE": 1, "SINT": 1, "USINT": 1, "CHAR": 1,
	"WORD": 2, "INT": 2, "UINT": 2, "WCHAR": 2,
	"DWORD": 4, "DINT": 4, "UDINT": 4, "REAL": 4, "TIME": 4, "DATE": 4,
	"TOD": 4, "TIME_OF_DAY": 4, "DT": 4, "DATE_AND_TIME": 4,
	"LWORD": 8, "LINT": 8, "ULINT": 8, "LREAL": 8,
	"LTIME": 8, "LDATE": 8, "LTOD": 8, "LDT": 8,
}

// SizeResolver gives SizeOfType what only the caller knows: the value of a
// constant expression (an array bound, a string length) and the
// declaration of a named type.
type SizeResolver struct {
	ConstInt func(ast.Expression) (int64, error)
	Type     func(name string) (*ast.TypeDeclaration, bool)
}

// SizeOfDeclaration returns the size of a declared variable or member.
func SizeOfDeclaration(decl *ast.VarDeclStatement, r SizeResolver) (int64, error) {
	return SizeOfType(decl.DataType, decl.StringLength, r)
}

// SizeOfType returns the size of a data type; length is a STRING's declared
// length, or nil for the default of 80 characters.
func SizeOfType(dt, length ast.Expression, r SizeResolver) (int64, error) {
	switch t := dt.(type) {
	case *ast.ArrayDefinition:
		size, err := SizeOfType(t.DataType, nil, r)
		if err != nil {
			return 0, err
		}
		for _, rng := range t.Ranges {
			in, ok := rng.(*ast.InfixExpression)
			if !ok || in.Operator != ".." {
				return 0, fmt.Errorf("array dimension %s is not a range", rng.String())
			}
			low, err1 := r.ConstInt(in.Left)
			high, err2 := r.ConstInt(in.Right)
			if err1 != nil || err2 != nil || high < low {
				return 0, fmt.Errorf("array bounds %s are not constant", rng.String())
			}
			size *= high - low + 1
		}
		return size, nil
	case *ast.StructDefinition:
		var total int64
		for _, m := range t.Members {
			n, err := SizeOfDeclaration(m, r)
			if err != nil {
				return 0, err
			}
			total += n
		}
		return total, nil
	}
	name := ""
	switch t := dt.(type) {
	case *ast.TypeSpecifier:
		name = t.Token.Literal
	case *ast.Identifier:
		name = t.Value
	default:
		return 0, fmt.Errorf("cannot tell the size of %v", dt)
	}
	upper := strings.ToUpper(name)
	if n, ok := elementarySizes[upper]; ok {
		return n, nil
	}
	if upper == "STRING" || upper == "WSTRING" {
		chars := int64(80)
		if length != nil {
			n, err := r.ConstInt(length)
			if err != nil {
				return 0, fmt.Errorf("string length %s is not constant", length.String())
			}
			chars = n
		}
		if upper == "WSTRING" {
			return 2 * (chars + 1), nil
		}
		return chars + 1, nil
	}
	td, ok := r.Type(name)
	if !ok {
		return 0, fmt.Errorf("unknown type %s", name)
	}
	if _, isEnum := td.DataType.(*ast.EnumDefinition); isEnum {
		return 2, nil // an enumeration is stored as an INT
	}
	return SizeOfType(td.DataType, td.StringLength, r)
}

// ElementarySize returns the size of an elementary type name, such as
// LREAL, and whether it is one.
func ElementarySize(name string) (int64, bool) {
	n, ok := elementarySizes[strings.ToUpper(name)]
	return n, ok
}

// StructMember returns the declaration of member name of a structure type,
// written inline or named (types finds a named type's declaration).
func StructMember(dt ast.Expression, name string, types func(string) (*ast.TypeDeclaration, bool)) *ast.VarDeclStatement {
	var members []*ast.VarDeclStatement
	switch t := dt.(type) {
	case *ast.StructDefinition:
		members = t.Members
	case *ast.TypeSpecifier, *ast.Identifier:
		typeName := ""
		if ts, ok := t.(*ast.TypeSpecifier); ok {
			typeName = ts.Token.Literal
		} else {
			typeName = t.(*ast.Identifier).Value
		}
		if td, ok := types(typeName); ok {
			if sd, ok := td.DataType.(*ast.StructDefinition); ok {
				members = sd.Members
			}
		}
	}
	for _, d := range members {
		if d.Name != nil && strings.EqualFold(d.Name.Value, name) {
			return d
		}
	}
	return nil
}
