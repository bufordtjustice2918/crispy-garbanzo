#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <iso-path> [timeout-seconds]" >&2
  exit 1
fi

ISO_PATH="$1"
TIMEOUT_SECS="${2:-700}"
LOG_PATH="${LOG_PATH:-build/iso/out/boot-test.log}"
PASS_MARKER="CLAWGRESS_LIVE_SELFTEST_PASS"
FAIL_MARKER="CLAWGRESS_LIVE_SELFTEST_FAIL"

mkdir -p "$(dirname "${LOG_PATH}")"

if [[ ! -f "${ISO_PATH}" ]]; then
  echo "ISO not found: ${ISO_PATH}" >&2
  exit 1
fi

# Prefer KVM hardware acceleration when available.
accel_args=()
if [ -r /dev/kvm ]; then
  accel_args=(-enable-kvm -cpu host)
  echo "KVM available -- using hardware virt (timeout: ${TIMEOUT_SECS}s)"
else
  accel_args=(-machine accel=tcg)
  echo "KVM not available -- using TCG software emulation (timeout: ${TIMEOUT_SECS}s)"
fi

BIOS_LOG="${LOG_PATH%.log}-bios.log"

set +e
timeout "${TIMEOUT_SECS}" qemu-system-x86_64 \
  -m 2048 \
  -smp 2 \
  "${accel_args[@]}" \
  -cdrom "${ISO_PATH}" \
  -boot d \
  -nographic \
  -no-reboot >"${BIOS_LOG}" 2>&1
QEMU_EXIT=$?
set -e

cp "${BIOS_LOG}" "${LOG_PATH}" || true

if grep -q "${PASS_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: PASS"
  exit 0
fi

if grep -q "${FAIL_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: FAIL marker detected" >&2
  tail -n 200 "${LOG_PATH}" >&2 || true
  exit 1
fi

echo "ISO boot self-test: timed out or failed without PASS marker (qemu exit ${QEMU_EXIT})" >&2
tail -n 200 "${LOG_PATH}" >&2 || true
exit 1
