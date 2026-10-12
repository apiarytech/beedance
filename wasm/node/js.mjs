// Runs beedance-wasm built for the web (GOOS=js) under Node.js, as a page
// runs it, and answers JSON requests, one per line on stdin, with the reply
// of globalThis.beedance.handle, one per line on stdout:
//
//   node js.mjs wasm_exec.js beedance.wasm < requests.jsonl
//
// wasm_exec.js is the one of the Go that built the module
// ($(go env GOROOT)/lib/wasm/wasm_exec.js).

import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { argv, exit, stdin, stdout } from 'node:process';
import { createInterface } from 'node:readline';

const [execJS, module] = argv.slice(2);
if (!execJS || !module) {
  console.error('usage: node js.mjs wasm_exec.js MODULE.wasm');
  exit(2);
}
createRequire(import.meta.url)(resolve(execJS)); // defines globalThis.Go

const go = new Go();
const ready = new Promise((r) => { globalThis.onBeedanceReady = r; });
const { instance } = await WebAssembly.instantiate(await readFile(module), go.importObject);
go.run(instance).then(() => {
  console.error('beedance-wasm exited');
  exit(1);
});
await ready;

for await (const line of createInterface({ input: stdin, crlfDelay: Infinity })) {
  if (line.trim() !== '') {
    stdout.write(globalThis.beedance.handle(line) + '\n');
  }
}
// Exit once the replies are written: a pipe takes them asynchronously.
stdout.write('', () => exit(0));
