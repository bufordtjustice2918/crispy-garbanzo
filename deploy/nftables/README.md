# nftables Baseline (Ubuntu 24.04)

This folder contains an MVPv1 baseline nftables policy for Clawgress transparent gateway mode.

## Apply
```bash
sudo nft -f deploy/nftables/clawgress.nft
```

## Validate
```bash
sudo nft list table inet clawgress
```

## Notes
- This is a starter baseline only.
- Production rules should be generated from active Clawgress policy commits.
- Keep rule updates atomic using nftables transactions (`nft -f`).
