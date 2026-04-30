package object

import (
	"beedance/ast"
	"bytes"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"
)

type BuiltinFunction func(args ...Object) Object

type ObjectType string

const (
	NULL_OBJ  = "NULL"
	ERROR_OBJ = "ERROR"

	INTEGER_OBJ       = "INTEGER"
	BOOLEAN_OBJ       = "BOOLEAN"
	REAL_OBJ          = "REAL"
	STRING_OBJ        = "STRING"
	BITSTRING_OBJ     = "BITSTRING"
	TIME_OBJ          = "TIME"
	DATE_OBJ          = "DATE"
	TIME_OF_DAY_OBJ   = "TIME_OF_DAY"
	DATE_AND_TIME_OBJ = "DATE_AND_TIME"

	RETURN_VALUE_OBJ = "RETURN_VALUE"

	FUNCTION_OBJ = "FUNCTION"
	BUILTIN_OBJ  = "BUILTIN"

	ARRAY_OBJ = "ARRAY"
	HASH_OBJ  = "HASH"

	QUOTE_OBJ = "QUOTE"
	MACRO_OBJ = "MACRO"
)

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey {
	var value uint64

	if b.Value {
		value = 1
	} else {
		value = 0
	}

	return HashKey{Type: b.Type(), Value: value}
}

type Real struct {
	Value float64
}

func (r *Real) Type() ObjectType { return REAL_OBJ }
func (r *Real) Inspect() string  { return fmt.Sprintf("%f", r.Value) }
func (r *Real) HashKey() HashKey {
	// Using Float64bits to get a unique uint64 representation of the float64 value.
	// This allows float64 to be used as a hash key, though care should be taken
	// with floating-point precision issues if comparing hashes of computed floats.
	return HashKey{Type: r.Type(), Value: math.Float64bits(r.Value)}
}

type BitString struct {
	Value uint64
	Width int // 8 for BYTE, 16 for WORD, 32 for DWORD, 64 for LWORD
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
		return fmt.Sprintf("BITSTRING#%d#%X", bs.Width, bs.Value) // Fallback for unknown width
	}
}
func (bs *BitString) HashKey() HashKey {
	// For hashing, we combine the value and width to ensure uniqueness.
	// A simple way is to shift the width and OR with the value, or use a more robust hash function.
	// For now, we'll use the value directly, assuming the combination of Type and Value is sufficient.
	return HashKey{Type: bs.Type(), Value: bs.Value}
}

type Time struct {
	Value time.Duration
}

func (t *Time) Type() ObjectType { return TIME_OBJ }
func (t *Time) Inspect() string  { return fmt.Sprintf("T#%s", t.Value.String()) }
func (t *Time) HashKey() HashKey {
	return HashKey{Type: t.Type(), Value: uint64(t.Value)}
}

type Date struct {
	Value time.Time
}

func (d *Date) Type() ObjectType { return DATE_OBJ }
func (d *Date) Inspect() string  { return fmt.Sprintf("D#%s", d.Value.Format("2006-01-02")) }
func (d *Date) HashKey() HashKey {
	return HashKey{Type: d.Type(), Value: uint64(d.Value.UnixNano())}
}

type TimeOfDay struct {
	Value time.Time // Stored as a time on a zero date
}

func (tod *TimeOfDay) Type() ObjectType { return TIME_OF_DAY_OBJ }
func (tod *TimeOfDay) Inspect() string  { return fmt.Sprintf("TOD#%s", tod.Value.Format("15:04:05")) }
func (tod *TimeOfDay) HashKey() HashKey {
	return HashKey{Type: tod.Type(), Value: uint64(tod.Value.UnixNano())}
}

type DateAndTime struct {
	Value time.Time
}

func (dt *DateAndTime) Type() ObjectType { return DATE_AND_TIME_OBJ }
func (dt *DateAndTime) Inspect() string {
	return fmt.Sprintf("DT#%s", dt.Value.Format("2006-01-02-15:04:05"))
}
func (dt *DateAndTime) HashKey() HashKey {
	return HashKey{Type: dt.Type(), Value: uint64(dt.Value.UnixNano())}
}

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")

	return out.String()
}

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))

	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

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

type HashPair struct {
	Key   Object
	Value Object
}

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

type Quote struct {
	Node ast.Node
}

func (q *Quote) Type() ObjectType { return QUOTE_OBJ }
func (q *Quote) Inspect() string {
	return "QUOTE(" + q.Node.String() + ")"
}

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

	out.WriteString("macro")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(m.Body.String())
	out.WriteString("\n}")

	return out.String()
}
