#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
LB_DIR="${ROOT_DIR}/build/iso/live-build"
OUT_DIR="${ROOT_DIR}/build/iso/out"

DISTRO="${DISTRO:-bookworm}"
ARCH="${ARCH:-amd64}"
MIRROR="${MIRROR:-http://deb.debian.org/debian}"
ISO_NAME="${ISO_NAME:-clawgress-${DISTRO}-${ARCH}.iso}"
LB_CACHE_HIT="${LB_CACHE_HIT:-false}"

if ! command -v lb >/dev/null 2>&1; then
  echo "live-build (lb) is required. Install package: live-build" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"
cd "${LB_DIR}"

if [ "${LB_CACHE_HIT}" = "true" ]; then
  echo "lb chroot cache hit -- skipping debootstrap (lb clean without --purge)"
  lb clean || true
else
  echo "lb chroot cache miss -- full clean and debootstrap"
  lb clean --purge || true
fi

lb config \
  --mode debian \
  --distribution "${DISTRO}" \
  --architectures "${ARCH}" \
  --linux-flavours amd64 \
  --binary-images iso \
  --bootloader grub2 \
  --bootappend-live "boot=live components quiet console=ttyS0,115200n8" \
  --archive-areas "main contrib non-free non-free-firmware" \
  --apt-indices false \
  --apt-recommends false \
  --security false \
  --mirror-bootstrap "${MIRROR}" \
  --mirror-chroot "${MIRROR}" \
  --mirror-binary "${MIRROR}"

lb build

SOURCE_ISO=""
if [ -f "live-image-${ARCH}.iso" ]; then
  SOURCE_ISO="live-image-${ARCH}.iso"
elif [ -f "live-image-${ARCH}.hybrid.iso" ]; then
  SOURCE_ISO="live-image-${ARCH}.hybrid.iso"
else
  SOURCE_ISO="$(find . -maxdepth 4 -type f \( -name "*.iso" -o -name "*.hybrid.iso" \) | sort | tail -n 1 || true)"
fi

if [ -z "${SOURCE_ISO}" ] || [ ! -f "${SOURCE_ISO}" ]; then
  echo "expected ISO not found." >&2
  ls -la >&2 || true
  find . -maxdepth 4 -type f | sort >&2 || true
  exit 1
fi

echo "using source ISO artifact: ${SOURCE_ISO}"
cp "${SOURCE_ISO}" "${OUT_DIR}/${ISO_NAME}"

echo "ISO created: ${OUT_DIR}/${ISO_NAME}"
