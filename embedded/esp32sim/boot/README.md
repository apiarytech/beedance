# ESP-IDF second-stage bootloaders

The ESP32-C3 and ESP32-S3 ROMs cannot load a TinyGo image themselves: its
first segment is mapped from flash, which only a bootloader sets up. So on
these chips esp32sim boots TinyGo's app the way ESP-IDF does, through
ESP-IDF's second-stage bootloader and partition table:

| Offset | File |
|---|---|
| `0x0` | `bootloader.bin` |
| `0x8000` | `partition-table.bin` (the default: `nvs`, `phy_init`, `factory` app at `0x10000`) |
| `0x10000` | the app, built with `../tinygo/esp32c3-idf.json` or `esp32s3-idf.json` |

The files are built by [ESP-IDF](https://github.com/espressif/esp-idf)
v5.4 (`espressif/idf:release-v5.4`, ESP-IDF v5.4.4) from its `hello_world`
example with the default configuration, and are kept here so that building
the emulator does not need the 9 GB ESP-IDF image. ESP-IDF is licensed
under the Apache License 2.0.

| File | SHA-256 |
|---|---|
| `esp32c3/bootloader.bin` | `759bfa829596ce6f148bfee82cda066ee2d5a33f1fee47ffca596194fc890e2b` |
| `esp32c3/partition-table.bin` | `7f00b6c042a89b15b0cac534f82ed988caf29278ff5700b0c511eb1b5bb7c820` |
| `esp32s3/bootloader.bin` | `2a90571ffc9765c71f633304744538384c288df7840863846fa12b053ddd01b3` |
| `esp32s3/partition-table.bin` | `7f00b6c042a89b15b0cac534f82ed988caf29278ff5700b0c511eb1b5bb7c820` |

To rebuild them (for example for a newer ESP-IDF), from this directory:

```bash
./build.sh                      # espressif/idf:release-v5.4
IDF_IMAGE=espressif/idf:v5.5 ./build.sh
```

The bootloader checks the app's description (`esp_app_desc_t`) at the start
of its first segment, which the TinyGo linker scripts in `../tinygo` add.
