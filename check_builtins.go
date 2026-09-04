/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// camelToSnake converts a camelCase string to snake_case.
// e.g., "AnyIntToDt" -> "Any_Int_To_Dt"
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		// Add an underscore before an uppercase letter, but not if it's the first character.
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return result.String()
}

// snakeToPascal converts a snake_case string to PascalCase.
// e.g., "ANY_INT_TO_DT" -> "AnyIntToDt"
func snakeToPascal(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '_' {
			capitalizeNext = true
		} else if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

// checkBuiltins is the entry point for the built-in consistency checker.
// It parses the stdlib source files to ensure that the iota constants,
// the name-to-index map, and the actual function registrations are all in sync.
func checkBuiltins() {
	// This assumes the script is run from the project root.
	stdlibPath := "stdlib"
	indicesFile := filepath.Join(stdlibPath, "indices.go")
	mapFile := filepath.Join(stdlibPath, "builtin_indices_map.go")

	fmt.Println("--- Running Built-in Consistency Check ---")

	// 1. Parse all stdlib files to get iota constants, map keys, and registered functions.
	iotaConsts, err := parseIotaConstants(indicesFile)
	if err != nil {
		log.Fatalf("Error parsing iota constants from %s: %v", indicesFile, err)
	}
	fmt.Printf("Found %d iota constants in indices.go\n", len(iotaConsts))

	mapKeys, err := parseMapKeys(mapFile, "BuiltinNameToIndex")
	if err != nil {
		log.Fatalf("Error parsing map keys from %s: %v", mapFile, err)
	}
	fmt.Printf("Found %d keys in BuiltinNameToIndex map\n", len(mapKeys))

	registeredFuncs, err := parseRegisteredFunctions(stdlibPath)
	if err != nil {
		log.Fatalf("Error parsing registered functions in %s: %v", stdlibPath, err)
	}
	fmt.Printf("Found %d calls to object.RegisterBuiltin in stdlib/\n", len(registeredFuncs))
	fmt.Println("-------------------------------------------")

	// 2. Perform consistency checks.
	errorsFound := false

	// Check 1: Iota constants vs. Map keys
	expectedMapKeys := make(map[string]bool)
	for constName := range iotaConsts {
		// Heuristic: "BuiltinAbs" -> "ABS", "BuiltinAnyIntToDt" -> "ANY_INT_TO_DT"
		trimmed := strings.TrimPrefix(constName, "Builtin")
		key := strings.ToUpper(camelToSnake(trimmed))
		expectedMapKeys[key] = true
	}

	for key := range expectedMapKeys {
		if _, ok := mapKeys[key]; !ok {
			fmt.Printf("ERROR: Constant 'Builtin%s' exists in indices.go, but key '%s' is missing from BuiltinNameToIndex map.\n", key, key)
			errorsFound = true
		}
	}

	for key := range mapKeys {
		// Convert snake_case key to PascalCase to match iota constant naming
		pascalKey := snakeToPascal(key)
		constName := "Builtin" + pascalKey
		if _, ok := iotaConsts[constName]; !ok {
			fmt.Printf("ERROR: Key '%s' exists in BuiltinNameToIndex map, but constant '%s' is missing from indices.go.\n", key, constName)
			errorsFound = true
		}
	}

	// Check 2: Map keys vs. Registered functions
	for key := range mapKeys {
		if _, ok := registeredFuncs[key]; !ok {
			fmt.Printf("ERROR: Key '%s' exists in BuiltinNameToIndex map, but no 'object.RegisterBuiltin' call was found for it.\n", key)
			errorsFound = true
		}
	}

	for name := range registeredFuncs {
		if _, ok := mapKeys[name]; !ok {
			fmt.Printf("ERROR: Function '%s' is registered, but its key is missing from the BuiltinNameToIndex map.\n", name)
			errorsFound = true
		}
	}

	fmt.Println("-------------------------------------------")
	if errorsFound {
		fmt.Println("Consistency check FAILED. Please fix the discrepancies.")
		os.Exit(1)
	} else {
		fmt.Println("Consistency check PASSED. All files are in sync.")
	}
}

// parseIotaConstants parses a Go file and returns a set of constant names
// found within the first `const (...)` block that uses iota.
func parseIotaConstants(filePath string) (map[string]bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, err
	}

	consts := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if ok && decl.Tok == token.CONST {
			for _, spec := range decl.Specs {
				if vspec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range vspec.Names {
						consts[name.Name] = true
					}
				}
			}
			return false // Stop after the first const block, which is the one with iota.
		}
		return true
	})
	return consts, nil
}

// parseMapKeys parses a Go file and returns a set of string literal keys
// from the specified map variable.
func parseMapKeys(filePath, mapName string) (map[string]bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, err
	}

	keys := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		vspec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}

		if len(vspec.Names) == 1 && vspec.Names[0].Name == mapName {
			if compLit, ok := vspec.Values[0].(*ast.CompositeLit); ok {
				for _, elt := range compLit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if keyLit, ok := kv.Key.(*ast.BasicLit); ok && keyLit.Kind == token.STRING {
							key := strings.Trim(keyLit.Value, `"`)
							keys[key] = true
						}
					}
				}
			}
			return false // Found the map, stop inspecting.
		}
		return true
	})
	return keys, nil
}

// parseRegisteredFunctions walks a directory, parses all .go files,
// and finds all calls to `object.RegisterBuiltin`, returning a set of the
// function names (the second argument to the call).
func parseRegisteredFunctions(dirPath string) (map[string]bool, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dirPath, nil, 0)
	if err != nil {
		return nil, err
	}

	registered := make(map[string]bool)

	// Pre-populate with the dynamically generated type conversion functions.
	types := []string{"BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT", "REAL", "LREAL", "TIME", "DATE", "TOD", "DT", "STRING", "WSTRING", "BYTE", "WORD", "DWORD", "LWORD", "ANY_INT", "ANY_REAL", "BCD"}
	for _, from := range types {
		for _, to := range types {
			if from != to {
				registered[fmt.Sprintf("%s_TO_%s", from, to)] = true
			}
		}
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				selExpr, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				if x, ok := selExpr.X.(*ast.Ident); ok && x.Name == "object" && selExpr.Sel.Name == "RegisterBuiltin" {
					if len(call.Args) >= 2 {
						if nameLit, ok := call.Args[1].(*ast.BasicLit); ok && nameLit.Kind == token.STRING {
							registered[strings.Trim(nameLit.Value, `"`)] = true
						}
					}
				}
				return true
			})
		}
	}

	return registered, nil
}
