/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

// The IEC 61131-3 functions of times and dates (Table 37).

import (
	"time"

	"github.com/apiarytech/beedance/object"
)

func init() {
	object.RegisterBuiltin(BuiltinConcatDateTod, "CONCAT_DATE_TOD", concatDateTodFn)
}

// concatDateTodFn is CONCAT_DATE_TOD(IN1 : DATE, IN2 : TOD) : DT, the date
// of IN1 at the time of day IN2.
var concatDateTodFn = func(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for CONCAT_DATE_TOD. got=%d, want=2", len(args))
	}
	d, ok := args[0].(*object.Date)
	if !ok {
		return object.NewBuiltinError("argument 1 to `CONCAT_DATE_TOD` must be DATE, got %s", args[0].Type())
	}
	tod, ok := args[1].(*object.TimeOfDay)
	if !ok {
		return object.NewBuiltinError("argument 2 to `CONCAT_DATE_TOD` must be TIME_OF_DAY, got %s", args[1].Type())
	}
	y, m, day := d.Value.Date()
	h, mi, s := tod.Value.Clock()
	return &object.DateAndTime{Value: time.Date(y, m, day, h, mi, s, tod.Value.Nanosecond(), d.Value.Location())}
}

func init() {
	object.RegisterBuiltin(BuiltinTimefn, "$TIMEFN", object.TimeFunction) // object.TimeFunctionBuiltinName
}

func init() {
	object.RegisterBuiltin(BuiltinDatepart, "$DATEPART", datePartFn)
}

// datePartFn is $DATEPART(value, 'PART'): a part of a DATE, TIME_OF_DAY or
// DATE_AND_TIME (YEAR, MONTH, DAY, HOUR, MINUTE, SECOND, MILLISECOND, or
// WEEKDAY, 0 for Sunday). SPLIT_DATE, SPLIT_TOD and SPLIT_DT are their
// outputs' $DATEPART (package stdcalls).
var datePartFn = func(args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewBuiltinError("wrong number of arguments for $DATEPART. got=%d, want=2", len(args))
	}
	var t time.Time
	switch v := args[0].(type) {
	case *object.Date:
		t = v.Value
	case *object.TimeOfDay:
		t = v.Value
	case *object.DateAndTime:
		t = v.Value
	default:
		return object.NewBuiltinError("argument 1 to `$DATEPART` must be a DATE, TOD or DT, got %s", args[0].Type())
	}
	part, ok := args[1].(*object.String)
	if !ok {
		return object.NewBuiltinError("argument 2 to `$DATEPART` must be a string, got %s", args[1].Type())
	}
	n, ok := DatePart(t, part.Value)
	if !ok {
		return object.NewBuiltinError("$DATEPART: no part %s", part.Value)
	}
	return &object.DInt{Value: int32(n)}
}

// DatePart returns a part of t: YEAR, MONTH, DAY, HOUR, MINUTE, SECOND,
// MILLISECOND or WEEKDAY (0 for Sunday).
func DatePart(t time.Time, part string) (int, bool) {
	switch part {
	case "YEAR":
		return t.Year(), true
	case "MONTH":
		return int(t.Month()), true
	case "DAY":
		return t.Day(), true
	case "HOUR":
		return t.Hour(), true
	case "MINUTE":
		return t.Minute(), true
	case "SECOND":
		return t.Second(), true
	case "MILLISECOND":
		return t.Nanosecond() / int(time.Millisecond), true
	case "WEEKDAY":
		return int(t.Weekday()), true
	}
	return 0, false
}

func init() {
	object.RegisterBuiltin(BuiltinDayOfWeek, "DAY_OF_WEEK", dayOfWeekFn)
}

// dayOfWeekFn is DAY_OF_WEEK(IN : DATE) : INT of IEC 61131-3, 0 for Sunday
// to 6 for Saturday. A program that declares DAY_OF_WEEK, as OSCAT's
// library does (1 for Monday to 7 for Sunday), calls its own.
var dayOfWeekFn = func(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("wrong number of arguments for DAY_OF_WEEK. got=%d, want=1", len(args))
	}
	var t time.Time
	switch v := args[0].(type) {
	case *object.Date:
		t = v.Value
	case *object.DateAndTime:
		t = v.Value
	default:
		return object.NewBuiltinError("argument to `DAY_OF_WEEK` must be a DATE, got %s", args[0].Type())
	}
	return &object.Int{Value: int16(t.Weekday())}
}
