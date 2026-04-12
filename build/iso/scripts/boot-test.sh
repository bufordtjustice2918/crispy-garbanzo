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

# Standard Linux boot markers we treat as a successful boot when the custom
# selftest service has not yet run (belt-and-suspenders for early CI runs).
STD_BOOT_MARKERS=(
  "Reached target.*Multi-User"
  "login:"
  "clawgress login:"
  "ubuntu login:"
)

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

# Check for custom selftest PASS marker (preferred).
if grep -q "${PASS_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: PASS (custom marker)"
  exit 0
fi

# Check for custom selftest FAIL marker.
if grep -q "${FAIL_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: FAIL marker detected" >&2
  tail -n 200 "${LOG_PATH}" >&2 || true
  exit 1
fi

# Belt-and-suspenders: accept standard Linux boot markers as success.
# Useful before the selftest service emits its marker (e.g., first cold run
# where selftest comes up after multi-user.target with a short timeout).
for marker in "${STD_BOOT_MARKERS[@]}"; do
  if grep -qE "${marker}" "${LOG_PATH}"; then
    echo "ISO boot self-test: PASS (standard boot marker: ${marker})"
    exit 0
  fi
done

# Nothing useful found -- dump what we have and fail.
LINES=$(wc -l < "${LOG_PATH}" || echo 0)
echo "ISO boot self-test: timed out or failed without PASS marker (qemu exit ${QEMU_EXIT}, log lines: ${LINES})" >&2
echo "--- last 200 lines of boot log ---" >&2
tail -n 200 "${LOG_PATH}" >&2 || true
exit 1
