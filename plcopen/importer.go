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
	"html"
	"os"
	"regexp"
	"strings"

	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
)

// Import unmarshals PLCopen TC6 XML data into a Project struct.
func Import(xmlData []byte) (*Project, error) {
	var proj Project
	if err := xml.Unmarshal(xmlData, &proj); err != nil {
		return nil, fmt.Errorf("unmarshal PLCopen XML: %w", err)
	}
	return &proj, nil
}

// ImportToIECText converts PLCopen XML into valid IEC 61131-3 Structured Text.
func ImportToIECText(xmlData []byte) (string, error) {
	proj, err := Import(xmlData)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	// 1. Data Types
	if len(proj.Types.DataTypes.DataTypes) > 0 {
		buf.WriteString("TYPE\n")
		for _, dt := range proj.Types.DataTypes.DataTypes {
			buf.WriteString(fmt.Sprintf("\t%s : %s;\n", dt.Name, dt.BaseType.String()))
		}
		buf.WriteString("END_TYPE\n\n")
	}

	// 2. POUs
	for _, pou := range proj.Types.Pous.Pous {
		pouType := strings.ToLower(pou.PouType)
		switch pouType {
		case "function":
			retType := "BOOL"
			if pou.Interface != nil && pou.Interface.ReturnType != nil {
				retType = pou.Interface.ReturnType.String()
			}
			buf.WriteString(fmt.Sprintf("FUNCTION %s : %s\n", pou.Name, retType))
		case "functionblock":
			buf.WriteString(fmt.Sprintf("FUNCTION_BLOCK %s\n", pou.Name))
		default:
			buf.WriteString(fmt.Sprintf("PROGRAM %s\n", pou.Name))
		}

		if pou.Interface != nil {
			writeVarLists(&buf, "VAR_INPUT", pou.Interface.InputVars)
			writeVarLists(&buf, "VAR_OUTPUT", pou.Interface.OutputVars)
			writeVarLists(&buf, "VAR_IN_OUT", pou.Interface.InOutVars)
			writeVarLists(&buf, "VAR", pou.Interface.LocalVars)
			writeVarLists(&buf, "VAR_TEMP", pou.Interface.TempVars)
			writeVarLists(&buf, "VAR_EXTERNAL", pou.Interface.ExternalVars)
			writeVarLists(&buf, "VAR_GLOBAL", pou.Interface.GlobalVars)
			writeVarLists(&buf, "VAR_ACCESS", pou.Interface.AccessVars)
		}

		bodyText := ""
		if pou.Body.ST != nil {
			bodyText = CleanFormattedText(pou.Body.ST.Text)
		} else if pou.Body.IL != nil {
			bodyText = CleanFormattedText(pou.Body.IL.Text)
		}

		if bodyText != "" {
			lines := strings.Split(bodyText, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					buf.WriteString("\t" + line + "\n")
				}
			}
		}

		switch pouType {
		case "function":
			buf.WriteString("END_FUNCTION\n\n")
		case "functionblock":
			buf.WriteString("END_FUNCTION_BLOCK\n\n")
		default:
			buf.WriteString("END_PROGRAM\n\n")
		}
	}

	// 3. Configurations
	for _, cfg := range proj.Instances.Configurations.Configurations {
		buf.WriteString(fmt.Sprintf("CONFIGURATION %s\n", cfg.Name))
		for _, res := range cfg.Resources {
			buf.WriteString(fmt.Sprintf("\tRESOURCE %s ON PLC\n", res.Name))
			for _, task := range res.Tasks {
				interval := ""
				if task.Interval != "" {
					interval = fmt.Sprintf(", INTERVAL := %s", task.Interval)
				}
				buf.WriteString(fmt.Sprintf("\t\tTASK %s (PRIORITY := %d%s);\n", task.Name, task.Priority, interval))
			}
			for _, inst := range res.PouInstances {
				buf.WriteString(fmt.Sprintf("\t\tPROGRAM %s : %s;\n", inst.Name, inst.TypeName))
			}
			buf.WriteString("\tEND_RESOURCE\n")
		}
		buf.WriteString("END_CONFIGURATION\n\n")
	}

	return strings.TrimSpace(buf.String()) + "\n", nil
}

// ImportToAST unmarshals PLCopen XML and parses it into a beedance AST Program.
func ImportToAST(xmlData []byte) (*ast.Program, error) {
	iecText, err := ImportToIECText(xmlData)
	if err != nil {
		return nil, err
	}

	l := lexer.New(iecText)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parsing generated IEC code: %s", strings.Join(p.Errors(), "; "))
	}
	return prog, nil
}

// ConvertXMLToIECText reads a PLCopen XML file and returns equivalent IEC text.
func ConvertXMLToIECText(xmlFile string) (string, error) {
	data, err := os.ReadFile(xmlFile)
	if err != nil {
		return "", fmt.Errorf("read XML file: %w", err)
	}
	return ImportToIECText(data)
}

func writeVarLists(buf *bytes.Buffer, sectionName string, lists []VarList) {
	for _, list := range lists {
		if len(list.Variables) == 0 {
			continue
		}
		qualifiers := ""
		if list.Constant {
			qualifiers += " CONSTANT"
		}
		if list.Retain {
			qualifiers += " RETAIN"
		}
		if list.NonRetain {
			qualifiers += " NON_RETAIN"
		}
		buf.WriteString(fmt.Sprintf("\t%s%s\n", sectionName, qualifiers))
		for _, v := range list.Variables {
			loc := ""
			if v.Address != "" {
				loc = fmt.Sprintf(" AT %s", v.Address)
			}
			initVal := ""
			if v.InitialValue != nil && v.InitialValue.String() != "" {
				initVal = " := " + v.InitialValue.String()
			}
			buf.WriteString(fmt.Sprintf("\t\t%s%s : %s%s;\n", v.Name, loc, v.Type.String(), initVal))
		}
		buf.WriteString("\tEND_VAR\n\n")
	}
}

// CleanFormattedText strips XHTML wrappers, CDATA blocks, and unescapes entities.
func CleanFormattedText(s string) string {
	s = strings.ReplaceAll(s, "<![CDATA[", "")
	s = strings.ReplaceAll(s, "]]>", "")
	reBr := regexp.MustCompile(`(?i)<(xhtml:)?br\s*/?>`)
	s = reBr.ReplaceAllString(s, "\n")
	reP := regexp.MustCompile(`(?i)</?(xhtml:)?p[^>]*>`)
	s = reP.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}
