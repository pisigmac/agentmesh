# Payments runbook

Stripe integration lives in `internal/billing/stripe.go`. Billing events are recorded in ClickHouse `billing_events` for reconciliation.

## Prerequisites

- Stripe account with metered products configured (see [17-PRICING.md](../docs/17-PRICING.md))
- Webhook signing secret in Kubernetes Secret `agentmesh-stripe`
- Org IDs match Stripe customer metadata `agentmesh_org_id`

## Event flow

1. Route or seat event emitted internally
2. Row inserted into `billing_events` with `stripe_event_id` for idempotency
3. Worker posts usage records to Stripe API
4. Invoice generated on billing cycle

## Idempotency

Always set `stripe_event_id` unique per logical event. Retries must reuse the same ID to avoid double billing.

## Failed webhooks

| Symptom | Action |
|---------|--------|
| 400 signature | Rotate webhook secret, update Secret |
| 500 handler | Check logs, replay from Stripe dashboard |
| Duplicate charge | Query CH for duplicate `stripe_event_id` |

## Manual credit

1. Issue Stripe credit note tied to customer
2. Insert adjusting row in `billing_events` with negative `quantity`

## Dunning

- Day 0: payment failed email (see [06-EMAIL.md](06-EMAIL.md))
- Day 3: throttle org to 5 RPS rate limit
- Day 7: suspend API keys except read-only discover

## Reconciliation query

```sql
SELECT org_id, event_type, sum(quantity) AS units
FROM billing_events
WHERE recorded_at >= toStartOfMonth(now())
GROUP BY org_id, event_type;
```

Compare totals to Stripe usage dashboard monthly.

## PCI scope

AgentMesh does not store PAN data—Stripe Checkout/Elements only. SAQ A applies when no card data touches our servers.

## Contacts

Finance ops escalation via on-call; include org ID and Stripe customer ID.
