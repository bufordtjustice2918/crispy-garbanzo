# End-to-End Testing

## Current State

E2E testing is ISO-level: build the live appliance image, boot it in QEMU,
and verify services are running correctly via the serial console.

The test harness is ported from `bufordtjustice2918/clawgress` (mvpv2.2
`scripts/test-iso-commands.py`) and adapted for Debian live-boot, CDROM/GRUB
boot, and a bash shell environment instead of VyOS CLI.

---

## CI Workflow: `build-iso.yml`

### Job 1: `build-livecd`

Runs in a `debian:bookworm` container (privileged).

- Builds the ISO with `live-build` (Debian bookworm, amd64, iso-hybrid)
- Uploads artifact: `clawgress-livecd-iso/clawgress-bookworm-amd64.iso`

### Job 2: `boot-test`

Runs in an `ubuntu:24.04` container (privileged, for KVM access).

#### Step 1 — GRUB/CDROM boot check (`boot-test.sh`)

```bash
./build/iso/scripts/boot-test.sh build/iso/out/clawgress-bookworm-amd64.iso 700
```

Boots the ISO via QEMU CDROM boot exactly as real hardware. Passes when the
live selftest emits `CLAWGRESS_LIVE_SELFTEST_PASS` on the serial console.

#### Step 2 — E2E pexpect command suite (`test-iso-commands.py`)

```bash
python3 build/iso/scripts/test-iso-commands.py \
  --iso build/iso/out/clawgress-bookworm-amd64.iso \
  --suite smoke \
  --boot-timeout 400 \
  --log-dir build/iso/out/e2e-logs
```

Boots a fresh QEMU VM, logs in on the serial console as `clawgress`/`clawgress`,
runs the selected command suite, and writes `summary.json`.

Artifacts uploaded: `boot-test-logs/` (boot-test.log + e2e-logs/).

---

## Command Suites

Defined in `build/iso/scripts/test-iso-commands.py`.

### `smoke` (default, CI)

Fast checks for baseline appliance health:

| Command | What it verifies |
|---------|-----------------|
| `uname -r` | Kernel booted |
| `hostname` | Identity |
| `sudo systemctl is-active nftables` | Firewall service active |
| `sudo systemctl is-active bind9` | DNS service active |
| `sudo systemctl is-active haproxy` | Proxy service active |
| `sudo systemctl is-active ssh` | SSH service active |
| `sudo nft list ruleset` | Firewall rules loaded |
| `ip route` | Routing table populated |
| `dig @127.0.0.1 localhost +short` | DNS resolver responding |
| `sudo haproxy -c -f /etc/haproxy/haproxy.cfg` | HAProxy config valid |
| `grep -q CLAWGRESS_LIVE_SELFTEST_PASS /var/log/clawgress-live-selftest.log` | Selftest passed |
| `test -x /usr/local/sbin/clawgress-install.sh` | Installer present |

### `service-check`

All smoke checks plus detailed service status, named-checkconf, installer
dry-run, and GRUB package presence checks.

### Custom commands file

```bash
python3 build/iso/scripts/test-iso-commands.py \
  --iso clawgress-bookworm-amd64.iso \
  --commands-file my-checks.txt
```

One shell command per line, `#` for comments.

---

## Running Locally

### Prerequisites

```bash
# Debian/Ubuntu
sudo apt-get install qemu-system-x86 qemu-utils python3-pexpect

# Build the ISO first (requires live-build in a debian:bookworm env)
# OR download the artifact from a CI run
```

### Boot check only

```bash
sudo ./build/iso/scripts/boot-test.sh /path/to/clawgress-bookworm-amd64.iso 700
```

### Full pexpect suite

```bash
python3 build/iso/scripts/test-iso-commands.py \
  --iso /path/to/clawgress-bookworm-amd64.iso \
  --suite smoke \
  --log-dir /tmp/clawgress-e2e
```

### Interactive debug on failure

```bash
python3 build/iso/scripts/test-iso-commands.py \
  --iso /path/to/clawgress-bookworm-amd64.iso \
  --suite smoke \
  --debug-on-failure \
  --keep-vm-on-failure
```

`--debug-on-failure` drops you into a live serial console (Ctrl-] to exit).
`--keep-vm-on-failure` leaves the QEMU process running for `screen /dev/pts/N`.

---

## Login Credentials (live ISO)

| Field | Value |
|-------|-------|
| Username | `clawgress` |
| Password | `clawgress` |
| Sudo | passwordless (NOPASSWD) |

Password expiry and root password setup happen in the disk installer
(`clawgress-install.sh`), not in the live environment.

---

## Summary Output

`summary.json` structure:

```json
{
  "iso": "...",
  "suite": "smoke",
  "login": { "success": true, "used_kvm": true },
  "commands": [
    { "command": "uname -r", "rc": 0, "status": "pass", "output_tail": "6.1.0-..." },
    ...
  ],
  "diagnostics": [...],
  "started_at": "...",
  "ended_at": "..."
}
```

Exit code 0 = all commands passed. Exit code 1 = one or more failures (full
JSON still printed to stdout for CI artifact capture).

---

## Future E2E Scope (post-infrastructure MVP)

Once the Clawgress daemon and policy engine exist:

- Policy load and enforcement verification (egress allow/deny by domain)
- RPZ zone reload and DNS query validation
- Per-agent rate limit enforcement under load
- REST API (`/clawgress/policy`, `/clawgress/telemetry`) response validation
- Installer smoke test (install to a second QCOW2 disk, reboot, verify)
