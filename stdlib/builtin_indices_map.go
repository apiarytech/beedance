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

// BuiltinNameToIndex maps the string name of a built-in to its stable iota index.
// The type conversions X_TO_Y are not in it: they are one built-in, $CONVERT
// (see object.ParseConversion).
var BuiltinNameToIndex = map[string]int{
	"ABS":                 BuiltinAbs,
	"ACOS":                BuiltinAcos,
	"ADD":                 BuiltinAdd,
	"AND":                 BuiltinAnd,
	"ASIN":                BuiltinAsin,
	"ATAN":                BuiltinAtan,
	"ATAN2":               BuiltinAtan2,
	"CONCAT":              BuiltinConcat,
	"COS":                 BuiltinCos,
	"DELETE":              BuiltinDelete,
	"DIV":                 BuiltinDiv,
	"EQ":                  BuiltinEq,
	"EXP":                 BuiltinExp,
	"EXPT":                BuiltinExpt,
	"FIND":                BuiltinFind,
	"FIRST":               BuiltinFirst,
	"GE":                  BuiltinGe,
	"GT":                  BuiltinGt,
	"GREAT":               BuiltinGreat,
	"INSERT":              BuiltinInsert,
	"LAST":                BuiltinLast,
	"LE":                  BuiltinLe,
	"LEFT":                BuiltinLeft,
	"LEN":                 BuiltinLen,
	"LIMIT":               BuiltinLimit,
	"LN":                  BuiltinLn,
	"LOG":                 BuiltinLog,
	"LOWER_BOUND":         BuiltinLowerBound,
	"LT":                  BuiltinLt,
	"MAX":                 BuiltinMax,
	"MID":                 BuiltinMid,
	"MIN":                 BuiltinMin,
	"MOD":                 BuiltinMod,
	"MOVE":                BuiltinMove,
	"MUL":                 BuiltinMul,
	"MUX":                 BuiltinMux,
	"NAND":                BuiltinNand,
	"NE":                  BuiltinNe,
	"NOR":                 BuiltinNor,
	"OR":                  BuiltinOr,
	"POP":                 BuiltinPop,
	"PUSH":                BuiltinPush,
	"PUTS":                BuiltinPuts,
	"REPLACE":             BuiltinReplace,
	"REST":                BuiltinRest,
	"RIGHT":               BuiltinRight,
	"ROL":                 BuiltinRol,
	"ROR":                 BuiltinRor,
	"ROUND":               BuiltinRound,
	"SEL":                 BuiltinSel,
	"SHL":                 BuiltinShl,
	"SHR":                 BuiltinShr,
	"SIN":                 BuiltinSin,
	"SQRT":                BuiltinSqrt,
	"SUB":                 BuiltinSub,
	"TAN":                 BuiltinTan,
	"TRUNC":               BuiltinTrunc,
	"UPPER_BOUND":         BuiltinUpperBound,
	"XOR":                 BuiltinXor,
	"__CLOCK":             BuiltinClock,
	"TIME":                BuiltinTime,
	"__NATIVE_NEW":        BuiltinNativeNew,
	"__NATIVE_SET":        BuiltinNativeSet,
	"__NATIVE_RUN":        BuiltinNativeRun,
	"__NATIVE_GET":        BuiltinNativeGet,
	"$CONVERT":            BuiltinConvert,
}
