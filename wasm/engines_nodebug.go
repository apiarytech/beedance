//go:build nodebug

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

// The build with no debugger (go build -tags nodebug): the toolchain's
// ops and the load/scan sessions, without packages dap and vmdebug, for a
// page that runs and stops programs but does not debug them.

// debugEngines are none: there is no debugger.
var debugEngines = []string{}
