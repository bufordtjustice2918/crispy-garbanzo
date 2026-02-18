#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
REPORT_DIR="${ROOT_DIR}/tests/e2e/out"
REPORT_FILE="${REPORT_DIR}/command-conformance.json"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cd "${ROOT_DIR}"
go build -o "${TMP_DIR}/clawgressctl" ./cmd/clawgressctl
python3 ./tests/e2e/command_conformance.py \
  --schema ./internal/cmdmap/command_schema.json \
  --binary "${TMP_DIR}/clawgressctl" \
  --report "${REPORT_FILE}"

echo "command conformance report: ${REPORT_FILE}"
