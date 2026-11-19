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
 * The bootloader's decisions (core.c), apart from the chip: what it needs of
 * the chip is the hal_ functions, which rp2040.c gives on the Pico and host.c
 * on a computer, for tests.
 */
#ifndef BOOT_CORE_H
#define BOOT_CORE_H

#include <stdint.h>

/* The status record: package boot's Record, little-endian. */
enum {
	ST_IDLE = 0,           /* RUN holds a kept image */
	ST_PENDING = 1,        /* STAGING holds an update to install */
	ST_INSTALL = 2,        /* RUN backed up; STAGING being copied into RUN */
	ST_TRIAL = 3,          /* the update runs, not yet confirmed by the app */
	ST_RESTORE_BACKUP = 4, /* BACKUP being copied into RUN */
	ST_RESTORE_GOLDEN = 5, /* GOLDEN being copied into RUN */
};

enum {
	F_ROLLED_BACK = 1, /* the last update was rolled back */
	F_RESTORED = 2,    /* RUN was found corrupt and restored */
	F_REJECTED = 4,    /* the last update did not match its CRC */
	F_NO_FALLBACK = 8, /* an image failed with nothing sound to go back to */
};

#define STATUS_MAGIC 0x53424442u /* "BDBS" */

typedef struct {
	uint32_t magic, seq, state, flags;
	uint32_t run_size, run_crc;
	uint32_t new_size, new_crc;
	uint32_t backup_size, backup_crc;
	uint32_t golden_size, golden_crc;
	uint32_t crc; /* CRC-32 of the words before it */
} boot_record;

#define RECORD_SIZE ((uint32_t)sizeof(boot_record))

/* What core.c needs of the chip. */
const uint8_t *hal_flash(void);                                   /* all of flash, to read */
void hal_erase(uint32_t off, uint32_t len);                       /* sectors */
void hal_program(uint32_t off, const uint8_t *data, uint32_t len); /* pages, into erased flash */
void hal_feed(void);                                              /* the watchdog */
void hal_jump(uint32_t off) __attribute__((noreturn));            /* start the image at off */
void hal_halt(void) __attribute__((noreturn));                    /* nothing to run */

/* boot runs the bootloader: it never returns. */
void boot(void) __attribute__((noreturn));

/* What the test app shares with core.c. */
uint32_t boot_crc32(uint32_t crc, const uint8_t *p, uint32_t n);
int boot_load(boot_record *r);
void boot_save(boot_record *r);

#endif
