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

// This file converts times and dates to and from numbers, and between each
// other. IEC 61131-3 leaves the numeric conversions to the implementation;
// these follow CODESYS, which OSCAT is written for: a TIME or TIME_OF_DAY is
// a number of milliseconds, and a DATE or DATE_AND_TIME a number of seconds
// since 1970-01-01. DT_TO_DATE and DT_TO_TOD split a date and time.

import (
	"math"
	"time"
)

// temporalTypeName returns the long name of a time or date type, given any
// of its names, or "" for any other type.
func temporalTypeName(typeName string) string {
	switch typeName {
	case "TIME", "T", "LTIME":
		return string(TIME_OBJ)
	case "DATE", "D", "LDATE":
		return string(DATE_OBJ)
	case "TIME_OF_DAY", "TOD", "LTOD", "LTIME_OF_DAY":
		return string(TIME_OF_DAY_OBJ)
	case "DATE_AND_TIME", "DT", "LDT", "LDATE_AND_TIME":
		return string(DATE_AND_TIME_OBJ)
	}
	return ""
}

// sinceMidnight returns the time of day of t.
func sinceMidnight(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second + time.Duration(t.Nanosecond())
}

// temporalAsNumber returns a time or date as the number CODESYS gives it.
func temporalAsNumber(input Object) (float64, bool) {
	switch v := input.(type) {
	case *Time:
		return float64(v.Value) / float64(time.Millisecond), true
	case *TimeOfDay:
		return float64(sinceMidnight(v.Value)) / float64(time.Millisecond), true
	case *Date:
		return float64(v.Value.Unix()), true
	case *DateAndTime:
		return float64(v.Value.Unix()), true
	}
	return 0, false
}

// numberAsTemporal returns the time or date of type typeName that n is.
func numberAsTemporal(n float64, typeName string) Object {
	switch typeName {
	case string(TIME_OBJ):
		return &Time{Value: time.Duration(math.Round(n * float64(time.Millisecond)))}
	case string(TIME_OF_DAY_OBJ):
		ms := math.Mod(math.Round(n), 24*60*60*1000)
		return &TimeOfDay{Value: time.Time{}.Add(time.Duration(ms) * time.Millisecond)}
	case string(DATE_OBJ):
		t := time.Unix(int64(n), 0).UTC()
		return &Date{Value: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
	default:
		return &DateAndTime{Value: time.Unix(int64(n), 0).UTC()}
	}
}

// convertTemporal converts input to toType when either is a time or date.
// It returns false when neither is, leaving the conversion to the caller.
func convertTemporal(input Object, toType string) (Object, bool) {
	target := temporalTypeName(toType)
	n, fromTemporal := temporalAsNumber(input)
	switch {
	case !fromTemporal && target == "":
		return nil, false
	case fromTemporal && target != "":
		return convertBetweenTemporal(input, target), true
	case fromTemporal && !IsIntegerType(toType) && !IsRealType(toType) && !IsBitStringType(toType):
		return nil, false // E.g. TIME_TO_STRING.
	case fromTemporal:
		// To a number. Integers and bit strings take the whole milliseconds
		// or seconds, through the usual range checks.
		if IsRealType(toType) {
			return ApplyConversion(&LReal{Value: n}, "LREAL", toType), true
		}
		whole := &LInt{Value: int64(math.Trunc(n))}
		return ApplyConversion(whole, "LINT", toType), true
	}
	// From a number.
	if value, ok := GetFloat64Value(input); ok {
		return numberAsTemporal(value, target), true
	}
	if bits, ok := input.(*BitString); ok {
		return numberAsTemporal(float64(bits.Value), target), true
	}
	return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), toType), true
}

// convertBetweenTemporal converts one time or date type to another, as
// DT_TO_DATE and DT_TO_TOD do. A conversion with no meaning, such as
// TIME_TO_DATE, is an error.
func convertBetweenTemporal(input Object, target string) Object {
	if string(input.Type()) == target {
		return input
	}
	if dt, ok := input.(*DateAndTime); ok {
		switch target {
		case string(DATE_OBJ):
			return numberAsTemporal(float64(dt.Value.Unix()), target)
		case string(TIME_OF_DAY_OBJ):
			return &TimeOfDay{Value: time.Time{}.Add(sinceMidnight(dt.Value))}
		}
	}
	if d, ok := input.(*Date); ok && target == string(DATE_AND_TIME_OBJ) {
		return &DateAndTime{Value: d.Value}
	}
	return NewBuiltinError("conversion from %s to %s is not supported", input.Type(), target)
}
