# MVPv1 Initial Development Tasks

## Immediate Track (This Week)
- [x] Define transactional `opmode=configure` and `opmode=commit` behavior.
- [x] Add minimal admin API endpoints for configure, commit, and state.
- [x] Add CLI client commands for configure, commit, and state.
- [x] Add Ubuntu 24.04 + `nftables` baseline deployment artifacts.
- [ ] Add identity validation middleware (JWT + API key) to admin API.
- [ ] Add signed policy bundle generation and verification.
- [ ] Add per-agent rate-limit enforcement service logic.
- [ ] Add immutable audit sink abstraction (file + DB backend).
- [ ] Add `set` and `show` command grammar with candidate config file support.
- [ ] Add `install` command plan/output for live-media to disk workflow.

## Sprint 1 (Weeks 1-3)
- [ ] Service config schema + strict validation.
- [ ] SQLite/PostgreSQL persistence layer for identity/policy/quota.
- [ ] API endpoints for agent CRUD.
- [ ] API endpoints for policy CRUD + publish workflow.
- [ ] API endpoints for quota CRUD.
- [ ] Unit test baseline for opmode and API handlers.

## Sprint 2 (Weeks 4-6)
- [ ] Gateway request path skeleton: identity -> policy -> quota -> decision.
- [ ] nftables set/map generation from active policy.
- [ ] Transparent gateway mode bootstrap on Ubuntu 24.04.
- [ ] Decision event schema validation and replay tests.
- [ ] Build SquashFS root image and bootable LiveCD ISO pipeline.

## Sprint 3 (Weeks 7-9)
- [ ] Basic operator UI with agent/policy/decision views.
- [ ] RPS/RPM enforcement under concurrent load.
- [ ] Policy conflict detection and deterministic ordering tests.

## Sprint 4 (Weeks 10-12)
- [ ] Hardening: mTLS, AppArmor profile templates, fail mode tests.
- [ ] Performance tuning for p50/p95 latency targets.
- [ ] Ubuntu 24.04 deployment runbook and release candidate.
- [ ] Validate end-to-end live boot -> configure/commit -> install-to-disk workflow.
