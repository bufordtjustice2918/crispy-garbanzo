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

### Test Maintenance
- When adding a feature: add unit + smoke + acceptance tests in the same commit
- When removing a feature: remove its tests in the same commit
- When modifying a feature: update all three test tiers to match new behavior
- The test suite is extensive — keep it in sync with the codebase, don't let tests go stale
- Run `go test ./... -count=1` locally before pushing to catch regressions early

### Advanced Testing
- **Fuzz tests** (`go test -fuzz`): security-critical parsers (JWT, policy eval, auth extraction)
- **Adversarial tests**: bypass attempts (encoded domains, null bytes, header injection)
- **Load tests**: concurrent connection benchmarks with `-race` flag
- **Chaos tests**: service death, disk full, corruption recovery
- **API contract tests**: JSON schema validation, backward compatibility
- **Security scanning**: `govulncheck` + `gosec` in CI

## CI Pipeline

- `e2e.yml`: Go build + unit tests + command conformance
- `build-iso.yml`: ISO build → QEMU/KVM boot → full e2e smoke + acceptance suite
- All tests must pass before merge

## Code Standards

- `go build ./...` clean
- `go vet ./...` clean  
- `gofmt` formatted
- No unused imports or variables
