# ESP-IDF second-stage bootloaders

The ESP32-C3, ESP32-S3 and ESP32-C6 ROMs cannot load a TinyGo image
themselves: segments mapped from flash need a bootloader to set up the
mapping. So on these chips esp32sim boots TinyGo's app the way ESP-IDF
does, through ESP-IDF's second-stage bootloader and partition table:

| Offset | File |
|---|---|
| `0x0` | `bootloader.bin` |
| `0x8000` | `partition-table.bin` (the default: `nvs`, `phy_init`, `factory` app at `0x10000`) |
| `0x10000` | the app, built with `../tinygo/<chip>-idf.json` |

The files are built by [ESP-IDF](https://github.com/espressif/esp-idf)
v5.4 (`espressif/idf:release-v5.4`, ESP-IDF v5.4.4) from its `hello_world`
example with the default configuration, except that the C6's is built for
4 MB of flash (`CONFIG_ESPTOOLPY_FLASHSIZE_4MB`): the C6's MMU page size
follows the flash size, and 4 MB gives the 64 KB pages the app is built for
(with 2 MB, 32 KB pages leave the app's read-only data outside the mapped
range). They are kept here so that building the emulator does not need the
9 GB ESP-IDF image. ESP-IDF is licensed under the Apache License 2.0.

| File | SHA-256 |
|---|---|
| `esp32c3/bootloader.bin` | `759bfa829596ce6f148bfee82cda066ee2d5a33f1fee47ffca596194fc890e2b` |
| `esp32s3/bootloader.bin` | `2a90571ffc9765c71f633304744538384c288df7840863846fa12b053ddd01b3` |
| `esp32c6/bootloader.bin` | `ac00991fb84cfe9f9b722904c06c44132e61e4c35da4c90a28d799e33bc65f64` |
| `*/partition-table.bin` | `7f00b6c042a89b15b0cac534f82ed988caf29278ff5700b0c511eb1b5bb7c820` |

The hashes identify the files committed here. A rebuild gives the same
partition tables but different bootloader bytes, since the bootloader holds
its compile time.

To rebuild them (for example for a newer ESP-IDF), from this directory:

```bash
./build.sh                      # espressif/idf:release-v5.4
IDF_IMAGE=espressif/idf:v5.5 ./build.sh
```

The bootloader checks the app's description (`esp_app_desc_t`) at the start
of its segment 0, which the TinyGo linker scripts in `../tinygo` add.
