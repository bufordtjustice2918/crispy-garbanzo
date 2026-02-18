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
- `set interfaces ethernet <ifname> address dhcp`
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
- Imported/expanded via: `scripts/import_command_schema.py`

CLI introspection:
- `clawgressctl show commands`
- Current schema size: `5220` set paths, `14` show paths.

Path model:
- `tokens[]` define fixed/variable command tree segments.
- `value_token` defines terminal value grammar for `set` commands.
- Hyphenated tokens (for example `host-name`) are normalized to internal underscore paths for backend storage.

## Backend Resolution
- Every command pattern in `internal/cmdmap/command_schema.json` is imported into the in-memory command model (`internal/cmdmap`).
- Backend mapping is resolved per pattern in `internal/cmdmap/catalog.go`.
- Current backend families: `nftables`, `bind9`, `haproxy`, `chrony`, `systemd-networkd`, `dhclient+systemd`, `openssh-server`, `systemd-unit`, `clawgress-policyd`, `control-daemon`, `control-plane`.

## Commit-Time Operation Plan
- The commit API now returns `ops_plan`, a backend action plan derived from staged commands.
- Commit request supports `ops_mode`:
  - `dry-run` (default): validate/plan only, no OS commands executed.
  - `apply`: execute allowed `write`, `systemctl`, `ip`, and `nft` actions transactionally.
- Example for `set interfaces ethernet eth0 address dhcp`:
  - write `/run/dhclient/dhclient_eth0.conf`
  - write `/run/systemd/system/dhclient@eth0.service.d/10-override.conf`
  - `systemctl daemon-reload`
  - `systemctl restart dhclient@eth0.service`
- Additional family mappings in commit plan:
  - `firewall` + `nat` -> render/validate/apply `nftables` snippets
  - `policy` -> render policy JSON + reload `clawgress-policyd`
  - `service <name>` (unmodeled families) -> render service JSON + `systemctl reload-or-restart <unit>`
  - root family fallback -> render `/etc/clawgress/rendered/<family>.json` + reload `clawgress-configd`

## CLI Behavior
- Candidate configuration is updated using `set` commands.
- `show configuration commands` renders deterministic `set ...` output.
- `configure` stages candidate into control-plane revision.
- `commit` atomically activates the staged revision.

## Validation
- Exhaustive schema parser validation: `GOCACHE=/tmp/go-build-cache go test ./...`
- Key exhaustive tests:
  - `internal/cmdmap/schema_test.go` validates every `set` token path/value pair.
  - `cmd/clawgressctl/parser_test.go` validates CLI `set` parsing for every schema command.

## Non-Goals
- No BGP/OSPF/MPLS routing command families in MVP.
- Scope is egress firewall + NAT + service control for appliance mode.
