# Clawgress

Agent-Aware Egress Governance Gateway.

## Docs
- `docs/SPEC.md` - Product specification
- `docs/ARCHITECTURE.md` - Architecture draft
- `docs/MVPv1.md` - MVP v1 draft
- `docs/INITIAL_TASKS.md` - Initial implementation task board

## Base Platform
This project is being planned with an Ubuntu 24.04 LTS baseline.

## Quickstart (Current Bootstrap)
Start the admin API:

```bash
go run ./cmd/clawgress-admin-api
```

In a second terminal, stage and commit configuration transactionally:

```bash
go run ./cmd/clawgressctl configure --file examples/configure.sample.json --actor kavansmith
go run ./cmd/clawgressctl state
go run ./cmd/clawgressctl commit --actor kavansmith
go run ./cmd/clawgressctl state
```

Appliance-style candidate config workflow:

```bash
go run ./cmd/clawgressctl configure --actor kavansmith
# then inside configure mode:
# set system host-name clawgress-gw
# set policy egress default-action deny
# show configuration commands
# commit
# exit
```

Live-media install planning command:

```bash
go run ./cmd/clawgressctl install --target-disk /dev/sda --hostname clawgress-gw --yes
```

## nftables Baseline
MVPv1 requires `nftables` for transparent gateway enforcement.

```bash
sudo nft -f deploy/nftables/clawgress.nft
sudo nft list table inet clawgress
```

## LiveCD ISO Workflow
Ubuntu 24.04 LiveCD ISO build workflow and assets:
- Workflow: `.github/workflows/build-iso.yml`
- Build script: `build/iso/scripts/build-livecd.sh`
- Boot test script: `build/iso/scripts/boot-test.sh`
- Details: `docs/ISO_WORKFLOW.md`

Default appliance login in current live image builds:
- Username: `clawgress`
- Password: `clawgress`

## Appliance Command Model
- Command mapping reference: `docs/APPLIANCE_COMMAND_MAPPING.md`
- Token path catalog (source of truth): `internal/cmdmap/command_schema.json`

## End-to-End Testing
- Workflow: `.github/workflows/e2e.yml`
- Test details: `docs/E2E_TESTING.md`
- Includes mandatory ISO boot validation in QEMU on runner
