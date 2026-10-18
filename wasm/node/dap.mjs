// Debugs a program with beedance-wasm built for the web (GOOS=js) under
// Node.js, as the page's debugger does: through globalThis.beedance.dap,
// one Debug Adapter Protocol message at a time. It checks the one thing a
// page needs that native tests cannot: a program left to run as fast as it
// goes still lets the event loop in, so timers fire and pause arrives.
//
//   node dap.mjs wasm_exec.js beedance.wasm
//
// It exits 0 when the session went as it should, and 1, with the reason,
// when not. Run it under a timeout: a program that never yields hangs it.

import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { argv, exit } from 'node:process';

const [execJS, module] = argv.slice(2);
if (!execJS || !module) {
  console.error('usage: node dap.mjs wasm_exec.js MODULE.wasm');
  exit(2);
}
createRequire(import.meta.url)(resolve(execJS)); // defines globalThis.Go

const go = new Go();
const ready = new Promise((r) => { globalThis.onBeedanceReady = r; });
const { instance } = await WebAssembly.instantiate(await readFile(module), go.importObject);
go.run(instance);
await ready;

const source = `PROGRAM Main
VAR
    n : DINT;
    start AT %IX0.0 : BOOL;
    lamp AT %QX0.0 : BOOL;
END_VAR
n := n + 1;
lamp := start;
END_PROGRAM
`;

let seq = 0;
const waiting = new Map(); // request seq -> resolve
const events = [];
let onEvent = null;
const session = globalThis.beedance.dap((text) => {
  const m = JSON.parse(text);
  if (m.type === 'response') {
    waiting.get(m.request_seq)?.(m);
    waiting.delete(m.request_seq);
  } else if (m.type === 'event') {
    events.push(m);
    onEvent?.();
  }
});

function call(command, args) {
  const s = ++seq;
  return new Promise((ok, fail) => {
    waiting.set(s, (m) => (m.success ? ok(m.body ?? {}) : fail(new Error(`${command}: ${m.message}`))));
    session.send(JSON.stringify({ seq: s, type: 'request', command, arguments: args }));
  });
}

function event(name) {
  return new Promise((ok) => {
    const look = () => {
      const i = events.findIndex((e) => e.event === name);
      if (i >= 0) {
        onEvent = null;
        ok(events.splice(i, 1)[0].body ?? {});
      }
    };
    onEvent = look;
    look();
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function check(ok, what) {
  if (!ok) {
    console.error('FAIL:', what);
    exit(1);
  }
}

// V8 compiles the module's functions for some seconds after it starts, and
// the event loop lags meanwhile: wait until a timer fires on time.
for (let i = 0; i < 100; i++) {
  const t = Date.now();
  await sleep(50);
  if (Date.now() - t < 80) break;
}

await call('initialize', { adapterID: 'beedance' });
await call('launch', { program: 'main.st', source, realtime: false });
await event('initialized');
await call('beedance/force', { name: '%IX0.0', value: 'TRUE' }); // applied before every scan
await call('configurationDone');
const t0 = Date.now();
await sleep(200); // fires only if the running program yields
const slept = Date.now() - t0;
const live = await call('beedance/live');
await call('pause', { threadId: 1 });
const stopped = await event('stopped');
const lamp = (live.Values ?? []).find((v) => v.Name === 'lamp')?.Text;
console.log(`a 200 ms timer fired after ${slept} ms; ${live.Scan} scans ran; stopped: ${stopped.reason}; lamp = ${lamp}`);
check(slept < 400, `a 200 ms timer took ${slept} ms: the running program starves the event loop`);
check(live.Scan > 100, `the program ran ${live.Scan} scans, want many`);
check(stopped.reason === 'pause', `stopped for ${stopped.reason}, want pause`);
check(lamp === 'TRUE', `lamp = ${lamp} with %IX0.0 forced TRUE`);
await call('disconnect');
console.log('ok');
exit(0);
