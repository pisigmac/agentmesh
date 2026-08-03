# Database schema (ClickHouse)

Schema is created automatically on control plane / data plane startup via `internal/analytics/clickhouse.go` `initSchema`.

## Database

Default database name: **`agentmesh`** (`CLICKHOUSE_DATABASE`).

## Table: `routes`

MergeTree fact table for individual routing/proxy outcomes.

| Column | Type | Description |
|--------|------|-------------|
| `org_id` | LowCardinality(String) | Tenant |
| `trace_id` | String | Correlates with TraceLens |
| `source_agent_id` | String | Caller agent |
| `destination_agent_id` | String | Selected agent |
| `routing_strategy` | LowCardinality(String) | e.g. `least-latency` |
| `latency_ms` | UInt32 | End-to-end or segment latency |
| `token_cost_usd` | Float64 | Estimated cost |
| `success` | UInt8 | 1 success, 0 failure |
| `retry_count` | UInt8 | Retries attempted |
| `recorded_at` | DateTime64(3) | Event timestamp |

**Engine:** `MergeTree()`  
**Partition:** `toStartOfDay(recorded_at)`  
**Order:** `(org_id, recorded_at, trace_id)`

### Example queries

Daily success rate by org:

```sql
SELECT org_id,
       countIf(success = 1) / count() AS success_rate
FROM routes
WHERE recorded_at >= now() - INTERVAL 7 DAY
GROUP BY org_id;
```

## Table: `agent_metrics`

ReplacingMergeTree for latest EWMA metrics per agent.

| Column | Type | Description |
|--------|------|-------------|
| `org_id` | LowCardinality(String) | Tenant |
| `agent_id` | String | Agent identifier |
| `agent_type` | LowCardinality(String) | Adapter type |
| `latency_ewma_ms` | Float64 | EWMA latency |
| `success_rate` | Float64 | Rolling success |
| `updated_at` | DateTime64(3) | Version column |

**Engine:** `ReplacingMergeTree(updated_at)`  
**Order:** `(org_id, agent_id)`

Router reads these values to rank `least-latency` candidates.

## Table: `billing_events`

MergeTree ledger for Stripe-aligned metering.

| Column | Type | Description |
|--------|------|-------------|
| `org_id` | LowCardinality(String) | Tenant |
| `event_type` | LowCardinality(String) | e.g. `route`, `seat` |
| `quantity` | Float64 | Billable units |
| `stripe_event_id` | String | Idempotency |
| `recorded_at` | DateTime64(3) | Event time |

**Order:** `(org_id, recorded_at)`

## etcd (registry)

Not ClickHouse—documented for completeness. Keys under `ETCD_PREFIX/{org_id}/agents/{agent_id}` store JSON agent records with lease TTL.

## Redis (ephemeral)

- Circuit: sorted sets + state keys `...:cb:{org}:{dest}`
- Rate limit: counters `...:rl:{org}:{agent}`

## etcd: org policy (config-driven)

Key: `{ETCD_PREFIX}/orgs/{org_id}/policy` — JSON document (`OrgPolicy`).

| Section | Purpose |
|---------|---------|
| `routing` | Default strategy, timeout, retries, max cost |
| `rate_limit` | Routes/min per org, per source agent, registrations/min |
| `circuit.default` | Global breaker thresholds |
| `circuit.by_agent_type` | Overrides per agent type (e.g. `cursor`) |
| `circuit.by_destination_agent` | Overrides per destination agent id |

Defaults merge with env bootstrap (`CIRCUIT_*`) when no etcd document exists.

Example: [deploy/examples/org-policy-default.json](../deploy/examples/org-policy-default.json)

REST: `GET/PUT /v1/policy` (authenticated, org-scoped).

Watch: control plane caches policy and invalidates on etcd watch.

## Retention

Operational policy: [ops/10-DATA-RETENTION.md](../ops/10-DATA-RETENTION.md).

## DSN helper

`analytics.DSN(cfg)` builds the ClickHouse DSN from config for tooling.
