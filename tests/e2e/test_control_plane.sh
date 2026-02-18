#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
STATE_DIR="${TMP_DIR}/state"
CANDIDATE="${TMP_DIR}/candidate.json"

cleanup() {
  if [[ -n "${API_PID:-}" ]] && kill -0 "${API_PID}" 2>/dev/null; then
    kill "${API_PID}" || true
  fi
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cd "${ROOT_DIR}"

go build -o "${TMP_DIR}/clawgress-admin-api" ./cmd/clawgress-admin-api
go build -o "${TMP_DIR}/clawgressctl" ./cmd/clawgressctl

CLAWGRESS_STATE_DIR="${STATE_DIR}" CLAWGRESS_ADMIN_LISTEN=":18080" "${TMP_DIR}/clawgress-admin-api" >/tmp/clawgress-admin-api.log 2>&1 &
API_PID=$!

for _ in {1..30}; do
  if curl -fsS "http://127.0.0.1:18080/healthz" >/dev/null; then
    break
  fi
  sleep 0.2
done

"${TMP_DIR}/clawgressctl" set --file "${CANDIDATE}" system host-name clawgress-gw
"${TMP_DIR}/clawgressctl" set --file "${CANDIDATE}" system ntp server 0.pool.ntp.org
"${TMP_DIR}/clawgressctl" set --file "${CANDIDATE}" policy egress default-action deny
"${TMP_DIR}/clawgressctl" show --file "${CANDIDATE}" configuration commands | grep -q "set system host_name clawgress-gw"

CONFIGURE_JSON="$("${TMP_DIR}/clawgressctl" configure --url "http://127.0.0.1:18080" --actor e2e --file "${CANDIDATE}")"
REVISION_ID="$(printf '%s' "${CONFIGURE_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["response"]["revision_id"])')"
[[ -n "${REVISION_ID}" && "${REVISION_ID}" != "null" ]]

COMMIT_JSON="$("${TMP_DIR}/clawgressctl" commit --url "http://127.0.0.1:18080" --actor e2e --expected-revision "${REVISION_ID}")"
printf '%s' "${COMMIT_JSON}" | python3 -c 'import json,sys; assert json.load(sys.stdin)["response"]["status"]=="committed"'

STATE_JSON="$("${TMP_DIR}/clawgressctl" state --url "http://127.0.0.1:18080")"
printf '%s' "${STATE_JSON}" | python3 -c 'import json,sys; x=json.load(sys.stdin)["response"]; assert x["active"]["revision_id"] is not None; assert x.get("staged") is None; assert x["history_size"] >= 1'

echo "control-plane e2e: PASS"
