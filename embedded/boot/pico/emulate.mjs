/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// emulate.mjs boots a flash image on an emulated RP2040 (rp2040js) until the
// bootloader starts an app or gives up, for package boot's tests: the same
// contract as host.c, on the bootloader built for the chip.
//
//   node emulate.mjs RP2040JS_ESM BOOTROM_JS FLASH_FILE [FAIL_AFTER]
//
// RP2040JS_ESM is rp2040js's ESM build (its dist/esm, with index.js);
// BOOTROM_JS exports bootromB1. It prints "jump OFFSET" when an app marks
// itself running (testapp.c), "halt" when the bootloader enters USB boot,
// "power lost" after FAIL_AFTER erases and programs, the last half done,
// or "timeout"; then writes the flash image back.
//
// rp2040js does not emulate the flash chip behind the SSI, so the bootrom's
// flash routines are stood in for here: an erase sets bytes to 0xFF, a
// program clears bits, as NOR flash does.

import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [esmDir, bootromPath, flashFile, failArg] = process.argv.slice(2);
if (!flashFile) {
  console.error('usage: node emulate.mjs RP2040JS_ESM BOOTROM_JS FLASH_FILE [FAIL_AFTER]');
  process.exit(2);
}
const { Simulator } = await import(pathToFileURL(path.join(esmDir, 'index.js')).href);
const { bootromB1 } = await import(pathToFileURL(bootromPath).href);

const FLASH_BASE = 0x10000000;
const RUN_OFF = 0x8000;
const MARK = 0x20041000;
const failAfter = failArg === undefined ? -1 : Number(failArg);

const sim = new Simulator();
const mcu = sim.rp2040;
mcu.loadBootrom(bootromB1);
const image = fs.readFileSync(flashFile);
mcu.flash.set(image, 0);

// The bootrom's function table: code -> address.
const rom = {};
for (let p = mcu.readUint16(0x14); ; p += 4) {
  const code = mcu.readUint16(p);
  if (code === 0) break;
  rom[String.fromCharCode(code & 0xff, code >> 8)] = mcu.readUint16(p + 2);
}

let ops = 0;
function finish(result) {
  fs.writeFileSync(flashFile, mcu.flash.subarray(0, image.length));
  console.log(result);
  process.exit(0);
}
function op() {
  ops++;
  return failAfter >= 0 && ops > failAfter;
}
function returnFromCall() {
  mcu.core.PC = mcu.core.LR & ~1;
}

const stand = {
  [rom.RE]() {
    const [addr, count] = mcu.core.registers;
    const fail = op();
    mcu.flash.fill(0xff, addr, addr + (fail ? count / 2 : count));
    if (fail) finish('power lost');
  },
  [rom.RP]() {
    const [addr, data, count] = mcu.core.registers;
    const fail = op();
    const n = fail ? count / 2 : count;
    for (let i = 0; i < n; i++) mcu.flash[addr + i] &= mcu.readUint8(data + i);
    if (fail) finish('power lost');
  },
  [rom.IF]() {},
  [rom.EX]() {},
  [rom.FC]() {},
  [rom.UB]() {
    finish('halt');
  },
};

// Start at boot2, as picosim does.
mcu.core.PC = FLASH_BASE;
const cycleNanos = 1e9 / 125_000_000;
for (let i = 0; i < 400_000_000; i++) {
  const pc = mcu.core.PC;
  const f = stand[pc] ?? stand[pc | 1];
  if (f) {
    f();
    returnFromCall();
    continue;
  }
  const cycles = mcu.core.executeInstruction();
  sim.clock.tick(cycles * cycleNanos);
  if ((i & 0xffff) === 0 && mcu.readUint32(MARK) === 0xa5a5a5a5) {
    const id = mcu.readUint32(MARK + 4);
    finish(`jump ${RUN_OFF.toString(16)} app ${id}`);
  }
}
finish('timeout');
