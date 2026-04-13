# ISO Build Workflow (Debian Bookworm LiveCD)

## Goal

Build a bootable Debian Bookworm-based LiveCD ISO that starts as a working
egress gateway appliance with:
- `nftables` (firewall/egress enforcement)
- `bind9` (DNS with RPZ for domain policy)
- `haproxy` (L4/L7 proxy)

The ISO runs live from CDROM/USB and includes a disk installer for permanent
deployment (image-based, VyOS-style).

## Build Trigger

GitHub Actions workflow: `.github/workflows/build-iso.yml`

Triggers:
- Manual dispatch (`workflow_dispatch`)
- Pushes touching ISO workflow/build files

## Build Stack

- `live-build` running inside a `debian:bookworm` container (not Ubuntu host)
- Distribution: `bookworm` (Debian 12)
- Architecture: `amd64`
- Output: hybrid boot ISO (BIOS via syslinux + UEFI via grub-efi)
- Bootloaders: `syslinux grub-efi` (`--binary-images iso-hybrid`)

Script: `build/iso/scripts/build-livecd.sh`

### Why Debian (not Ubuntu)

Ubuntu's `live-build` package uses different mirror conventions (`security.ubuntu.com`,
`bookworm/updates` suite naming) that conflict with Debian bookworm. Running
`live-build` inside a `debian:bookworm` container ensures Debian's tooling
is used for a Debian target — correct mirrors, correct bootloader names,
correct package defaults.

## Included Runtime Packages

From `build/iso/live-build/config/package-lists/clawgress.list.chroot`:

| Package | Purpose |
|---------|---------|
| `nftables`, `iproute2`, `conntrack` | Firewall and network stack |
| `bind9`, `bind9-utils` | DNS with RPZ support |
| `haproxy` | L4/L7 proxy |
| `curl`, `jq`, `ca-certificates`, `openssh-server` | Operational utilities |
| `live-boot`, `live-boot-initramfs-tools` | Debian live session mount |
| `live-config`, `live-config-systemd` | First-boot live configuration |
| `debootstrap`, `gdisk` | Installer dependencies |
| `grub-pc-bin`, `grub-efi-amd64-bin` | GRUB targets used by the installer |

## Size Reduction

A `001-strip-bloat.hook.chroot` hook removes docs, man pages, locales, and
apt caches from the chroot before squashfs is built. Target ISO size: ~400MB.

## Service Behavior on Live Boot

Hook scripts (run during `lb build` chroot stage):

| Hook | Purpose |
|------|---------|
| `001-strip-bloat.hook.chroot` | Strips docs/man/locale/apt cache |
| `010-enable-services.hook.chroot` | Enables nftables, bind9, haproxy, clawgress-live-selftest |
| `020-appliance-login.hook.chroot` | Creates `clawgress/clawgress` operator account with NOPASSWD sudo |
| `030-grub-serial.hook.binary` | Ensures GRUB serial config is applied to the ISO |

## GRUB Configuration

`build/iso/live-build/config/includes.binary/boot/grub/grub.cfg` overrides
the generated GRUB config to add serial console output:

```
insmod serial
serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1
terminal_input  serial console
terminal_output serial console
```

This ensures GRUB output is visible on `ttyS0` for QEMU `-nographic` and
headless hardware serial consoles. The kernel cmdline includes
`console=ttyS0,115200n8` so systemd and getty also output to serial.

## Full ISO Boot Validation (CI)

The `boot-test` CI job runs two steps against the built ISO:

### 1. GRUB/CDROM boot check (`boot-test.sh`)

Boots the ISO via QEMU CDROM boot (`-cdrom iso -boot order=d`), exactly as
real hardware would. Scans serial output for `CLAWGRESS_LIVE_SELFTEST_PASS`.

Self-test assets:
- `build/iso/live-build/config/includes.chroot/usr/local/sbin/clawgress-live-selftest.sh`
- `build/iso/live-build/config/includes.chroot/etc/systemd/system/clawgress-live-selftest.service`

### 2. E2E pexpect command suite (`test-iso-commands.py`)

Uses `pexpect` to:
1. Boot the ISO via QEMU CDROM boot
2. Log in on the serial console (`clawgress` / `clawgress`)
3. Run a structured command suite (service checks, DNS, installer presence)
4. Emit a `summary.json` artifact with per-command pass/fail

Suites: `smoke` (fast, CI default), `service-check` (verbose, for debugging).

## Firewall Baseline

Injected config: `build/iso/live-build/config/includes.chroot/etc/nftables.conf`

Default behavior:
- Input: default drop; allows established, DNS, SSH, HTTPS
- Forward: default drop
- Output: default accept

## Artifacts

Workflow uploads:
- `clawgress-livecd-iso/clawgress-bookworm-amd64.iso`
- `clawgress-livecd-iso/SHA256SUMS`
- `boot-test-logs/boot-test.log`
- `boot-test-logs/e2e-logs/summary.json`

## Image-Based Disk Install

`clawgress-install.sh` implements a VyOS-style image-based installer. Run
from the live environment as root:

```bash
sudo clawgress-install --target-disk /dev/sda --hostname mygw --apply
```

### Installed disk layout

```
Part 1  512M   EFI (vfat)
Part 2  1G     live-media (ext4, label=live-media)
               └── live/filesystem.squashfs  ← immutable OS image
               └── live/vmlinuz + initrd.img
               └── boot/grub/grub.cfg
Part 3  rest   clawgress-config (ext4, label=clawgress-config)
               └── persistence.conf ("/ union")
               └── etc/hostname, etc/network/interfaces, etc/shadow
```

The installed system boots via live-boot: squashfs is mounted read-only,
the config partition is overlaid on top via overlayfs (`/ union`). The OS
image is immutable; all persistent state (config, logs) lives in the config
partition and survives reboots.

To update the OS: drop a new `filesystem.squashfs` onto the live-media
partition and reboot — no reinstall needed.
