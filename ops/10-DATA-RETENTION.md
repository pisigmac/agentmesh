# Data retention

Policies for ClickHouse analytics, Redis ephemeral state, etcd registry, and TraceLens traces.

## ClickHouse

| Table | Retention | Action |
|-------|-----------|--------|
| `routes` | 90 days hot | TTL or partition drop `ALTER TABLE ... DROP PARTITION` |
| `agent_metrics` | 180 days | ReplacingMergeTree; periodic optimize |
| `billing_events` | 7 years | Legal/finance requirement; cold storage after 1 year |

Implement TTL example:

```sql
ALTER TABLE routes MODIFY TTL recorded_at + INTERVAL 90 DAY;
```

## etcd

- Agent keys expire with lease TTL automatically
- Audit key history not retained—export snapshots daily for DR only

## Redis

- Circuit and rate-limit keys expire via TTL (window + buffer)
- No backup required; state rebuilds from traffic

## TraceLens

- Default trace retention: **14 days** in TraceLens project settings
- JWT-scoped projects per environment (staging vs prod)

AgentMesh does not store full traces locally—only `trace_id` in ClickHouse.

## Logs

- Application logs: 30 days in centralized logging
- PII scrubbing at ingest (no prompt bodies)

## Backups

| Store | Frequency | RPO |
|-------|-----------|-----|
| ClickHouse | Daily incremental | 24h |
| etcd | Hourly snapshot | 1h |

## GDPR / deletion

On org delete request:

1. Revoke API keys
2. Delete etcd prefix `/agentmesh/{org_id}/`
3. `ALTER TABLE ... DELETE WHERE org_id = ?` on CH tables (mutations)
4. Request TraceLens project purge by org tag

## Staging

Shorter TTL: routes 7 days, traces 3 days.

## Compliance review

Quarterly audit sample: verify no raw prompts in `routes` or logs.
