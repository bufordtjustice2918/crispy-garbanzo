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
go run ./cmd/clawgressctl set gateway.mode explicit_proxy
go run ./cmd/clawgressctl set quotas.agent_alpha.rps_limit 10
go run ./cmd/clawgressctl show
go run ./cmd/clawgressctl configure --file candidate.json --actor kavansmith
go run ./cmd/clawgressctl commit --actor kavansmith
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
- Details: `docs/ISO_WORKFLOW.md`

## VyOS-Style Command Model
- Command mapping reference: `docs/VYOS_COMMAND_MAPPING.md`
- Token path catalog (source of truth): `internal/cmdmap/token_paths.go`
