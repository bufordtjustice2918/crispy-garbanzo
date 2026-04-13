#!/usr/bin/env bash
# clawgress-install: image-based installer for the Clawgress appliance.
#
# Architecture (VyOS-style):
#   - Part 1: EFI (512M, vfat)
#   - Part 2: live-media (1G, ext4) — holds the squashfs image + kernel + GRUB
#   - Part 3: clawgress-config (rest, ext4) — persisted overlayfs upper dir
#
# live-boot reads the squashfs on every boot and overlays the config partition
# on top (via "/ union" in persistence.conf). The OS image is immutable;
# all persistent state lives in the config partition.
#
# Usage: clawgress-install [--target-disk /dev/sda] [--hostname clawgress] [--apply]

set -euo pipefail

TARGET_DISK=""
HOSTNAME_VAL="clawgress"
APPLY=false
MOUNT_BASE="/mnt/clawgress-install"
VERSION="1.0.0"

usage() {
  cat >&2 <<EOF
Usage: clawgress-install [--target-disk <disk>] [--hostname <name>] [--apply]
  --target-disk  Block device to install to (e.g. /dev/sda).
                 Launches interactive picker if omitted.
  --hostname     Appliance hostname (default: clawgress)
  --apply        Execute install; omit to dry-run and print plan only
EOF
  exit 1
}

pick_disk() {
  echo ""
  echo "Available disks:"
  lsblk -d -o NAME,SIZE,MODEL,TYPE | grep disk || true
  echo ""
  read -r -p "Enter target disk name (e.g. sda): " DISK_NAME
  echo "/dev/${DISK_NAME}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target-disk) TARGET_DISK="$2"; shift 2 ;;
    --hostname)    HOSTNAME_VAL="$2"; shift 2 ;;
    --apply)       APPLY=true; shift ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
done

[[ "$(id -u)" -ne 0 ]] && { echo "error: must run as root" >&2; exit 1; }

if [[ -z "${TARGET_DISK}" ]]; then
  TARGET_DISK=$(pick_disk)
fi

[[ ! -b "${TARGET_DISK}" ]] && { echo "error: ${TARGET_DISK} is not a block device" >&2; exit 1; }

# Locate squashfs on the live medium
SQUASHFS=""
for search_dir in /run/live/medium/live /lib/live/mount/medium/live \
                   /media/cdrom/live /cdrom/live; do
  found=$(find "${search_dir}" -maxdepth 1 \( -name "filesystem.squashfs" -o -name "*.squashfs" \) 2>/dev/null | head -1)
  [[ -n "${found}" ]] && { SQUASHFS="${found}"; break; }
done

if [[ -z "${SQUASHFS}" ]]; then
  echo "error: could not locate squashfs image on live medium" >&2
  echo "  Searched: /run/live/medium/live, /lib/live/mount/medium/live, /media/cdrom/live, /cdrom/live" >&2
  exit 1
fi

LIVE_DIR=$(dirname "${SQUASHFS}")
VMLINUZ=$(find "${LIVE_DIR}" -maxdepth 2 -name "vmlinuz*" 2>/dev/null | head -1 || true)
INITRD=$(find "${LIVE_DIR}" -maxdepth 2 \( -name "initrd.img*" -o -name "initrd*" \) 2>/dev/null | head -1 || true)

# Handle nvme/mmcblk (e.g. nvme0n1 -> nvme0n1p1)
if [[ "${TARGET_DISK}" =~ (nvme|mmcblk) ]]; then
  EFI_PART="${TARGET_DISK}p1"
  LIVE_PART="${TARGET_DISK}p2"
  CONFIG_PART="${TARGET_DISK}p3"
else
  EFI_PART="${TARGET_DISK}1"
  LIVE_PART="${TARGET_DISK}2"
  CONFIG_PART="${TARGET_DISK}3"
fi

SQUASHFS_SIZE=$(du -sh "${SQUASHFS}" | cut -f1)
DISK_SIZE=$(lsblk -d -o SIZE -n "${TARGET_DISK}" | tr -d ' ')

cat <<PLAN

=== Clawgress Image Installer v${VERSION} ===
  Target disk   : ${TARGET_DISK} (${DISK_SIZE})
  Hostname      : ${HOSTNAME_VAL}
  Squashfs      : ${SQUASHFS} (${SQUASHFS_SIZE})
  Kernel        : ${VMLINUZ:-not found}
  Initrd        : ${INITRD:-not found}

  Partition layout:
    ${EFI_PART}    512M   EFI  (vfat)
    ${LIVE_PART}    1G     live-media (ext4) — OS image + GRUB
    ${CONFIG_PART}   rest   clawgress-config (ext4) — persistent /

  Boot model: live-boot squashfs + overlayfs persistence
  Config is writable and survives reboots; OS image is immutable.

PLAN

if [[ "${APPLY}" = false ]]; then
  echo "Dry-run complete. Pass --apply to execute."
  exit 0
fi

if [[ -z "${VMLINUZ}" || -z "${INITRD}" ]]; then
  echo "error: kernel or initrd not found on live medium — cannot install" >&2
  exit 1
fi

echo "WARNING: This will DESTROY all data on ${TARGET_DISK}."
read -r -p "Type 'yes' to continue: " CONFIRM
[[ "${CONFIRM}" != "yes" ]] && { echo "Aborted."; exit 0; }

echo ""
echo "[1/7] Partitioning ${TARGET_DISK} (GPT)..."
sgdisk --zap-all "${TARGET_DISK}"
sgdisk --new=1:0:+512M  --typecode=1:EF00 --change-name=1:"EFI"              "${TARGET_DISK}"
sgdisk --new=2:0:+1G    --typecode=2:8300 --change-name=2:"live-media"       "${TARGET_DISK}"
sgdisk --new=3:0:0      --typecode=3:8300 --change-name=3:"clawgress-config" "${TARGET_DISK}"
partprobe "${TARGET_DISK}"
sleep 2

echo "[2/7] Formatting partitions..."
mkfs.vfat -F32 -n EFI                   "${EFI_PART}"
mkfs.ext4 -L live-media          -F -q  "${LIVE_PART}"
mkfs.ext4 -L clawgress-config    -F -q  "${CONFIG_PART}"

echo "[3/7] Mounting target partitions..."
LIVE_MNT="${MOUNT_BASE}/live"
CFG_MNT="${MOUNT_BASE}/config"
EFI_MNT="${MOUNT_BASE}/efi"
mkdir -p "${LIVE_MNT}" "${CFG_MNT}" "${EFI_MNT}"
mount "${LIVE_PART}"   "${LIVE_MNT}"
mount "${CONFIG_PART}" "${CFG_MNT}"
mount "${EFI_PART}"    "${EFI_MNT}"

echo "[4/7] Copying squashfs image and kernel..."
mkdir -p "${LIVE_MNT}/live"
echo "  Copying squashfs (${SQUASHFS_SIZE})..."
cp --sparse=always "${SQUASHFS}" "${LIVE_MNT}/live/filesystem.squashfs"
cp "${VMLINUZ}"                  "${LIVE_MNT}/live/vmlinuz"
cp "${INITRD}"                   "${LIVE_MNT}/live/initrd.img"

echo "[5/7] Installing GRUB..."
LIVE_LABEL="live-media"
KERNEL_ARGS="boot=live live-media=LABEL=${LIVE_LABEL} persistence persistence-label=clawgress-config quiet console=ttyS0,115200n8"

# GRUB EFI
grub-install \
  --target=x86_64-efi \
  --efi-directory="${EFI_MNT}" \
  --boot-directory="${LIVE_MNT}/boot" \
  --bootloader-id=clawgress \
  --recheck \
  --no-nvram \
  2>/dev/null || echo "  (EFI install skipped — not running under EFI firmware)"

# GRUB BIOS
grub-install \
  --target=i386-pc \
  --boot-directory="${LIVE_MNT}/boot" \
  "${TARGET_DISK}" \
  2>/dev/null || echo "  (BIOS install skipped — i386-pc modules not available)"

mkdir -p "${LIVE_MNT}/boot/grub"
cat > "${LIVE_MNT}/boot/grub/grub.cfg" <<GRUBCFG
# Clawgress GRUB configuration
insmod serial
serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1
terminal_input  serial console
terminal_output serial console

set timeout=5
set default=0

menuentry "Clawgress ${VERSION}" {
    insmod linux
    insmod ext2
    search --label --no-floppy --set=root ${LIVE_LABEL}
    linux  /live/vmlinuz  ${KERNEL_ARGS}
    initrd /live/initrd.img
}
GRUBCFG

echo "[6/7] Seeding persistent config..."
# persistence.conf tells live-boot to overlay entire root from this partition
echo "/ union" > "${CFG_MNT}/persistence.conf"

# Hostname
mkdir -p "${CFG_MNT}/etc"
echo "${HOSTNAME_VAL}" > "${CFG_MNT}/etc/hostname"
cat > "${CFG_MNT}/etc/hosts" <<HOSTS
127.0.0.1   localhost
127.0.1.1   ${HOSTNAME_VAL}
::1         localhost ip6-localhost ip6-loopback
HOSTS

# Network seed — DHCP on eth0/ens3 by default
mkdir -p "${CFG_MNT}/etc/network"
cat > "${CFG_MNT}/etc/network/interfaces" <<NETCFG
# Clawgress network seed — edit to set static addressing
auto lo
iface lo inet loopback

auto eth0
iface eth0 inet dhcp
NETCFG

# Root password — hash 'clawgress', force expiry on first login
if command -v openssl &>/dev/null; then
  ROOT_HASH=$(openssl passwd -6 "clawgress")
  # Write a complete shadow file — live squashfs shadow will be overridden by persistence
  mkdir -p "${CFG_MNT}/etc"
  # Grab the base shadow from the live system, replace root entry
  if [[ -f /etc/shadow ]]; then
    grep -v '^root:' /etc/shadow > "${CFG_MNT}/etc/shadow" || true
  fi
  # root:HASH:0:0:1::: — lastchanged=0 forces password change on first login
  echo "root:${ROOT_HASH}:0:0:99999:7:::" >> "${CFG_MNT}/etc/shadow"
  chmod 640 "${CFG_MNT}/etc/shadow"
  echo "  Root password set to 'clawgress' (change on first login)"
fi

echo "[7/7] Unmounting..."
umount "${EFI_MNT}"
umount "${CFG_MNT}"
umount "${LIVE_MNT}"
rmdir "${EFI_MNT}" "${CFG_MNT}" "${LIVE_MNT}" "${MOUNT_BASE}" 2>/dev/null || true

cat <<DONE

=== Install Complete ===
Clawgress ${VERSION} installed to ${TARGET_DISK}.

  Default credentials: root / clawgress  (change on first login)
  To update the OS: drop a new .squashfs into /live/ and reboot.

Remove the live media and reboot.
DONE
