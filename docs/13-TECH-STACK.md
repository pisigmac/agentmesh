# Technology stack

## Languages and runtimes

| Layer | Technology | Version / notes |
|-------|------------|-----------------|
| Control plane | Go | 1.22 (`go.mod`, toolchain go1.22.5) |
| Data plane | Go | 1.22 |
| Frontend | TypeScript / Next.js | See `frontend/package.json` |
| Protobuf | proto3 | `api/proto/agentmesh/v1` |

## Core dependencies (Go)

| Library | Use |
|---------|-----|
| `go.etcd.io/etcd/client/v3` | Agent registry |
| `github.com/redis/go-redis/v9` | Circuit breaker + rate limits |
| `github.com/ClickHouse/clickhouse-go/v2` | Analytics inserts |
| `google.golang.org/grpc` | Control plane RPC |
| `github.com/go-chi/chi/v5` | REST routing |
| `go.opentelemetry.io/otel` + `otlptracehttp` | TraceLens export |
| `github.com/prometheus/client_golang` | Metrics |

## Infrastructure services

| Service | Role |
|---------|------|
| etcd 3.5 | Strongly consistent registry |
| Redis 7 | Ephemeral breaker/limit state |
| ClickHouse | MergeTree rollups for routes and metrics |
| TraceLens | Trace ingestion via OTLP HTTP JSON `POST /v1/traces` + JWT |

## Optional services

| Service | Role |
|---------|------|
| GuardLoop | External scoring for `score-based` routing |
| Stripe | Billing (`internal/billing/stripe.go`) |

## Build and codegen

- `make proto` — protoc with go + go-grpc plugins
- `buf.yaml` / `buf.gen.yaml` — Buf config for proto lint/generate
- Docker multi-stage builds in `deploy/Dockerfile.*`

## Observability stack (recommended)

- Prometheus scrapes control/data plane metrics
- Grafana dashboard: `ops/monitoring/grafana-agentmesh.json`
- Alert rules: `ops/monitoring/prometheus-rules.yml`

## Explicit non-stack

- **Jaeger** is not a supported trace backend; use TraceLens OTLP endpoint instead.
- AgentMesh does not embed model inference runtimes—it routes HTTP to external agents.

## Frontend stack

- Next.js App Router (`frontend/app/`)
- Tailwind/globals in `globals.css`
- Talks to control plane REST API with org headers

## Security

- API keys hashed (SHA-256) in memory store; production should use durable keystore
- TLS termination at ingress (Helm/Kubernetes), not enforced in default compose
