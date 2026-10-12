/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// minNode is the oldest Node.js the wasm engine runs on: 20 and 22 at times
// crash (SIGSEGV) running a Go WASI module after a run that grew its memory.
const minNode = 24

// wasmRuntime is the command that runs a WASI module on the wasm engine:
// Options.WasmRuntime, or the first of wasmtime, wazero and Node.js in PATH.
// For Node.js it writes its WASI runner into dir; wazero caches compiled
// modules there, so a run compiles the module once.
func wasmRuntime(ctx context.Context, opts Options, dir string) ([]string, error) {
	if len(opts.WasmRuntime) > 0 {
		if _, err := exec.LookPath(opts.WasmRuntime[0]); err != nil {
			return nil, fmt.Errorf("the WASI runtime %s: %v", opts.WasmRuntime[0], err)
		}
		return opts.WasmRuntime, nil
	}
	if p, err := exec.LookPath("wasmtime"); err == nil {
		return []string{p, "run", "-W", "max-wasm-stack=8388608"}, nil
	}
	if p, err := exec.LookPath("wazero"); err == nil {
		return []string{p, "run", "-cachedir", filepath.Join(dir, "wazero-cache")}, nil
	}
	if p, err := exec.LookPath("node"); err == nil {
		out, err := exec.CommandContext(ctx, p, "--version").Output()
		version := strings.TrimSpace(string(out))
		if err != nil {
			return nil, fmt.Errorf("node --version: %v", err)
		}
		major, _ := strconv.Atoi(strings.SplitN(strings.TrimPrefix(version, "v"), ".", 2)[0])
		if major < minNode {
			return nil, fmt.Errorf("Node.js %s is too old for WASI: the wasm engine needs %d or later, or wasmtime or wazero in PATH, or a runtime given (-wasm-runtime)", version, minNode)
		}
		runner := filepath.Join(dir, "wasi.mjs")
		if err := os.WriteFile(runner, []byte(nodeWASIRunner), 0o644); err != nil {
			return nil, err
		}
		return []string{p, "--no-warnings", runner}, nil
	}
	return nil, errors.New("no WASI runtime: the wasm engine needs wasmtime, wazero or Node.js in PATH, or a runtime given (-wasm-runtime)")
}

// nodeWASIRunner runs a WASI module under Node.js: node wasi.mjs MODULE ARGS.
const nodeWASIRunner = `import { readFile } from 'node:fs/promises';
import { argv, env, exit } from 'node:process';
import { WASI } from 'node:wasi';

const [module, ...args] = argv.slice(2);
const wasi = new WASI({ version: 'preview1', args: ['silrun', ...args], env, returnOnExit: true });
const instance = await WebAssembly.instantiate(
  await WebAssembly.compile(await readFile(module)),
  wasi.getImportObject(),
);
exit(wasi.start(instance));
`
