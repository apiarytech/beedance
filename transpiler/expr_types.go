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

// IEC 61131-3 widens a value implicitly, an INT to a DINT or a REAL, and
// treats AND, OR and XOR on bit strings and integers as bitwise; Go does
// neither. This file works out the type of an expression where it can, so
// the transpiler can convert the narrower operand and choose the operator.

import (
	"strconv"
	"strings"

	"beedance/ast"
)

// typeRank orders the elementary types by width: a value converts
// implicitly to a type of a higher rank.
var typeRank = map[string]int{
	"iec.BYTE": 1, "iec.SINT": 1, "iec.USINT": 1,
	"iec.WORD": 2, "iec.INT": 2, "iec.UINT": 2,
	"iec.DWORD": 3, "iec.DINT": 3, "iec.UDINT": 3,
	"iec.LWORD": 4, "iec.LINT": 4, "iec.ULINT": 4,
	"iec.REAL": 5, "iec.LREAL": 6,
}

// isIntegerGoType reports whether a Go type is an IEC integer or bit string.
func isIntegerGoType(goType string) bool {
	r, ok := typeRank[goType]
	return ok && r <= 4
}

// wider returns the wider of two types, or the known one if the other is
// not known.
func wider(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case typeRank[b] > typeRank[a]:
		return b
	}
	return a
}

// exprGoType returns the Go type of an expression's value when it is an
// elementary type the transpiler knows, such as "iec.INT", or "".
func (t *Transpiler) exprGoType(e ast.Expression) string {
	switch e := e.(type) {
	case *ast.Identifier:
		if t.currentFunc != nil && e.Value == t.currentFunc.Name.Value && t.currentFunc.ReturnType != nil {
			return iecOnly(t.mapIecTypeToGo(t.currentFunc.ReturnType))
		}
		if g, ok := t.beebreadGlobal(e); ok {
			if _, isStruct := beebreadFunctionBlocks[beebreadGlobals[strings.TrimPrefix(g, "oscat.")]]; !isStruct {
				return "iec.INT"
			}
			return ""
		}
		return t.elementaryGoType(e)
	case *ast.MemberAccessExpression:
		if b, ok := t.beebreadTypeOfValue(e.Struct); ok {
			if _, typ, ok := b.field(e.Member.Value); ok {
				return iecOnly(typ)
			}
			return ""
		}
		return t.elementaryGoType(e)
	case *ast.IndexExpression:
		return t.elementaryGoType(e)
	case *ast.BitAccessExpression, *ast.Boolean:
		return "iec.BOOL"
	case *ast.TypedLiteral:
		return iecOnly("iec." + strings.ToUpper(e.TypeName))
	case *ast.BitStringLiteral:
		return bitStringGoType(e.Width)
	case *ast.CallExpression:
		ident, ok := e.Function.(*ast.Identifier)
		if !ok {
			return ""
		}
		if fd, ok := t.lookupType(ident.Value).(*ast.FunctionDeclaration); ok && fd.ReturnType != nil {
			return iecOnly(t.mapIecTypeToGo(fd.ReturnType))
		}
		if isClockCall(e) {
			return "iec.TIME"
		}
		if _, fn, ok := t.beebreadFunctionOf(e.Function); ok {
			return iecOnly(fn.result)
		}
		if fn, ok := royaljellyFunctions[strings.ToUpper(ident.Value)]; ok {
			// A generic function gives the type of its first argument whose
			// type is known, or a shift or rotation of its first argument.
			if fn.result == "" {
				args := e.Arguments
				switch strings.ToUpper(ident.Value) {
				case "SHL", "SHR", "ROL", "ROR":
					if len(args) > 0 {
						args = args[:1]
					}
				}
				for _, a := range args {
					if at := t.exprGoType(a); at != "" && at != "iec.BOOL" {
						return at
					}
				}
			}
			return iecOnly(fn.result)
		}
	case *ast.PrefixExpression:
		return t.exprGoType(e.Right)
	case *ast.InfixExpression:
		switch strings.ToUpper(e.Operator) {
		case "=", "<>", "<", ">", "<=", ">=":
			return "iec.BOOL"
		}
		lt, rt := t.exprGoType(e.Left), t.exprGoType(e.Right)
		if isTimePoint(lt) && (e.Operator == "+" || e.Operator == "-") {
			if rt == lt {
				return "iec.TIME"
			}
			return lt
		}
		return wider(lt, rt)
	}
	return ""
}

// literalFit returns the narrowest signed integer type wider than INT that
// an integer literal needs, iec.DINT or iec.LINT, or "" for a literal that
// fits an INT, or another expression.
func literalFit(e ast.Expression) string {
	lit, ok := e.(*ast.IntegerLiteral)
	switch {
	case !ok || (lit.Value >= -32768 && lit.Value <= 32767):
		return ""
	case lit.Value >= -2147483648 && lit.Value <= 2147483647:
		return "iec.DINT"
	}
	return "iec.LINT"
}

// isBitStringGoType reports whether a Go type is an IEC bit string.
func isBitStringGoType(goType string) bool {
	switch goType {
	case "iec.BYTE", "iec.WORD", "iec.DWORD", "iec.LWORD":
		return true
	}
	return false
}

// isLibraryCall reports whether e calls a royaljelly or beebread function,
// which converts its own result to the type expected of it.
func (t *Transpiler) isLibraryCall(e ast.Expression) bool {
	call, ok := e.(*ast.CallExpression)
	if !ok {
		return false
	}
	ident, ok := call.Function.(*ast.Identifier)
	if !ok || t.lookupType(ident.Value) != nil {
		return false
	}
	_, isStd := royaljellyFunctions[strings.ToUpper(ident.Value)]
	_, _, isOSCAT := t.beebreadFunctionOf(ident)
	return isStd || isOSCAT
}

// isTimePoint reports whether a Go type is a point in time: a DATE, DT or TOD.
func isTimePoint(goType string) bool {
	return goType == "iec.DATE" || goType == "iec.DT" || goType == "iec.TOD"
}

// iecOnly returns goType if it is an elementary IEC type, and "" otherwise.
func iecOnly(goType string) string {
	if strings.HasPrefix(goType, "iec.") {
		return goType
	}
	return ""
}

// widening returns the type an operand of type from converts to for an
// operation or assignment of type to, or "" if it needs no conversion.
func widening(from, to string) string {
	if from == "" || to == "" || from == to {
		return ""
	}
	if _, ok := typeRank[from]; !ok {
		return ""
	}
	if _, ok := typeRank[to]; !ok {
		return ""
	}
	return to
}

// transpileConverted transpiles e, converted to the type to if it has
// another numeric type.
func (t *Transpiler) transpileConverted(e ast.Expression, to string) error {
	if b, ok := boolLiteral(e); ok && to == "iec.BOOL" {
		t.write("%s", b)
		return nil
	}
	conv := widening(t.exprGoType(e), to)
	if conv != "" {
		t.write("%s(", conv)
	}
	if err := t.transpileExpression(e); err != nil {
		return err
	}
	if conv != "" {
		t.write(")")
	}
	return nil
}

// boolLiteral returns the Go value of 0 or 1 used as a BOOL, as IEC
// 61131-3 allows: false or true.
func boolLiteral(e ast.Expression) (string, bool) {
	lit, ok := e.(*ast.IntegerLiteral)
	switch {
	case !ok:
		return "", false
	case lit.Value == 0:
		return "false", true
	case lit.Value == 1:
		return "true", true
	}
	return "", false
}

// usedNames returns the identifiers a statement uses.
func usedNames(stmt ast.Node) map[string]bool {
	used := map[string]bool{}
	if stmt == nil {
		return used
	}
	ast.Modify(stmt, func(n ast.Node) ast.Node {
		if id, ok := n.(*ast.Identifier); ok {
			used[id.Value] = true
		}
		return n
	})
	return used
}

// transpileValue transpiles a value assigned to a target of the type
// target, converting it if it has another numeric type. A call of a
// royaljelly or beebread function converts its own value.
func (t *Transpiler) transpileValue(value ast.Expression, target string) error {
	expected := t.expectedGoType
	t.expectedGoType = target
	defer func() { t.expectedGoType = expected }()
	if call, ok := value.(*ast.CallExpression); ok {
		if ident, ok := call.Function.(*ast.Identifier); ok {
			_, isStd := royaljellyFunctions[strings.ToUpper(ident.Value)]
			_, _, isOSCAT := t.beebreadFunctionOf(ident)
			if (isStd || isOSCAT) && t.lookupType(ident.Value) == nil {
				return t.transpileExpression(value)
			}
		}
	}
	t.expectedGoType = expected
	return t.transpileConverted(value, target)
}

// timeComparisons are the Go comparisons of two time.Time values a and b.
var timeComparisons = map[string]string{
	"=": "iec.BOOL(%s.Equal(%s))", "<>": "iec.BOOL(!%s.Equal(%s))",
	"<": "iec.BOOL(%s.Before(%s))", ">": "iec.BOOL(%s.After(%s))",
	"<=": "iec.BOOL(!%s.After(%s))", ">=": "iec.BOOL(!%s.Before(%s))",
}

// transpileTimeComparison transpiles a comparison of two DATEs, DTs or
// TODs, which are Go structures, and reports whether exp is one.
func (t *Transpiler) transpileTimeComparison(exp *ast.InfixExpression) (bool, error) {
	format, ok := timeComparisons[exp.Operator]
	if !ok {
		return false, nil
	}
	switch wider(t.exprGoType(exp.Left), t.exprGoType(exp.Right)) {
	case "iec.DATE", "iec.DT", "iec.TOD", "iec.TIME_OF_DAY":
	default:
		return false, nil
	}
	left, err := t.expressionString(exp.Left)
	if err != nil {
		return true, err
	}
	right, err := t.expressionString(exp.Right)
	if err != nil {
		return true, err
	}
	t.write("(")
	t.write(format, "time.Time("+left+")", "time.Time("+right+")")
	t.write(")")
	return true, nil
}

// transpileTimeArithmetic transpiles a DATE, DT or TOD plus or minus a
// TIME, or the TIME between two of them, and reports whether exp is one.
func (t *Transpiler) transpileTimeArithmetic(exp *ast.InfixExpression) (bool, error) {
	if exp.Operator == "*" || exp.Operator == "/" {
		return t.transpileTimeScaling(exp)
	}
	if exp.Operator != "+" && exp.Operator != "-" {
		return false, nil
	}
	lt, rt := t.exprGoType(exp.Left), t.exprGoType(exp.Right)
	isPoint := func(typ string) bool { return typ == "iec.DATE" || typ == "iec.DT" || typ == "iec.TOD" }
	if !isPoint(lt) {
		return false, nil
	}
	left, err := t.expressionString(exp.Left)
	if err != nil {
		return true, err
	}
	right, err := t.expressionString(exp.Right)
	if err != nil {
		return true, err
	}
	switch {
	case isPoint(rt) && exp.Operator == "-":
		t.write("iec.TIME(time.Time(%s).Sub(time.Time(%s)))", left, right)
	case rt == "iec.TIME" || rt == "":
		sign := ""
		if exp.Operator == "-" {
			sign = "-"
		}
		t.write("%s(time.Time(%s).Add(%stime.Duration(%s)))", lt, left, sign, right)
	default:
		return false, nil
	}
	return true, nil
}

// realLiteral returns the Go literal of a real value: the shortest decimal
// that keeps it, with a decimal point so that Go takes it as a float.
func realLiteral(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eEIN") {
		s += ".0"
	}
	return s
}

// stringLength returns the declared length of a STRING(n) assignment
// target, or 0.
func (t *Transpiler) stringLength(target ast.Expression) int64 {
	td := t.resolveAssignmentTargetType(target)
	if td == nil || td.StringLength == nil {
		return 0
	}
	if _, isIndex := target.(*ast.IndexExpression); isIndex {
		return 0
	}
	n, ok := constantInteger(td.StringLength)
	if !ok {
		return 0
	}
	return n
}

// stringCutHelper keeps the first n characters of a string, for an
// assignment to a STRING(n).
const stringCutHelper = `
// stringCut returns the first n characters of s, as a STRING(n) keeps them.
func stringCut(s iec.STRING, n int) iec.STRING {
	for i := range string(s) {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}
`

// transpileTimeScaling writes a TIME multiplied or divided by a number, as
// IEC 61131-3 allows (TIME * ANY_NUM, ANY_NUM * TIME, TIME / ANY_NUM): by an
// integer as a TIME, and by a real through float64. It returns false for
// any other operation.
func (t *Transpiler) transpileTimeScaling(exp *ast.InfixExpression) (bool, error) {
	lt, rt := t.exprGoType(exp.Left), t.exprGoType(exp.Right)
	duration, number := exp.Left, exp.Right
	numberType := rt
	switch {
	case lt == "iec.TIME" && rt != "iec.TIME":
	case rt == "iec.TIME" && lt != "iec.TIME" && exp.Operator == "*":
		duration, number, numberType = exp.Right, exp.Left, lt
	default:
		return false, nil
	}
	if numberType == "" && untypedLiteral(number) == "" {
		return false, nil
	}
	d, err := t.expressionString(duration)
	if err != nil {
		return true, err
	}
	n, err := t.expressionString(number)
	if err != nil {
		return true, err
	}
	if isRealGoType(numberType) || untypedLiteral(number) == "real" {
		t.write("iec.TIME(float64(%s) %s float64(%s))", d, exp.Operator, n)
		return true, nil
	}
	t.write("(%s %s iec.TIME(%s))", d, exp.Operator, n)
	return true, nil
}
