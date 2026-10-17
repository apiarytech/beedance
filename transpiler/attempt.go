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

// The assignment attempt (IEC 61131-3 3rd edition), target ?= value: a Go
// type assertion. An interface target takes the instance if it implements
// the interface; a REF_TO T target takes a pointer to the T of an instance
// of T or of a function block extending it (whose T is embedded); else nil.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// transpileAttempt writes target ?= value.
func (t *Transpiler) transpileAttempt(stmt *ast.AssignmentStatement) error {
	target, err := t.expressionString(stmt.Left)
	if err != nil {
		return err
	}
	value, err := t.expressionString(stmt.Value)
	if err != nil {
		return err
	}
	declared := t.valueDataType(stmt.Left)
	if rt, isRef := declared.(*ast.RefToType); isRef {
		base := declaredTypeName(rt.BaseType)
		if _, isFB := t.lookupType(base).(*ast.FunctionBlockDeclaration); isFB {
			t.write("\tswitch v := any(%s).(type) {\n", value)
			for _, c := range t.derivedPaths(base) {
				if c.path == "" {
					t.write("\tcase *%s:\n\t\t%s = v\n", c.name, target)
					continue
				}
				t.write("\tcase *%s:\n\t\t%s = &v%s\n", c.name, target, c.path)
			}
			t.write("\tdefault:\n\t\t%s = nil\n\t}\n", target)
			return nil
		}
		// A REF_TO an interface: as an interface target.
		t.write("\tif v, ok := any(%s).(%s); ok {\n\t\t%s = &v\n\t} else {\n\t\t%s = nil\n\t}\n", value, base, target, target)
		return nil
	}
	typeName := declaredTypeName(declared)
	if _, isIface := t.lookupType(typeName).(*ast.InterfaceDeclaration); !isIface {
		return fmt.Errorf("the target of ?= must be an interface or a REF_TO, got %s", stmt.Left.String())
	}
	// An instance of a function block is passed by its address, as its
	// methods are.
	if _, isFB := t.lookupType(declaredTypeName(t.valueDataType(stmt.Value))).(*ast.FunctionBlockDeclaration); isFB {
		value = "&" + value
	}
	// The blocks that implement the interface, by name: Go would let any
	// value with the methods (any value at all for an empty interface).
	implementers := t.implementers(typeName)
	if len(implementers) == 0 {
		t.write("\t_ = %s\n\t%s = nil\n", value, target)
		return nil
	}
	t.write("\tswitch v := any(%s).(type) {\n", value)
	for _, name := range implementers {
		t.write("\tcase *%s:\n\t\t%s = v\n", name, target)
	}
	t.write("\tdefault:\n\t\t%s = nil\n\t}\n", target)
	return nil
}

// derivedPath is a function block of a REF_TO's type, and the path from an
// instance of it to its embedded instance of that type ("" for the type
// itself, ".BASE" for a block extending BASE).
type derivedPath struct {
	name, path string
}

// derivedPaths returns base and the function blocks extending it, with the
// path to their base, by name.
func (t *Transpiler) derivedPaths(base string) []derivedPath {
	var out []derivedPath
	for _, n := range t.typeInfo {
		fb, ok := n.(*ast.FunctionBlockDeclaration)
		if !ok {
			continue
		}
		path := ""
		for cur, depth := fb, 0; cur != nil && depth < 16; depth++ {
			if strings.EqualFold(cur.Name.Value, base) {
				out = append(out, derivedPath{name: fb.Name.Value, path: path})
				break
			}
			if cur.Extends == nil {
				break
			}
			parent := cur.Extends.String()
			path += "." + parent
			cur = t.getFunctionBlockDefinitionFromTypeInfo(parent)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// implementers returns the function blocks whose instances are of the
// interface iface, by name: those that implement it, or an interface that
// extends it, or extend a block that does. IEC 61131-3 interfaces are
// nominal, where Go's are structural (an empty interface holds anything).
func (t *Transpiler) implementers(iface string) []string {
	var interfaceIs func(name string, depth int) bool
	interfaceIs = func(name string, depth int) bool {
		if strings.EqualFold(name, iface) {
			return true
		}
		if depth > 16 {
			return false
		}
		if decl, ok := t.lookupType(name).(*ast.InterfaceDeclaration); ok {
			for _, parent := range decl.Extends {
				if interfaceIs(parent.String(), depth+1) {
					return true
				}
			}
		}
		return false
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range t.typeInfo {
		fb, ok := n.(*ast.FunctionBlockDeclaration)
		if !ok || fb.Name == nil || seen[fb.Name.Value] {
			continue
		}
		seen[fb.Name.Value] = true
		for cur, depth := fb, 0; cur != nil && depth < 16; depth++ {
			found := false
			for _, impl := range cur.Implements {
				if interfaceIs(impl.String(), 0) {
					found = true
					break
				}
			}
			if found {
				out = append(out, fb.Name.Value)
				break
			}
			if cur.Extends == nil {
				break
			}
			cur = t.getFunctionBlockDefinitionFromTypeInfo(cur.Extends.String())
		}
	}
	sort.Strings(out)
	return out
}
