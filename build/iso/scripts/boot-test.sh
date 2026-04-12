#!/usr/bin/env bash
# Boot-test: mount ISO, extract kernel+initrd, boot directly via QEMU -kernel.
# This bypasses GRUB entirely — no grub.cfg path issues, serial always works.
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
  "login:"
  "clawgress login:"
  "ubuntu login:"
)

mkdir -p "$(dirname "${LOG_PATH}")"

if [[ ! -f "${ISO_PATH}" ]]; then
  echo "ISO not found: ${ISO_PATH}" >&2
  exit 1
fi

# --- Mount ISO and locate kernel + initrd ---
MNT=$(mktemp -d)
cleanup() { umount "${MNT}" 2>/dev/null || true; rm -rf "${MNT}"; }
trap cleanup EXIT

echo "Mounting ISO to inspect kernel paths..."
mount -o loop,ro "${ISO_PATH}" "${MNT}"

VMLINUZ=$(find "${MNT}" \( -name "vmlinuz" -o -name "vmlinuz-*" \) -not -name "*.efi" | head -1)
INITRD=$(find "${MNT}" \( -name "initrd.img" -o -name "initrd.img-*" -o -name "initrd" \) | head -1)

if [[ -z "${VMLINUZ}" ]]; then
  echo "ERROR: no vmlinuz found in ISO" >&2
  find "${MNT}" -name "vmlinuz*" >&2 || true
  exit 1
fi
if [[ -z "${INITRD}" ]]; then
  echo "ERROR: no initrd found in ISO" >&2
  find "${MNT}" -name "initrd*" >&2 || true
  exit 1
fi

echo "Kernel:  ${VMLINUZ}"
echo "Initrd:  ${INITRD}"

# --- KVM detection ---
accel_args=()
if [ -r /dev/kvm ]; then
  accel_args=(-enable-kvm -cpu host)
  echo "KVM available -- using hardware virt (timeout: ${TIMEOUT_SECS}s)"
else
  accel_args=(-machine accel=tcg)
  echo "KVM not available -- using TCG (timeout: ${TIMEOUT_SECS}s)"
fi

BIOS_LOG="${LOG_PATH%.log}-bios.log"

# Boot directly: bypass GRUB, force serial console via kernel cmdline.
set +e
timeout "${TIMEOUT_SECS}" qemu-system-x86_64 \
  -m 2048 \
  -smp 2 \
  "${accel_args[@]}" \
  -kernel "${VMLINUZ}" \
  -initrd "${INITRD}" \
  -append "boot=live components quiet console=ttyS0,115200n8 ---" \
  -drive file="${ISO_PATH},format=raw,media=cdrom,readonly=on" \
  -nographic \
  -no-reboot >"${BIOS_LOG}" 2>&1
QEMU_EXIT=$?
set -e

cp "${BIOS_LOG}" "${LOG_PATH}" || true

LINES=$(wc -l < "${LOG_PATH}" || echo 0)
echo "QEMU exit ${QEMU_EXIT}, log lines: ${LINES}"

if grep -q "${PASS_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: PASS (custom marker)"
  exit 0
fi

if grep -q "${FAIL_MARKER}" "${LOG_PATH}"; then
  echo "ISO boot self-test: FAIL marker detected" >&2
  tail -n 200 "${LOG_PATH}" >&2 || true
  exit 1
fi

for marker in "${STD_BOOT_MARKERS[@]}"; do
  if grep -qE "${marker}" "${LOG_PATH}"; then
    echo "ISO boot self-test: PASS (standard boot marker: ${marker})"
    exit 0
  fi
done

echo "ISO boot self-test: timed out without PASS marker" >&2
echo "--- last 200 lines ---" >&2
tail -n 200 "${LOG_PATH}" >&2 || true
exit 1
