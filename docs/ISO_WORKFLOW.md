# ISO Build Workflow (Ubuntu 24.04 LiveCD)

## Goal
Build a bootable Ubuntu 24.04-based LiveCD ISO that starts as a working egress firewall appliance with:
- `nftables`
- `bind9`
- `haproxy`

The ISO includes a live installer path to install onto HDD/SSD.

## Build Trigger
GitHub Actions workflow:
- `.github/workflows/build-iso.yml`

Triggers:
- Manual dispatch
- Pushes touching ISO workflow/build files

## Build Stack
- `live-build` in Ubuntu mode
- Distribution: `noble` (Ubuntu 24.04)
- Architecture: `amd64`
- Output: hybrid boot ISO with SquashFS root filesystem

Script:
- `build/iso/scripts/build-livecd.sh`
- `build/iso/scripts/boot-test.sh`

## Included Runtime Packages
From `build/iso/live-build/config/package-lists/clawgress.list.chroot`:
- Firewall/network: `nftables`, `iproute2`, `conntrack`
- DNS: `bind9`, `bind9-utils`
- Proxy: `haproxy`
- Live/installer: `casper`, `ubiquity`, `ubiquity-casper`

## Service Behavior on Live Boot
Hook script:
- `build/iso/live-build/config/hooks/live/010-enable-services.hook.chroot`
- `build/iso/live-build/config/hooks/live/020-appliance-login.hook.chroot`

Behavior:
- Enables `nftables`, `haproxy`, and `bind9` services.
- Seeds baseline config if missing.
- Enables `clawgress-live-selftest.service` for boot-time validation.
- Creates default login user `clawgress` with password `clawgress`.
- Installs ANSI MOTD banner shown on login.

## Full ISO Boot Validation
The workflow boots the generated ISO in QEMU (headless, serial console) and requires the live system to emit `CLAWGRESS_LIVE_SELFTEST_PASS`.

Self-test assets:
- `build/iso/live-build/config/includes.chroot/usr/local/sbin/clawgress-live-selftest.sh`
- `build/iso/live-build/config/includes.chroot/etc/systemd/system/clawgress-live-selftest.service`

Validation checks include:
- Live system fully boots from ISO/SquashFS
- `nftables`, `bind9`, and `haproxy` restart and reach active state
- nftables ruleset is loaded

## Firewall Baseline
Injected config:
- `build/iso/live-build/config/includes.chroot/etc/nftables.conf`

Default behavior:
- Input default drop
- Allows DNS ports and selected management/service ports
- Forward default drop
- Output default accept

## Artifacts
Workflow uploads:
- `build/iso/out/clawgress-noble-amd64.iso`
- `build/iso/out/SHA256SUMS`

## Install to Disk
The ISO is a live environment with installer packages present (`ubiquity` + `casper` stack).
For MVP appliance UX, `clawgressctl install` defines the explicit install workflow semantics.
