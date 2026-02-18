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
  --binary-images iso-hybrid \
  --bootloader grub-efi \
  --bootappend-live "boot=live components console=ttyS0,115200n8" \
  --archive-areas "main restricted universe multiverse" \
  --mirror-bootstrap "${MIRROR}" \
  --mirror-chroot "${MIRROR}" \
  --mirror-binary "${MIRROR}" \
  --mirror-binary-security "${SECURITY_MIRROR}"

# Build Ubuntu LiveCD ISO with SquashFS root.
lb build

if [ ! -f live-image-${ARCH}.hybrid.iso ]; then
  echo "expected ISO not found: live-image-${ARCH}.hybrid.iso" >&2
  exit 1
fi

cp "live-image-${ARCH}.hybrid.iso" "${OUT_DIR}/${ISO_NAME}"
echo "ISO created: ${OUT_DIR}/${ISO_NAME}"
