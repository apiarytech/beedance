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

// This file generates the starting values of variables: declared initial
// values, structure and function block initializers such as `(A := 1)`, and
// the IEC 61131-3 default of each type where Go's zero value differs from it
// (structure members with initial values, subranges, enumerations and aliases
// with an initial value, and arrays, which are Go slices).
//
// Names the generated code introduces start with two underscores, which IEC
// 61131-3 identifiers cannot contain, so they never hide a user's variable.

import (
	"beedance/ast"
	"beedance/evaluator"
	"beedance/object"
	"bytes"
	"fmt"
	"strings"
	"time"
)

// capture runs f and returns what it wrote instead of writing it.
func (t *Transpiler) capture(f func() error) (string, error) {
	original := t.w
	var buf bytes.Buffer
	t.w = &buf
	err := f()
	t.w = original
	return buf.String(), err
}

// expressionString transpiles an expression in the current context.
func (t *Transpiler) expressionString(exp ast.Expression) (string, error) {
	return t.capture(func() error { return t.transpileExpression(exp) })
}

// declaredTypeName returns the name of a named type, or "" for any other type
// (such as an array definition).
func declaredTypeName(dataType ast.Expression) string {
	switch dt := dataType.(type) {
	case *ast.TypeSpecifier:
		if dt != nil {
			return dt.Token.Literal
		}
	case *ast.Identifier:
		return dt.Value
	}
	return ""
}

// lookupType finds a user-defined type, FUNCTION_BLOCK or FUNCTION by name,
// ignoring case.
func (t *Transpiler) lookupType(name string) ast.Node {
	if name == "" {
		return nil
	}
	if node, ok := t.typeInfo[name]; ok {
		return node
	}
	for key, node := range t.typeInfo {
		if strings.EqualFold(key, name) {
			return node
		}
	}
	return nil
}

// lookupTypeDeclaration finds a TYPE declaration by name.
func (t *Transpiler) lookupTypeDeclaration(dataType ast.Expression) *ast.TypeDeclaration {
	td, _ := t.lookupType(declaredTypeName(dataType)).(*ast.TypeDeclaration)
	return td
}

// lookupFunctionBlock finds a user-defined FUNCTION_BLOCK by type.
func (t *Transpiler) lookupFunctionBlock(dataType ast.Expression) *ast.FunctionBlockDeclaration {
	fb, _ := t.lookupType(declaredTypeName(dataType)).(*ast.FunctionBlockDeclaration)
	return fb
}

// constantInteger evaluates a constant integer expression such as an array
// bound.
func constantInteger(exp ast.Expression) (int64, bool) {
	switch e := exp.(type) {
	case *ast.IntegerLiteral:
		return e.Value, true
	case *ast.UnsignedIntegerLiteral:
		return int64(e.Value), true
	case *ast.PrefixExpression:
		n, ok := constantInteger(e.Right)
		switch e.Operator {
		case "-":
			return -n, ok
		case "+":
			return n, ok
		}
	case *ast.InfixExpression:
		l, okL := constantInteger(e.Left)
		r, okR := constantInteger(e.Right)
		if !okL || !okR {
			return 0, false
		}
		switch e.Operator {
		case "+":
			return l + r, true
		case "-":
			return l - r, true
		case "*":
			return l * r, true
		}
	}
	return 0, false
}

// initialValue returns the Go expression for the starting value of a variable
// of dataType whose declared initial value is value (nil for none), or "" when
// Go's zero value of the variable's type is already the IEC 61131-3 default.
// A function block's starting value is set by its Init method and
// initializer instead; see functionBlockInit.
func (t *Transpiler) initialValue(dataType, value ast.Expression) (string, error) {
	return t.initialValueVisiting(dataType, value, map[ast.Node]bool{})
}

func (t *Transpiler) initialValueVisiting(dataType, value ast.Expression, visiting map[ast.Node]bool) (string, error) {
	if _, isMacro := value.(*ast.MacroLiteral); isMacro {
		return "", nil
	}
	if def, ok := dataType.(*ast.ArrayDefinition); ok {
		return t.arrayValue(def, value, visiting)
	}
	if def, ok := dataType.(*ast.StructDefinition); ok {
		// A structure declared in place: a Go anonymous struct.
		lit, isLit := value.(*ast.StructLiteral)
		if value != nil && !isLit {
			return t.expressionString(value)
		}
		td := &ast.TypeDeclaration{Name: &ast.Identifier{Value: t.mapIecTypeToGo(def)}, DataType: def}
		return t.structValue(td, def, lit, visiting)
	}
	if td := t.lookupTypeDeclaration(dataType); td != nil {
		return t.typeDeclarationValue(td, value, visiting)
	}
	if t.lookupFunctionBlock(dataType) != nil {
		return "", nil
	}
	if lit, ok := value.(*ast.StructLiteral); ok {
		return "", fmt.Errorf("structure initializer %s needs a structure or function block type, but '%s' is neither", lit.String(), dataType.String())
	}
	if value == nil {
		return "", nil
	}
	if b, ok := boolLiteral(value); ok && t.mapIecTypeToGo(dataType) == "iec.BOOL" {
		return b, nil
	}
	return t.expressionString(value)
}

// typeDeclarationValue returns the starting value of a variable of a
// user-defined type: its own initial value, else the type's, else the
// type's default.
func (t *Transpiler) typeDeclarationValue(td *ast.TypeDeclaration, value ast.Expression, visiting map[ast.Node]bool) (string, error) {
	if visiting[td] {
		return "", fmt.Errorf("type '%s' contains itself", td.Name.Value)
	}
	visiting[td] = true
	defer delete(visiting, td)

	switch def := td.DataType.(type) {
	case *ast.StructDefinition:
		lit, isLit := value.(*ast.StructLiteral)
		if value != nil && !isLit {
			return t.expressionString(value) // Another value, e.g. a structure variable.
		}
		return t.structValue(td, def, lit, visiting)
	case *ast.EnumDefinition:
		if value == nil {
			value = td.InitialValue
		}
		if ident, ok := value.(*ast.Identifier); ok {
			// A bare value name, unless it names a variable.
			for _, v := range def.Values {
				if strings.EqualFold(v.Value, ident.Value) {
					return fmt.Sprintf("%s_%s", td.Name.Value, v.Value), nil
				}
			}
		}
		if value == nil {
			return "", nil // The first value, which is Go's zero.
		}
		return t.expressionString(value)
	case *ast.ArrayDefinition:
		if value == nil {
			value = td.InitialValue
		}
		return t.arrayValue(def, value, visiting)
	}

	// A subrange or an alias of another type.
	if value == nil {
		value = td.InitialValue
	}
	if value != nil {
		if _, isLit := value.(*ast.StructLiteral); !isLit {
			return t.expressionString(value)
		}
	}
	if subrange, ok := td.Subrange.(*ast.InfixExpression); ok && value == nil {
		if low, ok := constantInteger(subrange.Left); ok && low != 0 {
			return fmt.Sprintf("%d", low), nil // A subrange starts at its lower limit.
		}
		return "", nil
	}
	return t.initialValueVisiting(td.DataType, value, visiting)
}

// structValue returns a structure value as a Go composite literal. Each member
// takes its value from init, if given there, else its declared initial value,
// else its type's default. Without init, the type's own initial value (a
// structure initializer) applies. It returns "" when every member takes Go's
// zero value and there is no initializer.
func (t *Transpiler) structValue(td *ast.TypeDeclaration, def *ast.StructDefinition, init *ast.StructLiteral, visiting map[ast.Node]bool) (string, error) {
	if init == nil {
		init, _ = td.InitialValue.(*ast.StructLiteral)
	}
	overrides, err := initializerValues(init)
	if err != nil {
		return "", err
	}
	for name := range overrides {
		found := false
		for _, m := range def.Members {
			found = found || strings.EqualFold(m.Name.Value, name)
		}
		if !found {
			return "", fmt.Errorf("structure '%s' has no member '%s'", td.Name.Value, name)
		}
	}

	fields := []string{}
	for _, m := range def.Members {
		value := m.Value
		if override, ok := lookupExpressionFold(overrides, m.Name.Value); ok {
			value = override
		}
		memberValue, err := t.initialValueVisiting(m.DataType, value, visiting)
		if err != nil {
			return "", err
		}
		if memberValue != "" {
			fields = append(fields, fmt.Sprintf("%s: %s", m.Name.Value, memberValue))
		}
	}
	if len(fields) == 0 && init == nil {
		return "", nil
	}
	return fmt.Sprintf("%s{%s}", td.Name.Value, strings.Join(fields, ", ")), nil
}

// initializerValues returns the `name := value` pairs of a structure or
// function block initializer.
func initializerValues(init *ast.StructLiteral) (map[string]ast.Expression, error) {
	out := map[string]ast.Expression{}
	if init == nil {
		return out, nil
	}
	for _, e := range init.Initializers {
		named, ok := e.(*ast.NamedArgument)
		if !ok {
			return nil, fmt.Errorf("an initializer must list members as name := value, got %s", e.String())
		}
		out[named.Name.Value] = named.Value
	}
	return out, nil
}

// lookupExpressionFold finds a key in m, ignoring case.
func lookupExpressionFold(m map[string]ast.Expression, key string) (ast.Expression, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// arrayLengths returns the length of each dimension of an array type, or
// false when a bound is not constant.
func arrayLengths(def *ast.ArrayDefinition) ([]int, bool) {
	lengths := []int{}
	for _, rng := range def.Ranges {
		infix, ok := rng.(*ast.InfixExpression)
		if !ok || infix.Operator != ".." {
			return nil, false
		}
		low, okLow := constantInteger(infix.Left)
		high, okHigh := constantInteger(infix.Right)
		if !okLow || !okHigh || high < low {
			return nil, false
		}
		lengths = append(lengths, int(high-low)+1)
	}
	return lengths, true
}

// arrayValue returns an array value as a Go array of the declared length.
// Without a value, every element takes the element type's default. A literal
// may list fewer elements than the array holds; the rest take the default. A
// multi-dimensional literal may be nested ([[1, 2], [3, 4]]) or flat
// ([1, 2, 3, 4]). An array whose bounds are not constant keeps its value as
// written.
func (t *Transpiler) arrayValue(def *ast.ArrayDefinition, value ast.Expression, visiting map[ast.Node]bool) (string, error) {
	lengths, ok := arrayLengths(def)
	if !ok {
		return t.valueAsWritten(value)
	}

	var elementType ast.Expression
	if def.DataType != nil {
		elementType = def.DataType
	}
	elementDefault, err := t.initialValueVisiting(elementType, nil, visiting)
	if err != nil {
		return "", err
	}
	a := &arrayBuilder{t: t, elementType: elementType, elementDefault: elementDefault, visiting: visiting}

	if value == nil {
		return a.build(t.mapIecTypeToGo(def), lengths, nil, false)
	}
	literal, ok := value.(*ast.ArrayLiteral)
	if !ok {
		return t.expressionString(value) // Another array value.
	}
	return a.build(t.mapIecTypeToGo(def), lengths, literal.Elements, true)
}

// valueAsWritten transpiles value, or returns "" for none.
func (t *Transpiler) valueAsWritten(value ast.Expression) (string, error) {
	if value == nil {
		return "", nil
	}
	return t.expressionString(value)
}

// arrayBuilder builds the Go expression for an array value.
type arrayBuilder struct {
	t              *Transpiler
	elementType    ast.Expression
	elementDefault string
	visiting       map[ast.Node]bool
}

// build returns a value of goType (such as [2][3]iec.INT) with the given
// dimensions. When given is true, elements are the literal's elements for
// this dimension.
func (a *arrayBuilder) build(goType string, dims []int, elements []ast.Expression, given bool) (string, error) {
	n := dims[0]
	subType := goType[strings.Index(goType, "]")+1:]

	var subDefault string
	if len(dims) == 1 {
		subDefault = a.elementDefault
	} else {
		var err error
		if subDefault, err = a.build(subType, dims[1:], nil, false); err != nil {
			return "", err
		}
	}
	if !given {
		if subDefault == "" || subDefault == subType+"{}" {
			return goType + "{}", nil
		}
		return fmt.Sprintf("func() %s { var __a %s; for __i := range __a { __a[__i] = %s }; return __a }()", goType, goType, subDefault), nil
	}

	elements, err := expandRepetitions(elements)
	if err != nil {
		return "", err
	}
	values := []string{}
	if len(dims) == 1 {
		for _, el := range elements {
			v, err := a.t.initialValueVisiting(a.elementType, el, a.visiting)
			if err != nil {
				return "", err
			}
			if v == "" {
				v = zeroValue(subType)
			}
			values = append(values, v)
		}
	} else if allArrayLiterals(elements) {
		for _, el := range elements {
			v, err := a.build(subType, dims[1:], el.(*ast.ArrayLiteral).Elements, true)
			if err != nil {
				return "", err
			}
			values = append(values, v)
		}
	} else {
		// A flat literal fills the array in row-major order.
		rowSize := 1
		for _, d := range dims[1:] {
			rowSize *= d
		}
		for start := 0; start < len(elements); start += rowSize {
			end := min(start+rowSize, len(elements))
			v, err := a.build(subType, dims[1:], elements[start:end], true)
			if err != nil {
				return "", err
			}
			values = append(values, v)
		}
	}
	if len(values) > n {
		return "", fmt.Errorf("array initial value has %d elements, but the array holds %d", len(values), n)
	}

	// Elements the literal leaves out take the zero value, unless the
	// element type's default is another value.
	literal := fmt.Sprintf("%s{%s}", goType, strings.Join(values, ", "))
	if len(values) == n || subDefault == "" || subDefault == subType+"{}" {
		return literal, nil
	}
	return fmt.Sprintf("func() %s { __a := %s; for __i := %d; __i < %d; __i++ { __a[__i] = %s }; return __a }()",
		goType, literal, len(values), n, subDefault), nil
}

// expandRepetitions expands repeated elements such as 3(0) of an array
// literal.
func expandRepetitions(elements []ast.Expression) ([]ast.Expression, error) {
	out := []ast.Expression{}
	for _, el := range elements {
		rep, ok := el.(*ast.ArrayRepetition)
		if !ok {
			out = append(out, el)
			continue
		}
		count, ok := constantInteger(rep.Factor)
		if !ok || count < 0 {
			return nil, fmt.Errorf("array repetition count must be a constant, got %s", rep.Factor.String())
		}
		for i := int64(0); i < count; i++ {
			out = append(out, rep.Elements...)
		}
	}
	return out, nil
}

func allArrayLiterals(elements []ast.Expression) bool {
	for _, el := range elements {
		if _, ok := el.(*ast.ArrayLiteral); !ok {
			return false
		}
	}
	return len(elements) > 0
}

// zeroValue returns a Go expression for the zero value of goType.
func zeroValue(goType string) string {
	switch goType {
	case "iec.BOOL":
		return "false"
	case "iec.STRING", "iec.WSTRING":
		return `""`
	case "iec.SINT", "iec.INT", "iec.DINT", "iec.LINT", "iec.USINT", "iec.UINT", "iec.UDINT", "iec.ULINT",
		"iec.REAL", "iec.LREAL", "iec.BYTE", "iec.WORD", "iec.DWORD", "iec.LWORD", "iec.TIME", "iec.LTIME":
		return "0"
	}
	if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "*") {
		return "nil"
	}
	return fmt.Sprintf("*new(%s)", goType)
}

// defaultArgument returns the value an omitted input of a call takes: its
// initial value, else its type's default. It is transpiled without a
// receiver, as it belongs to the callee's declaration.
func (t *Transpiler) defaultArgument(input *ast.VarDeclStatement) (string, error) {
	originalReceiver, originalLocals := t.programVarName, t.localVars
	t.programVarName, t.localVars = "", nil
	defer func() { t.programVarName, t.localVars = originalReceiver, originalLocals }()

	v, err := t.initialValue(input.DataType, input.Value)
	if err != nil || v != "" {
		return v, err
	}
	return zeroValue(t.mapIecTypeToGo(input.DataType)), nil
}

// functionBlockInit returns the statements that set the starting value of a
// function block instance named target (such as `instance.timer`): a call to
// its Init method, if it has one, then the variables set by its initializer.
func (t *Transpiler) functionBlockInit(target string, dataType, value ast.Expression) ([]string, error) {
	fb := t.lookupFunctionBlock(dataType)
	lit, isLit := value.(*ast.StructLiteral)
	if fb == nil && !t.isFunctionBlockType(dataType) {
		return nil, nil
	}
	stmts := []string{}
	if fb != nil && t.functionBlockNeedsInit(fb) {
		stmts = append(stmts, target+".Init()")
	}
	// A function block of OSCAT BASIC sets its initial values with INIT.
	if b, ok := t.beebreadTypeOf(dataType); ok && b.isFB {
		stmts = append(stmts, target+".INIT()")
	}
	if !isLit {
		return stmts, nil
	}
	overrides, err := initializerValues(lit)
	if err != nil {
		return nil, err
	}
	for _, e := range lit.Initializers {
		name := e.(*ast.NamedArgument).Name.Value
		fieldName := name
		if b, ok := t.beebreadTypeOf(dataType); ok {
			f, _, found := b.field(name)
			if !found {
				return nil, fmt.Errorf("function block '%s' has no input '%s'", dataType.String(), name)
			}
			fieldName = f
		}
		if fb != nil {
			decl := t.findFunctionBlockVar(fb, name)
			if decl == nil {
				return nil, fmt.Errorf("function block '%s' has no variable '%s'", fb.Name.Value, name)
			}
			fieldName = decl.Name.Value
		}
		v, err := t.expressionString(overrides[name])
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, fmt.Sprintf("%s.%s = %s", target, fieldName, v))
	}
	return stmts, nil
}

// findFunctionBlockVar finds a variable of a function block or its ancestors,
// ignoring case.
func (t *Transpiler) findFunctionBlockVar(fb *ast.FunctionBlockDeclaration, name string) *ast.VarDeclStatement {
	for fb != nil {
		for _, block := range [][]*ast.VarDeclStatement{fb.VarInputs, fb.VarOutputs, fb.Vars} {
			for _, decl := range block {
				if strings.EqualFold(decl.Name.Value, name) {
					return decl
				}
			}
		}
		if fb.Extends == nil {
			return nil
		}
		fb = t.getFunctionBlockDefinitionFromTypeInfo(fb.Extends.String())
	}
	return nil
}

// functionBlockNeedsInit reports whether a function block, or an ancestor,
// has an Init method.
func (t *Transpiler) functionBlockNeedsInit(fb *ast.FunctionBlockDeclaration) bool {
	if fb == nil || t.initVisiting[fb] {
		return false
	}
	if stmts, _ := t.functionBlockInitStatements(fb, "x"); len(stmts) > 0 {
		return true
	}
	if fb.Extends != nil {
		return t.functionBlockNeedsInit(t.getFunctionBlockDefinitionFromTypeInfo(fb.Extends.String()))
	}
	return false
}

// initializedVars returns the variables of a function block whose starting
// value is set by its Init method.
func initializedVars(fb *ast.FunctionBlockDeclaration) []*ast.VarDeclStatement {
	out := []*ast.VarDeclStatement{}
	for _, block := range [][]*ast.VarDeclStatement{fb.VarInputs, fb.VarOutputs, fb.Vars} {
		for _, decl := range block {
			if decl.Location != nil {
				continue // Set by the I/O image.
			}
			if typeSpec, ok := decl.DataType.(*ast.TypeSpecifier); ok && typeSpec != nil && strings.EqualFold(typeSpec.Token.Literal, "FUNCTION") {
				continue
			}
			if _, isRef := decl.DataType.(*ast.ReferenceType); isRef {
				continue
			}
			out = append(out, decl)
		}
	}
	return out
}

// functionBlockInitStatements returns the statements of a function block's
// own Init method, with receiver as the receiver name.
func (t *Transpiler) functionBlockInitStatements(fb *ast.FunctionBlockDeclaration, receiver string) ([]string, error) {
	if t.initVisiting == nil {
		t.initVisiting = map[*ast.FunctionBlockDeclaration]bool{}
	}
	t.initVisiting[fb] = true
	defer delete(t.initVisiting, fb)

	originalReceiver, originalLocals := t.programVarName, t.localVars
	t.programVarName, t.localVars = receiver, nil
	defer func() { t.programVarName, t.localVars = originalReceiver, originalLocals }()

	stmts := []string{}
	for _, decl := range initializedVars(fb) {
		target := receiver + "." + decl.Name.Value
		if nested := t.lookupFunctionBlock(decl.DataType); nested != nil || t.isFunctionBlockType(decl.DataType) {
			if nested != nil && t.initVisiting[nested] {
				return nil, fmt.Errorf("function block '%s' contains itself", nested.Name.Value)
			}
			init, err := t.functionBlockInit(target, decl.DataType, decl.Value)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, init...)
			continue
		}
		v, err := t.initialValue(decl.DataType, decl.Value)
		if err != nil {
			return nil, err
		}
		if v != "" {
			stmts = append(stmts, fmt.Sprintf("%s = %s", target, v))
		}
	}
	return stmts, nil
}

// transpileFunctionBlockInit writes the Init method of a function block that
// has variables to set: it sets each variable to its initial value, or its
// type's default where that differs from Go's zero value, after calling the
// parent's Init.
func (t *Transpiler) transpileFunctionBlockInit(fb *ast.FunctionBlockDeclaration, receiver string) error {
	stmts, err := t.functionBlockInitStatements(fb, receiver)
	if err != nil {
		return err
	}
	if len(stmts) == 0 {
		return nil
	}
	t.write("// Init sets the starting values of a %s.\n", fb.Name.Value)
	t.write("func (%s *%s) Init() {\n", receiver, fb.Name.Value)
	if fb.Extends != nil {
		if parent := t.getFunctionBlockDefinitionFromTypeInfo(fb.Extends.String()); t.functionBlockNeedsInit(parent) {
			t.write("\t%s.%s.Init()\n", receiver, fb.Extends.String())
		}
	}
	for _, stmt := range stmts {
		t.write("\t%s\n", stmt)
	}
	t.write("}\n\n")
	return nil
}

// transpileProgramVarInit writes, in a program's factory, the statements that
// set the starting value of one of its variables.
func (t *Transpiler) transpileProgramVarInit(decl *ast.VarDeclStatement) error {
	if _, isMacro := decl.Value.(*ast.MacroLiteral); isMacro {
		return nil
	}
	target := "instance." + decl.Name.Value
	if t.isFunctionBlockType(decl.DataType) {
		init, err := t.functionBlockInit(target, decl.DataType, decl.Value)
		if err != nil {
			return err
		}
		for _, stmt := range init {
			t.write("\t%s\n", stmt)
		}
		// A function block's EN input is TRUE by default. Some of
		// royaljelly's standard function blocks have no EN.
		if std, isStd := t.standardFunctionBlockOf(decl.DataType); !isStd || std.hasEN {
			t.write("\t%s.EN = true\n", target)
		}
		return nil
	}
	if decl.Location != nil && decl.Value == nil {
		return nil // Linked to the I/O image.
	}
	if typeSpec, ok := decl.DataType.(*ast.TypeSpecifier); ok && typeSpec != nil && strings.EqualFold(typeSpec.Token.Literal, "FUNCTION") {
		return t.valueStatement(target, decl.Value)
	}
	if _, isRef := decl.DataType.(*ast.ReferenceType); isRef {
		return t.valueStatement(target, decl.Value)
	}
	v, err := t.initialValue(decl.DataType, decl.Value)
	if err != nil {
		return fmt.Errorf("initial value of '%s': %w", decl.Name.Value, err)
	}
	if v != "" {
		t.write("\t%s = %s\n", target, v)
	}
	return nil
}

// valueStatement writes `target = value` for a value as written, if any.
func (t *Transpiler) valueStatement(target string, value ast.Expression) error {
	v, err := t.valueAsWritten(value)
	if err != nil || v == "" {
		return err
	}
	t.write("\t%s = %s\n", target, v)
	return nil
}

// transpileLocalVar writes a local variable of a function, method or Logic
// method (VAR_TEMP) with its starting value; locals start afresh on every
// call.
func (t *Transpiler) transpileLocalVar(decl *ast.VarDeclStatement) error {
	if _, isMacro := decl.Value.(*ast.MacroLiteral); isMacro {
		return nil
	}
	t.write("\tvar %s %s", decl.Name.Value, t.mapIecTypeToGo(decl.DataType))
	init, err := t.functionBlockInit(decl.Name.Value, decl.DataType, decl.Value)
	if err != nil {
		return err
	}
	if init == nil {
		v, err := t.initialValue(decl.DataType, decl.Value)
		if err != nil {
			return err
		}
		if v != "" {
			t.write(" = %s", v)
		}
	}
	t.write("\n")
	for _, stmt := range init {
		t.write("\t%s\n", stmt)
	}
	return nil
}

// transpileResultDeclarations writes a function's, method's or getter's named
// Go results, as in `(F iec.INT, out1 iec.BOOL)`. The result named after the
// POU holds its return value.
func (t *Transpiler) transpileResultDeclarations(name string, returnType ast.Expression, outputs []*ast.VarDeclStatement) bool {
	results := []string{}
	if returnType != nil {
		results = append(results, fmt.Sprintf("%s %s", name, t.mapIecTypeToGo(returnType)))
	}
	for _, p := range outputs {
		results = append(results, fmt.Sprintf("%s %s", p.Name.Value, t.mapIecTypeToGo(p.DataType)))
	}
	if len(results) > 0 {
		t.write("(%s) ", strings.Join(results, ", "))
	}
	return len(results) > 0
}

// transpileResultValues sets a function's, method's or getter's results to
// their starting values, where Go's zero value is not already right.
func (t *Transpiler) transpileResultValues(name *ast.Identifier, returnType ast.Expression, outputs []*ast.VarDeclStatement) error {
	if returnType != nil {
		v, err := t.initialValue(returnType, nil)
		if err != nil {
			return err
		}
		if v != "" {
			t.write("\t%s = %s\n", name.Value, v)
		}
	}
	for _, p := range outputs {
		v, err := t.initialValue(p.DataType, p.Value)
		if err != nil {
			return err
		}
		if v != "" {
			t.write("\t%s = %s\n", p.Name.Value, v)
		}
	}
	return nil
}

// withLocals runs f with the given names treated as locals of the current
// function or method, so they are written without the receiver.
func (t *Transpiler) withLocals(names []string, f func() error) error {
	original := t.localVars
	t.localVars = map[string]bool{}
	for _, name := range names {
		t.localVars[name] = true
	}
	defer func() { t.localVars = original }()
	return f()
}

// declNames returns the names of the declarations.
func declNames(blocks ...[]*ast.VarDeclStatement) []string {
	names := []string{}
	for _, block := range blocks {
		for _, decl := range block {
			names = append(names, decl.Name.Value)
		}
	}
	return names
}

// callSignature describes the parameters of a user-defined function or
// method, for transpiling calls to it.
type callSignature struct {
	name       string
	inputs     []*ast.VarDeclStatement
	inOuts     []*ast.VarDeclStatement
	outputs    []*ast.VarDeclStatement
	returnType ast.Expression
}

// lookupCallSignature returns the signature of a call's callee when it is a
// user-defined function, or a method of a function block instance.
func (t *Transpiler) lookupCallSignature(callee ast.Expression) (*callSignature, bool) {
	// A method of the function block being transpiled, called by name or
	// through THIS.
	if method := t.ownMethod(callee); method != nil {
		sig := &callSignature{name: method.Name.Value, inputs: method.VarInputs, inOuts: method.VarInOuts, outputs: method.VarOutputs}
		if method.ReturnType != nil {
			sig.returnType = method.ReturnType
		}
		return sig, true
	}
	switch fn := callee.(type) {
	case *ast.Identifier:
		fd, ok := t.lookupType(fn.Value).(*ast.FunctionDeclaration)
		if !ok {
			return nil, false
		}
		sig := &callSignature{name: fd.Name.Value, inputs: fd.VarInputs, inOuts: fd.VarInOuts, outputs: fd.VarOutputs}
		if fd.ReturnType != nil {
			sig.returnType = fd.ReturnType
		}
		return sig, true
	case *ast.MemberAccessExpression:
		typeDecl := t.resolveAssignmentTargetType(fn.Struct)
		if typeDecl == nil || typeDecl.DataType == nil {
			return nil, false
		}
		fb := t.lookupFunctionBlock(typeDecl.DataType)
		method := t.findMethodOnFBChain(fb, fn.Member.Value)
		if method == nil {
			return nil, false
		}
		sig := &callSignature{name: method.Name.Value, inputs: method.VarInputs, inOuts: method.VarInOuts, outputs: method.VarOutputs}
		if method.ReturnType != nil {
			sig.returnType = method.ReturnType
		}
		return sig, true
	}
	return nil, false
}

// findMethodOnFBChain finds a method of a function block or its ancestors.
func (t *Transpiler) findMethodOnFBChain(fb *ast.FunctionBlockDeclaration, name string) *ast.MethodImplementation {
	for fb != nil {
		if body, ok := fb.Body.(*ast.BlockStatement); ok {
			for _, stmt := range body.Statements {
				if method, ok := stmt.(*ast.MethodImplementation); ok && strings.EqualFold(method.Name.Value, name) {
					return method
				}
			}
		}
		if fb.Extends == nil {
			return nil
		}
		fb = t.getFunctionBlockDefinitionFromTypeInfo(fb.Extends.String())
	}
	return nil
}

// transpileUserCall transpiles a call to a user-defined function or method.
// Go has no named or optional parameters, so the arguments are put in
// declaration order: a formal call (`F(b := 2)`, or `F()`) may omit inputs,
// which take their defaults; a non-formal call (`F(1, 2)`) supplies every
// input and in-out. The VAR_OUTPUTs a call binds (`o => x`) are assigned from
// the Go results after the call.
func (t *Transpiler) transpileUserCall(exp *ast.CallExpression, sig *callSignature) error {
	params := append(append([]*ast.VarDeclStatement{}, sig.inputs...), sig.inOuts...)
	args := make([]string, len(params))
	positional, named := 0, 0
	outputTargets := make([]ast.Expression, len(sig.outputs))

	setArg := func(i int, value ast.Expression) error {
		// An input of another numeric type is converted to the parameter's.
		v, err := t.expressionString(value)
		if i < len(sig.inputs) {
			v, err = t.capture(func() error {
				return t.transpileConverted(value, iecOnly(t.mapIecTypeToGo(params[i].DataType)))
			})
		}
		// An array literal takes its type from the parameter.
		if lit, isLiteral := value.(*ast.ArrayLiteral); isLiteral && i < len(sig.inputs) {
			if def := t.arrayDefinitionOf(params[i].DataType); def != nil {
				v, err = t.arrayValue(def, lit, map[ast.Node]bool{})
			}
		}
		if err != nil {
			return err
		}
		if i >= len(sig.inputs) {
			v = "&" + v // A VAR_IN_OUT is passed by reference.
		}
		args[i] = v
		return nil
	}
	for _, arg := range exp.Arguments {
		switch a := arg.(type) {
		case *ast.NamedArgument:
			named++
			if strings.EqualFold(a.Name.Value, "EN") {
				continue
			}
			i := indexOfDecl(params, a.Name.Value)
			if i < 0 {
				return fmt.Errorf("'%s' has no input '%s'", sig.name, a.Name.Value)
			}
			if err := setArg(i, a.Value); err != nil {
				return err
			}
		case *ast.OutputArgument:
			if strings.EqualFold(a.Source.Value, "ENO") {
				continue
			}
			i := indexOfDecl(sig.outputs, a.Source.Value)
			if i < 0 {
				return fmt.Errorf("'%s' has no output '%s'", sig.name, a.Source.Value)
			}
			outputTargets[i] = a.Target
		default:
			if positional >= len(params) {
				return fmt.Errorf("'%s' takes %d arguments, got more", sig.name, len(params))
			}
			if err := setArg(positional, arg); err != nil {
				return err
			}
			positional++
		}
	}
	if positional > 0 && named == 0 && positional < len(params) {
		return fmt.Errorf("a non-formal call to '%s' must supply all %d inputs, got %d", sig.name, len(params), positional)
	}
	for i, p := range params {
		if args[i] != "" {
			continue
		}
		if i >= len(sig.inputs) {
			return fmt.Errorf("the call to '%s' must supply its VAR_IN_OUT '%s'", sig.name, p.Name.Value)
		}
		v, err := t.defaultArgument(p)
		if err != nil {
			return err
		}
		args[i] = v
	}

	callee, err := t.calleeString(exp.Function)
	if err != nil {
		return err
	}
	call := fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", "))
	if len(sig.outputs) == 0 {
		t.write("%s", call)
		return nil
	}

	// A call with VAR_OUTPUTs returns several Go values, so it is wrapped in a
	// function literal that assigns the bound outputs and yields the result.
	results := []string{}
	if sig.returnType != nil {
		results = append(results, "__r")
	}
	assigns := []string{}
	for i, target := range outputTargets {
		if target == nil {
			results = append(results, "_")
			continue
		}
		v, err := t.expressionString(target)
		if err != nil {
			return err
		}
		results = append(results, fmt.Sprintf("__o%d", i))
		assigns = append(assigns, fmt.Sprintf("%s = __o%d", v, i))
	}
	if sig.returnType == nil && len(assigns) == 0 {
		t.write("%s", call)
		return nil
	}
	body := fmt.Sprintf("%s := %s", strings.Join(results, ", "), call)
	for _, a := range assigns {
		body += "; " + a
	}
	if sig.returnType == nil {
		t.write("func() { %s }()", body)
	} else {
		t.write("func() %s { %s; return __r }()", t.mapIecTypeToGo(sig.returnType), body)
	}
	return nil
}

// calleeString transpiles the function part of a call: a function name as
// written, or a method with its instance.
func (t *Transpiler) calleeString(fn ast.Expression) (string, error) {
	if method := t.ownMethod(fn); method != nil {
		return t.programVarName + "." + method.Name.Value, nil
	}
	if ident, ok := fn.(*ast.Identifier); ok {
		if node, ok := t.lookupType(ident.Value).(*ast.FunctionDeclaration); ok {
			return node.Name.Value, nil
		}
		return ident.Value, nil
	}
	return t.expressionString(fn)
}

// indexOfDecl returns the position of the declaration named name, ignoring
// case, or -1.
func indexOfDecl(decls []*ast.VarDeclStatement, name string) int {
	for i, decl := range decls {
		if strings.EqualFold(decl.Name.Value, name) {
			return i
		}
	}
	return -1
}

// endsWithReturn reports whether a body's last statement is a RETURN, after
// which Go needs no further return.
func endsWithReturn(body ast.Node) bool {
	block, ok := body.(*ast.BlockStatement)
	if !ok || block == nil || len(block.Statements) == 0 {
		return false
	}
	_, isReturn := block.Statements[len(block.Statements)-1].(*ast.ReturnStatement)
	return isReturn
}

// timeDateLiteral returns the Go expression for a TIME, DATE, TIME_OF_DAY or
// DATE_AND_TIME literal, parsed at transpile time as the evaluator parses it,
// e.g. `iec.TIME(5000000000)` for T#5s.
func timeDateLiteral(value, typeName string) (string, error) {
	date := func(goType string, tm time.Time) string {
		return fmt.Sprintf("%s(time.Date(%d, %d, %d, %d, %d, %d, %d, time.UTC))", goType,
			tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond())
	}
	switch v := evaluator.TimeDateLiteral(value, typeName).(type) {
	case *object.Time:
		return fmt.Sprintf("iec.TIME(%d)", int64(v.Value)), nil
	case *object.Date:
		return date("iec.DATE", v.Value), nil
	case *object.TimeOfDay:
		return date("iec.TOD", v.Value), nil
	case *object.DateAndTime:
		return date("iec.DT", v.Value), nil
	case *object.Error:
		return "", fmt.Errorf("%s#%s: %s", typeName, value, strings.TrimPrefix(v.Message, "BUILTIN ERROR: "))
	}
	return "", fmt.Errorf("%s#%s is not a time or date literal", typeName, value)
}

// writeTimeDate writes a time or date literal; see timeDateLiteral.
func (t *Transpiler) writeTimeDate(value, typeName string) error {
	v, err := timeDateLiteral(value, typeName)
	if err != nil {
		return err
	}
	t.write("%s", v)
	return nil
}

// bitStringGoType returns the royaljelly type of a bit string of the given
// width.
func bitStringGoType(width int) string {
	switch width {
	case 8:
		return "iec.BYTE"
	case 16:
		return "iec.WORD"
	case 32:
		return "iec.DWORD"
	}
	return "iec.LWORD"
}

// withVarInfo records the declared types of the given variables, on top of
// the enclosing scope's when inherit is true, and returns a function that
// restores the previous scope.
func (t *Transpiler) withVarInfo(inherit bool, blocks ...[]*ast.VarDeclStatement) func() {
	original, originalArrays := t.varInfo, t.arrayDecls
	t.varInfo = map[string]*ast.TypeDeclaration{}
	t.arrayDecls = map[string]*ast.ArrayDefinition{}
	if inherit {
		for name, td := range original {
			t.varInfo[name] = td
		}
		for name, def := range originalArrays {
			t.arrayDecls[name] = def
		}
	}
	t.buildVarInfo(blocks...)
	return func() { t.varInfo, t.arrayDecls = original, originalArrays }
}

// arrayDefinitionOf returns the array type of a declared type, looking
// through a named TYPE, or nil.
func (t *Transpiler) arrayDefinitionOf(dataType ast.Expression) *ast.ArrayDefinition {
	for depth := 0; dataType != nil && depth < 16; depth++ {
		if def, ok := dataType.(*ast.ArrayDefinition); ok {
			return def
		}
		td := t.lookupTypeDeclaration(dataType)
		if td == nil {
			return nil
		}
		dataType = td.DataType
	}
	return nil
}

// indexedArray returns the array type an expression indexes into and which
// of its dimensions: 0 for `a`, 1 for `a[i]` of a two-dimensional array.
func (t *Transpiler) indexedArray(exp ast.Expression) (*ast.ArrayDefinition, int) {
	switch e := exp.(type) {
	case *ast.Identifier:
		return t.arrayDecls[e.Value], 0
	case *ast.IndexExpression:
		def, dim := t.indexedArray(e.Left)
		return def, dim + 1
	case *ast.MemberAccessExpression:
		decl := t.memberDecl(e.Struct, e.Member.Value)
		if decl == nil {
			return nil, 0
		}
		return t.arrayDefinitionOf(decl.DataType), 0
	}
	return nil, 0
}

// decl0 finds a declaration by name, ignoring case.
func decl0(decls []*ast.VarDeclStatement, name string) *ast.VarDeclStatement {
	if i := indexOfDecl(decls, name); i >= 0 {
		return decls[i]
	}
	return nil
}

// indexLowerBound returns the declared lower bound of the dimension that
// indexing exp selects, or 0 when it is not known.
func (t *Transpiler) indexLowerBound(exp ast.Expression) int64 {
	def, dim := t.indexedArray(exp)
	if def == nil || dim >= len(def.Ranges) {
		if low, ok := t.beebreadLowerBound(exp); ok {
			return low
		}
		return 0
	}
	infix, ok := def.Ranges[dim].(*ast.InfixExpression)
	if !ok || infix.Operator != ".." {
		return 0
	}
	low, _ := constantInteger(infix.Left)
	return low
}

// ownMethod returns the method a call names when it calls a method of the
// function block being transpiled, by its name (Inc) or through THIS
// (THIS.Inc), or nil. A function of the same name takes precedence over a
// bare name.
func (t *Transpiler) ownMethod(callee ast.Expression) *ast.MethodImplementation {
	if t.currentFuncBlock == nil {
		return nil
	}
	switch fn := callee.(type) {
	case *ast.Identifier:
		if _, isFunction := t.lookupType(fn.Value).(*ast.FunctionDeclaration); isFunction {
			return nil
		}
		return t.findMethodOnFBChain(t.currentFuncBlock, fn.Value)
	case *ast.MemberAccessExpression:
		if _, isThis := fn.Struct.(*ast.ThisExpression); isThis {
			return t.findMethodOnFBChain(t.currentFuncBlock, fn.Member.Value)
		}
	}
	return nil
}
