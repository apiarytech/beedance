#!/bin/sh
# Copyright (C) 2026 Franklin D. Amador
#
# This software is dual-licensed under the terms of the GPL v2.0 and
# a commercial license. You may choose to use this software under either
# license.
#
# See the LICENSE files in the project root for full license text.
#
# build.sh builds the Pico bootloader with clang and lld, into OUT (default
# ./out): bootloader.elf, bootloader.bin and bootloader.uf2, to flash once,
# before the app.
#
#   ./build.sh            CC=clang-18 ./build.sh            OUT=/tmp/b ./build.sh
set -eu
cd "$(dirname "$0")"
CC=${CC:-clang}
OBJCOPY=${OBJCOPY:-llvm-objcopy}
OUT=${OUT:-out}
mkdir -p "$OUT"
FLAGS="--target=thumbv6m-none-eabi -mcpu=cortex-m0plus -mthumb -Os -ffreestanding -fno-builtin -nostdlib -Wall -Werror"
$CC $FLAGS -c pico-boot-stage2.S -o "$OUT/boot2.o"
$CC $FLAGS -c core.c -o "$OUT/core.o"
$CC $FLAGS -c rp2040.c -o "$OUT/rp2040.o"
$CC $FLAGS -fuse-ld=lld -Wl,-T,bootloader.ld -Wl,--gc-sections \
	"$OUT/boot2.o" "$OUT/core.o" "$OUT/rp2040.o" -o "$OUT/bootloader.elf"
$OBJCOPY -O binary "$OUT/bootloader.elf" "$OUT/bootloader.bin"
go run mkimage.go "$OUT/bootloader.bin" "$OUT/bootloader.uf2"
echo "$OUT/bootloader.uf2: $(wc -c < "$OUT/bootloader.bin") bytes"
