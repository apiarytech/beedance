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

// The debugger's hooks in the generated Go (Options.Debug), for package
// vmdebug's go engine: a dbgAt call before each statement with its place in
// the source, dbgEnter and dbgLeave around each POU's body, a variable
// table (dbgVars) on each program, function block and structure, built
// without reflection, the locals of each function and method given to
// dbgEnter, and the located variables' entries in the process image
// (dbgPoints). The hooks go through dbgHook, nil unless a debugger sets
// it, so the file runs as it does without them. The runtime that speaks
// to the debugger is the engine's (vmdebug/engine_go.go), in the same
// package.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// dbgLocated is a located variable the debugger reads and writes in the
// process image.
type dbgLocated struct {
	address, goType, slot, slotType string
}

// debugAt writes the statement hook before stmt, if it is a statement that
// runs and has a place in the source.
func (t *Transpiler) debugAt(stmt ast.Statement) {
	if !t.Debug {
		return
	}
	switch stmt.(type) {
	case *ast.AssignmentStatement, *ast.ExpressionStatement, *ast.IfStatement,
		*ast.CaseStatement, *ast.ForLoopStatement, *ast.WhileStatement,
		*ast.RepeatStatement, *ast.ReturnStatement, *ast.ExitStatement,
		*ast.ContinueStatement, *ast.JumpStatement, *ast.IlInstructionStatement:
	default:
		return
	}
	row, col := debugPos(stmt)
	if row > 0 {
		t.write("\tdbgAt(%d, %d)\n", row, col)
	}
}

// debugPos is node's position, or 0 for a node that has none.
func debugPos(node ast.Node) (row, col int) {
	defer func() {
		if recover() != nil {
			row, col = 0, 0
		}
	}()
	return node.Pos()
}

// debugEnter writes the hooks around a body: pou is the POU's name, vars
// a Go expression giving its variables (a dbgVars method, or a function
// literal of the locals).
func (t *Transpiler) debugEnter(pou, vars string) {
	if !t.Debug {
		return
	}
	t.write("\tdbgEnter(%q, %s)\n\tdefer dbgLeave()\n", pou, vars)
}

// debugLocals is a function literal giving the locals of a function or
// method: its parameters (an in-out one through its pointer), its results
// and its VARs, and, with receiver, the receiver's variables after them.
func (t *Transpiler) debugLocals(name string, returnType ast.Expression, inputs, outputs, inOuts, vars []*ast.VarDeclStatement, receiver string) string {
	var items []string
	if returnType != nil {
		items = append(items, t.debugVarLiteral(name, "", name, returnType))
	}
	for _, p := range inputs {
		items = append(items, t.debugVarLiteral(p.Name.Value, "", p.Name.Value, p.DataType))
	}
	for _, p := range inOuts {
		items = append(items, t.debugVarLiteral(p.Name.Value, "", "(*"+p.Name.Value+")", p.DataType))
	}
	for _, p := range outputs {
		items = append(items, t.debugVarLiteral(p.Name.Value, "", p.Name.Value, p.DataType))
	}
	for _, v := range vars {
		if _, macro := v.Value.(*ast.MacroLiteral); macro {
			continue
		}
		items = append(items, t.debugVarLiteral(v.Name.Value, "", v.Name.Value, v.DataType))
	}
	list := "[]dbgVar{" + strings.Join(items, ", ") + "}"
	if receiver != "" {
		return fmt.Sprintf("func() []dbgVar { return append(%s, %s.dbgVars()...) }", list, receiver)
	}
	return fmt.Sprintf("func() []dbgVar { return %s }", list)
}

// transpileDebugVars writes the dbgVars method of the struct typeName: its
// variables of blocks, after those of the struct it extends, if any.
func (t *Transpiler) transpileDebugVars(typeName, extends string, blocks ...[]*ast.VarDeclStatement) {
	if !t.Debug {
		return
	}
	t.write("// dbgVars lists %s's variables for the debugger.\n", typeName)
	t.write("func (p *%s) dbgVars() []dbgVar {\n", typeName)
	if extends != "" {
		t.write("\tout := p.%s.dbgVars()\n", extends)
	} else {
		t.write("\tvar out []dbgVar\n")
	}
	for _, block := range blocks {
		for _, decl := range block {
			if decl == nil || decl.Name == nil || decl.DataType == nil {
				continue
			}
			if _, macro := decl.Value.(*ast.MacroLiteral); macro {
				continue
			}
			address := ""
			if decl.Location != nil && decl.Location.Location != nil {
				address = decl.Location.Location.String()
			}
			t.write("\tout = append(out, %s)\n", t.debugVarLiteral(decl.Name.Value, address, "p."+decl.Name.Value, decl.DataType))
		}
	}
	t.write("\treturn out\n}\n\n")
}

// debugVarLiteral is a dbgVar for the variable name of type dataType, held
// in the Go expression expr (addressable).
func (t *Transpiler) debugVarLiteral(name, address, expr string, dataType ast.Expression) string {
	typ := dataType.String()
	if ad, ok := dataType.(*ast.ArrayDefinition); ok {
		typ = ad.String()
	}
	lit := fmt.Sprintf("dbgVar{Name: %q, Type: %q, Address: %q", name, typ, address)
	text, set, members := t.debugAccess(expr, dataType)
	if text != "" {
		lit += ", Text: " + text
	}
	if set != "" {
		lit += ", Set: " + set
	}
	if members != "" {
		lit += ", Members: " + members
	}
	return lit + "}"
}

// debugAccess gives the Text, Set and Members functions (Go source) of the
// value of type dataType in expr; empty where the type has none.
func (t *Transpiler) debugAccess(expr string, dataType ast.Expression) (text, set, members string) {
	if ad, ok := dataType.(*ast.ArrayDefinition); ok {
		return t.debugArrayAccess(expr, ad)
	}
	switch dataType.(type) {
	case *ast.RefToType, *ast.ReferenceType, *ast.StructDefinition:
		return "", "", ""
	}
	goType := t.mapIecTypeToGo(dataType)
	// A type the program declares: its definition says what it is.
	if def, ok := t.typeInfo[goType]; ok {
		switch d := def.(type) {
		case *ast.FunctionBlockDeclaration:
			return "", "", expr + ".dbgVars"
		case *ast.TypeDeclaration:
			if _, isStruct := d.DataType.(*ast.StructDefinition); isStruct {
				return "", "", expr + ".dbgVars"
			}
			if _, isEnum := d.DataType.(*ast.EnumDefinition); isEnum {
				return debugScalar(expr, goType, "int")
			}
			if d.DataType != nil {
				return t.debugAccess(expr, d.DataType)
			}
			return "", "", ""
		default:
			return "", "", ""
		}
	}
	if fields, ok := debugStandardFBs[goType]; ok {
		var items []string
		for _, f := range fields {
			tx, st, _ := debugScalar(expr+"."+f.name, "iec."+f.typ, debugFamily(f.typ))
			items = append(items, fmt.Sprintf("dbgVar{Name: %q, Type: %q, Text: %s, Set: %s}", f.name, f.typ, tx, st))
		}
		return "", "", "func() []dbgVar { return []dbgVar{" + strings.Join(items, ", ") + "} }"
	}
	if strings.HasPrefix(goType, "iec.") {
		return debugScalar(expr, goType, debugFamily(strings.TrimPrefix(goType, "iec.")))
	}
	return "", "", ""
}

// debugArrayAccess gives the elements of an array of one dimension with
// constant bounds; an array of more has no members shown.
func (t *Transpiler) debugArrayAccess(expr string, ad *ast.ArrayDefinition) (text, set, members string) {
	if len(ad.Ranges) != 1 || ad.HasAnyRange() {
		return "", "", ""
	}
	infix, ok := ad.Ranges[0].(*ast.InfixExpression)
	if !ok || infix.Operator != ".." {
		return "", "", ""
	}
	low, ok := constantInteger(infix.Left)
	if !ok {
		return "", "", ""
	}
	elem := t.debugVarLiteral("", "", expr+"[i]", ad.DataType)
	elem = strings.Replace(elem, `Name: ""`, fmt.Sprintf(`Name: "[" + strconv.FormatInt(int64(i)+%d, 10) + "]"`, low), 1)
	return "", "", fmt.Sprintf("func() []dbgVar { out := make([]dbgVar, 0, len(%s)); for i := range %s { out = append(out, %s) }; return out }", expr, expr, elem)
}

// debugFamily is how a value of the elementary IEC type typ is read and
// written: bool, int, uint, bits, real, string, wstring, time, or "" for
// one shown as Go prints it.
func debugFamily(typ string) string {
	switch strings.ToUpper(typ) {
	case "BOOL":
		return "bool"
	case "SINT", "INT", "DINT", "LINT":
		return "int"
	case "USINT", "UINT", "UDINT", "ULINT":
		return "uint"
	case "BYTE", "WORD", "DWORD", "LWORD":
		return "bits"
	case "REAL", "LREAL":
		return "real"
	case "STRING", "CHAR":
		return "string"
	case "WSTRING", "WCHAR":
		return "wstring"
	case "TIME", "LTIME":
		return "time"
	}
	return ""
}

// debugScalar gives the Text and Set functions of a scalar of Go type
// goType in expr, read and written as family says.
func debugScalar(expr, goType, family string) (text, set, members string) {
	switch family {
	case "bool":
		return fmt.Sprintf("func() string { return dbgBoolText(bool(%s)) }", expr),
			fmt.Sprintf("func(s string) error { v, err := dbgParseBool(s); if err == nil { %s = %s(v) }; return err }", expr, goType), ""
	case "int":
		return fmt.Sprintf("func() string { return strconv.FormatInt(int64(%s), 10) }", expr),
			fmt.Sprintf("func(s string) error { v, err := dbgParseInt(s); if err == nil { %s = %s(v) }; return err }", expr, goType), ""
	case "uint":
		return fmt.Sprintf("func() string { return strconv.FormatUint(uint64(%s), 10) }", expr),
			fmt.Sprintf("func(s string) error { v, err := dbgParseUint(s); if err == nil { %s = %s(v) }; return err }", expr, goType), ""
	case "bits":
		return fmt.Sprintf("func() string { return \"16#\" + strconv.FormatUint(uint64(%s), 16) }", expr),
			fmt.Sprintf("func(s string) error { v, err := dbgParseUint(s); if err == nil { %s = %s(v) }; return err }", expr, goType), ""
	case "real":
		return fmt.Sprintf("func() string { return dbgRealText(float64(%s)) }", expr),
			fmt.Sprintf("func(s string) error { v, err := dbgParseReal(s); if err == nil { %s = %s(v) }; return err }", expr, goType), ""
	case "string":
		return fmt.Sprintf("func() string { return \"'\" + string(%s) + \"'\" }", expr),
			fmt.Sprintf("func(s string) error { %s = %s(dbgParseString(s)); return nil }", expr, goType), ""
	case "wstring":
		return fmt.Sprintf("func() string { return `\"` + string(%s) + `\"` }", expr),
			fmt.Sprintf("func(s string) error { %s = %s(dbgParseString(s)); return nil }", expr, goType), ""
	case "time":
		return fmt.Sprintf("func() string { return dbgTimeText(time.Duration(%s)) }", expr),
			fmt.Sprintf("func(s string) error { v, err := dbgParseTime(s); if err == nil { %s = %s(v) }; return err }", expr, goType), ""
	}
	return fmt.Sprintf("func() string { return fmt.Sprint(%s) }", expr), "", ""
}

// debugStandardFBs are the variables of royaljelly's standard function
// blocks, by Go type, as the debugger shows an instance.
var debugStandardFBs = map[string][]struct{ name, typ string }{
	"timers.TON":      {{"IN", "BOOL"}, {"PT", "TIME"}, {"Q", "BOOL"}, {"ET", "TIME"}},
	"timers.TOF":      {{"IN", "BOOL"}, {"PT", "TIME"}, {"Q", "BOOL"}, {"ET", "TIME"}},
	"timers.TP":       {{"IN", "BOOL"}, {"PT", "TIME"}, {"Q", "BOOL"}, {"ET", "TIME"}},
	"counters.CTU":    {{"CU", "BOOL"}, {"R", "BOOL"}, {"PV", "INT"}, {"Q", "BOOL"}, {"CV", "INT"}},
	"counters.CTD":    {{"CD", "BOOL"}, {"LD", "BOOL"}, {"PV", "INT"}, {"Q", "BOOL"}, {"CV", "INT"}},
	"counters.CTUD":   {{"CU", "BOOL"}, {"CD", "BOOL"}, {"R", "BOOL"}, {"LD", "BOOL"}, {"PV", "INT"}, {"QU", "BOOL"}, {"QD", "BOOL"}, {"CV", "INT"}},
	"triggers.R_TRIG": {{"CLK", "BOOL"}, {"Q", "BOOL"}},
	"triggers.F_TRIG": {{"CLK", "BOOL"}, {"Q", "BOOL"}},
	"triggers.SR_FB":  {{"S1", "BOOL"}, {"R", "BOOL"}, {"Q1", "BOOL"}},
	"triggers.RS_FB":  {{"S", "BOOL"}, {"R1", "BOOL"}, {"Q1", "BOOL"}},
}

// noteDebugLocated keeps a located variable's entry in the process image,
// for dbgPoints.
func (t *Transpiler) noteDebugLocated(address, goType, slot, slotType string) {
	if !t.Debug {
		return
	}
	for _, l := range t.debugLocated {
		if strings.EqualFold(l.address, address) {
			return
		}
	}
	t.debugLocated = append(t.debugLocated, dbgLocated{address: address, goType: goType, slot: slot, slotType: slotType})
}

// transpileDebugTail writes what the hooks and tables refer to: the types,
// the hook functions, the value helpers and dbgPoints.
func (t *Transpiler) transpileDebugTail() {
	if !t.Debug {
		return
	}
	t.write("%s", debugRuntimeSource)
	t.write("// dbgPoints are the located variables' entries in the process image, for the debugger.\n")
	t.write("func dbgPoints() []dbgPoint {\n\treturn []dbgPoint{\n")
	for _, l := range t.debugLocated {
		iecType := strings.TrimPrefix(l.goType, "iec.")
		family := debugFamily(iecType)
		if family == "" || !strings.HasPrefix(l.goType, "iec.") {
			continue
		}
		// The point is read into pv, and written from it (the Set functions
		// declare a v of their own).
		read := fmt.Sprintf("func() string { var pv %s; processImage.Read(func(img *%s) { pv = %s(%s) }); return %s }",
			l.goType, t.imageType(), l.goType, l.slot, strings.TrimSuffix(strings.TrimPrefix(mustText(debugScalar("pv", l.goType, family)), "func() string { return "), " }"))
		_, setV, _ := debugScalar("pv", l.goType, family)
		write := fmt.Sprintf("func(s string) error { var pv %s; err := (%s)(s); if err == nil { processImage.Write(func(img *%s) { %s = %s(pv) }) }; return err }",
			l.goType, setV, t.imageType(), l.slot, l.slotType)
		t.write("\t\t{Address: %q, Type: %q, Text: %s, Set: %s},\n", l.address, iecType, read, write)
		t.usesProcessImage = true
	}
	t.write("\t}\n}\n")
}

// mustText is the Text function of a debugScalar result.
func mustText(text, _, _ string) string { return text }

// debugRuntimeSource is the Go the hooks and tables need, in the generated
// file. The hook functions are called from everywhere; a program's host
// sets dbgHook. strconv, time and fmt are imported as used.
const debugRuntimeSource = `
// dbgVar is a variable as the debugger reads and writes it: its value as
// the source writes a literal (Text, Set), or its members (Members).
type dbgVar struct {
	Name, Type, Address string
	Text                func() string
	Set                 func(string) error
	Members             func() []dbgVar
}

// dbgPoint is a located variable's entry in the process image.
type dbgPoint struct {
	Address, Type string
	Text          func() string
	Set           func(string) error
}

// dbgHooks is what a debugger watches the program through.
type dbgHooks interface {
	At(row, col int)
	Enter(pou string, vars func() []dbgVar)
	Leave()
}

var dbgHook dbgHooks

func dbgAt(row, col int) {
	if dbgHook != nil {
		dbgHook.At(row, col)
	}
}

func dbgEnter(pou string, vars func() []dbgVar) {
	if dbgHook != nil {
		dbgHook.Enter(pou, vars)
	}
}

func dbgLeave() {
	if dbgHook != nil {
		dbgHook.Leave()
	}
}

func dbgBoolText(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

func dbgParseBool(s string) (bool, error) {
	switch strings.ToUpper(dbgLiteral(s)) {
	case "TRUE", "1":
		return true, nil
	case "FALSE", "0":
		return false, nil
	}
	return false, fmt.Errorf("%s is not a BOOL", s)
}

// dbgLiteral is s without a type prefix (INT#5) and underscores.
func dbgLiteral(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "#"); i > 0 && !strings.ContainsAny(s[:i], "0123456789") {
		s = s[i+1:]
	}
	return strings.ReplaceAll(s, "_", "")
}

func dbgParseInt(s string) (int64, error) {
	s = dbgLiteral(s)
	base := 10
	if b, digits, ok := strings.Cut(s, "#"); ok {
		n, err := strconv.Atoi(b)
		if err != nil {
			return 0, fmt.Errorf("%s is not an integer", s)
		}
		base, s = n, digits
	}
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not an integer", s)
	}
	return n, nil
}

func dbgParseUint(s string) (uint64, error) {
	s = dbgLiteral(s)
	base := 10
	if b, digits, ok := strings.Cut(s, "#"); ok {
		n, err := strconv.Atoi(b)
		if err != nil {
			return 0, fmt.Errorf("%s is not an integer", s)
		}
		base, s = n, digits
	}
	n, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not an unsigned integer", s)
	}
	return n, nil
}

func dbgRealText(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func dbgParseReal(s string) (float64, error) {
	f, err := strconv.ParseFloat(dbgLiteral(s), 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a number", s)
	}
	return f, nil
}

func dbgParseString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// dbgTimeText is d as the source writes it: T#1h2m3s4ms.
func dbgTimeText(d time.Duration) string {
	if d == 0 {
		return "T#0s"
	}
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	var b strings.Builder
	b.WriteString("T#" + sign)
	for _, u := range []struct {
		n    time.Duration
		name string
	}{{24 * time.Hour, "d"}, {time.Hour, "h"}, {time.Minute, "m"}, {time.Second, "s"}, {time.Millisecond, "ms"}, {time.Microsecond, "us"}, {time.Nanosecond, "ns"}} {
		if d >= u.n {
			b.WriteString(strconv.FormatInt(int64(d/u.n), 10) + u.name)
			d %= u.n
		}
	}
	return b.String()
}

// dbgParseTime reads a TIME literal: T#1h30m, TIME#500ms, 2.5s.
func dbgParseTime(s string) (time.Duration, error) {
	s = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "_", ""))
	s = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(s, "time#"), "t#"), "lt#")
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	units := []struct {
		name string
		n    time.Duration
	}{{"ms", time.Millisecond}, {"us", time.Microsecond}, {"ns", time.Nanosecond}, {"d", 24 * time.Hour}, {"h", time.Hour}, {"m", time.Minute}, {"s", time.Second}}
	var total time.Duration
	for s != "" {
		i := 0
		for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
			i++
		}
		if i == 0 {
			return 0, fmt.Errorf("%s is not a TIME", s)
		}
		f, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, fmt.Errorf("%s is not a TIME", s)
		}
		s = s[i:]
		found := false
		for _, u := range units {
			if strings.HasPrefix(s, u.name) {
				total += time.Duration(f * float64(u.n))
				s = s[len(u.name):]
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("%s: unknown unit", s)
		}
	}
	if neg {
		total = -total
	}
	return total, nil
}
`
