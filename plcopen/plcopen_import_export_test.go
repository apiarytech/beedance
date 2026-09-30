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

func TestImportSTAndExportXML(t *testing.T) {
	tempDir := t.TempDir()
	stPath := filepath.Join(tempDir, "motor_control.st")
	xmlPath := filepath.Join(tempDir, "motor_control.xml")

	stContent := `PROGRAM MotorControl
	VAR_INPUT
		Start : BOOL := FALSE;
		Stop : BOOL := FALSE;
	END_VAR

	VAR_OUTPUT
		Running : BOOL;
	END_VAR

	VAR
		CycleCount : INT := 0;
	END_VAR

	IF Start AND NOT Stop THEN
		Running := TRUE;
		CycleCount := CycleCount + 1;
	ELSIF Stop THEN
		Running := FALSE;
	END_IF;
END_PROGRAM
`
	if err := os.WriteFile(stPath, []byte(stContent), 0644); err != nil {
		t.Fatalf("failed to write ST source file: %v", err)
	}

	if err := ConvertIECToXMLFile(stPath, xmlPath); err != nil {
		t.Fatalf("ConvertIECToXMLFile failed: %v", err)
	}

	xmlBytes, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatalf("failed to read generated XML file: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<pou name="MotorControl" pouType="program">`) {
		t.Errorf("expected pou tag in exported XML, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<variable name="Start">`) {
		t.Errorf("expected variable 'Start' in exported XML, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for exported XML: %v", err)
	}
}

func TestImportXMLAndProduceST(t *testing.T) {
	tempDir := t.TempDir()
	xmlPath := filepath.Join(tempDir, "conveyor_system.xml")
	stOutputPath := filepath.Join(tempDir, "conveyor_system.st")

	sourceIEC := `PROGRAM ConveyorCtrl
	VAR_INPUT
		Sensor : BOOL := TRUE;
	END_VAR

	VAR
		Speed : INT := 100;
	END_VAR

	IF Sensor THEN
		Speed := Speed + 10;
	END_IF;
END_PROGRAM
`
	l := lexer.New(sourceIEC)
	p := parser.New(l)
	initialAST := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("initial parse errors: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(initialAST, "ConveyorProject")
	if err != nil {
		t.Fatalf("ExportToXML failed: %v", err)
	}
	if err := os.WriteFile(xmlPath, xmlBytes, 0644); err != nil {
		t.Fatalf("failed to write XML test fixture: %v", err)
	}

	reconstructedST, err := ConvertXMLToIECText(xmlPath)
	if err != nil {
		t.Fatalf("ConvertXMLToIECText failed: %v", err)
	}

	if err := os.WriteFile(stOutputPath, []byte(reconstructedST), 0644); err != nil {
		t.Fatalf("failed to write ST output file: %v", err)
	}

	reconstructedBytes, err := os.ReadFile(stOutputPath)
	if err != nil {
		t.Fatalf("failed to read reconstructed ST file: %v", err)
	}

	l2 := lexer.New(string(reconstructedBytes))
	p2 := parser.New(l2)
	reconstructedAST := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("reconstructed ST failed parsing:\n%s\nErrors: %v", string(reconstructedBytes), p2.Errors())
	}

	if len(reconstructedAST.Statements) != 1 {
		t.Fatalf("expected 1 POU declaration in reconstructed AST, got %d", len(reconstructedAST.Statements))
	}

	progDecl, ok := reconstructedAST.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("expected *ast.ProgramDeclaration, got %T", reconstructedAST.Statements[0])
	}
	if progDecl.Name.Value != "ConveyorCtrl" {
		t.Errorf("expected program name 'ConveyorCtrl', got '%s'", progDecl.Name.Value)
	}
}
