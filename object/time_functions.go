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

// The functions of time data types (IEC 61131-3 3rd edition, Table 36 and
// 37): ADD_TIME, SUB_DT_DT, MUL_TIME, CONCAT_DT and the others, with the
// long variants (ADD_LTIME, ...) and the 2nd edition's MULTIME and DIVTIME.
// Like the conversions, they are one built-in, $TIMEFN('NAME', args...),
// which the compiler calls; the evaluator gets each by name.

import (
	"strings"
	"sync"
	"time"
)

// TimeFunctionBuiltinName is the built-in every time function compiles to.
const TimeFunctionBuiltinName = "$TIMEFN"

// timeOperators are the time functions that are an operator on their two
// inputs, by name.
var timeOperators = map[string]string{
	"ADD_TIME": "+", "ADD_LTIME": "+", "ADD_TOD_TIME": "+", "ADD_LTOD_LTIME": "+",
	"ADD_DT_TIME": "+", "ADD_LDT_LTIME": "+",
	"SUB_TIME": "-", "SUB_LTIME": "-", "SUB_DATE_DATE": "-", "SUB_LDATE_LDATE": "-",
	"SUB_TOD_TIME": "-", "SUB_LTOD_LTIME": "-", "SUB_TOD_TOD": "-", "SUB_LTOD_LTOD": "-",
	"SUB_DT_TIME": "-", "SUB_LDT_LTIME": "-", "SUB_DT_DT": "-", "SUB_LDT_LDT": "-",
	"MUL_TIME": "*", "MUL_LTIME": "*", "MULTIME": "*",
	"DIV_TIME": "/", "DIV_LTIME": "/", "DIVTIME": "/",
}

// timeConcats are the CONCAT_ functions, by name, with the number of their
// inputs.
var timeConcats = map[string]int{
	"CONCAT_DATE_LTOD": 2, "CONCAT_DATE": 3, "CONCAT_TOD": 4, "CONCAT_LTOD": 4,
	"CONCAT_DT": 7, "CONCAT_LDT": 7,
}

// TimeOperator returns the operator a time function is, ADD_TIME's +, and
// whether name is one.
func TimeOperator(name string) (string, bool) {
	op, ok := timeOperators[strings.ToUpper(name)]
	return op, ok
}

// IsTimeFunction reports whether name is one of the time functions here.
func IsTimeFunction(name string) bool {
	upper := strings.ToUpper(name)
	_, isOp := timeOperators[upper]
	_, isConcat := timeConcats[upper]
	return isOp || isConcat
}

// TimeFunction is $TIMEFN: its first argument names the time function, the
// others are its inputs.
func TimeFunction(args ...Object) Object {
	if len(args) < 1 {
		return NewBuiltinError("%s: no function named", TimeFunctionBuiltinName)
	}
	name, ok := args[0].(*String)
	if !ok {
		return NewBuiltinError("%s: the function's name must be a string", TimeFunctionBuiltinName)
	}
	return callTimeFunction(strings.ToUpper(name.Value), args[1:])
}

func callTimeFunction(name string, args []Object) Object {
	if op, ok := timeOperators[name]; ok {
		if len(args) != 2 {
			return NewBuiltinError("wrong number of arguments for %s. got=%d, want=2", name, len(args))
		}
		return EvalInfix(args[0], op, args[1])
	}
	want, ok := timeConcats[name]
	if !ok {
		return NewBuiltinError("unknown time function %s", name)
	}
	if name == "CONCAT_DATE_LTOD" {
		d, okD := args[0].(*Date)
		tod, okT := args[1].(*TimeOfDay)
		if len(args) != 2 || !okD || !okT {
			return NewBuiltinError("CONCAT_DATE_LTOD takes a DATE and an LTOD")
		}
		y, m, day := d.Value.Date()
		h, mi, s := tod.Value.Clock()
		return &DateAndTime{Value: time.Date(y, m, day, h, mi, s, tod.Value.Nanosecond(), d.Value.Location())}
	}
	if len(args) != want {
		return NewBuiltinError("wrong number of arguments for %s. got=%d, want=%d", name, len(args), want)
	}
	n := make([]int, want)
	for i, a := range args {
		v, _, isInt := GetIntegerObjectValue(a)
		if !isInt {
			return NewBuiltinError("argument %d to %s must be an integer, got %s", i+1, name, a.Type())
		}
		n[i] = int(v)
	}
	switch name {
	case "CONCAT_DATE": // YEAR, MONTH, DAY
		return &Date{Value: time.Date(n[0], time.Month(n[1]), n[2], 0, 0, 0, 0, time.UTC)}
	case "CONCAT_TOD", "CONCAT_LTOD": // HOUR, MINUTE, SECOND, MILLISECOND
		return &TimeOfDay{Value: time.Date(0, 1, 1, n[0], n[1], n[2], n[3]*int(time.Millisecond), time.UTC)}
	}
	// CONCAT_DT, CONCAT_LDT: YEAR, MONTH, DAY, HOUR, MINUTE, SECOND, MILLISECOND
	return &DateAndTime{Value: time.Date(n[0], time.Month(n[1]), n[2], n[3], n[4], n[5], n[6]*int(time.Millisecond), time.UTC)}
}

// timeFunctions caches the built-ins timeFunctionBuiltin has made.
var timeFunctions struct {
	sync.Mutex
	byName map[string]*Builtin
}

// timeFunctionBuiltin is the built-in of a time function's name, for the
// evaluator.
func timeFunctionBuiltin(name string) (*Builtin, bool) {
	upper := strings.ToUpper(name)
	if !IsTimeFunction(upper) {
		return nil, false
	}
	timeFunctions.Lock()
	defer timeFunctions.Unlock()
	if b, ok := timeFunctions.byName[upper]; ok {
		return b, true
	}
	if timeFunctions.byName == nil {
		timeFunctions.byName = map[string]*Builtin{}
	}
	b := &Builtin{Fn: func(args ...Object) Object { return callTimeFunction(upper, args) }}
	timeFunctions.byName[upper] = b
	return b, true
}
