/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast

import "strings"

// ConfigVarEntry is one VAR_CONFIG declaration matched to the program instance
// it configures.
type ConfigVarEntry struct {
	Resource *ResourceDeclaration
	Program  *ProgramConfiguration
	// Path is the variable path relative to the program instance, e.g.
	// ["COUNT"] or ["FB1", "C2"] for `STATION_2.P4.FB1.C2`.
	Path []string
	// Decl holds the entry's location (Decl.Location), type (Decl.DataType)
	// and initial value (Decl.Value). Any of them may be nil.
	Decl *VarDeclStatement
}

// RelativePath returns the entry's path relative to its program instance,
// joined with dots.
func (e *ConfigVarEntry) RelativePath() string { return strings.Join(e.Path, ".") }

// ResolveConfigVars matches every VAR_CONFIG declaration in the configuration
// to the program instance it configures. It accepts:
//
//   - the standard form, `resource.program.{fb.}variable`;
//   - the standard single-resource form, `program.{fb.}variable`, when the
//     configuration declares its tasks and programs without a RESOURCE block;
//   - the program-scoped form, `VAR_CONFIG program ... END_VAR`, whose entries
//     are paths relative to that program instance.
//
// Names are compared case-insensitively. Declarations that match no program
// instance are returned in unmatched, so callers can report them.
func (cd *ConfigurationDeclaration) ResolveConfigVars() (entries []*ConfigVarEntry, unmatched []*VarDeclStatement) {
	for _, block := range cd.VarConfigs {
		for _, decl := range block.Declarations {
			found := false
			path := FlattenIdentifierPath(decl.AccessPath)
			for _, res := range cd.Resources {
				for _, prog := range res.Programs {
					if rel, ok := relativeConfigPath(block, res, prog, path); ok {
						entries = append(entries, &ConfigVarEntry{Resource: res, Program: prog, Path: rel, Decl: decl})
						found = true
					}
				}
			}
			if !found {
				unmatched = append(unmatched, decl)
			}
		}
	}
	return entries, unmatched
}

// relativeConfigPath reports whether path, from the given VAR_CONFIG block,
// addresses a variable inside prog, and returns the path relative to prog.
func relativeConfigPath(block *ConfigVarDeclaration, res *ResourceDeclaration, prog *ProgramConfiguration, path []string) ([]string, bool) {
	if len(path) == 0 {
		return nil, false
	}
	// Program-scoped (non-standard) form: the block names the program instance.
	if block.ProgramInstanceName != nil {
		if strings.EqualFold(block.ProgramInstanceName.Value, prog.InstanceName.Value) {
			return path, true
		}
		return nil, false
	}
	prefix := []string{prog.InstanceName.Value}
	if !res.IsImplicit {
		prefix = append([]string{res.Name.Value}, prefix...)
	}
	if len(path) <= len(prefix) {
		return nil, false
	}
	for i, name := range prefix {
		if !strings.EqualFold(path[i], name) {
			return nil, false
		}
	}
	return path[len(prefix):], true
}

// FlattenIdentifierPath converts an identifier or a chain of member accesses,
// such as `a.b.c`, into its parts ["a", "b", "c"]. It returns nil if the
// expression contains anything other than identifiers.
func FlattenIdentifierPath(expr Expression) []string {
	switch e := expr.(type) {
	case *Identifier:
		return []string{e.Value}
	case *MemberAccessExpression:
		head := FlattenIdentifierPath(e.Struct)
		if head == nil || e.Member == nil {
			return nil
		}
		return append(head, e.Member.Value)
	}
	return nil
}
