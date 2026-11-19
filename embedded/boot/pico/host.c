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
 * core.c on a computer, for package boot's tests: flash is a file, as NOR
 * flash behaves (an erase sets bytes to 0xFF, a program only clears bits).
 *
 *   host FLASH_FILE [FAIL_AFTER]
 *
 * prints "jump OFFSET" or "halt" and writes the file back. FAIL_AFTER cuts
 * the power after that many erases and programs, the last one half done:
 * it prints "power lost".
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "core.h"
#include "layout.h"

static uint8_t flash[FLASH_SIZE];
static const char *path;
static long ops, fail_after = -1;

static void save_file(void) {
	FILE *f = fopen(path, "wb");
	if (!f || fwrite(flash, 1, FLASH_SIZE, f) != FLASH_SIZE) {
		perror(path);
		exit(2);
	}
	fclose(f);
}

/* step counts an operation; it reports whether the power fails during it. */
static int step(void) {
	ops++;
	return fail_after >= 0 && ops > fail_after;
}

static void power_lost(void) {
	save_file();
	printf("power lost\n");
	exit(0);
}

const uint8_t *hal_flash(void) { return flash; }

void hal_erase(uint32_t off, uint32_t len) {
	if (off % SECTOR_SIZE || len % SECTOR_SIZE || off + len > FLASH_SIZE) {
		fprintf(stderr, "bad erase %x+%x\n", off, len);
		exit(2);
	}
	if (step()) {
		memset(flash + off, 0xFF, len / 2); /* half erased */
		power_lost();
	}
	memset(flash + off, 0xFF, len);
}

void hal_program(uint32_t off, const uint8_t *data, uint32_t len) {
	if (off % PAGE_SIZE || len % PAGE_SIZE || off + len > FLASH_SIZE) {
		fprintf(stderr, "bad program %x+%x\n", off, len);
		exit(2);
	}
	uint32_t n = len;
	int fail = step();
	if (fail) {
		n = len / 2;
	}
	for (uint32_t i = 0; i < n; i++) {
		flash[off + i] &= data[i];
	}
	if (fail) {
		power_lost();
	}
}

void hal_feed(void) {}

void hal_jump(uint32_t off) {
	save_file();
	printf("jump %x\n", off);
	exit(0);
}

void hal_halt(void) {
	save_file();
	printf("halt\n");
	exit(0);
}

int main(int argc, char **argv) {
	if (argc < 2) {
		fprintf(stderr, "usage: host FLASH_FILE [FAIL_AFTER]\n");
		return 2;
	}
	path = argv[1];
	if (argc > 2) {
		fail_after = atol(argv[2]);
	}
	FILE *f = fopen(path, "rb");
	if (!f || fread(flash, 1, FLASH_SIZE, f) != FLASH_SIZE) {
		perror(path);
		return 2;
	}
	fclose(f);
	boot();
}
