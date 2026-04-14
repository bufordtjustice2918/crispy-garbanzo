# Clawgress

```
  _____ _
 / ____| |
| |    | | __ ___      ____ _ _ __ ___  ___ ___
| |    | |/ _` \ \ /\ / / _` | '__/ _ \/ __/ __|
| |____| | (_| |\ V  V / (_| | | |  __/\__ \__ \
 \_____|_|\__,_| \_/\_/ \__, |_|  \___||___/___/
                         __/ |
                        |___/   Egress Governance
```

**Identity-aware egress policy enforcement for AI agents.**

Clawgress is a Debian Bookworm appliance that controls what your AI agents can access on the internet. Every outbound request is identity-checked, policy-evaluated, rate-limited, and audit-logged.

## Features

- **Explicit proxy** (`:3128`) — agents set `HTTP_PROXY`/`HTTPS_PROXY`
- **API key + JWT auth** — every request requires identity
- **Domain allowlist/denylist** — with method, path prefix, and condition matching
- **DNS RPZ** — blocked domains resolve to NXDOMAIN before TCP even starts
- **Per-agent rate limiting** — RPS/RPM with hard_stop or alert_only modes
- **Immutable audit log** — every decision recorded as JSONL
- **Admin API** (`:8080`) — full CRUD for agents, policies, quotas
- **Admin UI** — dark-theme dashboard at `/ui/`
- **VyOS-style CLI** — `show`, `configure`, `commit`, `set` commands
- **LiveCD ISO** — boot from USB, configure, install to disk
- **Signed policy bundles** — HMAC-SHA256 integrity verification
- **nftables integration** — dynamic firewall rules from policy + transparent gateway mode

## Quick Start

### Boot the ISO

Boot the Clawgress LiveCD ISO (UEFI or BIOS). Login:

| Field | Value |
|-------|-------|
| Username | `clawgress` |
| Password | `clawgress` |
| Sudo | passwordless |

### Appliance Shell

```
clawgress@clawgress:~$ show agents
clawgress@clawgress:~$ show audit --limit 10
clawgress@clawgress:~$ show health
clawgress@clawgress:~$ configure
clawgress(config)# set policy allow-api agent my-agent domains api.openai.com action allow
clawgress(config)# commit
clawgress(config)# exit
clawgress@clawgress:~$ ping google.com   ← normal bash still works
```

### Point Agents at the Proxy

```bash
export HTTP_PROXY=http://my-agent:my-api-key@gateway-ip:3128
export HTTPS_PROXY=http://my-agent:my-api-key@gateway-ip:3128
curl https://api.openai.com/v1/models  # → allowed or denied by policy
```

### Install to Disk

```bash
sudo clawgressctl install --target-disk /dev/sda --yes
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Health check |
| `POST/GET/DELETE /v1/agents[/{id}]` | Agent CRUD |
| `POST/GET/DELETE /v1/policies[/{id}]` | Policy CRUD (method/path/condition support) |
| `POST/GET/DELETE /v1/quotas[/{agent_id}]` | Rate limit CRUD |
| `GET /v1/audit` | Query audit log (filter by agent, decision, time) |
| `POST /v1/policy/sign` | Sign policy bundle |
| `GET /v1/policy/conflicts` | Detect shadowed rules |
| `POST /v1/rpz/generate` | Generate DNS RPZ zone + reload bind9 |
| `GET /v1/nft/render` | Render nftables from policy |
| `GET /ui/` | Admin dashboard |

Full API reference: [`docs/RUNBOOK.md`](docs/RUNBOOK.md)

## Testing

Three-tier validation for every feature:

| Tier | Count | What |
|------|-------|------|
| Unit tests | 135 | Isolated function tests with race detection |
| E2E smoke | 152 | API + CLI on live-booted ISO in QEMU/KVM |
| Acceptance | 9 | Real traffic through proxy proves enforcement |
| Fuzz | 4 | Random input to security-critical parsers |
| Adversarial | 14 | Bypass attempts (null bytes, alg:none JWT, case tricks) |
| Load | 6 | 100-goroutine concurrent stress with `-race` |

**287+ total automated validations**, all running in CI on every push.

## Architecture

```
Agent → HTTP_PROXY → clawgress-gateway (:3128)
                        ├─ Identity check (API key / JWT)
                        ├─ Quota check (token bucket)
                        ├─ Policy evaluation (domain/method/path/conditions)
                        ├─ Audit log (JSONL)
                        └─ Forward or reject

Admin → clawgress-admin-api (:8080)
           ├─ CRUD: agents, policies, quotas
           ├─ Audit query
           ├─ RPZ zone generation
           ├─ nftables rendering
           └─ Web UI (/ui/)
```

## Docs

- [`docs/RUNBOOK.md`](docs/RUNBOOK.md) — Deployment, configuration, monitoring, hardening
- [`docs/MVPv1.md`](docs/MVPv1.md) — MVP specification
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — System architecture
- [`docs/E2E_TESTING.md`](docs/E2E_TESTING.md) — Test infrastructure details
- [`docs/IMPLEMENTATION_PLAN.md`](docs/IMPLEMENTATION_PLAN.md) — Development status

## CI/CD

| Workflow | What | Time |
|----------|------|------|
| `e2e.yml` | Build, unit tests, race detector, govulncheck | ~1.5 min |
| `build-iso.yml` | ISO build → QEMU/KVM boot → 152 e2e commands | ~13 min |

## Performance

| Operation | Latency |
|-----------|---------|
| Identity lookup | ~40 ns |
| Policy evaluation | ~250 ns |
| Quota check | ~400 ns |
| **Full request path** | **~690 ns** |

7,000x faster than the 5ms p50 target.

## License

Proprietary. See LICENSE file.
