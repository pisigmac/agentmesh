# Features

AgentMesh delivers production routing for heterogeneous coding agents with observability and guardrails built in.

## Agent registry

- **etcd-backed** registration with lease TTL and automatic expiry when agents stop heartbeating.
- **Nine adapter types** aligned with GuardLoop: `cursor`, `claude_code`, `github_copilot`, `openai_codex`, `aider`, `continue_dev`, `windsurf`, `devin`, `custom`.
- Capability tags (`code_gen`, `refactor`, `completion`, `browser_verify`, etc.) for discovery filters.
- Per-agent **cost profiles** (per-request and per-1k-token USD) used in `least-cost` routing.

## Routing strategies

| Strategy | Behavior |
|----------|----------|
| `capability-match` | Default; ranks agents that expose the requested capability |
| `least-latency` | Prefers agents with lower EWMA latency from ClickHouse |
| `least-cost` | Minimizes estimated spend from cost profiles |
| `round-robin` | Distributes load across healthy candidates |
| `score-based` | Uses optional **GuardLoop** scores when enabled |

When GuardLoop is unavailable or times out, `score-based` degrades to latency/cost ranking (see chaos tests).

## Resilience

- **Redis circuit breaker** per `(org_id, destination_agent_id)` with sliding-window failures, open/half-open/closed states.
- **Rate limits** via Redis INCR + TTL windows per org and agent.
- Configurable retry counts and proxy timeouts on the data plane.

## API surfaces

- **REST** on control plane HTTP port: register, discover, route, circuit state.
- **gRPC** `AgentMesh` service for internal and high-throughput clients.
- **Auth**: `Authorization: Bearer <api_key>` plus `X-Org-ID` or `X-Workspace-ID`.

## Analytics

- ClickHouse tables for route events, agent metric rollups, and billing events.
- Prometheus metrics: `agentmesh_route_total`, `agentmesh_route_latency_ms`.

## Tracing

- **TraceLens** integration: OTLP over HTTP JSON to `/v1/traces` with optional JWT bearer on the exporter — **not** Jaeger.
- Route spans include org, source, destination, and strategy attributes.

## Operator UI

- Next.js frontend for routes, circuits, and settings (development/staging).

## Deployment

- Docker Compose for local stacks; **Helm chart** under `deploy/helm/agentmesh` with **HPA** and **PDB** for the control plane.

## Optional integrations

- **GuardLoop** for score-based routing and policy alignment.
- **Stripe** billing hooks (see `internal/billing` and ops payment runbook).
