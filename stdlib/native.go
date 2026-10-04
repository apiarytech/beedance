/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

// The built-ins behind native function blocks (see package native): a host
// gives a program the function blocks it allows, implemented in Go, and
// replaces these built-ins with its own for that program (VM.SetBuiltin, or
// in the evaluator's environment). Without a host they refuse every call,
// so a program can do nothing native unless its host allows it. Their names
// are not IEC identifiers, so no program can clash with them.

import "github.com/apiarytech/beedance/object"

func noNatives(name string) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		return object.NewBuiltinError("%s: this host allows no native function blocks", name)
	}
}

func init() {
	object.RegisterBuiltin(BuiltinNativeNew, "__NATIVE_NEW", noNatives("__NATIVE_NEW"))
	object.RegisterBuiltin(BuiltinNativeSet, "__NATIVE_SET", noNatives("__NATIVE_SET"))
	object.RegisterBuiltin(BuiltinNativeRun, "__NATIVE_RUN", noNatives("__NATIVE_RUN"))
	object.RegisterBuiltin(BuiltinNativeGet, "__NATIVE_GET", noNatives("__NATIVE_GET"))
}
