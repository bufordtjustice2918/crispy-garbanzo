# Clawgress MVPv1 Implementation Plan

## Status Legend
- ✅ Complete
- 🔄 In Progress
- ⬜ Not Started

---

## Infrastructure & CI
- ✅ Debian Bookworm LiveCD ISO (live-build, iso-hybrid)
- ✅ UEFI/GRUB EFI serial boot with OVMF
- ✅ ISO boot test (boot-test.sh — CLAWGRESS_LIVE_SELFTEST_PASS marker)
- ✅ Pexpect e2e smoke suite (login + service checks)
- ✅ GitHub Actions CI pipeline (build-livecd → boot-test → e2e)
- ✅ Image-based disk installer (clawgress-install.sh, VyOS-style squashfs)

---

## Step 1 — Gateway Proxy (current)

The data plane. Agents point `HTTP_PROXY`/`HTTPS_PROXY` at this. Every
outbound request is identity-checked, policy-evaluated, and audit-logged.

### Code
- ✅ `internal/identity` — API key registry (JSON file, SIGHUP reload)
- ✅ `internal/policy`   — domain allowlist/denylist engine
- ✅ `internal/audit`    — append-only JSONL decision log
- ✅ `cmd/clawgress-gateway` — explicit HTTP/HTTPS CONNECT proxy on :3128

### ISO integration
- ✅ Seed agent registry  `/etc/clawgress/agents.json`
- ✅ Seed policy file     `/etc/clawgress/policy.json`
- ✅ Systemd unit         `clawgress-gateway.service`
- ✅ Systemd unit         `clawgress-admin-api.service`
- ✅ Enable both services in `010-enable-services.hook.chroot`
- ✅ Open port 3128 and 8080 in nftables baseline
- ✅ Go build step in CI (`build-livecd` job, before `lb build`)

### E2E validation (CI)
- ✅ `clawgress-gateway` service active
- ✅ `clawgress-admin-api` service active
- ✅ Anonymous proxy request → 407 Proxy Authentication Required
- ✅ Valid API key → request proxied (non-407)
- ✅ Policy-blocked domain → 403 Forbidden
- ✅ Audit log exists and contains valid JSON entries
- ✅ Audit log contains both `allow` and `deny` decisions

---

## Step 2 — Identity + Policy CRUD via Admin API
- ✅ `POST /v1/agents`       — create/update agent records
- ✅ `GET  /v1/agents`       — list agents
- ✅ `GET  /v1/agents/{id}`  — get agent by ID
- ✅ `DELETE /v1/agents/{id}` — remove agent
- ✅ `POST /v1/policies`     — create/update policy rules
- ✅ `GET  /v1/policies`     — list active policy
- ✅ `GET  /v1/policies/{id}` — get policy by ID
- ✅ `DELETE /v1/policies/{id}` — remove policy
- ✅ Gateway hot-reload: admin API sends SIGHUP to gateway after every write
- ✅ Atomic disk persistence (write-tmp + rename) for agents.json and policy.json
- ✅ E2E: create agent via API, verify proxy accepts new key (hot-reload)
- ✅ E2E: delete agent, verify proxy rejects deleted key (407)
- ✅ E2E: create/get/delete policy cycle with verification

---

## Step 3 — Audit Query
- ✅ `internal/audit/query.go` — JSONL reader with Filter (agent_id, decision, since, limit)
- ✅ `GET /v1/audit`         — tail/filter audit log (query params: agent_id, decision, since, limit)
- ✅ `clawgressctl show audit [--agent <id>] [--decision <d>] [--since <ts>] [--limit N] [--json]`
- ✅ `clawgressctl` binary baked into ISO (`/usr/local/sbin/clawgressctl`)
- ✅ E2E: query audit API, filter by decision, filter by agent_id, limit, field validation
- ✅ E2E: `clawgressctl show audit` table output, JSON output, agent filter

---

## Step 4 — Quota / Rate Limiter
- ⬜ `internal/quota`        — in-memory token bucket per agent_id (RPS + RPM)
- ⬜ Wire quota check into gateway request path (after identity, before policy)
- ⬜ `POST /v1/quotas`       — set per-agent limits
- ⬜ Enforcement modes: `hard_stop` (429), `alert_only` (log + allow)
- ⬜ E2E: configure low RPS limit, burst past it, verify 429 responses

---

## Step 5 — Admin UI (basic)
- ⬜ Single-page app (or server-rendered): agent list, policy list, recent decisions
- ⬜ Served by `clawgress-admin-api` on `/ui/`
- ⬜ E2E: `curl http://localhost:8080/ui/` returns 200

---

## MVP Exit Criteria (from MVPv1.md)
- Anonymous outbound traffic denied 100% of the time
- Same request context always produces the same policy decision
- Per-agent limits apply correctly under concurrent traffic
- Every request has one and only one audit event
- p50 added latency ≤ 5ms (no TLS interception path)
- At least one successful pilot deployment in explicit proxy mode
