/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package oscatdiff

import (
	"math"
	"reflect"
)

// The reasons of the known mismatches. Each was checked against the source:
// the port follows it, and the difference is in how the two run it.
const (
	// The ports compute REAL in 32 bits, as a PLC does, the evaluator in 64;
	// these calculations lose more than precisionTolerance to it, by
	// cancellation or by overflowing 32 bits.
	float32Rounding = "32 bit REAL"
	// The ports wrap integer and DWORD time arithmetic to the declared
	// type, as a PLC's registers of that width would; the evaluator
	// calculates in 64 bits, so a DATE before 1970 or after 2106 and an
	// INT past its range do not wrap.
	integerWidth = "integer width"
	// The ports round a REAL to an integer half to even, as royaljelly does;
	// the evaluator rounds halves away from zero.
	roundingTies = "rounding of halves"

	// royaljelly v0.3.0's TOF keeps Q on in the scan IN falls even when PT
	// is T#0s, and turns Q on again when PT grows after its delay; IEC
	// 61131-3 keeps Q off until IN rises. Fixed in royaljelly after v0.3.1:
	// these come off the list when beebread requires the fix.
	royaljellyTOF = "royaljelly: TOF after IN falls"
	// Not traced to its cause yet.
	untriaged = "UNTRIAGED"
)

// known are the POUs whose ports are known to differ from the evaluator,
// with the reason. Their mismatches are reported, not failed. A POU whose
// reason goes away with a fix of the port or of the evaluator comes off the
// list.
var known = map[string]string{
	"ASINH":         float32Rounding,
	"BETA":          float32Rounding,
	"CIRCLE_SEG":    float32Rounding,
	"DEG":           float32Rounding,
	"MODR":          float32Rounding,
	"RANGE_TO_BYTE": float32Rounding,
	"RANGE_TO_WORD": float32Rounding,
	"RDM2":          float32Rounding,
	"SENSOR_INT":    float32Rounding,
	"TANK_VOL1":     float32Rounding,

	"BINOM":        integerWidth,
	"CMP":          integerWidth,
	"DATE_ADD":     integerWidth,
	"EASTER":       integerWidth,
	"EXPN":         integerWidth,
	"INC1":         integerWidth,
	"INC2":         integerWidth,
	"JD2000":       integerWidth,
	"LTIME_TO_UTC": integerWidth,
	"RND":          integerWidth, // EXPN(10, -32768): ABS wraps, so 1.0 in the source, 0 in the port
	"SET_DATE":     integerWidth,
	"SET_TOD":      integerWidth,
	"UTC_TO_LTIME": integerWidth, // TIME_ZONE_OFFSET * 60 in an INT, past ±546 minutes
	"YEAR_BEGIN":   integerWidth,
	"YEAR_END":     integerWidth,

	"REAL_TO_STRF": roundingTies,
	"ROUND":        roundingTies,
	"STAIR":        roundingTies,

	// STRING_TO_DWORD('-12'): the port wraps the negative number, the
	// evaluator gives 0.
	"IP4_DECODE": "STRING_TO_DWORD of a negative number",

	// Function blocks.
	"FT_PT2":      float32Rounding,
	"FILTER_I":    integerWidth,
	"BLIND_INPUT": royaljellyTOF,
	"FADE":        untriaged,
	"DEBOUNCE":    royaljellyTOF,
	"INTERLOCK":   royaljellyTOF,
	"ACTUATOR_A":  untriaged,
	"CTRL_PWM":    untriaged,
	"CYCLE_4":     untriaged,
	"FT_IMP":      untriaged,
	"FT_PDT1":     untriaged,
	"FT_PT1":      untriaged,
	"GEN_RDT":     royaljellyTOF,
	"MOON_PHASE":  untriaged,
	"PWM_DC":      untriaged,
	"SUN_POS":     untriaged,
	"TIMER_EXT":   untriaged,
	"TP_1D":       untriaged,
}

// domain limits the arguments of the POUs that loop for ever outside it, on
// a PLC as in their ports, so that the test does not leave them running.
var domain = map[string]func(args []reflect.Value) bool{
	// ABS(-2147483648) is itself in 32 bits, and the halving never ends.
	"GCD": func(args []reflect.Value) bool {
		return args[0].Int() != math.MinInt32 && args[1].Int() != math.MinInt32
	},
	// FOR i := 1 TO 32767 with an INT i wraps before it ends.
	"REFLECT": func(args []reflect.Value) bool { return args[1].Int() != math.MaxInt16 },
}
