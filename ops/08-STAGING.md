# Staging environment

Staging mirrors production topology at reduced scale for validation before Helm promote.

## Topology

| Component | Staging spec |
|-----------|--------------|
| Control plane | 2 replicas, HPA max 5 |
| Data plane | 1 replica |
| etcd | 3-node cluster (shared staging) |
| Redis | Single primary + replica |
| ClickHouse | 1 shard, 1 replica |
| TraceLens | Staging collector URL |

## URLs

- API: `https://staging-api.agentmesh.example`
- UI: `https://staging.agentmesh.example`

## Secrets

Kubernetes namespace `agentmesh-staging`:

- `TRACELENS_JWT` — staging project token
- `GUARDLOOP_API_KEY` — sandbox key when testing score-based routing
- Stripe test mode keys only

## Data

Synthetic orgs `staging-acme`, `staging-beta`. No production PII.

Refresh weekly from anonymized fixture scripts.

## Deploy pipeline

1. CI runs `make test` + `verify-openapi`
2. Build images tagged `0.1.0-rc.*` (control-plane, dataplane, frontend)
3. Ensure Secret `agentmesh-stripe` exists (`secret_key`) when metering is on
4. Helm upgrade:

```bash
helm upgrade --install agentmesh ./deploy/helm/agentmesh \
  -n agentmesh-staging --create-namespace \
  -f deploy/helm/agentmesh/values.yaml \
  -f deploy/helm/agentmesh/values-staging.yaml \
  --wait
```

5. Confirm Services: `agentmesh-control-plane`, `agentmesh-dataplane`, `agentmesh-frontend`
6. Run `tests/load/smoke.sh` against staging URL
7. Metering smoke: `POST /v1/route` → row in `billing_events`; Stripe test meter increments when `STRIPE_ORG_CUSTOMERS` is set
8. Manual sign-off for prod promote

## Feature flags

Enable experimental flags here first (`FF_EVENTS_WEBHOOKS`, etc.).

## GuardLoop / score-based routing

1. Set `GUARDLOOP_ENABLED=true`, `GUARDLOOP_BASE_URL`, `GUARDLOOP_API_KEY` (sandbox).
2. Leave `SCORE_PROVIDER=auto` (or set `guardloop` explicitly). Without GuardLoop, staging still uses the **fixture** provider so `score-based` demos work offline.
3. Verify: `POST /v1/route` with `"routing_strategy":"score-based"` returns `score.provider` (`guardloop` or `fixture`) and `x-agentmesh-score-detail` in injected headers.
4. Chaos: stop GuardLoop or set a bad URL — response should include `score.fallback=least-latency` (see [tests/chaos/README.md](../tests/chaos/README.md)).
5. Helm: `guardloop.enabled: true` in values with sandbox base URL.

## Auth testing

Use dedicated API keys per engineer; never share production Bearer tokens.

## Chaos

Run etcd partition and GuardLoop timeout scenarios monthly ([tests/chaos/README.md](../tests/chaos/README.md)).

## Teardown

`helm uninstall agentmesh -n agentmesh-staging` — ClickHouse data retained 30 days per retention policy.
