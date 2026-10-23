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

const (
	BuiltinAbs              = iota // ABS
	BuiltinAcos                    // ACOS
	BuiltinAdd                     // ADD
	BuiltinAnd                     // AND
	BuiltinAsin                    // ASIN
	BuiltinAtan                    // ATAN
	BuiltinAtan2                   // ATAN2
	BuiltinConcat                  // CONCAT
	BuiltinCos                     // COS
	BuiltinDelete                  // DELETE
	BuiltinDiv                     // DIV
	BuiltinEq                      // EQ
	BuiltinExp                     // EXP
	BuiltinExpt                    // EXPT
	BuiltinFind                    // FIND
	BuiltinFirst                   // FIRST
	BuiltinGe                      // GE
	BuiltinGt                      // GT
	BuiltinGreat                   // GREAT
	BuiltinInsert                  // INSERT
	BuiltinLast                    // LAST
	BuiltinLe                      // LE
	BuiltinLeft                    // LEFT
	BuiltinLen                     // LEN
	BuiltinLimit                   // LIMIT
	BuiltinLn                      // LN
	BuiltinLog                     // LOG
	BuiltinLowerBound              // LOWER_BOUND
	BuiltinLt                      // LT
	BuiltinMax                     // MAX
	BuiltinMid                     // MID
	BuiltinMin                     // MIN
	BuiltinMod                     // MOD
	BuiltinMove                    // MOVE
	BuiltinMul                     // MUL
	BuiltinMux                     // MUX
	BuiltinNand                    // NAND
	BuiltinNe                      // NE
	BuiltinNor                     // NOR
	BuiltinOr                      // OR
	BuiltinPop                     // POP
	BuiltinPush                    // PUSH
	BuiltinPuts                    // PUTS
	BuiltinReplace                 // REPLACE
	BuiltinRest                    // REST
	BuiltinRight                   // RIGHT
	BuiltinRol                     // ROL
	BuiltinRor                     // ROR
	BuiltinRound                   // ROUND
	BuiltinSel                     // SEL
	BuiltinShl                     // SHL
	BuiltinShr                     // SHR
	BuiltinSin                     // SIN
	BuiltinSqrt                    // SQRT
	BuiltinSub                     // SUB
	BuiltinTan                     // TAN
	BuiltinTrunc                   // TRUNC
	BuiltinUpperBound              // UPPER_BOUND
	BuiltinXor                     // XOR
	BuiltinClock                   // __CLOCK
	BuiltinTime                    // TIME
	BuiltinNativeNew               // __NATIVE_NEW
	BuiltinNativeSet               // __NATIVE_SET
	BuiltinNativeRun               // __NATIVE_RUN
	BuiltinNativeGet               // __NATIVE_GET
	BuiltinConvert                 // $CONVERT: every X_TO_Y type conversion
	BuiltinConcatDateTod           // CONCAT_DATE_TOD
	BuiltinAttempt                 // $ATTEMPT: the assignment attempt ?=
	BuiltinTimefn                  // $TIMEFN: every time function ADD_TIME, SUB_DT_DT, CONCAT_DT, ...
	BuiltinToBigEndian             // TO_BIG_ENDIAN
	BuiltinToLittleEndian          // TO_LITTLE_ENDIAN
	BuiltinFromBigEndian           // FROM_BIG_ENDIAN
	BuiltinFromLittleEndian        // FROM_LITTLE_ENDIAN
	BuiltinIsValid                 // IS_VALID
	BuiltinIsValidBcd              // IS_VALID_BCD
	BuiltinDatepart                // $DATEPART: a part of a date or time, for SPLIT_DATE, SPLIT_TOD and SPLIT_DT
	BuiltinDayOfWeek               // DAY_OF_WEEK
	BuiltinForStep                 // $FORSTEP: a FOR loop's step, checked for 0
)
