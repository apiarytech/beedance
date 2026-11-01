#!/bin/sh
# Rebuilds the ESP-IDF bootloader and partition table for the ESP32-C6 (see
# README.md) in the espressif/idf image, into this directory.
set -eu
image="${IDF_IMAGE:-espressif/idf:release-v5.4}"
cd "$(dirname "$0")"
for chip in esp32c6; do
  # The C6's MMU page size follows the flash size; 4 MB gives the 64 KB
  # pages the app is built for.
  defaults=""
  [ "$chip" = esp32c6 ] && defaults="CONFIG_ESPTOOLPY_FLASHSIZE_4MB=y"
  mkdir -p "$chip"
  docker run --rm -v "$PWD/$chip:/out" -e HOME=/tmp "$image" bash -c "
    set -e
    . \$IDF_PATH/export.sh >/dev/null 2>&1
    cp -r \$IDF_PATH/examples/get-started/hello_world /tmp/hw && cd /tmp/hw
    [ -n '$defaults' ] && echo '$defaults' > sdkconfig.defaults
    idf.py set-target $chip >/dev/null
    idf.py bootloader partition-table >/dev/null
    cp build/bootloader/bootloader.bin build/partition_table/partition-table.bin /out/
    echo $chip: \$(idf.py --version)"
done
sha256sum esp32c6/*.bin
