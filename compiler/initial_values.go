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

// This file compiles the starting value of a declared variable: either its
// initial value (`x : INT := 1`), checked against the declared type, or the
// IEC 61131-3 default value of that type when none is given.

import (
	"fmt"
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
	"strings"
	"time"
)

// declaredType describes a variable's declared type after resolving names.
// Exactly one of elementary or node is set.
type declaredType struct {
	name       string   // The type's name as written, e.g. "INT" or "Color".
	elementary string   // Normalized elementary type, e.g. "INT", "TIME", "WORD".
	node       ast.Node // *ast.ArrayDefinition, *ast.TypeDeclaration or *ast.FunctionBlockDeclaration.
}

// resolveDeclaredType resolves a declaration's data type. It returns false for
// a type the compiler does not know, such as a standard function block.
func (c *Compiler) resolveDeclaredType(dataType ast.Expression) (declaredType, bool) {
	if dataType == nil {
		return declaredType{}, false
	}
	// A nil *TypeSpecifier (e.g. a method without a return type) arrives as a
	// non-nil interface.
	if ts, ok := dataType.(*ast.TypeSpecifier); ok && ts == nil {
		return declaredType{}, false
	}
	if arrayDef, ok := dataType.(*ast.ArrayDefinition); ok {
		return declaredType{name: "ARRAY", node: arrayDef}, true
	}
	name := c.flattenExpressionToString(dataType)
	if normalized := string(typedLiteralType(name)); isElementaryTypeName(normalized) {
		return declaredType{name: name, elementary: normalized}, true
	}
	if node, ok := c.resolveTypeName(name); ok {
		switch n := node.(type) {
		case *ast.TypeDeclaration:
			return declaredType{name: n.Name.Value, node: n}, true
		case *ast.FunctionBlockDeclaration:
			return declaredType{name: n.Name.Value, node: n}, true
		}
	}
	return declaredType{name: name}, false
}

// isElementaryTypeName reports whether name is an elementary IEC 61131-3 type.
func isElementaryTypeName(name string) bool {
	return object.IsIntegerType(name) || object.IsRealType(name) || object.IsBooleanType(name) ||
		object.IsStringType(name) || object.IsBitStringType(name) || isTemporalType(object.ObjectType(name))
}

// compileVarValue leaves the starting value of decl on the stack: its initial
// value if it has one, otherwise the default value of its type.
func (c *Compiler) compileVarValue(decl *ast.VarDeclStatement) error {
	return c.compileStartingValue(decl.Name.Value, decl.DataType, decl.Value, map[ast.Node]bool{})
}

// compileStartingValue leaves on the stack the value a variable named name, of
// type dataType, starts with: value checked against the type, or the type's
// default when value is nil. visiting guards against types that contain
// themselves.
func (c *Compiler) compileStartingValue(name string, dataType ast.Expression, value ast.Expression, visiting map[ast.Node]bool) error {
	declared, known := c.resolveDeclaredType(dataType)

	if structInit, ok := value.(*ast.StructLiteral); ok {
		switch n := declared.node.(type) {
		case *ast.FunctionBlockDeclaration:
			if err := c.checkFBInstantiable(n); err != nil {
				return err
			}
			return c.emitFBInstanceWith(n, structInit, visiting)
		case *ast.TypeDeclaration:
			if structDef, ok := n.DataType.(*ast.StructDefinition); ok {
				return c.emitStructValue(n.Name.Value, structDef, structInit, visiting)
			}
		}
		if !known {
			return fmt.Errorf("the initial value of '%s' is a structure initializer, but type '%s' is not supported by the compiler", name, declared.name)
		}
		return fmt.Errorf("the initial value of '%s' is a structure initializer, which requires a structure or function block type, but '%s' is neither", name, declared.name)
	}

	if value == nil {
		if !known {
			// A type the compiler does not know, such as a standard function block.
			c.emit(code.OpNull)
			return nil
		}
		return c.emitDefaultValue(name, declared, visiting)
	}

	switch n := declared.node.(type) {
	case *ast.ArrayDefinition:
		return c.compileArrayValue(name, n, value, visiting)
	case *ast.TypeDeclaration:
		return c.compileTypeDeclarationValue(name, n, value, visiting)
	}
	if declared.elementary != "" {
		return c.compileElementaryValue(name, declared.elementary, value)
	}
	return c.Compile(value)
}

// emitDefaultValue leaves the default value of a resolved type on the stack.
func (c *Compiler) emitDefaultValue(name string, declared declaredType, visiting map[ast.Node]bool) error {
	if declared.elementary != "" {
		c.emitElementaryDefault(declared.elementary)
		return nil
	}
	switch n := declared.node.(type) {
	case *ast.ArrayDefinition:
		return c.compileArrayValue(name, n, nil, visiting)
	case *ast.FunctionBlockDeclaration:
		if err := c.checkFBInstantiable(n); err != nil {
			return err
		}
		return c.emitFBInstanceWith(n, nil, visiting)
	case *ast.TypeDeclaration:
		return c.compileTypeDeclarationValue(name, n, nil, visiting)
	}
	c.emit(code.OpNull)
	return nil
}

// emitElementaryDefault leaves the IEC 61131-3 default of an elementary type
// on the stack: 0, 0.0, FALSE, ”, T#0s and so on. Dates default to
// D#0001-01-01, as in the evaluator.
func (c *Compiler) emitElementaryDefault(typeName string) {
	switch {
	case object.IsBooleanType(typeName):
		c.emit(code.OpFalse)
	case object.IsIntegerType(typeName):
		c.emitConstant(c.addConstant(&object.LInt{Value: 0}))
	case object.IsRealType(typeName):
		c.emitConstant(c.addConstant(&object.LReal{Value: 0}))
	case typeName == "WSTRING":
		c.emitConstant(c.addConstant(&object.WString{Value: ""}))
	case object.IsStringType(typeName):
		c.emitConstant(c.addConstant(&object.String{Value: ""}))
	case object.IsBitStringType(typeName):
		width, _ := object.GetBitStringWidth(typeName)
		c.emitConstant(c.addConstant(&object.BitString{Value: 0, Width: width}))
	case typeName == object.TIME_OBJ:
		c.emitConstant(c.addConstant(&object.Time{Value: 0}))
	case typeName == object.DATE_OBJ:
		c.emitConstant(c.addConstant(&object.Date{Value: time.Time{}}))
	case typeName == object.TIME_OF_DAY_OBJ:
		c.emitConstant(c.addConstant(&object.TimeOfDay{Value: time.Time{}}))
	case typeName == object.DATE_AND_TIME_OBJ:
		c.emitConstant(c.addConstant(&object.DateAndTime{Value: time.Time{}}))
	default:
		c.emit(code.OpNull)
	}
}

// compileElementaryValue compiles an initial value for a variable of an
// elementary type, checking that the value fits the type. Integer constants
// are range-checked, and become a REAL or bit string where the type is one.
func (c *Compiler) compileElementaryValue(name, typeName string, value ast.Expression) error {
	mismatch := func(valueType object.ObjectType) error {
		return fmt.Errorf("cannot initialize '%s' of type %s with a value of type %s", name, typeName, valueType)
	}

	if n, err := c.evaluateConstantInteger(value); err == nil {
		switch {
		case object.IsIntegerType(typeName):
			if result := object.ApplyConversion(&object.LInt{Value: n}, "LINT", typeName); isObjectError(result) {
				return fmt.Errorf("initial value of '%s': %s", name, objectErrorMessage(result))
			}
			return c.Compile(value)
		case object.IsRealType(typeName):
			c.emitConstant(c.addConstant(&object.LReal{Value: float64(n)}))
			return nil
		case object.IsBitStringType(typeName):
			width, _ := object.GetBitStringWidth(typeName)
			if n < 0 || (width < 64 && uint64(n)>>uint(width) != 0) {
				return fmt.Errorf("initial value of '%s': %d does not fit in %s", name, n, typeName)
			}
			c.emitConstant(c.addConstant(&object.BitString{Value: uint64(n), Width: width}))
			return nil
		case object.IsBooleanType(typeName):
			// IEC 61131-3 writes the BOOL literals as 0 and 1 too.
			if _, literal := value.(*ast.IntegerLiteral); literal && (n == 0 || n == 1) {
				if n == 1 {
					c.emit(code.OpTrue)
				} else {
					c.emit(code.OpFalse)
				}
				return nil
			}
		}
		return mismatch(object.LINT_OBJ)
	}

	valueType, err := c.getExpressionType(value)
	if err != nil || valueType == anyType {
		// The value's type is checked when it runs; any error in the value
		// itself (e.g. an undefined variable) is reported by compiling it.
		return c.Compile(value)
	}
	compatible := false
	switch {
	case object.IsIntegerType(typeName):
		compatible = object.IsIntegerType(string(valueType))
	case object.IsRealType(typeName):
		compatible = object.IsRealType(string(valueType))
	case object.IsBooleanType(typeName):
		compatible = elementaryTypeName(valueType) == object.BOOLEAN_OBJ
	case typeName == "WSTRING":
		compatible = valueType == object.WSTRING_OBJ
	case object.IsStringType(typeName):
		compatible = valueType == object.STRING_OBJ
	case object.IsBitStringType(typeName):
		compatible = object.IsBitStringType(string(valueType))
	default:
		compatible = string(valueType) == typeName
	}
	if !compatible {
		return mismatch(valueType)
	}
	return c.Compile(value)
}

// compileTypeDeclarationValue compiles the starting value of a variable whose
// type is a user-defined TYPE: a structure, enumeration, array, alias or
// subrange. value may be nil, in which case the type's default is used.
func (c *Compiler) compileTypeDeclarationValue(name string, td *ast.TypeDeclaration, value ast.Expression, visiting map[ast.Node]bool) error {
	if visiting[td] {
		return fmt.Errorf("type '%s' contains itself", td.Name.Value)
	}
	visiting[td] = true
	defer delete(visiting, td)

	switch dt := td.DataType.(type) {
	case *ast.StructDefinition:
		if value == nil {
			// A structure's default comes from its type's initial value, if
			// any, and otherwise from its members' initial values.
			if init, ok := td.InitialValue.(*ast.StructLiteral); ok {
				return c.emitStructValue(td.Name.Value, dt, init, visiting)
			}
			return c.emitStructValue(td.Name.Value, dt, nil, visiting)
		}
		// Any other value must be a structure of the same type.
		if valueType, err := c.getExpressionType(value); err == nil && valueType != anyType && !strings.EqualFold(string(valueType), td.Name.Value) {
			return fmt.Errorf("cannot initialize '%s' of type %s with a value of type %s", name, td.Name.Value, valueType)
		}
		return c.Compile(value)
	case *ast.EnumDefinition:
		return c.compileEnumValue(name, td, dt, value)
	case *ast.ArrayDefinition:
		if value == nil {
			value = td.InitialValue
		}
		return c.compileArrayValue(name, dt, value, visiting)
	}

	// An alias or subrange of another type.
	if value == nil {
		value = td.InitialValue
	}
	if value == nil {
		if subrange, ok := td.Subrange.(*ast.InfixExpression); ok && subrange.Operator == ".." {
			value = subrange.Left // IEC 61131-3: a subrange defaults to its lower limit.
		}
	}
	if subrange, ok := td.Subrange.(*ast.InfixExpression); ok && subrange.Operator == ".." && value != nil {
		if err := c.checkSubrangeValue(name, td, subrange, value); err != nil {
			return err
		}
	}
	return c.compileStartingValue(name, td.DataType, value, visiting)
}

// checkSubrangeValue reports an error if a constant initial value lies outside
// a subrange type's limits.
func (c *Compiler) checkSubrangeValue(name string, td *ast.TypeDeclaration, subrange *ast.InfixExpression, value ast.Expression) error {
	n, err := c.evaluateConstantInteger(value)
	if err != nil {
		return nil // Not a constant; checked when it runs, if at all.
	}
	low, errLow := c.evaluateConstantInteger(subrange.Left)
	high, errHigh := c.evaluateConstantInteger(subrange.Right)
	if errLow == nil && errHigh == nil && (n < low || n > high) {
		return fmt.Errorf("initial value of '%s': %d is out of range for subrange type '%s' (%d..%d)", name, n, td.Name.Value, low, high)
	}
	return nil
}

// compileEnumValue compiles the starting value of an enumeration variable.
// The default is the type's initial value, or its first value. A value may
// be written as `Color#Green` or, for a variable of that type, as `Green`.
func (c *Compiler) compileEnumValue(name string, td *ast.TypeDeclaration, def *ast.EnumDefinition, value ast.Expression) error {
	if value == nil {
		value = td.InitialValue
	}
	emitMember := func(member string) error {
		for _, v := range def.Values {
			if strings.EqualFold(v.Value, member) {
				c.emitConstant(c.addConstant(&object.EnumeratedValue{TypeName: td.Name.Value, Value: v.Value}))
				return nil
			}
		}
		return fmt.Errorf("initial value of '%s': '%s' is not a value of enumeration '%s'", name, member, td.Name.Value)
	}

	switch v := value.(type) {
	case nil:
		if len(def.Values) == 0 {
			c.emit(code.OpNull)
			return nil
		}
		return emitMember(def.Values[0].Value)
	case *ast.Identifier:
		// A bare value name, unless it names a variable.
		if _, isSymbol := c.symbolTable.Resolve(v.Value); !isSymbol {
			return emitMember(v.Value)
		}
	case *ast.TypedLiteral:
		if !strings.EqualFold(v.TypeName, td.Name.Value) {
			return fmt.Errorf("cannot initialize '%s' of type %s with a value of type %s", name, td.Name.Value, v.TypeName)
		}
		if member, ok := v.Value.(*ast.Identifier); ok {
			return emitMember(member.Value)
		}
	case *ast.EnumeratedValueLiteral:
		if !strings.EqualFold(v.TypeName.Value, td.Name.Value) {
			return fmt.Errorf("cannot initialize '%s' of type %s with a value of type %s", name, td.Name.Value, v.TypeName.Value)
		}
		return emitMember(v.Value.Value)
	}
	return c.Compile(value)
}

// emitStructValue leaves a structure value on the stack: a hash with one entry
// per member. A member takes its value from init, if given there, otherwise
// from its declaration's initial value, otherwise its type's default.
func (c *Compiler) emitStructValue(typeName string, def *ast.StructDefinition, init *ast.StructLiteral, visiting map[ast.Node]bool) error {
	overrides, err := structInitializers(init)
	if err != nil {
		return err
	}
	for member := range overrides {
		found := false
		for _, m := range def.Members {
			if strings.EqualFold(m.Name.Value, member) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("structure '%s' has no member '%s'", typeName, member)
		}
	}
	for _, m := range def.Members {
		c.emitConstant(c.addConstant(&object.String{Value: m.Name.Value}))
		value := m.Value
		if override, ok := lookupFold(overrides, m.Name.Value); ok {
			value = override
		}
		if err := c.compileStartingValue(m.Name.Value, m.DataType, value, visiting); err != nil {
			return err
		}
	}
	c.emit(code.OpHash, len(def.Members)*2)
	return nil
}

// structInitializers returns the `name := value` pairs of a structure
// initializer such as `(PT := T#1s, IN := TRUE)`.
func structInitializers(init *ast.StructLiteral) (map[string]ast.Expression, error) {
	out := map[string]ast.Expression{}
	if init == nil {
		return out, nil
	}
	for _, e := range init.Initializers {
		named, ok := e.(*ast.NamedArgument)
		if !ok {
			return nil, fmt.Errorf("a structure initializer must list members as name := value, got %s", e.String())
		}
		out[named.Name.Value] = named.Value
	}
	return out, nil
}

// lookupFold finds a key in m, ignoring case.
func lookupFold(m map[string]ast.Expression, key string) (ast.Expression, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// arrayLength returns the number of elements for one array range such as
// `0..9`, or false if the bounds are not constant.
func (c *Compiler) arrayLength(rng ast.Expression) (int, bool) {
	infix, ok := rng.(*ast.InfixExpression)
	if !ok || infix.Operator != ".." {
		return 0, false
	}
	low, errLow := c.evaluateConstantInteger(infix.Left)
	high, errHigh := c.evaluateConstantInteger(infix.Right)
	if errLow != nil || errHigh != nil || high < low {
		return 0, false
	}
	return int(high-low) + 1, true
}

// compileArrayValue compiles the starting value of an array. Without a value,
// every element takes the element type's default; a multi-dimensional array
// is an array of arrays. A one-dimensional array literal may list fewer
// elements than the array holds (the rest take the default) but not more.
func (c *Compiler) compileArrayValue(name string, def *ast.ArrayDefinition, value ast.Expression, visiting map[ast.Node]bool) error {
	if err := c.compileArrayElements(name, def, value, visiting); err != nil {
		return err
	}
	return c.emitArrayBounds(def)
}

// compileArrayElements compiles the elements of an array value; see
// compileArrayValue.
func (c *Compiler) compileArrayElements(name string, def *ast.ArrayDefinition, value ast.Expression, visiting map[ast.Node]bool) error {
	var elementType ast.Expression
	if def.DataType != nil {
		elementType = def.DataType
	}
	lengths := []int{}
	for _, rng := range def.Ranges {
		n, ok := c.arrayLength(rng)
		if !ok {
			return fmt.Errorf("array '%s' must have constant bounds, got %s", name, rng.String())
		}
		lengths = append(lengths, n)
	}

	literal, isLiteral := value.(*ast.ArrayLiteral)
	if isLiteral && len(lengths) > 1 && !nestedArrayLiteral(literal) {
		return c.compileFlatArrayLiteral(name, elementType, lengths, literal, visiting)
	}
	if value != nil && (!isLiteral || len(lengths) != 1) {
		// Another array value, or a multi-dimensional literal: compiled as written.
		return c.Compile(value)
	}

	var emitDefaults func(dims []int) error
	emitDefaults = func(dims []int) error {
		for i := 0; i < dims[0]; i++ {
			if len(dims) > 1 {
				if err := emitDefaults(dims[1:]); err != nil {
					return err
				}
				continue
			}
			if err := c.compileStartingValue(name, elementType, nil, visiting); err != nil {
				return err
			}
		}
		c.emit(code.OpArray, dims[0])
		return nil
	}
	if !isLiteral {
		return emitDefaults(lengths)
	}

	count := 0
	for _, el := range literal.Elements {
		if rep, ok := el.(*ast.ArrayRepetition); ok {
			n, err := c.compileArrayRepetition(rep)
			if err != nil {
				return err
			}
			count += n
			continue
		}
		if err := c.compileStartingValue(name, elementType, el, visiting); err != nil {
			return err
		}
		count++
	}
	if count > lengths[0] {
		return fmt.Errorf("array '%s' has %d elements but its initial value lists %d", name, lengths[0], count)
	}
	for ; count < lengths[0]; count++ {
		if err := c.compileStartingValue(name, elementType, nil, visiting); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, lengths[0])
	return nil
}

// checkFBInstantiable reports whether a function block may be instantiated
// here: it must not be abstract, and an INTERNAL one must be in the same
// namespace.
func (c *Compiler) checkFBInstantiable(fbDef *ast.FunctionBlockDeclaration) error {
	if fbDef.IsAbstract {
		return fmt.Errorf("cannot instantiate abstract function block '%s'", fbDef.Name.Value)
	}
	if fbDef.AccessSpecifier == "INTERNAL" {
		var defFqn string
		for fqn, n := range c.typeInfo {
			if n == fbDef {
				defFqn = fqn
				break
			}
		}
		if c.pouNamespaces[defFqn] != c.currentNS {
			return fmt.Errorf("cannot access INTERNAL function block '%s' from a different namespace", fbDef.Name.Value)
		}
	}
	return nil
}

// isObjectError reports whether o is an *object.Error.
func isObjectError(o object.Object) bool {
	_, ok := o.(*object.Error)
	return ok
}

// objectErrorMessage returns an object.Error's message without the
// "BUILTIN ERROR: " prefix used by the object package.
func objectErrorMessage(o object.Object) string {
	if e, ok := o.(*object.Error); ok {
		return strings.TrimPrefix(e.Message, "BUILTIN ERROR: ")
	}
	return ""
}

// initSymbol stores the value on the stack into s as its initial value. Unlike
// setSymbol, it also initializes constants and read-only variables: a
// declaration's own initial value is not an assignment.
func (c *Compiler) initSymbol(s Symbol) error {
	s.IsConstant = false
	s.IsReadOnly = false
	return c.setSymbol(s)
}

// calleeInputs returns the VAR_INPUT declarations of the function or method a
// call targets, when it can be determined statically.
func (c *Compiler) calleeInputs(function ast.Expression) ([]*ast.VarDeclStatement, bool) {
	switch f := function.(type) {
	case *ast.Identifier:
		if def, ok := c.resolveTypeName(f.Value); ok {
			if fn, isFn := def.(*ast.FunctionDeclaration); isFn {
				return fn.VarInputs, true
			}
		}
	case *ast.MemberAccessExpression:
		var owner *ast.FunctionBlockDeclaration
		if deref, ok := f.Struct.(*ast.DereferenceExpression); ok {
			if _, isSuper := deref.Pointer.(*ast.SuperExpression); isSuper && c.currentFB != nil && c.currentFB.Extends != nil {
				if parent, ok := c.resolveTypeNode(c.currentFB.Extends); ok {
					owner, _ = parent.(*ast.FunctionBlockDeclaration)
				}
			}
		} else if typeName, ok := c.getExpressionTypeName(f.Struct); ok {
			if def, ok := c.resolveTypeName(typeName); ok {
				owner, _ = def.(*ast.FunctionBlockDeclaration)
			}
		}
		if owner != nil {
			if method, _ := c.findMethodOnFBChain(owner, f.Member.Value); method != nil {
				return method.VarInputs, true
			}
		}
	}
	return nil, false
}

// emitOmittedInputs appends, as named arguments, every input of the callee
// that a formal call does not supply: IEC 61131-3 gives an omitted input its
// initial value, or its type's default. A formal call names its arguments
// (`f(a := 1)`); an empty call `f()` is treated as formal. A non-formal call
// (`f(1, 2)`) must supply every input, so nothing is added and the VM reports
// a wrong argument count. It returns how many arguments were added.
func (c *Compiler) emitOmittedInputs(callee string, inputs []*ast.VarDeclStatement, args []ast.Expression) (int, error) {
	supplied := map[string]bool{}
	positional, named := 0, 0
	for _, arg := range args {
		if namedArg, ok := arg.(*ast.NamedArgument); ok {
			named++
			known := false
			for _, input := range inputs {
				if strings.EqualFold(input.Name.Value, namedArg.Name.Value) {
					known = true
				}
			}
			if !known {
				return 0, fmt.Errorf("'%s' has no input '%s'", callee, namedArg.Name.Value)
			}
			supplied[strings.ToUpper(namedArg.Name.Value)] = true
		} else {
			positional++
		}
	}
	if positional > 0 && named == 0 {
		return 0, nil // Non-formal call.
	}
	for i := 0; i < positional && i < len(inputs); i++ {
		supplied[strings.ToUpper(inputs[i].Name.Value)] = true
	}

	added := 0
	for _, input := range inputs {
		if supplied[strings.ToUpper(input.Name.Value)] {
			continue
		}
		if err := c.compileVarValue(input); err != nil {
			return 0, err
		}
		c.emit(code.OpMakeNamedArg, c.addConstant(&object.String{Value: input.Name.Value}))
		added++
	}
	return added, nil
}

// nestedArrayLiteral reports whether an array literal is written as rows,
// [[1, 2], [3, 4]], rather than as one list.
func nestedArrayLiteral(literal *ast.ArrayLiteral) bool {
	for _, el := range literal.Elements {
		if _, ok := el.(*ast.ArrayLiteral); !ok {
			return false
		}
	}
	return len(literal.Elements) > 0
}

// compileFlatArrayLiteral compiles the initial value of a multi-dimensional
// array written as one list, as IEC 61131-3 writes it: the elements in row
// order, the last index changing fastest, so that ARRAY[1..2, 1..2] OF INT
// := [1, 2, 3, 4] has the rows [1, 2] and [3, 4]. Elements the list leaves
// out take their type's default.
func (c *Compiler) compileFlatArrayLiteral(name string, elementType ast.Expression, lengths []int, literal *ast.ArrayLiteral, visiting map[ast.Node]bool) error {
	var elements []ast.Expression
	for _, el := range literal.Elements {
		rep, ok := el.(*ast.ArrayRepetition)
		if !ok {
			elements = append(elements, el)
			continue
		}
		n, err := c.evaluateConstantInteger(rep.Factor)
		if err != nil {
			return fmt.Errorf("array '%s': the repetition factor %s is not a constant", name, rep.Factor.String())
		}
		for i := int64(0); i < n; i++ {
			elements = append(elements, rep.Elements...)
		}
	}
	total := 1
	for _, n := range lengths {
		total *= n
	}
	if len(elements) > total {
		return fmt.Errorf("array '%s' has %d elements but its initial value lists %d", name, total, len(elements))
	}
	next := 0
	var emitRows func(dims []int) error
	emitRows = func(dims []int) error {
		for i := 0; i < dims[0]; i++ {
			if len(dims) > 1 {
				if err := emitRows(dims[1:]); err != nil {
					return err
				}
				continue
			}
			var el ast.Expression
			if next < len(elements) {
				el = elements[next]
			}
			next++
			if err := c.compileStartingValue(name, elementType, el, visiting); err != nil {
				return err
			}
		}
		c.emit(code.OpArray, dims[0])
		return nil
	}
	return emitRows(lengths)
}
