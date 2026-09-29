# Example: Running beedance VM on Raspberry Pi

This document provides instructions on how to compile and run the embedded `beedance` VM example on Raspberry Pi devices. This creates a standalone executable that runs the `beedance` VM with the embedded bytecode.

## Compilation for Raspberry Pi (e.g., Pi Zero, 3, 4)

1.  **Install TinyGo:** Follow the installation instructions on the official TinyGo website: https://tinygo.org/
2.  **Compile the example:** From the project root, run the following command:
    ```sh
    tinygo build -o myapp ./embedded/example/raspberrypi/main.go
    ```

## Compilation for Raspberry Pi Pico (RP2040 microcontroller)

1.  **Install TinyGo:** Follow the installation instructions on the official TinyGo website: https://tinygo.org/
2.  **Compile the example:** From the project root, run the following command:
    ```sh
    tinygo build -target=pico -o program-pico.uf2 ./embedded/example/raspberrypi/main.go
    ```
    Flash to target
    ```sh
    tinygo flash -target=pico ./embedded/example/raspberrypi/main.go
    ```
    Build and copy
    ```sh
    tinygo build -target=pico -o firmware.uf2 ./embedded/example/raspberrypi/main.go
    ```