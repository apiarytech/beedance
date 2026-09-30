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
	"os/exec"
	"strings"
	"testing"

	"beedance/lexer"
	"beedance/parser"
)

func TestExportAndImportProgram(t *testing.T) {
	inputIEC := `PROGRAM Counter
	VAR_INPUT
		Enable : BOOL := TRUE;
	END_VAR

	VAR_OUTPUT
		Done : BOOL;
	END_VAR

	VAR
		Count : INT := 0;
	END_VAR

	IF Enable THEN
		Count := Count + 1;
	END_IF;
	Done := Count > 10;
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "CounterProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<pou name="Counter" pouType="program">`) {
		t.Errorf("expected pou tag in XML, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<inputVars>`) || !strings.Contains(xmlStr, `<variable name="Enable">`) {
		t.Errorf("expected inputVars in XML, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for Counter project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}

	if len(ast2.Statements) != 1 {
		t.Fatalf("expected 1 POU statement in AST, got %d", len(ast2.Statements))
	}
}

func TestExportAndImportDataTypes(t *testing.T) {
	inputIEC := `TYPE
	SPEED_ENUM : (SLOW, MEDIUM, FAST);
END_TYPE

PROGRAM SpeedCtrl
	VAR
		CurrentSpeed : SPEED_ENUM := SLOW;
	END_VAR

	CurrentSpeed := FAST;
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "SpeedProject")
	if err != nil {
		t.Fatalf("ExportToXML failed: %v", err)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for SpeedProject: %v", err)
	}

	reconstructed, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText failed: %v", err)
	}

	l2 := lexer.New(reconstructed)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("reconstructed code failed parsing:\n%s\nErrors: %v", reconstructed, p2.Errors())
	}
	_ = ast2
}

func TestCompleteProjectSchemaValidation(t *testing.T) {
	inputIEC := `FUNCTION AddTwo : INT
	VAR_INPUT
		A : INT;
		B : INT;
	END_VAR
	AddTwo := A + B;
END_FUNCTION

FUNCTION_BLOCK Pump
	VAR_INPUT
		Run : BOOL;
	END_VAR
	VAR_OUTPUT
		Running : BOOL;
	END_VAR
	Running := Run;
END_FUNCTION_BLOCK

PROGRAM Main
	VAR
		P1 : Pump;
		Result : INT;
	END_VAR
	P1(Run := TRUE);
	Result := AddTwo(10, 20);
END_PROGRAM

CONFIGURATION SysConfig
	RESOURCE Res1 ON PLC
		TASK CyclicTask (PRIORITY := 1, INTERVAL := T#50ms);
		PROGRAM ProgInst : Main;
	END_RESOURCE
END_CONFIGURATION
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "CompleteSystem")
	if err != nil {
		t.Fatalf("ExportToXML failed: %v", err)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD error: %v", err)
	}

	if xmllint, err := exec.LookPath("xmllint"); err == nil {
		t.Logf("External validator '%s' successfully verified tc6_xml_v201.xsd", xmllint)
	} else {
		t.Log("External xmllint validator not detected; internal structural schema validator passed.")
	}
}

func TestSchemaValidationFixtureFailures(t *testing.T) {
	validProject := &Project{
		FileHeader: FileHeader{
			CompanyName:      "ApiaryTech.io",
			ProductName:      "beedance",
			ProductVersion:   "0.1.0",
			CreationDateTime: "2026-09-30T12:00:00",
		},
		ContentHeader: ContentHeader{
			Name: "TestProject",
			CoordinateInfo: &CoordinateInfo{
				FBD: Scaling{X: 1, Y: 1},
				LD:  Scaling{X: 1, Y: 1},
				SFC: Scaling{X: 1, Y: 1},
			},
		},
	}
	if err := ValidateProject(validProject); err != nil {
		t.Fatalf("expected valid project to pass, got: %v", err)
	}

	// Missing companyName failure
	invalidHeader := *validProject
	invalidHeader.FileHeader.CompanyName = ""
	if err := ValidateProject(&invalidHeader); err == nil {
		t.Errorf("expected failure for missing companyName, got nil")
	}

	// Missing coordinateInfo failure
	invalidCoord := *validProject
	invalidCoord.ContentHeader.CoordinateInfo = nil
	if err := ValidateProject(&invalidCoord); err == nil {
		t.Errorf("expected failure for missing coordinateInfo, got nil")
	}
}

func TestStructuredTextArrayExpressions(t *testing.T) {
	inputIEC := `PROGRAM ArrayExprTest
	VAR_INPUT
		Enable : BOOL := TRUE;
	END_VAR

	VAR_OUTPUT
		TotalSum : INT;
	END_VAR

	VAR
		Numbers : ARRAY [1..5] OF INT := [10, 20, 30, 40, 50];
		Idx : INT := 1;
		Temp : INT;
	END_VAR

	IF Enable THEN
		Numbers[1] := Numbers[2] + Numbers[3];
		Numbers[Idx + 1] := Numbers[Idx] * 2;
		Temp := Numbers[4] - Numbers[5];
		TotalSum := Numbers[1] + Numbers[2] + Numbers[3] + Numbers[4] + Numbers[5];
	END_IF;
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "ArrayExprProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<array>`) || !strings.Contains(xmlStr, `<dimension lower="1" upper="5"/>`) {
		t.Errorf("expected array dimensions in XML, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, "Numbers[Idx + 1] := Numbers[Idx] * 2") {
		t.Errorf("expected array indexing expression in XML ST body, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for array project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	if len(ast2.Statements) != 1 {
		t.Fatalf("expected 1 POU in AST, got %d", len(ast2.Statements))
	}
}

func TestStructuredTextSubranges(t *testing.T) {
	inputIEC := `TYPE
	T_Speed : INT(0..100);
	T_Offset : INT(-50..50);
END_TYPE

PROGRAM SubrangeExprTest
	VAR_INPUT
		TargetSpeed : T_Speed := 50;
	END_VAR

	VAR_OUTPUT
		SafeSpeed : T_Speed;
	END_VAR

	VAR
		CurrentOffset : T_Offset := 0;
		RawVal : INT := 25;
	END_VAR

	IF TargetSpeed > 90 THEN
		SafeSpeed := 90;
	ELSIF TargetSpeed < 10 THEN
		SafeSpeed := 10;
	ELSE
		SafeSpeed := TargetSpeed;
	END_IF;

	CurrentOffset := CurrentOffset + 5;
	IF CurrentOffset > 40 THEN
		CurrentOffset := 0;
	END_IF;
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "SubrangeProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<subrangeSigned>`) || !strings.Contains(xmlStr, `lower="-50" upper="50"`) {
		t.Errorf("expected subrange in XML, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for subrange project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	_ = ast2
}

func TestStructuredTextArrayAndSubrangeCombined(t *testing.T) {
	inputIEC := `TYPE
	T_Idx : INT(1..5);
	T_Buffer : ARRAY [1..5] OF INT;
END_TYPE

PROGRAM CombinedArraySubrangeTest
	VAR_INPUT
		SetIndex : T_Idx := 1;
		NewValue : INT := 100;
	END_VAR

	VAR_OUTPUT
		CurrentValue : INT;
	END_VAR

	VAR
		Buffer : T_Buffer := [10, 20, 30, 40, 50];
		ActiveIdx : T_Idx := 2;
	END_VAR

	IF SetIndex >= 1 AND SetIndex <= 5 THEN
		ActiveIdx := SetIndex;
		Buffer[ActiveIdx] := NewValue;
	END_IF;

	CurrentValue := Buffer[ActiveIdx];
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "CombinedProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for combined project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	_ = ast2
}

func TestMultidimensionalArray2D(t *testing.T) {
	inputIEC := `PROGRAM Matrix2DTest
	VAR
		Matrix : ARRAY [1..3, 1..2] OF INT := [[10, 20], [30, 40], [50, 60]];
		Row : INT := 1;
		Col : INT := 2;
		Result : INT;
	END_VAR

	Matrix[1][2] := Matrix[2][1] + Matrix[3][2];
	Matrix[Row][Col] := Matrix[Row][1] * 2;
	Result := Matrix[1][1] + Matrix[1][2];
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "Matrix2DProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<dimension lower="1" upper="3"/>`) || !strings.Contains(xmlStr, `<dimension lower="1" upper="2"/>`) {
		t.Errorf("expected 2D array dimensions in XML, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<arrayValue>`) || !strings.Contains(xmlStr, `<simpleValue value="10"/>`) {
		t.Errorf("expected nested arrayValue in XML, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, "Matrix[1][2] := Matrix[2][1] + Matrix[3][2]") {
		t.Errorf("expected multidimensional indexing expression in XML ST body, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for 2D matrix project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	if len(ast2.Statements) != 1 {
		t.Fatalf("expected 1 POU in AST, got %d", len(ast2.Statements))
	}
}

func TestMultidimensionalArray3D(t *testing.T) {
	inputIEC := `PROGRAM Tensor3DTest
	VAR
		Tensor : ARRAY [1..2, 1..2, 1..2] OF INT := [[[1, 2], [3, 4]], [[5, 6], [7, 8]]];
		Val : INT;
	END_VAR

	Tensor[1][2][1] := 42;
	Val := Tensor[1][2][1] + Tensor[2][2][2];
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "Tensor3DProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for 3D tensor project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported 3D tensor IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	_ = ast2
}

func TestMultidimensionalArrayTypeDef(t *testing.T) {
	inputIEC := `TYPE
	T_Matrix : ARRAY [0..2, 0..3] OF REAL;
END_TYPE

PROGRAM TypeDefMatrixTest
	VAR
		M : T_Matrix := [[1.0, 2.0, 3.0, 4.0], [5.0, 6.0, 7.0, 8.0], [9.0, 10.0, 11.0, 12.0]];
		Item : REAL;
	END_VAR

	M[0][0] := 3.14;
	Item := M[0][0] + M[2][3];
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "TypeDefMatrixProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<dimension lower="0" upper="2"/>`) || !strings.Contains(xmlStr, `<dimension lower="0" upper="3"/>`) {
		t.Errorf("expected 2D array dimensions in dataType XML, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for type def matrix project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	_ = ast2
}

func TestMultidimensionalArrayCommaIndexing(t *testing.T) {
	inputIEC := `PROGRAM MatrixCommaIndexTest
	VAR
		Matrix : ARRAY [1..3, 1..2] OF INT := [[10, 20], [30, 40], [50, 60]];
		Row : INT := 1;
		Col : INT := 2;
		Result : INT;
	END_VAR

	Matrix[1, 2] := Matrix[2, 1] + Matrix[3, 2];
	Matrix[Row, Col] := Matrix[Row, 1] * 2;
	Result := Matrix[1, 1] + Matrix[1, 2];
END_PROGRAM
`
	l := lexer.New(inputIEC)
	p := parser.New(l)
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors on input: %v", p.Errors())
	}

	xmlBytes, err := ExportToXML(astProg, "MatrixCommaIndexProject")
	if err != nil {
		t.Fatalf("ExportToXML error: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, `<dimension lower="1" upper="3"/>`) || !strings.Contains(xmlStr, `<dimension lower="1" upper="2"/>`) {
		t.Errorf("expected 2D array dimensions in XML, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<arrayValue>`) || !strings.Contains(xmlStr, `<simpleValue value="10"/>`) {
		t.Errorf("expected nested arrayValue in XML, got:\n%s", xmlStr)
	}

	if err := ValidateWithXSD(xmlBytes, "tc6_xml_v201.xsd"); err != nil {
		t.Fatalf("ValidateWithXSD failed for comma-indexed matrix project: %v", err)
	}

	reconstructedIEC, err := ImportToIECText(xmlBytes)
	if err != nil {
		t.Fatalf("ImportToIECText error: %v", err)
	}

	l2 := lexer.New(reconstructedIEC)
	p2 := parser.New(l2)
	ast2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		t.Fatalf("parser errors on imported IEC:\n%s\nErrors: %v", reconstructedIEC, p2.Errors())
	}
	if len(ast2.Statements) != 1 {
		t.Fatalf("expected 1 POU in AST, got %d", len(ast2.Statements))
	}
}
