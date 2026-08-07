# Deployment

AgentMesh ships Docker images and a Helm chart with autoscaling and disruption budgets.

## Local (Docker Compose)

```bash
./scripts/start-all.sh    # or: make start / make docker-up
./scripts/status-all.sh
./scripts/stop-all.sh     # REMOVE_VOLUMES=1 to drop ClickHouse data
```

Utility reference: [`scripts/README.md`](../scripts/README.md).

```bash
make docker-up
make docker-down
```

Compose file: `deploy/docker-compose.yml` — includes etcd, Redis, ClickHouse, control plane, data plane, and optional TraceLens profile (`tracelens`).

Build images:

```bash
docker build -f deploy/Dockerfile.control-plane -t agentmesh/control-plane:0.1.0 .
docker build -f deploy/Dockerfile.dataplane -t agentmesh/dataplane:0.1.0 .
```

## Kubernetes (Helm)

Chart path: **`deploy/helm/agentmesh`**

Templates: control-plane, dataplane, frontend Deployments; ClusterIP Services; PDB + HPA (gated by `autoscaling.enabled`); ServiceMonitors for CP and DP.

```bash
# Staging
helm upgrade --install agentmesh deploy/helm/agentmesh \
  -n agentmesh-staging --create-namespace \
  -f deploy/helm/agentmesh/values.yaml \
  -f deploy/helm/agentmesh/values-staging.yaml \
  --wait

# Custom
helm upgrade --install agentmesh deploy/helm/agentmesh \
  -n agentmesh --create-namespace \
  -f my-values.yaml
```

Build images:

```bash
docker build -f deploy/Dockerfile.control-plane -t agentmesh/control-plane:0.1.0 .
docker build -f deploy/Dockerfile.dataplane -t agentmesh/dataplane:0.1.0 .
docker build -f deploy/Dockerfile.frontend -t agentmesh/frontend:0.1.0 .
```

### Default values highlights

| Key | Default |
|-----|---------|
| `replicaCount` | 2 |
| `controlPlane.image` | `agentmesh/control-plane:0.1.0` |
| `dataPlane.image` | `agentmesh/dataplane:0.1.0` |
| `frontend.enabled` | true |
| `frontend.image` | `agentmesh/frontend:0.1.0` |
| `autoscaling.enabled` | true |
| `autoscaling.minReplicas` | 2 |
| `autoscaling.maxReplicas` | 20 |
| `pdb.minAvailable` | 1 |
| `serviceMonitor.enabled` | true |
| `stripe.enabled` | false |

### HPA and PDB

CP and DP templates define:

- **HorizontalPodAutoscaler** (autoscaling/v2) on CPU when `autoscaling.enabled` is true (Deployment `replicas` omitted so HPA owns scale)
- **PodDisruptionBudget** with `minAvailable` from values

Ensure metrics server is installed in the cluster for HPA. Metering: see [ops/01-PAYMENTS.md](../ops/01-PAYMENTS.md).

### Probes

- Liveness: `GET /health` on control plane HTTP port
- Readiness: `GET /ready`

### External dependencies

Point values at managed services:

```yaml
etcd:
  endpoints:
    - etcd-client.etcd.svc:2379
redis:
  addr: redis-master.redis.svc:6379
clickhouse:
  addr: clickhouse.clickhouse.svc:9000
  database: agentmesh
tracelens:
  enabled: true
  otlpEndpoint: http://tracelens-collector:8080/v1/traces
guardloop:
  enabled: false
```

## Ports

Local Docker Compose publishes **host ports with a `4` prefix** (see [`deploy/ports.env`](../deploy/ports.env)). Containers still use standard internal ports for inter-service traffic.

| Service | Host port | Internal (in compose) |
|---------|-----------|-------------------------|
| Control plane HTTP | **48080** | 8080 |
| Control plane gRPC | **49090** | 9090 |
| Data plane | **48081** | 8081 |
| etcd | **42379** | 2379 |
| Redis | **46379** | 6379 |
| ClickHouse HTTP | **48123** | 8123 |
| ClickHouse native | **49000** | 9000 |
| Prometheus | **49091** | 9090 |
| Grafana | **43001** | 3000 |
| TraceLens (profile) | **48082** | 8080 |
| Frontend (`npm run dev`) | **43000** | — |

## Upgrades

1. Run migrations implicitly (ClickHouse DDL on startup via `internal/analytics`).
2. Roll control plane with PDB ensuring `minAvailable: 1`.
3. Roll data plane after control plane is ready.

## Monitoring

Apply Prometheus rules from `ops/monitoring/prometheus-rules.yml` and import Grafana dashboard `ops/monitoring/grafana-agentmesh.json`.

## Production checklist

- [ ] TLS ingress terminating HTTPS
- [ ] Secrets for TraceLens JWT and API keys
- [ ] HA etcd and Redis
- [ ] ClickHouse backup policy
- [ ] HPA max replicas sized for peak QPS

See also [14-ENV.md](14-ENV.md).
