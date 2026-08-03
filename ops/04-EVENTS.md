# Events

Internal event dispatch is handled by `internal/events/dispatcher.go`. Use events for webhooks, email triggers, and analytics side effects.

## Event types (canonical)

| Type | Emitted when | Consumers |
|------|--------------|-----------|
| `agent.registered` | Successful lease | Search index, audit log |
| `agent.expired` | Lease TTL elapsed | Cleanup jobs |
| `route.completed` | Proxy success | ClickHouse, billing |
| `route.failed` | Proxy failure | Circuit breaker, alerts |
| `circuit.opened` | Threshold exceeded | PagerDuty, email |
| `circuit.closed` | Success after half-open | Dashboard |
| `billing.meter` | Billable unit | Stripe worker |

## Payload shape (JSON)

```json
{
  "id": "uuid",
  "type": "route.completed",
  "org_id": "acme",
  "occurred_at": "2026-08-02T12:00:00Z",
  "data": {
    "destination_agent_id": "cursor-1",
    "routing_strategy": "least-latency",
    "trace_id": "abc123"
  }
}
```

## Delivery guarantees

At-least-once internal delivery. Downstream webhook handlers must be idempotent on `id`.

## Webhooks (future)

Configure HTTPS endpoints per org in settings UI. Sign payloads with HMAC-SHA256 `X-AgentMesh-Signature`.

## Feature flags

`FF_EVENTS_WEBHOOKS` enables outbound delivery (default off in 0.1.0).

## Debugging

Enable debug log level on control plane; search for `event dispatch` and type filter.

## Retention

Event bus buffers are ephemeral; durable record is ClickHouse `routes` / `billing_events` where applicable.

## Security

Never include API keys or JWTs in event payloads. Redact `endpoint_url` query strings in external webhooks.

## Related

- [04-EVENTS in analytics](../docs/18-ANALYTICS.md)
- Email templates: [06-EMAIL.md](06-EMAIL.md)
