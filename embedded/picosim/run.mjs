/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// run.mjs boots a firmware image on an emulated RP2040 (Raspberry Pi Pico)
// and serves its UART0 over TCP, so tests can talk to it like a serial port.
//
//   node run.mjs <firmware.hex|firmware.uf2|firmware.bin>
//
// Environment:
//   UART_PORT     TCP port for UART0 (default 4000, 0 disables)
//   UART_BACKLOG  bytes of UART output replayed to each new client (default 65536)
//   GDB_PORT      TCP port for the GDB remote stub (default 3333, 0 disables)
//   LOG_LEVEL     emulator log level: debug, info, warn or error (default error;
//                 warn shows firmware access to peripherals rp2040js lacks)
//   WIRES         jumper wires, OUT:IN,...: e.g. 2:10 drives GP10 from GP2
//   ADC           analog inputs, CHANNEL:VALUE,...: e.g. 0:2048 (12-bit)

import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { ConsoleLogger, GPIOPinState, LogLevel, Simulator } from 'rp2040js';
import { GDBTCPServer } from 'rp2040js/gdb-tcp-server';
import { bootromB1 } from './bootrom.mjs';

const FLASH_BASE = 0x10000000;
const UF2_FAMILY_RP2040 = 0xe48bff56;

const uartPort = envInt('UART_PORT', 4000);
const backlogSize = envInt('UART_BACKLOG', 65536);
const gdbPort = envInt('GDB_PORT', 3333);
const logLevels = { debug: LogLevel.Debug, info: LogLevel.Info, warn: LogLevel.Warn, error: LogLevel.Error };
const logLevel = logLevels[(process.env.LOG_LEVEL || 'error').toLowerCase()];
if (logLevel === undefined) fail(`LOG_LEVEL must be one of ${Object.keys(logLevels).join(', ')}`);

function envInt(name, def) {
  const v = process.env[name];
  if (v === undefined || v === '') return def;
  const n = Number.parseInt(v, 10);
  if (!Number.isInteger(n) || n < 0) fail(`${name} must be a non-negative integer, got ${v}`);
  return n;
}

function fail(msg) {
  console.error(`picosim: ${msg}`);
  process.exit(1);
}

// flashWrite copies data to the absolute address addr in the XIP flash window.
function flashWrite(flash, addr, data) {
  const off = addr - FLASH_BASE;
  if (off < 0 || off + data.length > flash.length) {
    fail(`image writes 0x${addr.toString(16)}..+${data.length}, outside flash`);
  }
  flash.set(data, off);
}

function loadHex(text, flash) {
  let base = 0;
  for (const [i, raw] of text.split(/\r?\n/).entries()) {
    const line = raw.trim();
    if (line === '') continue;
    if (line[0] !== ':') fail(`hex line ${i + 1}: missing ':'`);
    const rec = Buffer.from(line.slice(1), 'hex');
    if (rec.length < 5 || rec.length !== rec[0] + 5) fail(`hex line ${i + 1}: bad length`);
    if (rec.reduce((sum, b) => (sum + b) & 0xff, 0) !== 0) fail(`hex line ${i + 1}: bad checksum`);
    const data = rec.subarray(4, 4 + rec[0]);
    switch (rec[3]) {
      case 0x00: // data
        flashWrite(flash, base + rec.readUInt16BE(1), data);
        break;
      case 0x01: // end of file
        return;
      case 0x02: // extended segment address
        base = data.readUInt16BE(0) * 16;
        break;
      case 0x04: // extended linear address
        base = data.readUInt16BE(0) * 0x10000;
        break;
      case 0x03: // start segment address
      case 0x05: // start linear address
        break;
      default:
        fail(`hex line ${i + 1}: unknown record type ${rec[3]}`);
    }
  }
}

function loadUF2(buf, flash) {
  if (buf.length === 0 || buf.length % 512 !== 0) fail('UF2 size is not a multiple of 512');
  for (let off = 0; off < buf.length; off += 512) {
    if (
      buf.readUInt32LE(off) !== 0x0a324655 ||
      buf.readUInt32LE(off + 4) !== 0x9e5d5157 ||
      buf.readUInt32LE(off + 508) !== 0x0ab16f30
    ) {
      fail(`UF2 block at ${off}: bad magic`);
    }
    const flags = buf.readUInt32LE(off + 8);
    if (flags & 0x1) continue; // not for main flash
    if (flags & 0x2000) {
      const family = buf.readUInt32LE(off + 28);
      if (family !== UF2_FAMILY_RP2040) {
        fail(`UF2 family 0x${family.toString(16)} is not RP2040 (RP2350/pico2 is not emulated)`);
      }
    }
    const addr = buf.readUInt32LE(off + 12);
    const size = buf.readUInt32LE(off + 16);
    if (size > 476) fail(`UF2 block at ${off}: payload ${size} too large`);
    flashWrite(flash, addr, buf.subarray(off + 32, off + 32 + size));
  }
}

const image = process.argv[2];
if (!image) fail('usage: node run.mjs <firmware.hex|firmware.uf2|firmware.bin>');

const simulator = new Simulator();
const mcu = simulator.rp2040;
mcu.logger = new ConsoleLogger(logLevel);
mcu.loadBootrom(bootromB1);

switch (path.extname(image).toLowerCase()) {
  case '.hex':
    loadHex(fs.readFileSync(image, 'latin1'), mcu.flash);
    break;
  case '.uf2':
    loadUF2(fs.readFileSync(image), mcu.flash);
    break;
  case '.bin':
    flashWrite(mcu.flash, FLASH_BASE, fs.readFileSync(image));
    break;
  default:
    fail(`unsupported image type ${image}: want .hex, .uf2 or .bin`);
}
console.error(`picosim: loaded ${image}`);

// Jumper wires: an output pin drives an input pin, as on a bench rig.
for (const w of pairs('WIRES')) {
  const [from, to] = w;
  if (from > 29 || to > 29) fail(`WIRES: GP${from}:GP${to} is not a pin`);
  mcu.gpio[from].addListener((state) => mcu.gpio[to].setInputValue(state === GPIOPinState.High));
}
// Analog inputs: fixed readings, as from a potentiometer.
for (const [ch, value] of pairs('ADC')) {
  if (ch > 4 || value > 4095) fail(`ADC: channel ${ch} value ${value} out of range`);
  mcu.adc.channelValues[ch] = value;
}

// pairs reads an environment variable of A:B,... pairs of integers.
function pairs(name) {
  const v = (process.env[name] || '').trim();
  if (v === '') return [];
  return v.split(',').map((p) => {
    const m = /^\s*(\d+):(\d+)\s*$/.exec(p);
    if (!m) fail(`${name}: want A:B,..., got ${p}`);
    return [Number(m[1]), Number(m[2])];
  });
}

// UART0: output goes to stdout, to every connected client, and to a backlog
// that is replayed on connect so a client attaching after boot misses nothing.
const clients = new Set();
let backlog = Buffer.alloc(0);

mcu.uart[0].onByte = (value) => {
  const b = Buffer.of(value);
  process.stdout.write(b);
  if (backlogSize > 0) {
    backlog = Buffer.concat([backlog, b]);
    if (backlog.length > backlogSize) backlog = backlog.subarray(backlog.length - backlogSize);
  }
  for (const c of clients) c.write(b);
};

function feed(data) {
  for (const b of data) mcu.uart[0].feedByte(b);
}

if (uartPort > 0) {
  net
    .createServer((sock) => {
      sock.setNoDelay(true);
      if (backlog.length > 0) sock.write(backlog);
      clients.add(sock);
      sock.on('data', feed);
      sock.on('close', () => clients.delete(sock));
      sock.on('error', () => clients.delete(sock));
    })
    .listen(uartPort, () => console.error(`picosim: UART0 on tcp :${uartPort}`));
}

if (!process.stdin.isTTY) process.stdin.on('data', feed);

if (gdbPort > 0) {
  new GDBTCPServer(simulator, gdbPort);
  console.error(`picosim: GDB on tcp :${gdbPort}`);
}

for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => process.exit(0));

// Start at the second-stage bootloader at the head of flash, as the bootrom
// would after finding a valid boot2.
mcu.core.PC = FLASH_BASE;
simulator.execute();
