# Payments runbook

Stripe integration lives in `internal/billing`. Successful control-plane routes insert ClickHouse `billing_events` and optionally post Stripe Billing Meter Events.

## Prerequisites

- Stripe account with a Billing Meter (default event name `agentmesh_routes`) — see [17-PRICING.md](../docs/17-PRICING.md)
- Secret `agentmesh-stripe` with key `secret_key` (`sk_test_…` for staging)
- Org → customer map: `STRIPE_ORG_CUSTOMERS=acme:cus_xxx,beta:cus_yyy` (also Stripe customer metadata `agentmesh_org_id`)

## Event flow

1. Client calls `POST /v1/route` (or gRPC `RouteRequest`) successfully
2. Control plane inserts `billing_events` with `event_type=route`, `quantity=1`, `stripe_event_id=route:{trace_id}`
3. If `STRIPE_ENABLED=true` and the org has a mapped customer, posts `POST /v1/billing/meter_events` with the same identifier
4. Stripe invoices on the meter’s billing cycle

## Configuration

| Env | Purpose |
|-----|---------|
| `STRIPE_ENABLED` | Master switch |
| `STRIPE_SECRET_KEY` | API key |
| `STRIPE_METER_EVENT_NAME` | Meter event name (default `agentmesh_routes`) |
| `STRIPE_ORG_CUSTOMERS` | `org:cus_…` CSV map |

Helm: `stripe.enabled`, `stripe.secretKeyExistingSecret`, `stripe.orgCustomers` in [`deploy/helm/agentmesh/values.yaml`](../deploy/helm/agentmesh/values.yaml).

## Idempotency

Always set `stripe_event_id` / Stripe `identifier` to `route:{trace_id}`. Retries must reuse the same ID to avoid double billing.

## Failed Stripe posts

| Symptom | Action |
|---------|--------|
| No customer mapping | CH row still written; add org to `STRIPE_ORG_CUSTOMERS` |
| 401/403 from Stripe | Rotate `secret_key` in Secret |
| 400 invalid meter | Align `STRIPE_METER_EVENT_NAME` with dashboard meter |

Webhook handlers / dunning are follow-ups (not in this MVP).

## Manual credit

1. Issue Stripe credit note tied to customer
2. Insert adjusting row in `billing_events` with negative `quantity`

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
