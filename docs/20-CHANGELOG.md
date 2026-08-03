# Changelog

All notable changes to AgentMesh are documented here.

## [0.1.0] - 2026-08-02

### Added

- Go **1.22** control plane with gRPC (`9090`) and REST gateway (`8080`)
- Data plane gateway mode with route cache and proxy retries
- **etcd** agent registry with lease TTL
- **Redis** circuit breaker (sliding window) and per-org/agent rate limiting
- **ClickHouse** tables: `routes`, `agent_metrics`, `billing_events`
- Routing strategies: `capability-match`, `least-latency`, `least-cost`, `round-robin`, `score-based`
- Optional **GuardLoop** integration for score-based routing
- **TraceLens** OTLP HTTP exporter (`POST /v1/traces`) with JWT — not Jaeger
- Nine GuardLoop-aligned adapter types in `adapters/register.go`
- Auth: Bearer API key + `X-Org-ID` / `X-Workspace-ID`
- OpenAPI 3 spec at `docs/openapi.yml` for `/v1/agents/register`, discover, route, circuit
- Docker Compose stack and Helm chart `deploy/helm/agentmesh` with **HPA** and **PDB**
- Next.js operator UI skeleton (routes, circuits, settings)
- Prometheus metrics `agentmesh_route_total`, `agentmesh_route_latency_ms`
- Documentation set `docs/01`–`20` and ops runbooks `ops/01`–`10`

### Known limitations

- In-memory API key store for development only
- Data plane Helm template bundled in compose-first workflow; extend chart for full parity
- Clerk auth mode stubbed in config

## [Unreleased]

### Planned

- Persistent keystore and Clerk integration
- Data plane HPA template in Helm
- Webhook event delivery for circuit state changes

---

Format based on Keep a Changelog. Versioning follows SemVer for public API (`/v1`).
