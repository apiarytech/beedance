/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

/* testapp.c, linked at the run slot as an app under the bootloader is. */

MEMORY
{
    FLASH (rx) : ORIGIN = 0x10008000, LENGTH = 0x70000
    RAM (rwx)  : ORIGIN = 0x20000000, LENGTH = 256K
}

ENTRY(app_reset)

SECTIONS
{
    .text : {
        KEEP(*(.vectors))
        *(.text .text.*)
        KEEP(*(.rodata .rodata.*))
    } > FLASH
    /DISCARD/ : { *(.ARM.exidx*) *(.ARM.attributes) *(.comment) }
}
