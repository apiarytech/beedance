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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// Import unmarshals PLCopen TC6 XML data into a Project struct.
func Import(xmlData []byte) (*Project, error) {
	var proj Project
	if err := xml.Unmarshal(xmlData, &proj); err != nil {
		return nil, fmt.Errorf("unmarshal PLCopen XML: %w", err)
	}
	return &proj, nil
}

// ImportOptions choose how ImportToIECTextOptions writes bodies.
type ImportOptions struct {
	// KeepDiagrams writes graphical LD and FBD bodies in beedance's text
	// form (LD ... END_LD, FBD ... END_FBD), so they stay diagrams. A body
	// the text form cannot hold is lowered to Structured Text, with a
	// comment saying why. Otherwise every graphical body is lowered.
	KeepDiagrams bool
}

// ImportToIECText converts PLCopen XML into IEC 61131-3 source, graphical
// LD and FBD bodies lowered to Structured Text. Graphical SFC bodies cannot
// be converted yet.
func ImportToIECText(xmlData []byte) (string, error) {
	return ImportToIECTextOptions(xmlData, ImportOptions{})
}

// ImportToIECTextOptions is ImportToIECText with options.
func ImportToIECTextOptions(xmlData []byte, opt ImportOptions) (string, error) {
	proj, err := Import(xmlData)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	// 1. Data Types
	if len(proj.Types.DataTypes.DataTypes) > 0 {
		buf.WriteString("TYPE\n")
		for _, dt := range proj.Types.DataTypes.DataTypes {
			initVal := ""
			if s := dt.InitialValue.String(); s != "" {
				initVal = " := " + s
			}
			fmt.Fprintf(&buf, "\t%s : %s%s;\n", dt.Name, typeText(dt.BaseType, dt.AddData), initVal)
		}
		buf.WriteString("END_TYPE\n\n")
	}

	// 2. POUs
	for _, pou := range proj.Types.Pous.Pous {
		var end string
		switch pou.PouType {
		case "function":
			if pou.Interface == nil || pou.Interface.ReturnType == nil {
				return "", fmt.Errorf("function '%s' has no return type", pou.Name)
			}
			fmt.Fprintf(&buf, "FUNCTION %s : %s\n", pou.Name, pou.Interface.ReturnType.String())
			end = "END_FUNCTION"
		case "functionBlock":
			fmt.Fprintf(&buf, "FUNCTION_BLOCK %s\n", pou.Name)
			end = "END_FUNCTION_BLOCK"
		case "program":
			fmt.Fprintf(&buf, "PROGRAM %s\n", pou.Name)
			end = "END_PROGRAM"
		default:
			return "", fmt.Errorf("pou '%s': unknown pouType '%s'", pou.Name, pou.PouType)
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

		var body *FormattedText
		switch {
		case pou.Body.ST != nil:
			body = pou.Body.ST
		case pou.Body.IL != nil:
			body = pou.Body.IL
		case pou.Body.FBD != nil, pou.Body.LD != nil:
			// Graphical FBD and LD are lowered to ST statements (graphical.go).
			raw, lang := pou.Body.FBD, "FBD"
			if raw == nil {
				raw, lang = pou.Body.LD, "LD"
			}
			if opt.KeepDiagrams {
				text, err := diagramText(pou, lang, raw.Inner, projectFunctionBlocks(proj))
				if err == nil {
					buf.WriteString("\t" + lang + "\n")
					for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
						buf.WriteString("\t" + line + "\n")
					}
					buf.WriteString("\tEND_" + lang + "\n")
					break
				}
				fmt.Fprintf(&buf, "\t(* The %s diagram is imported as Structured Text: %s *)\n", lang, strings.ReplaceAll(err.Error(), "*)", "* )"))
			}
			low, err := LowerGraphical(pou.Name, raw.Inner, projectFunctionBlocks(proj), declaredNames(pou))
			if err != nil {
				return "", err
			}
			if len(low.Decls) > 0 {
				buf.WriteString("\tVAR\n")
				for _, d := range low.Decls {
					buf.WriteString("\t\t" + d + "\n")
				}
				buf.WriteString("\tEND_VAR\n\n")
			}
			for _, line := range strings.Split(strings.TrimRight(low.Body, "\n"), "\n") {
				if line != "" {
					buf.WriteString("\t" + line + "\n")
				}
			}
		case pou.Body.SFC != nil:
			return "", fmt.Errorf("pou '%s': graphical SFC bodies cannot be converted to text yet", pou.Name)
		}
		if body != nil {
			text, err := FormattedTextContent(body.Text)
			if err != nil {
				return "", fmt.Errorf("pou '%s': %w", pou.Name, err)
			}
			for _, line := range strings.Split(text, "\n") {
				if strings.TrimSpace(line) != "" {
					buf.WriteString("\t" + line + "\n")
				}
			}
		}
		buf.WriteString(end + "\n\n")
	}

	// 3. Configurations
	for _, cfg := range proj.Instances.Configurations.Configurations {
		fmt.Fprintf(&buf, "CONFIGURATION %s\n", cfg.Name)
		writeVarLists(&buf, "VAR_GLOBAL", cfg.GlobalVars)
		for _, res := range cfg.Resources {
			fmt.Fprintf(&buf, "\tRESOURCE %s ON PLC\n", res.Name)
			writeVarLists(&buf, "VAR_GLOBAL", res.GlobalVars)
			for _, task := range res.Tasks {
				settings := []string{}
				if task.Single != "" {
					settings = append(settings, "SINGLE := "+task.Single)
				}
				if task.Interval != "" {
					settings = append(settings, "INTERVAL := "+durationText(task.Interval))
				}
				settings = append(settings, fmt.Sprintf("PRIORITY := %d", task.Priority))
				fmt.Fprintf(&buf, "\t\tTASK %s (%s);\n", task.Name, strings.Join(settings, ", "))
			}
			for _, task := range res.Tasks {
				for _, inst := range task.PouInstances {
					fmt.Fprintf(&buf, "\t\tPROGRAM %s WITH %s : %s;\n", inst.Name, task.Name, inst.TypeName)
				}
			}
			for _, inst := range res.PouInstances {
				fmt.Fprintf(&buf, "\t\tPROGRAM %s : %s;\n", inst.Name, inst.TypeName)
			}
			buf.WriteString("\tEND_RESOURCE\n")
		}
		buf.WriteString("END_CONFIGURATION\n\n")
	}

	return strings.TrimSpace(buf.String()) + "\n", nil
}

// durationText returns a task interval as an IEC duration. Some tools write
// the interval without its T# prefix (20ms) or as an XML duration (PT0.02S);
// a variable name is kept as it is.
func durationText(interval string) string {
	upper := strings.ToUpper(interval)
	switch {
	case strings.HasPrefix(upper, "T#"), strings.HasPrefix(upper, "TIME#"):
		return interval
	case strings.HasPrefix(upper, "PT"):
		return "T#" + strings.ToLower(interval[2:])
	case interval != "" && (interval[0] >= '0' && interval[0] <= '9'):
		return "T#" + interval
	}
	return interval
}

// ImportToAST unmarshals PLCopen XML and parses it into a beedance AST Program.
func ImportToAST(xmlData []byte) (*ast.Program, error) {
	iecText, err := ImportToIECText(xmlData)
	if err != nil {
		return nil, err
	}

	p := parser.New(lexer.New(iecText))
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
		fmt.Fprintf(buf, "\t%s%s\n", sectionName, qualifiers)
		for _, v := range list.Variables {
			loc := ""
			if v.Address != "" {
				loc = fmt.Sprintf(" AT %s", v.Address)
			}
			initVal := ""
			if s := v.InitialValue.String(); s != "" {
				initVal = " := " + s
			}
			fmt.Fprintf(buf, "\t\t%s%s : %s%s;\n", v.Name, loc, typeText(v.Type, v.AddData), initVal)
		}
		buf.WriteString("\tEND_VAR\n\n")
	}
}

// FormattedTextContent returns the text of a PLCopen formattedText, given
// as its inner XML: the character data of its XHTML, with CDATA sections
// and entities resolved, and a line break for each <br/> and between
// paragraphs. Tools wrap code differently, e.g. <xhtml:p><![CDATA[...]]>
// or <xhtml xmlns="http://www.w3.org/1999/xhtml">...</xhtml>.
func FormattedTextContent(innerXML string) (string, error) {
	dec := xml.NewDecoder(strings.NewReader("<text xmlns:xhtml=\"http://www.w3.org/1999/xhtml\">" + innerXML + "</text>"))
	dec.Strict = false
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("formatted text: %w", err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.StartElement:
			if t.Name.Local == "br" {
				b.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p", "div":
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// CleanFormattedText returns the text of a PLCopen formattedText, or the
// input trimmed if it is not well-formed XML.
func CleanFormattedText(s string) string {
	text, err := FormattedTextContent(s)
	if err != nil {
		return strings.TrimSpace(s)
	}
	return text
}
