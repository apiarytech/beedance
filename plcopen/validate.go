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
	_ "embed"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// schemaXSD is the PLCopen TC6 XML v2.01 schema, tc6_xml_v201.xsd.
//
//go:embed tc6_xml_v201.xsd
var schemaXSD []byte

// SchemaXSD returns the PLCopen TC6 XML v2.01 schema.
func SchemaXSD() []byte {
	return schemaXSD
}

// FindSchemaFile locates the given XSD schema file across common relative locations.
func FindSchemaFile(filename string) (string, error) {
	candidates := []string{
		filename,
		filepath.Join("plcopen", filename),
		filepath.Join("..", "plcopen", filename),
	}
	for _, cand := range candidates {
		if abs, err := filepath.Abs(cand); err == nil {
			if info, err := os.Stat(abs); err == nil && !info.IsDir() {
				return abs, nil
			}
		}
	}
	return "", fmt.Errorf("schema file '%s' not found", filename)
}

// XSDValidatorAvailable reports whether ValidateWithXSD can check documents
// against the XSD itself, which needs xmllint on the PATH. Without it,
// ValidateWithXSD makes only ValidateProject's structural checks.
func XSDValidatorAvailable() bool {
	_, err := exec.LookPath("xmllint")
	return err == nil
}

// ValidateProject performs structural verification on a Project to ensure it complies
// with the schema rules defined in tc6_xml_v201.xsd.
func ValidateProject(proj *Project) error {
	if proj == nil {
		return fmt.Errorf("project is nil")
	}
	if proj.XMLName.Local != "" && proj.XMLName.Local != "project" {
		return fmt.Errorf("invalid root element name '%s', expected 'project'", proj.XMLName.Local)
	}
	if proj.XMLName.Space != "" && proj.XMLName.Space != "http://www.plcopen.org/xml/tc6_0201" {
		return fmt.Errorf("invalid namespace '%s', expected 'http://www.plcopen.org/xml/tc6_0201'", proj.XMLName.Space)
	}

	if proj.FileHeader.CompanyName == "" {
		return fmt.Errorf("fileHeader.companyName is required")
	}
	if proj.FileHeader.ProductName == "" {
		return fmt.Errorf("fileHeader.productName is required")
	}
	if proj.FileHeader.ProductVersion == "" {
		return fmt.Errorf("fileHeader.productVersion is required")
	}
	if proj.FileHeader.CreationDateTime == "" {
		return fmt.Errorf("fileHeader.creationDateTime is required")
	}
	if proj.ContentHeader.Name == "" {
		return fmt.Errorf("contentHeader.name is required")
	}
	if proj.ContentHeader.CoordinateInfo == nil {
		return fmt.Errorf("contentHeader.coordinateInfo is required")
	}

	for i, dt := range proj.Types.DataTypes.DataTypes {
		if dt.Name == "" {
			return fmt.Errorf("dataType[%d].name is required", i)
		}
	}
	names := map[string]bool{}
	for i, pou := range proj.Types.Pous.Pous {
		if pou.Name == "" {
			return fmt.Errorf("pou[%d].name is required", i)
		}
		if names[pou.Name] {
			return fmt.Errorf("pou '%s' is declared twice", pou.Name)
		}
		names[pou.Name] = true
		// The schema's pouType enumeration is case-sensitive.
		switch pou.PouType {
		case "program", "functionBlock", "function":
		default:
			return fmt.Errorf("pou '%s': invalid pouType '%s', must be 'program', 'functionBlock', or 'function'", pou.Name, pou.PouType)
		}
		if pou.PouType == "function" && (pou.Interface == nil || pou.Interface.ReturnType == nil) {
			return fmt.Errorf("function '%s': returnType is required", pou.Name)
		}
		if pou.Interface != nil {
			for _, lists := range [][]VarList{pou.Interface.InputVars, pou.Interface.OutputVars, pou.Interface.InOutVars,
				pou.Interface.LocalVars, pou.Interface.TempVars, pou.Interface.ExternalVars, pou.Interface.GlobalVars, pou.Interface.AccessVars} {
				for _, list := range lists {
					for j, v := range list.Variables {
						if v.Name == "" {
							return fmt.Errorf("pou '%s': variable[%d].name is required", pou.Name, j)
						}
					}
				}
			}
		}
	}

	for _, cfg := range proj.Instances.Configurations.Configurations {
		if cfg.Name == "" {
			return fmt.Errorf("configuration.name is required")
		}
		for j, res := range cfg.Resources {
			if res.Name == "" {
				return fmt.Errorf("configuration '%s': resource[%d].name is required", cfg.Name, j)
			}
			for k, task := range res.Tasks {
				if task.Name == "" {
					return fmt.Errorf("resource '%s': task[%d].name is required", res.Name, k)
				}
				if task.Priority < 0 || task.Priority > 65535 {
					return fmt.Errorf("task '%s': priority %d out of range [0..65535]", task.Name, task.Priority)
				}
				for m, inst := range task.PouInstances {
					if inst.Name == "" || inst.TypeName == "" {
						return fmt.Errorf("task '%s': pouInstance[%d] requires both name and typeName", task.Name, m)
					}
				}
			}
			for m, inst := range res.PouInstances {
				if inst.Name == "" || inst.TypeName == "" {
					return fmt.Errorf("resource '%s': pouInstance[%d] requires both name and typeName", res.Name, m)
				}
			}
		}
	}
	return nil
}

// ValidateWithXSD validates XML bytes: first with ValidateProject's
// structural checks, then, when xmllint is installed (see
// XSDValidatorAvailable), against the schema. xsdPath names the schema file;
// when it is empty or not found, the embedded tc6_xml_v201.xsd is used.
func ValidateWithXSD(xmlData []byte, xsdPath string) error {
	var proj Project
	if err := xml.Unmarshal(xmlData, &proj); err != nil {
		return fmt.Errorf("XML unmarshal failed: %w", err)
	}
	if err := ValidateProject(&proj); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		return nil // Only the structural checks are available.
	}

	dir, err := os.MkdirTemp("", "plcopen")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	schema := ""
	if xsdPath != "" {
		schema, _ = FindSchemaFile(xsdPath)
	}
	if schema == "" {
		schema = filepath.Join(dir, "tc6_xml_v201.xsd")
		if err := os.WriteFile(schema, schemaXSD, 0644); err != nil {
			return fmt.Errorf("write schema: %w", err)
		}
	}
	doc := filepath.Join(dir, "project.xml")
	if err := os.WriteFile(doc, xmlData, 0644); err != nil {
		return fmt.Errorf("write temp XML: %w", err)
	}
	out, err := exec.Command(xmllintPath, "--noout", "--schema", schema, doc).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xmllint validation failed: %s\n%w", string(out), err)
	}
	return nil
}
