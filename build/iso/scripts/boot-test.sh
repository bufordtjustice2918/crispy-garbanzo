#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <iso-path> [timeout-seconds]" >&2
  exit 1
fi

ISO_PATH="$1"
TIMEOUT_SECS="${2:-900}"
LOG_PATH="${LOG_PATH:-build/iso/out/boot-test.log}"
PASS_MARKER="CLAWGRESS_LIVE_SELFTEST_PASS"
FAIL_MARKER="CLAWGRESS_LIVE_SELFTEST_FAIL"

mkdir -p "$(dirname "${LOG_PATH}")"

if [[ ! -f "${ISO_PATH}" ]]; then
  echo "ISO not found: ${ISO_PATH}" >&2
  exit 1
fi

run_qemu() {
  local mode="$1"
  local mode_timeout="$2"
  local mode_log="$3"
  shift 3
  local extra_args=("$@")

  set +e
  timeout "${mode_timeout}" qemu-system-x86_64 \
    -m 4096 \
    -smp 2 \
    -machine accel=tcg \
    "${extra_args[@]}" \
    -cdrom "${ISO_PATH}" \
    -boot d \
    -nographic \
    -no-reboot >"${mode_log}" 2>&1
  local qemu_exit=$?
  set -e

  if grep -q "${PASS_MARKER}" "${mode_log}"; then
    cp "${mode_log}" "${LOG_PATH}"
    echo "ISO boot self-test: PASS (${mode})"
    exit 0
  fi

  if grep -q "${FAIL_MARKER}" "${mode_log}"; then
    cp "${mode_log}" "${LOG_PATH}"
    echo "ISO boot self-test: FAIL marker detected (${mode})" >&2
    tail -n 200 "${LOG_PATH}" >&2 || true
    exit 1
  fi

  echo "ISO boot self-test: ${mode} attempt failed (exit ${qemu_exit})" >&2
}

BIOS_LOG="${LOG_PATH%.log}-bios.log"
run_qemu "bios" "${TIMEOUT_SECS}" "${BIOS_LOG}"
cp "${BIOS_LOG}" "${LOG_PATH}" || true

echo "ISO boot self-test: timed out or failed without PASS marker" >&2

tail -n 200 "${LOG_PATH}" >&2 || true
exit 1
