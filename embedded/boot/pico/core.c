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
 * The bootloader's decisions, apart from the chip.
 *
 * The status lives in two sectors, each a copy written whole: a new record
 * goes over the older copy, so a reset mid-write spoils at most the copy
 * being written. Every state that copies flash is recorded before the copy
 * starts and is safe to start again, so a reset at any point leaves the
 * bootloader something to finish or to run.
 */

#include "core.h"
#include "layout.h"

static uint32_t crc_table[256];
static int newest = -1; /* the status sector holding the newest record */
static uint8_t buf[SECTOR_SIZE];

static void crc_init(void) {
	for (uint32_t i = 0; i < 256; i++) {
		uint32_t c = i;
		for (int k = 0; k < 8; k++) {
			c = (c & 1) ? 0xEDB88320u ^ (c >> 1) : c >> 1;
		}
		crc_table[i] = c;
	}
}

/* boot_crc32 is CRC-32 (IEEE), as Go's crc32.Update with the IEEE table. */
uint32_t boot_crc32(uint32_t crc, const uint8_t *p, uint32_t n) {
	if (crc_table[1] == 0) {
		crc_init();
	}
	crc = ~crc;
	while (n--) {
		crc = crc_table[(crc ^ *p++) & 0xFF] ^ (crc >> 8);
	}
	return ~crc;
}

static void copy_bytes(uint8_t *dst, const uint8_t *src, uint32_t n) {
	while (n--) {
		*dst++ = *src++;
	}
}

static void fill_bytes(uint8_t *dst, uint8_t v, uint32_t n) {
	while (n--) {
		*dst++ = v;
	}
}

static int record_valid(const boot_record *r) {
	return r->magic == STATUS_MAGIC && r->crc == boot_crc32(0, (const uint8_t *)r, RECORD_SIZE - 4);
}

/* boot_load reads the newer valid status record into r; 0 when there is none. */
int boot_load(boot_record *r) {
	const uint8_t *f = hal_flash();
	boot_record a, b;
	copy_bytes((uint8_t *)&a, f + STATUS_OFF, RECORD_SIZE);
	copy_bytes((uint8_t *)&b, f + STATUS_OFF + SECTOR_SIZE, RECORD_SIZE);
	int va = record_valid(&a), vb = record_valid(&b);
	if (va && vb) {
		/* By serial number arithmetic, so the sequence may wrap. */
		newest = (int32_t)(b.seq - a.seq) > 0 ? 1 : 0;
	} else if (va) {
		newest = 0;
	} else if (vb) {
		newest = 1;
	} else {
		newest = -1;
		return 0;
	}
	copy_bytes((uint8_t *)r, (const uint8_t *)(newest ? &b : &a), RECORD_SIZE);
	return 1;
}

/* boot_save writes r, a sequence number on, over the older status copy. */
void boot_save(boot_record *r) {
	int i = newest == 0 ? 1 : 0;
	r->magic = STATUS_MAGIC;
	r->seq++;
	r->crc = boot_crc32(0, (const uint8_t *)r, RECORD_SIZE - 4);
	fill_bytes(buf, 0xFF, PAGE_SIZE);
	copy_bytes(buf, (const uint8_t *)r, RECORD_SIZE);
	hal_erase(STATUS_OFF + (uint32_t)i * SECTOR_SIZE, SECTOR_SIZE);
	hal_program(STATUS_OFF + (uint32_t)i * SECTOR_SIZE, buf, PAGE_SIZE);
	hal_feed();
	newest = i;
}

static int region_ok(uint32_t off, uint32_t size, uint32_t crc) {
	if (size == 0 || size > REGION_SIZE) {
		return 0;
	}
	uint32_t c = 0;
	for (uint32_t done = 0; done < size; done += SECTOR_SIZE) {
		uint32_t n = size - done < SECTOR_SIZE ? size - done : SECTOR_SIZE;
		c = boot_crc32(c, hal_flash() + off + done, n);
		hal_feed();
	}
	return c == crc;
}

/* image_size is the length of the image in a region, to its last byte that
 * is not erased, a page at a time: what a first boot copies of an app
 * flashed by hand, with no record of its size. */
static uint32_t image_size(uint32_t off) {
	const uint8_t *f = hal_flash() + off;
	uint32_t n = REGION_SIZE;
	while (n > 0 && f[n - 1] == 0xFF) {
		n--;
	}
	return (n + PAGE_SIZE - 1) / PAGE_SIZE * PAGE_SIZE;
}

/* vectors_ok reports whether the image at off starts with a vector table
 * the chip can run: a stack in RAM, a reset handler in Thumb code in RUN. */
static int vectors_ok(uint32_t off) {
	const uint8_t *f = hal_flash() + off;
	uint32_t sp = f[0] | f[1] << 8 | f[2] << 16 | (uint32_t)f[3] << 24;
	uint32_t pc = f[4] | f[5] << 8 | f[6] << 16 | (uint32_t)f[7] << 24;
	return sp > 0x20000000u && sp <= 0x20042000u && (pc & 1) &&
	       (pc & ~1u) >= XIP_BASE + RUN_OFF && (pc & ~1u) < XIP_BASE + RUN_OFF + REGION_SIZE;
}

/* copy copies size bytes from src to dst, a sector at a time. */
static void copy(uint32_t src, uint32_t dst, uint32_t size) {
	for (uint32_t done = 0; done < size; done += SECTOR_SIZE) {
		uint32_t n = size - done < SECTOR_SIZE ? size - done : SECTOR_SIZE;
		copy_bytes(buf, hal_flash() + src + done, n);
		fill_bytes(buf + n, 0xFF, SECTOR_SIZE - n);
		hal_erase(dst + done, SECTOR_SIZE);
		hal_program(dst + done, buf, (n + PAGE_SIZE - 1) / PAGE_SIZE * PAGE_SIZE);
		hal_feed();
	}
}

/* fallback is the state that restores RUN from the newest sound image
 * kept: BACKUP, else GOLDEN; ST_IDLE when neither is sound. */
static uint32_t fallback(const boot_record *r) {
	if (region_ok(BACKUP_OFF, r->backup_size, r->backup_crc)) {
		return ST_RESTORE_BACKUP;
	}
	if (region_ok(GOLDEN_OFF, r->golden_size, r->golden_crc)) {
		return ST_RESTORE_GOLDEN;
	}
	return ST_IDLE;
}

/* run starts RUN when it holds a sound image. */
static void run_if_sound(const boot_record *r) {
	if (region_ok(RUN_OFF, r->run_size, r->run_crc) && vectors_ok(RUN_OFF)) {
		hal_jump(RUN_OFF);
	}
}

void boot(void) {
	boot_record r;
	if (!boot_load(&r)) {
		/* First boot: the app was flashed by hand. Its copy in GOLDEN is the
		 * image of last resort, never overwritten. */
		fill_bytes((uint8_t *)&r, 0, RECORD_SIZE);
		r.state = ST_IDLE;
		r.run_size = image_size(RUN_OFF);
		if (r.run_size > 0) {
			r.run_crc = boot_crc32(0, hal_flash() + RUN_OFF, r.run_size);
			copy(RUN_OFF, GOLDEN_OFF, r.run_size);
			if (region_ok(GOLDEN_OFF, r.run_size, r.run_crc)) {
				r.golden_size = r.run_size;
				r.golden_crc = r.run_crc;
			}
		}
		boot_save(&r);
	}
	/* Each pass settles one state; a few always do. Should flash fail to
	 * take a write, the bound stops a loop. */
	for (int pass = 0; pass < 8; pass++) {
		switch (r.state) {
		case ST_PENDING:
			if (!region_ok(STAGING_OFF, r.new_size, r.new_crc)) {
				r.state = ST_IDLE;
				r.new_size = r.new_crc = 0;
				r.flags |= F_REJECTED;
				boot_save(&r);
				break;
			}
			r.backup_size = r.backup_crc = 0;
			if (region_ok(RUN_OFF, r.run_size, r.run_crc)) {
				copy(RUN_OFF, BACKUP_OFF, r.run_size);
				if (region_ok(BACKUP_OFF, r.run_size, r.run_crc)) {
					r.backup_size = r.run_size;
					r.backup_crc = r.run_crc;
				}
			}
			r.state = ST_INSTALL;
			r.flags = 0;
			boot_save(&r);
			break;
		case ST_INSTALL:
			copy(STAGING_OFF, RUN_OFF, r.new_size);
			if (!region_ok(RUN_OFF, r.new_size, r.new_crc) || !vectors_ok(RUN_OFF)) {
				r.state = fallback(&r);
				r.flags |= F_ROLLED_BACK;
				r.new_size = r.new_crc = 0;
				boot_save(&r);
				break;
			}
			r.run_size = r.new_size;
			r.run_crc = r.new_crc;
			r.new_size = r.new_crc = 0;
			r.state = ST_TRIAL;
			boot_save(&r);
			hal_jump(RUN_OFF);
		case ST_TRIAL:
			/* The chip reset before the app confirmed the update. */
			r.state = fallback(&r);
			r.flags |= F_ROLLED_BACK;
			if (r.state == ST_IDLE) {
				r.flags |= F_NO_FALLBACK;
			}
			boot_save(&r);
			break;
		case ST_RESTORE_BACKUP:
			copy(BACKUP_OFF, RUN_OFF, r.backup_size);
			if (region_ok(RUN_OFF, r.backup_size, r.backup_crc)) {
				r.run_size = r.backup_size;
				r.run_crc = r.backup_crc;
				r.state = ST_IDLE;
			} else {
				r.state = region_ok(GOLDEN_OFF, r.golden_size, r.golden_crc) ? ST_RESTORE_GOLDEN : ST_IDLE;
			}
			boot_save(&r);
			break;
		case ST_RESTORE_GOLDEN:
			copy(GOLDEN_OFF, RUN_OFF, r.golden_size);
			r.run_size = r.golden_size;
			r.run_crc = r.golden_crc;
			r.state = ST_IDLE;
			boot_save(&r);
			break;
		default: /* ST_IDLE */
			run_if_sound(&r);
			r.state = fallback(&r);
			if (r.state == ST_IDLE) {
				if (!(r.flags & F_NO_FALLBACK)) {
					r.flags |= F_NO_FALLBACK;
					boot_save(&r);
				}
				pass = 8;
				break;
			}
			r.flags |= F_RESTORED;
			boot_save(&r);
			break;
		}
	}
	/* Nothing sound: run what RUN holds if it can start at all. */
	if (vectors_ok(RUN_OFF)) {
		hal_jump(RUN_OFF);
	}
	hal_halt();
}
