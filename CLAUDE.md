# Clawgress Development Standards

## Testing Requirements

Every feature must include **three tiers of testing** before it ships:

### 1. Unit Tests (`go test ./internal/...`)
- Test the Go function/method in isolation
- Cover happy path, edge cases, and error conditions
- Located alongside the code in `*_test.go` files

### 2. Smoke Tests (API-level, in `test-iso-commands.py`)
- Verify the API endpoint returns correct status codes and JSON
- Test CRUD operations (create, read, update, delete)
- Verify field presence and types in responses

### 3. Behavioral Acceptance Tests (real traffic, in `test-iso-commands.py`)
- Prove the feature **actually works** with real traffic through the system
- Not just "does the API return JSON" but "does the system enforce the policy"
- Examples:
  - DNS RPZ: `dig blocked.domain` returns NXDOMAIN after RPZ is applied
  - Method filtering: POST through proxy returns 403 when only GET is allowed
  - Quota enforcement: rapid requests actually return 429 with "Too Many" in body
  - Audit correlation: specific proxy request generates audit event with correct fields
  - Hot-reload: change policy via API → next proxy request uses new policy

### Naming Convention
- Acceptance tests are prefixed with `ACCEPT-N:` in comments
- Each acceptance test documents what behavioral property it validates

## CI Pipeline

- `e2e.yml`: Go build + unit tests + command conformance
- `build-iso.yml`: ISO build → QEMU/KVM boot → full e2e smoke + acceptance suite
- All tests must pass before merge

## Code Standards

- `go build ./...` clean
- `go vet ./...` clean  
- `gofmt` formatted
- No unused imports or variables
