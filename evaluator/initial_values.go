/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

// This file evaluates the starting value of variables whose type is a
// structure or enumeration, and structure initializers such as
// `(PT := T#1s)`. A structure value is a Hash keyed by member name, as in the
// compiler and VM.

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
	"strings"
)

// lookupTypeDeclaration finds a user-defined TYPE by name, ignoring case.
func lookupTypeDeclaration(typeName string, env *object.Environment) (*ast.TypeDeclaration, bool) {
	// A type qualified by its namespace, such as Lib.Mode, is declared in
	// that namespace's environment.
	if nsName, rest, qualified := strings.Cut(typeName, "."); qualified {
		if ns, ok := lookupNamespace(nsName, env); ok {
			return lookupTypeDeclaration(rest, ns.Env)
		}
	}
	quoteObj, ok := env.Get("_type_" + strings.ToUpper(typeName))
	if !ok {
		return nil, false
	}
	quote, ok := quoteObj.(*object.Quote)
	if !ok {
		return nil, false
	}
	td, ok := quote.Node.(*ast.TypeDeclaration)
	return td, ok
}

// evalStructuredDeclaration evaluates the starting value of a declaration whose
// type is a structure or enumeration, or whose initial value is a structure
// initializer. It reports false when neither applies, leaving the declaration
// to evalVarDeclStatement.
func evalStructuredDeclaration(node *ast.VarDeclStatement, env *object.Environment) (object.Object, bool) {
	if node.DataType == nil {
		return nil, false
	}
	if def, isArray := node.DataType.(*ast.ArrayDefinition); isArray {
		return arrayValue(node, def, env), true
	}
	structInit, isStructInit := node.Value.(*ast.StructLiteral)
	td, isUserType := lookupTypeDeclaration(node.DataType.String(), env)

	if isUserType {
		switch def := td.DataType.(type) {
		case *ast.StructDefinition:
			if node.Value != nil && !isStructInit {
				return nil, false // Another value, e.g. a structure variable.
			}
			return structValue(node, td, def, structInit, env, map[*ast.TypeDeclaration]bool{}), true
		case *ast.EnumDefinition:
			return enumValue(node, td, def, env)
		case *ast.ArrayDefinition:
			// A variable of a named array type takes the type's initial value
			// unless it has its own.
			decl := *node
			if decl.Value == nil {
				decl.Value = td.InitialValue
			}
			return arrayValue(&decl, def, env), true
		}
	}

	if isStructInit {
		// A function block instance (user-defined or standard, such as TON)
		// with some variables set by the initializer.
		withoutValue := *node
		withoutValue.Value = nil
		instanceObj := evalVarDeclStatement(&withoutValue, env)
		if isError(instanceObj) {
			return instanceObj, true
		}
		instance, ok := instanceObj.(*object.FunctionBlockInstance)
		if !ok {
			return newError(node, "the initial value of '%s' is a structure initializer, which requires a structure or function block type, but '%s' is neither", node.Name.Value, node.DataType.String()), true
		}
		overrides, err := structInitializerValues(node, structInit, env)
		if err != nil {
			return err, true
		}
		if instance.Definition != nil {
			for name := range overrides {
				if varDecl, _ := findVarDeclOnFBChain(instance.Definition, name, env); varDecl == nil {
					return newError(node, "function block '%s' has no variable '%s'", instance.Definition.Name.Value, name), true
				}
			}
		}
		for name, val := range overrides {
			instance.Env.Set(name, val)
		}
		return instance, true
	}
	return nil, false
}

// structInitializerValues evaluates the `name := value` pairs of a structure
// initializer.
func structInitializerValues(node *ast.VarDeclStatement, init *ast.StructLiteral, env *object.Environment) (map[string]object.Object, *object.Error) {
	out := map[string]object.Object{}
	if init == nil {
		return out, nil
	}
	for _, e := range init.Initializers {
		named, ok := e.(*ast.NamedArgument)
		if !ok {
			return nil, newError(node, "a structure initializer must list members as name := value, got %s", e.String())
		}
		val := Eval(named.Value, env)
		if errObj, isErr := val.(*object.Error); isErr {
			return nil, errObj
		}
		out[named.Name.Value] = val
	}
	return out, nil
}

// structValue builds a structure value. Each member takes its value from init,
// if given there, otherwise from its declared initial value, otherwise from
// its type's default. Without an initializer on the variable, the type's own
// initial value (if it is a structure initializer) applies.
func structValue(node *ast.VarDeclStatement, td *ast.TypeDeclaration, def *ast.StructDefinition, init *ast.StructLiteral, env *object.Environment, visiting map[*ast.TypeDeclaration]bool) object.Object {
	if visiting[td] {
		return newError(node, "type '%s' contains itself", td.Name.Value)
	}
	visiting[td] = true
	defer delete(visiting, td)

	if init == nil {
		init, _ = td.InitialValue.(*ast.StructLiteral)
	}
	overrides, errObj := structInitializerValues(node, init, env)
	if errObj != nil {
		return errObj
	}
	for name := range overrides {
		found := false
		for _, m := range def.Members {
			if strings.EqualFold(m.Name.Value, name) {
				found = true
			}
		}
		if !found {
			return newError(node, "structure '%s' has no member '%s'", td.Name.Value, name)
		}
	}

	// Members are evaluated in a scratch environment enclosed by env, so their
	// initial values can use the surrounding constants.
	memberEnv := object.NewEnclosedEnvironment(env)
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{}}
	for _, m := range def.Members {
		var val object.Object
		if override, ok := lookupObjectFold(overrides, m.Name.Value); ok {
			val = override
		} else if nestedTD, ok := lookupTypeDeclaration(dataTypeName(m.DataType), env); ok {
			if nestedDef, isStruct := nestedTD.DataType.(*ast.StructDefinition); isStruct && m.Value == nil {
				val = structValue(m, nestedTD, nestedDef, nil, env, visiting)
			}
		}
		if val == nil {
			val = evalVarDeclStatement(m, memberEnv)
		}
		if isError(val) {
			return val
		}
		key := &object.String{Value: m.Name.Value}
		hash.Pairs[key.HashKey()] = object.HashPair{Key: key, Value: val}
	}
	return hash
}

// enumValue evaluates the starting value of an enumeration variable: the
// variable's initial value, else the type's, else the first value. A value
// may be written as `Color#Green` or, for a variable of that type, `Green`.
func enumValue(node *ast.VarDeclStatement, td *ast.TypeDeclaration, def *ast.EnumDefinition, env *object.Environment) (object.Object, bool) {
	value := node.Value
	if value == nil {
		value = td.InitialValue
	}
	member := func(name string) object.Object {
		for _, v := range def.Values {
			if strings.EqualFold(v.Value, name) {
				return &object.EnumeratedValue{TypeName: strings.ToUpper(td.Name.Value), Value: v.Value}
			}
		}
		return newError(node, "initial value of '%s': '%s' is not a value of enumeration '%s'", node.Name.Value, name, td.Name.Value)
	}

	switch v := value.(type) {
	case nil:
		if len(def.Values) == 0 {
			return NULL, true
		}
		return member(def.Values[0].Value), true
	case *ast.Identifier:
		// A bare value name, unless it names a variable.
		if _, isVar := env.Get(v.Value); !isVar {
			return member(v.Value), true
		}
	}
	// Any other value (e.g. `Color#Green`) is evaluated as written.
	return nil, false
}

// dataTypeName returns the name of a declared type, or "" for none.
func dataTypeName(dataType ast.Expression) string {
	if dataType == nil {
		return ""
	}
	return dataType.String()
}

// lookupObjectFold finds a key in m, ignoring case.
func lookupObjectFold(m map[string]object.Object, key string) (object.Object, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// hashMemberKey returns the key of a structure member in hash, matching the
// member name case-insensitively, or false if it has no such member.
func hashMemberKey(hash *object.Hash, member string) (object.HashKey, bool) {
	for key, pair := range hash.Pairs {
		if name, ok := pair.Key.(*object.String); ok && strings.EqualFold(name.Value, member) {
			return key, true
		}
	}
	return object.HashKey{}, false
}

// typeSpecifierExpression returns ts as an expression, or a true nil when ts
// is nil, so callers can test the result against nil.
func typeSpecifierExpression(ts *ast.TypeSpecifier) ast.Expression {
	if ts == nil {
		return nil
	}
	return ts
}

// declareResult sets a function's or method's result variable to the default
// of its return type (NULL when it has none).
func declareResult(name *ast.Identifier, returnType ast.Expression, env *object.Environment) object.Object {
	if returnType == nil {
		env.Set(name.Value, NULL)
		return NULL
	}
	return evalVarDeclStatement(&ast.VarDeclStatement{Token: name.Token, Name: name, DataType: returnType}, env)
}

// declareOmittedInputs declares, in a call's environment, each input that a
// formal call omits, with its initial value or its type's default. A formal
// call names its arguments (`f(a := 1)`); an empty call `f()` counts as
// formal. A non-formal (positional) call must supply every input, so nothing
// is declared for it.
func declareOmittedInputs(inputs []*ast.VarDeclStatement, args []ast.Expression, env *object.Environment) object.Object {
	positional, named := 0, 0
	for _, arg := range args {
		switch arg.(type) {
		case *ast.NamedArgument:
			named++
		case *ast.OutputArgument:
		default:
			positional++
		}
	}
	if positional > 0 && named == 0 {
		return NULL
	}
	for _, input := range inputs {
		if _, supplied := env.GetRaw(input.Name.Value); supplied {
			continue
		}
		if result := evalVarDeclStatement(input, env); isError(result) {
			return result
		}
	}
	return NULL
}

// declareOutputs declares a call's VAR_OUTPUTs with their initial values or
// their types' defaults.
func declareOutputs(outputs []*ast.VarDeclStatement, env *object.Environment) object.Object {
	for _, output := range outputs {
		if result := evalVarDeclStatement(output, env); isError(result) {
			return result
		}
	}
	return NULL
}

// arrayValue evaluates the starting value of an array variable, as in the
// compiler: each element takes the element type's default, a one-dimensional
// array literal may list fewer elements than the array holds (the rest take
// the default) but not more, and a multi-dimensional array is an array of
// arrays. Another value, or a multi-dimensional literal, is used as written.
func arrayValue(node *ast.VarDeclStatement, def *ast.ArrayDefinition, env *object.Environment) object.Object {
	value := arrayElements(node, def, env)
	if !isError(value) {
		object.SetLowerBounds(value, declaredLowerBounds(def, env))
	}
	return value
}

// arrayElements evaluates the elements of an array value; see arrayValue.
func arrayElements(node *ast.VarDeclStatement, def *ast.ArrayDefinition, env *object.Environment) object.Object {
	lengths := []int{}
	for _, rng := range def.Ranges {
		n, err := arrayLength(rng, env)
		if err != nil {
			return newError(node, "array '%s': %s", node.Name.Value, err.Message)
		}
		lengths = append(lengths, n)
	}

	elementDefault := func() object.Object {
		element := &ast.VarDeclStatement{Token: node.Token, Name: node.Name}
		if def.DataType != nil {
			element.DataType = def.DataType
		}
		// Declared in a scratch scope so the variable itself is not set.
		return evalVarDeclStatement(element, object.NewEnclosedEnvironment(env))
	}
	var defaults func(dims []int) object.Object
	defaults = func(dims []int) object.Object {
		elements := make([]object.Object, dims[0])
		for i := range elements {
			if len(dims) > 1 {
				elements[i] = defaults(dims[1:])
			} else {
				elements[i] = elementDefault()
			}
			if isError(elements[i]) {
				return elements[i]
			}
		}
		return &object.Array{Elements: elements}
	}

	if node.Value == nil {
		return defaults(lengths)
	}
	value := Eval(node.Value, env)
	if isError(value) {
		return value
	}
	array, isArray := value.(*object.Array)
	if _, isLiteral := node.Value.(*ast.ArrayLiteral); isLiteral && isArray && len(lengths) > 1 && !holdsRows(array) {
		return arrayRows(node, array.Elements, lengths, elementDefault)
	}
	if _, isLiteral := node.Value.(*ast.ArrayLiteral); !isLiteral || !isArray || len(lengths) != 1 {
		return value
	}
	if len(array.Elements) > lengths[0] {
		return newError(node, "array '%s' holds %d elements, but its initial value has %d", node.Name.Value, lengths[0], len(array.Elements))
	}
	for len(array.Elements) < lengths[0] {
		element := elementDefault()
		if isError(element) {
			return element
		}
		array.Elements = append(array.Elements, element)
	}
	return array
}

// arrayLength returns the number of elements of an array dimension such as
// 1..10, whose bounds must be constant integers.
func arrayLength(rng ast.Expression, env *object.Environment) (int, *object.Error) {
	infix, ok := rng.(*ast.InfixExpression)
	if !ok || infix.Operator != ".." {
		return 0, newError(rng, "dimension must be a range such as 0..9, got %s", rng.String())
	}
	bound := func(e ast.Expression) (int64, *object.Error) {
		v := Eval(e, env)
		if errObj, isErr := v.(*object.Error); isErr {
			return 0, errObj
		}
		if c, isConst := v.(*object.Constant); isConst {
			v = c.Value
		}
		n, _, ok := object.GetIntegerObjectValue(v)
		if !ok {
			return 0, newError(e, "bound must be an integer, got %s", v.Type())
		}
		return n, nil
	}
	low, err := bound(infix.Left)
	if err != nil {
		return 0, err
	}
	high, err := bound(infix.Right)
	if err != nil {
		return 0, err
	}
	if high < low {
		return 0, newError(rng, "upper bound %d is below lower bound %d", high, low)
	}
	return int(high-low) + 1, nil
}

// declaredLowerBounds returns the lower bound of each dimension of an array
// type, e.g. [1] for ARRAY[1..3]. A bound that is not a constant integer
// counts as 0; arrayLength reports it.
func declaredLowerBounds(def *ast.ArrayDefinition, env *object.Environment) []int64 {
	bounds := []int64{}
	for _, rng := range def.Ranges {
		low := int64(0)
		if infix, ok := rng.(*ast.InfixExpression); ok && infix.Operator == ".." {
			v := Eval(infix.Left, env)
			if c, isConst := v.(*object.Constant); isConst {
				v = c.Value
			}
			if n, _, ok := object.GetIntegerObjectValue(v); ok {
				low = n
			}
		}
		bounds = append(bounds, low)
	}
	return bounds
}

// keepArrayBounds gives an array assigned to a variable the lower bounds the
// variable's array was declared with, e.g. `a := [1, 2, 3]` for an
// ARRAY[1..3]. The value is returned unchanged.
func keepArrayBounds(existing, value object.Object) object.Object {
	if _, wasArray := existing.(*object.Array); wasArray {
		if _, isArray := value.(*object.Array); isArray {
			object.SetLowerBounds(value, object.LowerBounds(existing))
		}
	}
	return value
}

// stampArrayParameters gives each array parameter the lower bounds of its
// declared type, so an argument such as an array literal is indexed like the
// parameter.
func stampArrayParameters(params []*ast.VarDeclStatement, env *object.Environment) {
	for _, p := range params {
		def, ok := p.DataType.(*ast.ArrayDefinition)
		if !ok {
			if td, found := lookupTypeDeclaration(dataTypeName(p.DataType), env); found {
				def, ok = td.DataType.(*ast.ArrayDefinition)
			}
		}
		if !ok {
			continue
		}
		if value, found := env.GetRaw(p.Name.Value); found {
			object.SetLowerBounds(value, declaredLowerBounds(def, env))
		}
	}
}

// lookupNamespace finds a namespace by name, ignoring case, in env or an
// enclosing environment.
func lookupNamespace(name string, env *object.Environment) (*object.Namespace, bool) {
	for e := env; e != nil; e = e.Outer() {
		for _, n := range e.Names() {
			if !strings.EqualFold(n, name) {
				continue
			}
			if obj, ok := e.GetRaw(n); ok {
				if ns, isNamespace := obj.(*object.Namespace); isNamespace {
					return ns, true
				}
			}
		}
	}
	return nil, false
}

// holdsRows reports whether an array value is written as rows, [[1, 2],
// [3, 4]], rather than as one list.
func holdsRows(array *object.Array) bool {
	for _, el := range array.Elements {
		if _, ok := el.(*object.Array); !ok {
			return false
		}
	}
	return len(array.Elements) > 0
}

// arrayRows arranges the initial value of a multi-dimensional array, written
// as one list, into rows, as IEC 61131-3 reads it: the last index changes
// fastest, so ARRAY[1..2, 1..2] OF INT := [1, 2, 3, 4] has the rows [1, 2]
// and [3, 4]. Elements the list leaves out take their type's default.
func arrayRows(node *ast.VarDeclStatement, flat []object.Object, lengths []int, elementDefault func() object.Object) object.Object {
	total := 1
	for _, n := range lengths {
		total *= n
	}
	if len(flat) > total {
		return newError(node, "array '%s' holds %d elements, but its initial value has %d", node.Name.Value, total, len(flat))
	}
	next := 0
	var rows func(dims []int) object.Object
	rows = func(dims []int) object.Object {
		elements := make([]object.Object, dims[0])
		for i := range elements {
			switch {
			case len(dims) > 1:
				elements[i] = rows(dims[1:])
			case next < len(flat):
				elements[i] = flat[next]
				next++
			default:
				elements[i] = elementDefault()
			}
			if isError(elements[i]) {
				return elements[i]
			}
		}
		return &object.Array{Elements: elements}
	}
	return rows(lengths)
}
