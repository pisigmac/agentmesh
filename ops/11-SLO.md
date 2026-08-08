# AgentMesh SLOs

Targets for design-partner and limited GA. Burn-rate alerts live in [`monitoring/prometheus-rules.yml`](monitoring/prometheus-rules.yml).

## Service level objectives

| SLO | Target | Measurement |
|-----|--------|-------------|
| Control plane availability | 99.5% monthly | `up{job="agentmesh-control-plane"}` and `GET /ready` success |
| Route success rate | ≥ 99% (excl. client 4xx / no-route) | `1 - rate(agentmesh_route_total{status="error"}[…] / rate(agentmesh_route_total[…]))` |
| Route latency p99 | ≤ 120 ms control-plane decision (excludes upstream proxy) | `agentmesh_route_latency_ms` histogram |
| Dataplane scrape up | 99% | `up{job="agentmesh-dataplane"}` |

Error budget for route success: **1%** of monthly route volume. Page on fast burn (see Prometheus rules).

## On-call notes

1. Check Prometheus targets and Grafana dashboards under `ops/monitoring/`.
2. Control plane `/ready` failing → etcd/Redis/CH or process crash; see `./scripts/logs-all.sh control-plane`.
3. High route failures → circuits open, score provider timeout, or billing `blocked` status (`GET /v1/orgs/{org}/billing`).
4. Dataplane 401s → clients missing Bearer; `DATAPLANE_AUTH_ENABLED` + shared `AUTH_JWT_SECRET` / org API keys.
5. Staging dry-run (monthly or pre-release): `./scripts/dry-run-design-partner.sh` and `./tests/load/smoke.sh` with stack up.

## Monthly staging cron (suggested)

```cron
0 9 1 * * cd /opt/agentmesh && ./scripts/status-all.sh && ./tests/load/smoke.sh && ./scripts/dry-run-design-partner.sh
```

CI runs unit + chaos tests on every PR (see `.github/workflows/ci.yml`). Full stack smoke remains staging/manual until compose is available in CI.
