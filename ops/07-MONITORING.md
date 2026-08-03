# Monitoring

AgentMesh observability stack: Prometheus metrics, Grafana dashboards, TraceLens traces.

## Metrics endpoints

Scrape control plane and data plane pods on Prometheus `/metrics` (client_golang default handler when mounted).

Key series:

- `agentmesh_route_total{org,strategy,status}`
- `agentmesh_route_latency_ms_bucket{org,mode}`

Sample scrape config: `deploy/prometheus.yml`.

## Alert rules

Apply **`ops/monitoring/prometheus-rules.yml`** to your Prometheus or Prometheus Operator `PrometheusRule` CRD.

Critical alerts:

- High route failure ratio (>5% for 10m)
- p99 route latency above perf budget
- Control plane pod not ready

## Grafana

Import **`ops/monitoring/grafana-agentmesh.json`** (minimal dashboard):

- Route rate by strategy
- Error ratio
- Latency heatmap

## TraceLens

Verify OTLP export:

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TRACELENS_JWT" \
  -H "Content-Type: application/json" \
  -d '{"resourceSpans":[]}' \
  "$TRACELENS_OTLP_ENDPOINT"
```

Do **not** configure Jaeger exporter in production—TraceLens is the supported backend.

## Logs

Structured JSON logs via zap in control plane mains. Correlate with `trace_id` from route responses.

## On-call runbook links

| Alert | Doc |
|-------|-----|
| Circuit storm | [19-ERRORS.md](../docs/19-ERRORS.md) |
| etcd down | [tests/chaos/README.md](../tests/chaos/README.md) |
| Payment throttle | [01-PAYMENTS.md](01-PAYMENTS.md) |

## SLO dashboard

Track 30-day rolling availability for `/v1/route` success—align with [03-PERF-BUDGET.md](03-PERF-BUDGET.md) error budget.

## Kubernetes

HPA events: `kubectl describe hpa agentmesh-control-plane`. PDB status during node drains.
