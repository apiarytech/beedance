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
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// Project is the root element defined by http://www.plcopen.org/xml/tc6_0201.
type Project struct {
	XMLName       xml.Name       `xml:"http://www.plcopen.org/xml/tc6_0201 project"`
	XmlnsXhtml    string         `xml:"xmlns:xhtml,attr,omitempty"`
	FileHeader    FileHeader     `xml:"fileHeader"`
	ContentHeader ContentHeader  `xml:"contentHeader"`
	Types         Types          `xml:"types"`
	Instances     Instances      `xml:"instances"`
	AddData       *AddData       `xml:"addData,omitempty"`
	Doc           *FormattedText `xml:"documentation,omitempty"`
}

type FileHeader struct {
	CompanyName        string `xml:"companyName,attr"`
	CompanyURL         string `xml:"companyURL,attr,omitempty"`
	ProductName        string `xml:"productName,attr"`
	ProductVersion     string `xml:"productVersion,attr"`
	ProductRelease     string `xml:"productRelease,attr,omitempty"`
	CreationDateTime   string `xml:"creationDateTime,attr"`
	ContentDescription string `xml:"contentDescription,attr,omitempty"`
}

type ContentHeader struct {
	Name                 string          `xml:"name,attr"`
	Version              string          `xml:"version,attr,omitempty"`
	ModificationDateTime string          `xml:"modificationDateTime,attr,omitempty"`
	Organization         string          `xml:"organization,attr,omitempty"`
	Author               string          `xml:"author,attr,omitempty"`
	Language             string          `xml:"language,attr,omitempty"`
	Comment              string          `xml:"Comment,omitempty"`
	CoordinateInfo       *CoordinateInfo `xml:"coordinateInfo,omitempty"`
	AddDataInfo          *AddDataInfo    `xml:"addDataInfo,omitempty"`
	AddData              *AddData        `xml:"addData,omitempty"`
}

type CoordinateInfo struct {
	PageSize *PageSize `xml:"pageSize,omitempty"`
	FBD      Scaling   `xml:"fbd>scaling"`
	LD       Scaling   `xml:"ld>scaling"`
	SFC      Scaling   `xml:"sfc>scaling"`
}

type PageSize struct {
	X float64 `xml:"x,attr"`
	Y float64 `xml:"y,attr"`
}

type Scaling struct {
	X float64 `xml:"x,attr"`
	Y float64 `xml:"y,attr"`
}

type Types struct {
	DataTypes DataTypes `xml:"dataTypes"`
	Pous      Pous      `xml:"pous"`
}

type DataTypes struct {
	DataTypes []DataTypeDecl `xml:"dataType,omitempty"`
}

type DataTypeDecl struct {
	Name         string         `xml:"name,attr"`
	BaseType     DataType       `xml:"baseType"`
	InitialValue *Value         `xml:"initialValue,omitempty"`
	AddData      *AddData       `xml:"addData,omitempty"`
	Doc          *FormattedText `xml:"documentation,omitempty"`
}

type Pous struct {
	Pous []POU `xml:"pou,omitempty"`
}

type POU struct {
	Name      string         `xml:"name,attr"`
	PouType   string         `xml:"pouType,attr"` // "program", "functionBlock", "function"
	GlobalID  string         `xml:"globalId,attr,omitempty"`
	Interface *POUInterface  `xml:"interface,omitempty"`
	Body      POUBody        `xml:"body"`
	AddData   *AddData       `xml:"addData,omitempty"`
	Doc       *FormattedText `xml:"documentation,omitempty"`
}

type POUInterface struct {
	ReturnType   *DataType      `xml:"returnType,omitempty"`
	InputVars    []VarList      `xml:"inputVars,omitempty"`
	OutputVars   []VarList      `xml:"outputVars,omitempty"`
	InOutVars    []VarList      `xml:"inOutVars,omitempty"`
	LocalVars    []VarList      `xml:"localVars,omitempty"`
	TempVars     []VarList      `xml:"tempVars,omitempty"`
	ExternalVars []VarList      `xml:"externalVars,omitempty"`
	GlobalVars   []VarList      `xml:"globalVars,omitempty"`
	AccessVars   []VarList      `xml:"accessVars,omitempty"`
	AddData      *AddData       `xml:"addData,omitempty"`
	Doc          *FormattedText `xml:"documentation,omitempty"`
}

type VarList struct {
	Name          string         `xml:"name,attr,omitempty"`
	Constant      bool           `xml:"constant,attr,omitempty"`
	Retain        bool           `xml:"retain,attr,omitempty"`
	NonRetain     bool           `xml:"nonretain,attr,omitempty"`
	Persistent    bool           `xml:"persistent,attr,omitempty"`
	NonPersistent bool           `xml:"nonpersistent,attr,omitempty"`
	Variables     []Variable     `xml:"variable,omitempty"`
	AddData       *AddData       `xml:"addData,omitempty"`
	Doc           *FormattedText `xml:"documentation,omitempty"`
}

type Variable struct {
	Name         string         `xml:"name,attr"`
	Address      string         `xml:"address,attr,omitempty"`
	GlobalID     string         `xml:"globalId,attr,omitempty"`
	Type         DataType       `xml:"type"`
	InitialValue *Value         `xml:"initialValue,omitempty"`
	AddData      *AddData       `xml:"addData,omitempty"`
	Doc          *FormattedText `xml:"documentation,omitempty"`
}

type EmptyTag struct{}

type DataType struct {
	// Elementary types
	BOOL    *EmptyTag   `xml:"BOOL,omitempty"`
	BYTE    *EmptyTag   `xml:"BYTE,omitempty"`
	WORD    *EmptyTag   `xml:"WORD,omitempty"`
	DWORD   *EmptyTag   `xml:"DWORD,omitempty"`
	LWORD   *EmptyTag   `xml:"LWORD,omitempty"`
	SINT    *EmptyTag   `xml:"SINT,omitempty"`
	INT     *EmptyTag   `xml:"INT,omitempty"`
	DINT    *EmptyTag   `xml:"DINT,omitempty"`
	LINT    *EmptyTag   `xml:"LINT,omitempty"`
	USINT   *EmptyTag   `xml:"USINT,omitempty"`
	UINT    *EmptyTag   `xml:"UINT,omitempty"`
	UDINT   *EmptyTag   `xml:"UDINT,omitempty"`
	ULINT   *EmptyTag   `xml:"ULINT,omitempty"`
	REAL    *EmptyTag   `xml:"REAL,omitempty"`
	LREAL   *EmptyTag   `xml:"LREAL,omitempty"`
	TIME    *EmptyTag   `xml:"TIME,omitempty"`
	DATE    *EmptyTag   `xml:"DATE,omitempty"`
	DT      *EmptyTag   `xml:"DT,omitempty"`
	TOD     *EmptyTag   `xml:"TOD,omitempty"`
	STRING  *StringType `xml:"string,omitempty"`
	WSTRING *StringType `xml:"wstring,omitempty"`

	// Derived and user types
	Derived          *DerivedType  `xml:"derived,omitempty"`
	Array            *ArrayType    `xml:"array,omitempty"`
	Struct           *StructType   `xml:"struct,omitempty"`
	Enum             *EnumType     `xml:"enum,omitempty"`
	Pointer          *PointerType  `xml:"pointer,omitempty"`
	SubrangeSigned   *SubrangeType `xml:"subrangeSigned,omitempty"`
	SubrangeUnsigned *SubrangeType `xml:"subrangeUnsigned,omitempty"`
}

type StringType struct {
	Length string `xml:"length,attr,omitempty"`
}

type DerivedType struct {
	Name string `xml:"name,attr"`
}

type ArrayType struct {
	Dimensions []Dimension `xml:"dimension"`
	BaseType   *DataType   `xml:"baseType"`
}

type Dimension struct {
	Lower string `xml:"lower,attr"`
	Upper string `xml:"upper,attr"`
}

type StructType struct {
	Variables []Variable `xml:"variable,omitempty"`
}

type EnumType struct {
	Values   []EnumValue `xml:"values>value"`
	BaseType *DataType   `xml:"baseType,omitempty"`
}

type EnumValue struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr,omitempty"`
}

type PointerType struct {
	BaseType *DataType `xml:"baseType"`
}

type SubrangeType struct {
	Range    SubrangeRange `xml:"range"`
	BaseType *DataType     `xml:"baseType"`
}

type SubrangeRange struct {
	Lower string `xml:"lower,attr"`
	Upper string `xml:"upper,attr"`
}

type Value struct {
	SimpleValue *SimpleValue `xml:"simpleValue,omitempty"`
	ArrayValue  *ArrayValue  `xml:"arrayValue,omitempty"`
	StructValue *StructValue `xml:"structValue,omitempty"`
}

type SimpleValue struct {
	Value string `xml:"value,attr"`
}

type ArrayValue struct {
	Values []ArrayValueElement `xml:"value,omitempty"`
}

type ArrayValueElement struct {
	RepetitionValue string       `xml:"repetitionValue,attr,omitempty"`
	SimpleValue     *SimpleValue `xml:"simpleValue,omitempty"`
	ArrayValue      *ArrayValue  `xml:"arrayValue,omitempty"`
	StructValue     *StructValue `xml:"structValue,omitempty"`
}

type StructValue struct {
	Members []StructMemberValue `xml:"value,omitempty"`
}

type StructMemberValue struct {
	Member      string       `xml:"member,attr"`
	SimpleValue *SimpleValue `xml:"simpleValue,omitempty"`
	ArrayValue  *ArrayValue  `xml:"arrayValue,omitempty"`
	StructValue *StructValue `xml:"structValue,omitempty"`
}

type POUBody struct {
	ST *FormattedText `xml:"ST,omitempty"`
	IL *FormattedText `xml:"IL,omitempty"`
	// Graphical bodies are kept as they are, so an export writes them back;
	// FBD and LD are lowered to ST on import (graphical.go).
	FBD     *RawXML        `xml:"FBD,omitempty"`
	LD      *RawXML        `xml:"LD,omitempty"`
	SFC     *RawXML        `xml:"SFC,omitempty"`
	AddData *AddData       `xml:"addData,omitempty"`
	Doc     *FormattedText `xml:"documentation,omitempty"`
}

// RawXML holds an element's content unparsed.
type RawXML struct {
	Inner string `xml:",innerxml"`
}

type FormattedText struct {
	Text string `xml:",innerxml"`
}

type Instances struct {
	Configurations Configurations `xml:"configurations"`
}

type Configurations struct {
	Configurations []Configuration `xml:"configuration,omitempty"`
}

type Configuration struct {
	Name       string         `xml:"name,attr"`
	Resources  []Resource     `xml:"resource,omitempty"`
	GlobalVars []VarList      `xml:"globalVars,omitempty"`
	AddData    *AddData       `xml:"addData,omitempty"`
	Doc        *FormattedText `xml:"documentation,omitempty"`
}

type Resource struct {
	Name         string         `xml:"name,attr"`
	Tasks        []Task         `xml:"task,omitempty"`
	GlobalVars   []VarList      `xml:"globalVars,omitempty"`
	PouInstances []POUInstance  `xml:"pouInstance,omitempty"`
	AddData      *AddData       `xml:"addData,omitempty"`
	Doc          *FormattedText `xml:"documentation,omitempty"`
}

type Task struct {
	Name         string         `xml:"name,attr"`
	Priority     int            `xml:"priority,attr"`
	Interval     string         `xml:"interval,attr,omitempty"`
	Single       string         `xml:"single,attr,omitempty"`
	PouInstances []POUInstance  `xml:"pouInstance,omitempty"`
	AddData      *AddData       `xml:"addData,omitempty"`
	Doc          *FormattedText `xml:"documentation,omitempty"`
}

type POUInstance struct {
	Name     string `xml:"name,attr"`
	TypeName string `xml:"typeName,attr"`
}

func (dt DataType) String() string {
	switch {
	case dt.BOOL != nil:
		return "BOOL"
	case dt.BYTE != nil:
		return "BYTE"
	case dt.WORD != nil:
		return "WORD"
	case dt.DWORD != nil:
		return "DWORD"
	case dt.LWORD != nil:
		return "LWORD"
	case dt.SINT != nil:
		return "SINT"
	case dt.INT != nil:
		return "INT"
	case dt.DINT != nil:
		return "DINT"
	case dt.LINT != nil:
		return "LINT"
	case dt.USINT != nil:
		return "USINT"
	case dt.UINT != nil:
		return "UINT"
	case dt.UDINT != nil:
		return "UDINT"
	case dt.ULINT != nil:
		return "ULINT"
	case dt.REAL != nil:
		return "REAL"
	case dt.LREAL != nil:
		return "LREAL"
	case dt.TIME != nil:
		return "TIME"
	case dt.DATE != nil:
		return "DATE"
	case dt.DT != nil:
		return "DT"
	case dt.TOD != nil:
		return "TOD"
	case dt.STRING != nil:
		if dt.STRING.Length != "" {
			return "STRING[" + dt.STRING.Length + "]"
		}
		return "STRING"
	case dt.WSTRING != nil:
		if dt.WSTRING.Length != "" {
			return "WSTRING[" + dt.WSTRING.Length + "]"
		}
		return "WSTRING"
	case dt.Array != nil:
		var dims []string
		for _, d := range dt.Array.Dimensions {
			dims = append(dims, fmt.Sprintf("%s..%s", d.Lower, d.Upper))
		}
		base := "INT"
		if dt.Array.BaseType != nil {
			base = dt.Array.BaseType.String()
		}
		return fmt.Sprintf("ARRAY [%s] OF %s", strings.Join(dims, ", "), base)
	case dt.Derived != nil:
		return dt.Derived.Name
	case dt.Enum != nil:
		var vals []string
		for _, v := range dt.Enum.Values {
			vals = append(vals, v.Name)
		}
		return "(" + strings.Join(vals, ", ") + ")"
	case dt.Struct != nil:
		var members []string
		for _, v := range dt.Struct.Variables {
			members = append(members, fmt.Sprintf("\t\t%s : %s;", v.Name, typeText(v.Type, v.AddData)))
		}
		return "STRUCT\n" + strings.Join(members, "\n") + "\n\tEND_STRUCT"
	// TC6 <pointer> is a pointer, as CODESYS writes POINTER TO; see references.go.
	case dt.Pointer != nil && dt.Pointer.BaseType != nil:
		return "POINTER TO " + dt.Pointer.BaseType.String()
	case dt.SubrangeSigned != nil:
		base := "INT"
		if dt.SubrangeSigned.BaseType != nil {
			base = dt.SubrangeSigned.BaseType.String()
		}
		return fmt.Sprintf("%s(%s..%s)", base, dt.SubrangeSigned.Range.Lower, dt.SubrangeSigned.Range.Upper)
	case dt.SubrangeUnsigned != nil:
		base := "UINT"
		if dt.SubrangeUnsigned.BaseType != nil {
			base = dt.SubrangeUnsigned.BaseType.String()
		}
		return fmt.Sprintf("%s(%s..%s)", base, dt.SubrangeUnsigned.Range.Lower, dt.SubrangeUnsigned.Range.Upper)
	default:
		return "INT"
	}
}

func (v *Value) String() string {
	if v == nil {
		return ""
	}
	if v.SimpleValue != nil {
		return v.SimpleValue.Value
	}
	if v.StructValue != nil {
		var members []string
		for _, m := range v.StructValue.Members {
			val := ""
			if m.SimpleValue != nil {
				val = m.SimpleValue.Value
			} else if m.ArrayValue != nil {
				val = (&Value{ArrayValue: m.ArrayValue}).String()
			} else if m.StructValue != nil {
				val = (&Value{StructValue: m.StructValue}).String()
			}
			members = append(members, fmt.Sprintf("%s := %s", m.Member, val))
		}
		return "(" + strings.Join(members, ", ") + ")"
	}
	if v.ArrayValue != nil {
		var elems []string
		for _, el := range v.ArrayValue.Values {
			var elemStr string
			var hasVal bool
			if el.SimpleValue != nil {
				elemStr = el.SimpleValue.Value
				hasVal = true
			} else if el.ArrayValue != nil {
				elemStr = (&Value{ArrayValue: el.ArrayValue}).String()
				hasVal = true
			} else if el.StructValue != nil {
				elemStr = (&Value{StructValue: el.StructValue}).String()
				hasVal = true
			}
			if hasVal {
				if el.RepetitionValue != "" && el.RepetitionValue != "1" {
					elems = append(elems, fmt.Sprintf("%s(%s)", el.RepetitionValue, elemStr))
				} else {
					elems = append(elems, elemStr)
				}
			}
		}
		return "[" + strings.Join(elems, ", ") + "]"
	}
	return ""
}

// DataTypeFromAST converts an AST type expression into a PLCopen DataType.
func DataTypeFromAST(expr ast.Expression) DataType {
	if expr == nil {
		return DataType{INT: &EmptyTag{}}
	}

	switch e := expr.(type) {
	case *ast.Identifier:
		return dataTypeFromName(e.Value)
	case *ast.TypeSpecifier:
		return dataTypeFromName(e.Token.Literal)
	case *ast.IndexExpression:
		// A string with its length, such as STRING[20].
		dt := DataTypeFromAST(e.Left)
		length := &StringType{Length: constantText(e.Index)}
		switch {
		case dt.STRING != nil:
			return DataType{STRING: length}
		case dt.WSTRING != nil:
			return DataType{WSTRING: length}
		}
		return dataTypeFromName(expr.String())
	// REF_TO, POINTER TO and REFERENCE TO are all TC6 <pointer>; see references.go.
	case *ast.RefToType:
		base := DataTypeFromAST(e.BaseType)
		return DataType{Pointer: &PointerType{BaseType: &base}}
	case *ast.ReferenceType:
		base := DataTypeFromAST(e.BaseType)
		return DataType{Pointer: &PointerType{BaseType: &base}}
	case *ast.ArrayDefinition:
		var dims []Dimension
		for _, r := range e.Ranges {
			if infix, ok := r.(*ast.InfixExpression); ok && infix.Operator == ".." {
				dims = append(dims, Dimension{
					Lower: constantText(infix.Left),
					Upper: constantText(infix.Right),
				})
			}
		}
		baseType := DataTypeFromAST(e.DataType)
		return DataType{
			Array: &ArrayType{
				Dimensions: dims,
				BaseType:   &baseType,
			},
		}
	case *ast.EnumDefinition:
		var vals []EnumValue
		for _, v := range e.Values {
			vals = append(vals, EnumValue{Name: v.Value})
		}
		return DataType{
			Enum: &EnumType{
				Values: vals,
			},
		}
	case *ast.StructDefinition:
		var vars []Variable
		for _, m := range e.Members {
			vars = append(vars, Variable{
				Name:    m.Name.Value,
				Type:    DataTypeFromAST(m.DataType),
				AddData: stTypeData(m.DataType),
			})
		}
		return DataType{
			Struct: &StructType{
				Variables: vars,
			},
		}
	default:
		return dataTypeFromName(expr.String())
	}
}

// subrangePattern matches a subrange type such as INT(0..100).
var subrangePattern = regexp.MustCompile(`^([A-Za-z0-9_]*)\s*\(\s*(-?\d+)\s*\.\.\s*(-?\d+)\s*\)$`)

func dataTypeFromName(name string) DataType {
	trimmed := strings.TrimSpace(name)
	if matches := subrangePattern.FindStringSubmatch(trimmed); len(matches) == 4 {
		base := matches[1]
		if base == "" {
			base = "INT"
		}
		baseUpper := strings.ToUpper(base)
		isUnsigned := baseUpper == "USINT" || baseUpper == "UINT" || baseUpper == "UDINT" || baseUpper == "ULINT"
		baseDT := dataTypeFromName(base)
		sr := &SubrangeType{
			Range: SubrangeRange{
				Lower: matches[2],
				Upper: matches[3],
			},
			BaseType: &baseDT,
		}
		if isUnsigned {
			return DataType{SubrangeUnsigned: sr}
		}
		return DataType{SubrangeSigned: sr}
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "BOOL":
		return DataType{BOOL: &EmptyTag{}}
	case "BYTE":
		return DataType{BYTE: &EmptyTag{}}
	case "WORD":
		return DataType{WORD: &EmptyTag{}}
	case "DWORD":
		return DataType{DWORD: &EmptyTag{}}
	case "LWORD":
		return DataType{LWORD: &EmptyTag{}}
	case "SINT":
		return DataType{SINT: &EmptyTag{}}
	case "INT":
		return DataType{INT: &EmptyTag{}}
	case "DINT":
		return DataType{DINT: &EmptyTag{}}
	case "LINT":
		return DataType{LINT: &EmptyTag{}}
	case "USINT":
		return DataType{USINT: &EmptyTag{}}
	case "UINT":
		return DataType{UINT: &EmptyTag{}}
	case "UDINT":
		return DataType{UDINT: &EmptyTag{}}
	case "ULINT":
		return DataType{ULINT: &EmptyTag{}}
	case "REAL":
		return DataType{REAL: &EmptyTag{}}
	case "LREAL":
		return DataType{LREAL: &EmptyTag{}}
	case "TIME":
		return DataType{TIME: &EmptyTag{}}
	case "DATE":
		return DataType{DATE: &EmptyTag{}}
	case "DT", "DATE_AND_TIME":
		return DataType{DT: &EmptyTag{}}
	case "TOD", "TIME_OF_DAY":
		return DataType{TOD: &EmptyTag{}}
	case "STRING":
		return DataType{STRING: &StringType{}}
	case "WSTRING":
		return DataType{WSTRING: &StringType{}}
	default:
		return DataType{Derived: &DerivedType{Name: name}}
	}
}

// constantText returns a constant as IEC 61131-3 source: a literal as
// written, such as T#20ms, and a negative number without parentheses.
func constantText(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.PrefixExpression:
		if e.Operator == "-" || e.Operator == "+" {
			return e.Operator + constantText(e.Right)
		}
	// The lexer keeps a string's $ escapes, such as $' and $$, as written, so
	// its text only needs its quotes back.
	case *ast.StringLiteral:
		return "'" + e.Value + "'"
	case *ast.WStringLiteral:
		return `"` + e.Value + `"`
	}
	return expr.String()
}

// constantInt returns the value of a constant integer, such as a task's
// PRIORITY.
func constantInt(expr ast.Expression) (int, bool) {
	var n int
	if _, err := fmt.Sscanf(constantText(expr), "%d", &n); err != nil || fmt.Sprint(n) != constantText(expr) {
		return 0, false
	}
	return n, true
}

// isUnsignedName reports whether a type name is an unsigned integer type.
func isUnsignedName(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "USINT", "UINT", "UDINT", "ULINT":
		return true
	}
	return false
}
