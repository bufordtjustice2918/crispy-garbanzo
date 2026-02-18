# End-to-End Testing (MVPv1)

## Workflow
- GitHub Actions workflow: `.github/workflows/e2e.yml`
- Job: `mvpv1-e2e`

## What It Validates
1. Control-plane transactional behavior
- VyOS-style candidate edits via `set`
- `show configuration commands`
- `configure` stage operation
- `commit` activation operation
- Post-commit state integrity

Script:
- `tests/e2e/test_control_plane.sh`
Note: this script sets `CLAWGRESS_NFT_APPLY=false` because it validates control-plane flow without requiring root nft mutation.

2. Network dataplane behavior
- Source NAT masquerade for LAN -> WAN path
- Firewall forward default-deny with selective allow
- WAN-to-LAN block verification

Script:
- `tests/e2e/test_networking.sh`

## Local Run
```bash
./tests/e2e/test_control_plane.sh
sudo ./tests/e2e/test_networking.sh
```

## CI Requirements
- Ubuntu 24.04 runner
- Root privileges for netns + nftables tests
- `nftables`, `iproute2`, `conntrack`, `python3`, `curl`, `jq`
