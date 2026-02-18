#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "run as root (use sudo)" >&2
  exit 1
fi

LAN_NS="clg_lan"
WAN_NS="clg_wan"
LAN_HOST_IF="clg_lan_h"
LAN_NS_IF="clg_lan_n"
WAN_HOST_IF="clg_wan_h"
WAN_NS_IF="clg_wan_n"
BACKUP_RULESET="$(mktemp)"

cleanup() {
  set +e
  [[ -n "${SERVER_PID:-}" ]] && kill "${SERVER_PID}" 2>/dev/null || true
  ip netns del "${LAN_NS}" 2>/dev/null || true
  ip netns del "${WAN_NS}" 2>/dev/null || true
  ip link del "${LAN_HOST_IF}" 2>/dev/null || true
  ip link del "${WAN_HOST_IF}" 2>/dev/null || true
  if [[ -f "${BACKUP_RULESET}" ]]; then
    nft -f "${BACKUP_RULESET}" >/dev/null 2>&1 || true
    rm -f "${BACKUP_RULESET}"
  fi
  if [[ -n "${OLD_IP_FORWARD:-}" ]]; then
    sysctl -w net.ipv4.ip_forward="${OLD_IP_FORWARD}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

nft list ruleset > "${BACKUP_RULESET}"
OLD_IP_FORWARD="$(sysctl -n net.ipv4.ip_forward)"
sysctl -w net.ipv4.ip_forward=1 >/dev/null

ip netns add "${LAN_NS}"
ip netns add "${WAN_NS}"

ip link add "${LAN_HOST_IF}" type veth peer name "${LAN_NS_IF}"
ip link add "${WAN_HOST_IF}" type veth peer name "${WAN_NS_IF}"

ip link set "${LAN_NS_IF}" netns "${LAN_NS}"
ip link set "${WAN_NS_IF}" netns "${WAN_NS}"

ip addr add 10.10.0.1/24 dev "${LAN_HOST_IF}"
ip addr add 172.16.0.1/24 dev "${WAN_HOST_IF}"
ip link set "${LAN_HOST_IF}" up
ip link set "${WAN_HOST_IF}" up

ip netns exec "${LAN_NS}" ip addr add 10.10.0.2/24 dev "${LAN_NS_IF}"
ip netns exec "${LAN_NS}" ip link set lo up
ip netns exec "${LAN_NS}" ip link set "${LAN_NS_IF}" up
ip netns exec "${LAN_NS}" ip route add default via 10.10.0.1

ip netns exec "${WAN_NS}" ip addr add 172.16.0.2/24 dev "${WAN_NS_IF}"
ip netns exec "${WAN_NS}" ip link set lo up
ip netns exec "${WAN_NS}" ip link set "${WAN_NS_IF}" up
ip netns exec "${WAN_NS}" ip route add default via 172.16.0.1

nft delete table inet clawgress_e2e >/dev/null 2>&1 || true
nft delete table ip clawgress_e2e_nat >/dev/null 2>&1 || true

nft -f - <<NFT

table inet clawgress_e2e {
  chain forward {
    type filter hook forward priority 0; policy drop;
    ct state established,related accept
    iifname "${LAN_HOST_IF}" oifname "${WAN_HOST_IF}" tcp dport 18080 accept
  }
}

table ip clawgress_e2e_nat {
  chain postrouting {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "${WAN_HOST_IF}" ip saddr 10.10.0.0/24 masquerade
  }
}
NFT

ip netns exec "${WAN_NS}" python3 - <<'PY' >/tmp/clawgress-e2e-server.log 2>&1 &
import socket
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("172.16.0.2", 18080))
s.listen(1)
conn, addr = s.accept()
conn.recv(4096)
body = addr[0].encode()
resp = b"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: " + str(len(body)).encode() + b"\r\n\r\n" + body
conn.sendall(resp)
conn.close()
s.close()
PY
SERVER_PID=$!

sleep 0.5

SRC_IP="$(ip netns exec "${LAN_NS}" curl -fsS --max-time 5 http://172.16.0.2:18080)"
if [[ "${SRC_IP}" != "172.16.0.1" ]]; then
  echo "NAT test failed: expected source 172.16.0.1, got ${SRC_IP}" >&2
  exit 1
fi

echo "NAT test: PASS"

if ip netns exec "${WAN_NS}" ping -c 1 -W 1 10.10.0.2 >/dev/null 2>&1; then
  echo "Firewall test failed: WAN->LAN ping unexpectedly succeeded" >&2
  exit 1
fi

echo "Firewall block test: PASS"
