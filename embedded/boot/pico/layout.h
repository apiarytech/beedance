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
 * The Pico's 2 MB of flash under the bootloader, as offsets from the start of
 * flash (XIP_BASE). Package boot (layout.go) has the same constants, and a
 * test holds them equal.
 *
 * The app always runs from RUN: the RP2040 cannot remap flash, so a new image
 * is copied into RUN rather than run where it lies.
 */
#ifndef BOOT_LAYOUT_H
#define BOOT_LAYOUT_H

#define XIP_BASE     0x10000000u
#define FLASH_SIZE   0x00200000u
#define SECTOR_SIZE  0x00001000u /* the smallest erase */
#define PAGE_SIZE    0x00000100u /* the smallest program */

#define BOOT_OFF     0x00000000u /* boot2 (256 bytes), then the bootloader */
#define BOOT_SIZE    0x00008000u
#define RUN_OFF      0x00008000u /* the app that runs, linked here */
#define REGION_SIZE  0x00070000u /* each of RUN, STAGING, BACKUP and GOLDEN */
#define STATUS_OFF   0x00078000u /* two sectors, each a copy of the status */
#define STATUS_SIZE  0x00002000u
#define STAGING_OFF  0x0007A000u /* a new image, written by the app */
#define BACKUP_OFF   0x000EA000u /* the image before the last update */
#define GOLDEN_OFF   0x0015A000u /* the first image, copied at first boot; never overwritten */
#define DATA_OFF     0x001CA000u /* the app's own: RETAIN variables, program slots */
#define DATA_SIZE    (FLASH_SIZE - DATA_OFF)

#endif
