# Appliance Command Mapping (MVP Scope)

This project maps operator-facing commands to an appliance-style `set` and `show` workflow for the egress firewall scope.

The focus is command semantics and transactional behavior (`configure` -> `commit`) for Ubuntu-based appliance runtime.

## Control Plane Runtime Baseline
- Control plane and data-plane daemons run as `systemd` units.
- Time sync uses NTP service configuration (`chrony`/NTP stack) via command model.
- Firewall/NAT enforcement backend is `nftables`.
- Supporting services: `bind9`, `haproxy`.

## Supported `set` Commands (MVP)
- `set system host-name <name>`
- `set system ntp server <server>`
- `set interfaces ethernet <ifname> address <cidr>`
- `set interfaces ethernet <ifname> role <lan|wan>`
- `set firewall nftables input default-action <accept|drop>`
- `set firewall nftables forward default-action <accept|drop>`
- `set firewall nftables wan-block enable`
- `set firewall group address-group <name> address <cidr>`
- `set nat source rule <id> outbound-interface <ifname>`
- `set nat source rule <id> source address <cidr>`
- `set nat source rule <id> translation address masquerade`
- `set service dns forwarding listen-address <ip>`
- `set service dns forwarding allow-from <cidr>`
- `set service haproxy enable`
- `set service haproxy stats port <port>`
- `set policy egress default-action <allow|deny>`
- `set policy egress allow-domain <fqdn>`
- `set policy egress deny-domain <fqdn>`

## Supported `show` Commands (MVP)
- `show commands`
- `show configuration`
- `show configuration commands`
- `show system ntp`
- `show interfaces`
- `show firewall`
- `show nat source rules`
- `show service dns`
- `show service haproxy`

## Canonical Token Paths
Canonical token-path definitions are versioned in code:
- `internal/cmdmap/command_schema.json`

CLI introspection:
- `clawgressctl show commands`

Path model:
- `tokens[]` define fixed/variable command tree segments.
- `value_token` defines terminal value grammar for `set` commands.
- Hyphenated tokens (for example `host-name`) are normalized to internal underscore paths for backend storage.

## CLI Behavior
- Candidate configuration is updated using `set` commands.
- `show configuration commands` renders deterministic `set ...` output.
- `configure` stages candidate into control-plane revision.
- `commit` atomically activates the staged revision.

## Non-Goals
- No BGP/OSPF/MPLS routing command families in MVP.
- Scope is egress firewall + NAT + service control for appliance mode.
