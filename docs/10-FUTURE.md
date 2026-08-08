# Future roadmap

Planned enhancements beyond the current 0.1.x control/data plane split. Timelines are indicative, not commitments.

**Detailed market positioning, enhancement backlog (E1–E20), phases, and success metrics:** see root [`FUTURE_PLAN.md`](../FUTURE_PLAN.md).

## Q3 — Control plane hardening

- Native auth shipped (JWT sessions + org API keys + RBAC) — see [`FUTURE_PLAN.md`](../FUTURE_PLAN.md) E5
- Production soft spots (post Phase B): see [`FUTURE_PLAN.md` §7.1](../FUTURE_PLAN.md) — especially **E21** (`default` org isolation) and **E22** (ops/SLA maturity)
- Dataplane auth + Stripe webhooks/dunning (E11 follow-ups)
- Live `LatencyEWMA` writers from dataplane (E7 follow-up)
- mTLS between data plane and agent endpoints (opt-in)
- Optional later: password reset email, MFA; multi-chart Helm split

## Q4 — Routing intelligence

- Weighted multi-objective routing (latency + cost + GuardLoop score)
- Org-level routing policies stored in etcd
- Automatic promotion of half-open circuits based on canary traffic percentage
- Route decision cache invalidation webhooks

## Analytics and billing

- Pre-built ClickHouse materialized views for daily spend by `agent_type`
- Stripe usage records synced from `billing_events` with idempotency keys
- Self-serve usage dashboards in the Next.js UI (read-only CH API)

## Observability

- TraceLens correlation IDs propagated through `injected_headers`
- RED metrics per adapter type (`agentmesh_route_total{agent_type=...}`)
- SLO burn-rate alerts in `ops/monitoring/prometheus-rules.yml`

## Ecosystem

- Terraform module for Helm + external etcd/Redis
- Official Python and TypeScript SDKs generated from OpenAPI
- Webhook events for registration, circuit open/close (see `internal/events`)

## GuardLoop depth

- Bi-directional policy sync (deny-list capabilities per org)
- Score cache sharding in Redis for high QPS `score-based` routes

## Non-goals

- **Jaeger** as a first-class backend—we standardize on TraceLens OTLP HTTP JSON.
- Running LLM inference inside AgentMesh—we route to external agent endpoints only.

## How to propose changes

Open an RFC in your issue tracker referencing affected packages from [09-CODEMAP.md](09-CODEMAP.md). Breaking API changes require OpenAPI version bump and changelog entry in [20-CHANGELOG.md](20-CHANGELOG.md).
