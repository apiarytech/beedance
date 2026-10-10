/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// This file maps located variables (AT %IX0.0, %QW1, %MD4) to royaljelly's
// process image: the generated file's processImage, a vars.ProcessImage that
// I/O drivers also read and write. As in a PLC scan, a program or
// function block reads its inputs and memory from the image when it starts,
// and writes its outputs and memory back when it ends.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// processImageSize is the number of entries in each array of royaljelly's
// process image (vars.AddressSize).
const processImageSize = 255

// processImageArrays gives the Go type of each array of the process image.
var processImageArrays = map[string]string{
	"B": "iec.BOOL", "C": "iec.BYTE", "D": "iec.DWORD", "L": "iec.LWORD",
	"W": "iec.WORD", "R": "iec.REAL", "LR": "iec.LREAL", "S": "iec.STRING", "WS": "iec.WSTRING",
}

// processImageSlot returns the area (I, Q or M) of a located variable and
// the entry of the process image that holds it, such as `img.I.B[3]`, with
// the entry's Go type.
func processImageSlot(address, goType string) (area, slot, slotType string, err error) {
	addr := strings.ToUpper(strings.TrimPrefix(address, "%"))
	if addr == "" || !strings.ContainsRune("IQM", rune(addr[0])) {
		return "", "", "", fmt.Errorf("located address %%%s must be in the I, Q or M area", address)
	}
	area, addr = addr[:1], addr[1:]
	size := "X"
	if addr != "" && strings.ContainsRune("XBWDL", rune(addr[0])) {
		size, addr = addr[:1], addr[1:]
	}
	parts := strings.Split(addr, ".")
	numbers := make([]int, len(parts))
	for i, part := range parts {
		n, convErr := strconv.Atoi(part)
		if convErr != nil || n < 0 {
			return "", "", "", fmt.Errorf("located address %%%s is not supported; it must be a fixed address such as %%IX0.0", address)
		}
		numbers[i] = n
	}

	var array string
	index := numbers[0]
	switch {
	case goType == "iec.BOOL":
		if size != "X" || len(numbers) > 2 {
			return "", "", "", fmt.Errorf("a BOOL must be located at a bit address such as %%IX0.0, got %%%s", address)
		}
		array = "B"
		if len(numbers) == 2 {
			if numbers[1] > 7 {
				return "", "", "", fmt.Errorf("bit %d of %%%s is not a bit of a byte", numbers[1], address)
			}
			index = numbers[0]*8 + numbers[1]
		}
	case goType == "iec.REAL":
		array = "R"
	case goType == "iec.LREAL":
		array = "LR"
	case goType == "iec.STRING":
		array = "S"
	case goType == "iec.WSTRING":
		array = "WS"
	case isNumericGoType(goType) || goType == "iec.BYTE" || goType == "iec.WORD" || goType == "iec.DWORD" || goType == "iec.LWORD":
		switch size {
		case "B":
			array = "C"
		case "W":
			array = "W"
		case "D":
			array = "D"
		case "L":
			array = "L"
		default:
			return "", "", "", fmt.Errorf("a %s must be located at a byte, word, double word or long word address, got %%%s", strings.TrimPrefix(goType, "iec."), address)
		}
	default:
		return "", "", "", fmt.Errorf("a variable of type %s cannot be located at %%%s", strings.TrimPrefix(goType, "iec."), address)
	}
	if array != "B" && len(numbers) > 1 {
		return "", "", "", fmt.Errorf("located address %%%s must be a single number after its size", address)
	}
	if index >= processImageSize {
		return "", "", "", fmt.Errorf("located address %%%s is outside royaljelly's process image", address)
	}
	return area, fmt.Sprintf("img.%s.%s[%d]", area, array, index), processImageArrays[array], nil
}

// transpileProcessImage writes the statements that read the located
// variables of blocks from the process image, and a deferred function that
// writes them back when the scan ends. receiver is the program's or function
// block's receiver name.
func (t *Transpiler) transpileProcessImage(receiver string, blocks ...[]*ast.VarDeclStatement) error {
	reads, writes := []string{}, []string{}
	for _, block := range blocks {
		for _, decl := range block {
			if decl.Location == nil || decl.Location.Location == nil {
				continue
			}
			goType := t.mapIecTypeToGo(decl.DataType)
			area, slot, slotType, err := processImageSlot(decl.Location.Location.Address, goType)
			if err != nil {
				return fmt.Errorf("'%s': %w", decl.Name.Value, err)
			}
			field := receiver + "." + decl.Name.Value
			if area == "I" || area == "M" {
				reads = append(reads, fmt.Sprintf("\t\t%s = %s(%s)\n", field, goType, slot))
			}
			if area == "Q" || area == "M" {
				writes = append(writes, fmt.Sprintf("\t\t%s = %s(%s)\n", slot, slotType, field))
			}
		}
	}
	if len(reads) > 0 {
		t.usesProcessImage = true
		t.write("\tprocessImage.Read(func(img *vars.Image) {\n%s\t})\n", strings.Join(reads, ""))
	}
	if len(writes) > 0 {
		t.usesProcessImage = true
		t.write("\tdefer processImage.Write(func(img *vars.Image) {\n%s\t})\n", strings.Join(writes, ""))
	}
	return nil
}

// processImageDecl declares the process image of a generated file.
const processImageDecl = `
// processImage holds the located variables (AT %I, %Q, %M) of the programs
// and function blocks in this file. I/O drivers read the outputs from it and
// write the inputs to it.
var processImage vars.ProcessImage
`

// accessVarGoType returns the Go type of a VAR_ACCESS variable: its declared
// type, else the type of the variable its path names, else any.
func (t *Transpiler) accessVarGoType(decl *ast.VarDeclStatement) string {
	if decl.DataType != nil {
		return t.mapIecTypeToGo(decl.DataType)
	}
	if td := t.resolveAssignmentTargetType(decl.AccessPath); td != nil {
		return t.mapIecTypeToGo(td.DataType)
	}
	return "any"
}

// transpileLinkAccess writes the LinkAccess method of a program with
// VAR_ACCESS variables. Each is a pointer to the variable its access path
// names, which resolve finds, as the configuration that runs the program
// knows every program instance.
func (t *Transpiler) transpileLinkAccess(prog *ast.ProgramDeclaration) error {
	decls := []*ast.VarDeclStatement{}
	for _, block := range prog.VarAccess {
		decls = append(decls, block.Vars...)
	}
	if len(decls) == 0 {
		return nil
	}
	t.write("// LinkAccess points the program's VAR_ACCESS variables at the variables\n")
	t.write("// their access paths name; resolve returns a pointer to the variable a path names.\n")
	t.write("func (p *%s) LinkAccess(resolve func(path string) any) error {\n", prog.Name.Value)
	for _, decl := range decls {
		goType := t.accessVarGoType(decl)
		t.write("\tif v, ok := resolve(%q).(*%s); ok {\n", decl.AccessPath.String(), goType)
		t.write("\t\tp.%s = v\n\t} else {\n", decl.Name.Value)
		t.write("\t\treturn fmt.Errorf(%q)\n\t}\n", fmt.Sprintf("VAR_ACCESS %s: %s does not name a variable of type %s", decl.Name.Value, decl.AccessPath.String(), strings.TrimPrefix(goType, "iec.")))
	}
	t.write("\treturn nil\n}\n\n")
	return nil
}

// ProcessImageSlot returns where a variable of the elementary type typ
// (e.g. "INT"), located at address (e.g. "%IW0"), lives in the process
// image of a transpiled file: its area (I, Q or M), the Go expression of
// its entry in an image img, such as `img.I.W[0]`, and the entry's Go type,
// such as `iec.WORD`. A host that drives a transpiled program's I/O (the
// sil package's go engine) writes and reads the entries.
func ProcessImageSlot(address, typ string) (area, slot, slotType string, err error) {
	return processImageSlot(strings.TrimPrefix(address, "%"), "iec."+strings.ToUpper(typ))
}
