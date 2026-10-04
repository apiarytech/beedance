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

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// REFERENCE_OBJ is the type of a reference made by REF() or ADR().
const REFERENCE_OBJ = "REFERENCE"

// Reference is a typed reference to a variable, the value of a variable
// declared REF_TO <type> (IEC 61131-3) or POINTER TO <type> (CODESYS), which
// beedance treats alike: REF(x) and ADR(x) make one, r^ reads and writes the
// variable, and NULL refers to nothing. There is no pointer arithmetic.
//
// The variable is one of: Name in Env (the evaluator), a located variable
// Name in IO, a VM variable's Slot, or the element Index of Container, an
// array or a structure or function block instance. A Reference with none of
// them is a NULL reference that remembers its declared type's view (Lower).
type Reference struct {
	Env  *Environment
	Name string
	IO   map[string]Object
	Slot *Object

	Container Object
	Index     Object

	// Lower holds the lower bound of each dimension of the array type a
	// variable is declared to refer to, when the evaluator knows it:
	// through `pt : POINTER TO ARRAY[0..9] OF REAL`, pt^[0] is the first
	// element of the array pt refers to, whatever its own bounds.
	Lower []int64

	// DataType is the declared type of the variable referred to, when it is
	// known, so that a reference to a REAL is not given to a REF_TO INT.
	DataType ast.Expression

	// Live, when set, reports whether the variable still exists: a VM
	// function's local variable exists until the call returns.
	Live func() bool
}

// Type returns the object's type.
func (r *Reference) Type() ObjectType { return REFERENCE_OBJ }

// Inspect returns a description of the reference, e.g. REF(x).
func (r *Reference) Inspect() string {
	switch {
	case r.IsNull():
		return "NULL"
	case r.Container != nil:
		return fmt.Sprintf("REF(%s[%s])", r.Container.Type(), r.Index.Inspect())
	case r.Slot != nil:
		return "REF(variable)"
	}
	return fmt.Sprintf("REF(%s)", r.Name)
}

// IsNull reports whether the reference refers to nothing.
func (r *Reference) IsNull() bool {
	return r.Env == nil && r.IO == nil && r.Slot == nil && r.Container == nil
}

// Retarget returns a reference to the variable other refers to, keeping
// the view of r's declared type, as a variable declared REF_TO keeps its
// type when assigned. other may be NULL.
func (r *Reference) Retarget(other Object) *Reference {
	out := &Reference{Lower: r.Lower}
	if o, ok := other.(*Reference); ok {
		out.Env, out.Name, out.IO, out.Slot = o.Env, o.Name, o.IO, o.Slot
		out.Container, out.Index = o.Container, o.Index
		out.DataType, out.Live = o.DataType, o.Live
	}
	return out
}

// errGone is the error for a reference to a function's local variable used
// after the function has returned.
var errGone = fmt.Errorf("the variable a reference refers to no longer exists: it is a local variable of a call that has returned")

// Get returns the value of the variable the reference refers to.
func (r *Reference) Get() (Object, error) {
	switch {
	case r.IsNull():
		return nil, fmt.Errorf("dereferencing a NULL reference")
	case r.Live != nil && !r.Live():
		return nil, errGone
	case r.Slot != nil:
		return *r.Slot, nil
	case r.IO != nil:
		return r.IO[r.Name], nil
	case r.Container != nil:
		return r.element()
	}
	v, ok := r.Env.Get(r.Name)
	if !ok {
		return nil, fmt.Errorf("the variable %s a reference refers to no longer exists", r.Name)
	}
	if c, isConst := v.(*Constant); isConst {
		return c.Value, nil
	}
	return v, nil
}

// Set writes the variable the reference refers to.
func (r *Reference) Set(v Object) error {
	switch {
	case r.IsNull():
		return fmt.Errorf("dereferencing a NULL reference")
	case r.Live != nil && !r.Live():
		return errGone
	case r.Slot != nil:
		*r.Slot = v
	case r.IO != nil:
		r.IO[r.Name] = v
	case r.Container != nil:
		return r.setElement(v)
	default:
		if current, ok := r.Env.Get(r.Name); ok {
			if _, isConst := current.(*Constant); isConst {
				return fmt.Errorf("cannot assign to constant variable '%s' through a reference", r.Name)
			}
		}
		r.Env.Assign(r.Name, v)
	}
	return nil
}

// element returns the element of an array, or the member of a structure or
// function block instance, the reference refers to.
func (r *Reference) element() (Object, error) {
	switch c := r.Container.(type) {
	case *Array:
		i, err := r.arrayIndex(c)
		if err != nil {
			return nil, err
		}
		return c.Elements[i], nil
	case *Hash:
		pair, ok := c.Pairs[r.Index.(Hashable).HashKey()]
		if !ok {
			return nil, fmt.Errorf("no member %s", r.Index.Inspect())
		}
		return pair.Value, nil
	}
	return nil, fmt.Errorf("cannot refer into a %s", r.Container.Type())
}

func (r *Reference) setElement(v Object) error {
	switch c := r.Container.(type) {
	case *Array:
		i, err := r.arrayIndex(c)
		if err != nil {
			return err
		}
		c.Elements[i] = v
		return nil
	case *Hash:
		c.Pairs[r.Index.(Hashable).HashKey()] = HashPair{Key: r.Index, Value: v}
		return nil
	}
	return fmt.Errorf("cannot refer into a %s", r.Container.Type())
}

func (r *Reference) arrayIndex(a *Array) (int64, error) {
	i, _, ok := GetIntegerObjectValue(r.Index)
	if !ok {
		return 0, fmt.Errorf("array index must be an integer, got %s", r.Index.Type())
	}
	declared := i
	i -= a.LowerBound
	if i < 0 || i >= int64(len(a.Elements)) {
		return 0, fmt.Errorf("array index out of bounds: %d", declared)
	}
	return i, nil
}

// Owner returns the scope that holds the variable name, this one or an
// outer one, with the spelling it is stored under.
func (e *Environment) Owner(name string) (*Environment, string, bool) {
	for scope := e; scope != nil; scope = scope.outer {
		if stored, ok := scope.key(name); ok {
			return scope, stored, true
		}
	}
	return nil, "", false
}

// NewElementReference returns a reference to element index of an array, or
// to member index of a structure or function block instance.
func NewElementReference(container, index Object) (*Reference, error) {
	r := &Reference{Container: container, Index: index}
	switch c := container.(type) {
	case *Array:
		if _, err := r.arrayIndex(c); err != nil {
			return nil, err
		}
	case *Hash:
		if _, ok := index.(Hashable); !ok {
			return nil, fmt.Errorf("unusable as a member name: %s", index.Type())
		}
	default:
		return nil, fmt.Errorf("REF needs a variable; cannot refer into a %s", container.Type())
	}
	return r, nil
}

// ArrayView returns the array a reference's target value v shows through a
// reference declared to an array type with the lower bounds lower: the same
// elements, indexed from those bounds. Other values are returned as they are.
func ArrayView(v Object, lower []int64) Object {
	a, ok := v.(*Array)
	if !ok || len(lower) == 0 || a.LowerBound == lower[0] {
		return v
	}
	return &Array{Elements: a.Elements, LowerBound: lower[0]}
}

// sameVariable reports whether two non-NULL references refer to the same
// variable.
func sameVariable(a, b *Reference) bool {
	switch {
	case a.Slot != nil || b.Slot != nil:
		return a.Slot == b.Slot
	case a.Container != nil || b.Container != nil:
		if a.Container != b.Container {
			return false
		}
		ah, aok := a.Index.(Hashable)
		bh, bok := b.Index.(Hashable)
		return aok && bok && ah.HashKey() == bh.HashKey()
	case a.IO != nil || b.IO != nil:
		return a.Name == b.Name && b.IO != nil && a.IO != nil
	}
	return a.Env == b.Env && strings.EqualFold(a.Name, b.Name)
}

// isNullReference reports whether v is NULL or a NULL reference.
func isNullReference(v Object) bool {
	switch r := v.(type) {
	case *Null:
		return true
	case *Reference:
		return r.IsNull()
	}
	return false
}

// CompareReferences compares two references, or a reference and NULL, with
// = or <>: they are equal when they refer to the same variable or both to
// nothing. ok is false when neither operand is a reference, or for another
// operator.
func CompareReferences(left Object, operator string, right Object) (result bool, ok bool, err error) {
	_, lRef := left.(*Reference)
	_, rRef := right.(*Reference)
	if !lRef && !rRef {
		return false, false, nil
	}
	var equal bool
	switch {
	case isNullReference(left) || isNullReference(right):
		equal = isNullReference(left) && isNullReference(right)
	case lRef && rRef:
		equal = sameVariable(left.(*Reference), right.(*Reference))
	default:
		return false, true, fmt.Errorf("a reference can be compared only with a reference or NULL, not %s", map[bool]ObjectType{true: right.Type(), false: left.Type()}[lRef])
	}
	switch strings.ToUpper(operator) {
	case "=", "EQ":
		return equal, true, nil
	case "<>", "NE":
		return !equal, true, nil
	}
	return false, true, fmt.Errorf("operator %s is not defined for references; beedance references have no arithmetic", operator)
}

// ReferenceSize is the size of a reference, for SIZEOF: 4 bytes, as a pointer
// is on a 32-bit CODESYS target, which OSCAT assumes when it divides by
// SIZEOF(pt).
const ReferenceSize = 4

// ReferenceTypeKey names a type as references compare it: an array by its
// element type, as a reference to an array indexes it through its own
// bounds; a string without its length; POINTER TO as REF_TO.
func ReferenceTypeKey(t ast.Expression) string {
	switch v := t.(type) {
	case *ast.RefToType:
		return "REF_TO " + ReferenceTypeKey(v.BaseType)
	case *ast.ArrayDefinition:
		return "ARRAY OF " + ReferenceTypeKey(v.DataType)
	case *ast.CallExpression: // STRING(20)
		return ReferenceTypeKey(v.Function)
	case nil:
		return ""
	}
	name := strings.ToUpper(t.String())
	if i := strings.IndexAny(name, "(["); i > 0 {
		name = name[:i]
	}
	switch name {
	case "TOD":
		return "TIME_OF_DAY"
	case "DT":
		return "DATE_AND_TIME"
	}
	return name
}
