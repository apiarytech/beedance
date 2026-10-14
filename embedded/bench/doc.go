/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package bench measures beedance's three engines (the bytecode VM, the
// transpiler's Go and the evaluator) on microcontrollers: speed on a board
// or an emulator, size per engine, and a sampling profile. See README.md.
package bench

//go:generate go run ./gen
