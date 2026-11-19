/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

/* A TinyGo app under the bootloader (pico-boot.json): linked at the run
 * slot, with no boot2 of its own, as the bootloader's comes first in flash.
 * machine.Flash starts at the status sectors, for package boot, and runs to
 * the end of flash: the app writes the status, the staging slot and its
 * data area (layout.h); the backup and golden slots are the bootloader's. */

MEMORY
{
    FLASH_TEXT (rx) : ORIGIN = 0x10008000, LENGTH = 0x70000
    RAM (rwx)       : ORIGIN = 0x20000000, LENGTH = 256k
}

_stack_size = 2K;

INCLUDE "targets/arm.ld"

__flash_data_start = 0x10078000;
__flash_data_end = 0x10200000;

ASSERT(_sidata + SIZEOF(.data) <= 0x10078000, "the app does not fit the bootloader's run slot (448 KB)")
