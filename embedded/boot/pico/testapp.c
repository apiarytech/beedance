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
 * An app for the bootloader's tests on the emulated RP2040: linked at the
 * run slot, it writes APP_ID where the test reads it (0x20041000) and waits.
 * The test, not the app, confirms an update, through package boot.
 */

#include <stdint.h>

#ifndef APP_ID
#define APP_ID 1
#endif

void app_reset(void);

__attribute__((section(".vectors"), used)) static const void *const vectors[16] = {
	(const void *)0x20040000u, (const void *)app_reset,
};

/* Images of different sizes: an update must copy all of an image. */
__attribute__((used)) static const uint8_t padding[APP_ID * 3000] = {APP_ID};

void app_reset(void) {
	volatile uint32_t *mark = (volatile uint32_t *)0x20041000u;
	mark[1] = APP_ID + padding[0] - APP_ID;
	mark[0] = 0xA5A5A5A5u;
	for (;;) {
	}
}
