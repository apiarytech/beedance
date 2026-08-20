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

type ObjectType string

const (
	SINT_OBJ                    = "SINT"
	INT_OBJ                     = "INT"
	DINT_OBJ                    = "DINT"
	LINT_OBJ                    = "LINT"
	USINT_OBJ                   = "USINT"
	UINT_OBJ                    = "UINT"
	UDINT_OBJ                   = "UDINT"
	ULINT_OBJ                   = "ULINT"
	LREAL_OBJ                   = "LREAL"
	REAL_OBJ                    = "REAL"
	BOOLEAN_OBJ                 = "BOOLEAN"
	NULL_OBJ                    = "NULL"
	RETURN_VALUE_OBJ            = "RETURN_VALUE"
	EXIT_OBJ                    = "EXIT"
	ERROR_OBJ                   = "ERROR"
	FUNCTION_OBJ                = "FUNCTION"
	STRING_OBJ                  = "STRING"
	WSTRING_OBJ                 = "WSTRING"
	BUILTIN_OBJ                 = "BUILTIN"
	POINTER_OBJ                 = "POINTER"
	BUILTIN_FUNCTION_BLOCK_OBJ  = "BUILTIN_FUNCTION_BLOCK"
	ARRAY_OBJ                   = "ARRAY"
	HASH_OBJ                    = "HASH"
	TIME_OBJ                    = "TIME"
	DATE_OBJ                    = "DATE"
	LWORD_OBJ                   = "LWORD"
	DWORD_OBJ                   = "DWORD"
	WORD_OBJ                    = "WORD"
	BYTE_OBJ                    = "BYTE"
	TIME_OF_DAY_OBJ             = "TIME_OF_DAY"
	DATE_AND_TIME_OBJ           = "DATE_AND_TIME"
	BITSTRING_OBJ               = "BITSTRING"
	QUOTE_OBJ                   = "QUOTE"
	MACRO_OBJ                   = "MACRO"
	JUMP_OBJ                    = "JUMP"
	RETURN_OBJ                  = "RETURN"
	SFC_OBJ                     = "SFC"
	STEP_OBJ                    = "STEP"
	TRANSITION_OBJ              = "TRANSITION"
	FUNCTION_BLOCK_OBJ          = "FUNCTION_BLOCK"
	FUNCTION_BLOCK_INSTANCE_OBJ = "FUNCTION_BLOCK_INSTANCE"
	ENUMERATED_TYPE_OBJ         = "ENUMERATED_TYPE"
	ENUMERATED_VALUE_OBJ        = "ENUMERATED_VALUE"
	ACTION_OBJ                  = "ACTION"
	SUBRANGE_TYPE_OBJ           = "SUBRANGE_TYPE"
	PROGRAM_OBJ                 = "PROGRAM"
	PROGRAM_INSTANCE_OBJ        = "PROGRAM_INSTANCE"
	COMPILED_FUNCTION_OBJ       = "COMPILED_FUNCTION_OBJ"
	CLOSURE_OBJ                 = "CLOSURE"
)

// Generic ANY types
const (
	ANY_TYPE       = "ANY"
	ANY_ELEMENTARY = "ANY_ELEMENTARY"
	ANY_MAGNITUDE  = "ANY_MAGNITUDE"
	ANY_NUM        = "ANY_NUM"
	ANY_INT        = "ANY_INT"
	ANY_REAL       = "ANY_REAL"
	ANY_BIT        = "ANY_BIT"
	ANY_STRING     = "ANY_STRING"
	ANY_DATE       = "ANY_DATE"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

// SInt objects store 8-bit signed integers.
type SInt struct {
	Value int8
}

func (si *SInt) Type() ObjectType { return SINT_OBJ }
func (si *SInt) Inspect() string  { return fmt.Sprintf("%d", si.Value) }

// Int objects store 16-bit signed integers.
type Int struct {
	Value int16
}

func (i *Int) Type() ObjectType { return INT_OBJ }
func (i *Int) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

// DInt objects store 32-bit signed integers.
type DInt struct {
	Value int32
}

func (di *DInt) Type() ObjectType { return DINT_OBJ }
func (di *DInt) Inspect() string  { return fmt.Sprintf("%d", di.Value) }

// LInt objects store 64-bit signed integers.
type LInt struct {
	Value int64
}

func (li *LInt) Type() ObjectType { return LINT_OBJ }
func (li *LInt) Inspect() string  { return fmt.Sprintf("%d", li.Value) }

// USInt objects store 8-bit unsigned integers.
type USInt struct {
	Value uint8
}

func (usi *USInt) Type() ObjectType { return USINT_OBJ }
func (usi *USInt) Inspect() string  { return fmt.Sprintf("%d", usi.Value) }

// UInt objects store 16-bit unsigned integers.
type UInt struct {
	Value uint16
}

func (ui *UInt) Type() ObjectType { return UINT_OBJ }
func (ui *UInt) Inspect() string  { return fmt.Sprintf("%d", ui.Value) }

// Real objects store 64-bit floating-point numbers.
type Real struct {
	Value float64
}

func (r *Real) Type() ObjectType { return REAL_OBJ }
func (r *Real) Inspect() string  { return fmt.Sprintf("%f", r.Value) }

// LReal objects store 64-bit floating-point numbers (long reals).
type LReal struct {
	Value float64
}

func (lr *LReal) Type() ObjectType { return LREAL_OBJ }
func (lr *LReal) Inspect() string  { return fmt.Sprintf("%f", lr.Value) }

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
	Value Object // The value being returned.
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// Exit objects are used to signal an exit from a loop.
type Exit struct{}

func (e *Exit) Type() ObjectType { return EXIT_OBJ }
func (e *Exit) Inspect() string  { return "EXIT" }

// It is an internal object used by the evaluator and not exposed to the user.
type Jump struct {
	TargetLabel string
}

// Return objects are used to signal a return from an IL program.
type Return struct{}

func (r *Return) Type() ObjectType { return RETURN_OBJ }
func (r *Return) Inspect() string  { return "RETURN" }
func (r *Return) HashKey() HashKey { return HashKey{} } // Not hashable

func (j *Jump) Type() ObjectType { return JUMP_OBJ }
func (j *Jump) Inspect() string  { return "JUMP to " + j.TargetLabel }
func (j *Jump) HashKey() HashKey { return HashKey{} } // Not hashable

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

// WString objects store wide-character string values.
type WString struct {
	Value string // Internally represented as a Go string, but treated as wide characters.
}

func (ws *WString) Type() ObjectType { return WSTRING_OBJ }
func (ws *WString) Inspect() string  { return ws.Value }

// BuiltinFunction is the type for a built-in function.
type BuiltinFunction func(args ...Object) Object

// Builtin objects represent built-in functions.
type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

// BuiltinFunctionBlockFunction is the type for a built-in function block's execution logic.
// It receives the instance's environment and the calling environment.
type BuiltinFunctionBlockFunction func(instanceEnv, callEnv *Environment) Object

// BuiltinFunctionBlock represents a pre-defined, stateful function block like TON or CTU.
type BuiltinFunctionBlock struct {
	Fn BuiltinFunctionBlockFunction
}

func (bfb *BuiltinFunctionBlock) Type() ObjectType { return BUILTIN_FUNCTION_BLOCK_OBJ }
func (bfb *BuiltinFunctionBlock) Inspect() string  { return "builtin function block" }

// Pointer is an object that holds a reference to another variable in an environment.
// This is the mechanism for implementing VAR_IN_OUT (pass-by-reference).
type Pointer struct {
	Name string       // The name of the variable in the Env.
	Env  *Environment // The environment where the variable is stored.
}

func (p *Pointer) Type() ObjectType { return POINTER_OBJ }
func (p *Pointer) Inspect() string {
	return fmt.Sprintf("POINTER(%s)", p.Name)
}

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

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

func (si *SInt) HashKey() HashKey { return HashKey{Type: si.Type(), Value: uint64(si.Value)} }
func (i *Int) HashKey() HashKey   { return HashKey{Type: i.Type(), Value: uint64(i.Value)} }
func (di *DInt) HashKey() HashKey { return HashKey{Type: di.Type(), Value: uint64(di.Value)} }
func (li *LInt) HashKey() HashKey { return HashKey{Type: li.Type(), Value: uint64(li.Value)} }

func (usi *USInt) HashKey() HashKey { return HashKey{Type: usi.Type(), Value: uint64(usi.Value)} }
func (ui *UInt) HashKey() HashKey   { return HashKey{Type: ui.Type(), Value: uint64(ui.Value)} }

// UDInt objects store 32-bit unsigned integers.
type UDInt struct{ Value uint32 }

func (udi *UDInt) Type() ObjectType { return UDINT_OBJ }
func (udi *UDInt) Inspect() string  { return fmt.Sprintf("%d", udi.Value) }
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

func (uli *ULInt) Type() ObjectType { return ULINT_OBJ }
func (uli *ULInt) Inspect() string  { return fmt.Sprintf("%d", uli.Value) }
func (uli *ULInt) HashKey() HashKey { return HashKey{Type: uli.Type(), Value: uli.Value} }

func (r *Real) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(fmt.Sprintf("%f", r.Value)))
	return HashKey{Type: r.Type(), Value: h.Sum64()}
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

func (st *SubrangeType) Type() ObjectType { return SUBRANGE_TYPE_OBJ }
func (st *SubrangeType) Inspect() string {
	return fmt.Sprintf("SUBRANGE %s (%d..%d)", st.BaseType, st.LowerBound, st.UpperBound)
}

// EnumeratedValue represents a single value from an enumerated type.
type EnumeratedValue struct {
	TypeName string
	Value    string
}

func (ev *EnumeratedValue) Type() ObjectType { return ENUMERATED_VALUE_OBJ }
func (ev *EnumeratedValue) Inspect() string  { return fmt.Sprintf("%s#%s", ev.TypeName, ev.Value) }
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

// Byte objects store 8-bit unsigned integers.
type Byte struct {
	Value byte
}

func (b *Byte) Type() ObjectType { return BYTE_OBJ }
func (b *Byte) Inspect() string  { return fmt.Sprintf("BYTE#16#%X", b.Value) }
func (b *Byte) HashKey() HashKey { return HashKey{Type: b.Type(), Value: uint64(b.Value)} }

// Word objects store 16-bit unsigned integers.
type Word struct {
	Value uint16
}

func (w *Word) Type() ObjectType { return WORD_OBJ }
func (w *Word) Inspect() string  { return fmt.Sprintf("WORD#16#%X", w.Value) }
func (w *Word) HashKey() HashKey { return HashKey{Type: w.Type(), Value: uint64(w.Value)} }

// DWord objects store 32-bit unsigned integers.
type DWord struct {
	Value uint32
}

func (dw *DWord) Type() ObjectType { return DWORD_OBJ }
func (dw *DWord) Inspect() string  { return fmt.Sprintf("DWORD#16#%X", dw.Value) }
func (dw *DWord) HashKey() HashKey { return HashKey{Type: dw.Type(), Value: uint64(dw.Value)} }

// LWord objects store 64-bit unsigned integers.
type LWord struct {
	Value uint64
}

func (lw *LWord) Type() ObjectType { return LWORD_OBJ }
func (lw *LWord) Inspect() string  { return fmt.Sprintf("LWORD#16#%X", lw.Value) }
func (lw *LWord) HashKey() HashKey { return HashKey{Type: lw.Type(), Value: lw.Value} }

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
	Actions         map[string]*Action // Map of action name to action object
	ActiveSteps     map[string]bool    // Map of step name to active status
}

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

func (t *Task) Type() ObjectType { return "TASK" } // Custom type for tasks
func (t *Task) Inspect() string {
	return fmt.Sprintf("TASK(%s, Priority: %d, Interval: %s)", t.Name, t.Priority, t.Interval)
}

// Scheduler manages all tasks and program instances within a resource.
type Scheduler struct {
	Tasks []*Task
}

func (s *Scheduler) Type() ObjectType { return "SCHEDULER" } // Custom type for scheduler
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
	Body       ast.Statement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
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
	Body       ast.Statement
	Env        *Environment
	Name       *ast.Identifier // The name of the function block
	VarInputs  []*ast.VarDeclStatement
	VarOutputs []*ast.VarDeclStatement
	VarInOuts  []*ast.VarDeclStatement
	Vars       []*ast.VarDeclStatement
}

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

	out.WriteString(strings.Join(allParams, " "))

	out.WriteString(")")

	return out.String()
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

// Program represents the definition of a PROGRAM POU.
// It's a template for creating program instances.
type Program struct {
	Name       *ast.Identifier
	VarInputs  []*ast.VarDeclStatement
	VarOutputs []*ast.VarDeclStatement
	VarInOuts  []*ast.VarDeclStatement
	Vars       []*ast.VarDeclStatement
	Body       ast.Statement
	Env        *Environment
}

func (p *Program) Type() ObjectType { return PROGRAM_OBJ }
func (p *Program) Inspect() string {
	if p.Name != nil {
		return "PROGRAM " + p.Name.Value
	}
	return "PROGRAM"
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

func (pi *ProgramInstance) Type() ObjectType { return PROGRAM_INSTANCE_OBJ }
func (pi *ProgramInstance) Inspect() string {
	if pi.Definition != nil && pi.Definition.Name != nil {
		return fmt.Sprintf("INSTANCE OF %s", pi.Definition.Name.Value)
	}
	return "PROGRAM_INSTANCE"
}

type CompiledFunction struct {
	Instructions  code.Instructions
	NumLocals     int
	NumParameters int
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }
func (cf *CompiledFunction) Inspect() string {
	return fmt.Sprintf("CompiledFunction[%p]", cf)
}

type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (c *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (c *Closure) Inspect() string {
	return fmt.Sprintf("Closure[%p]", c)
}
