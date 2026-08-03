# Testing

AgentMesh uses Go unit tests, shell smoke tests, and documented chaos scenarios.

## Unit tests

Run the full suite:

```bash
make test
# or
GOTOOLCHAIN=go1.22.5 go test ./...
```

### Key packages

| Package | File | Focus |
|---------|------|-------|
| `internal/router` | `engine_test.go` | Strategies, filtering, score-based ordering |
| `internal/auth` | `auth_test.go` | Bearer parsing, org IDs, invalid keys |
| `adapters` | `register_test.go` | All nine agent types register cleanly |

Static analysis:

```bash
make lint   # go vet ./...
```

## Integration (local)

See **[21-MANUAL-TESTING.md](21-MANUAL-TESTING.md)** for a full step-by-step manual QA guide (stack, seed, API, UI, checklist).

Quick smoke:

1. `./scripts/start-all.sh` — builds and starts compose stack.
2. Export `Authorization: Bearer agentmesh-dev-key` and `X-Org-ID: default`.
3. Hit `/health`, `./scripts/seed-all-agents.sh`, discover, route.

## Load smoke

```bash
./tests/load/smoke.sh
```

Adjust endpoints via environment variables documented in the script header. Validates happy-path latency under minimal concurrency.

## Chaos scenarios

Documented in `tests/chaos/README.md`:

| Scenario | Expected behavior |
|----------|-------------------|
| etcd partition (port 2379 blocked) | Register fails gracefully; reads may degrade |
| Redis unavailable | Circuit/rate limit errors; routing may fail closed or bypass per config |
| GuardLoop timeout | `score-based` falls back to latency/cost ranking |

Run chaos tests in staging only—never against production etcd without maintenance windows.

## OpenAPI contract

```bash
make verify-openapi
```

Ensures `docs/openapi.yml` exists for CI gates.

## Frontend

From `frontend/`:

```bash
npm install && npm run build
```

Typecheck via `tsconfig.json`; pages under `app/` are smoke-tested in CI when enabled.

## Coverage goals

- Router strategies: 100% branch coverage for ranking functions
- Auth: all exported error paths
- Circuit breaker: Lua script behavior validated via integration tests (planned)

## Reporting failures

Include org ID (redacted), routing strategy, TraceLens trace ID, and circuit state from `GET /v1/circuit/{agent_id}`.
