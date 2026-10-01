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
	"strings"
	"testing"
	"time"
)

func TestTemporalConversions(t *testing.T) {
	day := time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)
	stamp := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	tod := func(d time.Duration) Object { return &TimeOfDay{Value: time.Time{}.Add(d)} }
	tests := []struct {
		input    Object
		from, to string
		want     string
	}{
		{&Time{Value: 1500 * time.Millisecond}, "TIME", "DWORD", "DWORD#16#5DC"},
		{&Time{Value: 1500 * time.Millisecond}, "TIME", "UDINT", "1500"},
		{&Time{Value: 1500 * time.Millisecond}, "TIME", "LREAL", "1500.000000"},
		{&Time{Value: 1500 * time.Millisecond}, "TIME", "STRING", "1.5s"},
		{&Time{Value: -time.Second}, "TIME", "UDINT", "ERROR: value -1000 is out of range for type UDINT (0 to 4294967295)"},
		{&BitString{Value: 1500, Width: 32}, "DWORD", "TIME", "T#1.5s"},
		{&LReal{Value: 2.5}, "LREAL", "TIME", "T#2.5ms"},
		{&Date{Value: day}, "DATE", "DWORD", "DWORD#16#15180"},
		{&DateAndTime{Value: day.Add(time.Minute)}, "DT", "UDINT", "86460"},
		{tod(time.Hour), "TOD", "UDINT", "3600000"},
		{&UDInt{Value: 86460}, "UDINT", "DT", "DT#1970-01-02-00:01:00"},
		{&UDInt{Value: 86460}, "UDINT", "DATE", "D#1970-01-02"},
		{&UDInt{Value: 3600000 + 24*3600000}, "UDINT", "TOD", "TOD#01:00:00"},
		{&DateAndTime{Value: stamp}, "DT", "DATE", "D#2024-05-06"},
		{&DateAndTime{Value: stamp}, "DT", "TOD", "TOD#07:08:09"},
		{&Date{Value: day}, "DATE", "DT", "DT#1970-01-02-00:00:00"},
		{&Time{Value: time.Second}, "TIME", "TIME", "T#1s"},
		{&Time{Value: time.Second}, "TIME", "DATE", "ERROR: conversion from TIME to DATE is not supported"},
		{&String{Value: "x"}, "STRING", "TIME", "ERROR: conversion from STRING to TIME is not supported"},
		{&Int{Value: 1}, "TIME", "INT", "ERROR: type mismatch for TIME_TO_INT: input is INT, expected a TIME type"},
	}
	for _, tt := range tests {
		got := ApplyConversion(tt.input, tt.from, tt.to)
		text := got.Inspect()
		if e, ok := got.(*Error); ok {
			text = "ERROR: " + e.Message
		}
		if text != tt.want && !(len(tt.want) > 7 && tt.want[:7] == "ERROR: " && len(text) > 7 && strings.Contains(text, tt.want[7:])) {
			t.Errorf("%s_TO_%s(%s) = %s, want %s", tt.from, tt.to, tt.input.Inspect(), text, tt.want)
		}
	}
}
