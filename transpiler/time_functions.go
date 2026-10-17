/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// The functions of time data types (IEC 61131-3 Table 36 and 37): ADD_TIME,
// SUB_DT_DT, MUL_TIME, CONCAT_DT and the others (see object.TimeFunction).
// A TIME is a time.Duration and a DATE, TOD or DT a time.Time, so each is Go
// on those. The long types are the short ones (token.LongTimeType).

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// timeFunctionResults is the Go type of each time function's result.
var timeFunctionResults = map[string]string{
	"ADD_TIME": "iec.TIME", "ADD_LTIME": "iec.TIME", "SUB_TIME": "iec.TIME", "SUB_LTIME": "iec.TIME",
	"MUL_TIME": "iec.TIME", "MUL_LTIME": "iec.TIME", "MULTIME": "iec.TIME",
	"DIV_TIME": "iec.TIME", "DIV_LTIME": "iec.TIME", "DIVTIME": "iec.TIME",
	"ADD_TOD_TIME": "iec.TOD", "ADD_LTOD_LTIME": "iec.TOD", "SUB_TOD_TIME": "iec.TOD", "SUB_LTOD_LTIME": "iec.TOD",
	"ADD_DT_TIME": "iec.DT", "ADD_LDT_LTIME": "iec.DT", "SUB_DT_TIME": "iec.DT", "SUB_LDT_LTIME": "iec.DT",
	"SUB_DATE_DATE": "iec.TIME", "SUB_LDATE_LDATE": "iec.TIME", "SUB_TOD_TOD": "iec.TIME",
	"SUB_LTOD_LTOD": "iec.TIME", "SUB_DT_DT": "iec.TIME", "SUB_LDT_LDT": "iec.TIME",
	"CONCAT_DATE_LTOD": "iec.DT", "CONCAT_DATE": "iec.DATE", "CONCAT_TOD": "iec.TOD",
	"CONCAT_LTOD": "iec.TOD", "CONCAT_DT": "iec.DT", "CONCAT_LDT": "iec.DT",
}

// timeFunctionGoType is the Go type of a call of a time function, or "".
func timeFunctionGoType(e *ast.CallExpression) string {
	ident, ok := e.Function.(*ast.Identifier)
	if !ok {
		return ""
	}
	if strings.EqualFold(ident.Value, "$DATEPART") {
		return "iec.DINT"
	}
	return timeFunctionResults[strings.ToUpper(ident.Value)]
}

// transpileTimeFunction writes a call of a time function; done is false if
// name is not one.
func (t *Transpiler) transpileTimeFunction(name string, exp *ast.CallExpression) (done bool, err error) {
	if name == "$DATEPART" {
		return true, t.transpileDatePart(exp)
	}
	result, ok := timeFunctionResults[name]
	if !ok || !object.IsTimeFunction(name) {
		return false, nil
	}
	args := exp.Arguments
	arg := func(i int) error { return t.transpileExpression(args[i]) }
	want := 2
	switch name {
	case "CONCAT_DATE":
		want = 3
	case "CONCAT_TOD", "CONCAT_LTOD":
		want = 4
	case "CONCAT_DT", "CONCAT_LDT":
		want = 7
	}
	if len(args) != want {
		return true, fmt.Errorf("%s takes %d arguments, got %d", name, want, len(args))
	}
	// write writes format, each %a the next argument as Go.
	write := func(format string, conv string) error {
		parts := strings.Split(format, "%a")
		next := 0
		for i, p := range parts {
			t.write("%s", p)
			if i < len(parts)-1 {
				if conv != "" {
					t.write("%s(", conv)
				}
				if err := arg(next); err != nil {
					return err
				}
				if conv != "" {
					t.write(")")
				}
				next++
			}
		}
		return nil
	}
	op, _ := object.TimeOperator(name)
	switch {
	case name == "CONCAT_DATE_LTOD":
		// CONCAT_DATE_TOD, of the long types.
		short := *exp
		short.Function = &ast.Identifier{Token: exp.Function.(*ast.Identifier).Token, Value: "CONCAT_DATE_TOD"}
		return true, t.transpileCallExpression(&short)
	case name == "CONCAT_DATE":
		return true, write("iec.DATE(time.Date(%a, time.Month(%a), %a, 0, 0, 0, 0, time.UTC))", "int")
	case name == "CONCAT_TOD" || name == "CONCAT_LTOD":
		return true, write("iec.TOD(time.Date(0, 1, 1, %a, %a, %a, %a*int(time.Millisecond), time.UTC))", "int")
	case name == "CONCAT_DT" || name == "CONCAT_LDT":
		return true, write("iec.DT(time.Date(%a, time.Month(%a), %a, %a, %a, %a, %a*int(time.Millisecond), time.UTC))", "int")
	case name == "ADD_TIME" || name == "ADD_LTIME" || name == "SUB_TIME" || name == "SUB_LTIME":
		return true, write("iec.TIME(time.Duration(%a) "+op+" time.Duration(%a))", "")
	case op == "*" || op == "/":
		// Nanoseconds keeps a constant TIME from making a constant Go
		// expression, which Go would not truncate.
		return true, write("iec.TIME(float64(time.Duration(%a).Nanoseconds()) "+op+" float64(%a))", "")
	case result == "iec.TIME":
		// SUB_DT_DT, SUB_TOD_TOD, SUB_DATE_DATE: the time between.
		return true, write("iec.TIME(time.Time(%a).Sub(time.Time(%a)))", "")
	}
	// ADD_TOD_TIME, SUB_DT_TIME: a point in time moved.
	sign := ""
	if op == "-" {
		sign = "-"
	}
	return true, write(result+"(time.Time(%a).Add("+sign+"time.Duration(%a)))", "")
}

// datePartMethods are the Go of each part $DATEPART takes of a time.Time.
var datePartMethods = map[string]string{
	"YEAR": "%s.Year()", "MONTH": "int(%s.Month())", "DAY": "%s.Day()",
	"HOUR": "%s.Hour()", "MINUTE": "%s.Minute()", "SECOND": "%s.Second()",
	"MILLISECOND": "%s.Nanosecond() / 1000000", "WEEKDAY": "int(%s.Weekday())",
}

// transpileDatePart writes $DATEPART(value, 'PART'), a part of a DATE, TOD
// or DT, which SPLIT_DATE, SPLIT_TOD and SPLIT_DT are (package stdcalls).
func (t *Transpiler) transpileDatePart(exp *ast.CallExpression) error {
	if len(exp.Arguments) != 2 {
		return fmt.Errorf("$DATEPART takes 2 arguments, got %d", len(exp.Arguments))
	}
	part, ok := exp.Arguments[1].(*ast.StringLiteral)
	if !ok || datePartMethods[part.Value] == "" {
		return fmt.Errorf("$DATEPART: no part %s", exp.Arguments[1].String())
	}
	value, err := t.expressionString(exp.Arguments[0])
	if err != nil {
		return err
	}
	t.write("iec.DINT(%s)", fmt.Sprintf(datePartMethods[part.Value], "time.Time("+value+")"))
	return nil
}

// isIECDayOfWeek reports whether a call is DAY_OF_WEEK of IEC 61131-3 (0
// for Sunday to 6 for Saturday): the program does not declare DAY_OF_WEEK,
// as OSCAT's library does (1 for Monday to 7 for Sunday), and does not
// prefer OSCAT (PreferOSCAT).
func (t *Transpiler) isIECDayOfWeek(exp *ast.CallExpression) bool {
	ident, ok := exp.Function.(*ast.Identifier)
	return ok && strings.EqualFold(ident.Value, "DAY_OF_WEEK") && len(exp.Arguments) == 1 &&
		!t.PreferOSCAT && t.lookupType(ident.Value) == nil
}

// transpileDayOfWeek writes DAY_OF_WEEK(IN) of IEC 61131-3.
func (t *Transpiler) transpileDayOfWeek(exp *ast.CallExpression) error {
	value, err := t.expressionString(exp.Arguments[0])
	if err != nil {
		return err
	}
	// The type an assignment expects, n : USINT := DAY_OF_WEEK(d), or INT.
	goType := "iec.INT"
	if strings.HasPrefix(t.expectedGoType, "iec.") && isNumericGoType(t.expectedGoType) {
		goType = t.expectedGoType
	}
	t.write("%s(time.Time(%s).Weekday())", goType, value)
	return nil
}
