//go:build !nodebug

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

import "github.com/apiarytech/beedance/vmdebug"

// debugEngines are the debugger's engines a launch may name.
var debugEngines = []string{string(vmdebug.EngineVM), string(vmdebug.EngineEvaluator)}
