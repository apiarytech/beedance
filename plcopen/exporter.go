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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// Export converts a parsed IEC 61131-3 AST into a PLCopen TC6 XML project.
// Without the source, a POU's body is written in the AST's String form, which
// parenthesizes every expression and drops comments; ExportSource keeps the
// body as written.
func Export(program *ast.Program, projectName string) (*Project, error) {
	return export(program, projectName, nil)
}

// ExportSource parses IEC 61131-3 source and converts it into a PLCopen TC6
// XML project, keeping each POU's body as written: ST and IL as text, LD
// and FBD written in beedance's text form as graphical diagrams.
func ExportSource(source, projectName string) (*Project, error) {
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parser errors: %s", strings.Join(p.Errors(), "; "))
	}
	proj, err := export(program, projectName, sourceBodies(source))
	if err != nil {
		return nil, err
	}
	if err := graphicalBodies(proj, p.Diagrams()); err != nil {
		return nil, err
	}
	return proj, nil
}

// ExportSourceKeeping is ExportSource for source that came from the project
// from, such as an imported project after an edit: the export keeps from's
// additional data, so that another tool's own data survives the round trip
// (see KeepAddData).
func ExportSourceKeeping(source, projectName string, from *Project) (*Project, error) {
	proj, err := ExportSource(source, projectName)
	if err != nil {
		return nil, err
	}
	KeepAddData(from, proj)
	return proj, nil
}

// export builds the project. bodies holds the source text of each POU's
// body by the byte offset of its keyword, or is nil.
func export(program *ast.Program, projectName string, bodies map[int]string) (*Project, error) {
	if projectName == "" {
		projectName = "BeedanceProject"
	}

	proj := &Project{
		XmlnsXhtml: "http://www.w3.org/1999/xhtml",
		FileHeader: FileHeader{
			CompanyName:      "ApiaryTech.io",
			ProductName:      "beedance",
			ProductVersion:   "0.1.0",
			CreationDateTime: time.Now().Format("2006-01-02T15:04:05"),
		},
		ContentHeader: ContentHeader{
			Name: projectName,
			CoordinateInfo: &CoordinateInfo{
				FBD: Scaling{X: 1, Y: 1},
				LD:  Scaling{X: 1, Y: 1},
				SFC: Scaling{X: 1, Y: 1},
			},
		},
	}

	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.TypeBlockDeclaration:
			for _, decl := range s.Declarations {
				proj.Types.DataTypes.DataTypes = append(proj.Types.DataTypes.DataTypes, dataTypeDecl(decl))
			}
		case *ast.TypeDeclaration:
			proj.Types.DataTypes.DataTypes = append(proj.Types.DataTypes.DataTypes, dataTypeDecl(s))

		case *ast.ProgramDeclaration:
			pou := POU{
				Name:    s.Name.Value,
				PouType: "program",
				Interface: &POUInterface{
					InputVars:    toVarLists(s.VarInputs),
					OutputVars:   toVarLists(s.VarOutputs),
					InOutVars:    toVarLists(s.VarInOuts),
					LocalVars:    toVarLists(s.Vars),
					TempVars:     toTempVarLists(s.VarTemp),
					ExternalVars: toExternalVarLists(s.VarExternal),
					GlobalVars:   toGlobalVarLists(s.VarGlobal),
				},
				Body: pouBody(s.Token.Pos, s.Body, bodies),
			}
			proj.Types.Pous.Pous = append(proj.Types.Pous.Pous, pou)

		case *ast.FunctionBlockDeclaration:
			pou := POU{
				Name:    s.Name.Value,
				PouType: "functionBlock",
				Interface: &POUInterface{
					InputVars:    toVarLists(s.VarInputs),
					OutputVars:   toVarLists(s.VarOutputs),
					InOutVars:    toVarLists(s.VarInOuts),
					LocalVars:    toVarLists(s.Vars),
					TempVars:     toTempVarLists(s.VarTemp),
					ExternalVars: toExternalVarLists(s.VarExternal),
				},
				Body: pouBody(s.Token.Pos, s.Body, bodies),
			}
			proj.Types.Pous.Pous = append(proj.Types.Pous.Pous, pou)

		case *ast.FunctionDeclaration:
			if s.ReturnType == nil {
				return nil, fmt.Errorf("function '%s' has no return type", s.Name.Value)
			}
			retType := DataTypeFromAST(s.ReturnType)
			pou := POU{
				Name:    s.Name.Value,
				PouType: "function",
				Interface: &POUInterface{
					ReturnType: &retType,
					InputVars:  toVarLists(s.VarInputs),
					OutputVars: toVarLists(s.VarOutputs),
					InOutVars:  toVarLists(s.VarInOuts),
					LocalVars:  toVarLists(s.Vars),
				},
				Body: pouBody(s.Token.Pos, s.Body, bodies),
			}
			proj.Types.Pous.Pous = append(proj.Types.Pous.Pous, pou)

		case *ast.ConfigurationDeclaration:
			cfg, err := configuration(s)
			if err != nil {
				return nil, err
			}
			proj.Instances.Configurations.Configurations = append(proj.Instances.Configurations.Configurations, cfg)

		case *ast.NamespaceDeclaration, *ast.InterfaceDeclaration, *ast.GlobalVarDeclaration:
			// PLCopen TC6 v2.01 has no place for these; say so rather than
			// drop them.
			return nil, fmt.Errorf("PLCopen TC6 v2.01 cannot hold %s", describeStatement(stmt))
		}
	}

	declareBeedanceData(proj)
	return proj, nil
}

// describeStatement names a declaration for an error message.
func describeStatement(stmt ast.Statement) string {
	switch s := stmt.(type) {
	case *ast.NamespaceDeclaration:
		return fmt.Sprintf("NAMESPACE %s", s.Name.String())
	case *ast.InterfaceDeclaration:
		return fmt.Sprintf("INTERFACE %s", s.Name.Value)
	}
	return "a VAR_GLOBAL block outside a CONFIGURATION or PROGRAM"
}

// dataTypeDecl converts a TYPE declaration.
func dataTypeDecl(decl *ast.TypeDeclaration) DataTypeDecl {
	var baseType DataType
	if infix, ok := decl.Subrange.(*ast.InfixExpression); ok && infix.Operator == ".." {
		baseDT := DataTypeFromAST(decl.DataType)
		sr := &SubrangeType{
			Range:    SubrangeRange{Lower: constantText(infix.Left), Upper: constantText(infix.Right)},
			BaseType: &baseDT,
		}
		if isUnsignedName(decl.DataType.String()) {
			baseType = DataType{SubrangeUnsigned: sr}
		} else {
			baseType = DataType{SubrangeSigned: sr}
		}
	} else {
		baseType = DataTypeFromAST(decl.DataType)
		if decl.StringLength != nil {
			length := &StringType{Length: constantText(decl.StringLength)}
			if baseType.WSTRING != nil {
				baseType.WSTRING = length
			} else if baseType.STRING != nil {
				baseType.STRING = length
			}
		}
	}
	return DataTypeDecl{
		Name:         decl.Name.Value,
		BaseType:     baseType,
		InitialValue: valueFromAST(decl.InitialValue),
		AddData:      stTypeData(decl.DataType),
	}
}

// configuration converts a CONFIGURATION with its globals, resources, tasks
// and program instances.
func configuration(s *ast.ConfigurationDeclaration) (Configuration, error) {
	cfg := Configuration{Name: s.Name.Value, GlobalVars: toGlobalVarLists(s.GlobalVars)}
	for _, res := range s.Resources {
		r := Resource{Name: res.Name.Value, GlobalVars: toGlobalVarLists(res.GlobalVars)}
		for _, task := range res.Tasks {
			t := Task{Name: task.Name.Value}
			if task.Priority != nil {
				priority, ok := constantInt(task.Priority)
				if !ok {
					return cfg, fmt.Errorf("task '%s': PRIORITY must be a constant integer, got %s", task.Name.Value, task.Priority.String())
				}
				t.Priority = priority
			}
			if task.Interval != nil {
				t.Interval = constantText(task.Interval)
			}
			if task.Single != nil {
				t.Single = constantText(task.Single)
			}
			r.Tasks = append(r.Tasks, t)
		}
		for _, prog := range res.Programs {
			inst := POUInstance{Name: prog.InstanceName.Value, TypeName: prog.TypeName.Value}
			assigned := false
			if prog.TaskName != nil {
				for idx := range r.Tasks {
					if strings.EqualFold(r.Tasks[idx].Name, prog.TaskName.Value) {
						r.Tasks[idx].PouInstances = append(r.Tasks[idx].PouInstances, inst)
						assigned = true
						break
					}
				}
				if !assigned {
					return cfg, fmt.Errorf("program '%s' of resource '%s' names task '%s', which the resource does not declare", prog.InstanceName.Value, res.Name.Value, prog.TaskName.Value)
				}
			}
			if !assigned {
				r.PouInstances = append(r.PouInstances, inst)
			}
		}
		cfg.Resources = append(cfg.Resources, r)
	}
	return cfg, nil
}

// ExportToXML marshals an AST into formatted XML conforming to PLCopen TC6.
func ExportToXML(program *ast.Program, projectName string) ([]byte, error) {
	proj, err := Export(program, projectName)
	if err != nil {
		return nil, err
	}
	return marshalProject(proj)
}

// ExportSourceToXML converts IEC 61131-3 source into formatted PLCopen TC6
// XML, keeping each POU's body as written.
func ExportSourceToXML(source, projectName string) ([]byte, error) {
	proj, err := ExportSource(source, projectName)
	if err != nil {
		return nil, err
	}
	return marshalProject(proj)
}

// marshalProject writes a project as an XML document.
func marshalProject(proj *Project) ([]byte, error) {
	data, err := xml.MarshalIndent(proj, "", "  ")
	if err != nil {
		return nil, err
	}
	out := append([]byte(xml.Header), data...)
	return append(out, '\n'), nil
}

// ConvertIECToXMLFile parses an IEC source file and saves it as PLCopen TC6
// XML. The project is named after the file.
func ConvertIECToXMLFile(inputFile, outputFile string) error {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	name := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
	xmlBytes, err := ExportSourceToXML(string(content), name)
	if err != nil {
		return fmt.Errorf("export xml: %w", err)
	}
	return os.WriteFile(outputFile, xmlBytes, 0644)
}

// pouBody returns a POU's body: its source text when bodies has it, else its
// String form, in an IL element for an Instruction List body and an ST
// element otherwise.
func pouBody(keywordPos int, body ast.Statement, bodies map[int]string) POUBody {
	text, fromSource := bodies[keywordPos]
	if !fromSource {
		text = formatBodyST(body)
	}
	formatted := &FormattedText{Text: xhtmlText(text)}
	if block, ok := body.(*ast.BlockStatement); ok && len(block.Statements) > 0 {
		if _, isIL := block.Statements[0].(*ast.IlInstructionStatement); isIL {
			return POUBody{IL: formatted}
		}
	}
	return POUBody{ST: formatted}
}

// formatBodyST returns a body in the AST's String form, a statement a line.
func formatBodyST(body ast.Statement) string {
	if body == nil {
		return ""
	}
	lines := []string{}
	if block, ok := body.(*ast.BlockStatement); ok {
		for _, stmt := range block.Statements {
			lines = append(lines, stmt.String())
		}
	} else {
		lines = append(lines, body.String())
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// xhtmlText wraps code as the XHTML paragraph of a PLCopen formattedText.
// The code is in CDATA, split where it holds "]]>", which would end it.
func xhtmlText(code string) string {
	code = strings.ReplaceAll(code, "]]>", "]]]]><![CDATA[>")
	return fmt.Sprintf("\n<xhtml:p><![CDATA[\n%s\n]]></xhtml:p>\n", code)
}

// toVarLists converts declarations into variable lists, one for each
// combination of CONSTANT, RETAIN and NON_RETAIN, in declaration order.
func toVarLists(decls []*ast.VarDeclStatement) []VarList {
	var lists []VarList
	for _, d := range decls {
		v := Variable{
			Name:    d.Name.Value,
			Type:    DataTypeFromAST(d.DataType),
			AddData: stTypeData(d.DataType),
		}
		if d.Location != nil && d.Location.Location != nil {
			v.Address = d.Location.Location.String()
		}
		v.InitialValue = valueFromAST(d.Value)

		n := len(lists)
		if n == 0 || lists[n-1].Constant != d.IsConstant || lists[n-1].Retain != d.IsRetain || lists[n-1].NonRetain != d.IsNonRetain {
			lists = append(lists, VarList{Constant: d.IsConstant, Retain: d.IsRetain, NonRetain: d.IsNonRetain})
			n++
		}
		lists[n-1].Variables = append(lists[n-1].Variables, v)
	}
	return lists
}

func toTempVarLists(blocks []*ast.TempVarDeclaration) []VarList {
	var res []VarList
	for _, b := range blocks {
		res = append(res, toVarLists(b.Vars)...)
	}
	return res
}

func toExternalVarLists(blocks []*ast.ExternalVarDeclaration) []VarList {
	var res []VarList
	for _, b := range blocks {
		res = append(res, toVarLists(b.Vars)...)
	}
	return res
}

func toGlobalVarLists(blocks []*ast.GlobalVarDeclaration) []VarList {
	var res []VarList
	for _, b := range blocks {
		res = append(res, toVarLists(b.Vars)...)
	}
	return res
}

// valueFromAST converts an initial value, or returns nil for none.
func valueFromAST(expr ast.Expression) *Value {
	if expr == nil {
		return nil
	}
	if arrLit, ok := expr.(*ast.ArrayLiteral); ok {
		var elems []ArrayValueElement
		for _, el := range arrLit.Elements {
			elems = append(elems, arrayValueElementFromAST(el))
		}
		return &Value{ArrayValue: &ArrayValue{Values: elems}}
	}
	if structLit, ok := expr.(*ast.StructLiteral); ok {
		var members []StructMemberValue
		for _, init := range structLit.Initializers {
			if named, ok := init.(*ast.NamedArgument); ok {
				sm := StructMemberValue{Member: named.Name.Value}
				if val := valueFromAST(named.Value); val != nil {
					sm.SimpleValue = val.SimpleValue
					sm.ArrayValue = val.ArrayValue
					sm.StructValue = val.StructValue
				}
				members = append(members, sm)
			}
		}
		return &Value{StructValue: &StructValue{Members: members}}
	}
	return &Value{SimpleValue: &SimpleValue{Value: constantText(expr)}}
}

func arrayValueElementFromAST(expr ast.Expression) ArrayValueElement {
	if rep, ok := expr.(*ast.ArrayRepetition); ok {
		repFactor := ""
		if rep.Factor != nil {
			repFactor = constantText(rep.Factor)
		}
		if len(rep.Elements) == 1 {
			if innerVal := valueFromAST(rep.Elements[0]); innerVal != nil {
				return ArrayValueElement{
					RepetitionValue: repFactor,
					SimpleValue:     innerVal.SimpleValue,
					ArrayValue:      innerVal.ArrayValue,
					StructValue:     innerVal.StructValue,
				}
			}
		}
		var innerElems []ArrayValueElement
		for _, ie := range rep.Elements {
			innerElems = append(innerElems, arrayValueElementFromAST(ie))
		}
		return ArrayValueElement{RepetitionValue: repFactor, ArrayValue: &ArrayValue{Values: innerElems}}
	}

	val := valueFromAST(expr)
	return ArrayValueElement{SimpleValue: val.SimpleValue, ArrayValue: val.ArrayValue, StructValue: val.StructValue}
}
