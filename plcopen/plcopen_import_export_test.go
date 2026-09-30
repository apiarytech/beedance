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

func TestRoundTripSTAndXML(t *testing.T) {
	tempDir := t.TempDir()
	origSTPath := filepath.Join(tempDir, "machine_control.st")
	xmlPath := filepath.Join(tempDir, "machine_control.xml")
	roundTripSTPath := filepath.Join(tempDir, "machine_control_reconstructed.st")
	roundTripXMLPath := filepath.Join(tempDir, "machine_control_reconstructed.xml")

	stContent := `
TYPE
	T_Status : (IDLE, ACTIVE, ERROR);
END_TYPE

PROGRAM MachineCtrl
	VAR_INPUT
		Execute : BOOL := FALSE;
	END_VAR

	VAR_OUTPUT
		CurrentStatus : T_Status;
	END_VAR

	VAR
		StepCounter : INT := 0;
	END_VAR

	IF Execute THEN
		CurrentStatus := ACTIVE;
		StepCounter := StepCounter + 1;
	ELSE
		CurrentStatus := IDLE;
	END_IF;
END_PROGRAM

CONFIGURATION MachineConfig
	RESOURCE MachineResource ON PLC
		TASK MachineTask (INTERVAL := T#20ms, PRIORITY := 1);
		PROGRAM MachineInst WITH MachineTask : MachineCtrl;
	END_RESOURCE
END_CONFIGURATION
`
	if err := os.WriteFile(origSTPath, []byte(stContent), 0644); err != nil {
		t.Fatalf("failed to write original ST file: %v", err)
	}

	// Step 1: Convert original ST file -> XML file
	if err := ConvertIECToXMLFile(origSTPath, xmlPath); err != nil {
		t.Fatalf("ConvertIECToXMLFile failed on original ST: %v", err)
	}

	xmlBytes, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatalf("failed to read initial XML file: %v", err)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for initial XML: %v", err)
	}

	// Step 2: Convert XML file -> reconstructed ST file
	reconstructedST, err := ConvertXMLToIECText(xmlPath)
	if err != nil {
		t.Fatalf("ConvertXMLToIECText failed on XML file: %v", err)
	}

	if err := os.WriteFile(roundTripSTPath, []byte(reconstructedST), 0644); err != nil {
		t.Fatalf("failed to write reconstructed ST file: %v", err)
	}

	// Step 3: Re-parse reconstructed ST to verify syntactic validity
	l := lexer.New(reconstructedST)
	p := parser.New(l)
	astReconstructed := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parsing reconstructed ST failed:\n%s\nErrors: %v", reconstructedST, p.Errors())
	}

	if len(astReconstructed.Statements) != 3 {
		t.Fatalf("expected 3 declarations (TYPE + PROGRAM + CONFIGURATION), got %d", len(astReconstructed.Statements))
	}

	// Step 4: Convert reconstructed ST file -> secondary XML file and validate schema
	if err := ConvertIECToXMLFile(roundTripSTPath, roundTripXMLPath); err != nil {
		t.Fatalf("ConvertIECToXMLFile failed on reconstructed ST: %v", err)
	}

	roundTripXMLBytes, err := os.ReadFile(roundTripXMLPath)
	if err != nil {
		t.Fatalf("failed to read secondary XML file: %v", err)
	}

	if err := ValidateWithXSD(roundTripXMLBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for round-trip XML: %v", err)
	}
}
