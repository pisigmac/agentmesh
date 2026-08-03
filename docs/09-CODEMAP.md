# Code map

Navigation guide for the AgentMesh repository (Go 1.22).

## Entry points

| Path | Purpose |
|------|---------|
| `cmd/control-plane/main.go` | Starts gRPC server, REST gateway, etcd/Redis/CH wiring |
| `cmd/dataplane/main.go` | Gateway proxy, TraceLens tracer, route cache |
| `cmd/agentmesh/main.go` | CLI umbrella binary |
| `Makefile` | `proto`, `build`, `test`, `docker-up`, `verify-openapi` |

## API contracts

| Path | Purpose |
|------|---------|
| `api/proto/agentmesh/v1/agentmesh.proto` | gRPC service definitions |
| `gen/agentmesh/v1/*.pb.go` | Generated stubs |
| `docs/openapi.yml` | REST mirror for `/v1/*` |

## Core packages

| Package | Responsibility |
|---------|----------------|
| `internal/registry` | etcd client, agent records, leases |
| `internal/router` | Ranking strategies including `score-based` |
| `internal/circuit` | Redis sliding-window breaker |
| `internal/ratelimit` | Redis INCR rate limits |
| `internal/analytics` | ClickHouse schema + inserts |
| `internal/rest` | chi router, auth middleware, gRPC dial |
| `internal/grpcserver` | gRPC method implementations |
| `internal/auth` | Bearer + org headers, KeyStore |
| `internal/config` | Env-based configuration |
| `internal/telemetry` | Prometheus + TraceLens OTLP HTTP |
| `internal/score/guardloop` | Optional GuardLoop client/bridge |
| `internal/billing` | Stripe integration |
| `internal/events` | Event dispatcher |
| `internal/featureflags` | `FF_*` env toggles |

## Adapters and SDK

| Path | Purpose |
|------|---------|
| `adapters/register.go` | Nine agent types + `Register()` helper |
| `pkg/agentmesh/client.go` | Go client for orchestrators |

## Frontend

| Path | Purpose |
|------|---------|
| `frontend/app/page.tsx` | Dashboard |
| `frontend/app/routes/page.tsx` | Route inspection |
| `frontend/app/circuits/page.tsx` | Circuit states |
| `frontend/app/settings/page.tsx` | Org settings |

## Deploy

| Path | Purpose |
|------|---------|
| `deploy/docker-compose.yml` | Local stack (etcd, Redis, CH, planes) |
| `deploy/Dockerfile.control-plane` | Control plane image |
| `deploy/Dockerfile.dataplane` | Data plane image |
| `deploy/helm/agentmesh/` | Chart, values, HPA, PDB templates |
| `deploy/prometheus.yml` | Sample scrape config |

## Tests

| Path | Purpose |
|------|---------|
| `internal/router/engine_test.go` | Strategy ranking |
| `internal/auth/auth_test.go` | Auth edge cases |
| `adapters/register_test.go` | Manifest coverage |
| `tests/load/smoke.sh` | HTTP smoke |
| `tests/chaos/README.md` | etcd partition, GuardLoop timeout |

## Documentation

All product docs live under `docs/01-*.md`–`20-*.md`; operations runbooks under `ops/`.
