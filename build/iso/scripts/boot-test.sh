#!/usr/bin/env bash
# boot-test.sh — UEFI/GRUB CDROM boot test for the Clawgress live ISO.
#
# Boots the ISO via QEMU with OVMF (UEFI firmware) so GRUB EFI is used instead
# of ISOLINUX.  GRUB EFI reads /boot/grub/grub.cfg which routes serial console
# output to ttyS0 — the serial log is then scanned for the live selftest marker.
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

# --- OVMF (UEFI firmware) detection ---
# UEFI boot causes GRUB EFI to run, which reads /boot/grub/grub.cfg from the ISO.
# Our grub.cfg routes output to serial (ttyS0) — required for headless CI boot.
# BIOS boot uses ISOLINUX which has no serial console and hangs headless.
OVMF_CODE=""
for candidate in \
  /usr/share/OVMF/OVMF_CODE_4M.fd \
  /usr/share/OVMF/OVMF_CODE.fd \
  /usr/share/edk2/x64/OVMF_CODE.fd \
  /usr/share/qemu/OVMF_CODE.fd; do
  if [[ -f "${candidate}" ]]; then
    OVMF_CODE="${candidate}"
    break
  fi
done

OVMF_VARS=""
for candidate in \
  /usr/share/OVMF/OVMF_VARS_4M.fd \
  /usr/share/OVMF/OVMF_VARS.fd \
  /usr/share/edk2/x64/OVMF_VARS.fd \
  /usr/share/qemu/OVMF_VARS.fd; do
  if [[ -f "${candidate}" ]]; then
    OVMF_VARS="${candidate}"
    break
  fi
done

# --- KVM detection ---
accel_args=()
if [[ -r /dev/kvm ]]; then
  accel_args=(-enable-kvm -cpu host)
  echo "KVM available — using hardware virt (timeout: ${TIMEOUT_SECS}s)"
else
  echo "KVM not available — using TCG (timeout: ${TIMEOUT_SECS}s)"
fi

# Build QEMU firmware args
fw_args=()
if [[ -n "${OVMF_CODE}" && -n "${OVMF_VARS}" ]]; then
  # UEFI boot: copy VARS to a writable temp file (UEFI needs to store boot state)
  VARS_TMP="$(mktemp /tmp/OVMF_VARS.XXXXXX.fd)"
  cp "${OVMF_VARS}" "${VARS_TMP}"
  fw_args=(
    -machine type=q35
    -drive "if=pflash,format=raw,readonly=on,file=${OVMF_CODE}"
    -drive "if=pflash,format=raw,file=${VARS_TMP}"
  )
  echo "UEFI boot: OVMF=${OVMF_CODE}"
else
  echo "WARNING: OVMF not found — falling back to BIOS boot (ISOLINUX, serial may not work)" >&2
  VARS_TMP=""
fi

echo "Booting ISO via CDROM: ${ISO_PATH}"

# CDROM boot with UEFI firmware.
# -serial stdio: serial console (ttyS0) goes to stdout for log capture.
# -monitor none: no QEMU monitor interleaved on stdio.
set +e
timeout "${TIMEOUT_SECS}" qemu-system-x86_64 \
  -m 2048 \
  -smp 2 \
  "${accel_args[@]}" \
  "${fw_args[@]}" \
  -cdrom "${ISO_PATH}" \
  -boot order=d,menu=off \
  -nographic \
  -serial stdio \
  -monitor none \
  -no-reboot >"${LOG_PATH}" 2>&1
QEMU_EXIT=$?
set -e

# Cleanup temp VARS file
[[ -n "${VARS_TMP:-}" && -f "${VARS_TMP}" ]] && rm -f "${VARS_TMP}"

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
