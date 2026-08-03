# AgentMesh

AgentMesh is a multi-agent routing mesh for development and automation agents (Cursor, Claude Code, Copilot, and six other adapter types). It provides a **Go 1.22** control plane and data plane that register agents in **etcd**, enforce **Redis**-backed circuit breakers and rate limits, and persist routing rollups in **ClickHouse**.

## Why AgentMesh

Organizations run many coding agents across teams. AgentMesh centralizes discovery, routing, health, and cost-aware selection so orchestrators do not hard-code endpoints or retry logic per agent vendor.

## Quick start

```bash
make docker-up          # etcd, Redis, ClickHouse, control-plane, dataplane
export AGENTMESH_API_KEY=dev
curl -s http://localhost:48080/health
```

Register an agent:

```bash
curl -s -X POST http://localhost:48080/v1/agents/register \
  -H "Authorization: Bearer dev" \
  -H "X-Org-ID: default" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"cursor-1","agent_type":"cursor","endpoint_url":"http://localhost:9001","capabilities":["code_gen"]}'
```

Route a request:

```bash
curl -s -X POST http://localhost:48080/v1/route \
  -H "Authorization: Bearer dev" \
  -H "X-Org-ID: default" \
  -H "Content-Type: application/json" \
  -d '{"capability":"code_gen","routing_strategy":"least-latency"}'
```

## Components

| Component | Role |
|-----------|------|
| Control plane | gRPC core + REST gateway (`/v1/*`) |
| Data plane | Gateway proxy, TraceLens spans, route cache |
| etcd | Agent registry with leases |
| Redis | Circuit breaker state, rate limit counters |
| ClickHouse | `routes`, `agent_metrics`, `billing_events` |
| TraceLens | OTLP HTTP traces (`POST /v1/traces`, JWT) — not Jaeger |

## Authentication

Send `Authorization: Bearer <api_key>` on every `/v1/*` call. Scope tenants with `X-Org-ID` or `X-Workspace-ID` (workspace acts as org alias when org is omitted).

## Documentation map

| Doc | Topic |
|-----|-------|
| [02-FEATURES.md](02-FEATURES.md) | Product capabilities |
| [05-API.md](05-API.md) | REST usage |
| [21-MANUAL-TESTING.md](21-MANUAL-TESTING.md) | Step-by-step manual QA |
| [12-ARCHITECTURE.md](12-ARCHITECTURE.md) | System design |
| [15-DEPLOY.md](15-DEPLOY.md) | Helm / Docker |

## Repository layout

- `cmd/control-plane`, `cmd/dataplane` — binaries
- `internal/router`, `internal/registry`, `internal/circuit`
- `adapters/` — nine GuardLoop-aligned agent types
- `deploy/helm/agentmesh` — HPA, PDB, values
- `frontend/` — Next.js operator UI

## License

Proprietary — see your enterprise agreement.
