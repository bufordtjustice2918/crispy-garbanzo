#!/usr/bin/env bash
set -euo pipefail

MARKER_PASS="CLAWGRESS_LIVE_SELFTEST_PASS"
MARKER_FAIL="CLAWGRESS_LIVE_SELFTEST_FAIL"
LOG_FILE="/var/log/clawgress-live-selftest.log"

log() {
  echo "[$(date -Is)] $*" | tee -a "${LOG_FILE}" >/dev/null
  if [[ -e /dev/ttyS0 ]]; then
    echo "$*" > /dev/ttyS0 || true
  fi
  if [[ -e /dev/console ]]; then
    echo "$*" > /dev/console || true
  fi
}

fail() {
  log "${MARKER_FAIL}: $*"
  exit 1
}

log "starting clawgress live self-test"

for svc in nftables haproxy bind9; do
  systemctl restart "${svc}" || fail "failed to restart ${svc}"
  systemctl is-active --quiet "${svc}" || fail "service not active: ${svc}"
  log "service active: ${svc}"
done

nft list ruleset >/dev/null 2>&1 || fail "nftables ruleset unavailable"
haproxy -c -f /etc/haproxy/haproxy.cfg >/dev/null 2>&1 || fail "haproxy config invalid"

log "${MARKER_PASS}"
