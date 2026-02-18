#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <iso-path> [timeout-seconds]" >&2
  exit 1
fi

ISO_PATH="$1"
TIMEOUT_SECS="${2:-900}"
LOG_PATH="${LOG_PATH:-build/iso/out/boot-test.log}"
PASS_MARKER="CLAWGRESS_LIVE_SELFTEST_PASS"
FAIL_MARKER="CLAWGRESS_LIVE_SELFTEST_FAIL"

mkdir -p "$(dirname "${LOG_PATH}")"

if [[ ! -f "${ISO_PATH}" ]]; then
  echo "ISO not found: ${ISO_PATH}" >&2
  exit 1
fi

set +e
timeout "${TIMEOUT_SECS}" qemu-system-x86_64 \
  -m 4096 \
  -smp 2 \
  -machine accel=tcg \
  -cdrom "${ISO_PATH}" \
  -boot d \
  -serial mon:stdio \
  -display none \
  -no-reboot >"${LOG_PATH}" 2>&1
QEMU_EXIT=$?
set -e

if grep -q "${PASS_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: PASS"
  exit 0
fi

if grep -q "${FAIL_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: FAIL marker detected" >&2
  tail -n 200 "${LOG_PATH}" >&2 || true
  exit 1
fi

if [[ ${QEMU_EXIT} -eq 124 ]]; then
  echo "ISO boot self-test: timed out waiting for PASS marker" >&2
else
  echo "ISO boot self-test: qemu exited with ${QEMU_EXIT} without PASS marker" >&2
fi

tail -n 200 "${LOG_PATH}" >&2 || true
exit 1
