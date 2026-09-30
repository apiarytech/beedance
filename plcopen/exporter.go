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
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
)

// Export converts a parsed IEC 61131-3 AST into a PLCopen TC6 XML project.
func Export(program *ast.Program, projectName string) (*Project, error) {
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
				dtDecl := DataTypeDecl{
					Name:     decl.Name.Value,
					BaseType: DataTypeFromAST(decl.DataType),
				}
				proj.Types.DataTypes.DataTypes = append(proj.Types.DataTypes.DataTypes, dtDecl)
			}
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
				Body: POUBody{
					ST: &FormattedText{
						Text: formatBodyST(s.Body),
					},
				},
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
				Body: POUBody{
					ST: &FormattedText{
						Text: formatBodyST(s.Body),
					},
				},
			}
			proj.Types.Pous.Pous = append(proj.Types.Pous.Pous, pou)

		case *ast.FunctionDeclaration:
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
				Body: POUBody{
					ST: &FormattedText{
						Text: formatBodyST(s.Body),
					},
				},
			}
			proj.Types.Pous.Pous = append(proj.Types.Pous.Pous, pou)

		case *ast.ConfigurationDeclaration:
			cfg := Configuration{Name: s.Name.Value}
			for _, res := range s.Resources {
				r := Resource{Name: res.Name.Value}
				for _, task := range res.Tasks {
					t := Task{Name: task.Name.Value}
					if task.Priority != nil {
						fmt.Sscanf(task.Priority.String(), "%d", &t.Priority)
					}
					if task.Interval != nil {
						t.Interval = task.Interval.String()
					}
					r.Tasks = append(r.Tasks, t)
				}
				for _, prog := range res.Programs {
					r.PouInstances = append(r.PouInstances, POUInstance{
						Name:     prog.InstanceName.Value,
						TypeName: prog.TypeName.Value,
					})
				}
				cfg.Resources = append(cfg.Resources, r)
			}
			proj.Instances.Configurations.Configurations = append(proj.Instances.Configurations.Configurations, cfg)
		}
	}

	return proj, nil
}

// ExportToXML marshals an AST into formatted XML conforming to PLCopen TC6.
func ExportToXML(program *ast.Program, projectName string) ([]byte, error) {
	proj, err := Export(program, projectName)
	if err != nil {
		return nil, err
	}

	data, err := xml.MarshalIndent(proj, "", "  ")
	if err != nil {
		return nil, err
	}

	header := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	out := append(header, data...)
	out = append(out, '\n')
	return out, nil
}

// ConvertIECToXMLFile parses an IEC source file and saves it as PLCopen TC6 XML.
func ConvertIECToXMLFile(inputFile, outputFile string) error {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	l := lexer.New(string(content))
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("parser errors: %s", strings.Join(p.Errors(), "; "))
	}

	xmlBytes, err := ExportToXML(prog, strings.TrimSuffix(inputFile, ".st"))
	if err != nil {
		return fmt.Errorf("export xml: %w", err)
	}

	return os.WriteFile(outputFile, xmlBytes, 0644)
}

func formatBodyST(body ast.Statement) string {
	if body == nil {
		return ""
	}
	var buf bytes.Buffer
	if block, ok := body.(*ast.BlockStatement); ok {
		for _, stmt := range block.Statements {
			buf.WriteString(stmt.String() + "\n")
		}
	} else {
		buf.WriteString(body.String() + "\n")
	}
	text := strings.TrimSpace(buf.String())
	return fmt.Sprintf("\n<xhtml:p><![CDATA[\n%s\n]]></xhtml:p>\n", text)
}

func toVarLists(decls []*ast.VarDeclStatement) []VarList {
	if len(decls) == 0 {
		return nil
	}
	vl := VarList{}
	for _, d := range decls {
		v := Variable{
			Name: d.Name.Value,
			Type: DataTypeFromAST(d.DataType),
		}
		if d.Location != nil {
			v.Address = d.Location.Location.String()
		}
		if d.Value != nil {
			v.InitialValue = valueFromAST(d.Value)
		}
		vl.Variables = append(vl.Variables, v)
	}
	return []VarList{vl}
}

func toTempVarLists(blocks []*ast.TempVarDeclaration) []VarList {
	var res []VarList
	for _, b := range blocks {
		res = append(res, toVarLists(b.Vars)...)
	}
	return res
}

func valueFromAST(expr ast.Expression) *Value {
	if expr == nil {
		return nil
	}
	if arrLit, ok := expr.(*ast.ArrayLiteral); ok {
		var elems []ArrayValueElement
		for _, el := range arrLit.Elements {
			elems = append(elems, arrayValueElementFromAST(el))
		}
		return &Value{
			ArrayValue: &ArrayValue{Values: elems},
		}
	}
	if structLit, ok := expr.(*ast.StructLiteral); ok {
		var members []StructMemberValue
		for _, init := range structLit.Initializers {
			if named, ok := init.(*ast.NamedArgument); ok {
				sm := StructMemberValue{Member: named.Name.Value}
				val := valueFromAST(named.Value)
				if val != nil {
					sm.SimpleValue = val.SimpleValue
					sm.ArrayValue = val.ArrayValue
					sm.StructValue = val.StructValue
				}
				members = append(members, sm)
			}
		}
		return &Value{
			StructValue: &StructValue{Members: members},
		}
	}
	return &Value{SimpleValue: &SimpleValue{Value: expr.String()}}
}

func arrayValueElementFromAST(expr ast.Expression) ArrayValueElement {
	if rep, ok := expr.(*ast.ArrayRepetition); ok {
		repFactor := ""
		if rep.Factor != nil {
			repFactor = rep.Factor.String()
		}
		if len(rep.Elements) == 1 {
			innerVal := valueFromAST(rep.Elements[0])
			if innerVal != nil {
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
		return ArrayValueElement{
			RepetitionValue: repFactor,
			ArrayValue:      &ArrayValue{Values: innerElems},
		}
	}

	val := valueFromAST(expr)
	if val != nil {
		return ArrayValueElement{
			SimpleValue: val.SimpleValue,
			ArrayValue:  val.ArrayValue,
			StructValue: val.StructValue,
		}
	}
	return ArrayValueElement{
		SimpleValue: &SimpleValue{Value: expr.String()},
	}
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
