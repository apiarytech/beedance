/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
)

const fidelitySource = `TYPE
	T_Mode : (IDLE, RUN);
	T_Limit : INT := 50;
END_TYPE

PROGRAM Main
	VAR CONSTANT
		MaxCount : INT := 10;
	END_VAR
	VAR RETAIN
		Total : DINT;
	END_VAR
	VAR
		Name : STRING[20] := 'it$'s $$ok';
		Wide : WSTRING[5] := "w";
		Delay : TIME := T#5s;
		Day : DATE := D#2026-01-01;
		Offset : INT := -5;
		Ref : REFERENCE TO INT;
		Button AT %IX0.1 : BOOL;
	END_VAR

	(* Count up to the limit. *)
	IF Total < MaxCount THEN
		Total := Total + 1; // One more.
	END_IF;
END_PROGRAM

PROGRAM IlProg
	VAR x : INT; END_VAR
	LD x
	ADD 1
	ST x
END_PROGRAM

CONFIGURATION Plant
	VAR_GLOBAL
		Start : BOOL;
	END_VAR
	RESOURCE Cpu ON PLC
		VAR_GLOBAL
			Speed : INT := 3;
		END_VAR
		TASK Fast (INTERVAL := T#20ms, PRIORITY := 1);
		TASK OnStart (SINGLE := Start, PRIORITY := 2);
		PROGRAM M1 WITH Fast : Main;
		PROGRAM I1 WITH OnStart : IlProg;
		PROGRAM Free : Main;
	END_RESOURCE
END_CONFIGURATION
`

// Exporting from source keeps bodies as written, and every declaration
// survives a round trip through XML and back.
func TestExportSourceFidelity(t *testing.T) {
	xmlBytes, err := ExportSourceToXML(fidelitySource, "Fidelity")
	if err != nil {
		t.Fatal(err)
	}
	xmlStr := string(xmlBytes)
	for _, want := range []string{
		// Bodies as written, with comments; an IL body in an IL element.
		"(* Count up to the limit. *)\nIF Total < MaxCount THEN\n\tTotal := Total + 1; // One more.\nEND_IF;",
		"<IL>", "LD x\nADD 1\nST x",
		// Qualifiers, strings, lengths, prefixes and references.
		`<localVars constant="true">`, `<localVars retain="true">`,
		`<string length="20">`, `<wstring length="5">`,
		`<simpleValue value="&#39;it$&#39;s $$ok&#39;">`,
		`<simpleValue value="T#5s">`, `<simpleValue value="D#2026-01-01">`, `<simpleValue value="-5">`,
		`<pointer>`, `address="%IX0.1"`,
		// Types' initial values, and the configuration's globals and tasks.
		`<dataType name="T_Limit">`, `<simpleValue value="50">`,
		`<globalVars>`, `interval="T#20ms"`, `single="Start"`,
	} {
		if !strings.Contains(xmlStr, want) {
			t.Errorf("expected %q in:\n%s", want, xmlStr)
		}
	}
	if err := ValidateWithXSD(xmlBytes, ""); err != nil {
		t.Fatalf("validation: %v", err)
	}

	st, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"T_Limit : INT := 50;",
		"VAR CONSTANT", "VAR RETAIN",
		"Name : STRING[20] := 'it$'s $$ok';", "Wide : WSTRING[5] := \"w\";",
		"Delay : TIME := T#5s;", "Day : DATE := D#2026-01-01;", "Offset : INT := -5;",
		"Ref : REFERENCE TO INT;", "Button AT %IX0.1 : BOOL;",
		"TASK Fast (INTERVAL := T#20ms, PRIORITY := 1);",
		"TASK OnStart (SINGLE := Start, PRIORITY := 2);",
		"PROGRAM M1 WITH Fast : Main;", "PROGRAM I1 WITH OnStart : IlProg;", "PROGRAM Free : Main;",
		"Speed : INT := 3;",
	} {
		if !strings.Contains(st, want) {
			t.Errorf("expected %q in:\n%s", want, st)
		}
	}
	prog, err := ImportToAST(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToAST: %v\n%s", err, st)
	}
	if len(prog.Statements) != 4 {
		t.Errorf("expected TYPE, 2 PROGRAMs and a CONFIGURATION, got %d statements", len(prog.Statements))
	}

	// Exporting the imported text again gives the same project.
	again, err := ExportSourceToXML(st, "Fidelity")
	if err != nil {
		t.Fatal(err)
	}
	if stripDate(string(again)) != stripDate(xmlStr) {
		t.Errorf("a second round trip changed the XML:\n%s\n---\n%s", again, xmlStr)
	}
}

// stripDate removes the creation time, which differs between exports.
func stripDate(s string) string {
	i := strings.Index(s, `creationDateTime="`)
	if i < 0 {
		return s
	}
	j := strings.Index(s[i+18:], `"`)
	return s[:i] + s[i+18+j:]
}

// Without the source, a body is written in the AST's String form.
func TestExportWithoutSource(t *testing.T) {
	p := parser.New(lexer.New("PROGRAM P VAR x : INT; END_VAR x := x + 1; END_PROGRAM"))
	proj, err := Export(p.ParseProgram(), "")
	if err != nil {
		t.Fatal(err)
	}
	if proj.ContentHeader.Name != "BeedanceProject" {
		t.Errorf("default project name = %s", proj.ContentHeader.Name)
	}
	if text := proj.Types.Pous.Pous[0].Body.ST.Text; !strings.Contains(text, "x := (x + 1);") {
		t.Errorf("body = %s", text)
	}
}

// Declarations PLCopen TC6 cannot hold, and bad configurations, are errors.
func TestExportErrors(t *testing.T) {
	for _, tt := range []struct{ source, want string }{
		{"NAMESPACE Lib FUNCTION F : INT F := 1; END_FUNCTION END_NAMESPACE", "cannot hold NAMESPACE Lib"},
		{"INTERFACE IRun METHOD Run : BOOL; END_INTERFACE", "cannot hold INTERFACE IRun"},
		{"VAR_GLOBAL g : INT; END_VAR", "cannot hold a VAR_GLOBAL block"},
		{"PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE R1 ON PLC TASK T1 (INTERVAL := T#1s, PRIORITY := k); END_RESOURCE END_CONFIGURATION", "PRIORITY must be a constant integer"},
		{"PROGRAM P VAR x : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE R1 ON PLC PROGRAM P1 WITH Nope : P; END_RESOURCE END_CONFIGURATION", "names task 'Nope'"},
		{"PROGRAM P VAR x : INT END_PROGRAM", "parser errors"},
	} {
		if _, err := ExportSourceToXML(tt.source, "E"); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", tt.source, tt.want, err)
		}
	}
	if _, err := ExportToXML(&ast.Program{Statements: []ast.Statement{&ast.FunctionDeclaration{Name: &ast.Identifier{Value: "F"}}}}, "E"); err == nil || !strings.Contains(err.Error(), "has no return type") {
		t.Errorf("expected a missing return type error, got %v", err)
	}
	dir := t.TempDir()
	if err := ConvertIECToXMLFile(filepath.Join(dir, "missing.st"), filepath.Join(dir, "out.xml")); err == nil {
		t.Error("expected an error for a missing file")
	}
	bad := filepath.Join(dir, "bad.st")
	os.WriteFile(bad, []byte("NAMESPACE N END_NAMESPACE"), 0644)
	if err := ConvertIECToXMLFile(bad, filepath.Join(dir, "out.xml")); err == nil || !strings.Contains(err.Error(), "export xml") {
		t.Errorf("expected an export error, got %v", err)
	}
	// A file's project is named after the file, not its path.
	good := filepath.Join(dir, "motor.st")
	os.WriteFile(good, []byte("PROGRAM P VAR x : INT; END_VAR x := 1; END_PROGRAM"), 0644)
	out := filepath.Join(dir, "motor.xml")
	if err := ConvertIECToXMLFile(good, out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `<contentHeader name="motor">`) {
		t.Errorf("project name is not the file's base name:\n%s", data)
	}
}

const codesysProject = `<?xml version="1.0" encoding="utf-8"?>
<project xmlns="http://www.plcopen.org/xml/tc6_0201">
 <fileHeader companyName="x" productName="CODESYS" productVersion="3" creationDateTime="2026-01-01T00:00:00"/>
 <contentHeader name="p"><coordinateInfo><fbd><scaling x="1" y="1"/></fbd><ld><scaling x="1" y="1"/></ld><sfc><scaling x="1" y="1"/></sfc></coordinateInfo></contentHeader>
 <types><dataTypes/><pous>
  <pou name="PLC_PRG" pouType="program">
   <interface><localVars><variable name="i"><type><INT/></type></variable></localVars></interface>
   <body><ST><xhtml xmlns="http://www.w3.org/1999/xhtml">i := i + 1;
IF i &gt; 10 THEN i := 0; END_IF</xhtml></ST></body>
  </pou>%s</pous></types>
 <instances><configurations>
  <configuration name="C"><resource name="R"><task name="T" priority="1" interval="PT0.02S"><pouInstance name="P1" typeName="PLC_PRG"/></task></resource></configuration>
 </configurations></instances>
</project>`

// Other tools' XHTML and durations are read; graphical bodies are refused.
func TestImportOtherTools(t *testing.T) {
	st, err := ImportToIECText([]byte(strings.Replace(codesysProject, "%s", "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\ti := i + 1;\n\tIF i > 10 THEN i := 0; END_IF\n", "INTERVAL := T#0.02s"} {
		if !strings.Contains(st, want) {
			t.Errorf("expected %q in:\n%s", want, st)
		}
	}
	for _, tt := range []struct{ pou, want string }{
		{`<pou name="Ladder" pouType="program"><body><LD><leftPowerRail localId="1"/></LD></body></pou>`, "graphical bodies"},
		{`<pou name="F" pouType="function"><body><ST><xhtml:p xmlns:xhtml="http://www.w3.org/1999/xhtml">F := 1;</xhtml:p></ST></body></pou>`, "function 'F' has no return type"},
		{`<pou name="X" pouType="class"><body/></pou>`, "unknown pouType 'class'"},
	} {
		if _, err := ImportToIECText([]byte(strings.Replace(codesysProject, "%s", tt.pou, 1))); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: expected an error containing %q, got %v", tt.pou, tt.want, err)
		}
	}
	if _, err := ImportToIECText([]byte("<project")); err == nil {
		t.Error("expected an error for malformed XML")
	}
	if _, err := ImportToAST([]byte(strings.Replace(codesysProject, "%s", `<pou name="Bad" pouType="program"><body><ST><xhtml xmlns="http://www.w3.org/1999/xhtml">x := ;</xhtml></ST></body></pou>`, 1))); err == nil || !strings.Contains(err.Error(), "parsing generated IEC code") {
		t.Errorf("expected a parse error, got %v", err)
	}
	if _, err := ConvertXMLToIECText(filepath.Join(t.TempDir(), "missing.xml")); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestFormattedText(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"<xhtml:p><![CDATA[a := 1;]]></xhtml:p>", "a := 1;"},
		{`<xhtml xmlns="http://www.w3.org/1999/xhtml">a &lt; b</xhtml>`, "a < b"},
		{"<xhtml:p>a := 1;<xhtml:br/>b := 2;</xhtml:p><xhtml:p>c := 3;</xhtml:p>", "a := 1;\nb := 2;\nc := 3;"},
		{"<xhtml:p><![CDATA[s := ']]]]><![CDATA[>';]]></xhtml:p>", "s := ']]>';"},
	} {
		if got := CleanFormattedText(tt.in); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := CleanFormattedText("<unclosed"); got != "<unclosed" {
		t.Errorf("malformed text: got %q", got)
	}
	// Code holding "]]>" survives export.
	if got := CleanFormattedText(xhtmlText("s := ']]>';")); got != "s := ']]>';" {
		t.Errorf("CDATA split: got %q", got)
	}
}

func TestDurationText(t *testing.T) {
	for in, want := range map[string]string{"T#20ms": "T#20ms", "TIME#1s": "TIME#1s", "20ms": "T#20ms", "PT0.5S": "T#0.5s", "CycleVar": "CycleVar"} {
		if got := durationText(in); got != want {
			t.Errorf("durationText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateProjectRules(t *testing.T) {
	valid := func() *Project {
		p, err := ExportSource("FUNCTION F : INT F := 1; END_FUNCTION CONFIGURATION C RESOURCE R1 ON PLC TASK T1 (PRIORITY := 1); PROGRAM P1 WITH T1 : F; END_RESOURCE END_CONFIGURATION", "V")
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := ValidateProject(valid()); err != nil {
		t.Fatalf("valid project: %v", err)
	}
	for _, tt := range []struct {
		name   string
		break_ func(p *Project)
		want   string
	}{
		{"root", func(p *Project) { p.XMLName.Local = "proj" }, "invalid root element"},
		{"namespace", func(p *Project) { p.XMLName.Local = "project"; p.XMLName.Space = "urn:x" }, "invalid namespace"},
		{"product", func(p *Project) { p.FileHeader.ProductName = "" }, "productName is required"},
		{"version", func(p *Project) { p.FileHeader.ProductVersion = "" }, "productVersion is required"},
		{"date", func(p *Project) { p.FileHeader.CreationDateTime = "" }, "creationDateTime is required"},
		{"content", func(p *Project) { p.ContentHeader.Name = "" }, "contentHeader.name is required"},
		{"dataType", func(p *Project) { p.Types.DataTypes.DataTypes = []DataTypeDecl{{}} }, "dataType[0].name is required"},
		{"pou name", func(p *Project) { p.Types.Pous.Pous[0].Name = "" }, "pou[0].name is required"},
		{"pou twice", func(p *Project) { p.Types.Pous.Pous = append(p.Types.Pous.Pous, p.Types.Pous.Pous[0]) }, "declared twice"},
		{"pouType case", func(p *Project) { p.Types.Pous.Pous[0].PouType = "Function" }, "invalid pouType 'Function'"},
		{"return type", func(p *Project) { p.Types.Pous.Pous[0].Interface.ReturnType = nil }, "returnType is required"},
		{"variable", func(p *Project) {
			p.Types.Pous.Pous[0].Interface.LocalVars = []VarList{{Variables: []Variable{{}}}}
		}, "variable[0].name is required"},
		{"config", func(p *Project) { p.Instances.Configurations.Configurations[0].Name = "" }, "configuration.name is required"},
		{"resource", func(p *Project) { p.Instances.Configurations.Configurations[0].Resources[0].Name = "" }, "resource[0].name is required"},
		{"task", func(p *Project) { p.Instances.Configurations.Configurations[0].Resources[0].Tasks[0].Name = "" }, "task[0].name is required"},
		{"priority", func(p *Project) { p.Instances.Configurations.Configurations[0].Resources[0].Tasks[0].Priority = 70000 }, "out of range"},
		{"task instance", func(p *Project) {
			p.Instances.Configurations.Configurations[0].Resources[0].Tasks[0].PouInstances[0].TypeName = ""
		}, "requires both name and typeName"},
		{"resource instance", func(p *Project) {
			p.Instances.Configurations.Configurations[0].Resources[0].PouInstances = []POUInstance{{Name: "X"}}
		}, "requires both name and typeName"},
	} {
		p := valid()
		tt.break_(p)
		if err := ValidateProject(p); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: expected an error containing %q, got %v", tt.name, tt.want, err)
		}
	}
	if err := ValidateProject(nil); err == nil {
		t.Error("expected an error for a nil project")
	}
	if err := ValidateWithXSD([]byte("<project"), ""); err == nil {
		t.Error("expected an error for malformed XML")
	}
	if err := ValidateWithXSD([]byte(`<project xmlns="http://www.plcopen.org/xml/tc6_0201"/>`), ""); err == nil || !strings.Contains(err.Error(), "companyName is required") {
		t.Errorf("expected a structural error, got %v", err)
	}
	if len(SchemaXSD()) == 0 {
		t.Error("the schema is not embedded")
	}
	if _, err := FindSchemaFile("nope.xsd"); err == nil {
		t.Error("expected an error for a missing schema file")
	}
	t.Logf("xmllint available: %v", XSDValidatorAvailable())
}

// Data types round-trip through their XML form.
func TestDataTypeStrings(t *testing.T) {
	for _, name := range []string{"BOOL", "BYTE", "WORD", "DWORD", "LWORD", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT",
		"REAL", "LREAL", "TIME", "DATE", "DT", "TOD", "STRING", "WSTRING", "MyType", "INT(1..5)", "UINT(0..9)"} {
		if got := dataTypeFromName(name).String(); got != name {
			t.Errorf("%s round-trips as %s", name, got)
		}
	}
	if got := dataTypeFromName("DATE_AND_TIME").String(); got != "DT" {
		t.Errorf("DATE_AND_TIME = %s", got)
	}
	if got := dataTypeFromName("TIME_OF_DAY").String(); got != "TOD" {
		t.Errorf("TIME_OF_DAY = %s", got)
	}
	if got := dataTypeFromName("(1..5)").String(); got != "INT(1..5)" {
		t.Errorf("(1..5) = %s", got)
	}
	if got := (DataType{}).String(); got != "INT" {
		t.Errorf("empty type = %s", got)
	}
	if got := DataTypeFromAST(nil).String(); got != "INT" {
		t.Errorf("nil type = %s", got)
	}
	enum := DataType{Enum: &EnumType{Values: []EnumValue{{Name: "A"}, {Name: "B"}}}}
	if got := enum.String(); got != "(A, B)" {
		t.Errorf("enum = %s", got)
	}
	st := DataType{Struct: &StructType{Variables: []Variable{{Name: "x", Type: dataTypeFromName("INT")}}}}
	if got := st.String(); !strings.Contains(got, "x : INT;") {
		t.Errorf("struct = %s", got)
	}
	arr := DataType{Array: &ArrayType{Dimensions: []Dimension{{Lower: "0", Upper: "1"}}}}
	if got := arr.String(); got != "ARRAY [0..1] OF INT" {
		t.Errorf("array without base = %s", got)
	}
	usr := DataType{SubrangeUnsigned: &SubrangeType{Range: SubrangeRange{Lower: "0", Upper: "3"}}}
	if got := usr.String(); got != "UINT(0..3)" {
		t.Errorf("unsigned subrange without base = %s", got)
	}
	ssr := DataType{SubrangeSigned: &SubrangeType{Range: SubrangeRange{Lower: "0", Upper: "3"}}}
	if got := ssr.String(); got != "INT(0..3)" {
		t.Errorf("signed subrange without base = %s", got)
	}
	// Values.
	v := &Value{StructValue: &StructValue{Members: []StructMemberValue{
		{Member: "a", SimpleValue: &SimpleValue{Value: "1"}},
		{Member: "b", ArrayValue: &ArrayValue{Values: []ArrayValueElement{{RepetitionValue: "2", SimpleValue: &SimpleValue{Value: "0"}}}}},
		{Member: "c", StructValue: &StructValue{Members: []StructMemberValue{{Member: "d", SimpleValue: &SimpleValue{Value: "4"}}}}},
	}}}
	if got := v.String(); got != "(a := 1, b := [2(0)], c := (d := 4))" {
		t.Errorf("struct value = %s", got)
	}
	nested := &Value{ArrayValue: &ArrayValue{Values: []ArrayValueElement{
		{ArrayValue: &ArrayValue{Values: []ArrayValueElement{{SimpleValue: &SimpleValue{Value: "1"}}}}},
		{StructValue: &StructValue{Members: []StructMemberValue{{Member: "x", SimpleValue: &SimpleValue{Value: "2"}}}}},
		{},
	}}}
	if got := nested.String(); got != "[[1], (x := 2)]" {
		t.Errorf("nested array value = %s", got)
	}
	if got := (&Value{}).String(); got != "" {
		t.Errorf("empty value = %q", got)
	}
}

// Structure and repeated array initial values are exported.
func TestExportStructuredValues(t *testing.T) {
	src := `TYPE Pt : STRUCT x : INT; y : INT; END_STRUCT; END_TYPE
PROGRAM P VAR p : Pt := (x := 1, y := 2); a : ARRAY[0..4] OF INT := [2(7), 3(1)]; m : ARRAY[0..1] OF Pt := [2((x := 5))]; END_VAR END_PROGRAM`
	xmlBytes, err := ExportSourceToXML(src, "S")
	if err != nil {
		t.Fatal(err)
	}
	st, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"p : Pt := (x := 1, y := 2);", "a : ARRAY [0..4] OF INT := [2(7), 3(1)];", "m : ARRAY [0..1] OF Pt := [2((x := 5))];", "x : INT;"} {
		if !strings.Contains(st, want) {
			t.Errorf("expected %q in:\n%s", want, st)
		}
	}
}
