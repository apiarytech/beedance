//go:build lite

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

// The lite build (go build -tags lite): no tests (package sil and the
// evaluator) and no transpiler, for a page that edits, checks and debugs.
// Its version reply lists the ops it has.

import "errors"

var buildOps []string

var errLite = errors.New("not in this build of the module (built with -tags lite): use the full build")

func parseErrors(error) ([]string, bool) { return nil, false }

func testOp(Request) (any, error) { return nil, errLite }

func transpileOp(Request) (any, error) { return nil, errLite }
