# Clawgress Deployment Runbook

## Prerequisites

- Debian 12 (Bookworm) x86_64 host or the Clawgress LiveCD ISO
- 4 vCPU, 8 GB RAM, 80 GB SSD minimum
- Static IP or reserved DHCP lease
- NTP synchronized

## 1. Boot from ISO

Boot the Clawgress LiveCD ISO (UEFI or BIOS). The system auto-starts with:
- `clawgress-gateway` on `:3128` (proxy)
- `clawgress-admin-api` on `:8080` (admin + UI)
- `nftables`, `bind9`, `haproxy`, `ssh` enabled

Login: `clawgress` / `clawgress` (passwordless sudo).

## 2. Install to Disk (Optional)

```bash
sudo clawgressctl install --target-disk /dev/sda --hostname mygateway --yes
```

This partitions the disk (EFI + live-media + config), copies the squashfs image,
installs GRUB, and sets up persistent config overlay.

## 3. Configure Agents

```bash
# Via admin API
curl -X POST http://localhost:8080/v1/agents \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"my-agent","api_key":"secret-key","team_id":"ops","project_id":"prod","status":"active"}'

# Via CLI
clawgressctl configure --url http://localhost:8080
```

## 4. Configure Policies

```bash
curl -X POST http://localhost:8080/v1/policies \
  -H 'Content-Type: application/json' \
  -d '{"policy_id":"allow-api","agent_id":"my-agent","domains":["api.openai.com","api.anthropic.com"],"action":"allow"}'
```

Rules are first-match-wins. Use `GET /v1/policy/conflicts` to check for shadowed rules.

## 5. Configure Rate Limits

```bash
curl -X POST http://localhost:8080/v1/quotas \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"my-agent","rps":10,"rpm":500,"mode":"hard_stop"}'
```

Modes: `hard_stop` (429 reject) or `alert_only` (log + allow).

## 6. Point Agents at the Proxy

```bash
export HTTP_PROXY=http://my-agent:secret-key@gateway-ip:3128
export HTTPS_PROXY=http://my-agent:secret-key@gateway-ip:3128
```

Or use JWT auth:
```bash
TOKEN=$(clawgressctl token --agent my-agent --secret $JWT_SECRET --ttl 1h)
export HTTP_PROXY=http://gateway-ip:3128
# Add Proxy-Authorization: Bearer $TOKEN to requests
```

## 7. Monitor

- **Admin UI**: `http://gateway-ip:8080/ui/`
- **Audit log**: `clawgressctl show audit --limit 50`
- **Audit API**: `GET /v1/audit?agent_id=my-agent&limit=100`
- **Health**: `GET /healthz`

## 8. Operations

### Reload config (no restart)
Writes via admin API automatically SIGHUP the gateway. For manual reload:
```bash
sudo kill -HUP $(pidof clawgress-gateway)
```

### Check for policy conflicts
```bash
curl -s http://localhost:8080/v1/policy/conflicts | jq
```

### Sign and verify policy bundle
```bash
curl -s -X POST http://localhost:8080/v1/policy/sign | jq
```

### Render nftables rules from policy
```bash
curl -s http://localhost:8080/v1/nft/render
```

### View running config
```bash
curl -s http://localhost:8080/v1/config/validate | jq
```

## 9. Firewall

Default nftables allows: SSH (22), DNS (53), HTTP (80), HTTPS (443), proxy (3128), admin (8080), HAProxy stats (8404).

For transparent mode, render and apply:
```bash
curl -s 'http://localhost:8080/v1/nft/transparent?iface=eth1&subnet=10.0.0.0/24' | sudo nft -f -
```

## 10. Security Hardening

### AppArmor
```bash
sudo cp deploy/apparmor/clawgress-gateway /etc/apparmor.d/
sudo cp deploy/apparmor/clawgress-admin-api /etc/apparmor.d/
sudo apparmor_parser -r /etc/apparmor.d/clawgress-gateway
sudo apparmor_parser -r /etc/apparmor.d/clawgress-admin-api
```

### mTLS (admin API)
Set environment variables in the systemd unit:
```
CLAWGRESS_TLS_CERT=/etc/clawgress/tls/server.crt
CLAWGRESS_TLS_KEY=/etc/clawgress/tls/server.key
CLAWGRESS_TLS_CA=/etc/clawgress/tls/ca.crt
```

## 11. Troubleshooting

| Symptom | Check |
|---------|-------|
| 407 on all requests | Agent not registered or API key wrong |
| 403 on allowed domain | Policy order — check `/v1/policy/conflicts` |
| 429 unexpectedly | Quota too low — check `/v1/quotas/{agent}` |
| Gateway not starting | `journalctl -xeu clawgress-gateway` |
| Audit log empty | Check permissions on `/var/log/clawgress/` |
| Slow responses | Check `clawgressctl show audit --limit 10` for latency_ms |

## 12. Performance Reference

Benchmarked on i5-2400 (4 core, 3.1 GHz):

| Operation | Latency | Notes |
|-----------|---------|-------|
| Identity lookup | ~40 ns | Hash map, zero alloc |
| Policy evaluation | ~250 ns | 3-rule set, first-match |
| Quota check | ~400 ns | Token bucket with mutex |
| **Full request path** | **~690 ns** | Well under 5ms p50 target |
| Default-deny scan (100 rules) | ~530 ns | Worst case |
