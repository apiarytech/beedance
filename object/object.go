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
	"math"
	"strings"
	"time"

	"beedance/ast"
	"beedance/code"
)

// ObjectType is a string that represents the type of an object.
type ObjectType string

// Constants for all the object types in the language.
const (
	SINT_OBJ                    = "SINT"                    // 8-bit signed integer
	INT_OBJ                     = "INT"                     // 16-bit signed integer
	DINT_OBJ                    = "DINT"                    // 32-bit signed integer
	LINT_OBJ                    = "LINT"                    // 64-bit signed integer
	USINT_OBJ                   = "USINT"                   // 8-bit unsigned integer
	UINT_OBJ                    = "UINT"                    // 16-bit unsigned integer
	UDINT_OBJ                   = "UDINT"                   // 32-bit unsigned integer
	ULINT_OBJ                   = "ULINT"                   // 64-bit unsigned integer
	LREAL_OBJ                   = "LREAL"                   // 64-bit floating-point number
	REAL_OBJ                    = "REAL"                    // 32-bit floating-point number
	BOOLEAN_OBJ                 = "BOOLEAN"                 // Boolean value
	NULL_OBJ                    = "NULL"                    // Represents a null or uninitialized value
	RETURN_VALUE_OBJ            = "RETURN_VALUE"            // A wrapper for return values
	EXIT_OBJ                    = "EXIT"                    // A signal to exit a loop
	ERROR_OBJ                   = "ERROR"                   // Represents a runtime error
	FUNCTION_OBJ                = "FUNCTION"                // A user-defined function
	STRING_OBJ                  = "STRING"                  // A single-byte character string
	WSTRING_OBJ                 = "WSTRING"                 // A wide-character string
	BUILTIN_OBJ                 = "BUILTIN"                 // A built-in function
	POINTER_OBJ                 = "POINTER"                 // A reference to another variable (for VAR_IN_OUT)
	BUILTIN_FUNCTION_BLOCK_OBJ  = "BUILTIN_FUNCTION_BLOCK"  // A built-in function block like TON
	ARRAY_OBJ                   = "ARRAY"                   // An array of objects
	HASH_OBJ                    = "HASH"                    // A hash map or dictionary
	TIME_OBJ                    = "TIME"                    // A time duration
	DATE_OBJ                    = "DATE"                    // A calendar date
	LWORD_OBJ                   = "LWORD"                   // 64-bit bit-string
	DWORD_OBJ                   = "DWORD"                   // 32-bit bit-string
	WORD_OBJ                    = "WORD"                    // 16-bit bit-string
	BYTE_OBJ                    = "BYTE"                    // 8-bit bit-string
	TIME_OF_DAY_OBJ             = "TIME_OF_DAY"             // A time of day
	DATE_AND_TIME_OBJ           = "DATE_AND_TIME"           // A specific date and time
	BITSTRING_OBJ               = "BITSTRING"               // A generic bit-string
	QUOTE_OBJ                   = "QUOTE"                   // A quoted AST node (for macros)
	MACRO_OBJ                   = "MACRO"                   // A user-defined macro
	JUMP_OBJ                    = "JUMP"                    // An internal object for IL control flow
	RETURN_OBJ                  = "RETURN"                  // An internal object for IL return
	SFC_OBJ                     = "SFC"                     // A Sequential Function Chart
	STEP_OBJ                    = "STEP"                    // A step within an SFC
	TRANSITION_OBJ              = "TRANSITION"              // A transition within an SFC
	FUNCTION_BLOCK_OBJ          = "FUNCTION_BLOCK"          // The definition of a function block
	FUNCTION_BLOCK_INSTANCE_OBJ = "FUNCTION_BLOCK_INSTANCE" // An instance of a function block
	ENUMERATED_TYPE_OBJ         = "ENUMERATED_TYPE"         // The definition of an enumerated type
	ENUMERATED_VALUE_OBJ        = "ENUMERATED_VALUE"        // A specific value from an enumerated type
	ACTION_OBJ                  = "ACTION"                  // An action within an SFC
	SUBRANGE_TYPE_OBJ           = "SUBRANGE_TYPE"           // The definition of a subrange type
	PROGRAM_OBJ                 = "PROGRAM"                 // The definition of a program
	PROGRAM_INSTANCE_OBJ        = "PROGRAM_INSTANCE"        // An instance of a program
	COMPILED_FUNCTION_OBJ       = "COMPILED_FUNCTION_OBJ"   // A compiled function's bytecode
	STRUCT_DEFINITION_OBJ       = "STRUCT_DEFINITION"       // The definition of a struct
	ENUM_DEFINITION_OBJ         = "ENUM_DEFINITION"         // The definition of an enum
	ARRAY_DEFINITION_OBJ        = "ARRAY_DEFINITION"        // The definition of an array type
	UNCOMPILED_MACRO_OBJ        = "UNCOMPILED_MACRO"        // A macro that has been parsed but not yet expanded
	CLOSURE_OBJ                 = "CLOSURE"                 // A function that captures its environment
	NAMED_ARGUMENT_OBJ          = "NAMED_ARGUMENT"          // A named argument for a function call
	CONSTANT_OBJ                = "CONSTANT"                // A wrapper for a constant value
	SUPER_CONTEXT_OBJ           = "SUPER_CONTEXT"           // A context for a SUPER call
	METHOD_OBJ                  = "METHOD"                  // A method bound to a function block instance
	INTERFACE_DEFINITION_OBJ    = "INTERFACE_DEFINITION"    // The definition of an interface
	NAMESPACE_OBJ               = "NAMESPACE"               // A namespace object
)

// Generic ANY types
const (
	ANY_TYPE       = "ANY"            // The most generic data type
	ANY_ELEMENTARY = "ANY_ELEMENTARY" // Any elementary data type
	ANY_MAGNITUDE  = "ANY_MAGNITUDE"  // Any numeric or time type
	ANY_NUM        = "ANY_NUM"        // Any numeric type (integer or real)
	ANY_INT        = "ANY_INT"        // Any integer type
	ANY_REAL       = "ANY_REAL"       // Any real (floating-point) type
	ANY_BIT        = "ANY_BIT"        // Any bit-string type (BOOL, BYTE, WORD, etc.)
	ANY_STRING     = "ANY_STRING"     // Any string type
	ANY_DATE       = "ANY_DATE"       // Any date or time type
)

// Object is the interface that all value types in the language must implement.
type Object interface {
	// Type returns the type of the object.
	Type() ObjectType
	// Inspect returns a string representation of the object's value.
	Inspect() string
}

// SInt objects store 8-bit signed integers.
type SInt struct {
	Value int8
}

// Type returns the object's type.
func (si *SInt) Type() ObjectType { return SINT_OBJ }

// Inspect returns a string representation of the object's value.
func (si *SInt) Inspect() string { return fmt.Sprintf("%d", si.Value) }

// Int objects store 16-bit signed integers.
type Int struct {
	Value int16
}

// Type returns the object's type.
func (i *Int) Type() ObjectType { return INT_OBJ }

// Inspect returns a string representation of the object's value.
func (i *Int) Inspect() string { return fmt.Sprintf("%d", i.Value) }

// DInt objects store 32-bit signed integers.
type DInt struct {
	Value int32
}

// Type returns the object's type.
func (di *DInt) Type() ObjectType { return DINT_OBJ }

// Inspect returns a string representation of the object's value.
func (di *DInt) Inspect() string { return fmt.Sprintf("%d", di.Value) }

// LInt objects store 64-bit signed integers.
type LInt struct {
	Value int64
}

// Type returns the object's type.
func (li *LInt) Type() ObjectType { return LINT_OBJ }

// Inspect returns a string representation of the object's value.
func (li *LInt) Inspect() string { return fmt.Sprintf("%d", li.Value) }

// USInt objects store 8-bit unsigned integers.
type USInt struct {
	Value uint8
}

// Type returns the object's type.
func (usi *USInt) Type() ObjectType { return USINT_OBJ }

// Inspect returns a string representation of the object's value.
func (usi *USInt) Inspect() string { return fmt.Sprintf("%d", usi.Value) }

// UInt objects store 16-bit unsigned integers.
type UInt struct {
	Value uint16
}

// Type returns the object's type.
func (ui *UInt) Type() ObjectType { return UINT_OBJ }

// Inspect returns a string representation of the object's value.
func (ui *UInt) Inspect() string { return fmt.Sprintf("%d", ui.Value) }

// Real objects store 64-bit floating-point numbers.
type Real struct {
	Value float64
}

// Type returns the object's type.
func (r *Real) Type() ObjectType { return REAL_OBJ }

// Inspect returns a string representation of the object's value.
func (r *Real) Inspect() string { return fmt.Sprintf("%f", r.Value) }

// LReal objects store 64-bit floating-point numbers (long reals).
type LReal struct {
	Value float64
}

// Type returns the object's type.
func (lr *LReal) Type() ObjectType { return LREAL_OBJ }

// Inspect returns a string representation of the object's value.
func (lr *LReal) Inspect() string { return fmt.Sprintf("%f", lr.Value) }

// Boolean objects store boolean values.
type Boolean struct {
	Value bool
}

// Type returns the object's type.
func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }

// Inspect returns a string representation of the object's value.
func (b *Boolean) Inspect() string { return fmt.Sprintf("%t", b.Value) }

// Null objects represent the absence of a value.
type Null struct{}

// Type returns the object's type.
func (n *Null) Type() ObjectType { return NULL_OBJ }

// Inspect returns a string representation of the object's value.
func (n *Null) Inspect() string { return "null" }

// ReturnValue objects wrap other objects to signal a return from a function.
type ReturnValue struct {
	Value Object // The value being returned.
}

// Type returns the object's type.
func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }

// Inspect returns a string representation of the wrapped object's value.
func (rv *ReturnValue) Inspect() string { return rv.Value.Inspect() }

// Exit objects are used to signal an exit from a loop.
type Exit struct{}

// Type returns the object's type.
func (e *Exit) Type() ObjectType { return EXIT_OBJ }

// Inspect returns a string representation of the object.
func (e *Exit) Inspect() string { return "EXIT" }

// It is an internal object used by the evaluator and not exposed to the user.
type Jump struct {
	TargetLabel string
}

// Type returns the object's type.
func (j *Jump) Type() ObjectType { return JUMP_OBJ }

// Inspect returns a string representation of the object.
func (j *Jump) Inspect() string { return "JUMP to " + j.TargetLabel }

// HashKey returns a hash key for the Jump object. Jumps are not hashable.
func (j *Jump) HashKey() HashKey { return HashKey{} } // Not hashable

// Return objects are used to signal a return from an IL program.
type Return struct{}

// Type returns the object's type.
func (r *Return) Type() ObjectType { return RETURN_OBJ }

// Inspect returns a string representation of the object.
func (r *Return) Inspect() string { return "RETURN" }

// HashKey returns a hash key for the Return object. Returns are not hashable.
func (r *Return) HashKey() HashKey { return HashKey{} } // Not hashable

// Error objects store error messages.
type Error struct {
	Message string
}

// Type returns the object's type.
func (e *Error) Type() ObjectType { return ERROR_OBJ }

// Inspect returns a string representation of the error.
func (e *Error) Inspect() string { return "ERROR: " + e.Message }

// String objects store string values.
type String struct {
	Value string
}

// Type returns the object's type.
func (s *String) Type() ObjectType { return STRING_OBJ }

// Inspect returns the string value.
func (s *String) Inspect() string { return s.Value }

// WString objects store wide-character string values.
type WString struct {
	Value string // Internally represented as a Go string, but treated as wide characters.
}

// Type returns the object's type.
func (ws *WString) Type() ObjectType { return WSTRING_OBJ }

// Inspect returns the string value.
func (ws *WString) Inspect() string { return ws.Value }

// BuiltinFunction is the type for a built-in function.
type BuiltinFunction func(args ...Object) Object

// Builtin objects represent built-in functions.
type Builtin struct {
	Fn BuiltinFunction
}

// Type returns the object's type.
func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }

// Inspect returns a generic string for built-in functions.
func (b *Builtin) Inspect() string { return "builtin function" }

// BuiltinFunctionBlockFunction is the type for a built-in function block's execution logic.
// It receives the instance's environment and the calling environment.
type BuiltinFunctionBlockFunction func(instanceEnv, callEnv *Environment) Object

// BuiltinFunctionBlock represents a pre-defined, stateful function block like TON or CTU.
type BuiltinFunctionBlock struct {
	Fn BuiltinFunctionBlockFunction
}

// Type returns the object's type.
func (bfb *BuiltinFunctionBlock) Type() ObjectType { return BUILTIN_FUNCTION_BLOCK_OBJ }

// Inspect returns a generic string for built-in function blocks.
func (bfb *BuiltinFunctionBlock) Inspect() string { return "builtin function block" }

// Pointer is an object that holds a reference to another variable in an environment.
// This is the mechanism for implementing VAR_IN_OUT (pass-by-reference).
type Pointer struct {
	Name string       // The name of the variable in the Env.
	Env  *Environment // The environment where the variable is stored.
}

// Type returns the object's type.
func (p *Pointer) Type() ObjectType { return POINTER_OBJ }

// Inspect returns a string representation of the pointer.
func (p *Pointer) Inspect() string {
	return fmt.Sprintf("POINTER(%s)", p.Name)
}

// Array objects store a slice of other objects.
type Array struct {
	Elements []Object
	// LowerBound is the index of the first element, as declared, e.g. 1 for
	// ARRAY[1..3]. Indexing subtracts it.
	LowerBound int64
}

// Type returns the object's type.
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

// HashKey represents the key used in a hash map, combining the object's type and a hash of its value.
type HashKey struct {
	Type  ObjectType
	Value uint64
}

// Hashable is an interface for objects that can be used as keys in a hash map.
type Hashable interface {
	HashKey() HashKey
}

// HashKey returns a hash key for an SInt object.
func (si *SInt) HashKey() HashKey { return HashKey{Type: si.Type(), Value: uint64(si.Value)} }

// HashKey returns a hash key for an Int object.
func (i *Int) HashKey() HashKey { return HashKey{Type: i.Type(), Value: uint64(i.Value)} }

// HashKey returns a hash key for a DInt object.
func (di *DInt) HashKey() HashKey { return HashKey{Type: di.Type(), Value: uint64(di.Value)} }

// HashKey returns a hash key for an LInt object.
func (li *LInt) HashKey() HashKey { return HashKey{Type: li.Type(), Value: uint64(li.Value)} }

// HashKey returns a hash key for a USInt object.
func (usi *USInt) HashKey() HashKey { return HashKey{Type: usi.Type(), Value: uint64(usi.Value)} }

// HashKey returns a hash key for a UInt object.
func (ui *UInt) HashKey() HashKey { return HashKey{Type: ui.Type(), Value: uint64(ui.Value)} }

// UDInt objects store 32-bit unsigned integers.
type UDInt struct{ Value uint32 }

// Type returns the object's type.
func (udi *UDInt) Type() ObjectType { return UDINT_OBJ }

// Inspect returns a string representation of the object's value.
func (udi *UDInt) Inspect() string { return fmt.Sprintf("%d", udi.Value) }

// HashKey returns a hash key for a UDInt object.
func (udi *UDInt) HashKey() HashKey { return HashKey{Type: udi.Type(), Value: uint64(udi.Value)} }

func (b *Boolean) HashKey() HashKey {
	var value uint64
	if b.Value {
		value = 1
	} else {
		value = 0
	}
	return HashKey{Type: b.Type(), Value: value}
}

// ULInt objects store 64-bit unsigned integers.
type ULInt struct{ Value uint64 }

// Type returns the object's type.
func (uli *ULInt) Type() ObjectType { return ULINT_OBJ }

// Inspect returns a string representation of the object's value.
func (uli *ULInt) Inspect() string { return fmt.Sprintf("%d", uli.Value) }

// HashKey returns a hash key for a ULInt object.
func (uli *ULInt) HashKey() HashKey { return HashKey{Type: uli.Type(), Value: uli.Value} }

func (r *Real) HashKey() HashKey {
	return HashKey{Type: r.Type(), Value: math.Float64bits(r.Value)}
}

func (lr *LReal) HashKey() HashKey {
	return HashKey{Type: lr.Type(), Value: math.Float64bits(lr.Value)}
}

func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))
	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

func (ws *WString) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(ws.Value)) // Note: Hashing is based on byte representation.
	return HashKey{Type: ws.Type(), Value: h.Sum64()}
}

// EnumeratedType represents the definition of an enumerated type.
type EnumeratedType struct {
	Name   string
	Values map[string]*EnumeratedValue
}

// Type returns the object's type.
func (et *EnumeratedType) Type() ObjectType { return ENUMERATED_TYPE_OBJ }
func (et *EnumeratedType) Inspect() string {
	var out bytes.Buffer
	vals := []string{}
	for valName := range et.Values {
		vals = append(vals, valName)
	}
	out.WriteString(fmt.Sprintf("ENUM(%s: %s)", et.Name, strings.Join(vals, ", ")))
	return out.String()
}

// SubrangeType represents the definition of a subrange type.
type SubrangeType struct {
	Name       string // The name of the subrange type.
	BaseType   ObjectType
	LowerBound int64
	UpperBound int64
}

// Type returns the object's type.
func (st *SubrangeType) Type() ObjectType { return SUBRANGE_TYPE_OBJ }
func (st *SubrangeType) Inspect() string {
	return fmt.Sprintf("SUBRANGE %s (%d..%d)", st.BaseType, st.LowerBound, st.UpperBound)
}

// EnumeratedValue represents a single value from an enumerated type.
type EnumeratedValue struct {
	TypeName string
	Value    string
}

// Type returns the object's type.
func (ev *EnumeratedValue) Type() ObjectType { return ENUMERATED_VALUE_OBJ }

// Inspect returns a string representation of the enumerated value (e.g., "COLOR#RED").
func (ev *EnumeratedValue) Inspect() string { return fmt.Sprintf("%s#%s", ev.TypeName, ev.Value) }

// HashKey returns a hash key for an EnumeratedValue object.
func (ev *EnumeratedValue) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(ev.TypeName + "#" + ev.Value))
	return HashKey{Type: ev.Type(), Value: h.Sum64()}
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

// Type returns the object's type.
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

// Type returns the object's type.
func (t *Time) Type() ObjectType { return TIME_OBJ }

// Inspect returns a string representation of the time duration (e.g., "T#5s").
func (t *Time) Inspect() string { return fmt.Sprintf("T#%s", t.Value.String()) }

// Date objects store time.Time values (date only).
type Date struct {
	Value time.Time
}

// Type returns the object's type.
func (d *Date) Type() ObjectType { return DATE_OBJ }

// Inspect returns a string representation of the date (e.g., "D#2026-01-02").
func (d *Date) Inspect() string { return fmt.Sprintf("D#%s", d.Value.Format("2006-01-02")) }

// TimeOfDay objects store time.Time values (time of day only).
type TimeOfDay struct {
	Value time.Time
}

// Type returns the object's type.
func (tod *TimeOfDay) Type() ObjectType { return TIME_OF_DAY_OBJ }

// Inspect returns a string representation of the time of day (e.g., "TOD#15:04:05.999").
func (tod *TimeOfDay) Inspect() string {
	return fmt.Sprintf("TOD#%s", tod.Value.Format("15:04:05.999"))
}

// DateAndTime objects store time.Time values (date and time).
type DateAndTime struct {
	Value time.Time
}

// Type returns the object's type.
func (dt *DateAndTime) Type() ObjectType { return DATE_AND_TIME_OBJ }
func (dt *DateAndTime) Inspect() string {
	return fmt.Sprintf("DT#%s", dt.Value.Format("2006-01-02-15:04:05.999"))
}

// BitString objects store unsigned integers with a specified width.
type BitString struct {
	Value uint64
	Width int // 8, 16, 32, 64
}

// Type returns the object's type.
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

// Byte objects store 8-bit unsigned integers.
type Byte struct {
	Value byte
}

// Type returns the object's type.
func (b *Byte) Type() ObjectType { return BYTE_OBJ }

// Inspect returns a hexadecimal string representation of the byte.
func (b *Byte) Inspect() string { return fmt.Sprintf("BYTE#16#%X", b.Value) }

// HashKey returns a hash key for a Byte object.
func (b *Byte) HashKey() HashKey { return HashKey{Type: b.Type(), Value: uint64(b.Value)} }

// Word objects store 16-bit unsigned integers.
type Word struct {
	Value uint16
}

// Type returns the object's type.
func (w *Word) Type() ObjectType { return WORD_OBJ }

// Inspect returns a hexadecimal string representation of the word.
func (w *Word) Inspect() string { return fmt.Sprintf("WORD#16#%X", w.Value) }

// HashKey returns a hash key for a Word object.
func (w *Word) HashKey() HashKey { return HashKey{Type: w.Type(), Value: uint64(w.Value)} }

// DWord objects store 32-bit unsigned integers.
type DWord struct {
	Value uint32
}

// Type returns the object's type.
func (dw *DWord) Type() ObjectType { return DWORD_OBJ }

// Inspect returns a hexadecimal string representation of the double word.
func (dw *DWord) Inspect() string { return fmt.Sprintf("DWORD#16#%X", dw.Value) }

// HashKey returns a hash key for a DWord object.
func (dw *DWord) HashKey() HashKey { return HashKey{Type: dw.Type(), Value: uint64(dw.Value)} }

// LWord objects store 64-bit unsigned integers.
type LWord struct {
	Value uint64
}

// Type returns the object's type.
func (lw *LWord) Type() ObjectType { return LWORD_OBJ }

// Inspect returns a hexadecimal string representation of the long word.
func (lw *LWord) Inspect() string { return fmt.Sprintf("LWORD#16#%X", lw.Value) }

// HashKey returns a hash key for an LWord object.
func (lw *LWord) HashKey() HashKey { return HashKey{Type: lw.Type(), Value: lw.Value} }

// Quote objects wrap an AST node.
type Quote struct {
	Node ast.Node
}

// Type returns the object's type.
func (q *Quote) Type() ObjectType { return QUOTE_OBJ }

// Inspect returns a string representation of the quoted node.
func (q *Quote) Inspect() string { return "QUOTE(" + q.Node.String() + ")" }

// Macro objects represent user-defined macros.
type Macro struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

// Type returns the object's type.
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
	Actions         map[string]*Action // Map of action name to action object
	ActiveSteps     map[string]bool    // Map of step name to active status
}

// Type returns the object's type.
func (s *SFC) Type() ObjectType { return SFC_OBJ }
func (s *SFC) Inspect() string  { return "SFC" }

// Step objects represent a step in an SFC.
type Step struct {
	Name           *ast.Identifier
	Body           *ast.BlockStatement // The action calls or ST statements within the step
	IsActive       bool
	ActivationTime time.Time // Time when the step became active
}

// ActionAssociation links an action to a step with a specific qualifier.
type ActionAssociation struct {
	Name      string
	Qualifier string
	Duration  time.Duration // For timed qualifiers like L, D
}

// Type returns the object's type.
func (s *Step) Type() ObjectType { return STEP_OBJ }

// Inspect returns a string representation of the step.
func (s *Step) Inspect() string { return "STEP " + s.Name.Value }

// Transition objects represent a transition in an SFC.
type Transition struct {
	FromSteps []*ast.Identifier
	ToSteps   []*ast.Identifier
	Condition ast.Expression
}

// Type returns the object's type.
func (t *Transition) Type() ObjectType { return TRANSITION_OBJ }
func (t *Transition) Inspect() string  { return "TRANSITION" }

// Action represents a declared action within an SFC.
type Action struct {
	Name *ast.Identifier
	Body *ast.BlockStatement

	// State for action control logic
	IsActive        bool // The 'Q' flag from the ACTION_CONTROL block
	ActivationCount int  // For handling 'P' (Pulse) qualifier
	AssociatedSteps []*Step
	Qualifier       string // This will now be determined dynamically per cycle
	Duration        time.Duration
	TimerStart      time.Time // When the timer for D, L, etc. started
}

// Type returns the object's type.
func (a *Action) Type() ObjectType { return ACTION_OBJ }
func (a *Action) Inspect() string  { return "ACTION " + a.Name.Value }

// IsStored checks if the action has a stored qualifier (S, SD, SL).
// This is a simplification; a more robust implementation would check the qualifier
// from the AST for each associated step. For this evaluator's logic, checking
// the dynamically determined qualifier is sufficient.
func (a *Action) IsStored() bool {
	// This check is based on the dynamically determined qualifier for the current cycle.
	// It's a pragmatic approach for the evaluator's state machine.
	return a.Qualifier == "S" || a.Qualifier == "SD" || a.Qualifier == "SL" || a.Qualifier == "DS"
}

// Task represents a runtime task with its configuration and state.
type Task struct {
	Name     string
	Priority int64
	Trigger  ast.Expression // The 'SINGLE' condition
	Interval time.Duration
	Programs []*ProgramInstance

	// Runtime state
	LastExecution    time.Time
	LastTriggerValue bool // To detect rising edge for SINGLE
	RunChannel       chan bool
}

// Type returns the object's type.
func (t *Task) Type() ObjectType { return "TASK" } // Custom type for tasks
// Inspect returns a string representation of the task.
func (t *Task) Inspect() string {
	return fmt.Sprintf("TASK(%s, Priority: %d, Interval: %s)", t.Name, t.Priority, t.Interval)
}

// Scheduler manages all tasks and program instances within a resource.
type Scheduler struct {
	Tasks []*Task
}

// Type returns the object's type.
func (s *Scheduler) Type() ObjectType { return "SCHEDULER" } // Custom type for scheduler
// Inspect returns a string representation of the scheduler and its tasks.
func (s *Scheduler) Inspect() string {
	var out bytes.Buffer
	tasks := []string{}
	for _, t := range s.Tasks {
		tasks = append(tasks, t.Inspect())
	}
	out.WriteString(fmt.Sprintf("SCHEDULER(%s)", strings.Join(tasks, ", ")))
	return out.String()
}

// Function objects represent user-defined functions.
type Function struct {
	Name       *ast.Identifier // Name of the function (nil for anonymous functions)
	VarInputs  []*ast.VarDeclStatement
	VarOutputs []*ast.VarDeclStatement
	VarInOuts  []*ast.VarDeclStatement
	Vars       []*ast.VarDeclStatement
	ReturnType ast.Expression // The declared return type, or nil; the result starts at its default.
	Body       ast.Statement
	Env        *Environment
}

// Type returns the object's type.
func (f *Function) Type() ObjectType { return FUNCTION_OBJ }

// Inspect returns a string representation of the function's signature.
func (f *Function) Inspect() string {
	var out bytes.Buffer

	out.WriteString("FUNCTION ")
	out.WriteString(f.Name.Value)
	out.WriteString(" (")

	var allParams []string
	if len(f.VarInputs) > 0 {
		params := []string{}
		for _, p := range f.VarInputs {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_INPUT "+strings.Join(params, "; ")+";")
	}
	if len(f.VarOutputs) > 0 {
		params := []string{}
		for _, p := range f.VarOutputs {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_OUTPUT "+strings.Join(params, "; ")+";")
	}
	if len(f.VarInOuts) > 0 {
		params := []string{}
		for _, p := range f.VarInOuts {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_IN_OUT "+strings.Join(params, "; ")+";")
	}

	out.WriteString(strings.Join(allParams, " "))

	out.WriteString(")")

	return out.String()
}

// FunctionBlock represents the definition of a function block.
// It's like a class blueprint.
type FunctionBlock struct {
	Body        ast.Statement
	Env         *Environment
	Name        *ast.Identifier // The name of the function block
	VarInputs   []*ast.VarDeclStatement
	VarOutputs  []*ast.VarDeclStatement
	VarInOuts   []*ast.VarDeclStatement
	Vars        []*ast.VarDeclStatement
	VarTemp     []*ast.TempVarDeclaration
	VarExternal []*ast.ExternalVarDeclaration
	Definition  *ast.FunctionBlockDeclaration
}

// Type returns the object's type.
func (fb *FunctionBlock) Type() ObjectType { return FUNCTION_BLOCK_OBJ }
func (fb *FunctionBlock) Inspect() string {
	var out bytes.Buffer

	out.WriteString("FUNCTION_BLOCK ")
	out.WriteString(fb.Name.Value)
	out.WriteString(" (")

	var allParams []string
	if len(fb.VarInputs) > 0 {
		params := []string{}
		for _, p := range fb.VarInputs {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_INPUT "+strings.Join(params, "; ")+";")
	}
	if len(fb.VarOutputs) > 0 {
		params := []string{}
		for _, p := range fb.VarOutputs {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_OUTPUT "+strings.Join(params, "; ")+";")
	}
	if len(fb.VarInOuts) > 0 {
		params := []string{}
		for _, p := range fb.VarInOuts {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_IN_OUT "+strings.Join(params, "; ")+";")
	}
	if len(fb.VarTemp) > 0 {
		params := []string{}
		for _, block := range fb.VarTemp {
			for _, v := range block.Vars {
				params = append(params, v.Name.String()+" : "+v.DataType.String())
			}
		}
		allParams = append(allParams, "VAR_TEMP "+strings.Join(params, "; ")+";")
	}
	if len(fb.VarExternal) > 0 {
		params := []string{}
		for _, block := range fb.VarExternal {
			for _, v := range block.Vars {
				params = append(params, v.Name.String()+" : "+v.DataType.String())
			}
		}
		allParams = append(allParams, "VAR_EXTERNAL "+strings.Join(params, "; ")+";")
	}
	out.WriteString(strings.Join(allParams, " "))

	out.WriteString(")")

	return out.String()
}

// FunctionBlockInstance represents an instantiated function block.
type FunctionBlockInstance struct {
	Definition *FunctionBlock
	Env        *Environment // Environment specific to this instance
}

// Type returns the object's type.
func (fbi *FunctionBlockInstance) Type() ObjectType { return FUNCTION_BLOCK_INSTANCE_OBJ }

// Inspect returns a string representation of the function block instance.
func (fbi *FunctionBlockInstance) Inspect() string {
	return fmt.Sprintf("FUNCTION_BLOCK_INSTANCE(%s)", fbi.Definition.Name.Value)
}

// Method represents a method bound to a specific function block instance.
// It holds the method's definition (from the AST) and a reference to the
// instance it's being called on, which provides the context (the instance's environment).
type Method struct {
	Definition *ast.MethodImplementation
	Instance   *FunctionBlockInstance
}

// Type returns the object's type.
func (m *Method) Type() ObjectType { return METHOD_OBJ }

// Inspect returns a string representation of the method.
func (m *Method) Inspect() string {
	if m.Definition != nil && m.Definition.Name != nil {
		return fmt.Sprintf("METHOD(%s)", m.Definition.Name.Value)
	}
	return "METHOD(<unnamed>)"
}

// SuperContext represents the context for a SUPER call, holding a reference
// to the current function block instance.
type SuperContext struct {
	Instance *FunctionBlockInstance
}

// Type returns the object's type.
func (sc *SuperContext) Type() ObjectType { return SUPER_CONTEXT_OBJ }

// Inspect returns a string representation of the super context.
func (sc *SuperContext) Inspect() string {
	return fmt.Sprintf("SUPER_CONTEXT(%s)", sc.Instance.Inspect())
}

// InterfaceDefinition represents the definition of an INTERFACE POU.
type InterfaceDefinition struct {
	Name       *ast.Identifier
	Methods    []*ast.MethodDeclaration
	Properties []*ast.PropertyDeclaration
}

// Type returns the object's type.
func (id *InterfaceDefinition) Type() ObjectType { return INTERFACE_DEFINITION_OBJ }

// Inspect returns a string representation of the interface definition.
func (id *InterfaceDefinition) Inspect() string {
	return fmt.Sprintf("INTERFACE %s", id.Name.Value)
}

// Namespace represents a container for other POUs and definitions.
type Namespace struct {
	Name string
	Env  *Environment
}

// Type returns the object's type.
func (ns *Namespace) Type() ObjectType { return NAMESPACE_OBJ }

// Inspect returns a string representation of the namespace.
func (ns *Namespace) Inspect() string {
	return fmt.Sprintf("NAMESPACE(%s)", ns.Name)
}

// Program represents the definition of a PROGRAM POU.
// It's a template for creating program instances.
type Program struct {
	Name        *ast.Identifier
	VarInputs   []*ast.VarDeclStatement
	VarOutputs  []*ast.VarDeclStatement
	VarInOuts   []*ast.VarDeclStatement
	Vars        []*ast.VarDeclStatement
	VarTemp     []*ast.TempVarDeclaration
	VarExternal []*ast.ExternalVarDeclaration
	VarGlobal   []*ast.GlobalVarDeclaration
	VarAccess   []*ast.AccessVarDeclaration
	Body        ast.Statement
	Env         *Environment
}

// Type returns the object's type.
func (p *Program) Type() ObjectType { return PROGRAM_OBJ }

// Inspect returns a string representation of the program's signature.
func (p *Program) Inspect() string {
	var out bytes.Buffer

	out.WriteString("PROGRAM ")
	out.WriteString(p.Name.Value)
	out.WriteString(" (")

	var allParams []string
	if len(p.VarInputs) > 0 {
		params := []string{}
		for _, p := range p.VarInputs {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_INPUT "+strings.Join(params, "; ")+";")
	}
	if len(p.VarOutputs) > 0 {
		params := []string{}
		for _, p := range p.VarOutputs {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_OUTPUT "+strings.Join(params, "; ")+";")
	}
	if len(p.VarInOuts) > 0 {
		params := []string{}
		for _, p := range p.VarInOuts {
			params = append(params, p.Name.String()+" : "+p.DataType.String())
		}
		allParams = append(allParams, "VAR_IN_OUT "+strings.Join(params, "; ")+";")
	}
	if len(p.VarTemp) > 0 {
		params := []string{}
		for _, block := range p.VarTemp {
			for _, v := range block.Vars {
				params = append(params, v.Name.String()+" : "+v.DataType.String())
			}
		}
		allParams = append(allParams, "VAR_TEMP "+strings.Join(params, "; ")+";")
	}
	if len(p.VarExternal) > 0 {
		params := []string{}
		for _, block := range p.VarExternal {
			for _, v := range block.Vars {
				params = append(params, v.Name.String()+" : "+v.DataType.String())
			}
		}
		allParams = append(allParams, "VAR_EXTERNAL "+strings.Join(params, "; ")+";")
	}
	if len(p.VarGlobal) > 0 {
		params := []string{}
		for _, block := range p.VarGlobal {
			for _, v := range block.Vars {
				params = append(params, v.Name.String()+" : "+v.DataType.String())
			}
		}
		allParams = append(allParams, "VAR_GLOBAL "+strings.Join(params, "; ")+";")
	}
	if len(p.VarAccess) > 0 {
		params := []string{}
		for _, block := range p.VarAccess {
			for _, v := range block.Vars {
				params = append(params, v.Name.String()+" : "+v.DataType.String())
			}
		}
		allParams = append(allParams, "VAR_ACCESS "+strings.Join(params, "; ")+";")
	}
	out.WriteString(strings.Join(allParams, " "))

	out.WriteString(")")

	return out.String()

}

// OutputMapping stores the `=>` mapping for a program instance's output.
type OutputMapping struct {
	SourceParamName string
	TargetVarName   string
}

// ProgramInstance represents a configured instance of a PROGRAM type.
type ProgramInstance struct {
	Definition     *Program
	Env            *Environment
	TaskName       string
	OutputMappings []OutputMapping
}

// Type returns the object's type.
func (pi *ProgramInstance) Type() ObjectType { return PROGRAM_INSTANCE_OBJ }

// Inspect returns a string representation of the program instance.
func (pi *ProgramInstance) Inspect() string {
	if pi.Definition != nil && pi.Definition.Name != nil {
		return fmt.Sprintf("INSTANCE OF %s", pi.Definition.Name.Value)
	}
	return "PROGRAM_INSTANCE"
}

// CompiledFunction holds the bytecode and metadata for a compiled function.
type CompiledFunction struct {
	Instructions   code.Instructions
	NumLocals      int
	NumParameters  int
	ParameterNames []string
	OutputIndices  []int
	OutputNames    []string
}

// Type returns the object's type.
func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }

// Inspect returns a string representation of the compiled function.
func (cf *CompiledFunction) Inspect() string {
	return fmt.Sprintf("CompiledFunction[%p]", cf)
}

// Closure wraps a compiled function and any free variables it captures from its enclosing scope.
type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

// Type returns the object's type.
func (c *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (c *Closure) Inspect() string {
	return fmt.Sprintf("Closure[%p]", c)
}

// Constant wraps another object to make it immutable.
type Constant struct {
	Value Object
}

// Type returns the object's type.
func (c *Constant) Type() ObjectType { return CONSTANT_OBJ }

// Inspect returns a string representation of the wrapped object's value.
func (c *Constant) Inspect() string { return c.Value.Inspect() }

// NamedArgument wraps a value with a parameter name for function calls.
type NamedArgument struct {
	Name  string
	Value Object
}

// Type returns the object's type.
func (na *NamedArgument) Type() ObjectType { return NAMED_ARGUMENT_OBJ }
func (na *NamedArgument) Inspect() string {
	return fmt.Sprintf("%s := %s", na.Name, na.Value.Inspect())
}

// UncompiledMacro represents a macro that has been parsed but whose body has not been compiled.
// The compiler stores this object, and at macro call time, it will expand the body AST
// and compile the result.
type UncompiledMacro struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
}

// Type returns the object's type.
func (um *UncompiledMacro) Type() ObjectType { return UNCOMPILED_MACRO_OBJ }

// Inspect returns a string representation of the uncompiled macro.
func (um *UncompiledMacro) Inspect() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range um.Parameters {
		params = append(params, p.String())
	}
	out.WriteString("macro(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(um.Body.String())
	out.WriteString("\n}")
	return out.String()
}

// StructDefinition represents the definition of a STRUCT type as a runtime object.
type StructDefinition struct {
	Name    *ast.Identifier
	Members []*ast.VarDeclStatement
}

// Type returns the object's type.
func (sd *StructDefinition) Type() ObjectType { return STRUCT_DEFINITION_OBJ }

// Inspect returns a string representation of the struct definition.
func (sd *StructDefinition) Inspect() string {
	var out bytes.Buffer
	out.WriteString("TYPE ")
	out.WriteString(sd.Name.String())
	out.WriteString(" : STRUCT\n")
	for _, m := range sd.Members {
		out.WriteString("\t")
		out.WriteString(m.String())
		out.WriteString("\n")
	}
	out.WriteString("END_STRUCT")
	return out.String()
}

// EnumDefinition represents the definition of an enumerated type as a runtime object.
type EnumDefinition struct {
	Name   *ast.Identifier
	Values []*ast.Identifier
}

// Type returns the object's type.
func (ed *EnumDefinition) Type() ObjectType { return ENUM_DEFINITION_OBJ }

// Inspect returns a string representation of the enum definition.
func (ed *EnumDefinition) Inspect() string {
	var out bytes.Buffer
	out.WriteString("TYPE ")
	out.WriteString(ed.Name.String())
	out.WriteString(" : (")
	vals := []string{}
	for _, v := range ed.Values {
		vals = append(vals, v.String())
	}
	out.WriteString(strings.Join(vals, ", "))
	out.WriteString(");")
	return out.String()
}

// ArrayDefinition represents the definition of an ARRAY type as a runtime object.
type ArrayDefinition struct {
	Name     *ast.Identifier
	Ranges   []ast.Expression
	DataType ast.Expression
}

// Type returns the object's type.
func (ad *ArrayDefinition) Type() ObjectType { return ARRAY_DEFINITION_OBJ }

// Inspect returns a string representation of the array definition.
func (ad *ArrayDefinition) Inspect() string {
	var out bytes.Buffer
	out.WriteString("TYPE ")
	out.WriteString(ad.Name.String())
	out.WriteString(" : ARRAY [")
	ranges := []string{}
	for _, r := range ad.Ranges {
		ranges = append(ranges, r.String())
	}
	out.WriteString(strings.Join(ranges, ", "))
	out.WriteString("] OF ")
	out.WriteString(ad.DataType.String())
	out.WriteString(";")
	return out.String()
}
