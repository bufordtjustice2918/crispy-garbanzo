# Clawgress MVP v1 (Draft)

## 1. MVP Objective
Deliver a production-viable first release that enforces identity-aware outbound policy for AI agents with auditable decisions and per-agent rate controls.

Primary outcome: organizations can place Clawgress in front of agent traffic and reliably control who can call what, how often, and with full request-level attribution.

## 2. Base Platform
- OS baseline: Ubuntu Server 24.04 LTS for all non-Kubernetes deployments.
- Preferred first deployment mode: Explicit Proxy on Ubuntu 24.04.
- Secondary MVP mode: Transparent Gateway on Ubuntu 24.04.

## 3. In-Scope Features (MVPv1)
1. Identity Binding
- Required identity on every outbound request.
- Supported in MVP: JWT and API key binding.
- Canonical fields: `agent_id`, `team_id`, `project_id`, `environment`.

2. Policy Enforcement
- Domain allowlist/denylist.
- HTTP method restrictions.
- Optional path prefix restrictions.
- Deterministic allow/deny decisions with policy ID attribution.

3. Rate Limiting
- Per-agent requests-per-second and requests-per-minute limits.
- Enforcement modes: hard stop and alert-only.

4. Logging and Audit
- Structured immutable decision logs for every request.
- Required fields: timestamp, identity, destination, decision, policy_id, latency, bytes, request_id.

5. Admin API (minimal)
- Agent registration CRUD.
- Policy CRUD + publish.
- Quota configuration endpoints.
- Audit log query endpoint.

6. Basic Operator UI
- Agent registry list.
- Policy list/status.
- Recent decisions dashboard.

## 4. Out-of-Scope (MVPv1)
- Full DLP classification/redaction.
- AI protocol deep parsing and tool-call enforcement.
- Behavioral anomaly engine in active enforcement path.
- Multi-tenant MSP/federated control planes.
- Full cost accounting per token/provider.

## 5. MVP Architecture Slice

### 5.1 Components to Build
- `gateway`: L7 proxy + enforcement plugin.
- `identity-service`: validates JWT/API key and resolves canonical identity.
- `policy-service`: policy storage, compilation, signed bundle publish.
- `quota-service`: per-agent counters and rate-limit decisions.
- `audit-service`: append-only event intake and query.
- `admin-api`: external management API.
- `admin-ui`: lightweight web UI over admin-api.

### 5.2 Request Path (MVP)
1. Agent sends request via explicit proxy or gateway route.
2. Gateway requests identity resolution.
3. Gateway evaluates policy from local signed bundle cache.
4. Gateway checks quota decision.
5. Gateway enforces allow/deny/rate-limit.
6. Gateway emits audit event asynchronously.

## 6. Ubuntu 24.04 Deployment Blueprint

### 6.1 Node Roles
- Gateway nodes (horizontal scale).
- Control-plane nodes (API/services).
- Data nodes (DB/log store), can be co-located for small environments.

### 6.2 Service Management
- systemd units for each Clawgress service.
- Environment files under `/etc/clawgress/`.
- Logs via journald + forwarder to SIEM.

### 6.3 Network and Security Defaults
- Inbound limited to required ports only.
- mTLS between gateway and internal services.
- Ubuntu 24.04 security updates enabled by default.
- AppArmor profiles for gateway and API services.

## 7. Data Contracts (MVP Minimum)

### 7.1 Identity Record
- `agent_id` (required)
- `team_id` (required)
- `project_id` (required)
- `environment` (required)
- `auth_type` (`jwt` | `api_key`)
- `status` (`active` | `disabled`)

### 7.2 Policy Record
- `policy_id`
- `version`
- `match` (identity selectors + destination/method/path rules)
- `action` (`allow` | `deny`)
- `priority`
- `enabled`

### 7.3 Quota Record
- `agent_id`
- `rps_limit`
- `rpm_limit`
- `mode` (`hard_stop` | `alert_only`)

## 8. SLOs and Acceptance Criteria

### 8.1 Functional Acceptance
- Anonymous requests are denied.
- Policy decisions are deterministic and reproducible.
- Per-agent limits are enforced under concurrent load.
- Every decision emits exactly one audit event.

### 8.2 Performance Acceptance
- p50 added latency <= 5ms in no-TLS-interception mode.
- p95 added latency <= 15ms under nominal load.
- Sustained 2k concurrent outbound connections per gateway node for MVP target hardware.

### 8.3 Reliability Acceptance
- Gateway remains functional with last-known-good policy if policy-service is unavailable.
- Audit pipeline backpressure does not block enforcement path (bounded queue + alert).

## 9. Delivery Plan (90 Days)

## Phase A (Weeks 1-3): Foundations
- Repository layout, service scaffolding, CI baseline.
- Identity model and schema finalized.
- Policy DSL v0 and compiler MVP.

## Phase B (Weeks 4-6): Enforce and Log
- Gateway enforcement integration.
- Policy bundle distribution and cache.
- Audit event pipeline and query API.

## Phase C (Weeks 7-9): Quotas and API/UI
- Quota service and in-line checks.
- Admin API for identity/policy/quota management.
- Basic UI for operators.

## Phase D (Weeks 10-12): Hardening and Release
- Load, chaos, and fail-mode testing.
- Ubuntu 24.04 install package and systemd templates.
- Security review and release candidate.

## 10. Test Strategy (MVP)
- Unit tests for policy evaluation determinism.
- Integration tests for request path (identity -> policy -> quota -> decision).
- Replay tests for audit schema compatibility.
- Load tests for latency and rate-limit correctness.
- Failure injection tests for policy and audit backend outages.

## 11. Release Artifacts
- Versioned service binaries or container images.
- Ubuntu 24.04 install/runbook.
- Default policy starter pack.
- API reference for admin endpoints.
- Operational dashboard starter queries.

## 12. MVP Exit Criteria
MVPv1 is complete when all in-scope features ship on Ubuntu 24.04 with passing acceptance criteria and runbook-verified deployment in at least one explicit-proxy environment.

