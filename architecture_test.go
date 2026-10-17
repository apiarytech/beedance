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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// packageDirs lists the directories under the module root that hold Go
// files, slash-separated, build tags or not. A directory in a module of its
// own (its go.mod) stands for the whole of that module; examples and
// testdata are left out.
func packageDirs(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		p := filepath.ToSlash(path)
		if d.IsDir() {
			name := d.Name()
			if p != "." && (strings.HasPrefix(name, ".") || name == "testdata" || name == "node_modules" || p == "examples") {
				return filepath.SkipDir
			}
			if p != "." {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					seen[p] = true
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			if dir := filepath.ToSlash(filepath.Dir(path)); dir != "." {
				seen[dir] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// TestArchitectureListsEveryPackage keeps doc/architecture.md and
// doc/architecture.svg up to date as packages are added: each package
// directory has a row in the packages table, or a row of a directory above
// it that names it (wasm/cmd/beedance-wasm under wasm), and each top-level
// package is drawn in the SVG.
func TestArchitectureListsEveryPackage(t *testing.T) {
	md, err := os.ReadFile("doc/architecture.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, m := range regexp.MustCompile("(?m)^\\| `([^`]+)` \\|(.*)$").FindAllStringSubmatch(string(md), -1) {
		rows[m[1]] = m[2]
	}
	covered := func(dir string) bool {
		if _, ok := rows[dir]; ok {
			return true
		}
		// A generated or helper package named in its parent's row.
		parent, base := filepath.ToSlash(filepath.Dir(dir)), filepath.Base(dir)
		return strings.Contains(rows[parent], "`"+base+"`") || strings.Contains(rows[parent], "/"+base+"`")
	}
	svg, err := os.ReadFile("doc/architecture.svg")
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, m := range regexp.MustCompile(`>([^<]+)<`).FindAllStringSubmatch(string(svg), -1) {
		text.WriteString(m[1])
		text.WriteString(" ")
	}
	drawn := text.String()

	tops := map[string]bool{}
	for _, dir := range packageDirs(t) {
		if !covered(dir) {
			t.Errorf("doc/architecture.md: the packages table has no row for %s", dir)
		}
		tops[strings.SplitN(dir, "/", 2)[0]] = true
	}
	for top := range tops {
		if !regexp.MustCompile(`(^|[^\w/])` + regexp.QuoteMeta(top) + `\b`).MatchString(drawn) {
			t.Errorf("doc/architecture.svg: %s is not drawn", top)
		}
	}
}
