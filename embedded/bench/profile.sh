#!/bin/sh
# profile.sh FW [SECONDS]: runs fw/FW (profvmcalc or profvmplc) on picosim and
# samples it with gdbprof through the emulator's GDB stub. See picosim.sh for
# the image. TOP sets the rows of each table (default 30).
set -e
cd "$(dirname "$0")"
fw=${1:?usage: profile.sh FW [SECONDS]}
seconds=${2:-60}
out=$(mktemp -d)
tinygo build -target=pico -serial=uart -o "$out/$fw.elf" "./fw/$fw"
tinygo build -target=pico -serial=uart -o "$out/$fw.uf2" "./fw/$fw"
mount=$out/$fw.uf2
elf=$out/$fw.elf
if command -v cygpath >/dev/null; then # Git Bash on Windows
	mount=$(cygpath -w "$mount")
	elf=$(cygpath -w "$elf")
fi
export MSYS_NO_PATHCONV=1
name=bench-profile-$$
docker run -d --name "$name" -p 3333:3333 -v "$mount:/fw.uf2:ro" beedance-picosim /fw.uf2 >/dev/null
trap 'docker rm -f "$name" >/dev/null 2>&1; rm -rf "$out"' EXIT
until docker logs "$name" 2>&1 | grep -q 'profile: start'; do sleep 1; done
go run ./gdbprof -elf "$elf" -d "${seconds}s" -top "${TOP:-30}"
