# Clawgress MVP v1

## 1. Goal
Ship a working Clawgress release that enforces identity-aware outbound egress policy for AI agents on an Ubuntu 24.04 LTS baseline.

Success means a team can deploy Clawgress, route agent egress through it, enforce policy and rate limits, and retrieve immutable decision logs.

## 2. Ubuntu 24.04 Baseline

### 2.1 Supported OS
- Ubuntu Server 24.04 LTS (x86_64) is the primary and required MVP host OS.
- Kernel and package updates come from official Ubuntu 24.04 repositories.

### 2.2 Host Requirements (per gateway node)
- 4 vCPU minimum
- 8 GB RAM minimum
- 80 GB SSD minimum
- Static IP or reserved DHCP lease
- NTP enabled and synchronized

### 2.3 Required Host Packages
- `ca-certificates`
- `curl`
- `jq`
- `nftables`
- `iproute2`
- `conntrack`
- `systemd`
- `rsyslog` or journald forwarder

`nftables` is mandatory for MVPv1 host-level traffic control and transparent-gateway enforcement.

## 3. MVP Scope

### 3.1 In Scope
1. Identity binding
- Required identity on every outbound request.
- MVP auth methods: signed JWT and API key.
- Canonical identity fields: `agent_id`, `team_id`, `project_id`, `environment`.

2. Egress policy enforcement
- Domain allowlist and denylist.
- HTTP method control.
- Optional path prefix matching.
- Deterministic allow/deny decision with `policy_id`.

3. Per-agent rate limits
- Requests per second.
- Requests per minute.
- Enforcement modes: `hard_stop`, `alert_only`.

4. Immutable audit logging
- One decision event per request.
- Log fields: timestamp, request_id, identity, destination, decision, policy_id, bytes, latency.

5. Minimal control plane
- Admin API for agents, policies, quotas, and log query.
- Basic web UI for agent list, policy list, and recent traffic decisions.

6. Appliance-style operations UX
- CLI command model aligned with network-appliance workflows.
- Transactional `set`/`show` operational commands backed by `configure`/`commit`.
- Installer command path for live-media to disk installation.

### 3.2 Out of Scope
- Full outbound DLP redaction pipeline.
- In-line anomaly model blocking.
- Deep AI payload parsing and tool-call governance.
- SaaS multi-region control plane.
- Full token-based cost chargeback.

## 4. Deployment Modes for MVP

### 4.1 Mode A (Primary): Explicit Proxy
- Agents set `HTTP_PROXY` and `HTTPS_PROXY`.
- Fastest to deploy with minimal network redesign.
- Preferred for MVP pilots.

### 4.2 Mode B (Secondary): Transparent Gateway
- Ubuntu host acts as default egress gateway.
- Traffic is routed/NATed through Clawgress.
- `nftables` is the required packet filter/NAT framework for this mode.
- Used for VM subnet egress control.

### 4.3 LiveCD Appliance Mode (SquashFS Boot)
- System boots read-only root from SquashFS image (VyOS-style operator experience).
- Runtime writes go to ephemeral overlay until explicit install.
- Includes first-boot CLI access for `set`, `show`, `configure`, `commit`, and `install`.
- Supports transition from live media to disk install with preserved committed config.
- Live image includes `nftables`, `bind9`, and `haproxy` enabled as core appliance daemons.

## 5. Component Plan

### 5.1 Services
- `clawgress-gateway`
- `clawgress-identity`
- `clawgress-policy`
- `clawgress-quota`
- `clawgress-audit`
- `clawgress-admin-api`
- `clawgress-admin-ui`

### 5.2 Request Flow
1. Agent sends outbound request to gateway.
2. Gateway resolves identity from JWT or API key.
3. Gateway evaluates local signed policy bundle.
4. Gateway checks quota state for the agent.
5. Gateway allows, denies, or rate-limits.
6. Gateway publishes an immutable audit event.

### 5.3 Runtime Topology (MVP)
- Gateway nodes: horizontally scalable.
- Control-plane node(s): admin API and services.
- Data node(s): relational DB + append-only audit store.

## 6. CLI and API Operation Modes (`opmode`)

MVPv1 includes a first-party CLI and API surface with explicit operational modes.

### 6.1 `opmode=configure`
- Purpose: stage and validate configuration changes.
- Behavior: update agents, policies, quotas, and gateway settings without activating them immediately.
- Output: returns a config revision ID and validation report.
- Safety: no runtime enforcement changes are applied until `opmode=commit`.

### 6.2 `opmode=commit`
- Purpose: transactionally apply staged configuration.
- Behavior: atomically publishes a signed policy/config bundle to gateways.
- Output: returns commit ID, applied revision, timestamp, and rollout status.
- Safety: all-or-nothing commit semantics with rollback to last-known-good revision on failure.

### 6.3 API Access
- All control-plane operations are exposed via authenticated REST APIs.
- CLI is a thin client over the same APIs; no hidden local-only mutations.
- Every `configure` and `commit` call is audit logged with actor, diff summary, and result.

### 6.4 Repo-Linked Auto-Commit Workflow
- Control-plane change artifacts are persisted in this repo for traceability.
- Default workflow for this project: commit generated config/spec updates after successful `opmode=commit`.
- Commit metadata includes revision ID and operator identity.

### 6.5 CLI Command Model
- `set <path> <value>` updates candidate configuration.
- `show` displays candidate or active state.
- `configure` pushes candidate changes to staged control-plane revision.
- `commit` atomically activates staged revision.
- `install --target-disk <disk>` executes live-media to disk installation workflow.

## 7. Ubuntu 24.04 System Layout

### 7.1 Directories
- `/etc/clawgress/` for service configs
- `/var/lib/clawgress/` for local state and cache
- `/var/log/clawgress/` if file logging is enabled

### 7.2 Service Management
All services run as non-root systemd units:
- `clawgress-gateway.service`
- `clawgress-identity.service`
- `clawgress-policy.service`
- `clawgress-quota.service`
- `clawgress-audit.service`
- `clawgress-admin-api.service`
- `clawgress-admin-ui.service`
- `chrony.service` (or equivalent NTP unit) for synchronized control-plane time

### 7.3 Security Defaults
- Ubuntu unattended security updates enabled.
- AppArmor enforced for gateway and API services.
- Inbound ports default-deny except management and proxy ports.
- mTLS for internal service-to-service traffic.
- No plaintext secrets on disk.

## 8. nftables Enforcement Baseline (Required)
- Backend: `nftables` only (no legacy iptables rule authoring in MVP code paths).
- Rule model: table/chain/set based rules with atomic updates for policy changes.
- Required usage: NAT and forwarding control for transparent gateway mode.
- Required usage: fast destination allow/deny matching via nft sets/maps.
- Required usage: packet/flow accounting hooks for egress observability.
- Optional usage: host-level egress deny guardrails for non-proxy traffic bypass attempts.

## 9. Data Contracts (MVP Minimum)

### 9.1 Identity
- `agent_id` (required)
- `team_id` (required)
- `project_id` (required)
- `environment` (required)
- `auth_type` (`jwt` or `api_key`)
- `status` (`active` or `disabled`)

### 9.2 Policy
- `policy_id`
- `version`
- `priority`
- `enabled`
- `match.identity`
- `match.destination`
- `match.method`
- `match.path_prefix`
- `action` (`allow` or `deny`)

### 9.3 Quota
- `agent_id`
- `rps_limit`
- `rpm_limit`
- `mode` (`hard_stop` or `alert_only`)

### 9.4 Audit Event
- `timestamp`
- `request_id`
- `agent_id`
- `team_id`
- `project_id`
- `environment`
- `destination_host`
- `http_method`
- `path`
- `decision`
- `policy_id`
- `quota_applied`
- `latency_ms`
- `bytes_out`
- `bytes_in`

## 10. SLOs and Acceptance Criteria

### 10.1 Functional
- Anonymous outbound traffic is denied 100% of the time.
- Same request context always produces the same policy decision.
- Per-agent limits apply correctly under concurrent traffic.
- Every request has one and only one decision log event.

### 10.2 Performance
- Added p50 latency <= 5 ms (no TLS interception path).
- Added p95 latency <= 15 ms under nominal load.
- At least 2k concurrent outbound connections per gateway node on MVP reference hardware.

### 10.3 Reliability
- Last-known-good policy remains active if policy service is down.
- Audit sink outages do not block request path; local queue and alerting must trigger.

## 11. 90-Day Delivery Plan

### Phase 1 (Weeks 1-3): Core Foundations
- Repo/service scaffolding and CI.
- Ubuntu 24.04 packaging and systemd templates.
- Identity schema and auth verification implementation.
- CLI parser for `set`/`show` command families.

### Phase 2 (Weeks 4-6): Enforcement Core
- Gateway integration with policy engine.
- Policy bundle compile, sign, and distribution.
- Deterministic decision tracing and audit events.
- SquashFS live boot image build pipeline and boot test harness.

### Phase 3 (Weeks 7-9): Quotas and Control Plane
- Per-agent rate limit engine.
- Admin API for identity/policy/quota operations.
- Basic admin UI views.

### Phase 4 (Weeks 10-12): Hardening and Release
- Load, failure, and recovery testing.
- Security baseline validation on Ubuntu 24.04.
- MVP release candidate and operator runbook.
- Disk installer (`install`) validation from live mode to persistent system.

## 12. Testing Matrix
- Unit: policy evaluator, identity resolver, quota logic.
- Integration: full request path identity -> policy -> quota -> decision -> audit.
- Performance: sustained load and latency budgets.
- Failure: policy service outage, audit backend outage, identity backend timeout.
- Upgrade: rolling restart of services on Ubuntu 24.04 without data loss.

## 13. Release Artifacts
- Versioned binaries or container images for all services.
- Ubuntu 24.04 install guide and operations runbook.
- LiveCD ISO with SquashFS root filesystem.
- Default starter policy bundle.
- Admin API reference.
- Dashboard and log query examples.

## 14. MVP Exit Criteria
MVPv1 is complete when all in-scope features run on Ubuntu 24.04, pass the acceptance criteria above, and complete at least one successful pilot deployment in explicit proxy mode.
