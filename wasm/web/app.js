// beedance playground: the page around beedance.wasm, which gives it
// globalThis.beedance.handle (see package wasm for the requests).
'use strict';

const example = `FUNCTION_BLOCK Blink
VAR_INPUT run : BOOL; END_VAR
VAR_OUTPUT q : BOOL; END_VAR
VAR t : TON; END_VAR
t(IN := run AND NOT t.Q, PT := T#500ms);
IF t.Q THEN q := NOT q; END_IF;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    start AT %IX0.0 : BOOL;
    lamp AT %QX0.0 : BOOL;
    blinks : INT;
    b : Blink;
END_VAR
b(run := start);
IF b.q AND NOT lamp THEN blinks := blinks + 1; END_IF;
lamp := b.q;
END_PROGRAM

PROGRAM TEST_Blink
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR b : Blink; END_VAR
b(run := TRUE);
IF b.q THEN done := TRUE; END_IF;
END_PROGRAM

PROGRAM TEST_Arithmetic
VAR_OUTPUT failures : INT; message : STRING; END_VAR
IF 7 MOD 3 <> 1 THEN failures := failures + 1; message := 'MOD'; END_IF;
IF ABS(-2.5) <> 2.5 THEN failures := failures + 1; message := 'ABS'; END_IF;
END_PROGRAM
`;

const $ = (id) => document.getElementById(id);
const source = $('source');
const output = $('output');
const results = $('results');
let session = 0;
let variables = [];
let timer = 0;

function call(req) {
  return JSON.parse(globalThis.beedance.handle(JSON.stringify(req)));
}

function show(text, muted = false) {
  output.textContent = text;
  output.classList.toggle('muted', muted);
}

function showError(reply) {
  show(reply.error + (reply.errors ? '\n  ' + reply.errors.join('\n  ') : ''));
}

function el(tag, attrs = {}, ...children) {
  const e = document.createElement(tag);
  Object.assign(e, attrs);
  e.append(...children);
  return e;
}

function intervalMs() {
  return Math.max(1, Number($('interval').value) || 10);
}

function refreshPrograms() {
  const reply = call({ op: 'check', source: source.value });
  const select = $('program');
  const keep = select.value;
  select.replaceChildren(...reply.programs.map((p) => el('option', { value: p, textContent: p })));
  if (reply.programs.includes(keep)) select.value = keep;
  return reply;
}

$('check').onclick = () => {
  const reply = refreshPrograms();
  results.replaceChildren();
  show(reply.ok
    ? `OK: ${reply.programs.length} programs, ${reply.tests.length} tests\n${reply.programs.join(', ')}`
    : 'Parse errors:\n  ' + reply.errors.join('\n  '));
};

$('test').onclick = () => {
  const reply = call({
    op: 'test', source: source.value, engines: $('engines').value.split(','), intervalMs: intervalMs(),
  });
  results.replaceChildren();
  if (reply.error) return showError(reply);
  const rows = reply.results.map((r) => el('tr', {},
    el('td', {}, r.name),
    ...r.engines.map((e) => el('td', { className: e.passed ? 'pass' : 'fail' },
      `${e.engine} ${e.error ? 'error' : e.passed ? 'pass' : 'fail'} (${e.scanCount})`)),
    el('td', { className: r.mismatch ? 'fail' : 'muted' }, r.mismatch ? 'differ' : r.engines.length > 1 ? 'agree' : ''),
  ));
  results.append(el('table', {}, el('tbody', {}, ...rows)));
  const problems = reply.results.flatMap((r) => [
    ...(r.mismatch ? [`${r.name}: ${r.mismatch}`] : []),
    ...r.engines.flatMap((e) => (e.problems || []).map((p) => `${r.name} ${e.engine}: ${p}`)),
  ]);
  show((reply.passed ? `PASSED: all ${reply.results.length} tests` : `FAILED: ${reply.failed} of ${reply.results.length} tests`)
    + (problems.length ? '\n\n' + problems.join('\n') : ''));
};

$('transpile').onclick = () => {
  const reply = call({ op: 'transpile', source: source.value });
  results.replaceChildren();
  if (reply.error) return showError(reply);
  show(reply.go);
};

function stop() {
  clearInterval(timer);
  timer = 0;
  $('run').textContent = 'Run';
}

$('load').onclick = () => {
  stop();
  if (session) call({ op: 'unload', session });
  session = 0;
  refreshPrograms();
  const reply = call({ op: 'load', source: source.value, program: $('program').value, intervalMs: intervalMs() });
  for (const id of ['scan1', 'scan10', 'run']) $(id).disabled = !!reply.error;
  if (reply.error) {
    $('vars').replaceChildren();
    return showError(reply);
  }
  session = reply.session;
  variables = reply.variables;
  show(`Loaded ${reply.program} (session ${session}): ${variables.length} variables.`, true);
  render(reply);
};

function scan(count) {
  const reply = call({ op: 'scan', session, count });
  if (reply.error) {
    stop();
    return showError(reply);
  }
  render(reply);
}

$('scan1').onclick = () => scan(1);
$('scan10').onclick = () => scan(10);
$('run').onclick = () => {
  if (timer) return stop();
  $('run').textContent = 'Stop';
  timer = setInterval(() => scan(1), intervalMs());
};

function set(name, value) {
  const reply = call({ op: 'set', session, values: { [name]: value } });
  if (reply.error) return showError(reply);
  render(reply);
}

// render shows the watched values: the declared variables, the members of
// instances under dotted names, and the I/O image.
function render(reply) {
  $('clock').textContent = `${reply.scans ?? 0} scans · ${(reply.timeMs ?? 0) / 1000} s`;
  const types = Object.fromEntries(variables.map((v) => [v.name.toUpperCase(), v]));
  const rows = [];
  const settable = (v) => v && v.block !== 'VAR_EXTERNAL' && /^(BOOL|S?INT|D?INT|L?INT|U[SDL]?INT|L?REAL|BYTE|D?WORD|LWORD|STRING|TIME)$/.test(v.type);
  for (const [name, value] of Object.entries(reply.values)) {
    const v = types[name.toUpperCase()];
    let control = '';
    if (settable(v)) {
      if (v.type === 'BOOL') {
        control = el('input', { type: 'checkbox', checked: value === true, onchange: (e) => set(name, e.target.checked) });
      } else {
        control = el('input', {
          type: 'text', placeholder: String(value),
          onkeydown: (e) => {
            if (e.key !== 'Enter') return;
            const text = e.target.value.trim();
            const n = Number(text);
            set(name, v.type === 'STRING' || v.type === 'TIME' || text === '' || Number.isNaN(n) ? text : n);
          },
        });
      }
    }
    rows.push(el('tr', {},
      el('td', {}, name + (v && v.address ? ` (${v.address})` : '')),
      el('td', { className: 'muted' }, v ? v.type : ''),
      el('td', { className: 'v' }, String(value)),
      el('td', {}, control)));
  }
  for (const [addr, value] of Object.entries(reply.io || {})) {
    if (variables.some((v) => v.address && v.address.toUpperCase() === addr.toUpperCase())) continue;
    rows.push(el('tr', {}, el('td', {}, addr), el('td', { className: 'muted' }, 'I/O'),
      el('td', { className: 'v' }, String(value)), el('td', {}, '')));
  }
  $('vars').replaceChildren(...rows);
}

// The module: fetched as bytes, so any server's MIME type for .wasm does.
async function start() {
  source.value = example;
  const go = new Go();
  const ready = new Promise((resolve) => { globalThis.onBeedanceReady = resolve; });
  const bytes = await (await fetch('beedance.wasm')).arrayBuffer();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await ready;
  const v = call({ op: 'version' });
  $('status').textContent = `ready · ${v.goVersion} ${v.goos}/${v.goarch} · ${(bytes.byteLength / 1048576).toFixed(1)} MB`;
  refreshPrograms();
  show('Ready. Run the tests, or load Main and scan it; tick start to make the lamp blink.', true);
}

start().catch((err) => {
  $('status').textContent = 'failed to load';
  show(String(err));
});
