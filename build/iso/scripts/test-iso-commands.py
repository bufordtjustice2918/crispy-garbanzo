#!/usr/bin/env python3
"""Clawgress ISO e2e command suite.

Boots the Clawgress live ISO via QEMU CDROM boot, logs in on the serial
console using pexpect, runs a command suite, and emits a JSON summary.

Ported from bufordtjustice2918/clawgress scripts/test-iso-commands.py (mvpv2.2).
Adapted for: Debian live-boot, CDROM GRUB boot, clawgress/clawgress login.
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
from pathlib import Path

import pexpect
import pexpect.fdpexpect

# ---------------------------------------------------------------------------
# Built-in command suites
# ---------------------------------------------------------------------------

DEFAULT_SMOKE_COMMANDS = [
    # System identity
    "uname -r",
    "hostname",
    "cat /etc/os-release | grep PRETTY_NAME",

    # Core services — these should be active on the live appliance
    "sudo systemctl is-active nftables",
    "sudo systemctl is-active bind9",
    "sudo systemctl is-active haproxy",
    "sudo systemctl is-active ssh",

    # Firewall ruleset loaded
    "sudo nft list ruleset",

    # Network stack
    "ip route",
    "ip addr show",

    # DNS responds on loopback
    "dig @127.0.0.1 localhost +short +timeout=5",

    # HAProxy binary present and valid config
    "sudo haproxy -c -f /etc/haproxy/haproxy.cfg",

    # Live selftest completed successfully
    "grep -q CLAWGRESS_LIVE_SELFTEST_PASS /var/log/clawgress-live-selftest.log",

    # Installer present and executable
    "test -x /usr/local/sbin/clawgress-install.sh",
    "/usr/local/sbin/clawgress-install.sh --help 2>&1 | grep -qi usage",

    # --- Gateway e2e ---
    # Services running
    "sudo systemctl is-active clawgress-gateway",
    "sudo systemctl is-active clawgress-admin-api",

    # Admin API health check
    "curl -sf http://localhost:8080/healthz | grep -q ok",

    # Anonymous proxy request must be rejected with 407
    "test \"$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://localhost:3128 http://localhost:8080/healthz)\" = 407",

    # Valid API key → request forwarded (non-407; 200 from local healthz)
    "test \"$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://test-agent-001:clawgress-test-key-001@localhost:3128 http://localhost:8080/healthz)\" = 200",

    # Policy-blocked domain → 403 Forbidden
    "test \"$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://test-agent-001:clawgress-test-key-001@localhost:3128 http://blocked.example.invalid/)\" = 403",

    # Audit log exists and has entries
    "test -s /var/log/clawgress/audit.jsonl",

    # Audit log is valid JSONL (every line parses as JSON)
    "sudo jq -c . /var/log/clawgress/audit.jsonl | wc -l | grep -qv '^0$'",

    # Audit log contains at least one deny decision
    "sudo jq -r '.decision' /var/log/clawgress/audit.jsonl | grep -q deny",

    # Audit log contains at least one allow decision
    "sudo jq -r '.decision' /var/log/clawgress/audit.jsonl | grep -q allow",

    # Audit events have required fields
    "sudo jq -e '.request_id and .agent_id and .destination and .decision and .policy_id' /var/log/clawgress/audit.jsonl | grep -q true",

    # --- Admin API CRUD e2e ---
    # List agents — should return at least the seed agent
    "curl -sf http://localhost:8080/v1/agents | jq -e 'length > 0'",

    # Get seed agent by ID
    "curl -sf http://localhost:8080/v1/agents/test-agent-001 | jq -e '.agent_id == \"test-agent-001\"'",

    # Create a new agent via POST
    "curl -sf -X POST http://localhost:8080/v1/agents -H 'Content-Type: application/json' -d '{\"agent_id\":\"e2e-crud-agent\",\"api_key\":\"e2e-crud-key\",\"team_id\":\"e2e-team\",\"project_id\":\"e2e-proj\",\"environment\":\"test\",\"status\":\"active\"}' | jq -e '.agent_id == \"e2e-crud-agent\"'",

    # Verify new agent appears in list
    "curl -sf http://localhost:8080/v1/agents/e2e-crud-agent | jq -e '.api_key == \"e2e-crud-key\"'",

    # Create a policy that allows the new agent to reach localhost
    "curl -sf -X POST http://localhost:8080/v1/policies -H 'Content-Type: application/json' -d '{\"policy_id\":\"e2e-crud-policy\",\"agent_id\":\"e2e-crud-agent\",\"domains\":[\"localhost\",\"127.0.0.1\"],\"action\":\"allow\"}' | jq -e '.policy_id == \"e2e-crud-policy\"'",

    # List policies — should include the new policy
    "curl -sf http://localhost:8080/v1/policies | jq -e '[.[] | select(.policy_id == \"e2e-crud-policy\")] | length == 1'",

    # Get policy by ID
    "curl -sf http://localhost:8080/v1/policies/e2e-crud-policy | jq -e '.action == \"allow\"'",

    # Hot-reload verify: new agent key works through the proxy (SIGHUP was sent)
    # Wait a moment for SIGHUP to propagate
    "sleep 2 && test \"$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://e2e-crud-agent:e2e-crud-key@localhost:3128 http://localhost:8080/healthz)\" = 200",

    # Delete the agent
    "curl -sf -X DELETE http://localhost:8080/v1/agents/e2e-crud-agent | jq -e '.deleted == \"e2e-crud-agent\"'",

    # Verify agent is gone (404)
    "sleep 1 && test \"$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/v1/agents/e2e-crud-agent)\" = 404",

    # Delete the policy
    "curl -sf -X DELETE http://localhost:8080/v1/policies/e2e-crud-policy | jq -e '.deleted == \"e2e-crud-policy\"'",

    # Verify policy is gone (404)
    "sleep 1 && test \"$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/v1/policies/e2e-crud-policy)\" = 404",

    # Hot-reload verify: deleted agent key is rejected (407) after SIGHUP
    "sleep 2 && test \"$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://e2e-crud-agent:e2e-crud-key@localhost:3128 http://localhost:8080/healthz)\" = 407",

    # --- Audit Query API e2e ---
    # Audit query endpoint returns 200 with events array
    "curl -sf http://localhost:8080/v1/audit | jq -e 'type == \"array\"'",

    # Audit query returns events (prior proxy requests generated them)
    "curl -sf http://localhost:8080/v1/audit | jq -e 'length > 0'",

    # Filter by decision=deny returns only deny events
    "curl -sf 'http://localhost:8080/v1/audit?decision=deny' | jq -e 'all(.decision == \"deny\")'",

    # Filter by decision=allow returns only allow events
    "curl -sf 'http://localhost:8080/v1/audit?decision=allow' | jq -e 'all(.decision == \"allow\")'",

    # Filter by agent_id returns only that agent's events
    "curl -sf 'http://localhost:8080/v1/audit?agent_id=test-agent-001' | jq -e 'all(.agent_id == \"test-agent-001\")'",

    # Limit parameter works
    "curl -sf 'http://localhost:8080/v1/audit?limit=2' | jq -e 'length <= 2'",

    # Events have all required fields
    "curl -sf 'http://localhost:8080/v1/audit?limit=1' | jq -e '.[0] | .timestamp and .request_id and .decision and .policy_id'",

    # clawgressctl show audit (table output, non-empty)
    "clawgressctl show audit --limit 5 | grep -q DECISION",

    # clawgressctl show audit --json returns valid JSON array
    "clawgressctl show audit --json --limit 3 | jq -e 'type == \"array\"'",

    # clawgressctl show audit --agent filter
    "clawgressctl show audit --json --agent test-agent-001 | jq -e 'all(.agent_id == \"test-agent-001\")'",

    # --- Quota / Rate Limiter e2e ---
    # No quotas initially — list returns empty array
    "curl -sf http://localhost:8080/v1/quotas | jq -e 'type == \"array\"'",

    # Create a strict 1 RPS quota for test-agent-001 (hard_stop mode)
    "curl -sf -X POST http://localhost:8080/v1/quotas -H 'Content-Type: application/json' -d '{\"agent_id\":\"test-agent-001\",\"rps\":1,\"mode\":\"hard_stop\"}' | jq -e '.agent_id == \"test-agent-001\"'",

    # Verify quota appears in list
    "curl -sf http://localhost:8080/v1/quotas/test-agent-001 | jq -e '.rps == 1'",

    # Wait for SIGHUP to propagate, then burst 5 rapid requests — at least one should be 429
    "sleep 2 && CODES=''; for i in 1 2 3 4 5; do CODES=\"$CODES $(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://test-agent-001:clawgress-test-key-001@localhost:3128 http://localhost:8080/healthz)\"; done; echo \"$CODES\" | grep -q 429",

    # Audit log should now have a quota-exceeded entry
    "sudo jq -r '.policy_id' /var/log/clawgress/audit.jsonl | grep -q quota-exceeded",

    # Delete the quota
    "curl -sf -X DELETE http://localhost:8080/v1/quotas/test-agent-001 | jq -e '.deleted == \"test-agent-001\"'",

    # Verify quota gone (404)
    "sleep 1 && test \"$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/v1/quotas/test-agent-001)\" = 404",

    # After removing quota + SIGHUP, requests flow freely again
    "sleep 2 && test \"$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --proxy http://test-agent-001:clawgress-test-key-001@localhost:3128 http://localhost:8080/healthz)\" = 200",
]

DEFAULT_SERVICE_CHECK_COMMANDS = DEFAULT_SMOKE_COMMANDS + [
    # Detailed service status
    "sudo systemctl status bind9 --no-pager -l",
    "sudo systemctl status haproxy --no-pager -l",
    "sudo systemctl status nftables --no-pager -l",
    "sudo systemctl status ssh --no-pager -l",

    # Named (bind9) config check
    "sudo named-checkconf",

    # nftables ruleset
    "sudo nft list tables",

    # Live selftest log
    "cat /var/log/clawgress-live-selftest.log",

    # GRUB packages present (needed by installer)
    "dpkg -l grub-pc-bin | grep -q ^ii",
    "dpkg -l grub-efi-amd64-bin | grep -q ^ii",

    # Installer dry-run (no --apply so nothing writes)
    "sudo /usr/local/sbin/clawgress-install.sh --target-disk /dev/sda 2>&1 | grep -q 'Dry-run'",
]

# Patterns that mark a command output as failed regardless of exit code.
DEFAULT_FAIL_PATTERNS = [
    r"(?im)\bcommand not found\b",
    r"(?im)\bNo such file or directory\b",
    r"(?im)^bash:.*not found",
]

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

QEMU_LOCK_FILE = "/tmp/clawgress-qemu.lock"
QEMU_PROCESS_PATTERN = r"qemu-system-.*clawgress-(cmd-suite|smoke-test)"

PROMPT_RE = r"(?m)(?:^[^\r\n]*\$\s*$|^[^\r\n]*#\s*$)"
LOGIN_RE = r"(?i)login:\s*$"
PASSWORD_RE = r"(?i)password:\s*$"
KVM_FAIL_RE = r"failed to initialize kvm|Could not access KVM kernel module"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def log(msg: str) -> None:
    print(f"[test-iso-commands] {msg}", flush=True)


def fail(msg: str, code: int = 1) -> None:
    print(f"[test-iso-commands] ERROR: {msg}", file=sys.stderr)
    raise SystemExit(code)


_OVMF_CODE_CANDIDATES = [
    "/usr/share/OVMF/OVMF_CODE_4M.fd",
    "/usr/share/OVMF/OVMF_CODE.fd",
    "/usr/share/edk2/x64/OVMF_CODE.fd",
    "/usr/share/qemu/OVMF_CODE.fd",
]
_OVMF_VARS_CANDIDATES = [
    "/usr/share/OVMF/OVMF_VARS_4M.fd",
    "/usr/share/OVMF/OVMF_VARS.fd",
    "/usr/share/edk2/x64/OVMF_VARS.fd",
    "/usr/share/qemu/OVMF_VARS.fd",
]


def _find_ovmf() -> tuple[str, str]:
    """Return (OVMF_CODE, OVMF_VARS) paths, or ('', '') if not found."""
    code = next((p for p in _OVMF_CODE_CANDIDATES if os.path.isfile(p)), "")
    vars_ = next((p for p in _OVMF_VARS_CANDIDATES if os.path.isfile(p)), "")
    return code, vars_


def qemu_cmd(
    iso: str,
    disk: str,
    ram_mb: int,
    cpus: int,
    use_kvm: bool,
    serial_pty: bool = False,
    ovmf_vars_tmp: str = "",
) -> list[str]:
    """Build QEMU UEFI/CDROM-boot command.  Serial output goes to stdio (or PTY).

    Uses OVMF (UEFI) when available so GRUB EFI runs instead of ISOLINUX.
    GRUB EFI reads /boot/grub/grub.cfg which routes serial to ttyS0 — required
    for headless CI boot.  BIOS/ISOLINUX has no serial console and hangs headless.
    """
    cmd = [
        "qemu-system-x86_64",
        "-name", "clawgress-cmd-suite",
        "-m", str(ram_mb),
        "-smp", str(cpus),
    ]

    # UEFI firmware — prefer OVMF so GRUB EFI runs (serial console works).
    ovmf_code, ovmf_vars = _find_ovmf()
    if ovmf_code and ovmf_vars:
        cmd.extend([
            "-machine", "type=q35",
            "-drive", f"if=pflash,format=raw,readonly=on,file={ovmf_code}",
            "-drive", f"if=pflash,format=raw,file={ovmf_vars_tmp or ovmf_vars}",
        ])
    # else: fall back to default BIOS (ISOLINUX) — serial may not work

    cmd.extend([
        # CDROM boot — boots GRUB EFI from the ISO, exactly like real hardware.
        "-cdrom", iso,
        "-boot", "order=d,menu=off",
        # Optional writable disk so the installer can be tested later.
        "-drive", f"file={disk},format=qcow2,if=virtio",
        # User-mode networking (outbound only, no host routes needed in CI).
        "-netdev", "user,id=net0",
        "-device", "virtio-net-pci,netdev=net0",
        "-monitor", "none",
        "-display", "none",
    ])
    if serial_pty:
        cmd.extend(["-serial", "pty"])
    else:
        cmd.extend(["-nographic", "-serial", "stdio"])
    if use_kvm:
        cmd.extend(["-enable-kvm", "-cpu", "host"])
    return cmd


def load_commands(commands_file: str | None, suite: str) -> list[str]:
    if commands_file:
        lines = []
        with open(commands_file, "r", encoding="utf-8") as fh:
            for raw in fh:
                line = raw.strip()
                if not line or line.startswith("#"):
                    continue
                lines.append(line)
        if not lines:
            fail(f"commands file has no runnable commands: {commands_file}")
        return lines
    if suite == "service-check":
        return list(DEFAULT_SERVICE_CHECK_COMMANDS)
    return list(DEFAULT_SMOKE_COMMANDS)


# ---------------------------------------------------------------------------
# Main runner
# ---------------------------------------------------------------------------

def run_suite(args: argparse.Namespace) -> int:
    if not os.path.isfile(args.iso):
        fail(f"ISO not found: {args.iso}")
    for cmd in ("qemu-system-x86_64", "qemu-img"):
        if shutil.which(cmd) is None:
            fail(f"Missing required command: {cmd}")

    commands = load_commands(args.commands_file, args.suite)
    fail_patterns = list(DEFAULT_FAIL_PATTERNS) if not args.no_default_fail_patterns else []
    fail_patterns.extend(args.fail_on_pattern or [])

    # Exclusive lock — prevent concurrent QEMU runs in the same CI job.
    lock_handle = open(QEMU_LOCK_FILE, "w", encoding="utf-8")
    try:
        import fcntl
        fcntl.flock(lock_handle.fileno(), fcntl.LOCK_EX)
    except Exception:
        log("Could not acquire file lock; continuing without lock protection")

    # Kill any stale clawgress QEMU processes.
    stale = subprocess.run(
        ["pgrep", "-f", QEMU_PROCESS_PATTERN], capture_output=True, text=True, check=False
    )
    stale_pids = stale.stdout.split()
    if stale_pids:
        log(f"Killing stale QEMU process(es): {' '.join(stale_pids)}")
        subprocess.run(["kill", *stale_pids], check=False)
        time.sleep(2)

    if args.log_dir:
        workdir = Path(args.log_dir)
        workdir.mkdir(parents=True, exist_ok=True)
    else:
        workdir = Path(tempfile.mkdtemp(prefix="clawgress-cmdsuite."))

    disk_file = workdir / "test-disk.qcow2"
    transcript_path = workdir / "serial-session.log"
    summary_path = workdir / "summary.json"

    log(f"ISO:     {args.iso}")
    log(f"Workdir: {workdir}")
    log(f"Suite:   {args.suite} ({len(commands)} commands)")
    log(f"Creating ephemeral disk ({args.disk_size})")
    subprocess.run(
        ["qemu-img", "create", "-f", "qcow2", str(disk_file), args.disk_size],
        check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )

    kvm_accessible = os.path.exists("/dev/kvm") and os.access("/dev/kvm", os.R_OK | os.W_OK)
    if args.force_kvm and not os.path.exists("/dev/kvm"):
        fail("KVM forced but /dev/kvm does not exist")

    if args.force_kvm:
        attempt_order = [True]
    elif args.use_kvm and kvm_accessible:
        attempt_order = [True, False]
    elif args.use_kvm:
        log("KVM not accessible; using software emulation")
        attempt_order = [False]
    else:
        attempt_order = [False]

    summary = {
        "iso": args.iso,
        "suite": args.suite,
        "workdir": str(workdir),
        "login": {"success": False, "used_kvm": False, "fallback_to_software": False},
        "commands": [],
        "diagnostics": [],
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "ended_at": None,
    }

    child = None
    transcript = None
    qemu_process = None
    qemu_boot_log = None
    serial_fd = None
    serial_pty_path = None
    keep_vm_alive = False

    # Create a writable copy of OVMF VARS so UEFI can store boot state.
    ovmf_vars_tmp = ""
    _, ovmf_vars_src = _find_ovmf()
    if ovmf_vars_src:
        import tempfile as _tf
        _tmp = _tf.NamedTemporaryFile(prefix="OVMF_VARS.", suffix=".fd", delete=False)
        _tmp.close()
        ovmf_vars_tmp = _tmp.name
        import shutil as _sh
        _sh.copy2(ovmf_vars_src, ovmf_vars_tmp)
        log(f"UEFI boot: OVMF VARS tmp={ovmf_vars_tmp}")

    try:
        for attempt_kvm in attempt_order:
            cmd = qemu_cmd(
                args.iso, str(disk_file), args.ram_mb, args.cpus,
                attempt_kvm, serial_pty=args.serial_pty,
                ovmf_vars_tmp=ovmf_vars_tmp,
            )
            log("Starting VM via CDROM boot with " + ("KVM" if attempt_kvm else "software emulation"))

            transcript = open(transcript_path, "w", encoding="utf-8")

            if args.serial_pty:
                boot_log_path = workdir / "qemu-boot.log"
                qemu_boot_log = open(boot_log_path, "w", encoding="utf-8")
                qemu_process = subprocess.Popen(
                    cmd, stdin=subprocess.DEVNULL,
                    stdout=qemu_boot_log, stderr=subprocess.STDOUT,
                    text=True, start_new_session=True,
                )
                pty_re = re.compile(r"/dev/pts/\d+")
                serial_pty_path = None
                for _ in range(60):
                    if qemu_process.poll() is not None:
                        break
                    try:
                        text = boot_log_path.read_text(encoding="utf-8", errors="ignore")
                        m = pty_re.search(text)
                        if m:
                            serial_pty_path = m.group(0)
                            break
                    except Exception:
                        pass
                    time.sleep(0.2)
                if not serial_pty_path:
                    _cleanup_attempt(child, qemu_process, serial_fd, transcript, qemu_boot_log)
                    child = transcript = qemu_process = qemu_boot_log = None
                    serial_fd = serial_pty_path = None
                    continue
                serial_fd = os.open(serial_pty_path, os.O_RDWR | os.O_NOCTTY)
                child = pexpect.fdpexpect.fdspawn(
                    serial_fd, encoding="utf-8", codec_errors="ignore", timeout=args.boot_timeout
                )
            else:
                child = pexpect.spawn(cmd[0], cmd[1:], encoding="utf-8", codec_errors="ignore", timeout=args.boot_timeout)

            child.logfile = transcript

            try:
                child.expect(LOGIN_RE)
                summary["login"]["success"] = True
                summary["login"]["used_kvm"] = attempt_kvm
                summary["login"]["fallback_to_software"] = (not attempt_kvm and attempt_order[0])
                summary["login"]["serial_pty"] = serial_pty_path
                break
            except pexpect.TIMEOUT:
                if attempt_kvm and not args.force_kvm:
                    log("Boot timed out with KVM; retrying without KVM")
                    _cleanup_attempt(child, qemu_process, serial_fd, transcript, qemu_boot_log)
                    child = transcript = qemu_process = qemu_boot_log = None
                    serial_fd = serial_pty_path = None
                    continue
                fail(f"Timed out waiting for login prompt after {args.boot_timeout}s")
            except pexpect.EOF:
                output = child.before or ""
                if attempt_kvm and re.search(KVM_FAIL_RE, output, re.IGNORECASE):
                    if args.force_kvm:
                        fail("KVM forced but QEMU failed to initialize KVM")
                    log("KVM failed; retrying without KVM")
                    _cleanup_attempt(child, qemu_process, serial_fd, transcript, qemu_boot_log)
                    child = transcript = qemu_process = qemu_boot_log = None
                    serial_fd = serial_pty_path = None
                    continue
                fail("VM exited before login prompt appeared")

        if child is None or not summary["login"]["success"]:
            fail("Unable to boot VM to login prompt")

        # --- Login ---
        log(f"Login prompt — authenticating as {args.username!r}")
        child.sendline(args.username)
        child.expect(PASSWORD_RE, timeout=args.cmd_timeout)
        child.sendline(args.password)
        child.expect(PROMPT_RE, timeout=args.cmd_timeout)
        log("Logged in — shell prompt detected")

        # --- Diagnostic helpers ---
        def run_diag(diag_cmd: str) -> dict:
            entry = {"command": diag_cmd, "status": "pass", "output_tail": ""}
            try:
                child.sendline(diag_cmd)
                child.expect(PROMPT_RE, timeout=60)
                out = child.before or ""
                entry["output_tail"] = "\n".join(out.strip().splitlines()[-80:])
                for pat in fail_patterns:
                    if re.search(pat, out):
                        entry["status"] = "fail"
                        break
            except Exception as exc:
                entry["status"] = "fail"
                entry["output_tail"] = f"diagnostic collection failed: {exc}"
            return entry

        def collect_diagnostics(for_cmd: str) -> None:
            diag_cmds = [
                "sudo systemctl status bind9 haproxy nftables ssh --no-pager -l",
                "sudo journalctl -xeu bind9 --no-pager -n 100",
                "sudo journalctl -xeu haproxy --no-pager -n 50",
                "cat /var/log/clawgress-live-selftest.log 2>/dev/null || true",
                "sudo dmesg | tail -50",
            ]
            for dc in diag_cmds:
                entry = run_diag(dc)
                entry["for_command"] = for_cmd
                entry["diagnostic_type"] = "failure_diag"
                summary["diagnostics"].append(entry)

        # --- Run command suite ---
        stop_after_failure = False
        for cmd_text in commands:
            if stop_after_failure:
                break
            log(f"Running: {cmd_text}")
            marker = f"__RC_{uuid.uuid4().hex}__"
            # Wrap command so exit code is captured via the unique marker.
            wrapped = f"if {cmd_text}; then echo {marker}0; else echo {marker}1; fi"
            child.sendline(wrapped)
            try:
                child.expect(PROMPT_RE, timeout=args.cmd_timeout)
                timed_out = False
            except pexpect.TIMEOUT:
                timed_out = True

            output = child.before or ""
            if timed_out:
                summary["commands"].append({
                    "command": cmd_text, "rc": 124, "status": "fail",
                    "matched_fail_patterns": ["timeout"],
                    "output_tail": "\n".join(output.strip().splitlines()[-30:]),
                })
                collect_diagnostics(cmd_text)
                stop_after_failure = True
                continue

            rc_matches = re.findall(rf"{re.escape(marker)}(\d+)", output)
            rc = int(rc_matches[-1]) if rc_matches else 999

            matched_patterns = [p for p in fail_patterns if re.search(p, output)]
            if matched_patterns:
                rc = 1

            status = "pass" if rc == 0 else "fail"
            summary["commands"].append({
                "command": cmd_text, "rc": rc, "status": status,
                "matched_fail_patterns": matched_patterns,
                "output_tail": "\n".join(output.strip().splitlines()[-30:]),
            })
            if args.diag_on_failure and status == "fail":
                collect_diagnostics(cmd_text)

        # Clean shutdown
        child.sendline("exit")
        try:
            child.expect(pexpect.EOF, timeout=10)
        except Exception:
            pass

    finally:
        had_failures = (not summary["login"]["success"]) or any(
            c.get("status") != "pass" for c in summary.get("commands", [])
        )
        vm_running = (
            (qemu_process is not None and qemu_process.poll() is None)
            or (child is not None and child.isalive())
        )
        keep_vm_alive = bool(args.keep_vm_on_failure and had_failures and vm_running)

        if args.debug_on_failure and had_failures and child is not None and child.isalive():
            log("FAILURE — entering interactive serial console (Ctrl-] to exit)")
            try:
                child.interact(escape_character=chr(29))
            except Exception:
                pass
            vm_running = (
                (qemu_process is not None and qemu_process.poll() is None)
                or (child is not None and child.isalive())
            )
            keep_vm_alive = bool(args.keep_vm_on_failure and vm_running)

        try:
            lock_handle.close()
        except Exception:
            pass

        if not keep_vm_alive:
            if child is not None and child.isalive():
                child.close(force=True)
            if qemu_process is not None and qemu_process.poll() is None:
                qemu_process.terminate()
                try:
                    qemu_process.wait(timeout=5)
                except Exception:
                    qemu_process.kill()
        if serial_fd is not None:
            try:
                os.close(serial_fd)
            except Exception:
                pass
        for fh in (transcript, qemu_boot_log):
            if fh is not None and not fh.closed:
                fh.close()

        # Clean up OVMF VARS temp file.
        if ovmf_vars_tmp and os.path.exists(ovmf_vars_tmp):
            try:
                os.unlink(ovmf_vars_tmp)
            except Exception:
                pass

        summary["ended_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        summary_path.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")

        if keep_vm_alive:
            vm_pid = (
                qemu_process.pid if qemu_process and qemu_process.poll() is None
                else (child.pid if child and child.isalive() else None)
            )
            if vm_pid:
                log(f"Leaving VM alive for debugging (PID {vm_pid})")
            if serial_pty_path:
                log(f"Attach: screen {serial_pty_path} 115200")

    failures = [c for c in summary["commands"] if c["status"] != "pass"]
    if not summary["login"]["success"] or failures:
        log(f"FAILED — {len(failures)} command(s) failed")
        print(json.dumps(summary, indent=2))
        return 1

    log(f"All {len(commands)} command(s) passed")
    print(json.dumps(summary, indent=2))
    return 0


def _cleanup_attempt(child, qemu_process, serial_fd, transcript, qemu_boot_log) -> None:
    if child is not None:
        try:
            child.close(force=True)
        except Exception:
            pass
    if qemu_process is not None and qemu_process.poll() is None:
        qemu_process.terminate()
        try:
            qemu_process.wait(timeout=5)
        except Exception:
            qemu_process.kill()
    if serial_fd is not None:
        try:
            os.close(serial_fd)
        except Exception:
            pass
    for fh in (transcript, qemu_boot_log):
        if fh is not None and not fh.closed:
            fh.close()


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Clawgress ISO e2e command suite — boots via QEMU CDROM, "
                    "logs in on serial, runs commands, emits JSON summary."
    )
    p.add_argument("--iso", required=True, help="Path to Clawgress live ISO")
    p.add_argument("--username", default="clawgress", help="Login username (default: clawgress)")
    p.add_argument("--password", default="clawgress", help="Login password (default: clawgress)")
    p.add_argument("--boot-timeout", type=int, default=400,
                   help="Seconds to wait for login prompt (default: 400)")
    p.add_argument("--cmd-timeout", type=int, default=60,
                   help="Seconds to wait per command (default: 60)")
    p.add_argument("--ram-mb", type=int, default=2048, help="VM RAM in MB (default: 2048)")
    p.add_argument("--cpus", type=int, default=2, help="VM CPU count (default: 2)")
    p.add_argument("--disk-size", default="4G", help="Ephemeral disk size (default: 4G)")
    p.add_argument("--commands-file",
                   help="Newline-delimited shell commands to run (overrides --suite)")
    p.add_argument("--suite", choices=["smoke", "service-check"], default="smoke",
                   help="Built-in command suite (default: smoke)")
    p.add_argument("--fail-on-pattern", action="append",
                   help="Regex that marks a command failed if matched in output (repeatable)")
    p.add_argument("--no-default-fail-patterns", action="store_true",
                   help="Disable built-in fail patterns")
    p.add_argument("--log-dir", help="Directory for logs and artifacts")
    p.add_argument("--diag-on-failure", action="store_true", default=True,
                   help="Collect diagnostics after each failure (default: on)")
    p.add_argument("--keep-vm-on-failure", action="store_true",
                   help="Leave QEMU running on failure for manual debugging")
    p.add_argument("--debug-on-failure", action="store_true",
                   help="Drop into interactive serial console on failure")
    p.add_argument("--serial-pty", action="store_true",
                   help="Use PTY serial backend (allows post-run reattachment)")
    p.add_argument("--no-kvm", dest="use_kvm", action="store_false",
                   help="Disable KVM acceleration")
    p.add_argument("--force-kvm", action="store_true",
                   help="Require KVM; fail if unavailable")
    p.set_defaults(use_kvm=True)
    return p.parse_args()


if __name__ == "__main__":
    sys.exit(run_suite(parse_args()))
