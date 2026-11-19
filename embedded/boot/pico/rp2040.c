/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

/*
 * The bootloader on the RP2040: its start, flash through the bootrom's
 * routines, the watchdog, and the jump into the app.
 *
 * boot2 enters reset() through the vector table at 0x10000100. reset()
 * copies the rest of the bootloader into RAM and runs it there, as flash
 * cannot be read while it is erased or programmed.
 */

#include <stdint.h>

#include "core.h"
#include "layout.h"

#define REG(a) (*(volatile uint32_t *)(a))

#define WATCHDOG_CTRL 0x40058000u
#define WATCHDOG_LOAD 0x40058004u
#define WATCHDOG_TICK 0x4005802Cu
#define WATCHDOG_ENABLE (1u << 30)
#define WATCHDOG_TRIGGER (1u << 31)
#define PSM_WDSEL 0x40010008u
#define SCB_VTOR 0xE000ED08u
/* The watchdog's load counts down twice a tick (RP2040-E1): its most is
 * about 8 seconds, far more than an erase. */
#define WATCHDOG_MAX 0x00FFFFFFu

extern uint32_t _ltext, _stext, _etext, _sbss, _ebss, _stack_top;

void reset(void);
static void fault(void);

/* The vector table boot2 enters through. Interrupts stay off; a fault
 * resets the chip through the watchdog. */
__attribute__((section(".vectors"), used)) static const void *const vectors[16] = {
	&_stack_top, (const void *)reset, (const void *)fault, (const void *)fault,
};

/* The bootrom's routines, by its table (RP2040 datasheet 2.8.3). */
typedef void *(*rom_lookup_fn)(uint32_t table, uint32_t code);

static void *rom_func(char a, char b) {
	rom_lookup_fn lookup = (rom_lookup_fn)(uintptr_t)(*(volatile uint16_t *)0x18u);
	uint32_t table = *(volatile uint16_t *)0x14u;
	return lookup(table, (uint32_t)a | (uint32_t)b << 8);
}

static void (*rom_connect_internal_flash)(void);
static void (*rom_flash_exit_xip)(void);
static void (*rom_flash_range_erase)(uint32_t addr, uint32_t count, uint32_t block_size, uint8_t block_cmd);
static void (*rom_flash_range_program)(uint32_t addr, const uint8_t *data, uint32_t count);
static void (*rom_flash_flush_cache)(void);
static void (*rom_reset_usb_boot)(uint32_t activity_pin_mask, uint32_t disable_interface_mask);

/* boot2, copied to RAM: run after each erase or program, it puts flash back
 * in its fast read mode, as the SDK does. */
static uint32_t boot2[64];

static void xip_off(void) {
	rom_connect_internal_flash();
	rom_flash_exit_xip();
}

static void xip_on(void) {
	rom_flash_flush_cache();
	((void (*)(void))((uintptr_t)boot2 | 1))();
}

const uint8_t *hal_flash(void) { return (const uint8_t *)XIP_BASE; }

void hal_erase(uint32_t off, uint32_t len) {
	xip_off();
	rom_flash_range_erase(off, len, 1u << 16, 0xD8);
	xip_on();
}

void hal_program(uint32_t off, const uint8_t *data, uint32_t len) {
	xip_off();
	rom_flash_range_program(off, data, len);
	xip_on();
}

void hal_feed(void) { REG(WATCHDOG_LOAD) = WATCHDOG_MAX; }

static void watchdog_start(void) {
	REG(WATCHDOG_CTRL) &= ~WATCHDOG_ENABLE;
	REG(WATCHDOG_TICK) = 1u << 9 | 12; /* a tick each 12 cycles of clk_ref */
	REG(PSM_WDSEL) = 0x0001FFFCu;      /* a watchdog reset resets all but the oscillators */
	REG(WATCHDOG_LOAD) = WATCHDOG_MAX;
	REG(WATCHDOG_CTRL) |= WATCHDOG_ENABLE;
}

/* hal_jump starts the image at off as boot2 starts an image: its vector
 * table, its stack, its reset handler. The watchdog stops: the app sets its
 * own. */
void hal_jump(uint32_t off) {
	REG(WATCHDOG_CTRL) &= ~WATCHDOG_ENABLE;
	const uint32_t *v = (const uint32_t *)(XIP_BASE + off);
	REG(SCB_VTOR) = XIP_BASE + off;
	__asm volatile("msr msp, %0\n\tbx %1\n" ::"r"(v[0]), "r"(v[1]));
	__builtin_unreachable();
}

/* hal_halt waits in the bootrom's USB mass storage mode, to be flashed
 * anew: nothing in flash can run. */
void hal_halt(void) {
	REG(WATCHDOG_CTRL) &= ~WATCHDOG_ENABLE;
	rom_reset_usb_boot(0, 0);
	for (;;) {
	}
}

static void fault(void) {
	REG(WATCHDOG_CTRL) |= WATCHDOG_TRIGGER;
	for (;;) {
	}
}

/* start runs from RAM. */
__attribute__((noreturn, noinline)) static void start(void) {
	rom_connect_internal_flash = rom_func('I', 'F');
	rom_flash_exit_xip = rom_func('E', 'X');
	rom_flash_range_erase = rom_func('R', 'E');
	rom_flash_range_program = rom_func('R', 'P');
	rom_flash_flush_cache = rom_func('F', 'C');
	rom_reset_usb_boot = rom_func('U', 'B');
	const volatile uint32_t *b2 = (const volatile uint32_t *)XIP_BASE;
	for (int i = 0; i < 64; i++) {
		boot2[i] = b2[i];
	}
	watchdog_start();
	boot();
}

/* reset runs from flash: it copies the bootloader to RAM and starts it
 * there. It calls nothing, so as to need nothing in RAM. */
__attribute__((section(".entry"), noreturn)) void reset(void) {
	volatile uint32_t *src = &_ltext, *dst = &_stext;
	while (dst < &_etext) {
		*dst++ = *src++;
	}
	for (dst = &_sbss; dst < &_ebss;) {
		*dst++ = 0;
	}
	void (*volatile f)(void) = start;
	f();
	__builtin_unreachable();
}

/* What the compiler may call for copies of structures. */
void *memcpy(void *dst, const void *src, unsigned n) {
	uint8_t *d = dst;
	const uint8_t *s = src;
	while (n--) {
		*d++ = *s++;
	}
	return dst;
}

void *memset(void *dst, int v, unsigned n) {
	uint8_t *d = dst;
	while (n--) {
		*d++ = (uint8_t)v;
	}
	return dst;
}
