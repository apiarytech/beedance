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
	"bytes"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"beedance/ast"
)

type ObjectType string

const (
	INTEGER_OBJ                 = "INTEGER"
	REAL_OBJ                    = "REAL"
	BOOLEAN_OBJ                 = "BOOLEAN"
	NULL_OBJ                    = "NULL"
	RETURN_VALUE_OBJ            = "RETURN_VALUE"
	ERROR_OBJ                   = "ERROR"
	FUNCTION_OBJ                = "FUNCTION"
	STRING_OBJ                  = "STRING"
	BUILTIN_OBJ                 = "BUILTIN"
	ARRAY_OBJ                   = "ARRAY"
	HASH_OBJ                    = "HASH"
	TIME_OBJ                    = "TIME"
	DATE_OBJ                    = "DATE"
	TIME_OF_DAY_OBJ             = "TIME_OF_DAY"
	DATE_AND_TIME_OBJ           = "DATE_AND_TIME"
	BITSTRING_OBJ               = "BITSTRING"
	QUOTE_OBJ                   = "QUOTE"
	MACRO_OBJ                   = "MACRO"
	SFC_OBJ                     = "SFC"
	STEP_OBJ                    = "STEP"
	TRANSITION_OBJ              = "TRANSITION"
	FUNCTION_BLOCK_INSTANCE_OBJ = "FUNCTION_BLOCK_INSTANCE"
)

// Object is the interface that all objects in the Monkey language must implement.
type Object interface {
	Type() ObjectType
	Inspect() string
}

// FunctionBlock represents the definition of a function block.
// It's like a class blueprint.
type FunctionBlock struct {
	Body       *ast.BlockStatement
	Env        *Environment
	Name       *ast.Identifier // The name of the function block
	VarInputs  []*ast.VarDeclStatement
	VarOutputs []*ast.VarDeclStatement
	VarInOuts  []*ast.VarDeclStatement
	Vars       []*ast.VarDeclStatement
}

// Integer objects store 64-bit integers.
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

// Real objects store 64-bit floating-point numbers.
type Real struct {
	Value float64
}

func (r *Real) Type() ObjectType { return REAL_OBJ }
func (r *Real) Inspect() string  { return fmt.Sprintf("%f", r.Value) }

// Boolean objects store boolean values.
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

// Null objects represent the absence of a value.
type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

// ReturnValue objects wrap other objects to signal a return from a function.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// Error objects store error messages.
type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

// String objects store string values.
type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

// BuiltinFunction is the type for a built-in function.
type BuiltinFunction func(args ...Object) Object

// Builtin objects represent built-in functions.
type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

// Array objects store a slice of other objects.
type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	var out bytes.Buffer
	elements := []string{}
	for _, e := range ao.Elements {
		elements = append(elements, e.Inspect())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")
	return out.String()
}

// HashKey is a struct used as a key in a hash map.
type HashKey struct {
	Type  ObjectType
	Value uint64
}

// Hashable is an interface for objects that can be used as hash keys.
type Hashable interface {
	HashKey() HashKey
}

// HashKey implementations for Integer, Boolean, String.
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

func (b *Boolean) HashKey() HashKey {
	var value uint64
	if b.Value {
		value = 1
	} else {
		value = 0
	}
	return HashKey{Type: b.Type(), Value: value}
}

func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))
	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

// HashPair stores a key-value pair in a hash map.
type HashPair struct {
	Key   Object
	Value Object
}

// Hash objects store a map of HashKey to HashPair.
type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var out bytes.Buffer
	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s",
			pair.Key.Inspect(), pair.Value.Inspect()))
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

// Time objects store time.Duration values.
type Time struct {
	Value time.Duration
}

func (t *Time) Type() ObjectType { return TIME_OBJ }
func (t *Time) Inspect() string  { return fmt.Sprintf("T#%s", t.Value.String()) }

// Date objects store time.Time values (date only).
type Date struct {
	Value time.Time
}

func (d *Date) Type() ObjectType { return DATE_OBJ }
func (d *Date) Inspect() string  { return fmt.Sprintf("D#%s", d.Value.Format("2006-01-02")) }

// TimeOfDay objects store time.Time values (time of day only).
type TimeOfDay struct {
	Value time.Time
}

func (tod *TimeOfDay) Type() ObjectType { return TIME_OF_DAY_OBJ }
func (tod *TimeOfDay) Inspect() string {
	return fmt.Sprintf("TOD#%s", tod.Value.Format("15:04:05.999"))
}

// DateAndTime objects store time.Time values (date and time).
type DateAndTime struct {
	Value time.Time
}

func (dt *DateAndTime) Type() ObjectType { return DATE_AND_TIME_OBJ }
func (dt *DateAndTime) Inspect() string {
	return fmt.Sprintf("DT#%s", dt.Value.Format("2006-01-02-15:04:05.999"))
}

// BitString objects store unsigned integers with a specified width.
type BitString struct {
	Value uint64
	Width int // 8, 16, 32, 64
}

func (bs *BitString) Type() ObjectType { return BITSTRING_OBJ }
func (bs *BitString) Inspect() string {
	switch bs.Width {
	case 8:
		return fmt.Sprintf("BYTE#16#%X", bs.Value)
	case 16:
		return fmt.Sprintf("WORD#16#%X", bs.Value)
	case 32:
		return fmt.Sprintf("DWORD#16#%X", bs.Value)
	case 64:
		return fmt.Sprintf("LWORD#16#%X", bs.Value)
	default:
		return fmt.Sprintf("BITSTRING#%d#%X", bs.Width, bs.Value)
	}
}

// Quote objects wrap an AST node.
type Quote struct {
	Node ast.Node
}

func (q *Quote) Type() ObjectType { return QUOTE_OBJ }
func (q *Quote) Inspect() string  { return "QUOTE(" + q.Node.String() + ")" }

// Macro objects represent user-defined macros.
type Macro struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (m *Macro) Type() ObjectType { return MACRO_OBJ }
func (m *Macro) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range m.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("macro(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(m.Body.String())
	out.WriteString("\n}")

	return out.String()
}

// SFC objects represent the state of a Sequential Function Chart.
type SFC struct {
	Steps           map[string]*Step
	Transitions     []*Transition
	InitialStepName string
	ActiveSteps     map[string]bool // Map of step name to active status
}

func (s *SFC) Type() ObjectType { return SFC_OBJ }
func (s *SFC) Inspect() string  { return "SFC" }

// Step objects represent a step in an SFC.
type Step struct {
	Name     *ast.Identifier
	Actions  []*ast.ActionBlockStatement
	IsActive bool
}

func (s *Step) Type() ObjectType { return STEP_OBJ }
func (s *Step) Inspect() string  { return "STEP " + s.Name.Value }

// Transition objects represent a transition in an SFC.
type Transition struct {
	FromSteps []*ast.Identifier
	ToSteps   []*ast.Identifier
	Condition ast.Expression
}

func (t *Transition) Type() ObjectType { return TRANSITION_OBJ }
func (t *Transition) Inspect() string  { return "TRANSITION" }

// Function objects represent user-defined functions.
type Function struct {
	Name       *ast.Identifier // Name of the function (nil for anonymous functions)
	VarInputs  []*ast.VarDeclStatement
	VarOutputs []*ast.VarDeclStatement
	VarInOuts  []*ast.VarDeclStatement
	Vars       []*ast.VarDeclStatement
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	// Simplified inspect for now
	return fmt.Sprintf("FUNCTION(%s)", f.Name.Value)
}

// FunctionBlockInstance represents an instantiated function block.
type FunctionBlockInstance struct {
	Definition *FunctionBlock
	Env        *Environment // Environment specific to this instance
}

func (fbi *FunctionBlockInstance) Type() ObjectType { return FUNCTION_BLOCK_INSTANCE_OBJ }
func (fbi *FunctionBlockInstance) Inspect() string {
	return fmt.Sprintf("FUNCTION_BLOCK_INSTANCE(%s)", fbi.Definition.Name.Value)
}
