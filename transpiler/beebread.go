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

// This file ties the generated Go to beebread, the Go port of the OSCAT
// BASIC library: a program can call OSCAT's functions, declare its function
// blocks and structured types, and use its constants. beebread_library.go,
// generated from beebread, describes them.
//
// beebread follows royaljelly's conventions. A function takes its inputs in
// OSCAT's order, then its VAR_IN_OUTs as pointers; a POINTER TO an array,
// which OSCAT passes as ADR(array) with SIZEOF(array), is a slice. A
// function block has its inputs and outputs as upper case fields, pointer
// fields for its VAR_IN_OUTs, and the methods INIT and Execute(now). A name
// that starts with an underscore ends with one instead: _BUFFER_CLEAR is
// BUFFER_CLEAR_.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

func init() {
	for alias, path := range beebreadPackages {
		generatedPackages[alias] = path
		reservedGoNames[alias] = true
	}
	// SIZEOF is the size of a Go array.
	generatedPackages["unsafe"] = "unsafe"
	reservedGoNames["unsafe"] = true
}

// beebreadGoName returns the Go name of an OSCAT name.
func beebreadGoName(name string) string {
	name = strings.ToUpper(name)
	if strings.HasPrefix(name, "_") {
		return strings.TrimPrefix(name, "_") + "_"
	}
	return name
}

// beebreadFunctionOf returns the beebread function a call names, and its Go
// name, unless the program declares the name or royaljelly has the
// function.
func (t *Transpiler) beebreadFunctionOf(fn ast.Expression) (string, stdFunction, bool) {
	ident, ok := fn.(*ast.Identifier)
	if !ok || t.lookupType(ident.Value) != nil {
		return "", stdFunction{}, false
	}
	_, isStd := royaljellyFunctions[strings.ToUpper(ident.Value)]
	name := beebreadGoName(ident.Value)
	if f, ok := beebreadFunctions[name]; ok && (!isStd || t.PreferOSCAT) {
		return name, f, true
	}
	// A standard function royaljelly does not have, such as STRING_TO_INT.
	if f, ok := beebreadStandard[name]; ok && !isStd {
		return name, f, true
	}
	return "", stdFunction{}, false
}

// beebreadTypeOf returns the beebread function block or structured type a
// data type names, unless the program declares a type of the same name.
func (t *Transpiler) beebreadTypeOf(dataType ast.Expression) (beebreadFunctionBlock, bool) {
	if dataType == nil {
		return beebreadFunctionBlock{}, false
	}
	var name string
	switch dt := dataType.(type) {
	case *ast.Identifier:
		name = dt.Value
	case *ast.TypeSpecifier:
		name = dt.Token.Literal
	default:
		return beebreadFunctionBlock{}, false
	}
	if t.lookupType(name) != nil {
		return beebreadFunctionBlock{}, false
	}
	if _, isStd := standardFunctionBlocks[strings.ToUpper(name)]; isStd {
		return beebreadFunctionBlock{}, false
	}
	b, ok := beebreadFunctionBlocks[beebreadGoName(name)]
	return b, ok
}

// beebreadTypeOfValue returns the beebread function block or structured type
// of an instance or variable expression.
func (t *Transpiler) beebreadTypeOfValue(exp ast.Expression) (beebreadFunctionBlock, bool) {
	if g, ok := t.beebreadGlobal(exp); ok {
		b, ok := beebreadFunctionBlocks[beebreadGlobals[strings.TrimPrefix(g, "oscat.")]]
		return b, ok
	}
	// The result of the current function, such as STATUS_TO_ESR.TYP.
	if ident, ok := exp.(*ast.Identifier); ok && t.currentFunc != nil && ident.Value == t.currentFunc.Name.Value && t.currentFunc.ReturnType != nil {
		return t.beebreadTypeOf(t.currentFunc.ReturnType)
	}
	td := t.resolveAssignmentTargetType(exp)
	if td == nil {
		return beebreadFunctionBlock{}, false
	}
	return t.beebreadTypeOf(td.DataType)
}

// field returns the Go name and type of a field, found ignoring case.
func (b beebreadFunctionBlock) field(name string) (string, string, bool) {
	// A member named like a Go keyword, such as DEFAULT, has been renamed
	// default_; see go_names.go.
	if reservedGoNames[strings.TrimSuffix(name, "_")] {
		name = strings.TrimSuffix(name, "_")
	}
	if typ, ok := b.fields[name]; ok {
		return name, typ, true
	}
	for f, typ := range b.fields {
		if strings.EqualFold(f, name) {
			return f, typ, true
		}
	}
	return "", "", false
}

// memberName returns the Go name of the member name of the instance or
// variable exp: a beebread field's upper case name, the member's name as
// its structure or function block declares it (VAR_IN_OUTs included), a
// standard function block's upper case name, or name as it is.
func (t *Transpiler) memberName(exp ast.Expression, name string) string {
	if b, ok := t.beebreadTypeOfValue(exp); ok {
		if f, _, ok := b.field(name); ok {
			return f
		}
	}
	// A member of a structure or function block, reached however.
	if decl := t.memberDecl(exp, name); decl != nil {
		return decl.Name.Value
	}
	td := t.resolveAssignmentTargetType(exp)
	if td == nil || td.DataType == nil {
		return name
	}
	typeName := td.DataType.String()
	// A member of the program's own function block or structure is spelled
	// as declared, as names ignore case in IEC 61131-3 and not in Go.
	if declared := t.declaredMemberName(t.lookupType(typeName), name, 0); declared != "" {
		return declared
	}
	// royaljelly spells the members of the standard function blocks in
	// upper case, IN and Q, as IEC 61131-3 does; a program may write in.
	if t.lookupType(typeName) == nil {
		if _, standard := standardFunctionBlocks[strings.ToUpper(typeName)]; standard {
			return strings.ToUpper(name)
		}
	}
	return name
}

// declaredMemberName returns how a function block (or one it extends) or a
// structure declares the member name, ignoring case, or "".
func (t *Transpiler) declaredMemberName(node ast.Node, name string, depth int) string {
	if depth > 16 {
		return ""
	}
	find := func(decls ...[]*ast.VarDeclStatement) string {
		for _, block := range decls {
			for _, d := range block {
				if strings.EqualFold(d.Name.Value, name) {
					return d.Name.Value
				}
			}
		}
		return ""
	}
	switch n := node.(type) {
	case *ast.FunctionBlockDeclaration:
		if found := find(n.VarInputs, n.VarOutputs, n.VarInOuts, n.Vars); found != "" {
			return found
		}
		if n.Extends != nil {
			return t.declaredMemberName(t.lookupType(n.Extends.String()), name, depth+1)
		}
	case *ast.TypeDeclaration:
		if s, ok := n.DataType.(*ast.StructDefinition); ok {
			return find(s.Members)
		}
	}
	return ""
}

// isBeebreadInOut reports whether the argument name of a call of the
// function block instance fbExpr is a beebread VAR_IN_OUT, a pointer field.
func (t *Transpiler) isBeebreadInOut(fbExpr ast.Expression, name string) bool {
	if b, ok := t.beebreadTypeOfValue(fbExpr); ok {
		_, typ, ok := b.field(name)
		return ok && strings.HasPrefix(typ, "*")
	}
	return false
}

// builtinArgument returns the argument of a call of the builtin ADR or
// SIZEOF with one argument, if arg is one.
func builtinArgument(arg ast.Expression, builtin string) (ast.Expression, bool) {
	call, ok := arg.(*ast.CallExpression)
	if !ok || len(call.Arguments) != 1 {
		return nil, false
	}
	ident, ok := call.Function.(*ast.Identifier)
	if !ok || !strings.EqualFold(ident.Value, builtin) {
		return nil, false
	}
	return call.Arguments[0], true
}

// transpileBeebreadCall transpiles a call of a beebread function. Arguments
// are converted to the parameters' types; a VAR_IN_OUT is passed by its
// address, ADR(array) as a slice of the array and SIZEOF(array) as the
// array's size in bytes, and the parameters a call leaves out get their
// initial values.
func (t *Transpiler) transpileBeebreadCall(name string, fn stdFunction, exp *ast.CallExpression) error {
	expected := t.expectedGoType
	t.expectedGoType = ""
	defer func() { t.expectedGoType = expected }()

	if len(exp.Arguments) > len(fn.params) {
		return fmt.Errorf("the function %s takes %d arguments, got %d", name, len(fn.params), len(exp.Arguments))
	}
	convert := expected != "" && expected != fn.result && strings.HasPrefix(expected, "iec.") && strings.HasPrefix(fn.result, "iec.")
	if convert {
		t.write("%s(", expected)
	}
	t.write("%s.%s(", fn.pkg, name)
	for i, param := range fn.params {
		if i > 0 {
			t.write(", ")
		}
		if i >= len(exp.Arguments) {
			defaults := beebreadDefaults[name]
			if i >= len(defaults) {
				return fmt.Errorf("the function %s takes %d arguments, got %d", name, len(fn.params), len(exp.Arguments))
			}
			t.write("%s", defaults[i])
			continue
		}
		arg := exp.Arguments[i]
		if _, named := arg.(*ast.NamedArgument); named {
			return fmt.Errorf("the function %s takes its arguments in order, got %s", name, arg.String())
		}
		var err error
		switch {
		case strings.HasPrefix(param, "[]"):
			// ADR(array), or the array itself.
			if a, ok := builtinArgument(arg, "ADR"); ok {
				arg = a
			}
			t.write("(")
			err = t.transpileExpression(arg)
			t.write(")[:]")
		case strings.HasPrefix(param, "*"):
			t.write("&")
			err = t.transpileExpression(arg)
		case strings.HasPrefix(param, "iec."):
			if a, ok := builtinArgument(arg, "SIZEOF"); ok {
				t.write("%s(unsafe.Sizeof(", param)
				err = t.transpileExpression(a)
				t.write("))")
				break
			}
			t.write("%s(", param)
			err = t.transpileExpression(arg)
			t.write(")")
		default:
			// An array or a structure of OSCAT, whose Go type the argument has.
			err = t.transpileExpression(arg)
		}
		if err != nil {
			return err
		}
	}
	t.write(")")
	if convert {
		t.write(")")
	}
	return nil
}

// beebreadGlobal returns the Go name of an OSCAT global variable, such as
// MATH, that an identifier names, unless the program declares the name.
func (t *Transpiler) beebreadGlobal(exp ast.Expression) (string, bool) {
	ident, ok := exp.(*ast.Identifier)
	if !ok {
		return "", false
	}
	name := ident.Value
	if t.localVars[name] || t.varInfo[name] != nil || t.globalVars[name] || t.tempVars[name] ||
		t.inOutVars[name] || t.accessVars[name] || t.lookupType(name) != nil {
		return "", false
	}
	up := strings.ToUpper(name)
	if _, ok := beebreadGlobals[up]; !ok {
		return "", false
	}
	return "oscat." + up, true
}

// beebreadLowerBound returns the lower bound OSCAT declares for the index of
// exp, an index into an array field of a beebread block, structure or
// global, and whether it knows one.
func (t *Transpiler) beebreadLowerBound(exp ast.Expression) (int64, bool) {
	dim := 0
	for {
		ix, ok := exp.(*ast.IndexExpression)
		if !ok {
			break
		}
		exp = ix.Left
		dim++
	}
	m, ok := exp.(*ast.MemberAccessExpression)
	if !ok {
		return 0, false
	}
	b, ok := t.beebreadTypeOfValue(m.Struct)
	if !ok {
		return 0, false
	}
	f, _, ok := b.field(m.Member.Value)
	if !ok {
		return 0, false
	}
	lows := b.lows[f]
	if dim >= len(lows) {
		return 0, true
	}
	return lows[dim], true
}
