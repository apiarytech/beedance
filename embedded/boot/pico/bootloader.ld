/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

/* The bootloader: boot2 in the first 256 bytes of flash, the vector table and
 * reset() after it, the rest copied by reset() into RAM and run there. */

MEMORY
{
    BOOT2 (rx) : ORIGIN = 0x10000000, LENGTH = 256
    FLASH (rx) : ORIGIN = 0x10000100, LENGTH = 0x8000 - 256
    RAM (rwx)  : ORIGIN = 0x20000000, LENGTH = 256K
}

ENTRY(reset)

SECTIONS
{
    /* 256 bytes, the last 4 the CRC the bootrom checks (mkimage.go). */
    .boot2 : {
        __boot2_start__ = .;
        KEEP(*(.boot2));
        . = __boot2_start__ + 256 - 4;
        LONG(0)
    } > BOOT2 = 0x0

    .vectors : { KEEP(*(.vectors)) } > FLASH
    .entry : { *(.entry) } > FLASH

    .text : {
        . = ALIGN(4);
        _stext = .;
        *(.text .text.*)
        *(.rodata .rodata.*)
        *(.data .data.*)
        . = ALIGN(4);
        _etext = .;
    } > RAM AT > FLASH
    _ltext = LOADADDR(.text);

    .bss (NOLOAD) : {
        . = ALIGN(4);
        _sbss = .;
        *(.bss .bss.*)
        *(COMMON)
        . = ALIGN(4);
        _ebss = .;
    } > RAM

    _stack_top = ORIGIN(RAM) + LENGTH(RAM);

    /DISCARD/ : { *(.ARM.exidx*) *(.ARM.attributes) *(.comment) }
}

ASSERT(LOADADDR(.text) + SIZEOF(.text) <= 0x10008000, "the bootloader does not fit its 32 KB")
