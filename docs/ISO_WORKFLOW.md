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

## Included Runtime Packages
From `build/iso/live-build/config/package-lists/clawgress.list.chroot`:
- Firewall/network: `nftables`, `iproute2`, `conntrack`
- DNS: `bind9`, `bind9-utils`
- Proxy: `haproxy`
- Live/installer: `casper`, `ubiquity`, `ubiquity-casper`

## Service Behavior on Live Boot
Hook script:
- `build/iso/live-build/config/hooks/live/010-enable-services.hook.chroot`

Behavior:
- Enables `nftables`, `haproxy`, and `bind9` services.
- Seeds baseline config if missing.

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
