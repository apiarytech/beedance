// Runs beedance-wasm built for WASI (GOOS=wasip1) under Node.js:
//
//   node --no-warnings wasi.mjs beedance-wasi.wasm [serve | test [flags] [FILE.st...]]
//
// The module gets the process's stdin, stdout, stderr and environment, and
// the file system from / (as Go's go_wasip1_wasm_exec gives it), so FILE
// paths work as on the host. The exit code is the module's.

import { readFile } from 'node:fs/promises';
import { argv, cwd, env, exit } from 'node:process';
import { WASI } from 'node:wasi';

const [module, ...args] = argv.slice(2);
if (!module) {
  console.error('usage: node wasi.mjs MODULE.wasm [ARGS...]');
  exit(2);
}
const wasi = new WASI({
  version: 'preview1',
  args: ['beedance-wasm', ...args],
  env: { ...env, PWD: cwd() }, // Go's WASI port resolves relative paths against PWD
  preopens: { '/': '/' },
  returnOnExit: true,
});
const instance = await WebAssembly.instantiate(
  await WebAssembly.compile(await readFile(module)),
  wasi.getImportObject(),
);
exit(wasi.start(instance));
