# End-to-End Testing (MVPv1)

## Workflow
- GitHub Actions workflow: `.github/workflows/e2e.yml`
- Job: `mvpv1-e2e`

## What It Validates
1. Control-plane transactional behavior
- Appliance-style candidate edits via `set`
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

3. Command conformance (mapped command catalog)
- Executes every mapped `set` command path from schema.
- Fails if any mapped command is rejected.

Scripts:
- `tests/e2e/test_command_conformance.sh`
- `tests/e2e/command_conformance.py`

Report artifact:
- `tests/e2e/out/command-conformance.json`

4. Full device boot validation (mandatory)
- Builds Ubuntu 24.04 LiveCD ISO in runner.
- Boots ISO in QEMU on runner.
- Requires live self-test pass marker from booted system.

Scripts:
- `build/iso/scripts/build-livecd.sh`
- `build/iso/scripts/boot-test.sh`

## Local Run
```bash
./tests/e2e/test_control_plane.sh
sudo ./tests/e2e/test_networking.sh
```

## CI Requirements
- Ubuntu 24.04 runner
- Root privileges for netns + nftables tests
- `nftables`, `iproute2`, `conntrack`, `python3`, `curl`, `jq`
- `live-build`, `qemu-system-x86` for full ISO boot validation
