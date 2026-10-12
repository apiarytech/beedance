# Dual Licensing

The `wasm` folder of beedance (package `wasm`, `cmd/beedance-wasm`, the
playground in `web/`, the Node.js hosts in `node/` and the Docker build) is
dual-licensed, like the rest of beedance. You may use it under the terms of
either the GNU General Public License version 2 (GPLv2) or a commercial
license.

## GPLv2

This folder can be used under the terms of the GNU General Public License as
published by the Free Software Foundation; either version 2 of the License,
or (at your option) any later version. The full text of the GPLv2 is in
[`gpl-2.0.md`](../gpl-2.0.md) at the root of beedance.

This is a good choice if you are developing open source software and want to
share your work with the community.

## Commercial License

If you wish to use this folder, or the WebAssembly modules built from it
(`beedance.wasm`, `beedance-wasi.wasm`), in a proprietary application, or
otherwise do not wish to be bound by the terms of the GPLv2, you must
purchase a commercial license. Please contact the author for more
information on obtaining a commercial license.

## The built modules

A module built from this folder holds the whole beedance toolchain (the
parser, the evaluator, the compiler and VM, the transpiler and the `TEST_`
runner), so it is under the same two licenses: whoever hosts or ships the
module, in a web page, a Node.js program or a WASI runtime, does so under
one of them.

A module also holds the Go runtime and standard library, and the playground
loads `wasm_exec.js`, which is copied from the Go that built the module.
Both are under Go's BSD-style license (`$(go env GOROOT)/LICENSE`), whose
notice goes with them when they are passed on. Neither is covered by the
licenses above.
