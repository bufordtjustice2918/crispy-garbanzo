#!/usr/bin/env bash
# Clawgress live ISO self-test.  Runs once after multi-user.target.
# Emits PASS/FAIL markers to serial console for CI detection.
set -euo pipefail

MARKER_PASS="CLAWGRESS_LIVE_SELFTEST_PASS"
MARKER_FAIL="CLAWGRESS_LIVE_SELFTEST_FAIL"
LOG_FILE="/var/log/clawgress-live-selftest.log"

emit() {
  local msg="$1"
  echo "[$(date -Is)] ${msg}" | tee -a "${LOG_FILE}"
  # Write directly to serial + console so CI captures it even if journal
  # buffering delays stdout.
  for dev in /dev/ttyS0 /dev/console; do
    [[ -e "${dev}" ]] && echo "${msg}" > "${dev}" || true
  done
}

fail() {
  emit "${MARKER_FAIL}: $*"
  exit 1
}

emit "clawgress-live-selftest: starting"

# Basic sanity: kernel + uname reachable.
uname -r >/dev/null 2>&1 || fail "uname -r failed"
emit "kernel: $(uname -r)"

# Check nftables if installed (not fatal if missing).
if systemctl list-units --full --all 2>/dev/null | grep -q nftables; then
  systemctl is-active --quiet nftables || emit "WARN: nftables not active"
  nft list ruleset >/dev/null 2>&1 && emit "nftables: ok" || emit "WARN: nftables ruleset error"
fi

# Check haproxy if installed (not fatal if missing).
if systemctl list-units --full --all 2>/dev/null | grep -q haproxy; then
  systemctl is-active --quiet haproxy && emit "haproxy: ok" || emit "WARN: haproxy not active"
fi

emit "${MARKER_PASS}"
