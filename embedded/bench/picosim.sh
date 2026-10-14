#!/bin/sh
# picosim.sh FW [SCANS]: builds fw/FW for the Pico and runs it on picosim, an
# emulated RP2040 (embedded/picosim), printing its output. Build the image
# first, from the beedance root:
#
#	docker build -f embedded/picosim/Dockerfile -t beedance-picosim .
#
# SCANS (default 50) is how many scans each workload runs; the emulator is
# slower than a board. Environment: OPT (TinyGo -opt, default z), STACK
# (-stack-size; fw/all and fw/eval need 16KB), TIMEOUT (seconds, default 600),
# FULL=1 to print everything, not only the bench lines.
set -e
cd "$(dirname "$0")"
fw=${1:?usage: picosim.sh FW [SCANS]}
scans=${2:-50}
out=$(mktemp -d)
uf2=$out/$fw.uf2
tinygo build -target=pico -serial=uart -opt="${OPT:-z}" ${STACK:+-stack-size=$STACK} \
	-ldflags="-X github.com/apiarytech/beedance/embedded/bench/report.scans=$scans" -o "$uf2" "./fw/$fw"
mount=$uf2
command -v cygpath >/dev/null && mount=$(cygpath -w "$uf2") # Git Bash on Windows
export MSYS_NO_PATHCONV=1
name=bench-picosim-$$
docker run -d --name "$name" -v "$mount:/fw.uf2:ro" beedance-picosim /fw.uf2 >/dev/null
trap 'docker rm -f "$name" >/dev/null 2>&1; rm -rf "$out"' EXIT
end=$(( $(date +%s) + ${TIMEOUT:-600} ))
while [ "$(date +%s)" -lt "$end" ]; do
	docker logs "$name" 2>&1 | grep -qa 'bench: done\|panic\|fatal error' && break
	[ "$(docker inspect -f '{{.State.Running}}' "$name")" = true ] || break
	sleep 2
done
if [ -n "$FULL" ]; then
	docker logs "$name" 2>&1
else
	docker logs "$name" 2>&1 | grep -a 'bench\|panic\|fatal error' || true
fi
