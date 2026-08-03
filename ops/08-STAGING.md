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
2. Build images tagged `0.1.0-rc.*`
3. Helm upgrade staging with `--wait`
4. Run `tests/load/smoke.sh` against staging URL
5. Manual sign-off for prod promote

## Feature flags

Enable experimental flags here first (`FF_EVENTS_WEBHOOKS`, etc.).

## GuardLoop

Set `guardloop.enabled: true` in Helm values with sandbox base URL.

## Auth testing

Use dedicated API keys per engineer; never share production Bearer tokens.

## Chaos

Run etcd partition and GuardLoop timeout scenarios monthly ([tests/chaos/README.md](../tests/chaos/README.md)).

## Teardown

`helm uninstall agentmesh -n agentmesh-staging` — ClickHouse data retained 30 days per retention policy.
