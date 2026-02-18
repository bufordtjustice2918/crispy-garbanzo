# Clawgress Architecture (Draft)

## 1. Purpose
Clawgress is an agent-aware egress governance gateway. It enforces identity, policy, quotas, and outbound DLP for autonomous AI workloads.

This draft maps the product spec into an implementable architecture with Ubuntu 24.04 as the base operating system for all gateway and control-plane nodes.

## 2. Scope and Principles
- Identity is mandatory for any outbound decision.
- Policy evaluation is deterministic and auditable.
- Data plane remains stateless where possible.
- Control plane is authoritative for policy and identity metadata.
- Fail mode (`fail-open` or `fail-closed`) is explicit and configurable per environment.

## 3. Deployment Targets (Ubuntu 24.04)
- Host OS: Ubuntu Server 24.04 LTS.
- Runtime: systemd-managed services and/or containerized workloads on Ubuntu 24.04.
- Networking stack: nftables/iptables compatibility, conntrack, policy routing.
- Optional Kubernetes mode: Ubuntu 24.04 worker nodes with sidecar/egress-gateway deployment.

## 4. High-Level Runtime Flow
1. Agent emits outbound request with identity assertion (mTLS/JWT/API key).
2. Traffic enters Clawgress data plane (transparent gateway or explicit proxy).
3. Identity Binding Layer resolves canonical identity:
   - `agent_id`, `team_id`, `project_id`, `environment`, `workload_fingerprint`.
4. Policy Engine evaluates request context.
5. Quota/Budget Engine applies limits and cost controls.
6. DLP Engine classifies outbound payload.
7. Enforcement action is executed: allow, deny, throttle, quarantine, transform, alert, log-only.
8. Request metadata and decision are written to immutable audit stream.

## 5. Core Components

### 5.1 Data Plane Gateway
Responsibilities:
- L4/L7 request interception and forwarding.
- Identity extraction and propagation to policy engine.
- In-line enforcement and response shaping.

Implementation notes:
- Prefer a high-performance proxy core for HTTP(S) and CONNECT traffic.
- Keep per-request decision path local and low-latency via policy cache.

### 5.2 Identity Binding Layer
Responsibilities:
- Validate identity assertions.
- Map runtime credentials to canonical agent identity.
- Reject anonymous/unknown identities.

Data sources:
- Agent registry from control plane.
- Trust anchors/cert bundles and token verification keys.

### 5.3 Policy Engine
Responsibilities:
- Deterministic evaluation over identity, destination, protocol, payload classification, historical signals.
- Return action + reason + policy ID.

Design:
- Declarative policy DSL compiled to an execution plan.
- Versioned policy bundles with signed metadata.

### 5.4 AI Protocol Analyzer
Responsibilities:
- Parse AI/LLM request structures.
- Extract provider/model metadata, estimated token usage, tool call intents.
- Provide normalized attributes to policy and quota engines.

### 5.5 Quota & Budget Engine
Responsibilities:
- Enforce per-agent/team/project quotas.
- Track token and request consumption windows.
- Support hard-stop, soft-stop, throttle, alert-only modes.

### 5.6 DLP Engine
Responsibilities:
- Scan outbound payload for sensitive classes (PII/PHI/keys/secrets).
- Produce match taxonomy and confidence.
- Return block/redact/alert/hash-log actions.

### 5.7 Behavioral Analysis Engine
Responsibilities:
- Maintain baseline per agent.
- Score anomalies (new domain, spike, ASN change, token surge).
- Trigger risk actions (alert/quarantine/rate reduction).

For MVP this runs near-real-time from audit stream, not in-line critical path.

### 5.8 Control Plane API and UI
Responsibilities:
- Agent lifecycle, policy lifecycle, quota management, audit search, metrics export.
- Multi-tenant RBAC and audit-tracked change management.

## 6. Data and Storage Architecture

### 6.1 Operational Datastores
- Relational DB: policy versions, agent registry, RBAC, configuration.
- Time-series/columnar store: request/usage metrics.
- Immutable log store: append-only decision and audit events.
- Optional cache: policy and identity cache for low-latency data plane reads.

### 6.2 Event Schema (minimum)
- `timestamp`
- `request_id`
- `agent_id`, `team_id`, `project_id`, `environment`
- `destination_host`, `destination_asn`, `destination_geo`
- `http_method`, `path`
- `decision`, `policy_id`, `enforcement_mode`
- `token_estimate`, `bytes_out`, `bytes_in`, `latency_ms`
- `dlp_matches[]`, `anomaly_score`

## 7. Network Modes

### 7.1 Transparent Gateway
- Ubuntu host routes workload subnet egress through Clawgress.
- NAT and policy routing at host edge.
- Best for VM fleets and central egress points.

### 7.2 Explicit Proxy
- Agents use `HTTP_PROXY`/`HTTPS_PROXY`.
- Strongest L7 context with minimal network re-architecture.
- Best MVP adoption path.

### 7.3 Kubernetes Egress Gateway
- Namespace or service-account scoped egress control.
- Identity via workload identity + mTLS.

## 8. Security Model
- mTLS for service-to-service control/data-plane communication.
- Encrypted at rest for configuration and secrets.
- No plaintext credentials in config files or logs.
- RBAC with least privilege roles.
- Signed policy bundles and audit of all policy changes.

## 9. Performance and Availability Targets
- No TLS interception path target: <5ms median added latency.
- Horizontal scale-out with stateless data plane replicas.
- Control plane HA with replicated datastore.
- Backpressure behavior is explicit under downstream degradation.

## 10. Failure Handling
- Identity resolver unavailable: deny by default unless explicitly configured otherwise.
- Policy fetch failure: use last-known-good signed bundle.
- Quota backend degraded: configurable soft-fail vs hard-fail.
- Audit sink unavailable: local write-ahead queue with bounded retention and alerts.

## 11. Ubuntu 24.04 Baseline Build
Minimum host profile for gateway nodes:
- Ubuntu Server 24.04 LTS
- systemd service units for gateway/control agents
- nftables + conntrack tooling
- journald + log shipper to central SIEM
- chrony/ntp for deterministic timestamps

Hardening baseline:
- Unattended security updates enabled.
- AppArmor enforced where feasible.
- CIS-aligned SSH and kernel network sysctl profile.

## 12. Out-of-Scope for MVPv1
- Full in-line behavioral anomaly blocking.
- Advanced payload transformation pipelines.
- Full SaaS multi-region control plane.

