# Analytics

AgentMesh analytics combine ClickHouse historical rollups, Prometheus real-time metrics, and TraceLens distributed traces.

## ClickHouse

### Ingest

The analytics store inserts:

- **Route events** after proxy completion (`routes` table)
- **Agent metric updates** for EWMA latency and success rate (`agent_metrics`)
- **Billing events** for Stripe alignment (`billing_events`)

Schema details: [16-DB-SCHEMA.md](16-DB-SCHEMA.md).

### Typical dashboards

| Question | Query pattern |
|----------|---------------|
| Routes per strategy | `GROUP BY routing_strategy` on `routes` |
| p95 latency by agent_type | Join `routes` with latest `agent_metrics` |
| Spend estimate | `sum(token_cost_usd)` by org/day |
| Circuit impact | Filter `success = 0` spikes by `destination_agent_id` |

Use `LowCardinality` columns in filters for performant scans.

## Prometheus

Exported from `internal/telemetry`:

| Metric | Labels | Meaning |
|--------|--------|---------|
| `agentmesh_route_total` | `org`, `strategy`, `status` | Route counter |
| `agentmesh_route_latency_ms` | `org`, `mode` | Histogram of decision/proxy latency |

Scrape control plane and data plane pods; sample config in `deploy/prometheus.yml`.

## TraceLens (not Jaeger)

Traces export via OpenTelemetry **OTLP HTTP** to:

- Endpoint: `TRACELENS_OTLP_ENDPOINT` (host default `http://localhost:48082/v1/traces` when using Compose profile)
- Auth: `Authorization: Bearer TRACELENS_JWT`

Span name `route` includes attributes:

- `agentmesh.org_id`
- `agentmesh.source_agent`
- `agentmesh.destination_agent`
- `agentmesh.routing_strategy`

Correlate with ClickHouse using `trace_id` on route events.

## Frontend

Next.js pages (`/routes`, `/circuits`) visualize recent routing and breaker states—backed by control plane APIs in dev; production may query CH read replicas.

## Feature flags

Toggle experimental analytics via `FF_*` env vars (see [ops/05-FEATURE-FLAGS.md](../ops/05-FEATURE-FLAGS.md)).

## Privacy

Avoid storing prompt content in ClickHouse; `payload_hint` on gRPC is not persisted by default analytics path.

## Alerting

See `ops/monitoring/prometheus-rules.yml` for SLO-oriented alerts on error rate and latency.
