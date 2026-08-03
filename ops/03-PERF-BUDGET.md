# Performance budget

Target budgets for AgentMesh 0.1.x at Growth tier scale (~500 RPS aggregate).

## Control plane

| Path | p50 | p99 | Notes |
|------|-----|-----|-------|
| `POST /v1/route` | 8 ms | 40 ms | Excludes GuardLoop |
| `POST /v1/route` + GuardLoop | 15 ms | 120 ms | Includes 5s timeout cap |
| `GET /v1/agents/discover` | 10 ms | 50 ms | etcd read + rank |
| `POST /v1/agents/register` | 20 ms | 80 ms | etcd lease write |

## Data plane

| Segment | p99 | Notes |
|---------|-----|-------|
| Route cache hit | 2 ms | `DATAPLANE_ROUTE_CACHE_TTL_MS=500` |
| Proxy TTFB | agent-specific | Not counted in mesh SLO |
| OTLP export batch | 50 ms async | Must not block response |

## Resource budgets (per control plane pod)

| Resource | Request | Limit |
|----------|---------|-------|
| CPU | 250m | 1000m |
| Memory | 256Mi | 512Mi |

HPA scales at 70% CPU (`deploy/helm/agentmesh/values.yaml`), max 20 replicas.

## Dependency SLAs

| Service | Max added p99 |
|---------|---------------|
| etcd | 5 ms read |
| Redis | 2 ms |
| ClickHouse insert | 10 ms async |

## Error budget

Monthly **99.9%** successful routes (HTTP 200 on `/v1/route` + proxy success). Burn alerts in Prometheus rules.

## Load testing

Run `tests/load/smoke.sh` in CI; full load test weekly in staging at 2× prod peak.

## Regression gates

PRs that increase router CPU >5% in bench or add sync CH writes on hot path require perf review.

## Frontend

Dashboard TTI < 3s on 4G—Next.js static optimization, no blocking CH queries in SSR.
