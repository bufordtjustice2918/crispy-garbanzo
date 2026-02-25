#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
LB_DIR="${ROOT_DIR}/build/iso/live-build"
OUT_DIR="${ROOT_DIR}/build/iso/out"

DISTRO="${DISTRO:-noble}"
ARCH="${ARCH:-amd64}"
MIRROR="${MIRROR:-http://archive.ubuntu.com/ubuntu/}"
SECURITY_MIRROR="${SECURITY_MIRROR:-http://security.ubuntu.com/ubuntu/}"
ISO_NAME="${ISO_NAME:-clawgress-${DISTRO}-${ARCH}.iso}"

if ! command -v lb >/dev/null 2>&1; then
  echo "live-build (lb) is required. Install package: live-build" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"
cd "${LB_DIR}"

# Start from clean state each run for reproducibility.
lb clean --purge || true

lb config \
  --mode ubuntu \
  --distribution "${DISTRO}" \
  --architectures "${ARCH}" \
  --linux-flavours generic \
  --binary-images iso \
  --bootloader grub2 \
  --bootappend-live "boot=live components console=ttyS0,115200n8" \
  --archive-areas "main restricted universe multiverse" \
  --mirror-bootstrap "${MIRROR}" \
  --mirror-chroot "${MIRROR}" \
  --mirror-binary "${MIRROR}" \
  --mirror-binary-security "${SECURITY_MIRROR}"

# Build Ubuntu LiveCD ISO with SquashFS root.
lb build

SOURCE_ISO=""
if [ -f "live-image-${ARCH}.iso" ]; then
  SOURCE_ISO="live-image-${ARCH}.iso"
elif [ -f "live-image-${ARCH}.hybrid.iso" ]; then
  SOURCE_ISO="live-image-${ARCH}.hybrid.iso"
else
  # Fallback: detect ISO artifact location/name changes across live-build versions.
  SOURCE_ISO="$(find . -maxdepth 4 -type f \( -name '*.iso' -o -name '*.hybrid.iso' \) | sort | tail -n 1 || true)"
fi

if [ -z "${SOURCE_ISO}" ] || [ ! -f "${SOURCE_ISO}" ]; then
  echo "expected ISO not found. looked for live-image-${ARCH}.iso/hybrid and scanned workspace." >&2
  echo "debug listing (top-level):" >&2
  ls -la >&2 || true
  echo "debug listing (*.iso within depth 4):" >&2
  find . -maxdepth 4 -type f | sort >&2 || true
  exit 1
fi

echo "using source ISO artifact: ${SOURCE_ISO}"
cp "${SOURCE_ISO}" "${OUT_DIR}/${ISO_NAME}"

# If invoked via sudo, hand artifacts back to the invoking user so later
# non-root steps (QEMU self-test, checksum, upload) can write/read in OUT_DIR.
if [ -n "${SUDO_UID:-}" ] && [ -n "${SUDO_GID:-}" ]; then
  chown -R "${SUDO_UID}:${SUDO_GID}" "${OUT_DIR}" || true
fi

echo "ISO created: ${OUT_DIR}/${ISO_NAME}"
