#!/usr/bin/env bash
# boot-test.sh — GRUB/CDROM boot test for the Clawgress live ISO.
#
# Boots the ISO via QEMU exactly as real hardware would: CDROM boot → GRUB →
# live-boot → systemd.  Scans serial output for the live selftest PASS marker.
# For surgical pexpect-based e2e testing run test-iso-commands.py instead.
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

STD_BOOT_MARKERS=(
  "Reached target.*Multi-User"
  "clawgress login:"
  "login:"
)

mkdir -p "$(dirname "${LOG_PATH}")"

if [[ ! -f "${ISO_PATH}" ]]; then
  echo "ISO not found: ${ISO_PATH}" >&2
  exit 1
fi

# --- KVM detection ---
accel_args=()
if [[ -r /dev/kvm ]]; then
  accel_args=(-enable-kvm -cpu host)
  echo "KVM available — using hardware virt (timeout: ${TIMEOUT_SECS}s)"
else
  echo "KVM not available — using TCG (timeout: ${TIMEOUT_SECS}s)"
fi

echo "Booting ISO via CDROM/GRUB: ${ISO_PATH}"

# CDROM boot — GRUB loads from the ISO, exactly like real hardware.
# -serial stdio: serial console (ttyS0) goes to stdout for log capture.
# -monitor none: no QEMU monitor interleaved on stdio.
set +e
timeout "${TIMEOUT_SECS}" qemu-system-x86_64 \
  -m 2048 \
  -smp 2 \
  "${accel_args[@]}" \
  -cdrom "${ISO_PATH}" \
  -boot order=d,menu=off \
  -nographic \
  -serial stdio \
  -monitor none \
  -no-reboot >"${LOG_PATH}" 2>&1
QEMU_EXIT=$?
set -e

LINES=$(wc -l < "${LOG_PATH}" || echo 0)
echo "QEMU exit ${QEMU_EXIT}, log lines: ${LINES}"

if grep -q "${PASS_MARKER}" "${LOG_PATH}"; then
  echo "Boot test: PASS (${PASS_MARKER})"
  exit 0
fi

if grep -q "${FAIL_MARKER}" "${LOG_PATH}"; then
  echo "Boot test: FAIL marker detected" >&2
  tail -n 200 "${LOG_PATH}" >&2 || true
  exit 1
fi

for marker in "${STD_BOOT_MARKERS[@]}"; do
  if grep -qE "${marker}" "${LOG_PATH}"; then
    echo "Boot test: PASS (standard marker: ${marker})"
    exit 0
  fi
done

echo "Boot test: timed out without PASS marker" >&2
echo "--- last 200 lines ---" >&2
tail -n 200 "${LOG_PATH}" >&2 || true
exit 1
