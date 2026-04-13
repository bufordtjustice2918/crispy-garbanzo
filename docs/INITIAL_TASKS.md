# MVPv1 Initial Development Tasks

## Immediate Track (This Week)
- [x] Define transactional `opmode=configure` and `opmode=commit` behavior.
- [x] Add minimal admin API endpoints for configure, commit, and state.
- [x] Add CLI client commands for configure, commit, and state.
- [x] Add Debian Bookworm + `nftables` baseline deployment artifacts.
- [ ] Add identity validation middleware (JWT + API key) to admin API.
- [ ] Add signed policy bundle generation and verification.
- [x] Add per-agent rate-limit enforcement service logic.
- [x] Add immutable audit sink abstraction (file + DB backend).
- [x] Add `set` and `show` command grammar with candidate config file support.
- [ ] Add `install` command plan/output for live-media to disk workflow.

## Sprint 1 (Weeks 1-3)
- [x] Service config schema + strict validation.
- [x] SQLite persistence layer for identity/policy/quota.
- [x] API endpoints for agent CRUD.
- [x] API endpoints for policy CRUD + publish workflow.
- [x] API endpoints for quota CRUD.
- [x] Unit test baseline for identity, policy, quota, audit, config, store.

## Sprint 2 (Weeks 4-6)
- [ ] Gateway request path skeleton: identity -> policy -> quota -> decision.
- [ ] nftables set/map generation from active policy.
- [ ] Transparent gateway mode bootstrap on Debian Bookworm.
- [ ] Decision event schema validation and replay tests.
- [ ] Build SquashFS root image and bootable LiveCD ISO pipeline.

## Sprint 3 (Weeks 7-9)
- [ ] Basic operator UI with agent/policy/decision views.
- [ ] RPS/RPM enforcement under concurrent load.
- [ ] Policy conflict detection and deterministic ordering tests.

## Sprint 4 (Weeks 10-12)
- [ ] Hardening: mTLS, AppArmor profile templates, fail mode tests.
- [ ] Performance tuning for p50/p95 latency targets.
- [ ] Debian Bookworm deployment runbook and release candidate.
- [ ] Validate end-to-end live boot -> configure/commit -> install-to-disk workflow.
