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

## nftables Baseline
MVPv1 requires `nftables` for transparent gateway enforcement.

```bash
sudo nft -f deploy/nftables/clawgress.nft
sudo nft list table inet clawgress
```
